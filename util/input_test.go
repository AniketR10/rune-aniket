package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeFilename(t *testing.T) {
	tsuite := []struct {
		in  string
		out string
	}{
		{"", "."},
		{"file.sql", "file.sql"},
		{"/file.sql", "/file.sql"},
		{"w\x00ps", "wps"},
		{"RE\nADME.md\n", "README.md"},
	}

	for _, tcase := range tsuite {
		out := SanitizeResourceName(tcase.in)
		assert.Equal(t, tcase.out, out)
	}
}
