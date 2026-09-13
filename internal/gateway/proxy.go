package gateway

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/suprelory/redact-gateway/internal/config"
	"github.com/suprelory/redact-gateway/internal/redact"
	"github.com/suprelory/redact-gateway/internal/route"
	"github.com/suprelory/redact-gateway/internal/store"
)

type Status struct {
	Service      string `json:"service"`
	Version      string `json:"version"`
	ProxyAddr    string `json:"proxy_addr"`
	AdminAddr    string `json:"admin_addr"`
	StartedAt    string `json:"started_at"`
	UptimeSecond int64  `json:"uptime_seconds"`
	InFlight     int64  `json:"in_flight"`
	AllowedHosts int    `json:"allowed_hosts"`
	AllowPrivate bool   `json:"allow_private_upstreams"`
	MaxBodyBytes int64  `json:"max_body_bytes"`
}

type Proxy struct {
	cfg          config.Config
	store        *store.Store
	client       *http.Client
	logger       *slog.Logger
	startedAt    time.Time
	version      string
	inFlight     atomic.Int64
	settingsMu   sync.RWMutex
	allowedHosts []string
}

func NewProxy(cfg config.Config, eventStore *store.Store, logger *slog.Logger, version string) *Proxy {
	return &Proxy{
		cfg: cfg, store: eventStore, client: newHTTPClient(cfg), logger: logger,
		startedAt: time.Now(), version: version, allowedHosts: cloneHosts(cfg.AllowedHosts),
	}
}

func (p *Proxy) AllowedHosts() []string {
	p.settingsMu.RLock()
	defer p.settingsMu.RUnlock()
	return cloneHosts(p.allowedHosts)
}

func (p *Proxy) SetAllowedHosts(hosts []string) {
	p.settingsMu.Lock()
	p.allowedHosts = cloneHosts(hosts)
	p.settingsMu.Unlock()
}

func cloneHosts(hosts []string) []string {
	if hosts == nil {
		return []string{}
	}
	return append([]string(nil), hosts...)
}

func (p *Proxy) Status() Status {
	return Status{
		Service: "redact-gateway", Version: p.version,
		ProxyAddr: p.cfg.ListenAddr, AdminAddr: p.cfg.AdminAddr,
		StartedAt:    p.startedAt.UTC().Format(time.RFC3339),
		UptimeSecond: int64(time.Since(p.startedAt).Seconds()),
		InFlight:     p.inFlight.Load(), AllowedHosts: len(p.AllowedHosts()),
		AllowPrivate: p.cfg.AllowPrivateHosts, MaxBodyBytes: p.cfg.MaxBodyBytes,
	}
}

