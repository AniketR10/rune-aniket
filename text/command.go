package text

import (
	"context"

	"github.com/ernestrc/blue/iterator"
	textapi "unstable.build/go-tui/api/text"
)

// CommandHandler wraps the basic methods HandleCommand and Complete.
type CommandHandler interface {
	// HandleCommand is called when user issued a command previously registered via SubscribeCommand.
	HandleCommand(context.Context, textapi.Command) (exit bool, err error)

	// Complete takes command args and returns a list of expanded options for them.
	// It also returns a expanded version of the last arg, or an empty string
	// if the last arg could/should not be automatically expanded.
	Complete(ctx context.Context, name string, args []string) (
		iterator.Iterator[string], string, error,
	)
}

// CommandManual represents a command's manual and documentation.
// It adds an AliasOf field to textapi.CommandManual, something
// we don't want to expose to external clients.
type CommandManual struct {
	textapi.CommandManual

	// AliasOf defines this command as an alias of the
	// given command or sequence of commands.
	AliasOf []string
}

// FuncCommandHandler returns an CommandHandler that calls fn
// every time HandleCommand is invoked. If completeFn is not nil,
// then it is called when Complete is invoked.
func FuncCommandHandler(
	fn func(context.Context, textapi.Command) (bool, error),
	completeFn func(context.Context, string, []string) (iterator.Iterator[string], string, error),
) CommandHandler {
	return fnCommandHandler{
		cb:         fn,
		completeFn: completeFn,
	}
}

type fnCommandHandler struct {
	cb         func(context.Context, textapi.Command) (bool, error)
	completeFn func(context.Context, string, []string) (iterator.Iterator[string], string, error)
}

func (f fnCommandHandler) HandleCommand(ctx context.Context, c textapi.Command) (bool, error) {
	return f.cb(ctx, c)
}

func (f fnCommandHandler) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], string, error,
) {
	if f.completeFn != nil {
		return f.completeFn(ctx, name, args)
	}
	return iterator.FromSlice[string](nil), "", nil
}
