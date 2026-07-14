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

package emacs

import (
	"context"
	"strings"
	"unicode"

	"github.com/ernestrc/logd-go/logging"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/ide/syntax"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/registerhistory"
	"unstable.build/go-tui/text/registerset"
)

var _ component.Scrollable = (*emacsHandler)(nil)

const emacsMarkLocationListID = "mark"

type emacsHandler struct {
	cfg              emacsConfig
	buf              *cell.Buffer
	less             handler.Less
	statusBar        statusBar
	pasteBuf         strings.Builder
	pasteStarted     bool
	resource         workspaceapi.URI
	cursor           text.Cursor
	height           int
	mouse            *mouse.Mouse
	clipboard        clipboard.Register
	macroRecorder    MacroRecorder
	macroPlayer      MacroPlayer
	pendingSetCursor *term.Coordinates
	lastIterateWord  term.Coordinates
	setLocations     bool
	lastPaste        bool
	historyIdx       int
}

// NewHandler returns a emacs, simple-to-use text.Handler. indentTabspaces
// is the number of spaces per indent level when indentRune is IndentRuneSpace;
// pass 0 to fall back to the editor's configured tabspaces.
func NewHandler(
	buf *cell.Buffer, resource workspaceapi.URI,
	indentRune rune, indentTabspaces int, opts ...Option,
) text.Handler {
	ret := new(emacsHandler)
	ret.Init(buf, resource, indentRune, indentTabspaces, opts...)
	return ret
}

func (h *emacsHandler) Init(
	buf *cell.Buffer, resource workspaceapi.URI,
	indentRune rune, indentTabspaces int, opts ...Option,
) {
	h.cfg = defaultConfig()
	h.cfg.indentRune = indentRune
	h.cfg.indentTabspaces = indentTabspaces
	for _, o := range opts {
		o(&h.cfg)
	}
	h.buf = buf
	h.resource = resource
	h.less.InitWithBuffer(buf, handler.LessConfig{
		Wrap:               h.cfg.wrap,
		NoBar:              !h.cfg.commandBar,
		SuperimposeMessage: true,
		ResAttr:            h.cfg.resAttr,
		Attributes:         h.cfg.attr,
	})
	h.less.Scroll().SetTabspaces(h.cfg.tabspaces)
	h.cursor.Init(h.less.Scroll(), h.cfg.scheduleNextTick)
	if spec, ok := text.CommentSpecForURI(resource, h.cfg.comments); ok {
		h.cursor.SetCommentSpec(spec)
	}
	h.mouse = mouse.New(text.CursorMouseDelegate(&h.cursor))
	h.clipboard = h.cfg.clipboard
	h.macroRecorder = h.cfg.macroRecorder
	h.macroPlayer = h.cfg.macroPlayer
	h.statusBar = nopBar{}
	h.lastIterateWord.X = -1
	if h.cfg.enableInitialFolds {
		h.hideInitialFolds()
	}
}

// Resize satisfies tui.Component
func (h *emacsHandler) Resize(width, height int) {
	h.height = height
	h.less.Resize(width, height)
	if h.pendingSetCursor != nil {
		h.SetCursorAtScroll(*h.pendingSetCursor)
		h.pendingSetCursor = nil
	}
}

func (h *emacsHandler) setActiveLocationListMessage(locs []textapi.Location) {
	// NOTE: if there are multiple location lists with a message
	// in current cursor position, then there's no guarantee of which one
	// is going to be rendered.
	for _, loc := range locs {
		if loc.Message != "" {
			h.less.SetMessage("%s", loc.Message)
			return
		}
	}
	h.less.SetMessage("")
}

func (h *emacsHandler) drawLocationMessage() {
	locs, ok := h.cursor.LocationsAtCursor()
	if ok {
		h.setActiveLocationListMessage(locs)
		h.setLocations = true
	} else if h.setLocations {
		h.less.SetMessage("")
		h.setLocations = false
	}
}

// Draw satisfies tui.Component
func (h *emacsHandler) Draw(w term.Writer) {
	h.drawLocationMessage()
	h.less.Draw(w)
	text.DrawLocations(h.cursor.SortedLocations(), h.less.Scroll(), w)
}

