package workspace

import (
	"io"

	"github.com/ernestrc/go-tui/cell"
)

// FlusherCloser wraps Flush and Close methods to be used
// as editor file abstractions.
type FlusherCloser interface {
	Flush() error
	io.Closer
}

// Recover recovers the file with the swap file.
func Recover(file, swapFile URI, buf *cell.Buffer) (
	FlusherCloser, error,
) {
	return recoverLocalFile(file, swapFile, buf)
}

// Open opens the file at the given URI and initializes buf with the contents of it.
// It uses swapDir as the file recovery and swap directory.
func Open(
	file URI, buf *cell.Buffer, swapDir URI, readOnly bool,
) (
	FlusherCloser, error,
) {
	return openLocalFile(file, buf, swapDir, readOnly)
}
