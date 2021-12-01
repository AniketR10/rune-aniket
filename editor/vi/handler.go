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
	deleteMode
	gMode
	yankMode
	visualMode
	visualLineMode
	visualBlockMode
	replaceMode
	replaceOneMode
)

const (
	moveNone moveMode = iota
	moveToNext
	moveToPrev
	// moveToNextPad
	// moveToPrevPad
)

// Vi implements a basic vi-like text editor which satisfies tui.Handler
// and tui.Component.
type Vi struct {
	config       viConfig
	less         handler.Less // used for message bar and text search capabilities
	free         term.Coordinates
	cursor       editor.Cursor
	repeater     editor.Repeater
	mode         viMode
	moveMode     moveMode
	searchMode   moveMode
	moveChar     rune
	deleteInsert bool
	blockRepeat  struct {
		From term.Coordinates
		To   term.Coordinates
	}
}

// DefaultViConfig is a sane configuration defaults for Vi.
var defaultViConfig = viConfig{
	resAttr: term.Attributes{
		Fg: term.AttrReverse,
		Bg: term.ColorDefault,
	},
	clipboard:       editor.NewInMemoryClipboard(),
	defaultRegister: editor.DefaultRegisterID,
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

	vi.less.InitWithBuffer(buf, handler.LessConfig{
		Wrap:    true,
		Debug:   vi.config.debug,
		ResAttr: vi.config.resAttr,
	})
	vi.cursor.Init(vi.less.Scroll())

	editor.WithCopyDelete(vi.config.defaultRegister, vi.config.clipboard, &vi.cursor, buf)
	vi.repeater.Init(&vi.cursor, buf)

	vi.free, _ = vi.cursor.Cursor()

	vi.setNormalMode()
}

// Resize : tui.Component
func (vi *Vi) Resize(width, height int) {
	vi.less.Resize(width, height)
}