func (p *Proxy) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/healthz" {
		writeJSON(writer, http.StatusOK, map[string]any{"ok": true, "service": "redact-gateway", "version": p.version}, p.cfg.CORSOrigin)
		return
	}
	if request.Method == http.MethodOptions {
		writePreflight(writer, request, p.cfg.CORSOrigin)
		return
	}

	p.inFlight.Add(1)
	defer p.inFlight.Add(-1)
	started := time.Now()
	requestID := newRequestID()

	proxyRoute, err := route.ParseRequestURI(request.RequestURI)
	if err != nil {
		writeGatewayError(writer, http.StatusBadRequest, "invalid_route", err.Error(), p.cfg.CORSOrigin)
		return
	}
	event := store.Event{
		RequestID: requestID, Timestamp: started.UTC().Format(time.RFC3339Nano), Method: request.Method,
		UpstreamScheme: proxyRoute.Upstream.Scheme, UpstreamHost: proxyRoute.Upstream.Hostname(),
		UpstreamPort: proxyRoute.Upstream.Port(), UpstreamPath: proxyRoute.Upstream.EscapedPath(),
		Flags: proxyRoute.Flags.Raw,
	}
	if event.UpstreamPath == "" {
		event.UpstreamPath = "/"
	}
	recorded := false
	record := func() {
		if recorded {
			return
		}
		recorded = true
		event.DurationMS = time.Since(started).Milliseconds()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := p.store.InsertEvent(ctx, event); err != nil {
			p.logger.Error("record request event", "request_id", requestID, "error", err)
		}
	}
	defer record()

	validationConfig := p.cfg
	validationConfig.AllowedHosts = p.AllowedHosts()
	if err := validateUpstream(request.Context(), proxyRoute.Upstream, validationConfig); err != nil {
		event.Status = http.StatusForbidden
		event.ErrorClass = "upstream_blocked"
		writeGatewayError(writer, event.Status, event.ErrorClass, err.Error(), p.cfg.CORSOrigin)
		return
	}
	if encoding := strings.TrimSpace(request.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		event.Status = http.StatusUnsupportedMediaType
		event.ErrorClass = "encoded_request"
		writeGatewayError(writer, event.Status, event.ErrorClass, "encoded request bodies are not supported", p.cfg.CORSOrigin)
		return
	}

	contextMap := redact.NewContext(p.cfg.MaxRedactions)
	body, protocol, originalBytes, err := p.prepareRequestBody(request, proxyRoute, contextMap)
	event.RequestBytes = originalBytes
	event.Protocol = protocol
	if err != nil {
		status, class := requestErrorStatus(err)
		event.Status = status
		event.ErrorClass = class
		writeGatewayError(writer, status, class, err.Error(), p.cfg.CORSOrigin)
		return
	}
	event.RedactionCount = contextMap.RedactionCount()
	event.RuleHits = contextMap.Hits()
	event.RedactionFields = contextMap.RedactionFields()

	upstreamRequest, err := http.NewRequestWithContext(request.Context(), request.Method, proxyRoute.Upstream.String(), bytes.NewReader(body))
	if err != nil {
		event.Status = http.StatusBadRequest
		event.ErrorClass = "invalid_upstream_request"
		writeGatewayError(writer, event.Status, event.ErrorClass, err.Error(), p.cfg.CORSOrigin)
		return
	}
	copyRequestHeaders(upstreamRequest.Header, request.Header)
	upstreamRequest.Header.Set("Accept-Encoding", "identity")
	if len(body) > 0 {
		upstreamRequest.Header.Set("Content-Type", "application/json")
	}
	upstreamRequest.Host = proxyRoute.Upstream.Host

	response, err := p.client.Do(upstreamRequest)
	if err != nil {
		event.Status = http.StatusBadGateway
		event.ErrorClass = "upstream_failed"
		writeGatewayError(writer, event.Status, event.ErrorClass, "upstream request failed", p.cfg.CORSOrigin)
		p.logger.Warn("upstream request failed", "request_id", requestID, "host", event.UpstreamHost, "error", err)
		return
	}
	defer response.Body.Close()

	event.Status = response.StatusCode
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	event.Streaming = strings.Contains(contentType, "text/event-stream")
	if event.Streaming {
		event.ResponseBytes, event.ErrorClass = p.streamSSE(writer, request, response, contextMap)
		if event.ErrorClass == "encoded_response" {
			event.Status = http.StatusBadGateway
		}
		event.RestoreCount = contextMap.RestoreCount()
		return
	}
	event.ResponseBytes, event.ErrorClass = p.writeNonStreaming(writer, response, contextMap)
	switch event.ErrorClass {
	case "encoded_response", "response_read_failed", "response_too_large":
		event.Status = http.StatusBadGateway
	}
	event.RestoreCount = contextMap.RestoreCount()
}

