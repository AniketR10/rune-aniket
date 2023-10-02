package browser

import (
	"time"

	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

// DefaultConfig returns the default Config.
func DefaultConfig() Config {
	return Config{
		FocusTabAttr:        term.Attributes{Fg: term.ColorWhite},
		NonFocusTabAttr:     term.Attributes{Fg: term.ColorRed},
		WallpaperAttr:       term.Attributes{Fg: term.ColorRed | term.AttrBold},
		FrameUnionCharSet:   component.DefaultFrameUnionCharSet(),
		WindowManagerConfig: handler.DefaultWindowManagerConfig(),
		PromptConfig: PromptConfig{
			Width:         50,
			Height:        14,
			TextAttr:      term.Attributes{},
			HighlightAttr: term.Attributes{Bg: term.ColorRed, Fg: term.ColorWhite},
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
	Width, Height int
	TextAttr      term.Attributes
	HighlightAttr term.Attributes
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
