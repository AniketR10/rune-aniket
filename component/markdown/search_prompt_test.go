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

package markdown

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func promptKey(ch rune) term.Event {
	return term.Event{Type: term.EventKey, Ch: ch}
}

func promptSpecial(k term.Key) term.Event {
	return term.Event{Type: term.EventKey, Key: k}
}

func TestPromptInactive(t *testing.T) {
	var p searchPrompt
	assert.False(t, p.isActive())
	assert.Equal(t, searchIgnored, p.handleKey(promptKey('a')))
}

func TestPromptOpenClose(t *testing.T) {
	var p searchPrompt
	p.open()
	assert.True(t, p.isActive())

	assert.Equal(t, searchConsumed, p.handleKey(promptKey('h')))
	assert.Equal(t, searchConsumed, p.handleKey(promptKey('i')))
	assert.Equal(t, "hi", p.query())

	assert.Equal(t, searchCancel, p.handleKey(promptSpecial(term.KeyEsc)))
	assert.False(t, p.isActive())
}

func TestPromptConfirm(t *testing.T) {
	var p searchPrompt
	p.open()
	p.handleKey(promptKey('f'))
	p.handleKey(promptKey('o'))
	p.handleKey(promptKey('o'))

	assert.Equal(t, searchConfirm, p.handleKey(promptSpecial(term.KeyEnter)))
	assert.False(t, p.isActive())
	assert.Equal(t, "foo", p.query())
}

func TestPromptBackspace(t *testing.T) {
	var p searchPrompt
	p.open()
	p.handleKey(promptKey('a'))
	p.handleKey(promptKey('b'))
	assert.Equal(t, "ab", p.query())

	p.handleKey(promptSpecial(term.KeyBackspace))
	assert.Equal(t, "a", p.query())

	// Backspace on empty buffer is a no-op.
	p.handleKey(promptSpecial(term.KeyBackspace))
	p.handleKey(promptSpecial(term.KeyBackspace))
	assert.Equal(t, "", p.query())
}

func TestPromptDraw(t *testing.T) {
	var p searchPrompt
	w := term.NewStringWriter(10, 1)

	// Inactive prompt draws nothing.
	_ = w.Clear(term.Attributes{})
	p.draw(w, 0, 10)
	_ = w.Flush()
	assert.Equal(t, "          ", w.String())

	// Active prompt draws "/{buf}".
	p.open()
	p.handleKey(promptKey('h'))
	p.handleKey(promptKey('i'))
	w.Reset()
	_ = w.Clear(term.Attributes{})
	p.draw(w, 0, 10)
	_ = w.Flush()
	assert.Equal(t, "/hi       ", w.String())
}

func TestPromptCursor(t *testing.T) {
	var p searchPrompt

	_, _, show := p.cursor(5)
	assert.False(t, show)

	p.open()
	pos, _, show := p.cursor(5)
	assert.True(t, show)
	assert.Equal(t, term.Coordinates{X: 1, Y: 5}, pos)

	p.handleKey(promptKey('a'))
	p.handleKey(promptKey('b'))
	pos, _, show = p.cursor(5)
	assert.True(t, show)
	assert.Equal(t, term.Coordinates{X: 3, Y: 5}, pos)
}

func TestPromptWideClusters(t *testing.T) {
	tests := []struct {
		name        string
		input       []rune
		wantQuery   string
		wantCursorX int
		wantCells   map[int]term.Cell
	}{
		{
			name:        "emoji query occupies two columns",
			input:       []rune{'🚀', 'x'},
			wantQuery:   "🚀x",
			wantCursorX: 4,
			wantCells: map[int]term.Cell{
				0: {Ch: '/', Width: 1},
				1: {Ch: '🚀', Width: 2},
				2: {Ch: ' ', Width: 1}, // background fill under the wide cell
				3: {Ch: 'x', Width: 1},
			},
		},
		{
			name:        "cjk query",
			input:       []rune{'中', '文'},
			wantQuery:   "中文",
			wantCursorX: 5,
			wantCells: map[int]term.Cell{
				0: {Ch: '/', Width: 1},
				1: {Ch: '中', Width: 2},
				2: {Ch: ' ', Width: 1}, // background fill under the wide cell
				3: {Ch: '文', Width: 2},
				4: {Ch: ' ', Width: 1}, // background fill under the wide cell
			},
		},
		{
			name:        "accented query stays narrow",
			input:       []rune{'é', 'a'},
			wantQuery:   "éa",
			wantCursorX: 3,
			wantCells: map[int]term.Cell{
				0: {Ch: '/', Width: 1},
				1: {Ch: 'é', Width: 1},
				2: {Ch: 'a', Width: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p searchPrompt
			p.open()
			for _, r := range tt.input {
				assert.Equal(t, searchConsumed, p.handleKey(promptKey(r)))
			}
			assert.Equal(t, tt.wantQuery, p.query())

			pos, _, show := p.cursor(0)
			assert.True(t, show)
			assert.Equal(t, tt.wantCursorX, pos.X,
				"cursor must sit after the last display column")

			w := term.NewStringWriter(10, 1)
			_ = w.Clear(term.Attributes{})
			p.draw(w, 0, 10)
			cells := w.Cells()
			for x, want := range tt.wantCells {
				assert.Equal(t, want.Ch, cells[x].Ch, "cell %d rune", x)
				assert.Equal(t, want.Width, cells[x].Width, "cell %d width", x)
			}
		})
	}
}
