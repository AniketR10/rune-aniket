package workspace

import (
	"fmt"
	os "os"

	"github.com/ernestrc/go-tui/cell"
)

// Manager manages resources on a workspace. It satisfies ResourceOpener.
type Manager struct {
	workspace URI

	osChdir func(string) error
	osGetwd func() (string, error)
}

// NewManager allocates storage fore a new Manage and initializes it with workspace.
func NewManager(workspace URI) (*Manager, error) {
	ret := new(Manager)
	err := ret.Init(workspace)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Init initializes m with workspace and calles os.Chdir to the new workspace
// if current working directory is not already equal to the given workspace.
func (m *Manager) Init(workspace URI) error {
	m.osChdir = os.Chdir
	m.osGetwd = os.Getwd
	return m.init(workspace)
}

func (m *Manager) init(workspace URI) error {
	m.workspace = workspace
	cwd, err := m.osGetwd()
	if err != nil {
		return fmt.Errorf("Failed to get working directory: %s", err)
	}
	absCwd, err := extractAbsPath(cwd)
	if err != nil {
		return err
	}
	workspacewd, err := LocalPath(workspace)
	if err != nil {
		return err
	}
	absWorkspacewd, err := extractAbsPath(workspacewd)
	if err != nil {
		return err
	}
	if absCwd != absWorkspacewd {
		err = m.osChdir(absWorkspacewd)
		if err != nil {
			return fmt.Errorf("failed to change to workspace directory %s: %s",
				absWorkspacewd, err)
		}
	}
	return nil
}

// Recover recovers the file with the swap file.
func (m *Manager) Recover(file, swapFile URI, buf *cell.Buffer) (
	FlusherCloser, error,
) {
	return recoverLocalFile(file, swapFile, buf)
}

// Open opens the file at the given URI and initializes buf with the contents of it.
// It uses swapDir as the file recovery and swap directory.
func (m *Manager) Open(
	file URI, buf *cell.Buffer, swapDir URI, readOnly bool,
) (
	FlusherCloser, error,
) {
	return openLocalFile(file, buf, swapDir, readOnly)
}
