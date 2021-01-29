package vi

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
)

type viMode uint8
type moveMode uint8

const (
	normalMode viMode = iota
	insertMode
	visualMode
	visualLineMode
	visualBlockMode
	moveToCharMode
	replaceMode
	replaceOneMode
)

const (
	moveToNext moveMode = iota
	moveToPrev
	// moveToNextPad
	// moveToPrevPad
)

func moveOpposite(m moveMode) moveMode {
	switch m {
	case moveToNext:
		return moveToPrev
	case moveToPrev:
		return moveToNext
	default:
		panic("not a known moving mode")
	}
}

// Vi implements a basic vi-like text editor which satisfies tui.Handler
// and tui.Component.
type Vi struct {
	config      viConfig
	less        handler.Less // used for message bar and text search capabilities
	raw, cursor editor.Cursor
	mode        viMode
	moveMode    moveMode
	searchMode  moveMode
	moveChar    rune
}

// DefaultViConfig is a sane configuration defaults for Vi.
var defaultViConfig = viConfig{
	resAttr: term.Attributes{
		Fg: term.AttrReverse,
		Bg: term.ColorDefault,
	},
	clipboard: editor.NewEphemeralClipboard(),
}

// New allocates storage for a new Vi handler, initializes it and returns it.
func New(buf *cell.Buffer, opts ...Option) *Vi {
	vi := new(Vi)
	vi.Init(buf, opts...)
	return vi
}

// Init initialies this vi handle with a new Buffer.
func (vi *Vi) Init(buf *cell.Buffer, opts ...Option) {
	vi.config = defaultViConfig
	for _, o := range opts {
		o(&vi.config)
	}

	vi.less.Scroll.ResultsAttr = vi.config.resAttr
	vi.less.InitWithBuffer(buf)
	vi.cursor.Init(&vi.less.Scroll)

	editor.WithCopyDelete(vi.config.clipboard, buf)

	vi.raw = vi.cursor

	vi.setNormalMode()
}

// Resize : tui.Component
func (vi *Vi) Resize(width, height int) {
	vi.less.Resize(width, height)
}

func (vi *Vi) setActiveLocationListMessage(locs map[string]editor.Location) {
	// TODO implement active location list
	for _, loc := range locs {
		vi.SetMessage(loc.Message)
		return
	}
}

// Draw : tui.Component
func (vi *Vi) Draw(w term.Writer) {
	locs, ok := vi.cursor.Locations()
	if ok {
		vi.setActiveLocationListMessage(locs)
	} else {
		vi.setMode(vi.mode)
	}
	vi.less.Draw(w)
}

// Man : tui.Handler
func (vi *Vi) Man() tui.Manual {
	panic("TODO")
}

// Cursor : tui.Handler
func (vi *Vi) Cursor() (term.Coordinates, bool) {
	// use less Cursor if we are in search mode
	if vi.less.Mode() != handler.LessNormalMode {
		return vi.less.Cursor()
	}
	return vi.cursor.Cursor()
}

// SetMessage uses vi's configured Messenger to set msg with args.
func (vi *Vi) SetMessage(msg string, args ...interface{}) {
	if vi.config.messenger != nil {
		vi.config.messenger.SetMessage(msg, args...)
		return
	}
	vi.less.SetMessage(msg, args...)
}

func (vi *Vi) setMode(mode viMode) {
	var text string
	switch mode {
	case normalMode:
		text = "NORMAL"
	case insertMode:
		text = "INSERT"
	case visualMode:
		text = "VISUAL"
	case visualLineMode:
		text = "V-LINE"
	case visualBlockMode:
		text = "V-BLOCK"
	case moveToCharMode:
		text = "MOVE-TO"
	case replaceMode:
		text = "REPLACE"
	case replaceOneMode:
		text = "NORMAL"
	default:
		panic(fmt.Sprintf("unknown mode: %v", mode))
	}
	vi.less.SetMessageAlt(":")
	vi.less.SetMessage(text)
	vi.mode = mode
}

func (vi *Vi) setNormalMode() {
	vi.setMode(normalMode)
	vi.less.SetMessageAlt(":")
}

func (vi *Vi) setInsertMode() {
	vi.setMode(insertMode)
}

