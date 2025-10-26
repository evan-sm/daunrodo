package gen_test

import (
	"crypto/sha1"
	"daunrodo/pkg/gen"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
)

var namespaceURL = mustParseUUID("6ba7b811-9dad-11d1-80b4-00c04fd430c8")

func TestKey(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{name: "basic", a: "foo", b: "bar", want: "foo|bar"},
		{name: "emptyA", a: "", b: "value", want: "|value"},
		{name: "emptyB", a: "value", b: "", want: "value|"},
		{name: "bothEmpty", a: "", b: "", want: "|"},
		{name: "inputContainsSeparator", a: "foo|bar", b: "baz", want: "foo|bar|baz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gen.Key(tt.a, tt.b); got != tt.want {
				t.Fatalf("Key(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestUUIDv5(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
	}{
		{name: "basic", a: "foo", b: "bar"},
		{name: "emptyInputs", a: "", b: ""},
		{name: "inputContainsSeparator", a: "foo|bar", b: "baz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := expectedUUIDv5(tt.a, tt.b)
			if got := gen.UUIDv5(tt.a, tt.b); got != want {
				t.Fatalf("UUIDv5(%q, %q) = %q, want %q", tt.a, tt.b, got, want)
			}

			if got := gen.UUIDv5(tt.a, tt.b); got != want {
				t.Fatalf("UUIDv5 repeated call mismatch: %q vs %q", got, want)
			}
		})
	}
}

func expectedUUIDv5(a, b string) string {
	key := fmt.Sprintf("%s|%s", a, b)

	hash := sha1.New()
	_, _ = hash.Write(namespaceURL)
	_, _ = hash.Write([]byte(key))

	sum := hash.Sum(nil)
	uuidBytes := make([]byte, 16)
	copy(uuidBytes, sum)

	uuidBytes[6] = (uuidBytes[6] & 0x0f) | 0x50
	uuidBytes[8] = (uuidBytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", uuidBytes[0:4], uuidBytes[4:6], uuidBytes[6:8], uuidBytes[8:10], uuidBytes[10:16])
}

func mustParseUUID(uuid string) []byte {
	cleaned := strings.ReplaceAll(uuid, "-", "")

	decoded, err := hex.DecodeString(cleaned)
	if err != nil {
		panic(fmt.Sprintf("invalid UUID %q: %v", uuid, err))
	}

	if len(decoded) != 16 {
		panic(fmt.Sprintf("invalid UUID length for %q: %d", uuid, len(decoded)))
	}

	return decoded
}
