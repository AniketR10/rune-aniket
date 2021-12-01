package handler

import (
	"fmt"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
)

const (
	cmdBarHeight = 1
)

// LessConfig holds configuration values for a Less instance.
type LessConfig struct {
	Debug   bool
	Wrap    bool
	ResAttr term.Attributes
	Handler func(LessEvent)
}

// DefaultLessConfig is a sane configuration defaults for Less.
func DefaultLessConfig() LessConfig {
	return LessConfig{
		Wrap: false,
		ResAttr: term.Attributes{
			Fg: term.AttrReverse,
			Bg: term.ColorDefault,
		},
	}
}

// Less is a clone of Unix' less program which implements
// the Handler and Component interfaces.
type Less struct {
	scroll       component.Scroll
	msgAltScroll component.Virtual
	searchScroll component.Virtual
	msgScroll    component.Virtual
	mode         LessMode
	delEOF       bool
	cursorOffset int
	height       int
	width        int
	search       string
	config       LessConfig
}

// LessEventType represents a less event
type LessEventType uint8

const (
	// EOF is dispatched when user has reached end of buffer
	EOF LessEventType = iota
	// Search is dispatched when user has performed a text search
	// `Data` field in `Event` struct will be set to the search text
	Search
)

// LessMode represents one of the two modes of a less Handler.
// See Manual for more information on how to switch between modes.
type LessMode uint8

const (
	LessNormalMode LessMode = iota
	LessSearchMode
)

// LessEvent type represents a less event.
type LessEvent struct {
	Type LessEventType
	Data []byte
	Err  error
}

func (l *Less) sendEvent(ev LessEvent) {
	if l.config.Handler != nil {
		l.config.Handler(ev)
	}
}

func getBuffer(virtualScroll component.Virtual) *cell.Buffer {
	return virtualScroll.C.(*component.Scroll).Buffer()
}

// SetNormalMode sets the mode to normal.
func (l *Less) SetNormalMode() {
	l.cursorOffset = 1
	getBuffer(l.searchScroll).Reset()
	getBuffer(l.searchScroll).WriteString(":")
	l.mode = LessNormalMode
}

// SetSearchMode sets the mode to search mode.
func (l *Less) SetSearchMode() {
	getBuffer(l.searchScroll).Reset()
	getBuffer(l.searchScroll).WriteString("/")
	l.mode = LessSearchMode
}

// SearchText returns the contents of the search buffer.
func (l *Less) SearchText() string {
	str := getBuffer(l.searchScroll).String()
	bytes := []byte(str)[1:]
	return string(bytes)
}

func (l *Less) searchHandleEvent(ev term.Event) (bool, bool) {
	switch ev.Key {
	case term.KeyBackspace:
		fallthrough
	case term.KeyBackspace2:
		if l.cursorOffset > 1 {
			l.cursorOffset--
			l.searchScroll.C.(*component.Scroll).Buffer().
				DeleteCell(term.Coordinates{X: l.cursorOffset, Y: 0})
		}
	case term.KeyEnter:
		l.search = l.SearchText()
		l.scroll.Search(l.search)
		l.SetNormalMode()
		l.scroll.SeekNextResult()
		l.sendEvent(LessEvent{Type: Search, Data: []byte(l.search)})

	case term.KeyEsc:
		l.SetNormalMode()

	case term.KeySpace:
		ev.Ch = ' '
		fallthrough

	default:
		l.cursorOffset++
		getBuffer(l.searchScroll).WriteString(string(ev.Ch))
	}

	return false, true
}

