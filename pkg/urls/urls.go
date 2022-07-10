package urls

import (
	"net/url"
	"strings"
)

func IsURLValid(raw string) bool {
	u, err := url.Parse(raw)

	return err == nil && u.Scheme != "" && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https")
}

// FixURL prepends https scheme to URL.
// Example: instagram.com => https://instagram.com
func FixURL(raw string) string {
	u, err := url.Parse(raw)
	if err == nil && (u.Scheme == "" || (u.Scheme != "http" && u.Scheme != "https")) {
		u.Scheme = "https"

		return u.String()
	}

	return raw
}

func Normalize(raw string) string {
	raw = strings.TrimSpace(raw)

	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	return u.String()
}
