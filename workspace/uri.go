package workspace

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// URI represents a parsed URI reference.
type URI struct {
	uri  string
	name string
}

// String returns the full string representation of this URI.
// The result can be fed back into ParseURI to get back a URI.
func (u URI) String() string {
	return u.uri
}

// Name returns a short and non-unique representation of this URI.
func (u URI) Name() string {
	return u.name
}

// ParseURI parses s as a URI in the form of:
// [scheme:][//[userinfo@]host][/]path[?query][#fragment]
func ParseURI(s string) (URI, error) {
	if strings.HasPrefix(s, "file:///") {
		return parseFileURI(s)
	}
	return URI{}, fmt.Errorf("unknown scheme: %s", s)
}

func parseFileURI(s string) (URI, error) {
	path := sanitizeFilePath(s[len("file://"):])
	u := url.URL{Scheme: fileScheme, Path: path}
	return URI{uri: u.String(), name: filepath.Base(path)}, nil
}
