package workspace

import (
	"fmt"
	"net/url"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"unstable.build/go-tui/util"
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

// Scheme returns the scheme of this URI.
func (u URI) Scheme() string {
	return u.parsed.Scheme
}

// Path relative or absolute path component of this URI.
func (u URI) Path() string {
	return u.parsed.Path
}

// Hostname returns the hostname of this URI or an empty string
// if hostname is not specified.
func (u URI) Hostname() string {
	return u.parsed.Hostname()
}

// Host returns the host of this URI or an empty string
// if host is not specified.
func (u URI) Host() string {
	return u.parsed.Host
}

// Port returns the port of this URI or an empty string
// if port is not specified.
func (u URI) Port() string {
	return u.parsed.Port()
}

// User returns the user of this URI or an empty string
// if a user is not specified.
func (u URI) User() string {
	if u.parsed.User != nil {
		return u.parsed.User.Username()
	}
	return ""
}

// Password returns the password of this URI or an empty string
// if a password is not specified.
func (u URI) Password() (string, bool) {
	if u.parsed.User != nil {
		return u.parsed.User.Password()
	}
	return "", false
}

// Equal returns true if this uri is equal to other.
func (u URI) Equal(other URI) bool {
	return u.Scheme() == other.Scheme() &&
		u.Hostname() == other.Hostname() &&
		u.Port() == other.Port() &&
		u.User() == other.User() &&
		u.Path() == other.Path()
}

func sanitizeFilePath(resource string) string {
	resolvedPath, err := filepath.EvalSymlinks(resource)
	if err != nil {
		resolvedPath = filepath.Clean(resource)
	}
	return util.SanitizeLine(resolvedPath)
}

func makeRemoteURI(u *url.URL) URI {
	if u.Path == "" {
		u.Path = "/"
	}
	name := fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, path.Join("/", u.Path))
	return URI{uri: u.String(), parsed: *u, name: name}
}

func makeFileURI(u *url.URL) (URI, error) {
	path := sanitizeFilePath(u.Path)
	u.Path = path
	if u.Scheme == "" {
		return URI{}, fmt.Errorf("invalid empty scheme %#v", u)
	}
	return URI{uri: u.String(), parsed: *u, name: filepath.Base(path)}, nil
}

func uriFromURL(u *url.URL) (URI, error) {
	if u.Host != "" {
		return makeRemoteURI(u), nil
	}
	return makeFileURI(u)
}

func checkURIRelative(a, b URI) error {
	if a.parsed.Scheme != b.parsed.Scheme {
		return fmt.Errorf("unexpected different schemes: %s vs %s",
			a.String(), b.String())
	}
	if a.parsed.Scheme == FileScheme {
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
	return uriFromURL(u)
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
	return uriFromURL(&file.parsed)
}

// DefaultSwapDirectory returns a file's default swap directory in the
// local or remote workspace.
func DefaultSwapDirectory(file URI) (URI, error) {
	swapDir, _ := swapFileName(filepath.Dir(file.parsed.Path), file.parsed.Path)
	file.parsed.Path = swapDir
	return uriFromURL(&file.parsed)
}

// Join joins any number of path elements into this URI's path, separating them
// with slashes. For more details see path.Join.
func Join(uri URI, elem ...string) URI {
	pathElems := make([]string, 0, len(elem)+1)
	pathElems = append(pathElems, uri.Path())
	pathElems = append(pathElems, elem...)
	newPath := path.Join(pathElems...)
	uri.parsed.Path = newPath
	ret, err := ParseURI(uri.parsed.String())
	if err != nil {
		panic("failed to parse internally generated URI")
	}
	return ret
}

// Dir returns all but the last element of this URI's path.
func Dir(uri URI) URI {
	uri.parsed.Path = filepath.Dir(uri.parsed.Path)
	ret, err := ParseURI(uri.parsed.String())
	if err != nil {
		panic("failed to parse internally generated URI")
	}
	return ret
}

// ExpandPathWithURI finds the absolute path of a relative path.
// It uses the given URI to resolve user and cwd so ~ is an alias
// for a path relative to the base of the URI.
// See ExpandPath for more details.
func ExpandPathWithURI(
	path string, uri URI,
) (string, error) {
	return ExpandPath(path, func() (*user.User, error) {
		return &user.User{
			Username: uri.User(),
			HomeDir:  uri.Path(),
		}, nil
	}, func() (string, error) {
		return uri.Path(), nil
	})
}

// ExpandPath finds the absolute path of a relative path and
// expands the home shortcut (~) if any. If path is already
// absolute then this function returns the path unchanged.
func ExpandPath(
	path string, getUser func() (*user.User, error),
	cwdFn func() (string, error),
) (string, error) {
	if path == "~" || path == "/~" {
		usr, err := getUser()
		if err != nil {
			return "", fmt.Errorf("could not get current user: %s", err)
		}
		path = usr.HomeDir
	} else if strings.HasPrefix(path, "~/") {
		usr, err := getUser()
		if err != nil {
			return "", fmt.Errorf("could not get current user: %s", err)
		}
		path = filepath.Join(usr.HomeDir, path[2:])
	} else if strings.HasPrefix(path, "/~/") {
		usr, err := getUser()
		if err != nil {
			return "", fmt.Errorf("could not get current user: %s", err)
		}
		path = filepath.Join(usr.HomeDir, path[3:])
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	cwd, err := cwdFn()
	if err != nil {
		return "", fmt.Errorf("could not get cwd: %s", err)
	}
	abs := filepath.Join(cwd, path)
	return abs, nil
}

// HasPrefix returns tests whether uri begins with prefix.
func HasPrefix(uri, prefix URI) bool {
	return uri.Scheme() == prefix.Scheme() &&
		uri.Hostname() == prefix.Hostname() &&
		uri.Port() == prefix.Port() &&
		uri.User() == prefix.User() &&
		strings.HasPrefix(uri.Path(), prefix.Path())
}

// RelPath returns target's path as a relative path of base,
// or if that's not possible, it will return target's Path
// without change.
func RelPath(base, target URI) string {
	if HasPrefix(target, base) {
		relpath, err := filepath.Rel(base.Path(), target.Path())
		if err == nil {
			return relpath
		}
	}
	return target.Path()
}

// IsWorkspaceURI returns whether this uri can be managed by
// the given workspace. For Scheme implementations that
// do not support user and host/port, this method returns
// true if the URI schemes of the workspace and the supplied uri
// are the same.
func IsWorkspaceURI(workspace Workspace, uri URI) (bool, error) {
	uriAtWorkspace, err := workspace.URI(uri.Path())
	if err != nil {
		return false, err
	}
	return uri.Scheme() == uriAtWorkspace.Scheme() &&
		uri.Hostname() == uriAtWorkspace.Hostname() &&
		uri.Port() == uriAtWorkspace.Port() &&
		uri.User() == uriAtWorkspace.User(), nil
}

// WorkspaceURI expands the given path with the given workspace URI
// and returns its corresponding URI. See ExpandPathWithURI for more details.
func WorkspaceURI(workspace URI, path string) (URI, error) {
	absPath, err := ExpandPathWithURI(path, workspace)
	if err != nil {
		return URI{}, err
	}

	var uriStr string
	if workspace.User() != "" {
		uriStr = fmt.Sprintf("%s://%s@%s%s", workspace.Scheme(),
			workspace.User(), workspace.Host(), absPath)
	} else {
		uriStr = fmt.Sprintf("%s://%s%s", workspace.Scheme(), workspace.Host(), absPath)
	}
	return ParseURI(uriStr)
}
