package cell

import (
	"github.com/ernestrc/fractal/term"
)

// TODO think about how to dismiss caches

type Token struct {
	Cells    []term.Cell
	Metadata interface{}
}

type Scanner interface {
	// TokenAt(term.Coordinates) (Token, bool)
	Scan() (Token, bool)
}

type wordScanner struct {
}

func NewWordScanner(r Reader) Scanner {
	panic("TODO")
}