func (h *emacsHandler) setMarkLocation() bool {
	locs := h.markLocations()
	pos := h.cursor.CursorAtScroll()
	locs = append(locs, h.newMarkLocation(pos))
	h.cursor.SetLocationList(textapi.LocationPriorityInfo, emacsMarkLocationListID,
		text.LocationSlice(locs))
	return true
}

func (h *emacsHandler) newMarkLocation(pos term.Coordinates) textapi.Location {
	return textapi.Location{
		From: pos,
		To:   term.Coordinates{Y: pos.Y, X: pos.X + 1},
		Attr: term.Attributes{Bg: term.ColorGray},
	}
}

func (h *emacsHandler) markLocations() []textapi.Location {
	for _, list := range h.cursor.LocationLists() {
		if list.ID != emacsMarkLocationListID {
			continue
		}
		locs := make([]textapi.Location, len(list.Locations))
		copy(locs, list.Locations)
		return locs
	}
	return nil
}

func (h *emacsHandler) markLocation() (textapi.Location, bool) {
	locs := h.markLocations()
	if len(locs) == 0 {
		return textapi.Location{}, false
	}
	return locs[len(locs)-1], true
}

func (h *emacsHandler) clearMarkLocation() bool {
	h.cursor.SetLocationList(textapi.LocationPriorityInfo, emacsMarkLocationListID, nil)
	return true
}

func (h *emacsHandler) selectToMark(delete bool) bool {
	loc, ok := h.markLocation()
	if !ok {
		return false
	}
	from := loc.From
	to := h.cursor.CursorAtScroll()
	if delete {
		from, to = to, from
	}
	if !h.cursor.SelectRange(from, to) {
		return false
	}
	if !delete {
		return true
	}
	if !h.cursor.DeleteSelection() {
		return false
	}
	h.popMarkLocation()
	return true
}

func (h *emacsHandler) popMarkLocation() bool {
	locs := h.markLocations()
	if len(locs) == 0 {
		return false
	}
	locs = locs[:len(locs)-1]
	if len(locs) == 0 {
		return h.clearMarkLocation()
	}
	h.cursor.SetLocationList(textapi.LocationPriorityInfo, emacsMarkLocationListID,
		text.LocationSlice(locs))
	return true
}

func (h *emacsHandler) playMacro() bool {
	if h.macroPlayer == nil {
		return false
	}
	if err := h.macroPlayer.Play(registerset.UnnamedRegisterID, 1); err != nil {
		h.log(log.ErrorLevel, "macro playback: %v", err)
	}
	return true
}

// killRegion deletes the text between the mark and point (C-w).
func (h *emacsHandler) killRegion() bool {
	return h.selectToMark(true)
}

// copyRegion copies the text between the mark and point to the clipboard,
// leaving point and the buffer unchanged (M-w).
func (h *emacsHandler) copyRegion() bool {
	loc, ok := h.markLocation()
	if !ok {
		return false
	}
	point := h.cursor.CursorAtScroll()
	if !h.cursor.SelectRange(loc.From, point) {
		return false
	}
	if _, err := h.cursor.CopySelection(clipboard.DefaultRegisterID, h.clipboard); err != nil {
		h.log(log.ErrorLevel, "cursor copy selection: %v", err)
		return false
	}
	h.cursor.MoveToScroll(point)
	return true
}

var sexpOpeners = map[rune]struct{}{'(': {}, '[': {}, '{': {}}
var sexpClosers = map[rune]struct{}{')': {}, ']': {}, '}': {}}

// moveForwardSexp implements a bracket-based forward-sexp (C-M-f): it scans
// forward for the next opening bracket, jumps to its balanced close and leaves
// point just past it. It is intentionally bracket-only and does not treat bare
// atoms as sexps. Point is restored when no bracket is found.
func (h *emacsHandler) moveForwardSexp() bool {
	mark := h.cursor.Mark()
	for {
		cell, ok := h.cursor.Cell()
		if ok {
			if _, isOpen := sexpOpeners[cell.Ch]; isOpen {
				if h.cursor.MoveToMatchingRune() {
					h.cursor.MoveRight()
					return true
				}
				break
			}
		}
		if !h.cursor.MoveRight() {
			break
		}
	}
	h.cursor.MoveToMark(mark)
	return false
}

