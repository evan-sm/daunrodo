// Package proxy handles proxy management including selection and health checking.
package proxy

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	defaultSOCKSPort = "1080"
	defaultHTTPPort  = "8080"
)

// Manager handles proxy selection and health checking.
type Manager struct {
	proxies       []string
	healthCheck   bool
	healthTimeout time.Duration
}

// New creates a new proxy manager.
func New(proxyURLs string, healthCheck bool, healthTimeout time.Duration) (*Manager, error) {
	if proxyURLs == "" {
		return &Manager{
			proxies:       []string{},
			healthCheck:   healthCheck,
			healthTimeout: healthTimeout,
		}, nil
	}

	proxies := strings.Split(proxyURLs, ",")
	cleaned := make([]string, 0, len(proxies))

	for _, p := range proxies {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Validate proxy URL
		if _, err := url.Parse(p); err != nil {
			return nil, fmt.Errorf("invalid proxy URL %q: %w", p, err)
		}

		cleaned = append(cleaned, p)
	}

	return &Manager{
		proxies:       cleaned,
		healthCheck:   healthCheck,
		healthTimeout: healthTimeout,
	}, nil
}

// GetProxy returns a random healthy proxy URL, or empty string if no proxies configured.
func (m *Manager) GetProxy(ctx context.Context) (string, error) {
	if len(m.proxies) == 0 {
		return "", nil
	}

	if !m.healthCheck {
		return m.selectRandom(), nil
	}

	// Try to find a healthy proxy - shuffle and try each once
	indices := rand.Perm(len(m.proxies))
	for _, idx := range indices {
		proxy := m.proxies[idx]
		if m.checkHealth(ctx, proxy) {
			return proxy, nil
		}
	}

	return "", fmt.Errorf("no healthy proxies available")
}

// selectRandom returns a random proxy from the list.
func (m *Manager) selectRandom() string {
	if len(m.proxies) == 0 {
		return ""
	}
	return m.proxies[rand.IntN(len(m.proxies))]
}

// checkHealth checks if a proxy is healthy by attempting to connect to it.
func (m *Manager) checkHealth(ctx context.Context, proxyURL string) bool {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return false
	}

	// Extract host and port
	host := u.Host
	if !strings.Contains(host, ":") {
		// Add default port based on scheme
		switch u.Scheme {
		case "socks5", "socks5h":
			host = host + ":" + defaultSOCKSPort
		case "http", "https":
			host = host + ":" + defaultHTTPPort
		default:
			return false
		}
	}

	// Create a context with timeout
	checkCtx, cancel := context.WithTimeout(ctx, m.healthTimeout)
	defer cancel()

	// Try to establish a TCP connection
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(checkCtx, "tcp", host)
	if err != nil {
		return false
	}
	conn.Close()

	return true
}

// Count returns the number of configured proxies.
func (m *Manager) Count() int {
	return len(m.proxies)
}