func (vi *Vi) setVisualMode() {
	if vi.cursor.Select() {
		vi.setMode(visualMode)
	}
}

func (vi *Vi) setVisualLineMode() {
	if vi.cursor.SelectLine() {
		vi.setMode(visualLineMode)
	}
}

func (vi *Vi) setVisualBlockMode() {
	if vi.cursor.SelectBlock() {
		vi.setMode(visualBlockMode)
	}
}

func (vi *Vi) setMoveToCharacterMode(mode moveMode) {
	vi.setMode(moveToCharMode)
	vi.moveMode = mode
}

func (vi *Vi) setReplaceMode() {
	vi.setMode(replaceMode)
}

func (vi *Vi) setReplaceOneMode() {
	vi.setMode(replaceOneMode)
}

// we delegate search buffer Component to Less but delegate cursor position
// and results seeking to Editor so this function makes sure that we only
// perform the search once, at the same time we delegate the right logic to
// Editor and Less.
func (vi *Vi) handleSearch(ev term.Event) (bool, bool) {
	switch ev.Key {
	case term.KeyEnter:
		text := vi.less.SearchText()
		vi.less.SetNormalMode()
		vi.cursor.Search(text)
		return false, true
	default:
		return vi.less.Handle(ev)
	}
}

func (vi *Vi) insertBlock(str string) {
	reader := bufio.NewReader(strings.NewReader(str))
	for {
		str, err := reader.ReadString('\n')
		if err == nil && len(str) > 0 {
			str = str[:len(str)-1]
		}
		cur, _ := vi.cursor.Cursor()
		vi.cursor.InsertString(str)
		if err != nil {
			break
		}
		vi.cursor.MoveTo(cur)
		vi.cursor.MoveDown()
	}
}

func (vi *Vi) logError(err error) {
	if vi.config.logger == nil {
		return
	}
	vi.config.logger.Error(err)
}

func (vi *Vi) pasteClipboard(before bool) bool {
	paste, err := vi.config.clipboard.Get()
	str := paste.Data
	mode, ok := paste.Metadata.(viMode)
	if !ok {
		mode = visualMode
	}
	if err != nil {
		vi.logError(fmt.Errorf("clipboard.Get: %s", err))
		return false
	}

	cur, _ := vi.cursor.Cursor()

	switch mode {
	case visualMode:
		if !before {
			vi.cursor.MoveRight()
			vi.cursor.InsertString(str)
			vi.cursor.MoveTo(cur)
			vi.cursor.MoveRight()
		} else {
			vi.cursor.InsertString(str)
			vi.cursor.MoveTo(cur)
		}
	case visualLineMode:
		if !before {
			vi.cursor.InsertRowBelow()
			vi.cursor.MoveStartLine()
			vi.cursor.InsertString(str)
			vi.cursor.MoveTo(cur)
			vi.cursor.MoveDown()
			vi.cursor.MoveStartLine()
		} else {
			vi.cursor.MoveUp()
			vi.cursor.InsertRowBelow()
			vi.cursor.MoveStartLine()
			vi.cursor.InsertString(str)
			vi.cursor.MoveTo(cur)
			vi.cursor.MoveStartLine()
		}
	case visualBlockMode:
		if !before {
			vi.cursor.MoveRight()
			vi.insertBlock(str)
			vi.cursor.MoveTo(cur)
			vi.cursor.MoveRight()
		} else {
			vi.insertBlock(str)
			vi.cursor.MoveTo(cur)
		}
	}
	return true
}