// moveBackwardSexp implements a bracket-based backward-sexp (C-M-b): it scans
// backward for the closing bracket that ends the previous sexp, jumps to its
// balanced open and leaves point on it. Point is restored when no bracket is
// found.
func (h *emacsHandler) moveBackwardSexp() bool {
	mark := h.cursor.Mark()
	for h.cursor.MoveLeft() {
		cell, ok := h.cursor.Cell()
		if !ok {
			continue
		}
		if _, isClose := sexpClosers[cell.Ch]; isClose {
			if h.cursor.MoveToMatchingRune() {
				return true
			}
			break
		}
	}
	h.cursor.MoveToMark(mark)
	return false
}

// selectWordForward selects from point to the end of the current or next
// word, matching how the Emacs word-case commands operate on the word at or
// after point.
func (h *emacsHandler) selectWordForward() bool {
	if _, ok := h.cursor.SelectionMode(); ok {
		return true
	}
	if !h.cursor.Select() {
		return false
	}
	if !h.cursor.MoveRightEndWord() {
		h.cursor.Unselect()
		return false
	}
	return true
}

func (h *emacsHandler) upcaseWord() bool {
	if !h.selectWordForward() {
		return false
	}
	handled := h.cursor.UppercaseSelection()
	h.cursor.Unselect()
	return handled
}

func (h *emacsHandler) downcaseWord() bool {
	if !h.selectWordForward() {
		return false
	}
	handled := h.cursor.LowercaseSelection()
	h.cursor.Unselect()
	return handled
}

func (h *emacsHandler) capitalizeWord() bool {
	if !h.selectWordForward() {
		return false
	}
	word := h.cursor.Selection()
	capitalized := capitalize(word)
	if capitalized == word {
		h.cursor.Unselect()
		return true
	}
	if !h.cursor.DeleteSelection() {
		h.cursor.Unselect()
		return false
	}
	h.cursor.InsertString(capitalized)
	return true
}

// capitalize upper-cases the first letter of s and lower-cases the rest,
// matching Emacs capitalize-word.
func capitalize(s string) string {
	upped := false
	return strings.Map(func(r rune) rune {
		if !upped {
			upped = true
			return unicode.ToUpper(r)
		}
		return unicode.ToLower(r)
	}, s)
}

