package fractal

import (
	"fmt"

	"termbox"
)

const (
	cmdBarHeight = 1
)

type LessConfig struct {
	Tabspaces int
	Wrap      bool
	ResFG     termbox.Attribute
	ResBG     termbox.Attribute
	Handler   func(LessEvent) error
}

var defaultConfig = LessConfig{
	Tabspaces: 8,
	Wrap:      false,
	ResFG:     termbox.AttrReverse,
	ResBG:     termbox.ColorDefault,
}

func DefaultLessConfig() *LessConfig {
	cfg := new(LessConfig)
	*cfg = defaultConfig
	return cfg
}

type Less struct {
	*Scroll
	cmdScroll    Scroll
	msgScroll    Scroll
	mode         mode
	delEOF       bool
	pos          Coordinates
	cursorOffset int
	height       int
	width        int
	search       string
	config       *LessConfig
}

// EventType represents a less event
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

// Event type represents a less event.
type LessEvent struct {
	Type LessEventType
	Data []byte
	Err  error
}

func (l *Less) sendEvent(ev LessEvent) error {
	if l.config.Handler != nil {
		if err := l.config.Handler(ev); err != nil {
			return fmt.Errorf("less handler failed to process event: %+v: %v", ev, err)
		}
	}

	return nil
}

func (l *Less) setNormalMode() error {
	l.cursorOffset = 1
	l.cmdScroll.Reset()
	if _, err := l.cmdScroll.WriteRune(':'); err != nil {
		return err
	}
	l.mode = normalMode

	return nil
}

func (l *Less) setSearchMode() error {
	l.cmdScroll.Reset()
	if _, err := l.cmdScroll.WriteRune('/'); err != nil {
		return err
	}
	l.mode = searchMode

	return nil
}

func (l *Less) searchHandleEvent(ev termbox.Event) (exit bool, err error) {
	switch ev.Key {

	case termbox.KeyBackspace:
		fallthrough
	case termbox.KeyBackspace2:
		if l.cursorOffset > 0 {
			l.cursorOffset--
		}
		l.cmdScroll.Truncate(l.cmdScroll.Len() - 1)

	case termbox.KeyEnter:
		str := l.cmdScroll.String()
		bytes := []byte(str)[1:]
		l.search = string(bytes)
		l.Scroll.Search(l.search)
		if err = l.setNormalMode(); err != nil {
			return
		}
		l.Scroll.SeekNextResult()
		if err = l.sendEvent(LessEvent{Type: Search, Data: bytes}); err != nil {
			return
		}

	case termbox.KeyEsc:
		if err = l.setNormalMode(); err != nil {
			return
		}

	default:
		l.cursorOffset++
		if _, err = l.cmdScroll.WriteRune(ev.Ch); err != nil {
			return
		}
	}

	return
}

func (l *Less) normalHandleEvent(ev termbox.Event) (exit bool, err error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Ch {
		case 'q':
			return true, nil
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
			err = l.setSearchMode()
		}
	}

	return
}

// Message will draw a message on the bottom right corner
func (l *Less) SetMessage(text string, args ...interface{}) (err error) {
	l.msgScroll.Reset()
	if _, err := l.msgScroll.Write(fmt.Sprintf(text, args...)); err != nil {
		return err
	}

	return l.Resize(l.width, l.height)
}

func (l *Less) Reset() {
	l.Scroll.Reset()
	l.delEOF = false
}

func (l *Less) SetScroll(scroll *Scroll) (orig *Scroll) {
	orig = l.Scroll
	l.Scroll = scroll
	l.Reset()
	l.setupScroll(l.Scroll)

	if len(l.search) != 0 {
		l.Scroll.Search(l.search)
	}

	return
}

func (l *Less) SetContent(text string, args ...interface{}) error {
	l.Reset()

	if _, err := l.Scroll.Write(fmt.Sprintf(text, args...)); err != nil {
		return err
	}

	if len(l.search) != 0 {
		l.Scroll.Search(l.search)
	}

	return nil
}

