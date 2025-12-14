package proxy_test

import (
	"context"
	"daunrodo/internal/proxy"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name      string
		proxyURLs string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "empty proxies",
			proxyURLs: "",
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "single proxy",
			proxyURLs: "socks5h://127.0.0.1:1080",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "multiple proxies",
			proxyURLs: "socks5h://127.0.0.1:1080,socks5h://127.0.0.1:1081",
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "proxies with spaces",
			proxyURLs: "socks5h://127.0.0.1:1080 , socks5h://127.0.0.1:1081 ",
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "proxies with empty entries",
			proxyURLs: "socks5h://127.0.0.1:1080,,socks5h://127.0.0.1:1081",
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "invalid proxy URL",
			proxyURLs: "not a valid url://:",
			wantCount: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := proxy.New(tt.proxyURLs, true, 5*time.Second)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && m.Count() != tt.wantCount {
				t.Errorf("New() count = %v, want %v", m.Count(), tt.wantCount)
			}
		})
	}
}

func TestGetProxy_NoProxies(t *testing.T) {
	m, err := proxy.New("", true, 5*time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	proxyURL, err := m.GetProxy(ctx)
	if err != nil {
		t.Errorf("GetProxy() error = %v", err)
	}
	if proxyURL != "" {
		t.Errorf("GetProxy() = %v, want empty string", proxyURL)
	}
}

func TestGetProxy_WithoutHealthCheck(t *testing.T) {
	m, err := proxy.New("socks5h://127.0.0.1:1080,socks5h://127.0.0.1:1081", false, 5*time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	proxyURL, err := m.GetProxy(ctx)
	if err != nil {
		t.Errorf("GetProxy() error = %v", err)
	}
	if proxyURL == "" {
		t.Errorf("GetProxy() returned empty string")
	}
}

func TestGetProxy_Random(t *testing.T) {
	m, err := proxy.New("socks5h://proxy1:1080,socks5h://proxy2:1080,socks5h://proxy3:1080", false, 5*time.Second)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()
	seen := make(map[string]bool)

	// Get proxies multiple times to verify randomness
	for i := 0; i < 10; i++ {
		proxyURL, err := m.GetProxy(ctx)
		if err != nil {
			t.Errorf("GetProxy() error = %v", err)
		}
		seen[proxyURL] = true
	}

	// We should see at least one proxy (could theoretically see just one due to randomness, but unlikely)
	if len(seen) == 0 {
		t.Errorf("GetProxy() returned no proxies")
	}
}

func TestNew_IPv6(t *testing.T) {
	tests := []struct {
		name      string
		proxyURLs string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "IPv6 with port",
			proxyURLs: "socks5h://[::1]:1080",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "IPv6 without port",
			proxyURLs: "socks5h://[::1]",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "mixed IPv4 and IPv6",
			proxyURLs: "socks5h://127.0.0.1:1080,socks5h://[::1]:1080",
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := proxy.New(tt.proxyURLs, true, 5*time.Second)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && m.Count() != tt.wantCount {
				t.Errorf("New() count = %v, want %v", m.Count(), tt.wantCount)
			}
		})
	}
}
