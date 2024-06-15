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
package parser

import (
	"github.com/unstablebuild/tcell/v3"
)

// Handler abstracts a terminal TUI implementation.
type Handler interface {
	// SetTitle sets the terminal's window title.
	SetTitle(title string)

	// SetCursorStyle Set the cell cursor style.
	SetCursorStyle(style CursorStyle)

	// SetCursorShape sets the cell cursor shape.
	SetCursorShape(shape CursorShape)

	// Input sets the character to be displayed at the current cell.
	Input(c rune)

	// Goto sets the cell cursor to the given position.
	Goto(line int, col int)

	// GotoLine sets the cell cursor to specific row.
	GotoLine(line int)

	// GotoCol sets the cell cursor to specific column.
	GotoCol(col int)

	// Insertblank inserts blank characters in current line starting from cell cursor.
	InsertBlank(count int)

	// MoveUp moves up the cell cursor `rows`.
	MoveUp(rows int)

	// MoveDown moves down the cell cursor `rows`.
	MoveDown(rows int)

	// IdentifyTerminal identifies the terminal on the pty stream.
	IdentifyTerminal(identifySecondary bool)

	// DeviceStatus writes the device status on the pty stream.
	DeviceStatus(status int)

	// MoveForward moves the cell cursor forward `cols`.
	MoveForward(cols int)

	// MoveBackward moves the cell cursor backward `cols`.
	MoveBackward(cols int)

	// MoveDownAndCR moves the cell cursor down `rows` and set to column 1.
	MoveDownAndCR(rows int)

	// MoveUpAndCR moves the cell cursor up `rows` and set to column 1.
	MoveUpAndCR(rows int)

	// PutTab sets the current cell cursor's position to be `count` number of tabs.
	PutTab()

	// Backspace deletes backward one character.
	Backspace()

	// CarriageReturn performes a carriage return.
	CarriageReturn()

	// Linefeed performs a line feed.
	Linefeed()

	// Bell ring the system bell.
	Bell()

	// Substitute subsitutes the char under the cell cursor.
	Substitute()

	// SetHorizontalTabstop sets current position as a tabstop.
	SetHorizontalTabstop()

	// ScrollUp scrolls up `rows` rows.
	ScrollUp(rows int)

	// ScrollDown scrolls down `rows` rows.
	ScrollDown(rows int)

	// InsertBlankLines insert `count` blank lines.
	InsertBlankLines(count int)

	// DeleteLines delete `count` lines.
	DeleteLines(count int)

	// EraseChars erases `count` chars in the current line following the cursor.
	//
	// Erase means resetting to the default state (default colors, no content,
	// no mode flags).
	EraseChars(count int)

	// DeleteChars deletes `count` chars.
	//
	// Deleting a character is like the delete key on the keyboard - everything
	// to the right of the deleted things is shifted left.
	DeleteChars(count int)

	// MoveBackwardTabs moves backward `count` tabs.
	MoveBackwardTabs(count int)

	// MoveForwardTabs moves forward `count` tabs.
	MoveForwardTabs(count int)

	// SaveCursorPosition saves the current cursor position.
	SaveCursorPosition()

	// RestoreCursorPosition restores the cursor position.
	RestoreCursorPosition()

	// ClearLine clears the current line.
	ClearLine(mode LineClearMode)

	// ClearScreen clears the screen.
	ClearScreen(mode ClearMode)

	// ClearTabs clears tab stops.
	ClearTabs(mode TabulationClearMode)

	// ResetState resets the terminal's state.
	ResetState()

	// ReverseIndex moves the active position to the same horizontal position on the
	// preceding line. If the active position is at the top margin, a scroll
	// down is performed.
	ReverseIndex()

	// TerminalAttribute sets a terminal attribute at the current cursor position.
	TerminalAttribute(attr Attr)

	// SetMode sets an parser.Mode.
	SetMode(mode Mode)

	// UnsetMode unsets an parser.Mode.
	UnsetMode(mode Mode)

	// ReportMode reports an parser.Mode back into the pty stream.
	ReportMode(mode Mode)

	// SetPrivate sets an parser.PrivateMode.
	SetPrivateMode(PrivateMode)

	// UnsetPrivate unsets an parser.PrivateMode.
	UnsetPrivateMode(PrivateMode)

	// ReportPrivateMode (DECRPM) reports an parser.PrivateMode back into the pty stream.
	ReportPrivateMode(PrivateMode)

	// SetScrollingRegion (DECSTBM) sets the terminal scrolling region.
	SetScrollingRegion(top, bottom int, end bool)

	// SetKeypadApplicationMode (DECKPAM) sets the keypad to applications
	// mode (ESCape instead of digits).
	SetKeypadApplicationMode()

	// UnsetKeypadApplicationMode (DECKPNM) sets the keypad to numeric
	// mode (digits instead of ESCape seq).
	UnsetKeypadApplicationMode()

	// SetActiveCharset sets one of the graphic character sets,
	// G0 to G3, as the active charset.
	//
	// 'Invoke' one of G0 to G3 in the GL area. Also referred to as shift in,
	// shift out and locking shift depending on the set being activated.
	SetActiveCharset(index CharsetIndex)

	// ConfigureCharset assigns a graphic character set to G0, G1, G2 or G3.
	//
	// 'Designate' a graphic character set as one of G0 to G3 so that it can
	// later be 'invoked' by `SetActiveCharset`.
	ConfigureCharset(index CharsetIndex, charset StandardCharset)

	// ClipboardStore stores data into the clipboard.
	ClipboardStore(register int, data []byte)

	// ClipboardLoad loads data from the clipboard.
	ClipboardLoad(register int, data string)

	// Decaln runs the decaln routine.
	Decaln()

	// PushTitle pushes a title onto the stack.
	PushTitle()

	// PopTitle pops the last title from the stack.
	PopTitle()

	// TextAreaSizePixels reports text area size in pixels.
	TextAreaSizePixels()

	// TextAreaSizeChars reports text area size in characters.
	TextAreaSizeChars()

	// SetHyperlink sets hyperlink.
	SetHyperlink(link *Hyperlink)

	// ReportKeyboardMode reports current keyboard mode.
	ReportKeyboardMode()

	// PushKeyboardMode pushes the keyboard mode into the keyboard mode stack.
	PushKeyboardMode(mode KeyboardMode)

	// PopKeyboardModes pops the given amount of keyboard modes
	// from the keyboard mode stack.
	PopKeyboardModes(count int)

	// SetKeyboardMode sets the [`keyboard mode`] using the given [`behavior`].
	SetKeyboardMode(mode KeyboardMode, behavior KeyboardModesApplyBehavior)

	// SetModifyOtherKeys sets XTerm's [`ModifyOtherKeys`] option.
	SetModifyOtherKeys(mode ModifyOtherKeysMode)

	// ReportModifyOtherKeys report XTerm's [`ModifyOtherKeys`] state
	// back into the pty stream.
	ReportModifyOtherKeys()
}