func (h *emacsHandler) Handle(ev term.Event) (exit, handled bool) {
	ctx := context.Background()

	// Track whether the current event is a paste-related action.
	// Reset lastPaste at the end unless the handler explicitly sets it.
	pastedThisTurn := false
	defer func() {
		if !pastedThisTurn {
			h.lastPaste = false
			h.historyIdx = 0
		}
	}()

	// only a user event clears a pending set cursor
	h.pendingSetCursor = nil

	switch ev.Type {
	case term.EventMouse:
		return h.mouse.Handle(ev)
	case term.EventPasteStart:
		h.pasteBuf.Reset()
		h.pasteStarted = true
		handled = true
		return
	case term.EventPasteEnd:
		str := h.pasteBuf.String()
		if _, ok := h.cursor.SelectionMode(); ok {
			handled = h.cursor.DeleteSelection()
			h.cursor.Unselect()
		} else {
			h.cursor.InsertString(str)
			handled = true
		}
		h.pasteStarted = false
		return
	case term.EventKey:
	default:
		return
	}

	if h.pasteStarted {
		if ev.Ch != 0 {
			h.pasteBuf.WriteRune(ev.Ch)
			handled = true
		}
		return
	}

	if ev.Mod == term.ModShift {
		switch ev.Key {
		case term.KeyTab:
			if _, ok := h.cursor.SelectionMode(); ok {
				if handled = h.cursor.ShiftSelectionLeft(h.cfg.indentRune, h.cfg.indentTabspaces); handled {
					h.cursor.Unselect()
				}
			} else {
				handled = h.cursor.ShiftLineLeft(h.cfg.indentRune, h.cfg.indentTabspaces)
			}
			return
		}
	}

	var shift bool
	if ev.Mod&term.ModShift != 0 && ev.Mod == term.ModShift {
		if _, ok := h.cursor.SelectionMode(); !ok {
			h.cursor.Select()
		}
		ev.Mod = ev.Mod &^ term.ModShift
		shift = true
	}

	cursorAt := h.cursor.CursorAtScroll()
	switch ev.Mod {
	case term.ModAltShift:
		switch ev.Key {
		case term.KeyArrowDown:
			handled = h.duplicateLine(false /* down */)
		case term.KeyArrowUp:
			handled = h.duplicateLine(true /* up */)
		}
	// <alt> is authentic Emacs Meta (Option on macOS reaches the GUI as
	// ModAlt). The editor owns the real M- editing chords here.
	case term.ModAlt:
		switch ev.Key {
		case term.KeyArrowDown:
			handled = h.moveLine(false /* down */)
		case term.KeyArrowUp:
			handled = h.moveLine(true /* up */)
		case term.KeyArrowLeft:
			handled = h.cursor.MoveLeftStartWord()
		case term.KeyArrowRight:
			handled = h.cursor.MoveRightEndWord()
		case term.KeyBackspace:
			// M-DEL: backward-kill-word.
			h.cursor.Select()
			h.cursor.MoveLeftStartWord()
			handled = h.cursor.DeleteSelection()
		case term.KeyDelete:
			// M-Delete: kill-word (forward).
			h.cursor.Select()
			h.cursor.MoveRightEndWord()
			handled = h.cursor.DeleteSelection()
		case 0:
			switch ev.Ch {
			case 'f':
				handled = h.cursor.MoveRightEndWord()
			case 'b':
				handled = h.cursor.MoveLeftStartWord()
			case 'd':
				// M-d: kill-word (forward).
				h.cursor.Select()
				h.cursor.MoveRightEndWord()
				handled = h.cursor.DeleteSelection()
			case 'w':
				// M-w: kill-ring-save (copy the region).
				handled = h.copyRegion()
			case ',':
				// M-<: beginning-of-buffer.
				handled = h.cursor.MoveFirstLine()
			case '.':
				// M->: end-of-buffer.
				handled = h.cursor.MoveLastLine()
			case 'm':
				// M-m: back-to-indentation.
				handled = h.cursor.MoveStartLineNonBlank()
			case '^':
				// M-^: delete-indentation. Conflate joins the following line
				// onto the current one (bracket-free line join); it does not
				// implement GNU Emacs join-with-previous semantics.
				handled = h.cursor.Conflate()
			case '\\':
				// M-\: delete-horizontal-space.
				handled = h.cursor.DeleteHorizontalSpace()
			case 'u':
				handled = h.upcaseWord()
				return
			case 'l':
				handled = h.downcaseWord()
				return
			case 'c':
				handled = h.capitalizeWord()
				return
			case ';':
				// M-;: comment-dwim (toggle line comment).
				handled = h.cursor.ToggleLineComment()
			case 'y':
				// M-y: yank-pop (replace the last yank with an older kill).
				if handled = h.pasteFromHistory(); handled {
					pastedThisTurn = true
				}
			case 'q':
				// M-q: fill-paragraph.
				handled = h.cursor.WrapParagraph(h.cfg.ruler)
			case '{':
				handled = h.cursor.CollapseFold(context.Background())
			case '}':
				handled = h.cursor.ExpandFold(context.Background())
			}
		}
	// no modifier
	case 0:
		switch ev.Key {
		case term.KeyEsc:
			handled = h.cursor.Unselect()
			h.cursor.Search("")
		case term.KeyEnd:
			handled = h.cursor.MoveEndLine()
		case term.KeyHome:
			handled = h.cursor.MoveStartLine()
		case term.KeyPgup:
			handled = h.cursor.MoveUpLines(h.less.Scroll().SizeHeight())
			if handled {
				h.cursor.RepositionBottom()
			}
		case term.KeyPgdn:
			handled = h.cursor.MoveDownLines(h.less.Scroll().SizeHeight())
			if handled {
				h.cursor.RepositionTop()
			}
		case term.KeyArrowLeft:
			handled = h.cursor.MoveLeft()
		case term.KeyArrowRight:
			handled = h.cursor.MoveRight()
		case term.KeyArrowUp:
			handled = h.cursor.MoveUp()
		case term.KeyArrowDown:
			handled = h.cursor.MoveDown()
		case term.KeyEnter:
			if _, ok := h.cursor.SelectionMode(); ok {
				h.cursor.DeleteSelection()
				h.cursor.Unselect()
			}
			if h.cfg.autoPair {
				h.cursor.InsertAutoPairNewline(h.cfg.indentRune, h.cfg.indentTabspaces)
			} else {
				h.cursor.InsertWithIndentRune('\n', h.cfg.indentRune, h.cfg.indentTabspaces)
			}
			handled = true
		case term.KeySpace:
			if _, ok := h.cursor.SelectionMode(); ok {
				h.cursor.DeleteSelection()
				h.cursor.Unselect()
			}
			h.cursor.InsertWithIndentRune(' ', h.cfg.indentRune, h.cfg.indentTabspaces)
			handled = true
		case term.KeyTab:
			if _, ok := h.cursor.SelectionMode(); ok {
				h.cursor.ShiftSelectionRight(h.cfg.indentRune, h.cfg.indentTabspaces)
				h.cursor.Unselect()
			} else {
				if !h.cursor.TryIndent(h.cfg.indentRune, h.cfg.indentTabspaces) {
					h.cursor.InsertWithIndentRune(
						h.cfg.indentRune, h.cfg.indentRune, h.cfg.indentTabspaces)
				}
			}
			handled = true
		case term.KeyBackspace:
			if _, ok := h.cursor.SelectionMode(); ok {
				handled = h.cursor.DeleteSelection()
				h.cursor.Unselect()
			} else if h.cfg.autoPair {
				handled = h.cursor.BackspaceAutoPair()
			} else {
				handled = h.cursor.Backspace()
			}
		case term.KeyDelete:
			if _, ok := h.cursor.SelectionMode(); ok {
				handled = h.cursor.DeleteSelection()
				h.cursor.Unselect()
			} else {
				handled = h.cursor.Delete()
			}
		default:
			if ev.Ch != 0 {
				if _, ok := h.cursor.SelectionMode(); ok {
					handled = h.cursor.DeleteSelection()
					h.cursor.Unselect()
				} else if h.cfg.autoPair {
					handled = h.cursor.InsertWithAutoPair(ev.Ch, h.cfg.indentRune, h.cfg.indentTabspaces)
				} else {
					h.cursor.InsertWithIndentRune(ev.Ch, h.cfg.indentRune, h.cfg.indentTabspaces)
					handled = true
				}
			}
		}
	case term.ModCtrl:
		switch ev.Key {
		case term.KeyEnter:
			h.cursor.InsertLineBelow(h.cfg.indentRune, h.cfg.indentTabspaces)
			handled = true
			return
		case term.KeySpace:
			// C-SPC: set-mark-command.
			handled = h.setMarkLocation()
			return
		}
		switch ev.Ch {
		case 'c':
			_, err := h.cursor.CopySelection(clipboard.DefaultRegisterID, h.clipboard)
			if err != nil {
				h.log(log.ErrorLevel, "cursor copy selection: %v", err)
			}
			handled = true
		case 'y':
			// C-y: yank (paste from the clipboard).
			paste, err := h.clipboard.Paste(clipboard.DefaultRegisterID)
			if err != nil {
				h.log(log.ErrorLevel, "clipboard paste: %v", err)
			} else {
				mode, _ := paste.Metadata.(text.SelectMode)
				h.cursor.Paste(paste.Text, mode, false)
				handled = true
				// Prime the yank-pop cycle so a following M-y replaces
				// this yank with an older kill-ring entry.
				h.lastPaste = true
				h.historyIdx = 0
				pastedThisTurn = true
			}
		case 'l':
			handled = h.cursor.Center()
		case 'v':
			handled = h.less.Scroll().SeekDownPage()
		case 'g':
			// C-g: keyboard-quit. Clear selection, search highlight and any
			// pending mark.
			handled = h.cursor.Unselect()
			h.cursor.Search("")
			h.clearMarkLocation()
		case 'o':
			// C-o: open-line. Insert a newline but keep point before it.
			mark := h.cursor.Mark()
			h.cursor.InsertWithIndentRune('\n', h.cfg.indentRune, h.cfg.indentTabspaces)
			h.cursor.MoveToMark(mark)
			handled = true
		case 'j':
			// C-j: newline-and-indent.
			h.cursor.InsertWithIndentRune('\n', h.cfg.indentRune, h.cfg.indentTabspaces)
			handled = true
		case 'm':
			handled = h.cursor.MoveToMatchingRune()
		case 'M':
			handled = h.cursor.SelectABlockClose('(', ')') ||
				h.cursor.SelectABlockClose('{', '}') ||
				h.cursor.SelectABlockClose('[', ']')
		case 'd':
			handled = h.cursor.Delete()
		case 'h':
			handled = h.cursor.Backspace()
		case 'a':
			handled = h.cursor.MoveStartLine()
		case 'p':
			handled = h.cursor.MoveUp()
		case 'n':
			handled = h.cursor.MoveDown()
		case 'q':
			if h.macroRecorder != nil {
				if h.macroRecorder.IsRecording() {
					h.macroRecorder.Stop()
				} else {
					h.macroRecorder.Start(registerset.UnnamedRegisterID)
				}
				handled = true
			}
		case 'Q':
			handled = h.playMacro()
		case 'e':
			handled = h.cursor.MoveEndLine()
		case 'z':
			handled = h.cursor.Undo()
		case 'Z':
			handled = h.cursor.ToggleFold(ctx)
		case 'A':
			handled = h.cursor.ToggleAllFolds(ctx)
		case 'f':
			handled = h.cursor.MoveRight()
		case 'b':
			handled = h.cursor.MoveLeft()
		case 'k':
			h.cursor.Select()
			h.cursor.MoveEndLine()
			handled = h.cursor.DeleteSelection()
		case '-':
			handled = h.cursor.MoveToNextMatch()
		case '_':
			handled = h.cursor.MoveToPrevMatch()
		case 't':
			if h.cursor.Select() {
				handled, _ = h.cursor.CopySelectionNoUnselect(
					clipboard.DefaultRegisterID, h.clipboard)
				if handled {
					h.cursor.DeleteSelection()
					paste, err := h.clipboard.Paste(clipboard.DefaultRegisterID)
					if err != nil {
						h.log(log.ErrorLevel, "clipboard paste: %v", err)
					} else {
						str := paste.Text
						h.cursor.Paste(str, text.StandardSelection, false)
					}
				}
			}
		case 'K':
			if h.cursor.SelectLine() {
				handled = h.cursor.DeleteSelection()
			}
		case 'w':
			// C-w: kill-region (mark to point).
			handled = h.killRegion()
		case '=':
			// C-= : expand-region (grow the syntactic selection).
			handled = h.cursor.ExpandSelection(ctx)
		}
	case term.ModCtrlShift:
		switch ev.Key {
		case term.KeyEnter:
			h.cursor.InsertLineAbove(h.cfg.indentRune, h.cfg.indentTabspaces)
			handled = true
			return
		}
		switch ev.Ch {
		case 'Q':
			handled = h.playMacro()
		case 'M':
			handled = h.cursor.SelectABlockClose('(', ')') ||
				h.cursor.SelectABlockClose('{', '}') ||
				h.cursor.SelectABlockClose('[', ']')
		case 'W':
			handled = h.cursor.ShrinkSelection()
		}
	case term.ModCtrlAlt:
		switch ev.Key {
		case term.KeyArrowUp:
			pos := h.cursor.CursorAtScroll()
			if handled = h.less.Scroll().SeekUp(); handled {
				win, ok := h.cursor.WindowCoordinates(pos)
				if ok && win.Y >= 0 {
					h.cursor.MoveToScroll(pos)
				}
			}
		case term.KeyArrowDown:
			pos := h.cursor.CursorAtScroll()
			if handled = h.less.Scroll().SeekDown(); handled {
				win, ok := h.cursor.WindowCoordinates(pos)
				if ok && win.Y >= 0 {
					h.cursor.MoveToScroll(pos)
				}
			}
		case 0:
			switch ev.Ch {
			case 'h':
				handled = h.cursor.HideSelection()
				h.cursor.Unselect()
			case 'v':
				handled = h.cursor.Unhide()
				h.cursor.Unselect()
			case 'f':
				// C-M-f: forward-sexp (bracket-based).
				handled = h.moveForwardSexp()
			case 'b':
				// C-M-b: backward-sexp (bracket-based).
				handled = h.moveBackwardSexp()
			}
		}
	}

	// if moved and shift is not pressed
	if cursorAt != h.cursor.CursorAtScroll() && !shift {
		h.cursor.Unselect()
		h.cursor.Search("")
	}
	return
}

