package handler

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
)

// Button is a component with a frame and on-click handler.
type Button struct {
	textStr      string
	textComp     tui.Component
	frameCharSet component.FrameCharSet
	clickCharSet component.FrameCharSet
	spanConfig   component.SpanConfig
	attr         term.Attributes

	frame component.Frame
	span  component.Span

	release bool
	onClick func()
}

// NewButton instantiates a new Button and initializes it.
func NewButton(text string, onClick func()) *Button {
	b := new(Button)
	b.Init(text, onClick)
	return b
}

// Init initializes this Button with text and onClick callback.
// It also sets a default FrameCharSet for both release and clicked states.
// To change these defaults use SetFrameCharSet and SetClickCharSet.
func (b *Button) Init(text string, onClick func()) {
	b.textStr = text
	b.textComp = component.StaticStringAttr(b.textStr, b.attr)
	b.onClick = onClick
	b.span.Init(b.textComp, b.spanConfig)
	b.frame.Init(&b.span)
	b.frame.FrameCharSet = b.frameCharSet
}

// SetAttr sets the attributes of the button text.
func (b *Button) SetAttr(newAttr term.Attributes) {
	b.attr = newAttr
	b.Init(b.textStr, b.onClick)
}

// SetSpanConfig sets the span configuration for the text in this Button.
func (b *Button) SetSpanConfig(cfg component.SpanConfig) {
	b.spanConfig = cfg
	b.span.Init(b.textComp, b.spanConfig)
}

// SetFrameCharSet sets the FrameCharSet when button is released.
func (b *Button) SetFrameCharSet(cs component.FrameCharSet) {
	b.frameCharSet = cs
	if !b.release {
		b.frame.FrameCharSet = b.frameCharSet
	}
}

// SetClickCharSet returns a copy of this Button with
// cs as the FrameCharSet to use when button is clicked and before it's released.
func (b *Button) SetClickCharSet(cs component.FrameCharSet) {
	b.clickCharSet = cs
	if b.release {
		b.frame.FrameCharSet = cs
	}
}

// Handle satisfies tui.Handler
func (b *Button) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type == term.EventMouse && ev.Key == term.MouseLeft ||
		ev.Type == term.EventKey && ev.Key == term.KeyEnter {
		b.release = true
		b.onClick()
		b.frame.FrameCharSet = b.clickCharSet
		handled = true
		return
	}
	if b.release && ev.Type == term.EventMouse &&
		ev.Key == term.MouseRelease {
		b.release = false
		b.frame.FrameCharSet = b.frameCharSet
		handled = true
		return
	}

	return
}

// Cursor satisfies tui.Handler
func (b *Button) Cursor() (term.Coordinates, bool) {
	return term.Coordinates{}, false
}

// Man satisfies tui.Handler
func (b *Button) Man() tui.Manual {
	return tui.Manual{
		Summary: "Button implements a simple click handler.",
		Keys: tui.KeyMap{
			term.Event{Type: term.EventMouse, Key: term.MouseLeft}: {
				ID:          "Click",
				Description: "Runs onClick callback",
			},
		},
	}
}

// Resize satisfies tui.Component
func (b *Button) Resize(width, height int) {
	b.frame.Resize(width, height)
}

// Draw staisfies tui.Component
func (b *Button) Draw(w term.Writer) {
	b.frame.Draw(w)
}
