package command

import (
	"time"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// Config represents the configuration needed to initialize a Handler.
type Config struct {
	// HistoryKey is the key used to trigger scrolling through history.
	HistoryKey       term.KeyComb
	MatchedTextAttr  term.Attributes
	FocusElementAttr term.Attributes
	ElementAttr      term.Attributes
	// DocumentID is the key used to store data in the underlying document.Service.
	DocumentID string
	MaxHistory int

	// Sync makes auto-completion deterministic but very very slow.
	// It should only be used in tests.
	Sync bool

	// ShowManualAfter configures how long to sit idle until
	// command manual is displayed.
	ShowManualAfter time.Duration

	// ManualAttr is used to configure the style of the alternate manual window.
	ManualAttr term.Attributes

	// FrameCharSet is used to determine if a frame is to be used to separate manual from search list.
	FrameCharSet component.FrameCharSet
	// FrameAttr if a frame is to be used to separate manual from search list.
	FrameAttr term.Attributes
}

// DefaultConfig returns a sane configuration for initializing a Handler.
func DefaultConfig() Config {
	return Config{
		MaxHistory:       100,
		HistoryKey:       term.KeyComb{Ch: ':'},
		MatchedTextAttr:  term.Attributes{Fg: tcell.ColorRed},
		FocusElementAttr: term.Attributes{Attrs: tcell.AttrBold | tcell.AttrUnderline, Fg: tcell.ColorRed},
		ElementAttr:      term.Attributes{},
		DocumentID:       "command-history",
		ShowManualAfter:  1 * time.Second,
	}
}