// Cursor satisfies tui.Handler
func (h *emacsHandler) Cursor() (
	pos term.Coordinates, style term.CursorStyle, show bool,
) {
	return h.cursor.Coordinates(), term.CursorStyleSteadyBar, true
}

// Selection satisfies tui.Handler.
func (h *emacsHandler) Selection() (string, bool) {
	text := h.cursor.Selection()
	return text, text != ""
}

// SelectionBounds returns the unsorted (anchor, cursor) buffer
// coordinates of the active selection, if any. ok is false when no
// selection is active. Hosts that render the buffer outside the
// editor's Draw pipeline use this to paint the selection highlight
// in their own coordinate system.
func (h *emacsHandler) SelectionBounds() (from, to term.Coordinates, ok bool) {
	return h.cursor.SelectionBounds()
}

// Close satisfies editor.Handler.
func (h *emacsHandler) Close() error {
	return nil
}

// Resource satisfies editor.Handler.
func (h *emacsHandler) Resource() workspaceapi.URI {
	return h.resource
}

// SetWrap satisfies editor.Handler.
func (h *emacsHandler) SetWrap(wrap bool) {
	h.less.Scroll().Wrap = wrap
}

func (h *emacsHandler) IsSearchMode() bool {
	return h.less.Mode() == handler.LessSearchMode
}