func (l *Less) GetCursor() Coordinates {
	return Coordinates{X: l.pos.X + l.cursorOffset, Y: l.pos.Y + l.height - 1}
}

func (l *Less) Draw(w Writer) (err error) {
	if err = l.Scroll.Draw(w); err != nil {
		return err
	}

	if !l.Scroll.CanSeekDown() && !l.delEOF {
		l.delEOF = true
		if err = l.sendEvent(LessEvent{Type: EOF}); err != nil {
			return err
		}
	}

	if err = l.cmdScroll.Draw(w); err != nil {
		return err
	}

	if err = l.msgScroll.Draw(w); err != nil {
		return err

	}

	return nil
}

func (l *Less) Resize(width, height int) error {
	l.width, l.height = width, height
	return l.resize()
}

func (l *Less) Move(x, y int) error {
	l.pos.X, l.pos.Y = x, y
	return l.resize()
}

func (l *Less) Height() int {
	return l.height
}

func (l *Less) Width() int {
	return l.width
}

func (l *Less) Position() (int, int) {
	return l.pos.X, l.pos.Y
}

func (l *Less) resize() error {
	var err error

	contentHeight := l.height - cmdBarHeight
	msgWidth := l.msgScroll.Len()
	cmdBarWidth := l.width - msgWidth

	if err = l.Scroll.Move(l.pos.X, l.pos.Y); err != nil {
		return err
	}

	if err = l.Scroll.Resize(l.width, contentHeight); err != nil {
		return err
	}

	if err = l.cmdScroll.Move(l.pos.X, l.pos.Y+contentHeight); err != nil {
		return err
	}

	if err = l.cmdScroll.Resize(cmdBarWidth, cmdBarHeight); err != nil {
		return err
	}

	if err = l.msgScroll.Move(l.pos.X+cmdBarWidth, l.pos.Y+contentHeight); err != nil {
		return err
	}

	if err = l.msgScroll.Resize(msgWidth, cmdBarHeight); err != nil {
		return err
	}

	return nil
}

func (l *Less) Handle(ev termbox.Event) (exit bool, err error) {
	switch ev.Type {
	case termbox.EventError:
		return false, ev.Err
	case termbox.EventKey:
		switch l.mode {
		case normalMode:
			exit, err = l.normalHandleEvent(ev)
		case searchMode:
			exit, err = l.searchHandleEvent(ev)
		}
	}

	return
}

func (l *Less) setupScroll(w *Scroll) {
	w.ResultsFG = l.config.ResFG
	w.ResultsBG = l.config.ResBG
	w.Tabspaces, w.Wrap = l.config.Tabspaces, l.config.Wrap
}

// Init will initialize a less handler. If config is null, the default
// configuration will be used.
func (l *Less) Init(cfg *LessConfig) (err error) {
	s := new(Scroll)
	s.Init()
	if err = l.InitWithScroll(s, cfg); err != nil {
		return err
	}

	return nil
}

func (l *Less) InitWithScroll(scroll *Scroll, cfg *LessConfig) (err error) {
	if cfg == nil {
		l.config = DefaultLessConfig()
	} else {
		l.config = cfg
	}

	l.Scroll = scroll
	l.cmdScroll.Init()
	l.msgScroll.Init()
	l.Scroll.Init()

	l.setupScroll(&l.cmdScroll)
	l.setupScroll(&l.msgScroll)
	l.setupScroll(l.Scroll)

	if err = l.setNormalMode(); err != nil {
		return
	}

	return
}

func NewLess(cfg *LessConfig) (*Less, error) {
	l := new(Less)

	if err := l.Init(cfg); err != nil {
		return nil, err
	}

	return l, nil
}

func (l *Less) Man() string {
	return `
q: exit
j: scroll down
k: scroll up
l: scroll right
h: scroll left
/: enter search mode
G: scroll to end of file
g: scroll to start of file
$: scroll to end of line
0: scroll to start of line
n: scroll to next search result
n: scroll to prev search result
`
}
