package tui

import (
	"fmt"
	"sync"

	"unstable.build/go-tui/term"
)

// Component represents a visual element that can be drawn
// in a text-based user interface. It wraps the basic Draw and Resize methods.
type Component interface {
	// Resize is used by clients to indicate what's the virtual space available
	// for this component to be drawn in subsequent calls to Draw.
	// When a component is initialized, its width and height is 0 until Resize is
	// called to set the appropiate dimensions.
	Resize(width, height int)
	// Draw draws this component to the underlying Writer. It returns non-nil error
	// if something went wrong in the process of writing or the writer returned
	// an error.
	Draw(term.Writer)
}

// Handler builds upon Component to add event-handling behavior.
// It wraps the basic Handle, Cursor and Man methods.
type Handler interface {
	Component
	// Handle abstracts the ability to handle input events. These events could
	// be key presses or other types of events. See tcell's documentation for more
	// information. Handle returns true if a handler is done processing events.
	// Clients can have multiple handlers in the same interface so
	// this method will be called only when a handler is in focus.
	Handle(term.Event) (exit, handled bool)
	// Cursor should return the cursor coordinates, style and whether it should be shown at all.
	Cursor() (c term.Coordinates, s term.CursorStyle, show bool)
	// Man returns a Handler's usage manual. See Manual for more information.
	Man() Manual
}

// Manual represents a handler's instruction manual and other
// useful information to be displayed in a text-based user interface.
type Manual struct {
	Summary string
	Keys    KeyMap
}

// EventDesc represents a description of how a Handler handles a certain event.
type EventDesc struct {
	ID          string
	Description string
}

// KeyMap represents a Handler's key mapping information in the Manual.
type KeyMap map[term.KeyComb]EventDesc

// Init initializes this library. This function should be called before any
// other functions. 'Close' must be called at the end to ensure graceful shutdown.
func Init() error {
	if err := term.Init(); err != nil {
		return fmt.Errorf("failed term init: %v", err)
	}

	return nil
}

type nopLocker struct{}

func (l nopLocker) Lock() {
}
func (l nopLocker) Unlock() {
}

// Run takes the given root handler, renders it full-screen,
// and starts feeding it with term.Events.
// Error is non-nil if there were any errors.
func Run(root Handler) (err error) {
	return RunWithLocker(root, nopLocker{})
}

// RunWithLocker runs the given root handler and uses lock to
// synchronize access to root.
func RunWithLocker(root Handler, lock sync.Locker) error {
	return run(root, lock, term.DefaultWriter)
}

// Size returns the total available width and height in the current terminal.
func Size() (width int, height int) {
	return term.Size()
}

// Close should be called when this library is not required anynmore.
func Close() {
	term.Close()
}