// KeyboardMode
type KeyboardMode uint8

const (
	// No keyboard protocol mode is set.
	KeyboardModeNoMode KeyboardMode = 0b0000_0000
	// Report `Esc`, `alt` + `key`, `ctrl` + `key`, `ctrl` + `alt` + `key`, `shift`
	// + `alt` + `key` keys using `CSI u` sequence instead of raw ones.
	KeyboardModeDisambiguateEscCodes KeyboardMode = 0b0000_0001
	// Report key presses, release, and repetition alongside the escape. Key events
	// that result in text are reported as plain UTF-8, unless
	// KeyboardModeReportAllKeysAsEsc is enabled.
	KeyboardModeReportEventTypes KeyboardMode = 0b0000_0010
	// Report shifted key an dbase layout key.
	KeyboardModeReportAlternateKeys KeyboardMode = 0b0000_0100
	// Report every key as an escape sequence.
	KeyboardModeReportAllKeysAsEsc KeyboardMode = 0b0000_1000
	// Report the text generated by the key event.
	KeyboardModeReportAssociatedText KeyboardMode = 0b0001_0000
)

// KeyboardModesApplyBehaviour encodes how to apply parsed keyboard modes.
type KeyboardModesApplyBehavior uint8

const (
	// Replace the active flags with the new ones.
	KeyboardModesApplyBehaviorReplace KeyboardModesApplyBehavior = iota
	// Merge the given flags with currently active ones.
	KeyboardModesApplyBehaviorUnion
	// Remove the given flags from the active ones.
	KeyboardModesApplyBehaviorDifference
)

type Hyperlink struct {
	// Identifier for the given hyperlink.
	ID string
	// Resource identifier of the hyperlink.
	URI string
}

// CursorShape represents the terminal cursor shape.
type CursorShape int

const (
	CursorShapeBlock CursorShape = iota
	CursorShapeUnderline
	CursorShapeBeam
	CursorShapeHollowBlock
	CursorShapeHidden
)

// CursorStyle represents the terminal cursor configuration.
type CursorStyle struct {
	Shape    CursorShape
	Blinking bool
}

// Mode represents terminal modes.
type Mode int

const (
	ModeInsert Mode = 4
	// https://vt100.net/docs/vt510-rm/LNM.html
	ModeLineFeedNewLine Mode = 20
)

// NewMode maps a param to a Mode.
func NewMode(param uint16) Mode {
	// do not validate here, so we avoid having to make Mode
	// into a struct that holds the raw value as well as the named mode.
	return Mode(param)
}

// NewPrivateMode maps a param to a (private) Mode.
func NewPrivateMode(param uint16) PrivateMode {
	// do not validate here, so we avoid having to make PrivateMode
	// into a struct that holds the raw value as well as the named mode.
	return PrivateMode(param)
}

