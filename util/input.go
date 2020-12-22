package util

import (
	"path/filepath"
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

// SanitizeResourceName escapes tained user input and validates
// a file name.
func SanitizeResourceName(resource string) string {
	resolvedPath, err := filepath.EvalSymlinks(resource)
	if err != nil {
		resolvedPath = filepath.Clean(resource)
	}
	return SanitizeLine(resolvedPath)
}
