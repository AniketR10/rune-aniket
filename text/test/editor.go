package test

import (
	context "context"
	"errors"

	"github.com/unstablebuild/blue/iterator"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	cell "unstable.build/go-tui/cell"
	term "unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

// EditorFromAPIEditor wraps a textapi.Editor to satisfy text.Editor.
type EditorFromAPIEditor struct {
	Ed textapi.Editor
}

func (e EditorFromAPIEditor) CellView(h text.Handler) text.CellView {
	return e.Ed.CellView(h)
}

func (e EditorFromAPIEditor) CellEditor(h text.Handler) text.CellEditor {
	return e.Ed.CellEditor(h)
}
func (e EditorFromAPIEditor) Edit(file workspaceapi.URI, buf *cell.Buffer) (text.Handler, error) {
	return nil, errors.New("Edit is unimplemented on textapi.Editor")
}

func (e EditorFromAPIEditor) SubscribeEvents(t []textapi.EventType, h text.EventHandler) error {
	return e.Ed.SubscribeEvents(t, h)
}

func (e EditorFromAPIEditor) UnsubscribeEvents(h text.EventHandler) (bool, error) {
	return e.Ed.(interface {
		UnsubscribeEvents(text.EventHandler) (bool, error)
	}).UnsubscribeEvents(h)
}

func (e EditorFromAPIEditor) Editor(file workspaceapi.URI) (text.Handler, error) {
	ed, err := e.Ed.Editor(file)
	if err != nil {
		return nil, err
	}
	return HandlerFromAPIHandler{ed}, nil
}

func (e EditorFromAPIEditor) SubscribeCommand(cmd textapi.CommandManual, h text.CommandHandler) error {
	return e.Ed.SubscribeCommand(cmd, APICommandHandlerFromCommandHandler{h})
}

func (e EditorFromAPIEditor) UnsubscribeCommand(cmd string) error {
	return e.Ed.(interface {
		UnsubscribeCommand(string) error
	}).UnsubscribeCommand(cmd)
}

func (e EditorFromAPIEditor) SetLocationList(h text.Handler, p textapi.LocationPriority, arg string, l text.LocationList) error {
	return e.Ed.SetLocationList(h, p, arg, l)
}

func (e EditorFromAPIEditor) MoveToNextLocation(h text.Handler, ID string) error {
	return e.Ed.MoveToNextLocation(h, ID)
}

func (e EditorFromAPIEditor) MoveToPrevLocation(h text.Handler, ID string) error {
	return e.Ed.MoveToPrevLocation(h, ID)
}

func (e EditorFromAPIEditor) Cursor(h text.Handler) (term.Coordinates, error) {
	return e.Ed.Cursor(h)
}

func (e EditorFromAPIEditor) SetCursor(h text.Handler, pos term.Coordinates) error {
	return e.Ed.SetCursor(h, pos)
}

func (e EditorFromAPIEditor) SetDefaultAttributes(h text.Handler, attr term.Attributes) error {
	return e.Ed.SetDefaultAttributes(h, attr)
}

// HandlerFromAPIHandler adapts api Handler to text.Handler.
type HandlerFromAPIHandler struct {
	textapi.Handler
}

func (w HandlerFromAPIHandler) SetWrap(wrap bool) {
}

func (w HandlerFromAPIHandler) ShowCommandBar(show bool) {
}

func (w HandlerFromAPIHandler) SetCursorAtScroll(pos term.Coordinates) bool {
	return false
}

type APICommandHandlerFromCommandHandler struct {
	text.CommandHandler
}

func (c APICommandHandlerFromCommandHandler) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], error,
) {
	it, _, err := c.CommandHandler.Complete(ctx, name, args)
	return it, err
}