// LineClearMode is a mode of clearing a line,
// relative to the cell cursor.
type LineClearMode int

const (
	// LineClearModeRight clears right of cursor.
	LineClearModeRight LineClearMode = iota
	// LineClearModeLeft clears left of cursor.
	LineClearModeLeft
	// LineClearModeAll clears an entire line.
	LineClearModeAll
)

// ClearMode for clearing a terminal, relative to the cell cursor.
type ClearMode int

const (
	// ClearModeBelow clears below cursor.
	ClearModeBelow ClearMode = iota
	// ClearModeAbove clears above cursor.
	ClearModeAbove
	// ClearModeAll clears entire terminal.
	ClearModeAll
	// ClearModeSaved clears 'saved' lines (scrollback).
	ClearModeSaved
)

// TabulationClearMode for clearing tab stops.
type TabulationClearMode int

const (
	// TabulationClearModeCurrent clears stop under cursor.
	TabulationClearModeCurrent TabulationClearMode = iota
	// TabulationClearModeAll clears all stops.
	TabulationClearModeAll
)

// Attr is a terminal cell attributes.
type Attr struct {
	Type  AttrType
	Color tcell.Color
}

// AttrType encodes the type of attribute in an Attr.
type AttrType uint8

const (
	// Clear all special abilities.
	ResetAttr AttrType = iota
	// Bold text.
	BoldAttr
	// Dim or secondary color.
	DimAttr
	// Italic text.
	ItalicAttr
	// Underline text.
	UnderlineAttr
	// Underlined twice.
	DoubleUnderlineAttr
	// Undercurled text.
	UndercurlAttr
	// Dotted underlined text.
	DottedUnderlineAttr
	// Dashed underlined text.
	DashedUnderlineAttr
	// Blink cursor slowly.
	BlinkSlowAttr
	// Blink cursor fast.
	BlinkFastAttr
	// Invert colors.
	ReverseAttr
	// Do not display characters.
	HiddenAttr
	// Strikeout text.
	StrikeAttr
	// Cancel bold.
	CancelBoldAttr
	// Cancel bold and dim.
	CancelBoldDimAttr
	// Cancel italic.
	CancelItalicAttr
	// Cancel all underlines.
	CancelUnderlineAttr
	// Cancel blink.
	CancelBlinkAttr
	// Cancel inversion.
	CancelReverseAttr
	// Cancel text hiding.
	CancelHiddenAttr
	// Cancel strikeout.
	CancelStrikeAttr
	// ForegroundAttr sets foreground color.
	ForegroundAttr
	// BackgroundAttr sets background color.
	BackgroundAttr
	// Underline color.
	UnderlineColorAttr
)

// PrivateMode defines named private DEC modes as constants.
type PrivateMode int16

// List of named private DEC modes.
const (
	PrivateModeCursorKeys                    PrivateMode = 1
	PrivateModeColumnMode                    PrivateMode = 3
	PrivateModeScreen                        PrivateMode = 5
	PrivateModeOrigin                        PrivateMode = 6
	PrivateModeLineWrap                      PrivateMode = 7
	PrivateModeBlinkingCursor                PrivateMode = 12
	PrivateModeShowCursor                    PrivateMode = 25
	PrivateModeReportMouseClicks             PrivateMode = 1000
	PrivateModeReportCellMouseMotion         PrivateMode = 1002
	PrivateModeReportAllMouseMotion          PrivateMode = 1003
	PrivateModeReportFocusInOut              PrivateMode = 1004
	PrivateModeUtf8Mouse                     PrivateMode = 1005
	PrivateModeSgrMouse                      PrivateMode = 1006
	PrivateModeAlternateScroll               PrivateMode = 1007
	PrivateModeUrgencyHints                  PrivateMode = 1042
	PrivateModeSwapScreenAndSetRestoreCursor PrivateMode = 1049
	PrivateModeBracketedPaste                PrivateMode = 2004
	PrivateModeSyncUpdate                    PrivateMode = 2026
)

// ModifyOtherKeysMode represents one of Xterm's `ModifyOtherKeys` option.
type ModifyOtherKeysMode uint8

const (
	ModifyOtherKeysReset ModifyOtherKeysMode = iota
	ModifyOtherKeysEnableExceptWellDefined
	ModifyOtherKeysEnableAll
)

// CharsetIndex represents the index identifider of a StandardCharset.
type CharsetIndex int

const (
	CharsetIndexG0 CharsetIndex = iota
	CharsetIndexG1
	CharsetIndexG2
	CharsetIndexG3
)

// NewKeyboardMode wraps param into a Keyboard mode without any validation.
func NewKeyboardMode(param uint8) KeyboardMode {
	return KeyboardMode(param)
}
