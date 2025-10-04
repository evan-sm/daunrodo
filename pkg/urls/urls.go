// Package urls provides utility functions for working with URLs.
package urls

import (
	"net/url"
	"strings"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

// IsURLValid checks if the given URL is valid.
func IsURLValid(raw string) bool {
	u, err := url.Parse(raw)

	return err == nil && u.Scheme != "" && u.Host != "" && (u.Scheme == schemeHTTP || u.Scheme == schemeHTTPS)
}

// FixURL prepends https scheme to URL.
// Example: instagram.com => https://instagram.com
func FixURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && (u.Scheme == "" || (u.Scheme != schemeHTTP && u.Scheme != schemeHTTPS)) {
		u.Scheme = schemeHTTPS

		return u.String()
	}

	return raw
}

// Normalize trims spaces, parses and returns the URL in string format.
func Normalize(raw string) string {
	raw = strings.TrimSpace(raw)

	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	return u.String()
}
