//go:build !js

package term

import (
	"github.com/unstablebuild/tcell/v3/termbox"
)

// Event type. See Event.Type field.
const (
	EventKey        EventType = EventType(termbox.EventKey)
	EventResize               = EventType(termbox.EventResize)
	EventMouse                = EventType(termbox.EventMouse)
	EventError                = EventType(termbox.EventError)
	EventInterrupt            = EventType(termbox.EventInterrupt)
	EventRaw                  = EventType(termbox.EventRaw)
	EventNone                 = EventType(termbox.EventNone)
	EventPasteStart           = EventType(termbox.EventPasteStart)
	EventPasteEnd             = EventType(termbox.EventPasteEnd)
	EventFocus                = EventType(termbox.EventFocus)
	EventUnfocus              = EventType(termbox.EventUnfocus)
)

const (
	KeyF1          Key = Key(termbox.KeyF1)
	KeyF2              = Key(termbox.KeyF2)
	KeyF3              = Key(termbox.KeyF3)
	KeyF4              = Key(termbox.KeyF4)
	KeyF5              = Key(termbox.KeyF5)
	KeyF6              = Key(termbox.KeyF6)
	KeyF7              = Key(termbox.KeyF7)
	KeyF8              = Key(termbox.KeyF8)
	KeyF9              = Key(termbox.KeyF9)
	KeyF10             = Key(termbox.KeyF10)
	KeyF11             = Key(termbox.KeyF11)
	KeyF12             = Key(termbox.KeyF12)
	KeyInsert          = Key(termbox.KeyInsert)
	KeyDelete          = Key(termbox.KeyDelete)
	KeyHome            = Key(termbox.KeyHome)
	KeyEnd             = Key(termbox.KeyEnd)
	KeyPgup            = Key(termbox.KeyPgup)
	KeyPgdn            = Key(termbox.KeyPgdn)
	KeyArrowUp         = Key(termbox.KeyArrowUp)
	KeyArrowDown       = Key(termbox.KeyArrowDown)
	KeyArrowLeft       = Key(termbox.KeyArrowLeft)
	KeyArrowRight      = Key(termbox.KeyArrowRight)
	MouseLeft          = Key(termbox.MouseLeft)
	MouseMiddle        = Key(termbox.MouseMiddle)
	MouseRight         = Key(termbox.MouseRight)
	MouseRelease       = Key(termbox.MouseRelease)
	MouseWheelUp       = Key(termbox.MouseWheelUp)
	MouseWheelDown     = Key(termbox.MouseWheelDown)
	KeyBackspace       = Key(termbox.KeyBackspace2)
	KeyTab             = Key(termbox.KeyTab)
	KeyEnter           = Key(termbox.KeyEnter)
	KeyEsc             = Key(termbox.KeyEsc)
	KeySpace           = Key(termbox.KeySpace)
)

// Input mode. See SetInputMode function.
const (
	InputEsc     InputMode = InputMode(termbox.InputEsc)
	InputAlt               = InputMode(termbox.InputAlt)
	InputMouse             = InputMode(termbox.InputMouse)
	InputCurrent           = InputMode(termbox.InputCurrent)
)

// Alt modifier constant, see Event.Mod field and SetInputMode function.
const (
	ModAlt           Modifier = Modifier(termbox.ModAlt)
	ModShift                  = Modifier(0x10)
	ModMeta                   = Modifier(0x12)
	ModCtrl                   = Modifier(0x11)
	ModCtrlShift              = Modifier(0x91)
	ModCtrlAlt                = Modifier(0x92)
	ModCtrlMeta               = Modifier(0x93)
	ModCtrlShiftAlt           = Modifier(0x95)
	ModCtrlShiftMeta          = Modifier(0x94)
	ModCtrlAltMeta            = Modifier(0x97)
	ModShiftMeta              = Modifier(0x81)
	ModAltMeta                = Modifier(0x82)
	ModAltShiftMeta           = Modifier(0x83)
	ModAltShift               = Modifier(0x71)
)
