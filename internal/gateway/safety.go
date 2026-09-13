package gateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/suprelory/redact-gateway/internal/config"
)

func newHTTPClient(cfg config.Config) *http.Client {
	dialer := &safeDialer{allowPrivate: cfg.AllowPrivateHosts}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		DisableCompression:    true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return &http.Client{
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func validateUpstream(ctx context.Context, target *url.URL, cfg config.Config) error {
	host := strings.ToLower(target.Hostname())
	if len(cfg.AllowedHosts) > 0 {
		allowed := false
		for _, candidate := range cfg.AllowedHosts {
			if host == candidate || strings.HasSuffix(host, "."+candidate) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("upstream host is not allowed")
		}
	}
	if cfg.AllowPrivateHosts {
		return nil
	}
	ips, err := resolveHost(ctx, host)
	if err != nil {
		return fmt.Errorf("resolve upstream host: %w", err)
	}
	for _, ip := range ips {
		if forbiddenIP(ip) {
			return fmt.Errorf("private or special-use upstream address is blocked")
		}
	}
	return nil
}

type safeDialer struct {
	allowPrivate bool
}

func (d *safeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	if d.allowPrivate {
		return (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, address)
	}
	ips, err := resolveHost(ctx, host)
	if err != nil {
		return nil, err
	}
	for _, ip := range ips {
		if forbiddenIP(ip) {
			return nil, fmt.Errorf("refusing to dial private or special-use address %s", ip)
		}
	}
	var lastErr error
	for _, ip := range ips {
		conn, dialErr := (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext(
			ctx, network, net.JoinHostPort(ip.String(), port),
		)
		if dialErr == nil {
			return conn, nil
		}
		lastErr = dialErr
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("upstream host resolved to no addresses")
	}
	return nil, lastErr
}

func resolveHost(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		ips = append(ips, address.IP)
	}
	return ips, nil
}

func forbiddenIP(ip net.IP) bool {
	return ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast()
}