func (t *emacsHandler) ShowCommandBar(show bool) {
	t.less.ShowCommandBar(show)
}

func (h *emacsHandler) SetCursorAtScroll(pos term.Coordinates) bool {
	// Clamp to buffer bounds so a stale or overshooting position (for
	// example after the buffer shrank underneath the cursor) cannot leave
	// the cursor pointing past the last row or column. This mirrors the
	// modal handler and prevents out-of-range panics in downstream edits.
	pos.Y = max(0, min(pos.Y, h.less.Buffer().Rows()-1))
	pos.X = max(0, min(pos.X, h.less.Buffer().Columns(pos.Y)))
	// setCursor should be robust against resizes, etc.
	// only the first client interaction should clear this position
	if h.less.Scroll().Width() == 0 || h.less.Scroll().SizeHeight() == 0 {
		h.pendingSetCursor = new(term.Coordinates)
		*h.pendingSetCursor = pos
		return false
	}

	_, ok := h.cursor.MoveToScroll(pos)
	if !ok || !h.cfg.autoCenter {
		return ok
	}
	h.cursor.Center()
	return true
}

func (h *emacsHandler) SeekUp() bool {
	return h.less.Scroll().SeekUp()
}

func (h *emacsHandler) SeekDown() bool {
	return h.less.Scroll().SeekDown()
}

