package proxymgr

import (
	"log/slog"
	"testing"
	"time"

	"daunrodo/internal/config"
)

// testProxyURL is the proxy URL used in tests.
const testProxyURL = "socks5h://localhost:1080"

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		proxies   []string
		wantCount int
		wantHas   bool
	}{
		{
			name:      "no proxies",
			proxies:   nil,
			wantCount: 0,
			wantHas:   false,
		},
		{
			name:      "single proxy",
			proxies:   []string{testProxyURL},
			wantCount: 1,
			wantHas:   true,
		},
		{
			name:      "multiple proxies",
			proxies:   []string{"socks5h://proxy1:1080", "socks5h://proxy2:1080"},
			wantCount: 2,
			wantHas:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			log := slog.Default()
			cfg := &config.Config{
				Proxy: config.Proxy{
					Proxies: tc.proxies,
				},
			}

			mgr := New(log, cfg)

			if got := mgr.ProxyCount(); got != tc.wantCount {
				t.Errorf("ProxyCount() = %d, want %d", got, tc.wantCount)
			}

			if got := mgr.HasProxies(); got != tc.wantHas {
				t.Errorf("HasProxies() = %v, want %v", got, tc.wantHas)
			}
		})
	}
}

func TestGetRandomProxy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		proxies   []string
		wantEmpty bool
	}{
		{
			name:      "no proxies returns empty",
			proxies:   nil,
			wantEmpty: true,
		},
		{
			name:      "single proxy returns that proxy",
			proxies:   []string{testProxyURL},
			wantEmpty: false,
		},
		{
			name:      "multiple proxies returns one of them",
			proxies:   []string{"socks5h://proxy1:1080", "socks5h://proxy2:1080"},
			wantEmpty: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			log := slog.Default()
			cfg := &config.Config{
				Proxy: config.Proxy{
					Proxies: tc.proxies,
				},
			}

			mgr := New(log, cfg)
			got := mgr.GetRandomProxy()

			if tc.wantEmpty && got != "" {
				t.Errorf("GetRandomProxy() = %q, want empty", got)
			}

			if !tc.wantEmpty && got == "" {
				t.Errorf("GetRandomProxy() = empty, want non-empty")
			}
		})
	}
}

func TestGetProxy(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{
		Proxy: config.Proxy{
			Proxies: []string{testProxyURL},
		},
	}

	mgr := New(log, cfg)

	proxy, exists := mgr.GetProxy(testProxyURL)
	if !exists {
		t.Error("GetProxy() returned false for existing proxy")
	}

	if proxy != testProxyURL {
		t.Errorf("GetProxy() = %q, want %q", proxy, testProxyURL)
	}

	proxy, exists = mgr.GetProxy("socks5h://nonexistent:1080")
	if exists {
		t.Error("GetProxy() returned true for non-existent proxy")
	}

	if proxy != "" {
		t.Errorf("GetProxy() = %q, want empty", proxy)
	}
}

func TestMarkFailed(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{
		Proxy: config.Proxy{
			Proxies:        []string{testProxyURL},
			MaxFailures:    3,
			FailureBackoff: 1 * time.Minute,
		},
	}

	mgr := New(log, cfg)
	proxy := testProxyURL

	for range 3 {
		mgr.MarkFailed(proxy)
	}

	stats := mgr.GetStats()
	if stats[proxy].State != ProxyStateFailed {
		t.Errorf("State = %v, want ProxyStateFailed", stats[proxy].State)
	}

	if stats[proxy].FailureCount != 3 {
		t.Errorf("FailureCount = %d, want 3", stats[proxy].FailureCount)
	}

	if mgr.AvailableCount() != 0 {
		t.Errorf("AvailableCount() = %d, want 0 during backoff", mgr.AvailableCount())
	}
}

func TestMarkSuccess(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{
		Proxy: config.Proxy{
			Proxies:        []string{testProxyURL},
			MaxFailures:    3,
			FailureBackoff: 1 * time.Minute,
		},
	}

	mgr := New(log, cfg)
	proxy := testProxyURL

	for range 3 {
		mgr.MarkFailed(proxy)
	}

	mgr.MarkSuccess(proxy)

	stats := mgr.GetStats()
	if stats[proxy].State != ProxyStateAvailable {
		t.Errorf("State = %v, want ProxyStateAvailable", stats[proxy].State)
	}

	if stats[proxy].FailureCount != 0 {
		t.Errorf("FailureCount = %d, want 0", stats[proxy].FailureCount)
	}
}

func TestRestoreProxy(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{
		Proxy: config.Proxy{
			Proxies:        []string{testProxyURL},
			MaxFailures:    3,
			FailureBackoff: 1 * time.Minute,
		},
	}

	mgr := New(log, cfg)
	proxy := testProxyURL

	for range 5 {
		mgr.MarkFailed(proxy)
	}

	mgr.RestoreProxy(proxy)

	stats := mgr.GetStats()
	if stats[proxy].State != ProxyStateAvailable {
		t.Errorf("State = %v, want ProxyStateAvailable", stats[proxy].State)
	}

	if mgr.AvailableCount() != 1 {
		t.Errorf("AvailableCount() = %d, want 1", mgr.AvailableCount())
	}
}

func TestBackoffExpiry(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{
		Proxy: config.Proxy{
			Proxies:        []string{testProxyURL},
			MaxFailures:    1,
			FailureBackoff: 100 * time.Millisecond, // Longer backoff to avoid flakiness
		},
	}

	mgr := New(log, cfg)
	proxy := testProxyURL

	mgr.MarkFailed(proxy)

	// Check immediately that proxy is unavailable
	if mgr.AvailableCount() != 0 {
		t.Errorf("AvailableCount() = %d, want 0", mgr.AvailableCount())
	}

	// Wait for backoff to expire
	time.Sleep(150 * time.Millisecond)

	if mgr.AvailableCount() != 1 {
		t.Errorf("AvailableCount() after backoff = %d, want 1", mgr.AvailableCount())
	}
}

func TestMarkFailedNonExistent(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{
		Proxy: config.Proxy{
			Proxies: []string{testProxyURL},
		},
	}

	mgr := New(log, cfg)
	mgr.MarkFailed("socks5h://nonexistent:1080")
}

func TestGetStats(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{
		Proxy: config.Proxy{
			Proxies:        []string{"socks5h://proxy1:1080", "socks5h://proxy2:1080"},
			MaxFailures:    3,
			FailureBackoff: 1 * time.Minute,
		},
	}

	mgr := New(log, cfg)

	stats := mgr.GetStats()

	if len(stats) != 2 {
		t.Errorf("len(stats) = %d, want 2", len(stats))
	}

	for proxy, stat := range stats {
		if stat.State != ProxyStateAvailable {
			t.Errorf("proxy %s: State = %v, want ProxyStateAvailable", proxy, stat.State)
		}

		if stat.FailureCount != 0 {
			t.Errorf("proxy %s: FailureCount = %d, want 0", proxy, stat.FailureCount)
		}
	}
}

func TestStartHealthChecker_NoProxies(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{
		Proxy: config.Proxy{
			Proxies:             nil,
			HealthCheckInterval: 1 * time.Second,
		},
	}

	mgr := New(log, cfg)

	mgr.StartHealthChecker(t.Context())
}

func TestStartHealthChecker_ZeroInterval(t *testing.T) {
	t.Parallel()

	log := slog.Default()
	cfg := &config.Config{
		Proxy: config.Proxy{
			Proxies:             []string{testProxyURL},
			HealthCheckInterval: 0,
		},
	}

	mgr := New(log, cfg)

	mgr.StartHealthChecker(t.Context())
}
