package command

import (
	"context"

	"github.com/ernestrc/blue/iterator"
)

// Completer abstracts the ability to complete command arguments.
type Completer interface {
	Complete(ctx context.Context, cmd string, args ...string) iterator.Iterator[string]
}

// FuncCompleter returns a Completer that calls fn every time Complete is called.
func FuncCompleter(
	fn func(context.Context, string, ...string) iterator.Iterator[string],
) Completer {
	return fnCompleter{fn: fn}
}

type fnCompleter struct {
	fn func(context.Context, string, ...string) iterator.Iterator[string]
}

func (d fnCompleter) Complete(
	ctx context.Context, cmd string, args ...string,
) iterator.Iterator[string] {
	return d.fn(ctx, cmd, args...)
}