func (vi *Vi) handleNormal(ev term.Event) (quit bool, handled bool) {
	switch ev.Type {
	case term.EventKey:
		handled = true
		switch ev.Ch {
		case 'R':
			vi.setReplaceMode()
		case 'r':
			vi.setReplaceOneMode()
		case '>':
			vi.cursor.ShiftLineRight()
		case '<':
			vi.cursor.ShiftLineLeft()
		case ',':
			mode := moveOpposite(vi.moveMode)
			event := term.Event{Type: term.EventKey, Ch: vi.moveChar}
			vi.handleMoveToCharacter(mode, event)
		case ';':
			event := term.Event{Type: term.EventKey, Ch: vi.moveChar}
			vi.handleMoveToCharacter(vi.moveMode, event)
		case 'f':
			vi.setMoveToCharacterMode(moveToNext)
		case 'F':
			vi.setMoveToCharacterMode(moveToPrev)
		case 'N':
			switch vi.searchMode {
			case moveToNext:
				vi.cursor.MoveToPrevMatch()
			case moveToPrev:
				vi.cursor.MoveToNextMatch()
			}
		case 'n':
			switch vi.searchMode {
			case moveToNext:
				vi.cursor.MoveToNextMatch()
			case moveToPrev:
				vi.cursor.MoveToPrevMatch()
			}
		case 'p':
			vi.pasteClipboard(false)
		case 'P':
			vi.pasteClipboard(true)
		case '0':
			vi.cursor.MoveStartLine()
		case '$':
			vi.cursor.MoveEndLine()
		case 'g':
			vi.cursor.MoveFirstLine()
		case 'G':
			vi.cursor.MoveLastLine()
		case 'j':
			vi.raw.MoveDown()
			vi.cursor = vi.raw
		case 'k':
			vi.raw.MoveUp()
			vi.cursor = vi.raw
		case 'h':
			vi.cursor.MoveLeft()
		case 'l':
			vi.cursor.MoveRight()
		case 'O':
			vi.setInsertMode()
			vi.cursor.InsertRowAbove()
		case 'o':
			vi.setInsertMode()
			vi.cursor.InsertRowBelow()
		case 'i':
			vi.setInsertMode()
		case 'I':
			vi.cursor.MoveStartLine()
			vi.setInsertMode()
		case 'J':
			vi.cursor.Conflate()
		case 'a':
			vi.cursor.MoveRight()
			vi.setInsertMode()
		case 'A':
			vi.setInsertMode()
			vi.cursor.MoveEndLine()
			vi.cursor.MoveRight()
		case 'C':
			vi.setInsertMode()
			fallthrough
		case 'D':
			if vi.cursor.Select() {
				vi.cursor.MoveEndLine()
				vi.cursor.DeleteSelection()
			}
		case 'x':
			vi.cursor.Delete()
		case 's':
			vi.cursor.Delete()
			vi.setInsertMode()
		case 'v':
			vi.setVisualMode()
		case 'V':
			vi.setVisualLineMode()
		case 'w':
			vi.cursor.MoveRightStartWord()
		case 'u':
			vi.cursor.Undo()
		case 'e':
			vi.cursor.MoveRightEndWord()
		case 'b':
			vi.cursor.MoveLeftStartWord()
		case '?':
			vi.searchMode = moveToPrev
			ev.Ch = '/'
			vi.less.Handle(ev)
		case '/':
			vi.searchMode = moveToNext
			vi.less.Handle(ev)
		case '%':
			vi.cursor.MoveToMatchingRune()
		default:
			switch ev.Key {
			case term.KeyCtrlR:
				vi.cursor.Redo()
			case term.KeyCtrlV:
				vi.setVisualBlockMode()
			default:
				handled = false
			}
		}
	}

	return
}

func (vi *Vi) handleInsert(ev term.Event) (quit, handled bool) {
	handled = true
	switch ev.Key {
	case term.KeyEnter:
		vi.cursor.Insert('\n')
	case term.KeySpace:
		vi.cursor.Insert(' ')
	case term.KeyTab:
		vi.cursor.Insert('\t')
	case term.KeyBackspace, term.KeyBackspace2:
		vi.cursor.Backspace()
	case term.KeyEsc:
		vi.cursor.MoveLeft()
		vi.setNormalMode()
	default:
		if ev.Ch != 0 {
			vi.cursor.Insert(ev.Ch)
		} else {
			handled = false
		}
	}
	return
}

