package admin

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/suprelory/redact-gateway/internal/config"
	"github.com/suprelory/redact-gateway/internal/gateway"
	"github.com/suprelory/redact-gateway/internal/route"
	"github.com/suprelory/redact-gateway/internal/store"
)

//go:embed web
var webAssets embed.FS

type Server struct {
	token      string
	proxy      *gateway.Proxy
	store      *store.Store
	web        http.Handler
	settingsMu sync.Mutex
}

type ruleInfo struct {
	Flag        string `json:"flag"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     bool   `json:"default"`
}

type settingsResponse struct {
	AllowedHosts []string `json:"allowed_hosts"`
}

func NewServer(token string, proxy *gateway.Proxy, eventStore *store.Store) *Server {
	assets, err := fs.Sub(webAssets, "web/dist")
	server := &Server{token: token, proxy: proxy, store: eventStore}
	if err == nil {
		server.web = http.FileServer(http.FS(assets))
	} else {
		fallback, _ := webAssets.ReadFile("web/fallback.html")
		server.web = http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = writer.Write(fallback)
		})
	}
	return server
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", func(writer http.ResponseWriter, _ *http.Request) {
		respondJSON(writer, http.StatusOK, map[string]any{"ok": true})
	})
	mux.Handle("GET /api/v1/status", s.auth(http.HandlerFunc(s.status)))
	mux.Handle("GET /api/v1/events", s.auth(http.HandlerFunc(s.events)))
	mux.Handle("GET /api/v1/stats", s.auth(http.HandlerFunc(s.stats)))
	mux.Handle("GET /api/v1/rules", s.auth(http.HandlerFunc(s.rules)))
	mux.Handle("GET /api/v1/settings", s.auth(http.HandlerFunc(s.settings)))
	mux.Handle("PUT /api/v1/settings", s.auth(http.HandlerFunc(s.updateSettings)))
	mux.HandleFunc("/", s.serveWeb)
	return securityHeaders(mux)
}

func (s *Server) status(writer http.ResponseWriter, _ *http.Request) {
	respondJSON(writer, http.StatusOK, s.proxy.Status())
}

func (s *Server) events(writer http.ResponseWriter, request *http.Request) {
	limit, _ := strconv.Atoi(request.URL.Query().Get("limit"))
	events, err := s.store.Events(request.Context(), limit, strings.TrimSpace(request.URL.Query().Get("upstream")))
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "query_failed", "failed to query request events")
		return
	}
	respondJSON(writer, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) stats(writer http.ResponseWriter, request *http.Request) {
	windowHours, _ := strconv.Atoi(request.URL.Query().Get("hours"))
	if windowHours <= 0 || windowHours > 24*90 {
		windowHours = 24
	}
	stats, err := s.store.Stats(request.Context(), time.Now().Add(-time.Duration(windowHours)*time.Hour))
	if err != nil {
		respondError(writer, http.StatusInternalServerError, "query_failed", "failed to query gateway statistics")
		return
	}
	respondJSON(writer, http.StatusOK, stats)
}

func (s *Server) rules(writer http.ResponseWriter, _ *http.Request) {
	rules := []ruleInfo{
		{Flag: "H", Name: "高熵字符串", Description: "检测混合字母数字的高随机度令牌", Default: true},
		{Flag: "P", Name: "电话号码", Description: "中国大陆手机号和国际电话号码", Default: true},
		{Flag: "S", Name: "sk- 密钥", Description: "检测常见 sk- 前缀 API 密钥", Default: true},
		{Flag: "I", Name: "身份证", Description: "带校验位验证的 18 位中国身份证号", Default: true},
		{Flag: "B", Name: "银行卡", Description: "13 至 19 位并通过 Luhn 校验的卡号", Default: true},
		{Flag: "E", Name: "邮箱", Description: "标准电子邮箱地址", Default: true},
		{Flag: "G", Name: "凭据规则包", Description: "私钥、JWT、云密钥、GitHub/GitLab Token 和连接串", Default: true},
	}
	respondJSON(writer, http.StatusOK, map[string]any{"all_flags": route.AllFlagLetters, "rules": rules})
}

func (s *Server) settings(writer http.ResponseWriter, _ *http.Request) {
	respondJSON(writer, http.StatusOK, settingsResponse{AllowedHosts: s.proxy.AllowedHosts()})
}

func (s *Server) updateSettings(writer http.ResponseWriter, request *http.Request) {
	s.settingsMu.Lock()
	defer s.settingsMu.Unlock()

	var input struct {
		AllowedHosts *[]string `json:"allowed_hosts"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || input.AllowedHosts == nil {
		respondError(writer, http.StatusBadRequest, "invalid_settings", "allowed_hosts must be a JSON array")
		return
	}
	hosts, err := config.NormalizeAllowedHosts(*input.AllowedHosts)
	if err != nil {
		respondError(writer, http.StatusBadRequest, "invalid_allowed_hosts", err.Error())
		return
	}
	if err := s.store.SaveAllowedHosts(request.Context(), hosts); err != nil {
		respondError(writer, http.StatusInternalServerError, "settings_save_failed", "failed to save gateway settings")
		return
	}
	s.proxy.SetAllowedHosts(hosts)
	respondJSON(writer, http.StatusOK, settingsResponse{AllowedHosts: hosts})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !sameOrigin(request) {
			respondError(writer, http.StatusForbidden, "origin_rejected", "request origin does not match the control plane host")
			return
		}
		provided := strings.TrimSpace(request.Header.Get("X-Redact-Token"))
		if provided == "" {
			provided = strings.TrimSpace(strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "))
		}
		if len(provided) != len(s.token) || subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
			respondError(writer, http.StatusUnauthorized, "unauthorized", "valid control plane token required")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (s *Server) serveWeb(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.web.ServeHTTP(writer, request)
}

func sameOrigin(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, request.Host)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		next.ServeHTTP(writer, request)
	})
}

func respondJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func respondError(writer http.ResponseWriter, status int, code, message string) {
	respondJSON(writer, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
