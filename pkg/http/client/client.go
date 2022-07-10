package httpclient

import (
	"log"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	Client *http.Client
}

func New(proxy string) *Client {
	httpClient := &http.Client{
		Timeout:   60 * time.Second,
		Transport: http.DefaultTransport,
	}

	// set proxy?
	if proxy != "" {
		setProxy(httpClient, proxy)
	}

	c := &Client{
		Client: httpClient,
	}

	return c
}

func setProxy(httpClient *http.Client, proxy string) {
	u, err := url.Parse(proxy)
	if err != nil {
		log.Printf("WARNING: failed to set proxy: %v", err)
	}

	httpClient.Transport = &http.Transport{Proxy: http.ProxyURL(u)}

	go testProxy(httpClient)
}

// testProxy tests if proxy is working
func testProxy(httpClient *http.Client) {
	_, err := httpClient.Head("https://example.org")
	if err != nil {
		log.Printf("WARNING: proxy test failed: %v", err)

		return
	}

	log.Printf("Using proxy. Test succeeded.")
}
