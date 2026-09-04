// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package font

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"unstable.build/rune/term/gui/font/builtinfont"
)

// stubEmojiPaths pins the candidate list so the emoji resolver runs
// deterministically regardless of the host's installed fonts.
func stubEmojiPaths(paths ...string) func() []string {
	return func() []string { return paths }
}

// TestEmojiFacePrefersSystemFont asserts a usable system font is
// preferred over the bundled fallback and labeled as such.
func TestEmojiFacePrefersSystemFont(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SystemColorEmoji.ttf")
	require.NoError(t, os.WriteFile(path, builtinfont.EmojiTTF, 0o600))

	m := &Manager{emojiPaths: stubEmojiPaths(path)}
	face, source := m.EmojiFace()

	require.NotNil(t, face)
	assert.Equal(t, "system: "+path, source)
	assert.True(t, face.Has([]rune{'😀'}))
}

// TestEmojiFaceFallsBackToBundled asserts that with no usable system
// font the resolver falls back to the bundled Noto, holding even on CI
// hosts with no color-emoji fonts.
func TestEmojiFaceFallsBackToBundled(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "NotThere.ttf")
	m := &Manager{emojiPaths: stubEmojiPaths(missing)}
	face, source := m.EmojiFace()

	require.NotNil(t, face)
	assert.Equal(t, "bundled: Noto Color Emoji", source)
	assert.True(t, face.Has([]rune{'😀'}))
}

// TestEmojiFaceSkipsUnusableSystemFont asserts a candidate path holding
// a non-color font is skipped in favor of the bundled fallback.
func TestEmojiFaceSkipsUnusableSystemFont(t *testing.T) {
	// A non-font file makes NewFaceFromFile reject the candidate.
	bad := filepath.Join(t.TempDir(), "notafont.ttf")
	require.NoError(t, os.WriteFile(bad, []byte("not a font"), 0o600))

	m := &Manager{emojiPaths: stubEmojiPaths(bad)}
	face, source := m.EmojiFace()

	require.NotNil(t, face)
	assert.Equal(t, "bundled: Noto Color Emoji", source)
}

// TestEmojiFaceResolvesOnce asserts resolution happens once and is
// cached, so repeated calls do not re-parse a system font collection.
func TestEmojiFaceResolvesOnce(t *testing.T) {
	m := &Manager{emojiPaths: stubEmojiPaths()}
	face1, source1 := m.EmojiFace()
	face2, source2 := m.EmojiFace()

	assert.Same(t, face1, face2, "face is cached across calls")
	assert.Equal(t, source1, source2)
}