func (h *emacsHandler) SeekOffset() int {
	return h.less.Scroll().SeekOffset()
}

func (h *emacsHandler) MaxSeekOffset() int {
	return h.less.Scroll().MaxSeekOffset()
}

func (h emacsHandler) SetLocationList(
	pri textapi.LocationPriority, ID string, loc text.LocationList,
) {
	h.cursor.SetLocationList(pri, ID, loc)
}

func (h *emacsHandler) MoveToNextLocation(ID string) bool {
	return h.cursor.MoveToNextLocation(ID)
}

func (h *emacsHandler) MoveToPrevLocation(ID string) bool {
	return h.cursor.MoveToPrevLocation(ID)
}

func (h *emacsHandler) CellView() cell.View {
	return h.buf.View()
}

func (h *emacsHandler) CellEditor() cell.Editor {
	return text.ExternalEditor(&h.cursor, h.buf.Editor())
}

// Dimensions satisfies text.Handler (and component.Floating).
// emacsHandler is a leaf handler with no sidebar chrome, so the
// ideal size is the widest row in the underlying buffer by the
// buffer row count. Bar-wrapping handlers add their own
// contribution on top. See text.ViewDimensions for why this must
// sum each cell's Width rather than use View.Columns.
func (h *emacsHandler) Dimensions() (int, int) {
	return text.ViewDimensions(h.buf.View())
}

func (e *emacsHandler) SetDefaultAttributes(attr term.Attributes) {
	e.less.Scroll().Attributes = attr
}

func (h *emacsHandler) CursorAtScroll() term.Coordinates {
	return h.cursor.CursorAtScroll()
}

func (h *emacsHandler) LocationLists() []text.LocationSet {
	return h.cursor.LocationLists()
}

