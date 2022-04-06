package workspace

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseURI(t *testing.T) {
	tsuite := []struct {
		in      string
		wantOut URI
		wantErr bool
	}{
		{"file:///", URI{
			uri: "file:///", name: "/", parsed: url.URL{Scheme: "file", Path: "/"},
		}, false},
		{"file:///tmp/a", URI{
			uri: "file:///tmp/a", name: "a", parsed: url.URL{Scheme: "file", Path: "/tmp/a"},
		}, false},
		{"ssh://unstable.build/tmp/a", URI{
			uri:    "ssh://unstable.build/tmp/a",
			name:   "ssh://unstable.build/tmp/a",
			parsed: url.URL{Scheme: "ssh", Host: "unstable.build", Path: "/tmp/a"},
		}, false},
		{"ssh://potato:farmer@unstable.build/tmp/a", URI{
			uri:  "ssh://potato:farmer@unstable.build/tmp/a",
			name: "ssh://unstable.build/tmp/a",
			parsed: url.URL{
				Scheme: "ssh",
				Host:   "unstable.build",
				Path:   "/tmp/a",
				User:   url.UserPassword("potato", "farmer"),
			},
		}, false},
		{"/tmp/a", URI{}, true},
		{"f$le://", URI{}, true},
	}

	for _, tcase := range tsuite {
		t.Run(fmt.Sprintf("ParseURI(%s)", tcase.in), func(t *testing.T) {
			actualOut, actualErr := ParseURI(tcase.in)
			if tcase.wantErr {
				require.Error(t, actualErr)
			} else {
				require.NoError(t, actualErr)
			}
			assert.Equal(t, tcase.wantOut, actualOut)
		})
	}
}

func TestDefaultSwapDirectory(t *testing.T) {
	tsuite := []struct {
		file    string
		wantDir string
		wantErr bool
	}{
		{"other:///tmp/a.go", "", true},
		{"file:///a.go", "file:///", false},
		{"file:///tmp/a.go", "file:///tmp", false},
		{"file://tmp/a.go", "file://tmp/", false},
		// TODO add ssh:// tests
	}

	for _, tcase := range tsuite {
		t.Run(fmt.Sprintf("DefaultLocalSwapDirectory of %s", tcase.file), func(t *testing.T) {
			u, err := url.Parse(tcase.file)
			require.NoError(t, err)
			uri := URI{uri: tcase.file, parsed: *u}
			out, err := DefaultSwapDirectory(uri)
			if tcase.wantErr {
				assert.Error(t, err)
			} else {
				assert.Equal(t, tcase.wantDir, out.String())
			}
		})
	}
}

func TestDefaultSwapFile(t *testing.T) {
	tsuite := []struct {
		fileIn       string
		swapDirIn    string
		wantSwapFile string
		wantErr      bool
	}{
		{"other:///tmp/a.go", "other:///tmp", "", true},
		{"file:///a.go", "file:///tmp", "file:///tmp/.a.go.swp", false},
		{"file:///a.go", "file:///", "file:///.a.go.swp", false},
		{"file:///tmp/a.go", "file:///tmp", "file:///tmp/.a.go.swp", false},
		{"file:///tmp/a.go", "file:///", "file:///.a.go.swp", false},
		{"file://./tmp/a.go", "file://./", "file://./.a.go.swp", false},
		{"file://./a.go", "file://./tmp", "file://./tmp/.a.go.swp", false},
		{"ssh:///a.go", "ssh://my_host/tmp", "", true},
		{"ssh://my_host/a.go", "ssh:///tmp", "", true},
		{"ssh://my_host/a.go", "ssh://creepy_host/tmp", "", true},
		{"ssh://ernestrc@my_host/a.go", "ssh://jj.furman@my_host/tmp", "", true},
		{"ssh://user@my_host/a.go", "ssh://user@my_host/tmp", "ssh://user@my_host/tmp/.a.go.swp", false},
		{"ssh://my_host/a.go", "ssh://my_host/tmp", "ssh://my_host/tmp/.a.go.swp", false},
		{"ssh://my_host/./a.go", "ssh://my_host/./tmp", "ssh://my_host/tmp/.a.go.swp", false},
	}

	for i, tcase := range tsuite {
		desc := fmt.Sprintf("DefaultLocalSwapFile %d of %s", i, tcase.fileIn)
		t.Run(desc, func(t *testing.T) {
			u, err := url.Parse(tcase.fileIn)
			require.NoError(t, err)
			uDir, err := url.Parse(tcase.swapDirIn)
			require.NoError(t, err)
			uri := URI{uri: tcase.fileIn, parsed: *u}
			swapUri := URI{uri: tcase.swapDirIn, parsed: *uDir}

			// sut
			out, err := DefaultSwapFile(swapUri, uri)

			if tcase.wantErr {
				assert.Error(t, err)
			} else {
				assert.Equal(t, tcase.wantSwapFile, out.String())
			}
		})
	}
}

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
		out := sanitizeFilePath(tcase.in)
		assert.Equal(t, tcase.out, out)
	}
}