func (vi *Vi) setActiveLocationListMessage(locs map[string]editor.Location) {
	// NOTE: if therea re multiple location lists with a message
	// in current cursor position, then there's no guarantee of which one
	// is going to be rendered.
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
	if vi.config.logger != nil {
		vi.config.logger.Debugf(msg, args...)
	}

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
	case deleteMode:
		text = "DELETE"
	case gMode:
		text = "NORMAL"
	case yankMode:
		text = "YANK"
	case visualMode:
		text = "VISUAL"
	case visualLineMode:
		text = "V-LINE"
	case visualBlockMode:
		text = "V-BLOCK"
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

func (vi *Vi) setNormalMode() bool {
	if vi.mode == normalMode && vi.moveMode == moveNone {
		return false
	}
	vi.setMode(normalMode)
	vi.moveMode = moveNone
	return true
}

func (vi *Vi) setInsertMode() {
	vi.blockRepeat.From = term.Coordinates{}
	vi.blockRepeat.To = term.Coordinates{}
	vi.setMode(insertMode)
}

func (vi *Vi) setDeleteMode(thenInsert bool) {
	vi.setMode(deleteMode)
	vi.deleteInsert = thenInsert
}

func (vi *Vi) setGMode() {
	vi.setMode(gMode)
}

func (vi *Vi) setYankMode() {
	vi.setMode(yankMode)
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
		vi.searchMode = moveToNext
		vi.SetMessage("searching '%s'", text)
		vi.search(text)
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

func (vi *Vi) pasteClipboard(registerID string, after bool) bool {
	paste, err := vi.config.clipboard.Paste(registerID)
	if err != nil {
		vi.logError(fmt.Errorf("clipboard.Get: %s", err))
		return false
	}

	str := paste.Text
	mode, ok := paste.Metadata.(editor.SelectMode)
	if !ok {
		mode = editor.StandardSelection
	}

	cur, _ := vi.cursor.Cursor()

	switch mode {
	case editor.StandardSelection:
		if after {
			vi.cursor.MoveRight()
			vi.cursor.InsertString(str)
		} else {
			vi.cursor.InsertString(str)
			vi.cursor.MoveTo(cur)
		}
	case editor.LineSelection:
		if after {
			vi.cursor.MoveStartLine()
			if !vi.cursor.MoveDown() {
				vi.cursor.InsertString(fmt.Sprintf("%s\n", str))
			} else {
				vi.cursor.InsertString(str)
			}
			vi.cursor.MoveTo(cur)
			vi.cursor.MoveDown()
			vi.cursor.MoveStartLine()
		} else {
			vi.cursor.MoveStartLine()
			vi.cursor.InsertString(str)
			vi.cursor.MoveTo(cur)
			vi.cursor.MoveStartLine()
		}
	case editor.BlockSelection:
		if after {
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

func (vi *Vi) handleNormal(ev term.Event) (quit, handled bool) {
	quit, handled = vi.handleMoveToCharacter(vi.moveMode, ev)
	if handled {
		return
	}

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
			event := term.Event{Type: term.EventKey, Ch: vi.moveChar}
			vi.handleMoveToCharacter(moveToPrev, event)
		case ';':
			event := term.Event{Type: term.EventKey, Ch: vi.moveChar}
			vi.handleMoveToCharacter(moveToNext, event)
		case 'f':
			vi.setMoveToCharacterMode(moveToNext)
		case 'F':
			vi.setMoveToCharacterMode(moveToPrev)
		case 'g':
			vi.setGMode()
		case 'd':
			vi.setDeleteMode(false)
		case 'c':
			vi.setDeleteMode(true)
		case 'y':
			vi.setYankMode()
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
			vi.pasteClipboard(vi.config.defaultRegister, true)
		case 'P':
			vi.pasteClipboard(vi.config.defaultRegister, false)
		case '0':
			vi.cursor.MoveStartLine()
		case '$':
			vi.cursor.MoveEndLine()
		case 'G':
			vi.cursor.MoveLastLine()
		case 'j':
			vi.cursor.MoveTo(vi.free)
			vi.cursor.MoveDown()
		case 'k':
			vi.cursor.MoveTo(vi.free)
			vi.cursor.MoveUp()
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
		case 'W':
			vi.cursor.MoveRightStartWordGroup()
		case 'u':
			vi.cursor.Undo()
		case 'e':
			vi.cursor.MoveRightEndWord()
		case 'E':
			vi.cursor.MoveRightEndWordGroup()
		case 'b':
			vi.cursor.MoveLeftStartWord()
		case 'B':
			vi.cursor.MoveLeftStartWordGroup()
		case '?':
			vi.searchMode = moveToPrev
			ev.Ch = '/'
			vi.less.Handle(ev)
		case '/':
			vi.searchMode = moveToNext
			vi.less.Handle(ev)
		case '%':
			vi.cursor.MoveToMatchingRune()
		case '#':
			vi.searchMode = moveToPrev
			vi.search(vi.cursor.Word())
		case '*':
			vi.searchMode = moveToNext
			vi.search(vi.cursor.Word())
		case '.':
			vi.repeater.Repeat()
		default:
			switch ev.Key {
			case term.KeyCtrlR:
				vi.cursor.Redo()
			case term.KeyCtrlV:
				vi.setVisualBlockMode()
			case term.KeyEsc:
				handled = vi.setNormalMode()
			default:
				handled = false
			}
		}
	}

	return
}

func (vi *Vi) search(text string) {
	vi.cursor.Search(text)
	switch vi.searchMode {
	case moveToNext:
		vi.cursor.MoveToNextMatch()
	case moveToPrev:
		vi.cursor.MoveToPrevMatch()
	default:
	}
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
		vi.repeatInsertStart()
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

func (vi *Vi) copySelection() {
	vi.cursor.CopySelection(vi.config.defaultRegister, vi.config.clipboard)
}

func (vi *Vi) repeatInsertStart() {
	from, to := cell.SortFromTo(vi.blockRepeat.From, vi.blockRepeat.To)
	n := to.Y - from.Y
	for i := 0; i < n; i++ {
		vi.blockRepeat.From.Y++
		vi.cursor.MoveToScroll(vi.blockRepeat.From)
		vi.repeater.Repeat()
	}
}

func (vi *Vi) handleVisualBlockInsertStart() {
	vi.setInsertMode()
	vi.blockRepeat.From, _ = vi.cursor.SelectionFrom()
	vi.blockRepeat.To = vi.cursor.CursorAtScroll()
	vi.SetCursorAtScroll(vi.blockRepeat.From)
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
			vi.copySelection()
			vi.setNormalMode()
		case 'd', 'x':
			vi.cursor.DeleteSelection()
			vi.setNormalMode()
		case 's', 'c':
			vi.cursor.DeleteSelection()
			vi.setInsertMode()
		case 'I':
			switch vi.mode {
			case visualBlockMode:
				vi.handleVisualBlockInsertStart()
			default:
				handled = false
			}
		default:
			handled = false
		}
	}

	if !handled {
		quit, handled = vi.handleNormal(ev)
	}

	switch vi.mode {
	case visualMode, visualLineMode, visualBlockMode:
	default:
		vi.cursor.Unselect()
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
		case moveNone:
			return false, false
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
		if vi.cursor.Column() < vi.less.Buffer().Columns(vi.cursor.Row()) {
			vi.cursor.Delete()
		}
		vi.cursor.Insert(ev.Ch)
	}
	return
}

func (vi *Vi) handleMetaNormal(ev term.Event) (quit, handled, done bool) {
	before, _ := vi.cursor.Cursor()
	vi.cursor.Select()

	prevMode := vi.moveMode
	quit, handled = vi.handleNormal(ev)
	isMoveSwitch := prevMode == moveNone && vi.moveMode != moveNone
	after, _ := vi.cursor.Cursor()

	if before == after {
		vi.cursor.Unselect()
		if !isMoveSwitch {
			handled = false
			vi.setNormalMode()
		}
		return
	}

	done = true

	// handle <op>wWeEbB idiosyncrasies
	after, _ = vi.cursor.Cursor()
	switch ev.Ch {
	case 'e', 'E':
	case 'w', 'W':
		vi.cursor.MoveLeft()
		if before.Y < after.Y {
			vi.cursor.MoveLeftEndWord()
		}
	case 'b', 'B':
		if before.Y > after.Y {
			vi.cursor.MoveLeftStartWord()
		}
	default:
		if before.Y != after.Y {
			vi.cursor.SelectLine()
		}
	}
	return
}

func (vi *Vi) handleYank(ev term.Event) (quit, handled bool) {
	if ev.Ch == 'y' {
		if vi.cursor.SelectLine() {
			vi.copySelection()
			handled = true
		}
		vi.setNormalMode()
		return
	}

	var done bool
	quit, handled, done = vi.handleMetaNormal(ev)
	if !done {
		return
	}

	vi.copySelection()
	vi.setNormalMode()
	return
}

func (vi *Vi) handleDelete(ev term.Event) (quit, handled bool) {
	if !vi.deleteInsert && vi.moveMode == moveNone && ev.Ch == 'd' {
		if vi.cursor.SelectLine() {
			vi.cursor.DeleteSelection()
		}
		vi.setNormalMode()
		handled = true
		return
	}

	if vi.deleteInsert && vi.moveMode == moveNone && ev.Ch == 'c' {
		vi.cursor.MoveStartLine()
		if vi.cursor.Select() {
			vi.cursor.MoveEndLine()
			vi.cursor.DeleteSelection()
		}
		vi.setInsertMode()
		handled = true
		return
	}

	var done bool
	quit, handled, done = vi.handleMetaNormal(ev)
	if !done {
		return
	}

	vi.cursor.DeleteSelection()
	if vi.deleteInsert {
		vi.setInsertMode()
	} else {
		vi.setNormalMode()
	}
	return
}

func (vi *Vi) handleGo(ev term.Event) (quit, handled bool) {
	defer vi.setNormalMode()

	switch ev.Ch {
	case 'g':
		vi.cursor.MoveFirstLine()
		handled = true
	default:
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
	case gMode:
		quit, handled = vi.handleGo(ev)
	case yankMode:
		quit, handled = vi.handleYank(ev)
	case deleteMode:
		quit, handled = vi.handleDelete(ev)
	case visualMode, visualLineMode, visualBlockMode:
		quit, handled = vi.handleVisual(ev)
	case replaceMode:
		quit, handled = vi.handleReplace(ev)
	case replaceOneMode:
		quit, _ = vi.handleReplace(ev)
		handled = true
		vi.setNormalMode()
	default:
		panic(fmt.Sprintf("unknown mode: %d", vi.mode))
	}

	// copy the cursor to maintain original cursor for next vertcial move
	// except when moving the cursor beyond last line
	free, _ := vi.cursor.Cursor()
	if free.Y < vi.less.Buffer().Rows() {
		vi.free = free
	}

	switch vi.mode {
	case normalMode, yankMode, gMode, deleteMode, visualMode, visualLineMode, visualBlockMode:
		if !vi.config.debug {
			vi.cursor.MoveToBounds(0)
			vi.cursor.MoveToNextNonNull()
		}
	case insertMode, replaceMode, replaceOneMode:
		if !vi.config.debug {
			vi.cursor.MoveToBounds(1)
		}
	default:
		panic(fmt.Sprintf("unknown mode: %d", vi.mode))
	}

	return
}

// MoveToNextLocation moves the cursor to the next location
// in the location list identified by ID.
func (vi *Vi) MoveToNextLocation(ID string) {
	vi.cursor.MoveToNextLocation(ID)
	vi.free, _ = vi.cursor.Cursor()
}

// MoveToPrevLocation moves the cursor to the previous location
// in the location list identified by ID.
func (vi *Vi) MoveToPrevLocation(ID string) {
	vi.cursor.MoveToPrevLocation(ID)
	vi.free, _ = vi.cursor.Cursor()
}

// SetLocationList sets a location list of this handler. See Cursor.SetLocationList
func (vi *Vi) SetLocationList(ID string, l editor.LocationList) {
	_ = vi.cursor.SetLocationList(ID, l)
}

// SetCursorAtScroll sets the cursor of this Vi handler at content pos.
func (vi *Vi) SetCursorAtScroll(pos term.Coordinates) bool {
	_, ok := vi.cursor.MoveToScroll(pos)
	vi.free, _ = vi.cursor.Cursor()
	return ok
}

// CursorAtScroll sets the cursor of this Vi handler at content pos.
func (vi *Vi) CursorAtScroll() term.Coordinates {
	return vi.cursor.CursorAtScroll()
}
