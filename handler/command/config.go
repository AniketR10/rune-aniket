// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package command

import (
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/cell"
)

// EditHandler is the tui.Handler returned by Editor.Edit. It exposes
// SetCursorAtScroll so the Prompt can position the editor's cursor at
// the same buffer coordinates the prompt was showing before entering
// edit mode (without this we would jump back to the start of the
// buffer every time).
type EditHandler interface {
	tui.Handler
	// SetCursorAtScroll moves the editor cursor to the given buffer
	// coordinates. The receiver typically wants to be Resize()d to a
	// non-zero size first so its internal scroll state can satisfy the
	// move.
	SetCursorAtScroll(pos term.Coordinates) bool
	// CursorAtScroll returns the editor cursor's position in buffer
	// (scroll) coordinates. Hosts that wrap the editor's buffer through
	// their own renderer (e.g. the command Prompt's responsive view)
	// rely on this to translate the buffer position to their own
	// visual layout, since Cursor() returns window coordinates that
	// reflect the editor's own scroll/wrap configuration.
	CursorAtScroll() term.Coordinates
}

// SelectionBoundsHandler is implemented by EditHandlers that can
// report their active selection range in buffer (scroll)
// coordinates. Hosts that re-render the editor's buffer through
// their own layout (e.g. the command Prompt's responsive view) use
// this optional capability to paint a selection highlight in their
// own coordinate system, since the editor's selection rendering is
// otherwise tied to its own Draw pipeline (DrawLocations) and would
// not survive an external responsive draw.
type SelectionBoundsHandler interface {
	SelectionBounds() (from, to term.Coordinates, ok bool)
}

// Editor is the interface implemented by text editors that can be
// plugged into the Prompt to drive its modal edit mode. It is
// satisfied by call-site adapters that construct a bare vi.Vi or
// modeless editor handler bound to the given buffer (avoiding any
// auxiliary status / icons / location bars that the full text.Editor
// pipeline layers on top, which would otherwise shift the visible
// cursor coordinates).
//
// While in edit mode the Prompt forwards every input event to the
// EditHandler returned by Edit, allowing users to freely modify the
// prompt's input buffer with the editor's full feature set (cursor
// navigation, undo, selection, …). The Prompt Close()s the handler
// (if it implements io.Closer) when edit mode exits.
type Editor interface {
	// Edit returns an EditHandler bound to buf. The Prompt retains
	// ownership of the returned handler only while edit mode is active.
	Edit(buf *cell.Buffer) EditHandler
}

// Config represents the configuration needed to initialize a Handler.
type Config struct {
	// HistoryCycleKey is the key used to cycle through previously executed
	// commands one at a time, replacing the input buffer with each entry.
	HistoryCycleKey term.KeyComb
	// HistoryToggleKey is the key used to toggle the entire prompt list
	// between the available commands and the command history. Pressing it
	// once shows history entries; pressing it again restores the command list.
	HistoryToggleKey term.KeyComb
	// EditModeKey toggles a modal text-editor mode where the prompt
	// input can be freely edited without triggering completions,
	// searches or manual updates. Pressing it again, <enter> or <tab>
	// exits the mode and replays the buffer through the prompt as if
	// it had been pasted, re-computing completions accordingly.
	EditModeKey term.KeyComb
	// Editor drives the prompt's modal edit mode (see EditModeKey).
	// Required: NewPrompt/Init panic if nil.
	Editor           Editor
	MatchedTextAttr  term.Attributes
	FocusElementAttr term.Attributes
	ElementAttr      term.Attributes
	// DocumentID is the key used to store data in the underlying document.Service.
	DocumentID string
	MaxHistory int

	// Sync makes auto-completion deterministic but very very slow.
	// It should only be used in tests.
	Sync bool

	// ShowManual controls whether the alternate manual side-panel
	// is rendered for the focused command/argument. When true (the
	// default) the manual is computed and shown synchronously on
	// every input change.
	ShowManual bool

	// ShowProgressHint controls whether an asynchronous completion in progress
	// renders a transient animation in the prompt.
	ShowProgressHint bool

	// ManualAttr is used to configure the style of the alternate manual window.
	ManualAttr term.Attributes

	// FrameCharSet is used to determine if a frame is to be used to separate manual from search list.
	FrameCharSet component.FrameCharSet
	// FrameAttr if a frame is to be used to separate manual from search list.
	FrameAttr  term.Attributes
	NoMarkdown bool

	// KeyBindingHint returns the long-form display label for the key
	// bound to the given full command line (e.g. "lsp diagnostics"), or
	// "" if none/unbound. When nil, no hints are shown.
	KeyBindingHint func(commandLine string) string
	// KeyBindingHintAttr styles the right-aligned key hint.
	KeyBindingHintAttr term.Attributes
	// KeyBindingHintFocusAttr styles the key hint on the focused row.
	KeyBindingHintFocusAttr term.Attributes
}

// DefaultConfig returns a sane configuration for initializing a Handler.
func DefaultConfig() Config {
	return Config{
		NoMarkdown:              true,
		MaxHistory:              100,
		HistoryCycleKey:         term.KeyComb{Ch: ':'},
		HistoryToggleKey:        term.KeyComb{Mod: term.ModMeta, Ch: 'r'},
		EditModeKey:             term.KeyComb{Mod: term.ModShift, Key: term.KeyEsc},
		MatchedTextAttr:         term.Attributes{Fg: term.ColorRed},
		FocusElementAttr:        term.Attributes{Attrs: term.AttrBold | term.AttrUnderline, Fg: term.ColorRed},
		ElementAttr:             term.Attributes{},
		DocumentID:              "command-history",
		ShowManual:              true,
		ShowProgressHint:        true,
		KeyBindingHintAttr:      term.Attributes{Fg: term.ColorGray},
		KeyBindingHintFocusAttr: term.Attributes{Fg: term.ColorSilver},
	}
}
