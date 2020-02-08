package handler

import (
	"fmt"
	"strings"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/editor"
	"github.com/ernestrc/fractal/term"
	log "github.com/sirupsen/logrus"
)

type viMode uint8
type moveMode uint8

const (
	normalMode viMode = iota
	insertMode
	visualMode
	commandMode
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

// viConfig holds configuration for Vi.
type viConfig struct {
	Filepath         string
	Buffer           *cell.Buffer
	ResAttr          term.Attributes
	Tabspaces        int
	Clipboard        editor.Clipboard
	Logger           *log.Logger
	SwapDir          string
	RecoverySwapFile string
}

// ViOption represents a Vi handler configuration option.
type ViOption func(*viConfig)

// WithViSwapDir defines the swap directory to use if WithViFilepath option is set.
// The swap directory is used to keep persist recovery files. If this option is not
// defined, the directory of WithViFilepath is used as a swap directory.
func WithViSwapDir(dir string) ViOption {
	return func(cfg *viConfig) {
		cfg.SwapDir = dir
	}
}

// WithViRecoveryFile indicates that a Vi handler is to be initialized
// from recovery file swapFilePath. This option overrides WithViSwapDir because
// the swap directory of swapFilePath is used instead.
func WithViRecoveryFile(swapFilePath string) ViOption {
	return func(cfg *viConfig) {
		cfg.RecoverySwapFile = swapFilePath
	}
}

// WithViLogger sets the editor.Clipboard implementation to use.
func WithViLogger(l *log.Logger) ViOption {
	return func(cfg *viConfig) {
		cfg.Logger = l
	}
}

// WithViClipboard sets the editor.Clipboard implementation to use.
func WithViClipboard(clip editor.Clipboard) ViOption {
	return func(cfg *viConfig) {
		cfg.Clipboard = clip
	}
}

// WithViTabspaces sets the number of spaces used to render a tab.
func WithViTabspaces(tabspaces int) ViOption {
	return func(cfg *viConfig) {
		cfg.Tabspaces = tabspaces
	}
}

// WithViResAttr sets the search result cell attributes to be rendered.
func WithViResAttr(attr term.Attributes) ViOption {
	return func(cfg *viConfig) {
		cfg.ResAttr = attr
	}
}

// WithViBuffer is a ViOption that sets the buffer to use with
// Vi handler. If this option is set, note that it overrides
// WithViFilepath so some functions (like Saving to disk) will be disabled.
// This option also overrides WithViRecoveryFile.
func WithViBuffer(buf *cell.Buffer) ViOption {
	return func(cfg *viConfig) {
		cfg.Buffer = buf
	}
}

// WithViFilepath is a ViOption that sets the filepath of the file to open with
// a Vi handler.
func WithViFilepath(filepath string) ViOption {
	return func(cfg *viConfig) {
		cfg.Filepath = filepath
	}
}

// Vi implements a basic vi-like text editor which satisfies fractal.Handler
// and fractal.Component.
type Vi struct {
	config      viConfig
	less        Less // used for message bar and text search capabilities
	logger      *log.Logger
	raw, cursor editor.Cursor
	fileBuf     *editor.FileBuffer
	mode        viMode
	moveMode    moveMode
	moveChar    rune
	command     []rune
}

// DefaultViConfig is a sane configuration defaults for Vi.
var defaultViConfig = viConfig{
	ResAttr: term.Attributes{
		Fg: term.AttrReverse,
		Bg: term.ColorDefault,
	},
	Tabspaces: 4,
	Clipboard: editor.NewEphemeralClipboard(),
	Logger:    nil,
}

// NewVi allocates storage for a new Vi handle, initializes it and returns it.
func NewVi(opts ...ViOption) (*Vi, error) {
	vi := new(Vi)
	err := vi.Init(opts...)
	if err != nil {
		return nil, err
	}
	return vi, nil
}

// Init initialies this vi handle with a new Buffer.
func (vi *Vi) Init(opts ...ViOption) (err error) {
	vi.config = defaultViConfig
	for _, o := range opts {
		o(&vi.config)
	}
	buf := vi.config.Buffer
	if buf == nil {
		if vi.config.Filepath == "" {
			panic("either WithViFilepath or WithViBuffer must be set")
		}
		buf = cell.NewBuffer()
		buf.InitWithTabspaces(vi.config.Tabspaces)
	} else {
		// initialize Buffer but with the configured tabspaces
		str := buf.String()
		buf.InitWithTabspaces(vi.config.Tabspaces)
		_, _ = buf.ReadFrom(strings.NewReader(str))
	}

	if vi.logger != nil {
		buf = buf.WithLogger(vi.logger)
	}

	// this copies buf to the Scroll's Buffer
	vi.less.InitWithBuffer(buf)
	vi.cursor.Init(&vi.less.Scroll)

	if vi.config.Filepath != "" {
		if vi.config.RecoverySwapFile != "" {
			vi.fileBuf, err = editor.RecoverFile(vi.config.Filepath,
				vi.config.RecoverySwapFile, &vi.less.Scroll.Buffer)
		} else {
			vi.fileBuf, err = editor.NewFileBuffer(
				vi.config.Filepath, &vi.less.Scroll.Buffer, vi.config.SwapDir)
		}
		if err != nil {
			return err
		}
	}

	editor.WithCopyDelete(vi.config.Clipboard, &vi.less.Scroll.Buffer)
	vi.less.Scroll.ResultsAttr = vi.config.ResAttr
	vi.logger = vi.config.Logger

	vi.raw = vi.cursor

	vi.setNormalMode()
	return nil
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

// Cursor : fractal.Handler
func (vi *Vi) Cursor() (term.Coordinates, bool) {
	if vi.mode == commandMode {
		pos := term.Coordinates{X: len(vi.command), Y: vi.less.Height()}
		return pos, true
	}
	// use less Cursor if we are in search mode
	if vi.less.Mode() != LessNormalMode {
		return vi.less.Cursor()
	}
	return vi.cursor.Cursor()
}

func (vi *Vi) setMode(text string, mode viMode) {
	vi.less.SetMessageAlt(":")
	vi.less.SetMessage(text)
	vi.mode = mode
}

func (vi *Vi) setError(err error) {
	vi.less.SetMessageAlt("Error: %s", err)
}

func (vi *Vi) setNormalMode() {
	vi.setMode("NORMAL", normalMode)
	vi.command = vi.command[:0]
	vi.less.SetMessageAlt(":")
}

func (vi *Vi) setInsertMode() {
	vi.setMode("INSERT", insertMode)
}

func (vi *Vi) setVisualMode() {
	if vi.cursor.Select() {
		vi.setMode("VISUAL", visualMode)
	}
}

func (vi *Vi) setVisualLineMode() {
	if vi.cursor.SelectLine() {
		vi.setMode("V-LINE", visualMode)
	}
}

func (vi *Vi) setVisualBlockMode() {
	if vi.cursor.SelectBlock() {
		vi.setMode("V-BLOCK", visualMode)
	}
}

func (vi *Vi) setCommandMode() {
	vi.setMode("COMMAND", commandMode)
}

func (vi *Vi) setMoveToCharacterMode(mode moveMode) {
	vi.setMode("MOVE-TO", moveToCharMode)
	vi.moveMode = mode
}

func (vi *Vi) setReplaceMode() {
	vi.setMode("REPLACE", replaceMode)
}

func (vi *Vi) setReplaceOneMode() {
	vi.setMode("NORMAL", replaceOneMode)
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
		case ':':
			vi.setCommandMode()
			vi.handleCommand(ev)
		case 'N':
			vi.cursor.MoveToPrevMatch()
		case 'p':
			vi.cursor.MoveRight()
			vi.pasteClipboard()
			vi.cursor.MoveLeft()
		case 'P':
			vi.pasteClipboard()
		case 'n':
			vi.cursor.MoveToNextMatch()
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

func (vi *Vi) handleVisual(ev term.Event) (quit bool) {
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
		quit = vi.handleNormal(ev)
	}
	return
}

func (vi *Vi) handleMoveToCharacter(mode moveMode, ev term.Event) (quit bool) {
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
	return
}

func (vi *Vi) runCommand() (quit bool, err error) {
	cmd := string(vi.command[1:])
	switch cmd {
	case "wq", "wq!":
		quit = true
		fallthrough
	case "w", "w!":
		if vi.fileBuf != nil {
			err = vi.fileBuf.Flush()
		} else {
			err = fmt.Errorf("Cannot save non-file buffer")
		}
	case "q!", "q":
		quit = true
	default:
		err = fmt.Errorf("Unknown command: %s", cmd)
	}
	return
}

func (vi *Vi) handleCommand(ev term.Event) (quit bool) {
	switch ev.Key {
	case term.KeyEnter:
		var err error
		quit, err = vi.runCommand()
		vi.setNormalMode()
		if err != nil {
			vi.setError(err)
		}
	case term.KeyEsc:
		vi.setNormalMode()
	case term.KeyBackspace, term.KeyBackspace2:
		if len(vi.command) == 1 {
			vi.setNormalMode()
			return
		}
		vi.command = vi.command[:len(vi.command)-1]
		vi.less.SetMessageAlt(string(vi.command))
	case term.KeySpace:
		ev.Ch = ' '
		fallthrough
	default:
		if ev.Type == term.EventKey && ev.Ch != 0 {
			vi.command = append(vi.command, ev.Ch)
			vi.less.SetMessageAlt(string(vi.command))
		}
	}
	return
}

func (vi *Vi) handleReplace(ev term.Event) (quit bool) {
	if ev.Type != term.EventKey {
		return
	}

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
		if vi.cursor.Column() < vi.less.Columns(vi.cursor.Row()) {
			vi.cursor.Delete()
		}
		vi.cursor.Insert(ev.Ch)
	}
	return
}

// Handle : fractal.Handler
func (vi *Vi) Handle(ev term.Event) (quit bool) {
	switch vi.mode {
	case normalMode:
		if vi.less.Mode() != LessNormalMode {
			quit = vi.handleSearch(ev)
		} else {
			quit = vi.handleNormal(ev)
		}
	case insertMode:
		quit = vi.handleInsert(ev)
	case visualMode:
		quit = vi.handleVisual(ev)
	case moveToCharMode:
		quit = vi.handleMoveToCharacter(vi.moveMode, ev)
	case commandMode:
		quit = vi.handleCommand(ev)
	case replaceMode:
		quit = vi.handleReplace(ev)
	case replaceOneMode:
		quit = vi.handleReplace(ev)
		vi.setNormalMode()
	default:
		panic(fmt.Sprintf("unknown mode: %d", vi.mode))
	}

	// copy the cursor to maintain original cursor for next vertcial move.
	vi.raw = vi.cursor

	switch vi.mode {
	case normalMode, visualMode, commandMode, moveToCharMode:
		vi.cursor.MoveToBounds(0)
		vi.cursor.MoveToNextNonNull()
	case insertMode, replaceMode, replaceOneMode:
		vi.cursor.MoveToBounds(1)
	default:
		panic(fmt.Sprintf("unknown mode: %d", vi.mode))
	}

	return
}

// Close closes the resources associated with this instance of Vi.
func (vi *Vi) Close() error {
	if vi.fileBuf != nil {
		return vi.fileBuf.Close()
	}
	return nil
}
