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

package browserrpc

import handlerrpc "unstable.build/go-tui/handler/handlerrpc"

/* boilerplate so we can re-use handlerrpc stream implementation */

// SetDraw satisfies handlerrpc.StreamMessage.
func (m *FloatingWindowMessage) SetDraw(r *handlerrpc.DrawStreamResponse) {
	m.Draw = r
	m.Type = handlerrpc.MessageType_Draw
}

// SetResize satisfies handlerrpc.StreamMessage.
func (m *FloatingWindowMessage) SetResize(r *handlerrpc.ResizeStreamResponse) {
	m.Resize = r
	m.Type = handlerrpc.MessageType_Resize
}

// SetHandle satisfies handlerrpc.StreamMessage.
func (m *FloatingWindowMessage) SetHandle(r *handlerrpc.HandleStreamResponse) {
	m.Handle = r
	m.Type = handlerrpc.MessageType_Handle
}

// SetMan satisfies handlerrpc.StreamMessage.
func (m *FloatingWindowMessage) SetMan(r *handlerrpc.ManStreamResponse) {
	m.Man = r
	m.Type = handlerrpc.MessageType_Man
}

// SetClose satisfies handlerrpc.StreamMessage.
func (m *FloatingWindowMessage) SetClose(r *handlerrpc.CloseStreamResponse) {
	m.Close = r
	m.Type = handlerrpc.MessageType_Close
}

// SetCursor satisfies handlerrpc.StreamMessage.
func (m *FloatingWindowMessage) SetCursor(r *handlerrpc.CursorStreamResponse) {
	m.Cursor = r
	m.Type = handlerrpc.MessageType_Cursor
}

// SetSelection satisfies handlerrpc.StreamMessage.
func (m *FloatingWindowMessage) SetSelection(r *handlerrpc.SelectionStreamResponse) {
	m.Selection = r
	m.Type = handlerrpc.MessageType_Selection
}

// SetDimensions satisfies handlerrpc.StreamMessage.
func (m *FloatingWindowMessage) SetDimensions(r *handlerrpc.DimensionsStreamResponse) {
	m.Dimensions = r
	m.Type = handlerrpc.MessageType_Dimensions
}

// SetDraw satisfies handlerrpc.StreamMessage.
func (m *SplitWindowMessage) SetDraw(r *handlerrpc.DrawStreamResponse) {
	m.Draw = r
	m.Type = handlerrpc.MessageType_Draw
}

// SetResize satisfies handlerrpc.StreamMessage.
func (m *SplitWindowMessage) SetResize(r *handlerrpc.ResizeStreamResponse) {
	m.Resize = r
	m.Type = handlerrpc.MessageType_Resize
}

// SetHandle satisfies handlerrpc.StreamMessage.
func (m *SplitWindowMessage) SetHandle(r *handlerrpc.HandleStreamResponse) {
	m.Handle = r
	m.Type = handlerrpc.MessageType_Handle
}

// SetMan satisfies handlerrpc.StreamMessage.
func (m *SplitWindowMessage) SetMan(r *handlerrpc.ManStreamResponse) {
	m.Man = r
	m.Type = handlerrpc.MessageType_Man
}

// SetClose satisfies handlerrpc.StreamMessage.
func (m *SplitWindowMessage) SetClose(r *handlerrpc.CloseStreamResponse) {
	m.Close = r
	m.Type = handlerrpc.MessageType_Close
}

// SetCursor satisfies handlerrpc.StreamMessage.
func (m *SplitWindowMessage) SetCursor(r *handlerrpc.CursorStreamResponse) {
	m.Cursor = r
	m.Type = handlerrpc.MessageType_Cursor
}

// SetSelection satisfies handlerrpc.StreamMessage.
func (m *SplitWindowMessage) SetSelection(r *handlerrpc.SelectionStreamResponse) {
	m.Selection = r
	m.Type = handlerrpc.MessageType_Selection
}

// SetDimensions satisfies handlerrpc.StreamMessage.
func (m *SplitWindowMessage) SetDimensions(r *handlerrpc.DimensionsStreamResponse) {
	m.Dimensions = r
	m.Type = handlerrpc.MessageType_Dimensions
}
