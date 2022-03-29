package workspace

import "github.com/ernestrc/go-tui/cell"

// Manager manages resources on a workspace. It satisfies ResourceOpener.
type Manager struct {
}

// NewManager allocates storage fore a new Manage and initializes it.
func NewManager() *Manager {
	ret := new(Manager)
	ret.Init()
	return ret
}

// Init initializes m.
func (m *Manager) Init() {
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