func (l *Less) normalHandleEvent(ev term.Event) (exit, handled bool) {
	switch ev.Type {
	case term.EventKey:
		handled = true
		switch ev.Ch {
		case 'q':
			exit = true
		case 'N':
			l.scroll.SeekPrevResult()
		case 'n':
			l.scroll.SeekNextResult()
		case '0':
			l.scroll.SeekStartLine()
		case '$':
			l.scroll.SeekEndLine()
		case 'g':
			l.scroll.SeekStartFile()
		case 'G':
			l.scroll.SeekEndFile()
		case 'j':
			l.scroll.SeekDown()
		case 'k':
			l.scroll.SeekUp()
		case 'h':
			l.scroll.SeekLeft()
		case 'l':
			l.scroll.SeekRight()
		case '/':
			l.SetSearchMode()
		default:
			switch ev.Key {
			case term.KeyEsc:
				exit = true
			default:
				handled = false
			}
		}
	}

	return
}

// SetMessage sets a message to be displayed on the bottom right corner.
func (l *Less) SetMessage(text string, args ...interface{}) {
	getBuffer(l.msgScroll).Reset()
	getBuffer(l.msgScroll).WriteString(fmt.Sprintf(text, args...))
	l.Resize(l.width, l.height)
}

// SetMessageAlt sets a message to be displayed on the bottom left corner.
func (l *Less) SetMessageAlt(text string, args ...interface{}) {
	getBuffer(l.msgAltScroll).Reset()
	getBuffer(l.msgAltScroll).WriteString(fmt.Sprintf(text, args...))
	l.Resize(l.width, l.height)
}

// Mode returns the current LessMode.
func (l *Less) Mode() LessMode {
	return l.mode
}

// Cursor : Handler
func (l *Less) Cursor() (term.Coordinates, bool) {
	return term.Coordinates{X: l.cursorOffset, Y: l.height - 1}, true
}

// Draw : Component
func (l *Less) Draw(w term.Writer) {
	l.scroll.Draw(w)

	if !l.scroll.CanSeekDown() && !l.delEOF {
		l.delEOF = true
		l.sendEvent(LessEvent{Type: EOF})
	}

	l.msgAltScroll.Draw(w)
	l.searchScroll.Draw(w)
	l.msgScroll.Draw(w)
}

// Resize : Component
func (l *Less) Resize(width, height int) {
	l.width, l.height = width, height
	l.resize()
}

func (l *Less) resize() {
	contentHeight := l.height - cmdBarHeight
	msgWidth := len(getBuffer(l.msgScroll).String())
	cmdBarWidth := l.width - msgWidth

	l.scroll.Resize(l.width, contentHeight)
	l.msgAltScroll.Move(term.Coordinates{X: 0, Y: contentHeight})
	l.msgAltScroll.Resize(cmdBarWidth, cmdBarHeight)
	l.searchScroll.Move(term.Coordinates{X: 0, Y: contentHeight})
	l.searchScroll.Resize(cmdBarWidth, cmdBarHeight)
	l.msgScroll.Move(term.Coordinates{X: cmdBarWidth, Y: contentHeight})
	l.msgScroll.Resize(msgWidth, cmdBarHeight)
}

// Handle : Handler
func (l *Less) Handle(ev term.Event) (exit bool, handled bool) {
	switch ev.Type {
	case term.EventKey:
		switch l.mode {
		case LessNormalMode:
			exit, handled = l.normalHandleEvent(ev)
		case LessSearchMode:
			exit, handled = l.searchHandleEvent(ev)
		}
	}

	return
}

func (l *Less) setupScroll(w *component.Scroll) {
	w.ResultsAttr = l.config.ResAttr
	w.Wrap = l.config.Wrap
	w.Debug = l.config.Debug
}

// Buffer returns the internal scroll's Buffer.
func (l *Less) Buffer() *cell.Buffer {
	return l.scroll.Buffer()
}

// Scroll returns the internal scroll. Scroll's public properties
// should not be updated. Use LessConfig instead.
func (l *Less) Scroll() *component.Scroll {
	return &l.scroll
}

