package api

import (
	"context"

	"github.com/ernestrc/blue/iterator"
	browserapi "unstable.build/go-tui/api/browser"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term"
)

// Command represents a command issued by the user.
type Command struct {
	Name string
	Args []string

	// optional. If command is dispatched while non-tab is in focus,
	// then these fields will be zero-valued.
	URI      workspaceapi.URI
	Resource Handler
	Window   browserapi.Window
	Cursor   struct {
		Content term.Coordinates
		Window  term.Coordinates
	}
}

// CommandHandler is a callback interface that wraps the basic method Command.
type CommandHandler interface {
	// Handle is called when user issued a command previously registered via SubscribeCommand.
	HandleCommand(context.Context, Command) (exit bool, err error)
}

type fnCommandHandler struct {
	cb         func(context.Context, Command) (bool, error)
	completeFn func(context.Context, []string) (iterator.Iterator[string], error)
}

func (f fnCommandHandler) HandleCommand(ctx context.Context, c Command) (bool, error) {
	return f.cb(ctx, c)
}

func (f fnCommandHandler) Complete(ctx context.Context, args []string) (
	iterator.Iterator[string], error,
) {
	if f.completeFn != nil {
		return f.completeFn(ctx, args)
	}
	return iterator.FromSlice[string](nil), nil
}

// FuncCommandHandler returns an CommandHandler that calls fn
// every time HandleCommand is invoked.
func FuncCommandHandler(fn func(context.Context, Command) (bool, error)) CommandHandler {
	return fnCommandHandler{
		cb: fn,
	}
}
