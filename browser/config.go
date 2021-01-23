package browser

import (
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

// DefaultConfig returns the default Config.
func DefaultConfig() Config {
	return Config{
		Logger:              nil,
		MessageBarAttr:      term.Attributes{Bg: term.ColorRed, Fg: term.ColorWhite},
		FocusTabAttr:        term.Attributes{Fg: term.ColorWhite},
		NonFocusTabAttr:     term.Attributes{Fg: term.ColorRed},
		StartTextAttr:       term.Attributes{Fg: term.ColorRed | term.AttrBold},
		FrameUnionCharSet:   component.DefaultFrameUnionCharSet(),
		WindowManagerConfig: component.DefaultWindowManagerConfig(),
	}
}

// Config holds configuration for an browser.Component.
type Config struct {
	Logger    *log.Logger
	StartText string

	StartTextAttr           term.Attributes
	StartTextBackgroundAttr term.Attributes
	MessageBarAttr          term.Attributes
	FocusTabAttr            term.Attributes
	NonFocusTabAttr         term.Attributes

	component.FrameUnionCharSet
	component.WindowManagerConfig
}
