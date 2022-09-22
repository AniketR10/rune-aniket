package main

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/workspace"
)

type nopLoader struct {
}

func (n nopLoader) Load(
	file workspace.URI, buf *cell.Buffer, swapDir workspace.URI, readOnly bool,
) (workspace.FlusherCloser, error) {
	panic("called Open on nop loader")
}
func (n nopLoader) Recover(
	file, swapFilePath workspace.URI, buf *cell.Buffer, force bool,
) (workspace.FlusherCloser, error) {
	panic("called Recover on nop loader")
}
func (n nopLoader) URI(string) (workspace.URI, error) {
	panic("called URI on nop loader")
}
