package command

import "unstable.build/go-tui/term"

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
}

// DefaultConfig returns a sane configuration for initializing a Handler.
func DefaultConfig() Config {
	return Config{
		MaxHistory:       100,
		HistoryKey:       term.KeyComb{Ch: ':'},
		MatchedTextAttr:  term.Attributes{Fg: term.ColorRed},
		FocusElementAttr: term.Attributes{Fg: term.AttrBold | term.AttrUnderline | term.ColorRed},
		ElementAttr:      term.Attributes{},
		DocumentID:       "command-history",
	}
}
