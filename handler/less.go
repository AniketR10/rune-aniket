package handler

import (
	"fmt"
	"math"

	"termbox"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
)

type LessConfig struct {
	Tabspaces    int
	Fg           termbox.Attribute
	Bg           termbox.Attribute
	Msgwidth     int8 // 0 - 100%
	CmdBarHeight int  // in cells
	Wrap         bool
	Debug        bool
	ResFG        termbox.Attribute
	ResBG        termbox.Attribute
	ScrollBorder termbox.Attribute
	// TODO keyMap *KeyMap
}

var defaultConfig = LessConfig{
	Tabspaces:    8,
	Fg:           termbox.ColorDefault,
	Bg:           termbox.ColorDefault,
	Msgwidth:     70,
	CmdBarHeight: 1,
	Wrap:         false,
	Debug:        false,
	ResFG:        termbox.AttrReverse,
	ResBG:        termbox.ColorDefault,
}

func DefaultLessConfig() *LessConfig {
	cfg := new(LessConfig)
	*cfg = defaultConfig
	return cfg
}

type Less struct {
	cmdScroll    *component.Scroll
	cmdBuf       fractal.Buffer
	msgScroll    *component.Scroll
	msgBuf       fractal.Buffer
	contScroll   *component.Scroll
	contBuf      fractal.Buffer
	mode         mode
	delEOF       bool
	pos          fractal.Coordinates
	cursorOffset int
	height       int
	width        int
	config       *LessConfig
	search       string
	handler      LessHandler
	exit         bool
}

type LessHandler func(LessEvent) error

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
	if l.handler != nil {
		if err := l.handler(ev); err != nil {
			return fmt.Errorf("less handler failed to process event: %+v: %v", ev, err)
		}
	}

	return nil
}

func (l *Less) setNormalMode() error {
	l.cursorOffset = 1
	l.cmdBuf.Reset()
	if _, err := l.cmdBuf.WriteRune(':'); err != nil {
		return err
	}
	l.mode = normalMode

	return nil
}

func (l *Less) setSearchMode() error {
	l.cmdBuf.Reset()
	if _, err := l.cmdBuf.WriteRune('/'); err != nil {
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
		l.cmdBuf.Truncate(l.cmdBuf.Len() - 1)

	case termbox.KeyEnter:
		str := l.cmdBuf.String()
		bytes := []byte(str)[1:]
		l.search = string(bytes)
		l.contScroll.Search(l.search)
		if err = l.setNormalMode(); err != nil {
			return
		}
		l.contScroll.SeekNextResult()
		if err = l.sendEvent(LessEvent{Type: Search, Data: bytes}); err != nil {
			return
		}

	case termbox.KeyEsc:
		if err = l.setNormalMode(); err != nil {
			return
		}

	default:
		l.cursorOffset++
		if _, err = l.cmdBuf.WriteRune(ev.Ch); err != nil {
			return
		}
	}

	return
}

func (l *Less) normalHandleEvent(ev termbox.Event) (exit bool, err error) {
	switch ev.Type {
	case termbox.EventKey:
		switch ev.Key {
		case termbox.KeyEsc:
			return true, nil
		default:
			switch ev.Ch {
			case 'q':
				return true, nil
			case 'N':
				l.contScroll.SeekPrevResult()
			case 'n':
				l.contScroll.SeekNextResult()
			case '0':
				l.contScroll.SeekStartLine()
			case '$':
				l.contScroll.SeekEndLine()
			case 'g':
				l.contScroll.SeekStartFile()
			case 'G':
				l.contScroll.SeekEndFile()
			case 'j':
				l.contScroll.SeekDown()
			case 'k':
				l.contScroll.SeekUp()
			case 'h':
				l.contScroll.SeekLeft()
			case 'l':
				l.contScroll.SeekRight()
			case '/':
				err = l.setSearchMode()
			}
		}
	}

	return
}

