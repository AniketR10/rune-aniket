package workspace

import (
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/ernestrc/go-tui/util"
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

func sanitizeFilePath(resource string) string {
	resolvedPath, err := filepath.EvalSymlinks(resource)
	if err != nil {
		resolvedPath = filepath.Clean(resource)
	}
	return util.SanitizeLine(resolvedPath)
}

func makeSSHURI(u *url.URL) URI {
	name := fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, u.Path)
	return URI{uri: u.String(), parsed: *u, name: name}
}

func makeFileURI(u *url.URL) URI {
	path := sanitizeFilePath(u.Path)
	u.Path = path
	return URI{uri: u.String(), parsed: *u, name: filepath.Base(path)}
}

func makeURI(u *url.URL) (URI, error) {
	if u.Scheme == fileScheme {
		return makeFileURI(u), nil
	}
	if u.Scheme == sshScheme {
		return makeSSHURI(u), nil
	}
	return URI{}, fmt.Errorf("unsupported scheme: %s", u.Scheme)
}

func checkURIRelative(a, b URI) error {
	if a.parsed.Scheme != b.parsed.Scheme {
		return fmt.Errorf("unexpected different schemes: %s vs %s",
			a.String(), b.String())
	}
	if a.parsed.Scheme == fileScheme {
		return nil
	}
	if a.parsed.Host != b.parsed.Host {
		return fmt.Errorf("unexpected different hosts: %s vs %s",
			a.String(), b.String())
	}
	if a.parsed.User.String() != b.parsed.User.String() {
		return fmt.Errorf("unexpected different users: %s vs %s",
			a.String(), b.String())
	}
	return nil
}

// ParseURI parses s as a URI in the form of:
// [scheme:][//[userinfo@]host][/]path[?query][#fragment]
func ParseURI(s string) (URI, error) {
	u, err := url.Parse(s)
	if err != nil {
		return URI{}, fmt.Errorf("failed to parse URI: %s", err)
	}
	return makeURI(u)
}

// DefaultSwapFile returns a file's default swap directory in the
// local or remote workspace.
func DefaultSwapFile(swapDir URI, file URI) (URI, error) {
	err := checkURIRelative(swapDir, file)
	if err != nil {
		return URI{}, err
	}
	_, swapFilePath := swapFileName(swapDir.parsed.Path, file.parsed.Path)
	file.parsed.Path = swapFilePath
	return makeURI(&file.parsed)
}

// DefaultSwapDirectory returns a file's default swap directory in the
// local or remote workspace.
func DefaultSwapDirectory(file URI) (URI, error) {
	swapDir, _ := swapFileName(filepath.Dir(file.parsed.Path), file.parsed.Path)
	file.parsed.Path = swapDir
	return makeURI(&file.parsed)
}
