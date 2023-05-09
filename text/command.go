package text

import (
	"context"

	"github.com/ernestrc/blue/iterator"
	textapi "unstable.build/go-tui/api/text"
)

// EventTypeCommand extends textapi.EventType to re-use functionality.
const EventTypeCommand textapi.EventType = textapi.EventType(uint8(99))

// CommandHandler wraps the basic methods HandleCommand and Complete.
type CommandHandler interface {
	// HandleCommand is called when user issued a command previously registered via SubscribeCommand.
	HandleCommand(context.Context, textapi.Command) (exit bool, err error)

	// Complete takes command args and returns a list of expanded options for them.
	// It also returns a expanded version of the last arg, or an empty string
	// if the last arg could/should not be automatically expanded.
	Complete(ctx context.Context, args []string) (
		iterator.Iterator[string], string, error,
	)
}

type fnCommandHandler struct {
	cb         func(context.Context, textapi.Command) (bool, error)
	completeFn func(context.Context, []string) (iterator.Iterator[string], string, error)
}

func (f fnCommandHandler) HandleCommand(ctx context.Context, c textapi.Command) (bool, error) {
	return f.cb(ctx, c)
}

func (f fnCommandHandler) Complete(ctx context.Context, args []string) (
	iterator.Iterator[string], string, error,
) {
	if f.completeFn != nil {
		return f.completeFn(ctx, args)
	}
	return iterator.FromSlice[string](nil), "", nil
}

// FuncCommandHandler returns an CommandHandler that calls fn
// every time HandleCommand is invoked.
func FuncCommandHandler(fn func(context.Context, textapi.Command) (bool, error)) CommandHandler {
	return fnCommandHandler{
		cb: fn,
	}
}

// FuncCommandCompleter returns an CommandHandler that calls fn
// every time HandleCommand is invoked but also satisfies CommandCompleter,
// and so calls completeFn when Complete is called.
func FuncCommandCompleter(
	fn func(context.Context, textapi.Command) (bool, error),
	completeFn func(context.Context, []string) (iterator.Iterator[string], string, error),
) CommandHandler {
	return fnCommandHandler{
		cb:         fn,
		completeFn: completeFn,
	}
}
