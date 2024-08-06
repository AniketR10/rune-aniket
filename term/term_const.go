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
