package util

import (
	"strings"
)

// SanitizeLine escapes tainted user input.
func SanitizeLine(in string) string {
	var b strings.Builder
	for _, r := range []rune(in) {
		switch r {
		case '\x00':
		case '\n':
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
