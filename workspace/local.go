package workspace

import (
	"net/url"
	"os"
	"os/user"

	"github.com/ernestrc/go-tui/cell"
)

// CurrentUserHostURI builds a URI from a path. If path is relative
// it uses the current working directory as the base of the path and
// if ~ is used to identify the home directory, the current user's home
// directory is used as the base.
// This should only used instead of Manager.URI before a workspace.Manager is
// constructed or for other advanced uses cases.
func CurrentUserHostURI(path string) (URI, error) {
	absPath, err := extractAbsPath(path, user.Current, os.Getwd)
	if err != nil {
		return URI{}, err
	}
	return makeLocalURI(absPath)
}

func makeLocalURI(path string) (URI, error) {
	uriStr := "file://" + path
	u, err := url.Parse(uriStr)
	if err != nil {
		return URI{}, err
	}

	return makeFileURI(u), nil
}

func (m *Manager) localURI(path string) (URI, error) {
	absPath, err := m.extractAbsPath(path)
	if err != nil {
		return URI{}, err
	}
	return makeLocalURI(absPath)
}

func newOsLocalFile() *file {
	ret := new(file)
	ret.openFunc = osOpenFileFunc()
	ret.removeFunc = os.Remove
	ret.renameFunc = os.Rename
	ret.statFunc = os.Stat
	ret.lstatFunc = os.Lstat
	ret.readLinkFunc = os.Readlink
	return ret
}

// recoverfile recovers the file at filePath with the swap file swapFilePath.
func recoverLocalFile(file, swapFile URI, buf *cell.Buffer) (
	FlusherCloser, error,
) {
	ret := newOsLocalFile()
	err := ret.recoverFile(file.Path(), swapFile.Path(), buf)
	if err != nil {
		return nil, err
	}
	return ret, err
}

// openLocalFile allocates store for a new file and then calls Init.
func openLocalFile(file URI, buf *cell.Buffer, swapDir URI, readOnly bool) (
	FlusherCloser, error,
) {
	ret := newOsLocalFile()
	err := ret.init(file.Path(), buf, swapDir.Path(), readOnly)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
