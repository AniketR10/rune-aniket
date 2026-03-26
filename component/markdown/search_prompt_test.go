// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

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
