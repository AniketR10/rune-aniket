// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package search

import (
	"time"

	fzf "github.com/junegunn/fzf/src/algo"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const (
	// interrupt periodically but not on every new chunk
	defaultInterruptEvery    = 200 * time.Millisecond
	defaultSetFileCountEvery = 1024 // chunks
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

	// SyncSearch makes buffer-driven searches deterministic.
	// It should only be used in tests.
	SyncSearch bool

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
		Fg:    term.ColorRed,
		Attrs: term.AttrBold,
	}
	matchedTextAttr := term.Attributes{
		Fg: term.ColorRed,
	}
	searchBaseAttr := term.Attributes{}
	focusAttr := term.Attributes{
		Fg:    term.ColorRed,
		Attrs: term.AttrBold,
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
		syncSearch:        c.SyncSearch,
		caseSensitive:     c.CaseSensitive,
		bottomSearchBar:   c.BottomSearchBar,
		interruptEvery:    c.interruptEvery,
		setFileCountEvery: c.setFileCountEvery,
	}
}

// internal representation of ListConfig
type listConfig struct {
	matchedTextAttr   term.Attributes
	matchCountAttr    term.Attributes
	searchBaseAttr    term.Attributes
	textAttr          term.Attributes
	focusAttr         term.Attributes
	algo              fzf.Algo
	interrupter       term.Interrupter
	syncSearch        bool
	caseSensitive     bool
	bottomSearchBar   bool
	interruptEvery    time.Duration
	setFileCountEvery int
}