// Man : Handler
func (l *Less) Man() tui.Manual {
	return tui.Manual{
		Summary: "Less is a handler similar to Unix' less program, but simplified. It allows basic navigation with vi-style key bindings and text search.",
		Keys: tui.KeyMap{
			term.Event{Type: term.EventKey, Ch: 'q'}: {
				ID:          "Normal.Exit",
				Description: "Exit handler.",
			},
			term.Event{Type: term.EventKey, Ch: 'N'}: {
				ID:          "Normal.SeekPrevResult",
				Description: "Seek to previous search result. See 'SetSearchMode' for more info.",
			},
			term.Event{Type: term.EventKey, Ch: 'n'}: {
				ID:          "Normal.SeekNextResult",
				Description: "Seek to next search result. See 'SetSearchMode' for more info.",
			},
			term.Event{Type: term.EventKey, Ch: '0'}: {
				ID:          "Normal.SeekStartLine",
				Description: "Seek scroll enough columns to render start of the line.",
			},
			term.Event{Type: term.EventKey, Ch: '$'}: {
				ID:          "Normal.SeekEndLine",
				Description: "Seek scroll enough columns to render the end of the line.",
			},
			term.Event{Type: term.EventKey, Ch: 'g'}: {
				ID:          "Normal.SeekStartScroll",
				Description: "Seek to start of scroll",
			},
			term.Event{Type: term.EventKey, Ch: 'G'}: {
				ID:          "Normal.SeekEndScroll",
				Description: "Seek to end of scroll.",
			},
			term.Event{Type: term.EventKey, Ch: 'j'}: {
				ID:          "Normal.SeekDown",
				Description: "Seek scroll one row down.",
			},
			term.Event{Type: term.EventKey, Ch: 'k'}: {
				ID:          "Normal.SeekUp",
				Description: "Seek scroll one row up.",
			},
			term.Event{Type: term.EventKey, Ch: 'h'}: {
				ID:          "Normal.SeekLeft",
				Description: "Seek scroll one column to the left.",
			},
			term.Event{Type: term.EventKey, Ch: 'l'}: {
				ID:          "Normal.SeekRight",
				Description: "Seek scroll one column to the right.",
			},
			term.Event{Type: term.EventKey, Ch: '/'}: {
				ID:          "Normal.SetSearchMode",
				Description: "Enter search mode. After typing search text, press ENTER to perform a text-search or ESC to go back to normal mode.",
			},
			term.Event{Type: term.EventKey, Key: term.KeyEsc}: {
				ID: "Search.SetNormalMode", Description: "Enter normal mode",
			},
			term.Event{Type: term.EventKey, Key: term.KeyEnter}: {
				ID: "Search.Search", Description: "Perform text search with current search buffer.",
			},
		},
	}
}

// Init initializes this instance or resets it if already initialized.
func (l *Less) Init(cfg LessConfig) {
	l.InitWithBuffer(cell.NewBuffer(), cfg)
}

// InitWithBuffer initialzes this instance with the given Buffer and configuration.
// If config is nil, the default one is used.
func (l *Less) InitWithBuffer(buf *cell.Buffer, cfg LessConfig) {
	l.delEOF = false
	l.scroll.InitWithBuffer(buf)
	if cfg.ResAttr == (term.Attributes{}) {
		cfg.ResAttr = DefaultLessConfig().ResAttr
	}

	l.config = cfg
	l.msgAltScroll.C = component.NewScroll()
	l.searchScroll.C = component.NewScroll()
	l.msgScroll.C = component.NewScroll()

	l.setupScroll(l.searchScroll.C.(*component.Scroll))
	l.setupScroll(l.msgAltScroll.C.(*component.Scroll))
	l.setupScroll(l.msgScroll.C.(*component.Scroll))
	l.setupScroll(&l.scroll)

	l.SetNormalMode()

	return
}

// NewLess allocates storage and returns a new instance of Less.
func NewLess(cfg LessConfig) *Less {
	l := new(Less)
	l.Init(cfg)
	return l
}