// Message will draw a message on the bottom right corner
func (l *Less) SetMessage(text string, args ...interface{}) (err error) {
	l.msgBuf.Reset()
	if _, err := l.msgBuf.Write([]byte(fmt.Sprintf(text, args...))); err != nil {
		return err
	}

	return l.Resize(l.width, l.height)
}

func (l *Less) SetContent(text string, args ...interface{}) error {
	l.contBuf.Reset()
	l.delEOF = false

	if _, err := l.contBuf.Write([]byte(fmt.Sprintf(text, args...))); err != nil {
		return err
	}

	if len(l.search) != 0 {
		l.contScroll.Search(l.search)
	}

	return nil
}

func (l *Less) GetCursor() fractal.Coordinates {
	return fractal.Coordinates{X: l.pos.X + l.cursorOffset, Y: l.pos.Y + l.height - 1}
}

func (l *Less) Draw(w fractal.Writer) (err error) {
	if err = l.contScroll.Draw(w); err != nil {
		return err
	}

	if !l.contScroll.CanSeekDown() && !l.delEOF {
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

	contentHeight := l.height - l.config.CmdBarHeight
	msgScrollWidth := int(float32(l.width) * float32(l.config.Msgwidth) / 100)
	msgWidth := int(math.Min(float64(msgScrollWidth), float64(l.msgBuf.Len())))
	cmdBarWidth := l.width - msgWidth

	if err = l.contScroll.Resize(l.width, contentHeight); err != nil {
		return err
	}

	l.cmdScroll.Move(l.pos.X, l.pos.Y+contentHeight)
	if err = l.cmdScroll.Resize(cmdBarWidth, l.config.CmdBarHeight); err != nil {
		return err
	}

	l.msgScroll.Move(l.pos.X+cmdBarWidth, l.pos.Y+contentHeight)
	if err = l.msgScroll.Resize(msgWidth, l.config.CmdBarHeight); err != nil {
		return err
	}

	return nil
}

func (l *Less) IsActive() bool {
	return !l.exit
}

func (l *Less) GetAttr() (fg termbox.Attribute, bg termbox.Attribute) {
	return l.config.Fg, l.config.Bg
}

func (l *Less) Handle(ev termbox.Event) (err error) {
	switch ev.Type {
	case termbox.EventError:
		return ev.Err
	case termbox.EventResize:
		if err = l.Resize(ev.Width, ev.Height); err != nil {
			return
		}
	case termbox.EventKey:
		switch l.mode {
		case normalMode:
			l.exit, err = l.normalHandleEvent(ev)
		case searchMode:
			l.exit, err = l.searchHandleEvent(ev)
		}
	case termbox.EventMouse:
	case termbox.EventInterrupt:
	case termbox.EventRaw:
	case termbox.EventNone:
	}

	return
}

func (l *Less) setupScroll(w *component.Scroll) {
	w.ResultsFG = l.config.ResFG
	w.ResultsBG = l.config.ResBG
	w.Tabspaces, w.Wrap = l.config.Tabspaces, l.config.Wrap
}

func (l *Less) Init(content string, handler LessHandler, cfg *LessConfig) (err error) {
	if cfg == nil {
		l.config = DefaultLessConfig()
	} else {
		l.config = cfg
	}
	l.cmdScroll = component.NewScroll(&l.cmdBuf, l.width, l.height, 0, 0)
	l.msgScroll = component.NewScroll(&l.msgBuf, l.width, l.height, 0, 0)
	l.contScroll = component.NewScroll(&l.contBuf, l.width, l.height, 0, 0)

	l.setupScroll(l.cmdScroll)
	l.setupScroll(l.msgScroll)
	l.setupScroll(l.contScroll)

	l.handler = handler

	if content != "" {
		if _, err = l.contBuf.Write([]byte(content)); err != nil {
			return
		}
	}

	if err = termbox.Init(); err != nil {
		return
	}

	if err = l.setNormalMode(); err != nil {
		return
	}

	if err = l.resize(); err != nil {
		return
	}

	return
}

func NewLess(content string, handler LessHandler, cfg *LessConfig) (*Less, error) {
	l := new(Less)

	if err := l.Init(content, handler, cfg); err != nil {
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
