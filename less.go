package fractal

import (
	"fmt"

	"github.com/nsf/termbox-go"
)

const (
	cmdBarHeight = 1
)

// LessConfig holds configuration values for a Less instance.
type LessConfig struct {
	Tabspaces int
	Wrap      bool
	ResFG     termbox.Attribute
	ResBG     termbox.Attribute
	Handler   func(LessEvent)
}

var defaultConfig = LessConfig{
	Tabspaces: 4,
	Wrap:      false,
	ResFG:     termbox.AttrReverse,
	ResBG:     termbox.ColorDefault,
}

// DefaultLessConfig returns sane configuration defaults for a less instance.
func DefaultLessConfig() *LessConfig {
	cfg := new(LessConfig)
	*cfg = defaultConfig
	return cfg
}

// Less is a clone of Unix' less program.
type Less struct {
	*Scroll
	cmdScroll    VirtualComponent
	msgScroll    VirtualComponent
	mode         mode
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

type mode uint8

const (
	normalMode mode = iota
	searchMode
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

func (l *Less) setNormalMode() {
	l.cursorOffset = 1
	l.cmdScroll.C.(*Scroll).Reset()
	l.cmdScroll.C.(*Scroll).WriteRune(':')
	l.mode = normalMode
}

func (l *Less) setSearchMode() {
	l.cmdScroll.C.(*Scroll).Reset()
	l.cmdScroll.C.(*Scroll).WriteRune('/')
	l.mode = searchMode
}

func (l *Less) searchHandleEvent(ev termbox.Event) (exit bool) {
	switch ev.Key {

	case termbox.KeyBackspace:
		fallthrough
	case termbox.KeyBackspace2:
		if l.cursorOffset > 1 {
			l.cursorOffset--
			l.cmdScroll.C.(*Scroll).TruncateCellAt(Coordinates{X: l.cursorOffset, Y: 0})
		}
	case termbox.KeyEnter:
		str := l.cmdScroll.C.(*Scroll).String()
		bytes := []byte(str)[1:]
		l.search = string(bytes)
		l.Scroll.Search(l.search)
		l.setNormalMode()
		l.Scroll.SeekNextResult()
		l.sendEvent(LessEvent{Type: Search, Data: bytes})

	case termbox.KeyEsc:
		l.setNormalMode()

	default:
		l.cursorOffset++
		l.cmdScroll.C.(*Scroll).WriteRune(ev.Ch)
	}

	return
}

func (l *Less) normalHandleEvent(ev termbox.Event) (exit bool) {
	switch ev.Type {
	case termbox.EventKey:
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
	l.msgScroll.C.(*Scroll).Reset()
	l.msgScroll.C.(*Scroll).WriteString(fmt.Sprintf(text, args...))
	l.Resize(l.width, l.height)
}

// Reset resets the contents and state of this instance.
func (l *Less) Reset() {
	l.Scroll.Reset()
	l.delEOF = false
}

// SetScroll swaps the main scroll for s and returns the original scroll.
func (l *Less) SetScroll(s *Scroll) (orig *Scroll) {
	orig = l.Scroll
	l.Scroll = s
	l.Reset()
	l.setupScroll(l.Scroll)
	return
}

// SetContent replaces the content of the underlying scroll with 'text'.
func (l *Less) SetContent(text string, args ...interface{}) {
	l.Reset()
	l.Scroll.WriteString(fmt.Sprintf(text, args...))
	l.Resize(l.width, l.height)
}

// GetCursor : Handler
func (l *Less) GetCursor() Coordinates {
	return Coordinates{X: l.cursorOffset, Y: l.height - 1}
}

// Draw : Component
func (l *Less) Draw(w Writer) (err error) {
	if err = l.Scroll.Draw(w); err != nil {
		return err
	}

	if !l.Scroll.CanSeekDown() && !l.delEOF {
		l.delEOF = true
		l.sendEvent(LessEvent{Type: EOF})
	}

	if err = l.cmdScroll.Draw(w); err != nil {
		return err
	}

	if err = l.msgScroll.Draw(w); err != nil {
		return err

	}

	return nil
}

// Resize : Component
func (l *Less) Resize(width, height int) {
	l.width, l.height = width, height
	l.resize()
}

func (l *Less) resize() {
	contentHeight := l.height - cmdBarHeight
	msgWidth := len(l.msgScroll.C.(*Scroll).String())
	cmdBarWidth := l.width - msgWidth

	l.Scroll.Resize(l.width, contentHeight)
	l.cmdScroll.Move(Coordinates{0, contentHeight})
	l.cmdScroll.Resize(cmdBarWidth, cmdBarHeight)
	l.msgScroll.Move(Coordinates{cmdBarWidth, contentHeight})
	l.msgScroll.Resize(msgWidth, cmdBarHeight)
}

// Handle : Handler
func (l *Less) Handle(ev termbox.Event) (exit bool) {
	switch ev.Type {
	case termbox.EventError:
		return false
	case termbox.EventKey:
		switch l.mode {
		case normalMode:
			exit = l.normalHandleEvent(ev)
		case searchMode:
			exit = l.searchHandleEvent(ev)
		}
	}

	return
}

func (l *Less) setupScroll(w *Scroll) {
	w.ResultsFG = l.config.ResFG
	w.ResultsBG = l.config.ResBG
	w.Buffer.tabspaces, w.Wrap = l.config.Tabspaces, l.config.Wrap
}

// Man : Handler
func (l *Less) Man() Manual {
	return Manual{
		Summary: "Less is a handler similar to Unix' less program, but simplified. It allows basic navigation with vi-style key bindings and text search.",
		Keys: KeyMap{
			termbox.Event{Type: termbox.EventKey, Ch: 'q'}: {
				ID:          "Normal.Exit",
				Description: "Exit handler.",
			},
			termbox.Event{Type: termbox.EventKey, Ch: 'N'}: {
				ID:          "Normal.SeekPrevResult",
				Description: "Seek to previous search result. See 'SetSearchMode' for more info.",
			},
			termbox.Event{Type: termbox.EventKey, Ch: 'n'}: {
				ID:          "Normal.SeekNextResult",
				Description: "Seek to next search result. See 'SetSearchMode' for more info.",
			},
			termbox.Event{Type: termbox.EventKey, Ch: '0'}: {
				ID:          "Normal.SeekStartLine",
				Description: "Seek scroll enough columns to render start of the line.",
			},
			termbox.Event{Type: termbox.EventKey, Ch: '$'}: {
				ID:          "Normal.SeekEndLine",
				Description: "Seek scroll enough columns to render the end of the line.",
			},
			termbox.Event{Type: termbox.EventKey, Ch: 'g'}: {
				ID:          "Normal.SeekStartScroll",
				Description: "Seek to start of scroll",
			},
			termbox.Event{Type: termbox.EventKey, Ch: 'G'}: {
				ID:          "Normal.SeekEndScroll",
				Description: "Seek to end of scroll.",
			},
			termbox.Event{Type: termbox.EventKey, Ch: 'j'}: {
				ID:          "Normal.SeekDown",
				Description: "Seek scroll one row down.",
			},
			termbox.Event{Type: termbox.EventKey, Ch: 'k'}: {
				ID:          "Normal.SeekUp",
				Description: "Seek scroll one row up.",
			},
			termbox.Event{Type: termbox.EventKey, Ch: 'h'}: {
				ID:          "Normal.SeekLeft",
				Description: "Seek scroll one column to the left.",
			},
			termbox.Event{Type: termbox.EventKey, Ch: 'l'}: {
				ID:          "Normal.SeekRight",
				Description: "Seek scroll one column to the right.",
			},
			termbox.Event{Type: termbox.EventKey, Ch: '/'}: {
				ID:          "Normal.SetSearchMode",
				Description: "Enter search mode. After typing search text, press ENTER to perform a text-search or ESC to go back to normal mode.",
			},
			termbox.Event{Type: termbox.EventKey, Key: termbox.KeyEsc}: {
				ID: "Search.SetNormalMode", Description: "Enter normal mode",
			},
			termbox.Event{Type: termbox.EventKey, Key: termbox.KeyEnter}: {
				ID: "Search.Search", Description: "Perform text search with current search buffer.",
			},
		},
	}
}

// InitWithConfig will initialize a less handler.
// If config is nil this method will panic.
func (l *Less) InitWithConfig(cfg *LessConfig) {
	if cfg == nil {
		panic("initializing less handler with nil configuration")
	}
	l.initWithScrollConfig(new(Scroll), cfg)
}

// Init initializes this instance or resets it if already initialized.
func (l *Less) Init() {
	l.initWithScrollConfig(new(Scroll), nil)
}

// initWithScrollConfig initialzes this instance with the given scroll and configuration.
// If config is nil, the default one will be used.
func (l *Less) initWithScrollConfig(scroll *Scroll, cfg *LessConfig) {
	if cfg == nil {
		l.config = DefaultLessConfig()
	} else {
		l.config = cfg
	}

	l.Scroll = scroll
	l.Scroll.Init(l.config.Tabspaces)

	l.cmdScroll.C = NewScroll()
	l.msgScroll.C = NewScroll()

	l.setupScroll(l.cmdScroll.C.(*Scroll))
	l.setupScroll(l.msgScroll.C.(*Scroll))
	l.setupScroll(l.Scroll)

	l.setNormalMode()

	return
}

// NewLess allocates storage and returns a new instance of Less.
func NewLess() *Less {
	l := new(Less)
	l.Init()
	return l
}
