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
	"time"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"github.com/unstablebuild/tcell/v3"
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

	// ShowManualAfter configures how long to sit idle until
	// command manual is displayed.
	ShowManualAfter time.Duration

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
}

// DefaultConfig returns a sane configuration for initializing a Handler.
func DefaultConfig() Config {
	return Config{
		NoMarkdown:       true,
		MaxHistory:       100,
		HistoryCycleKey:  term.KeyComb{Ch: ':'},
		HistoryToggleKey: term.KeyComb{Mod: term.ModMeta, Ch: 'r'},
		EditModeKey:      term.KeyComb{Mod: term.ModShift, Key: term.KeyEsc},
		MatchedTextAttr:  term.Attributes{Fg: tcell.ColorRed},
		FocusElementAttr: term.Attributes{Attrs: tcell.AttrBold | tcell.AttrUnderline, Fg: tcell.ColorRed},
		ElementAttr:      term.Attributes{},
		DocumentID:       "command-history",
		ShowManualAfter:  1 * time.Second,
		ShowProgressHint: true,
	}
}
