package search

import (
	"time"

	fzf "github.com/junegunn/fzf/src/algo"
	"unstable.build/go-tui/term"
)

const (
	// interrupt periodically but not on every new chunk
	defaultInterruptEvery    = 200 * time.Millisecond
	defaultSetFileCountEvery = 256 // chunks
)

// AlgoConfig is the algoritum to use to search through the input data.
type AlgoConfig uint

const (
	// FuzzyMatch instructs list to perform approximate string matching.
	FuzzyMatch AlgoConfig = iota
	// EqualMatch instructs list to perform equal string matching.
	EqualMatch
	// ContainsMatch instructs list to match strings that contain search term.
	ContainsMatch
)

// ListConfig is used to initialize a List.
type ListConfig struct {
	// Algorithm to use. See AlgoConfig.
	Algo AlgoConfig

	// function to use to force a redraw of the list.
	Interrupter term.Interrupter

	// Attributes to use to highlight matched text
	MatchedTextAttr *term.Attributes

	// Attributes to use on match and total count row
	CountAttr *term.Attributes

	// FocusElementAttr attributes to use for the element in focus.
	FocusElementAttr *term.Attributes

	// ElementAttr attributes to use for the list elements.
	ElementAttr *term.Attributes

	// CaseSensitive determines whether Algo is case sensitive.
	CaseSensitive bool

	// BottomSearchBar determines whether the search bar and therefore
	// the highest score matches should be at the top or at the bottom.
	BottomSearchBar bool

	interruptEvery    time.Duration
	setFileCountEvery int
}

func (c ListConfig) toInternal() listConfig {
	matchCountAttr := term.Attributes{
		Fg: term.ColorRed | term.AttrBold,
	}
	matchedTextAttr := term.Attributes{
		Fg: term.ColorRed,
	}
	searchBaseAttr := term.Attributes{}
	focusAttr := term.Attributes{
		Fg: term.AttrBold | term.ColorRed,
	}
	textAttr := term.Attributes{}
	if c.MatchedTextAttr != nil {
		matchedTextAttr = *c.MatchedTextAttr
	}
	if c.CountAttr != nil {
		matchCountAttr = *c.CountAttr
	}
	if c.FocusElementAttr != nil {
		focusAttr = *c.FocusElementAttr
	}
	if c.ElementAttr != nil {
		textAttr = *c.ElementAttr
	}

	interrupter := term.NopInterrupter()
	if c.Interrupter != nil {
		interrupter = c.Interrupter
	}

	algo := fzf.FuzzyMatchV2
	switch c.Algo {
	case EqualMatch:
		algo = fzf.EqualMatch
	case ContainsMatch:
		algo = containsMatch
	default:
		algo = fzf.FuzzyMatchV2
	}

	if c.interruptEvery == 0 {
		c.interruptEvery = defaultInterruptEvery
	}

	if c.setFileCountEvery == 0 {
		c.setFileCountEvery = defaultSetFileCountEvery
	}

	return listConfig{
		algo:              algo,
		matchedTextAttr:   matchedTextAttr,
		matchCountAttr:    matchCountAttr,
		searchBaseAttr:    searchBaseAttr,
		textAttr:          textAttr,
		focusAttr:         focusAttr,
		interrupter:       interrupter,
		caseSensitive:     c.CaseSensitive,
		bottomSearchBar:   c.BottomSearchBar,
		interruptEvery:    c.interruptEvery,
		setFileCountEvery: c.setFileCountEvery,
	}
}
