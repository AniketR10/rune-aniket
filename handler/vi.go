package handler

import (
	"fmt"
	"io"
	"strings"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/editor"
	"github.com/ernestrc/fractal/term"
	log "github.com/sirupsen/logrus"
)

type viMode uint8

const (
	normal viMode = iota
	insert
	visual
)

// ViConfig holds configuration for Vi.
type ViConfig struct {
	ResAttr   term.Attributes
	Tabspaces int
	Clipboard editor.Clipboard
	Logger    *log.Logger
}

// Vi implements a basic vi-like text editor which satisfies fractal.Handler
// and fractal.Component without ex commands.
type Vi struct {
	config      ViConfig
	less        Less // used for message bar and text search capabilities
	logger      *log.Logger
	raw, cursor editor.Cursor
	mode        viMode
}

// DefaultViConfig is a sane configuration defaults for Vi.
var DefaultViConfig = ViConfig{
	ResAttr: term.Attributes{
		Fg: term.AttrReverse,
		Bg: term.ColorDefault,
	},
	Tabspaces: 4,
	Clipboard: editor.NewEphemeralClipboard(),
	Logger:    nil,
}

// NewVi allocates storage for a new Vi handle, initializes it and returns it.
func NewVi() *Vi {
	vi := new(Vi)
	vi.Init()
	return vi
}

// WithConfig sets cfg as the new Less handler configuration.
func (vi *Vi) WithConfig(cfg ViConfig) (ret *Vi) {
	ret = new(Vi)
	*ret = *vi
	ret.config = cfg
	ret.initWithBuffer(&vi.less.Buffer)
	return ret
}

// Init initialies this vi handle with a new Buffer.
func (vi *Vi) Init() {
	vi.config = DefaultViConfig
	vi.initWithBuffer(cell.NewBuffer())
}

func (vi *Vi) initWithBuffer(buf *cell.Buffer) {
	vi.less.Scroll.ResultsAttr = vi.config.ResAttr
	vi.logger = vi.config.Logger

	if buf.Rows() != 0 {
		str := buf.String()
		buf.InitWithTabspaces(vi.config.Tabspaces)
		_, _ = buf.ReadFrom(strings.NewReader(str))
	} else {
		buf.InitWithTabspaces(vi.config.Tabspaces)
	}

	if vi.logger != nil {
		buf = buf.WithLogger(vi.logger)
	}

	vi.less.InitWithBuffer(buf)
	vi.cursor.Init(&vi.less.Scroll)
	vi.raw = vi.cursor

	vi.setNormalMode()
}

// Resize : fractal.Component
func (vi *Vi) Resize(width, height int) {
	vi.less.Resize(width, height)
}

// Draw : fractal.Component
func (vi *Vi) Draw(w fractal.Writer) {
	vi.less.Draw(w)
}

// Man : fractal.Handler
func (vi *Vi) Man() fractal.Manual {
	panic("TODO")
}

func moveInBounds(scroll *component.Scroll, cursor *editor.Cursor, padding int) {
	for cursor.Row() >= scroll.Rows() &&
		cursor.MoveUp() {
	}

	for cursor.Column() >= scroll.Columns(cursor.Row())+padding &&
		cursor.MoveLeft() {
	}
}

func moveInBoundsNormal(scroll *component.Scroll, cursor *editor.Cursor) {
	moveInBounds(scroll, cursor, 0)
}

func moveInBoundsInsert(scroll *component.Scroll, cursor *editor.Cursor) {
	moveInBounds(scroll, cursor, 1)
}

func skipNulls(cursor *editor.Cursor) {
	for c, ok := cursor.Cell(); ; c, ok = cursor.Cell() {
		if !ok {
			if !cursor.MoveLeft() {
				break
			}
			continue
		}
		if c.Ch == '\x00' {
			if !cursor.MoveRight() {
				break
			}
			continue
		}
		break
	}
}

// Cursor : fractal.Handler
func (vi *Vi) Cursor() (term.Coordinates, bool) {
	// use less Cursor if we are in search mode
	if vi.less.Mode() != LessNormalMode {
		return vi.less.Cursor()
	}
	return vi.cursor.Cursor()
}

func (vi *Vi) setMode(text string, mode viMode) {
	vi.less.SetMessage(text)
	vi.mode = mode
}

func (vi *Vi) setNormalMode() {
	vi.setMode("NORMAL", normal)
}

