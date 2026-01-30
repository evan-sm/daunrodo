// Package shellquote provides utilities for constructing shell-quoted command strings.
package shellquote

import (
	"strings"
)

// // Join constructs a shell-quoted command string from the binary and its arguments.
// func Join(bin string, args []string) string {
// 	parts := make([]string, 0, 1+len(args))

// 	parts = append(parts, strconv.Quote(bin))
// 	for _, a := range args {
// 		parts = append(parts, strconv.Quote(a))
// 	}
// 	// Quote() returns a double-quoted, shell-friendly string.
// 	// Paste into bash/zsh as-is.
// 	return strings.Join(parts, " ")
// }

// shellEscapeDQ returns a bash/zsh-safe argument using double quotes when needed.
// In double quotes, these must be escaped: \ " $ `.
func shellEscapeDQ(s string) string { //nolint:varnamelen
	if s == "" {
		return `""`
	}

	// "Simple" chars safe to keep unquoted.
	const safe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_@%+=:,./-"

	needsQuotes := false

	for _, r := range s {
		if !strings.ContainsRune(safe, r) {
			needsQuotes = true

			break
		}
	}

	if !needsQuotes {
		return s
	}

	var b strings.Builder //nolint:varnamelen
	b.WriteByte('"')

	for _, r := range s { //nolint:varnamelen
		switch r {
		case '\\', '"', '$', '`':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			// Newlines are rare in CLI args; keep it pasteable.
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}

	b.WriteByte('"')

	return b.String()
}

// Join constructs a shell-pasteable command line from bin and args.
func Join(bin string, args []string) string {
	var cmdLine strings.Builder

	cmdLine.WriteString(shellEscapeDQ(bin))

	for _, arg := range args {
		cmdLine.WriteByte(' ')
		cmdLine.WriteString(shellEscapeDQ(arg))
	}

	return cmdLine.String()
}
