package workspace

import (
	"fmt"
	"net/url"
	"path/filepath"
)

// URI represents a parsed URI reference.
type URI struct {
	uri    string
	parsed url.URL
	name   string
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

// Path relative or absolute path component of this URI.
func (u URI) Path() string {
	return u.parsed.Path
}

// ParseURI parses s as a URI in the form of:
// [scheme:][//[userinfo@]host][/]path[?query][#fragment]
func ParseURI(s string) (URI, error) {
	u, err := url.Parse(s)
	if err != nil {
		return URI{}, fmt.Errorf("failed to parse URI: %s", err)
	}
	if u.Scheme == "file" {
		return makeFileURI(u)
	}
	return URI{}, fmt.Errorf("unsupported scheme: %s", s)
}

func makeFileURI(u *url.URL) (URI, error) {
	path := sanitizeFilePath(u.Path)
	return URI{uri: u.String(), parsed: *u, name: filepath.Base(path)}, nil
}
