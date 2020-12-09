// +build wasm,js

package term

// Key constants, see Event.Key field.
const (
	KeyF1         Key = 0x70
	KeyF2             = 0x71
	KeyF3             = 0x72
	KeyF4             = 0x73
	KeyF5             = 0x74
	KeyF6             = 0x75
	KeyF7             = 0x76
	KeyF8             = 0x77
	KeyF9             = 0x78
	KeyF10            = 0x79
	KeyF11            = 0x7a
	KeyF12            = 0x7b
	KeyInsert         = 0x2d
	KeyDelete         = 0x2e
	KeyHome           = 0x24
	KeyEnd            = 0x23
	KeyPgup           = 0x21
	KeyPgdn           = 0x22
	KeyArrowUp        = 0x26
	KeyArrowDown      = 0x28
	KeyArrowLeft      = 0x25
	KeyArrowRight     = 0x27
	KeyBackspace      = 0x08
	KeyTab            = 0x09
	KeyEnter          = 0x0D
	KeyEsc            = 0x1B
	KeySpace          = 0x20
	KeyBackspace2     = 0x7F

	MouseLeft      = 0xFFF1
	MouseMiddle    = 0xFFF2
	MouseRight     = 0xFFF3
	MouseRelease   = 0xFFF4
	MouseWheelUp   = 0xFFF5
	MouseWheelDown = 0xFFF6

	KeyCtrlTilde      Key = 0xC0
	KeyCtrl2          Key = 0xC0
	KeyCtrlSpace      Key = 0xC0
	KeyCtrlA          Key = 0xC1
	KeyCtrlB          Key = 0xC2
	KeyCtrlC          Key = 0xC3
	KeyCtrlD          Key = 0xC4
	KeyCtrlE          Key = 0xC5
	KeyCtrlF          Key = 0xC6
	KeyCtrlG          Key = 0xC7
	KeyCtrlH          Key = 0xC8
	KeyCtrlI          Key = 0xC9
	KeyCtrlJ          Key = 0xCA
	KeyCtrlK          Key = 0xCB
	KeyCtrlL          Key = 0xCC
	KeyCtrlM          Key = 0xCD
	KeyCtrlN          Key = 0xCE
	KeyCtrlO          Key = 0xCF
	KeyCtrlP          Key = 0xD0
	KeyCtrlQ          Key = 0xD1
	KeyCtrlR          Key = 0xD2
	KeyCtrlS          Key = 0xD3
	KeyCtrlT          Key = 0xD4
	KeyCtrlU          Key = 0xD5
	KeyCtrlV          Key = 0xD6
	KeyCtrlW          Key = 0xD7
	KeyCtrlX          Key = 0xD8
	KeyCtrlY          Key = 0xD9
	KeyCtrlZ          Key = 0xDA
	KeyCtrlLsqBracket Key = 0xDB
	KeyCtrl3          Key = 0xDB
	KeyCtrl4          Key = 0xDC
	KeyCtrlBackslash  Key = 0xDC
	KeyCtrl5          Key = 0xDD
	KeyCtrlRsqBracket Key = 0xDD
	KeyCtrl6          Key = 0xDE
	KeyCtrl7          Key = 0xDF
	KeyCtrlSlash      Key = 0xDF
	KeyCtrlUnderscore Key = 0xDF
	KeyCtrl8          Key = 0x7F
)

// Alt modifier constant, see Event.Mod field and SetInputMode function.
const (
	ModMotion Modifier = 0x01
	ModAlt             = 0x12
	modShift           = 0x10
	modCtrl            = 0x11
)

// Cell colors, you can combine a color with multiple attributes using bitwise
// OR ('|').
const (
	ColorDefault Attribute = iota
	ColorBlack
	ColorRed
	ColorGreen
	ColorYellow
	ColorBlue
	ColorMagenta
	ColorCyan
	ColorWhite
)

// Cell attributes, it is possible to use multiple attributes by combining them
// using bitwise OR ('|'). Although, colors cannot be combined. But you can
// combine attributes and a single color.
//
// It's worth mentioning that some platforms don't support certain attributes.
// For example windows console doesn't support AttrUnderline. And on some
// terminals applying AttrBold to background may result in blinking text. Use
// them with caution and test your code on various terminals.
const (
	AttrBold Attribute = 1 << (iota + 9)
	AttrUnderline
	AttrReverse
)

// Input mode. See SetInputMode function.
const (
	InputEsc InputMode = 1 << iota
	InputAlt
	InputMouse
	InputCurrent InputMode = 0
)

// Output mode. See SetOutputMode function.
const (
	OutputCurrent OutputMode = iota
	OutputNormal
	Output256
	Output216
	OutputGrayscale
)

// Event type. See Event.Type field.
const (
	EventKey EventType = iota
	EventResize
	EventMouse
	EventError
	EventInterrupt
	EventRaw
	EventNone
)
