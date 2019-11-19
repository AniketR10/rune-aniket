package handler

import (
	"fmt"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/term"
)

const (
	cmdBarHeight = 1
)

// LessConfig holds configuration values for a Less instance.
type LessConfig struct {
	Wrap    bool
	ResAttr term.Attributes
	Handler func(LessEvent)
}

var defaultConfig = LessConfig{
	Wrap: false,
	ResAttr: term.Attributes{
		Fg: term.AttrReverse,
		Bg: term.ColorDefault,
	},
}

// DefaultLessConfig returns sane configuration defaults for a less instance.
func DefaultLessConfig() *LessConfig {
	cfg := new(LessConfig)
	*cfg = defaultConfig
	return cfg
}

// Less is a clone of Unix' less program which implements
// the Handler and Component interfaces.
type Less struct {
	component.Scroll
	buf          *cell.Buffer
	cmdScroll    component.VirtualComponent
	msgScroll    component.VirtualComponent
	mode         LessMode
	delEOF       bool
	cursorOffset int
	height       int
	width        int
	search       string
	config       *LessConfig
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

func getBuffer(virtualScroll component.VirtualComponent) *cell.Buffer {
	return virtualScroll.C.(*component.Scroll).Buffer()
}

func (l *Less) setNormalMode() {
	l.cursorOffset = 1
	getBuffer(l.cmdScroll).Reset()
	getBuffer(l.cmdScroll).WriteRune(':')
	l.mode = LessNormalMode
}

func (l *Less) setSearchMode() {
	getBuffer(l.cmdScroll).Reset()
	getBuffer(l.cmdScroll).WriteRune('/')
	l.mode = LessSearchMode
}

func (l *Less) searchHandleEvent(ev term.Event) (exit bool) {
	switch ev.Key {

	case term.KeyBackspace:
		fallthrough
	case term.KeyBackspace2:
		if l.cursorOffset > 1 {
			l.cursorOffset--
			l.cmdScroll.C.(*component.Scroll).Buffer().
				DeleteCell(term.Coordinates{X: l.cursorOffset, Y: 0})
		}
	case term.KeyEnter:
		str := getBuffer(l.cmdScroll).String()
		bytes := []byte(str)[1:]
		l.search = string(bytes)
		l.Scroll.Search(l.search)
		l.setNormalMode()
		l.Scroll.SeekNextResult()
		l.sendEvent(LessEvent{Type: Search, Data: bytes})

	case term.KeyEsc:
		l.setNormalMode()

	default:
		l.cursorOffset++
		getBuffer(l.cmdScroll).WriteRune(ev.Ch)
	}

	return
}

func (l *Less) normalHandleEvent(ev term.Event) (exit bool) {
	switch ev.Type {
	case term.EventKey:
		switch ev.Ch {
		case 'q':
			exit = true
		case 'N':
			l.Scroll.SeekPrevResult()
		case 'n':
			l.Scroll.SeekNextResult()
		case '0':
			l.Scroll.SeekStartLine()
		case '$':
			l.Scroll.SeekEndLine()
		case 'g':
			l.Scroll.SeekStartFile()
		case 'G':
			l.Scroll.SeekEndFile()
		case 'j':
			l.Scroll.SeekDown()
		case 'k':
			l.Scroll.SeekUp()
		case 'h':
			l.Scroll.SeekLeft()
		case 'l':
			l.Scroll.SeekRight()
		case '/':
			l.setSearchMode()
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

// Reset resets the contents and state of this instance.
func (l *Less) Reset() {
	l.InitWithBufferConfig(l.buf, l.config)
}

// SetBuffer swaps the main scroll for s and returns the original scroll.
func (l *Less) SetBuffer(b *cell.Buffer) (orig *cell.Buffer) {
	orig = l.buf
	l.buf = b
	l.Reset()
	return
}

// SetContent replaces the content of the underlying scroll with 'text'.
func (l *Less) SetContent(text string, args ...interface{}) {
	l.Reset()
	l.Scroll.Buffer().WriteString(fmt.Sprintf(text, args...))
	l.Resize(l.width, l.height)
}

// Mode returns the current LessMode.
func (l *Less) Mode() LessMode {
	return l.mode
}

// GetCursor : Handler
func (l *Less) GetCursor() term.Coordinates {
	return term.Coordinates{X: l.cursorOffset, Y: l.height - 1}
}

// Draw : Component
func (l *Less) Draw(w fractal.Writer) {
	l.Scroll.Draw(w)

	if !l.Scroll.CanSeekDown() && !l.delEOF {
		l.delEOF = true
		l.sendEvent(LessEvent{Type: EOF})
	}

	l.cmdScroll.Draw(w)
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

	l.Scroll.Resize(l.width, contentHeight)
	l.cmdScroll.Move(term.Coordinates{X: 0, Y: contentHeight})
	l.cmdScroll.Resize(cmdBarWidth, cmdBarHeight)
	l.msgScroll.Move(term.Coordinates{X: cmdBarWidth, Y: contentHeight})
	l.msgScroll.Resize(msgWidth, cmdBarHeight)
}

// Handle : Handler
func (l *Less) Handle(ev term.Event) (exit bool) {
	switch ev.Type {
	case term.EventError:
		return false
	case term.EventKey:
		switch l.mode {
		case LessNormalMode:
			exit = l.normalHandleEvent(ev)
		case LessSearchMode:
			exit = l.searchHandleEvent(ev)
		}
	}

	return
}

func (l *Less) setupScroll(w *component.Scroll) {
	w.ResultsAttr = l.config.ResAttr
	w.Wrap = l.config.Wrap
}

// Buffer provides acces to the underlying Buffer.
func (l *Less) Buffer() *cell.Buffer {
	return l.buf
}

// Man : Handler
func (l *Less) Man() fractal.Manual {
	return fractal.Manual{
		Summary: "Less is a handler similar to Unix' less program, but simplified. It allows basic navigation with vi-style key bindings and text search.",
		Keys: fractal.KeyMap{
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

// InitWithConfig initializes a Less Handler with the given Buffer.
// If config is nil this method panics.
func (l *Less) InitWithConfig(cfg *LessConfig) {
	if cfg == nil {
		panic("initializing less handler with nil configuration")
	}
	l.InitWithBufferConfig(new(cell.Buffer), cfg)
}

// InitWithBuffer initializes a Less Handler with the given buffer.
func (l *Less) InitWithBuffer(buf *cell.Buffer) {
	l.InitWithBufferConfig(buf, nil)
}

// Init initializes this instance or resets it if already initialized.
func (l *Less) Init() {
	l.InitWithBufferConfig(new(cell.Buffer), nil)
}

// InitWithBufferConfig initialzes this instance with the given Buffer and configuration.
// If config is nil, the default one is used.
func (l *Less) InitWithBufferConfig(buf *cell.Buffer, cfg *LessConfig) {
	if cfg == nil {
		l.config = DefaultLessConfig()
	} else {
		l.config = cfg
	}

	l.delEOF = false
	l.buf = buf
	l.Scroll.InitWithBuffer(l.buf)

	// TODO what should scroll be?
	l.cmdScroll.C = component.NewScroll()
	l.msgScroll.C = component.NewScroll()

	l.setupScroll(l.cmdScroll.C.(*component.Scroll))
	l.setupScroll(l.msgScroll.C.(*component.Scroll))
	l.setupScroll(&l.Scroll)

	l.setNormalMode()

	return
}

// NewLess allocates storage and returns a new instance of Less.
func NewLess() *Less {
	l := new(Less)
	l.Init()
	return l
}
