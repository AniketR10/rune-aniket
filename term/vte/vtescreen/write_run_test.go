// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package vtescreen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/vte/vteparser"
)

func TestWriteRun(t *testing.T) {
	newBuf := func() *PrimaryBuffer {
		b := NewPrimaryBuffer(0, testHistory)
		b.SetDefaultChar(' ')
		b.Resize(10, 3)
		return b
	}

	t.Run("writes cells with cursor attributes", func(t *testing.T) {
		b := newBuf()
		attr := term.Attributes{Fg: term.ColorRed}
		b.SetCursorAttributes(attr)
		n := b.WriteRun([]byte("hello"), vteparser.CharsetIndexG0)
		require.Equal(t, 5, n)
		assert.Equal(t, "hello", cellsRowString(b, 0)[:5])
		for x := range 5 {
			cell := b.CellAt(term.Coordinates{X: x})
			assert.Equal(t, term.ColorRed, cell.Fg)
			assert.Equal(t, uint8(1), cell.Width)
		}
	})

	t.Run("writes at the cursor position", func(t *testing.T) {
		b := newBuf()
		resetPrimaryBuffer(b, "aaaaaaaaaa\nbbbbbbbbbb\ncccccccccc")
		b.SetCursorAtScreen(term.Coordinates{X: 3, Y: 1}, false)
		n := b.WriteRun([]byte("xy"), vteparser.CharsetIndexG0)
		require.Equal(t, 2, n)
		assert.Equal(t, "bbbxy", cellsRowString(b, 1)[:5])
	})

	t.Run("falls back on concealed cursor", func(t *testing.T) {
		b := newBuf()
		b.SetHiddenCursor(true)
		assert.Equal(t, 0, b.WriteRun([]byte("hi"), vteparser.CharsetIndexG0))
	})

	t.Run("falls back on line drawing charset", func(t *testing.T) {
		b := newBuf()
		b.ConfigureCharset(vteparser.CharsetIndexG0,
			vteparser.StandardCharsetSpecialCharacterAndLineDrawing)
		assert.Equal(t, 0, b.WriteRun([]byte("lq"), vteparser.CharsetIndexG0))
	})

	t.Run("falls back on wide cell in target", func(t *testing.T) {
		b := newBuf()
		b.Write('漢', 2, vteparser.CharsetIndexG0)
		assert.Equal(t, 0, b.WriteRun([]byte("ab"), vteparser.CharsetIndexG0))
	})

	t.Run("falls back when run exceeds row length", func(t *testing.T) {
		b := newBuf()
		assert.Equal(t, 0, b.WriteRun(make([]byte, 11), vteparser.CharsetIndexG0))
	})
}
