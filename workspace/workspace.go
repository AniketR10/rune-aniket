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

// ResourceOpener abstract resource openining and recovering.
type ResourceOpener interface {
	Open(file URI, buf *cell.Buffer, swapDir URI, readOnly bool) (FlusherCloser, error)
	Recover(file, swapFilePath URI, buf *cell.Buffer) (FlusherCloser, error)
}