func (vi *Vi) handleVisual(ev term.Event) (quit, handled bool) {
	if ev.Key == term.KeyEsc {
		vi.setNormalMode()
		vi.cursor.Unselect()
		handled = true
		return
	}

	if ev.Type == term.EventKey {
		handled = true
		switch ev.Ch {
		case '>':
			vi.cursor.ShiftSelectionRight()
		case '<':
			vi.cursor.ShiftSelectionLeft()
		case 'y':
			selection := vi.cursor.Selection()
			vi.config.clipboard.Set(editor.Paste{Data: selection, Metadata: vi.mode})
			vi.cursor.Unselect()
			vi.setNormalMode()
		case 'd', 'x':
			vi.cursor.DeleteSelection()
			vi.setNormalMode()
		case 's', 'c':
			vi.cursor.DeleteSelection()
			vi.setInsertMode()
		default:
			handled = false
		}
	}

	if !handled {
		quit, handled = vi.handleNormal(ev)
	}
	return
}

func (vi *Vi) handleMoveToCharacter(mode moveMode, ev term.Event) (bool, bool) {
	switch ev.Type {
	case term.EventKey:
		switch mode {
		case moveToNext:
			vi.cursor.MoveToNextChar(ev.Ch)
		case moveToPrev:
			vi.cursor.MoveToPrevChar(ev.Ch)
		}
		vi.moveChar = ev.Ch
		vi.setNormalMode()
	default:
		vi.setNormalMode()
	}
	return false, true
}

func (vi *Vi) handleReplace(ev term.Event) (quit, handled bool) {
	if ev.Type != term.EventKey {
		return
	}

	handled = true

	switch ev.Key {
	case term.KeyEnter:
		ev.Ch = '\n'
	case term.KeySpace:
		ev.Ch = ' '
	case term.KeyTab:
		ev.Ch = '\t'
	case term.KeyBackspace, term.KeyBackspace2:
		vi.cursor.MoveLeft()
	case term.KeyEsc:
		vi.cursor.MoveLeft()
		vi.setNormalMode()
	}

	if ev.Ch != 0 {
		// do not delete column == len(row); it contains a newline
		// and that would conflate the current row with the next
		if vi.cursor.Column() < vi.less.Scroll.Buffer().Columns(vi.cursor.Row()) {
			vi.cursor.Delete()
		}
		vi.cursor.Insert(ev.Ch)
	}
	return
}

// Handle : tui.Handler
func (vi *Vi) Handle(ev term.Event) (quit, handled bool) {
	switch vi.mode {
	case normalMode:
		if vi.less.Mode() != handler.LessNormalMode {
			quit, handled = vi.handleSearch(ev)
		} else {
			quit, handled = vi.handleNormal(ev)
		}
	case insertMode:
		quit, handled = vi.handleInsert(ev)
	case visualMode, visualLineMode, visualBlockMode:
		quit, handled = vi.handleVisual(ev)
	case moveToCharMode:
		quit, handled = vi.handleMoveToCharacter(vi.moveMode, ev)
	case replaceMode:
		quit, handled = vi.handleReplace(ev)
	case replaceOneMode:
		quit, _ = vi.handleReplace(ev)
		handled = true
		vi.setNormalMode()
	default:
		panic(fmt.Sprintf("unknown mode: %d", vi.mode))
	}

	// copy the cursor to maintain original cursor for next vertcial move.
	vi.raw = vi.cursor

	switch vi.mode {
	case normalMode, visualMode, visualLineMode,
		visualBlockMode, moveToCharMode:
		vi.cursor.MoveToBounds(0)
		vi.cursor.MoveToNextNonNull()
	case insertMode, replaceMode, replaceOneMode:
		vi.cursor.MoveToBounds(1)
	default:
		panic(fmt.Sprintf("unknown mode: %d", vi.mode))
	}

	return
}

// MoveToNextLocation moves the cursor to the next location
// in the location list identified by ID.
func (vi *Vi) MoveToNextLocation(ID string) {
	vi.cursor.MoveToNextLocation(ID)
	vi.raw = vi.cursor
}

// MoveToPrevLocation moves the cursor to the previous location
// in the location list identified by ID.
func (vi *Vi) MoveToPrevLocation(ID string) {
	vi.cursor.MoveToPrevLocation(ID)
	vi.raw = vi.cursor
}

// SetLocationList sets a location list of this handler. See Cursor.SetLocationList
func (vi *Vi) SetLocationList(ID string, l editor.LocationList) {
	_ = vi.cursor.SetLocationList(ID, l)
}
