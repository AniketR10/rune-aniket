// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package searchbox

import (
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/text"
)

// Mode selects which inputs the search box displays.
type Mode uint8

const (
	// ModeFind displays the query input only.
	ModeFind Mode = iota
	// ModeReplace additionally displays the replacement input.
	ModeReplace
)

// WindowManager manages the floating window a Box is presented in. Hosts
// that draw Content themselves do not need one.
type WindowManager interface {
	Floating(browserapi.Floating, browserapi.FloatingConfig) (browserapi.Window, error)
	CloseWindow(browserapi.Window) error
}

// Controller searches the content a Box is attached to. Coordinates are
// expressed in the controller's own content space; the box only relays them.
//
// Selection reports the selection of the controller's own content. Hosts
// report the selection of whatever holds the focus, which is the box while
// it is open, so the box hands the question over whenever its own inputs
// hold no selection.
type Controller interface {
	BeginSearch(query []rune, origin term.Coordinates)
	SetSearchQuery(query string)
	AdvanceSearch()
	FinishSearch()
	SearchLast() string
	SearchOrigin() term.Coordinates
	Selection() (string, bool)
}

// ReplaceController is a Controller that can also rewrite matches. The
// replacement input and the find-to-replace affordance are only rendered
// for controllers that implement it.
type ReplaceController interface {
	Controller
	ReplaceNext(replacement string)
	ReplaceAll(replacement string)
}

// Config configures Box.
type Config struct {
	// WindowManager, when set, presents the box in a floating window.
	// Otherwise the host draws Content wherever it sees fit.
	WindowManager WindowManager
	// Editor provides the key bindings of the query and replacement inputs.
	Editor text.Editor
	// Title is rendered on the floating window bar. When empty a title is
	// derived from whether the controller supports replacing.
	Title string

	FindKey term.KeyComb
	// FindKeyAliases are additional key combinations that open and advance
	// the search, on top of FindKey.
	FindKeyAliases []term.KeyComb
	ReplaceKey     term.KeyComb

	// PaddingTop is the number of blank rows drawn above the frame. Hosts
	// that present the box flush against an edge, such as a terminal
	// overlay, can set this to 0.
	PaddingTop int

	Attr            term.Attributes
	InputAttr       term.Attributes
	PlaceholderAttr term.Attributes
	FrameAttr       term.Attributes
	FocusFrameAttr  term.Attributes
	ButtonAttr      term.Attributes
	ButtonHoverAttr term.Attributes

	// OnOpenError is called when the floating window cannot be opened, so
	// hosts can report the failure or fall back to another prompt.
	OnOpenError func(Mode, error)
}

// Box is a floating find and replace prompt driven by a Controller.
type Box struct {
	controller Controller
	replace    ReplaceController
	config     Config
	floating   *floating
	window     browserapi.Window
}

// New allocates storage for a new Box and initializes it. If controller also satisfies
// ReplaceController then replace functionality is added to the search box.
func New(controller Controller, config Config) *Box {
	b := &Box{controller: controller, config: config}
	b.replace, _ = controller.(ReplaceController)
	return b
}

// Active reports whether the box is currently open.
func (b *Box) Active() bool { return b.floating != nil }

// Content returns the open box, for hosts that present it themselves. It
// returns nil while the box is closed.
func (b *Box) Content() browserapi.Floating {
	if b.floating == nil {
		return nil
	}
	return b.floating
}

// HandleKey opens, focuses or advances the search box and reports whether
// ev was consumed.
func (b *Box) HandleKey(ev term.Event) bool {
	if b.matchesFind(ev) {
		if b.floating == nil {
			b.open(ModeFind)
		} else {
			b.floating.setFocus(focusQuery)
			b.controller.AdvanceSearch()
			b.floating.ensureFocusVisible()
		}
		return true
	}
	if b.replace != nil && keyMatches(ev, b.config.ReplaceKey) {
		if b.floating == nil {
			b.open(ModeReplace)
		} else if b.floating.mode == ModeFind {
			b.floating.upgrade()
		} else {
			b.floating.setFocus(focusReplacement)
			b.floating.ensureFocusVisible()
		}
		return true
	}
	return false
}

// Finish tears down the search, optionally closing the floating window.
func (b *Box) Finish(closeWindow bool) error {
	if b.floating == nil {
		return nil
	}
	b.floating = nil
	win := b.window
	b.window = nil
	b.controller.FinishSearch()
	if closeWindow && win != nil {
		return b.config.WindowManager.CloseWindow(win)
	}
	return nil
}

// Close satisfies tui.Component.
func (b *Box) Close() error { return b.Finish(true) }

func (b *Box) matchesFind(ev term.Event) bool {
	if keyMatches(ev, b.config.FindKey) {
		return true
	}
	for _, alias := range b.config.FindKeyAliases {
		if keyMatches(ev, alias) {
			return true
		}
	}
	return false
}

func (b *Box) open(mode Mode) {
	origin := b.controller.SearchOrigin()
	query := b.controller.SearchLast()
	f := newFloating(b, mode, query)
	if b.config.WindowManager != nil {
		win, err := b.config.WindowManager.Floating(f, browserapi.FloatingConfig{
			Alignment: component.AlignmentTop | component.AlignmentHorizontallyCentered,
			Offset:    term.Coordinates{Y: 2},
			Title:     b.title(),
		})
		if err != nil {
			if b.config.OnOpenError != nil {
				b.config.OnOpenError(mode, err)
			}
			return
		}
		b.window = win
	}
	b.floating = f
	b.controller.BeginSearch([]rune(query), origin)
}

func (b *Box) title() string {
	if b.config.Title != "" {
		return b.config.Title
	}
	if b.replace != nil {
		return "Find / Replace"
	}
	return "Find"
}

func keyMatches(ev term.Event, key term.KeyComb) bool {
	return ev.Type == term.EventKey && ev.Mod == key.Mod && ev.Key == key.Key && ev.Ch == key.Ch
}
