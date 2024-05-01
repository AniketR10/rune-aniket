package browser

import (
	"time"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

// DefaultConfig returns the default Config.
func DefaultConfig() Config {
	return Config{
		FocusTabAttr:        term.Attributes{Fg: tcell.ColorWhite},
		NonFocusTabAttr:     term.Attributes{Fg: tcell.ColorRed},
		WallpaperAttr:       term.Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold},
		FrameUnionCharSet:   component.DefaultFrameUnionCharSet(),
		WindowManagerConfig: handler.DefaultWindowManagerConfig(),
		PromptConfig: PromptConfig{
			TextAttr:       term.Attributes{},
			HighlightAttr:  term.Attributes{Bg: tcell.ColorRed, Fg: tcell.ColorWhite},
			BackgroundAttr: term.Attributes{},
			MinWidth:       60,
		},
		Notifications: notifications.Config{
			AutoClose:            5 * time.Second,
			ProgressBar:          true,
			Width:                50,
			Attributes:           term.Attributes{},
			BackgroundAttributes: term.Attributes{},
			FrameCharSet:         component.FrameCharSetDefault(),
			Interrupter:          term.NopInterrupter(),
		},
	}
}

// PromptConfig holds configuration for the browser's Prompt component.
type PromptConfig struct {
	TextAttr       term.Attributes
	HighlightAttr  term.Attributes
	BackgroundAttr term.Attributes
	MinWidth       int
}

// Config holds configuration for an browser.Component.
type Config struct {
	Wallpaper               string
	WallpaperAttr           term.Attributes
	WallpaperBackgroundAttr term.Attributes

	FocusTabAttr    term.Attributes
	NonFocusTabAttr term.Attributes

	PromptConfig

	component.FrameUnionCharSet
	handler.WindowManagerConfig
	Notifications notifications.Config
}
