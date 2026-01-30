// Package proxymgr provides proxy management for download requests.
// It handles proxy rotation, health checking, and failure tracking.
package proxymgr

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/url"
	"sync"
	"time"

	"daunrodo/internal/config"
)

// ProxyState represents the current state of a proxy.
type ProxyState int

const (
	// ProxyStateAvailable indicates the proxy is available for use.
	ProxyStateAvailable ProxyState = iota
	// ProxyStateFailed indicates the proxy has failed and is in backoff.
	ProxyStateFailed
)

// Internal constants.
const (
	// healthCheckTimeout is the timeout for proxy health checks.
	healthCheckTimeout = 10 * time.Second
)

// proxyInfo holds information about a proxy.
type proxyInfo struct {
	URL           string
	State         ProxyState
	FailureCount  int
	LastFailure   time.Time
	BackoffUntil  time.Time
	LastHealthChk time.Time
}

// Manager manages proxy rotation and health.
type Manager struct {
	log *slog.Logger
	cfg *config.Config

	mu      sync.RWMutex
	proxies map[string]*proxyInfo
	order   []string // maintains insertion order for consistent iteration
}

// New creates a new proxy manager.
func New(log *slog.Logger, cfg *config.Config) *Manager {
	mgr := &Manager{
		log:     log.With(slog.String("package", "proxymgr")),
		cfg:     cfg,
		proxies: make(map[string]*proxyInfo),
		order:   make([]string, 0, len(cfg.Proxy.Proxies)),
	}

	for _, proxy := range cfg.Proxy.Proxies {
		mgr.proxies[proxy] = &proxyInfo{
			URL:   proxy,
			State: ProxyStateAvailable,
		}
		mgr.order = append(mgr.order, proxy)
	}

	return mgr
}

// GetRandomProxy returns a random available proxy URL.
// Returns empty string if no proxies are available.
func (m *Manager) GetRandomProxy() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	available := m.getAvailableProxies()
	if len(available) == 0 {
		return ""
	}

	return available[rand.IntN(len(available))]
}

// GetProxy returns a specific proxy if available.
func (m *Manager) GetProxy(proxyURL string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.proxies[proxyURL]
	if !exists {
		return "", false
	}

	if info.State == ProxyStateFailed && time.Now().Before(info.BackoffUntil) {
		return "", false
	}

	return proxyURL, true
}

// MarkFailed marks a proxy as failed and applies backoff.
func (m *Manager) MarkFailed(proxyURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.proxies[proxyURL]
	if !exists {
		return
	}

	info.FailureCount++
	info.LastFailure = time.Now()

	if info.FailureCount >= m.cfg.Proxy.MaxFailures {
		info.State = ProxyStateFailed
		// Exponential backoff
		backoff := m.cfg.Proxy.FailureBackoff * time.Duration(1<<(info.FailureCount-m.cfg.Proxy.MaxFailures))

		maxBackoff := 1 * time.Hour
		if backoff > maxBackoff {
			backoff = maxBackoff // cap at 1 hour
		}

		info.BackoffUntil = time.Now().Add(backoff)

		m.log.Warn("proxy marked as failed",
			slog.String("proxy", proxyURL),
			slog.Int("failure_count", info.FailureCount),
			slog.Duration("backoff", backoff))
	}
}

// MarkSuccess marks a proxy as successful and resets failure count.
func (m *Manager) MarkSuccess(proxyURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.proxies[proxyURL]
	if !exists {
		return
	}

	info.State = ProxyStateAvailable
	info.FailureCount = 0
	info.BackoffUntil = time.Time{}
}

// RestoreProxy manually restores a failed proxy.
func (m *Manager) RestoreProxy(proxyURL string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info, exists := m.proxies[proxyURL]
	if !exists {
		return
	}

	info.State = ProxyStateAvailable
	info.FailureCount = 0
	info.BackoffUntil = time.Time{}

	m.log.Info("proxy restored", slog.String("proxy", proxyURL))
}

// HealthCheck performs a health check on a specific proxy.
func (m *Manager) HealthCheck(ctx context.Context, proxyURL string) error {
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("parse proxy URL: %w", err)
	}

	// For SOCKS5 proxies, we just try to establish a connection
	dialer := &net.Dialer{
		Timeout: healthCheckTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", parsedURL.Host)
	if err != nil {
		m.MarkFailed(proxyURL)

		return fmt.Errorf("dial proxy: %w", err)
	}
	defer conn.Close()

	m.mu.Lock()

	if info, exists := m.proxies[proxyURL]; exists {
		info.LastHealthChk = time.Now()
	}

	m.mu.Unlock()

	m.MarkSuccess(proxyURL)

	return nil
}

// StartHealthChecker starts background health checking for all proxies.
func (m *Manager) StartHealthChecker(ctx context.Context) {
	if m.cfg.Proxy.HealthCheckInterval <= 0 || len(m.proxies) == 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(m.cfg.Proxy.HealthCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.checkAllProxies(ctx)
			}
		}
	}()

	m.log.Info("proxy health checker started",
		slog.Duration("interval", m.cfg.Proxy.HealthCheckInterval),
		slog.Int("proxy_count", len(m.proxies)))
}

// GetStats returns current proxy statistics.
func (m *Manager) GetStats() map[string]ProxyStats {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := make(map[string]ProxyStats, len(m.proxies))
	for proxyURL, info := range m.proxies {
		stats[proxyURL] = ProxyStats{
			State:         info.State,
			FailureCount:  info.FailureCount,
			LastFailure:   info.LastFailure,
			BackoffUntil:  info.BackoffUntil,
			LastHealthChk: info.LastHealthChk,
		}
	}

	return stats
}

// ProxyStats represents statistics for a proxy.
type ProxyStats struct {
	State         ProxyState
	FailureCount  int
	LastFailure   time.Time
	BackoffUntil  time.Time
	LastHealthChk time.Time
}

// HasProxies returns true if any proxies are configured.
func (m *Manager) HasProxies() bool {
	return len(m.proxies) > 0
}

// ProxyCount returns the total number of configured proxies.
func (m *Manager) ProxyCount() int {
	return len(m.proxies)
}

// AvailableCount returns the number of currently available proxies.
func (m *Manager) AvailableCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.getAvailableProxies())
}

func (m *Manager) getAvailableProxies() []string {
	now := time.Now()
	available := make([]string, 0, len(m.order))

	for _, proxyURL := range m.order {
		info := m.proxies[proxyURL]
		if info.State == ProxyStateAvailable {
			available = append(available, proxyURL)
		} else if info.State == ProxyStateFailed && now.After(info.BackoffUntil) {
			// Backoff expired, make available again
			available = append(available, proxyURL)
		}
	}

	return available
}

func (m *Manager) checkAllProxies(ctx context.Context) {
	m.mu.Lock()
	proxies := make([]string, len(m.order))
	copy(proxies, m.order)
	m.mu.Unlock()

	for _, proxy := range proxies {
		select {
		case <-ctx.Done():
			return
		default:
			if err := m.HealthCheck(ctx, proxy); err != nil {
				m.log.Debug("proxy health check failed",
					slog.String("proxy", proxy),
					slog.Any("error", err))
			}
		}
	}
}
