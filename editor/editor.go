package editor

//go:generate mockgen -destination=./editor_gomock.go -package editor -self_package editor -source editor.go

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
)

// Handler just wraps a tui.Handler to indicate that this API's handlers might
// not be compatible with other APIs.
type Handler interface {
	tui.Handler
}

// Editor is the interface that wraps an API to manage a text editor.
type Editor interface {
	// Edit opens a file and returns a tui.Handler to edit it or an error
	// if there was an error opening it.
	Edit(name string, buf *cell.Buffer) (Handler, error)

	// SubscribeEditor subscribes EventHandler to events of type EventType.
	// Note that it's suffixed with Editor so implementors
	// can also implement browser.Subscriber.
	// TODO should rename browser.Subscribe to browser.SubscribeTerm
	SubscribeEditor(EventType, EventHandler) error

	SetLocationList(Handler, LocationList) error
}
