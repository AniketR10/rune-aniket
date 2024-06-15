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
package term

import (
	"bytes"
	"context"
	"fmt"
)

var _ Writer = (*StringWriter)(nil)

// StringWriter satisfies Writer by rendering the cells into a plain string.
type StringWriter struct {
	cellbuf       []Cell
	buffer        bytes.Buffer
	width, height int
	CursorCh      rune
	SetContext    context.Context
}

// NewStringWriter allocates storage for a new StringWriter and
// itnializes it.
func NewStringWriter(width, height int) (t *StringWriter) {
	t = new(StringWriter)
	t.Init(width, height)
	return
}

func (w *StringWriter) Init(width, height int) {
	w.Resize(width, height)
	w.CursorCh = '▐'
	w.SetContext = context.Background()
}

// Context returns context.Background
func (w *StringWriter) Context() context.Context {
	return w.SetContext
}

// Resize satisfies Writer.
func (w *StringWriter) Resize(width, height int) {
	w.width, w.height = width, height
	w.cellbuf = make([]Cell, width*height)
}

// Reset resets this writer.
func (w *StringWriter) Reset() {
	w.Init(w.width, w.height)
	w.buffer.Reset()
}

func outOfBounds(height, width int, pos Coordinates) bool {
	return pos.X >= width || pos.Y >= height || pos.X < 0 || pos.Y < 0
}

// SetCell satisfies Writer.
func (w *StringWriter) SetCell(pos Coordinates, cell Cell) {
	if outOfBounds(w.height, w.width, pos) {
		panic(fmt.Sprintf("SetCell(x=%d;y=%d): out of bounds: width=%d;height=%d",
			pos.X, pos.Y, w.width, w.height))
	}
	idx := pos.Y*w.width + pos.X
	w.cellbuf[idx] = cell
}

func (w *StringWriter) UnionAttributes(pos Coordinates, attr Attributes) {
	if outOfBounds(w.height, w.width, pos) {
		panic(fmt.Sprintf("SetCell(x=%d;y=%d): out of bounds: width=%d;height=%d",
			pos.X, pos.Y, w.width, w.height))
	}
	idx := pos.Y*w.width + pos.X
	w.cellbuf[idx].Attributes = AttributesUnion(w.cellbuf[idx].Attributes, attr)
}

// Flush satisfies Writer.
func (w *StringWriter) Flush() (err error) {
	for i, c := range w.cellbuf {
		if i != 0 && i%w.width == 0 {
			w.buffer.WriteRune('\n')
		}
		ch := c.Ch
		switch ch {
		case '\t', '\n', 0:
			ch = ' '
		}
		w.buffer.WriteRune(ch)
	}
	return
}

// Cells returns the internal cell slice.
func (w *StringWriter) Cells() []Cell {
	return w.cellbuf
}

// Clear satisfies Writer. Note that attr are ignored as they
// can't be represented in a string.
func (w *StringWriter) Clear(attr Attributes) (err error) {
	w.cellbuf = make([]Cell, w.width*w.height)
	w.buffer.Reset()
	return
}

func (w *StringWriter) String() string {
	return w.buffer.String()
}

// SetCursor satisfies Writer by substituting the rune
// at pos for a pre-defined cursor-like rune.
func (w *StringWriter) SetCursor(pos Coordinates) {
	i := pos.X + pos.Y*w.width
	if i < len(w.cellbuf) {
		w.cellbuf[i].Ch = w.CursorCh
	}
}
