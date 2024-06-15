// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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
package handler

import (
	"fmt"
	"math"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// LessConfig holds configuration values for a Less instance.
type LessConfig struct {
	Debug bool
	Wrap  bool
	// SuperimposeMessage changes the behaviour to instead of drawing
	// a bottom bar permanently on which messages are written,
	// messages are superimposed on the last row of the scroll content.
	SuperimposeMessage bool
	ResAttr            term.Attributes
	BarAttr            term.Attributes
	Attributes         term.Attributes
	Handler            func(LessEvent)
	// NoBar disables SetMessage and search functionality.
	// It takes prevalence over SuperimposeMessage.
	NoBar bool
}

// DefaultLessConfig is a sane configuration defaults for Less.
func DefaultLessConfig() LessConfig {
	return LessConfig{
		Wrap: false,
		ResAttr: term.Attributes{
			Attrs: tcell.AttrReverse,
		},
		NoBar: false,
	}
}

// Less is a clone of Unix' less program which implements
// the Handler and Component interfaces.
type Less struct {
	scroll            *component.Scroll
	searchScroll      component.Scroll
	searchScrollVirt  component.Virtual
	msgStr            string
	msg               component.Responsive
	msgVirt           component.Virtual
	mode              LessMode
	usedMsgBarAttr    term.Attributes
	usedSearchBarAttr term.Attributes
	delEOF            bool
	cursorOffset      int
	height            int
	width             int
	search            string
	config            LessConfig
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

// NewLess allocates storage and returns a new instance of Less.
func NewLess(cfg LessConfig) *Less {
	l := new(Less)
	l.Init(cfg)
	return l
}

// Init initializes this instance or resets it if already initialized.
func (l *Less) Init(cfg LessConfig) {
	l.InitWithBuffer(cell.NewBuffer(), cfg)
}

// InitWithBuffer initialzes this instance with the given Buffer and configuration.
// If config is nil, the default one is used.
func (l *Less) InitWithBuffer(buf *cell.Buffer, cfg LessConfig) {
	l.scroll = component.NewScroll(buf)
	l.initWithBuffer(buf, cfg)
}

// InitWithScroll initialzes this instance with the given main Scroll and configuration.
// If config is nil, the default one is used.
func (l *Less) InitWithScroll(scroll *component.Scroll, cfg LessConfig) {
	l.scroll = scroll
	l.initWithBuffer(scroll.Buffer(), cfg)
}

// SetNormalMode sets the mode to normal.
func (l *Less) SetNormalMode() {
	l.cursorOffset = 1
	l.searchScroll.Buffer().Reset()
	l.mode = LessNormalMode
}

// SetSearchMode sets the mode to search mode.
func (l *Less) SetSearchMode() {
	buf := l.searchScroll.Buffer()
	buf.Reset()
	buf.WriteString("/")
	l.mode = LessSearchMode
	if l.config.SuperimposeMessage && l.scroll.Attributes != l.usedSearchBarAttr {
		l.updateSearchBarAttr()
	}
	_, cmdBarHeight := l.cmdBarHeight()
	l.resizeSearchScroll(cmdBarHeight)
}

// SearchText returns the contents of the search buffer.
func (l *Less) SearchText() string {
	str := l.searchScroll.Buffer().String()
	bytes := []byte(str)[1:]
	return string(bytes)
}

// SetMessage sets a message to be displayed on the bottom right corner.
func (l *Less) SetMessage(text string, args ...interface{}) {
	if (len(args) == 0 && l.setMessage(text)) || l.setMessage(fmt.Sprintf(text, args...)) {
		l.resize()
	} else {
		cmdBarWidth, cmdBarHeight := l.cmdBarHeight()
		l.resizeMoveMessage(cmdBarWidth, cmdBarHeight)
	}
}

// Mode returns the current LessMode.
func (l *Less) Mode() LessMode {
	return l.mode
}

// Cursor satisfies tui.Handler
func (l *Less) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	ret := term.Coordinates{X: l.cursorOffset, Y: l.height - 1}
	return ret, term.CursorStyleDefault, true
}

// Draw satisfies tui.Component
func (l *Less) Draw(w term.Writer) {
	l.scroll.Draw(w)

	if !l.scroll.CanSeekDown() && !l.delEOF {
		l.delEOF = true
		l.sendEvent(LessEvent{Type: EOF})
	}

	// if attrs were changed dynamically, ensure attributes of superimposed message
	// match those of the Scroll.
	if l.config.SuperimposeMessage && l.usedMsgBarAttr != l.scroll.Attributes {
		l.setMessage(l.msgStr)
		cmdBarWidth, cmdBarHeight := l.cmdBarHeight()
		l.resizeMoveMessage(cmdBarWidth, cmdBarHeight)
	}

	l.msgVirt.Draw(w)
	if l.mode == LessSearchMode {
		l.searchScrollVirt.Draw(w)
	}
}

// Resize satisfies tui.Component.
func (l *Less) Resize(width, height int) {
	l.width, l.height = width, height
	l.resize()
}

// Handle satisfies tui.Handler.
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

// Buffer returns the internal scroll's Buffer.
func (l *Less) Buffer() *cell.Buffer {
	return l.scroll.Buffer()
}

// Scroll returns the internal scroll. Scroll's public properties
// should not be updated. Use LessConfig instead.
func (l *Less) Scroll() *component.Scroll {
	return l.scroll
}

// Man satisfies tui.Handler.
func (l *Less) Man() tui.Manual {
	return tui.Manual{
		Summary: "Less is a handler similar to Unix' less program, but simplified. It allows basic navigation with vi-style key bindings and text search.",
		Keys: tui.KeyMap{
			term.KeyComb{Ch: 'q'}: {
				ID:          "Normal.Exit",
				Description: "Exit handler.",
			},
			term.KeyComb{Ch: 'N'}: {
				ID:          "Normal.SeekPrevResult",
				Description: "Seek to previous search result. See 'SetSearchMode' for more info.",
			},
			term.KeyComb{Ch: 'n'}: {
				ID:          "Normal.SeekNextResult",
				Description: "Seek to next search result. See 'SetSearchMode' for more info.",
			},
			term.KeyComb{Ch: '0'}: {
				ID:          "Normal.SeekStartLine",
				Description: "Seek scroll enough columns to render start of the line.",
			},
			term.KeyComb{Ch: '$'}: {
				ID:          "Normal.SeekEndLine",
				Description: "Seek scroll enough columns to render the end of the line.",
			},
			term.KeyComb{Ch: 'g'}: {
				ID:          "Normal.SeekStartScroll",
				Description: "Seek to start of scroll",
			},
			term.KeyComb{Ch: 'G'}: {
				ID:          "Normal.SeekEndScroll",
				Description: "Seek to end of scroll.",
			},
			term.KeyComb{Ch: 'j'}: {
				ID:          "Normal.SeekDown",
				Description: "Seek scroll one row down.",
			},
			term.KeyComb{Ch: 'k'}: {
				ID:          "Normal.SeekUp",
				Description: "Seek scroll one row up.",
			},
			term.KeyComb{Ch: 'h'}: {
				ID:          "Normal.SeekLeft",
				Description: "Seek scroll one column to the left.",
			},
			term.KeyComb{Ch: 'l'}: {
				ID:          "Normal.SeekRight",
				Description: "Seek scroll one column to the right.",
			},
			term.KeyComb{Ch: '/'}: {
				ID:          "Normal.SetSearchMode",
				Description: "Enter search mode. After typing search text, press ENTER to perform a text-search or ESC to go back to normal mode.",
			},
			term.KeyComb{Key: term.KeyEsc}: {
				ID: "Search.SetNormalMode", Description: "Enter normal mode",
			},
			term.KeyComb{Key: term.KeyEnter}: {
				ID: "Search.Search", Description: "Perform text search with current search buffer.",
			},
		},
	}
}

// ShowCommandBar determines whether the command bar should be
// displayed or not.
func (l *Less) ShowCommandBar(show bool) {
	l.config.NoBar = !show
}

func (l *Less) searchHandleEvent(ev term.Event) (bool, bool) {
	if ev.Type != term.EventKey || ev.Mod != 0 {
		return false, false
	}

	switch ev.Key {
	case term.KeyBackspace:
		if l.cursorOffset > 1 {
			l.cursorOffset--
			l.searchScroll.Buffer().
				DeleteCell(term.Coordinates{X: l.cursorOffset, Y: 0})
			_, cmdBarHeight := l.cmdBarHeight()
			l.resizeSearchScroll(cmdBarHeight)
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
		l.searchScroll.Buffer().WriteString(string(ev.Ch))
		_, cmdBarHeight := l.cmdBarHeight()
		l.resizeSearchScroll(cmdBarHeight)
	}

	if l.config.SuperimposeMessage && l.scroll.Attributes != l.usedSearchBarAttr {
		l.updateSearchBarAttr()
	}
	return false, true
}

func (l *Less) updateSearchBarAttr() {
	l.usedSearchBarAttr = l.scroll.Attributes
	l.searchScroll.Attributes = l.usedSearchBarAttr
}

func (l *Less) normalHandleEvent(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey || ev.Mod != 0 {
		return
	}

	switch ev.Ch {
	case 'q':
		exit = true
		handled = true
	case 'N':
		handled = l.scroll.SeekPrevResult()
	case 'n':
		handled = l.scroll.SeekNextResult()
	case '0':
		handled = l.scroll.SeekStartLine()
	case '$':
		handled = l.scroll.SeekEndLine()
	case 'g':
		handled = l.scroll.SeekStartFile()
	case 'G':
		handled = l.scroll.SeekEndFile()
	case 'j':
		handled = l.scroll.SeekDown()
	case 'k':
		handled = l.scroll.SeekUp()
	case 'h':
		handled = l.scroll.SeekLeft()
	case 'l':
		handled = l.scroll.SeekRight()
	case '/':
		l.SetSearchMode()
		handled = true
	default:
		switch ev.Key {
		case term.KeyEsc:
			exit = true
			handled = true
		default:
			handled = false
		}
	}

	return
}

func (l *Less) setMessage(msg string) bool {
	// if bar is going to be limited to its strict width
	// ensure the backgrounds blend. Use scroll Attributes
	// so dynamically changed background attributes are captured.
	attr := l.config.BarAttr
	if l.config.SuperimposeMessage {
		attr.Bg = l.scroll.Attributes.Bg
	}
	newMsg := component.NewResponsiveString(msg, component.StringResponsiveConfig{
		StringConfig: component.StringConfig{
			Alignment:            component.SpanAlignmentRight,
			Attributes:           attr,
			BackgroundAttributes: attr,
		},
	})
	shouldResize := l.msg == nil ||
		l.msg.Height(l.width) != newMsg.Height(l.width) ||
		l.usedMsgBarAttr != attr

	l.usedMsgBarAttr = attr
	l.msg = newMsg
	l.msgVirt.C = l.msg
	l.msgStr = msg
	return shouldResize
}

func (l *Less) cmdBarHeight() (cmdBarWidth, cmdBarHeight int) {
	if !l.config.NoBar {
		cmdBarWidth = l.width
		if l.config.SuperimposeMessage {
			cmdBarWidth = int(math.Min(float64(l.width), float64(len(l.msgStr))))
		}
		cmdBarHeight = l.msg.Height(cmdBarWidth)
		if l.height <= cmdBarHeight {
			cmdBarHeight = 1
		}
	}
	return
}

func (l *Less) resizeSearchScroll(cmdBarHeight int) {
	l.searchScrollVirt.Resize(l.width-len(l.msgStr), cmdBarHeight)
}

func (l *Less) resize() {
	cmdBarWidth, cmdBarHeight := l.cmdBarHeight()
	contentHeight := l.height - cmdBarHeight

	// allow content to be superimposed on bar
	if cmdBarWidth != l.width {
		contentHeight = l.height
	}
	l.scroll.Resize(l.width, contentHeight)

	l.searchScrollVirt.Move(term.Coordinates{X: 0, Y: l.height - cmdBarHeight})
	l.resizeSearchScroll(cmdBarHeight)

	l.resizeMoveMessage(cmdBarWidth, cmdBarHeight)
}

func (l *Less) resizeMoveMessage(cmdBarWidth, cmdBarHeight int) {
	// don't occlude other content if bar background is empty
	l.msgVirt.Move(term.Coordinates{X: l.width - cmdBarWidth, Y: l.height - cmdBarHeight})
	l.msgVirt.Resize(cmdBarWidth, cmdBarHeight)
}

func (l *Less) setupScroll(w *component.Scroll, attr term.Attributes) {
	w.ResultsAttr = l.config.ResAttr
	w.Wrap = l.config.Wrap
	w.Debug = l.config.Debug
	w.Attributes = attr
}

func (l *Less) initWithBuffer(buf *cell.Buffer, cfg LessConfig) {
	l.delEOF = false
	if cfg.ResAttr == (term.Attributes{}) {
		cfg.ResAttr = DefaultLessConfig().ResAttr
	}

	l.config = cfg
	l.searchScroll.Init(cell.NewBuffer())
	l.searchScrollVirt.C = &l.searchScroll

	// initialize message comps
	l.setMessage("")

	searchBarAttr := l.config.BarAttr
	l.setupScroll(&l.searchScroll, searchBarAttr)
	l.setupScroll(l.scroll, l.config.Attributes)
	if l.config.SuperimposeMessage {
		l.updateSearchBarAttr()
	}

	l.SetNormalMode()
}

func (l *Less) sendEvent(ev LessEvent) {
	if l.config.Handler != nil {
		l.config.Handler(ev)
	}
}
