package workspace

import (
	"fmt"
	"path/filepath"

	workspaceapi "unstable.build/go-tui/api/workspace"
)

func checkURIRelative(a, b workspaceapi.URI) error {
	if a.Scheme() != b.Scheme() {
		return fmt.Errorf("unexpected different schemes: %s vs %s",
			a.String(), b.String())
	}
	if a.Scheme() == FileScheme {
		return nil
	}
	if a.Host() != b.Host() {
		return fmt.Errorf("unexpected different hosts: %s vs %s",
			a.String(), b.String())
	}
	if a.User() != b.User() {
		return fmt.Errorf("unexpected different users: %s vs %s",
			a.String(), b.String())
	}
	return nil
}

// DefaultSwapFile returns a file's default swap directory in the
// local or remote workspace.
func DefaultSwapFile(swapDir workspaceapi.URI, file workspaceapi.URI) (workspaceapi.URI, error) {
	err := checkURIRelative(swapDir, file)
	if err != nil {
		return workspaceapi.URI{}, err
	}
	_, swapFilePath := swapFileName(swapDir.Path(), file.Path())
	return workspaceapi.WithPath(file, swapFilePath)
}

// DefaultSwapDirectory returns a file's default swap directory in the
// local or remote workspace.
func DefaultSwapDirectory(file workspaceapi.URI) (workspaceapi.URI, error) {
	swapDir, _ := swapFileName(filepath.Dir(file.Path()), file.Path())
	return workspaceapi.WithPath(file, swapDir)
}

// IsWorkspaceURI returns whether this uri can be managed by
// the given workspace. For Scheme implementations that
// do not support user and host/port, this method returns
// true if the URI schemes of the workspace and the supplied uri
// are the same.
func IsWorkspaceURI(workspace Workspace, uri workspaceapi.URI) (bool, error) {
	uriAtWorkspace, err := workspace.URI(uri.Path())
	if err != nil {
		return false, err
	}
	return uri.Scheme() == uriAtWorkspace.Scheme() &&
		uri.Hostname() == uriAtWorkspace.Hostname() &&
		uri.Port() == uriAtWorkspace.Port() &&
		uri.User() == uriAtWorkspace.User(), nil
}

// WorkspaceURI expands the given path with the given workspaceapi.URI
// and returns its corresponding URI. See ExpandPathWithURI for more details.
func WorkspaceURI(workspace workspaceapi.URI, path string) (workspaceapi.URI, error) {
	absPath, err := workspaceapi.ExpandPathWithURI(path, workspace)
	if err != nil {
		return workspaceapi.URI{}, err
	}

	var uriStr string
	if workspace.User() != "" {
		uriStr = fmt.Sprintf("%s://%s@%s%s", workspace.Scheme(),
			workspace.User(), workspace.Host(), absPath)
	} else {
		uriStr = fmt.Sprintf("%s://%s%s", workspace.Scheme(), workspace.Host(), absPath)
	}
	return workspaceapi.ParseURI(uriStr)
}
