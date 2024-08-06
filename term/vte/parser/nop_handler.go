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

// NopHandler returns an implementation of Handler that does nothing.
func NopHandler() Handler {
	return nopHandler{}
}

type nopHandler struct{}

func (h nopHandler) SetTitle(title string) {

}

func (h nopHandler) SetCursorStyle(style CursorStyle) {

}

func (h nopHandler) SetCursorShape(shape CursorShape) {

}

func (h nopHandler) Input(c rune) {

}

func (h nopHandler) Goto(line int, col int) {

}

func (h nopHandler) GotoLine(line int) {

}

func (h nopHandler) GotoCol(col int) {

}

func (h nopHandler) InsertBlank(count int) {

}

func (h nopHandler) MoveUp(rows int) {

}

func (h nopHandler) MoveDown(rows int) {

}

func (h nopHandler) IdentifyTerminal(identifySecondary bool) {

}

func (h nopHandler) DeviceStatus(status int) {

}

func (h nopHandler) MoveForward(cols int) {

}

func (h nopHandler) MoveBackward(cols int) {

}

func (h nopHandler) MoveDownAndCR(row int) {

}

func (h nopHandler) MoveUpAndCR(row int) {

}

func (h nopHandler) PutTab() {

}

func (h nopHandler) Backspace() {

}

func (h nopHandler) CarriageReturn() {

}

func (h nopHandler) Linefeed() {

}

func (h nopHandler) Bell() {

}

func (h nopHandler) Substitute() {

}

func (h nopHandler) Newline() {

}

func (h nopHandler) SetHorizontalTabstop() {

}

func (h nopHandler) ScrollUp(rows int) {

}

func (h nopHandler) ScrollDown(rows int) {

}

func (h nopHandler) InsertBlankLines(count int) {

}

func (h nopHandler) DeleteLines(count int) {

}

func (h nopHandler) EraseChars(count int) {

}

func (h nopHandler) DeleteChars(count int) {

}

func (h nopHandler) MoveBackwardTabs(count int) {

}

func (h nopHandler) MoveForwardTabs(count int) {

}

func (h nopHandler) SaveCursorPosition() {

}

func (h nopHandler) RestoreCursorPosition() {

}

func (h nopHandler) ClearLine(mode LineClearMode) {

}

func (h nopHandler) ClearScreen(mode ClearMode) {

}

func (h nopHandler) ClearTabs(mode TabulationClearMode) {

}

func (h nopHandler) ResetState() {

}

func (h nopHandler) ReverseIndex() {

}

func (h nopHandler) TerminalAttribute(attr Attr) {

}

func (h nopHandler) SetMode(mode Mode) {

}

func (h nopHandler) UnsetMode(mode Mode) {

}

func (h nopHandler) ReportMode(mode Mode) {

}

func (h nopHandler) SetPrivateMode(mode PrivateMode) {

}

func (h nopHandler) UnsetPrivateMode(mode PrivateMode) {

}

func (h nopHandler) ReportPrivateMode(mode PrivateMode) {

}

func (h nopHandler) SetScrollingRegion(top, bottom int, end bool) {

}

func (h nopHandler) SetKeypadApplicationMode() {

}

func (h nopHandler) UnsetKeypadApplicationMode() {

}

func (h nopHandler) SetActiveCharset(index CharsetIndex) {

}

func (h nopHandler) ConfigureCharset(index CharsetIndex, charset StandardCharset) {

}

func (h nopHandler) ClipboardStore(register int, data []byte) {

}

func (h nopHandler) ClipboardLoad(register int, data string) {

}

func (h nopHandler) Decaln() {

}

func (h nopHandler) PushTitle() {

}

func (h nopHandler) PopTitle() {

}

func (h nopHandler) TextAreaSizePixels() {

}

func (h nopHandler) TextAreaSizeChars() {

}

func (h nopHandler) SetHyperlink(link *Hyperlink) {

}

func (h nopHandler) ReportKeyboardMode() {

}

func (h nopHandler) PushKeyboardMode(mode KeyboardMode) {

}

func (h nopHandler) PopKeyboardModes(count int) {

}

func (h nopHandler) SetKeyboardMode(
	mode KeyboardMode, behavior KeyboardModesApplyBehavior,
) {

}

func (h nopHandler) SetModifyOtherKeys(mode ModifyOtherKeysMode) {

}

func (h nopHandler) ReportModifyOtherKeys() {

}
