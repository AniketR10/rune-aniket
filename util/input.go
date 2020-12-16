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

// SanitizeInputFilename escapes tained user input and validates
// a file name.
func SanitizeFilename(filename string) string {
	resolvedPath, err := filepath.EvalSymlinks(filename)
	if err != nil {
		resolvedPath = filepath.Clean(filename)
	}
	return SanitizeLine(resolvedPath)
}