func (p *Proxy) prepareRequestBody(request *http.Request, proxyRoute route.ProxyRoute, contextMap *redact.Context) ([]byte, string, int64, error) {
	if request.Body == nil || request.Body == http.NoBody {
		return nil, protocolFromPath(proxyRoute.Upstream.Path), 0, nil
	}
	if request.ContentLength > p.cfg.MaxBodyBytes {
		return nil, "generic", request.ContentLength, errRequestTooLarge
	}
	raw, err := io.ReadAll(io.LimitReader(request.Body, p.cfg.MaxBodyBytes+1))
	if err != nil {
		return nil, "generic", 0, fmt.Errorf("read request body: %w", err)
	}
	if int64(len(raw)) > p.cfg.MaxBodyBytes {
		return nil, "generic", int64(len(raw)), errRequestTooLarge
	}
	if len(raw) == 0 {
		return nil, protocolFromPath(proxyRoute.Upstream.Path), 0, nil
	}
	if !isJSONContentType(request.Header.Get("Content-Type")) {
		return nil, "generic", int64(len(raw)), errUnsupportedBody
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, "generic", int64(len(raw)), fmt.Errorf("invalid JSON request body: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, "generic", int64(len(raw)), err
	}
	flags := redact.DetectorFlags{
		HighEntropy: proxyRoute.Flags.HighEntropy, Phone: proxyRoute.Flags.Phone,
		Secret: proxyRoute.Flags.Secret, Identity: proxyRoute.Flags.Identity,
		Bank: proxyRoute.Flags.Bank, Email: proxyRoute.Flags.Email, Gitleaks: proxyRoute.Flags.Gitleaks,
	}
	redactedValue, err := redact.RedactJSON(value, contextMap, flags)
	if err != nil {
		return nil, "generic", int64(len(raw)), err
	}
	protocol := redact.DetectProtocol(redactedValue, proxyRoute.Upstream.Path, request.Header.Get("Anthropic-Version") != "")
	redact.InjectNotice(redactedValue, protocol)
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(redactedValue); err != nil {
		return nil, protocol, int64(len(raw)), fmt.Errorf("encode redacted request: %w", err)
	}
	return bytes.TrimSuffix(encoded.Bytes(), []byte("\n")), protocol, int64(len(raw)), nil
}

func (p *Proxy) streamSSE(writer http.ResponseWriter, request *http.Request, response *http.Response, contextMap *redact.Context) (int64, string) {
	if encoding := strings.TrimSpace(response.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		writeGatewayError(writer, http.StatusBadGateway, "encoded_response", "upstream returned an encoded stream", p.cfg.CORSOrigin)
		return 0, "encoded_response"
	}
	copyResponseHeaders(writer.Header(), response.Header, p.cfg.CORSOrigin)
	writer.WriteHeader(response.StatusCode)
	flusher, _ := writer.(http.Flusher)
	restorer := redact.NewSSEStreamRestorer(contextMap)
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		read, readErr := response.Body.Read(buffer)
		if read > 0 {
			output, restoreErr := restorer.Push(buffer[:read])
			if restoreErr != nil {
				p.logger.Error("restore SSE", "error", restoreErr)
				return written, "restore_failed"
			}
			if len(output) > 0 {
				n, writeErr := writer.Write(output)
				written += int64(n)
				if writeErr != nil {
					return written, "client_disconnected"
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) && request.Context().Err() == nil {
				return written, "upstream_stream_failed"
			}
			break
		}
	}
	tail, err := restorer.Finish()
	if err != nil {
		return written, "restore_failed"
	}
	if len(tail) > 0 {
		n, writeErr := writer.Write(tail)
		written += int64(n)
		if writeErr != nil {
			return written, "client_disconnected"
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	return written, ""
}

func (p *Proxy) writeNonStreaming(writer http.ResponseWriter, response *http.Response, contextMap *redact.Context) (int64, string) {
	if response.StatusCode == http.StatusNoContent || response.StatusCode == http.StatusNotModified {
		copyResponseHeaders(writer.Header(), response.Header, p.cfg.CORSOrigin)
		writer.WriteHeader(response.StatusCode)
		return 0, ""
	}
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if !isTextualContentType(contentType) {
		copyResponseHeaders(writer.Header(), response.Header, p.cfg.CORSOrigin)
		writer.WriteHeader(response.StatusCode)
		written, err := io.Copy(writer, response.Body)
		if err != nil {
			return written, "response_copy_failed"
		}
		return written, ""
	}
	if encoding := strings.TrimSpace(response.Header.Get("Content-Encoding")); encoding != "" && !strings.EqualFold(encoding, "identity") {
		writeGatewayError(writer, http.StatusBadGateway, "encoded_response", "upstream returned an encoded textual response", p.cfg.CORSOrigin)
		return 0, "encoded_response"
	}
	limit := p.cfg.MaxBodyBytes * 2
	raw, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		writeGatewayError(writer, http.StatusBadGateway, "response_read_failed", "failed to read upstream response", p.cfg.CORSOrigin)
		return 0, "response_read_failed"
	}
	if int64(len(raw)) > limit {
		writeGatewayError(writer, http.StatusBadGateway, "response_too_large", "textual upstream response exceeds the safety limit", p.cfg.CORSOrigin)
		return 0, "response_too_large"
	}
	output := raw
	if isJSONContentType(contentType) {
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var value any
		if decoder.Decode(&value) == nil && ensureJSONEOF(decoder) == nil {
			value = redact.RestoreJSON(value, contextMap)
			if encoded, encodeErr := json.Marshal(value); encodeErr == nil {
				output = encoded
			}
		} else {
			output = []byte(contextMap.RestoreText(string(raw)))
		}
	} else {
		output = []byte(contextMap.RestoreText(string(raw)))
	}
	copyResponseHeaders(writer.Header(), response.Header, p.cfg.CORSOrigin)
	writer.WriteHeader(response.StatusCode)
	written, writeErr := writer.Write(output)
	if writeErr != nil {
		return int64(written), "client_disconnected"
	}
	return int64(written), ""
}

var (
	errRequestTooLarge = errors.New("request body exceeds the configured limit")
	errUnsupportedBody = errors.New("non-empty request bodies must use a JSON content type")
)

func requestErrorStatus(err error) (int, string) {
	switch {
	case errors.Is(err, errRequestTooLarge), errors.Is(err, redact.ErrRedactionLimit):
		return http.StatusRequestEntityTooLarge, "request_too_large"
	case errors.Is(err, errUnsupportedBody):
		return http.StatusUnsupportedMediaType, "unsupported_body"
	default:
		return http.StatusBadRequest, "invalid_request"
	}
}

func protocolFromPath(path string) string {
	return redact.DetectProtocol(nil, path, false)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("request body must contain exactly one JSON value")
		}
		return fmt.Errorf("invalid JSON request body: %w", err)
	}
	return nil
}

func isJSONContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "/json") || strings.Contains(contentType, "+json")
}

func isTextualContentType(contentType string) bool {
	return isJSONContentType(contentType) || strings.HasPrefix(contentType, "text/") ||
		strings.Contains(contentType, "javascript") || strings.Contains(contentType, "xml")
}

func copyRequestHeaders(target, source http.Header) {
	connectionHeaders := connectionHeaderNames(source)
	for name, values := range source {
		if skipRequestHeader(name) {
			continue
		}
		if _, blocked := connectionHeaders[strings.ToLower(name)]; blocked {
			continue
		}
		for _, value := range values {
			target.Add(name, value)
		}
	}
}

func copyResponseHeaders(target, source http.Header, corsOrigin string) {
	connectionHeaders := connectionHeaderNames(source)
	for name, values := range source {
		if skipResponseHeader(name) {
			continue
		}
		if _, blocked := connectionHeaders[strings.ToLower(name)]; blocked {
			continue
		}
		for _, value := range values {
			target.Add(name, value)
		}
	}
	target.Set("Access-Control-Allow-Origin", corsOrigin)
	target.Set("Access-Control-Expose-Headers", "*")
	target.Del("Content-Length")
}

func skipRequestHeader(name string) bool {
	key := strings.ToLower(name)
	switch key {
	case "host", "content-length", "connection", "transfer-encoding", "keep-alive",
		"proxy-authenticate", "proxy-authorization", "te", "trailer", "upgrade",
		"proxy-connection", "accept-encoding", "x-real-ip", "forwarded", "via",
		"cookie", "cookie2", "origin", "referer", "x-redact-token":
		return true
	default:
		return strings.HasPrefix(key, "cf-") || strings.HasPrefix(key, "sec-") || strings.HasPrefix(key, "x-forwarded-")
	}
}

func skipResponseHeader(name string) bool {
	switch strings.ToLower(name) {
	case "content-length", "content-encoding", "connection", "transfer-encoding", "keep-alive",
		"proxy-authenticate", "proxy-authorization", "proxy-connection", "te", "trailer", "upgrade":
		return true
	default:
		return false
	}
}

func connectionHeaderNames(headers http.Header) map[string]struct{} {
	names := make(map[string]struct{})
	for _, value := range headers.Values("Connection") {
		for _, name := range strings.Split(value, ",") {
			if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
				names[name] = struct{}{}
			}
		}
	}
	return names
}

func writePreflight(writer http.ResponseWriter, request *http.Request, origin string) {
	writer.Header().Set("Access-Control-Allow-Origin", origin)
	writer.Header().Set("Access-Control-Allow-Methods", "GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS")
	headers := request.Header.Get("Access-Control-Request-Headers")
	if headers == "" {
		headers = "authorization,content-type,x-api-key,anthropic-version,openai-organization,openai-project"
	}
	writer.Header().Set("Access-Control-Allow-Headers", headers)
	writer.Header().Set("Access-Control-Max-Age", "86400")
	writer.WriteHeader(http.StatusNoContent)
}

func writeGatewayError(writer http.ResponseWriter, status int, code, message, origin string) {
	writeJSON(writer, status, map[string]any{
		"error": map[string]any{"message": message, "type": "redact_gateway_error", "code": code},
	}, origin)
}

func writeJSON(writer http.ResponseWriter, status int, value any, origin string) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Access-Control-Allow-Origin", origin)
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func newRequestID() string {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}