func (vi *Vi) setInsertMode() {
	vi.setMode("INSERT", insert)
}

func (vi *Vi) setVisualMode() {
	if vi.cursor.Select() {
		vi.setMode("VISUAL", visual)
	}
}

func (vi *Vi) setVisualLineMode() {
	if vi.cursor.SelectLine() {
		vi.setMode("V-LINE", visual)
	}
}

func (vi *Vi) setVisualBlockMode() {
	if vi.cursor.SelectBlock() {
		vi.setMode("V-BLOCK", visual)
	}
}

// we delegate search buffer Component to Less but delegate cursor position
// and results seeking to Editor so this function makes sure that we only
// perform the search once, at the same time we delegate the right logic to
// Editor and Less.
func (vi *Vi) handleSearch(ev term.Event) bool {
	switch ev.Key {
	case term.KeyEnter:
		text := vi.less.SearchText()
		vi.less.SetNormalMode()
		vi.cursor.Search(text)
	default:
		vi.less.Handle(ev)
	}
	return false
}

func (vi *Vi) pasteClipboard() bool {
	str, err := vi.config.Clipboard.Get()
	if err != nil {
		vi.logger.Error("clipboard.Get: ", err)
		return false
	}
	vi.cursor.InsertString(str)
	return true
}

func (vi *Vi) handleNormal(ev term.Event) bool {
	switch ev.Type {
	case term.EventKey:
		switch ev.Ch {
		case 'q':
			return true
		case 'N':
			vi.cursor.MovePrevSearchResult()
		case 'p':
			vi.cursor.MoveRight()
			vi.pasteClipboard()
			vi.cursor.MoveLeft()
		case 'P':
			vi.pasteClipboard()
		case 'n':
			vi.cursor.MoveNextSearchResult()
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
		case 'x':
			vi.cursor.Delete()
		case 's':
			vi.cursor.Delete()
			vi.cursor.MoveRight()
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
		case '/':
			vi.less.Handle(ev)
		case '%':
			vi.cursor.MoveToMatchingRune()
		default:
			switch ev.Key {
			case term.KeyCtrlR:
				vi.cursor.Redo()
			case term.KeyCtrlV:
				vi.setVisualBlockMode()
			}
		}
	}

	return false
}

func (vi *Vi) handleInsert(ev term.Event) bool {
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
		}
	}
	return false
}

func (vi *Vi) handleVisual(ev term.Event) (ok bool) {
	if ev.Key == term.KeyEsc {
		vi.setNormalMode()
		vi.cursor.Unselect()
		return
	}

	handled := false
	if ev.Type == term.EventKey {
		handled = true
		switch ev.Ch {
		case 'y':
			vi.config.Clipboard.Set(vi.cursor.Selection())
			vi.cursor.Unselect()
		case 'd', 'x':
			vi.cursor.DeleteSelection()
		case 's', 'c':
			vi.cursor.DeleteSelection()
			vi.setInsertMode()
		default:
			handled = false
		}
	}

	if !handled {
		ok = vi.handleNormal(ev)
	}
	return
}

// Handle : fractal.Handler
func (vi *Vi) Handle(ev term.Event) (ok bool) {
	switch vi.mode {
	case normal:
		if vi.less.Mode() != LessNormalMode {
			ok = vi.handleSearch(ev)
		} else {
			ok = vi.handleNormal(ev)
		}
	case insert:
		ok = vi.handleInsert(ev)
	case visual:
		ok = vi.handleVisual(ev)
	default:
		panic(fmt.Sprintf("unknown mode: %d", vi.mode))
	}

	// copy the cursor to maintain original cursor for next vertcial move.
	vi.raw = vi.cursor

	switch vi.mode {
	case normal, visual:
		moveInBoundsNormal(&vi.less.Scroll, &vi.cursor)
	case insert:
		moveInBoundsInsert(&vi.less.Scroll, &vi.cursor)
	default:
		panic(fmt.Sprintf("unknown mode: %d", vi.mode))
	}

	skipNulls(&vi.cursor)

	return
}

// ReadFrom reads data from r until EOF and appends it to the buffer, growing
// the buffer as needed. The return value n is the number of bytes read. Any
// error except io.EOF encountered during the read is also returned.
func (vi *Vi) ReadFrom(reader io.Reader) (int64, error) {
	return vi.less.ReadFrom(reader)
}
