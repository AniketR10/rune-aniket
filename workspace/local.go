package workspace

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ernestrc/go-tui/cell"
)

// LocalPath attempts to return a unix path in the local system
// or returns an error if URI could not be mapped to path.
// This function returns an error if URI is empty.
func LocalPath(u URI) (string, error) {
	if u == (URI{}) {
		return "", errors.New("empty URI")
	}
	return localPath(u)
}

// LocalURI returns a URI that references the file at local path.
func LocalURI(path string) (URI, error) {
	absPath, err := extractAbsPath(path)
	if err != nil {
		return URI{}, err
	}
	uriStr := "file://" + absPath
	u, err := url.Parse(uriStr)
	if err != nil {
		return URI{}, err
	}

	return URI{
		uri:    uriStr,
		name:   filepath.Base(absPath),
		parsed: *u,
	}, nil
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

func extractAbsPath(filename string) (string, error) {
	if filename == "~" {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("could not get current user: %s", err)
		}
		filename = usr.HomeDir
	} else if strings.HasPrefix(filename, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("could not get current user: %s", err)
		}
		filename = filepath.Join(usr.HomeDir, filename[2:])
	}
	abs, err := filepath.Abs(filename)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path of %s: %s", filename, err)
	}
	return abs, nil
}

func localPath(u URI) (string, error) {
	url, err := url.ParseRequestURI(u.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse URI: %s: %s", u.String(), err)
	}
	if url.Scheme != fileScheme {
		return "", fmt.Errorf("non supported scheme: %s", url.Scheme)
	}
	return url.Path, nil
}

// recoverfile recovers the file at filePath with the swap file swapFilePath.
func recoverLocalFile(file, swapFile URI, buf *cell.Buffer) (
	FlusherCloser, error,
) {
	ret := newOsLocalFile()
	filePath, err := localPath(file)
	if err != nil {
		return nil, err
	}
	swapFilePath, err := localPath(swapFile)
	if err != nil {
		return nil, err
	}

	err = ret.recoverFile(filePath, swapFilePath, buf)
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
	filePath, err := localPath(file)
	if err != nil {
		return nil, err
	}
	swapDirPath, err := localPath(swapDir)
	if err != nil {
		return nil, err
	}

	err = ret.init(filePath, buf, swapDirPath, readOnly)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
