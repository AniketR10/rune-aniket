package browser

import (
	"io"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// Window is the interface that represents
// a closeable window in a WindowManager.
type Window interface {
	Close() error
}

// WindowManager is the interface that groups tile
// window management methods.
type WindowManager interface {
	SplitVerticalRight(tui.Handler) (Window, error)
	SplitVerticalLeft(tui.Handler) (Window, error)
	SplitHorizontalAbove(tui.Handler) (Window, error)
	SplitHorizontalBelow(tui.Handler) (Window, error)
}

// EventHandler wraps the basic tui.Handler method Handle.
type EventHandler interface {
	Handle(term.Event) (exit bool)
}

// EventPublisher handler is the interface that wraps
// the method Subscribe to install
type EventPublisher interface {
	Subscribe(term.Event, EventHandler) error
}

// KeyMapper is the interface that wraps the method MergeKeyMap
// to merge new key mappings.
type KeyMapper interface {
	MergeKeyMap(map[term.Event]term.Event) error
}

// Messenger is the interface that wraps methods to display
// messages to the user.
type Messenger interface {
	SetMessage(msg string, args ...interface{}) error
}

// FileOpener is the interface that wraps the method OpenFile.
type FileOpener interface {
	OpenFile(filename string) error
}

// Browser is an interface that groups methods to manipulate
// the user interface of a browser.
type Browser interface {
	WindowManager
	EventPublisher
	KeyMapper
	FileOpener
	Messenger
	io.Closer
}
