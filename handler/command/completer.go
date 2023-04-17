package command

import (
	"context"

	"github.com/ernestrc/blue/iterator"
)

// Completer abstracts the ability to complete command arguments.
type Completer interface {
	// Complete takes the given command and arguments and returns an iterator
	// over an expanded list of options for the last argument. It also returns
	// an expanded version of the last argument, if there is one, or an empty
	// string if the last argument could/should not be automatically expanded.
	Complete(ctx context.Context, cmd string, args ...string) (
		iterator.Iterator[string], string,
	)
}

// FuncCompleter returns a Completer that calls fn every time Complete is called.
func FuncCompleter(
	fn func(context.Context, string, ...string) (iterator.Iterator[string], string),
) Completer {
	return fnCompleter{fn: fn}
}

type fnCompleter struct {
	fn func(context.Context, string, ...string) (iterator.Iterator[string], string)
}

func (d fnCompleter) Complete(
	ctx context.Context, cmd string, args ...string,
) (iterator.Iterator[string], string) {
	return d.fn(ctx, cmd, args...)
}