func (h *emacsHandler) moveLine(up bool) (handled bool) {
	if _, ok := h.cursor.SelectionMode(); !ok {
		if !h.cursor.SelectLine() {
			return
		}
	}
	handled, _ = h.cursor.CopySelectionNoUnselect(
		clipboard.DefaultRegisterID, h.clipboard)
	if !handled {
		return
	}
	h.cursor.DeleteSelection()
	if up {
		h.cursor.MoveUp()
	} else {
		h.cursor.MoveDown()
	}
	h.cursor.MoveStartLine()
	paste, err := h.clipboard.Paste(clipboard.DefaultRegisterID)
	if err != nil {
		h.log(log.ErrorLevel, "clipboard paste: %v", err)
	} else {
		str := paste.Text
		curr := h.cursor.CursorAtScroll()
		h.cursor.Paste(str, text.StandardSelection, false)
		if up {
			curr.Y--
		}
		h.cursor.MoveToScroll(curr)
	}
	return
}

func (h *emacsHandler) duplicateLine(up bool) (handled bool) {
	if _, ok := h.cursor.SelectionMode(); !ok {
		if !h.cursor.SelectLine() {
			return
		}
	}
	handled, _ = h.cursor.CopySelection(
		clipboard.DefaultRegisterID, h.clipboard)
	if !handled {
		return
	}
	if !up {
		h.cursor.MoveDown()
	}
	h.cursor.MoveStartLine()
	paste, err := h.clipboard.Paste(clipboard.DefaultRegisterID)
	if err != nil {
		h.log(log.ErrorLevel, "clipboard paste: %v", err)
	} else {
		str := paste.Text
		curr := h.cursor.CursorAtScroll()
		h.cursor.Paste(str, text.StandardSelection, false)
		h.cursor.MoveToScroll(curr)
	}
	return
}

// pasteFromHistory pastes from clipboard history. If the last action was a
// paste, it replaces that paste with the next older history entry.
func (h *emacsHandler) pasteFromHistory() (handled bool) {
	history, ok := registerhistory.AsHistory(h.clipboard)
	if !ok {
		return h.lastPaste
	}
	next := 0
	if h.lastPaste {
		next = h.historyIdx + 1
	}
	if next >= history.HistoryLen() {
		return h.lastPaste
	}
	data, ok := history.HistoryAt(next)
	if !ok {
		return h.lastPaste
	}
	if h.lastPaste {
		// Undo the previous paste before replacing it with an older entry.
		h.cursor.Undo()
	}
	mode, _ := data.Metadata.(text.SelectMode)
	h.cursor.Paste(data.Text, mode, false)
	h.historyIdx = next
	h.lastPaste = true
	handled = true
	return
}

func (h *emacsHandler) setStatusBar(bar statusBar) {
	h.statusBar = bar
}

func (h *emacsHandler) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "emacs.handler").Logf(level, msg, args...)
}

var _ foldsService = (*syntax.Tree)(nil)

type foldsService interface {
	FoldsFrom(pos term.Coordinates) (iterator.Iterator[term.Range], bool)
	Folds() (iterator.Iterator[term.Range], bool)
	InitialFolds() (iterator.Iterator[term.Range], bool)
}

func (h *emacsHandler) hideInitialFolds() {
	svc, ok := h.less.Buffer().View().(foldsService)
	if !ok {
		h.log(log.DebugLevel, "folds service not available for resource: %s", h.resource)
		return
	}
	folds, ok := svc.InitialFolds()
	if !ok {
		h.log(log.DebugLevel, "initial folds returned false")
		return
	}

	scroll := h.less.Scroll()

	go debug.CapturePanicReport(func() {
		folds, isEmpty := iterator.IsEmpty(context.Background(), folds)
		if isEmpty {
			folds.Close()
			return
		}
		h.cfg.scheduleNextTick(func() {
			defer folds.Close()
			cursor := h.CursorAtScroll()
			for {
				fold, ok := folds.Next(context.Background())
				if !ok {
					break
				}
				scroll.MarkHidden(fold.Start.Y, fold.End.Y)
			}
			if err := folds.Err(); err != nil {
				h.log(log.ErrorLevel, "error hiding initial folds: %v", err)
			}
			h.SetCursorAtScroll(cursor)
		})
	})
}
