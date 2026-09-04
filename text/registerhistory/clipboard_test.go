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

package registerhistory

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/rune/text/registerset"
)

type errorClipboard struct{}

func (errorClipboard) Paste(string) (clipboard.Data, error) {
	return clipboard.Data{}, errors.New("clipboard error")
}

func (errorClipboard) Copy(string, clipboard.Data) error {
	return errors.New("clipboard error")
}

func TestClipboardHistory(t *testing.T) {
	root := clipboard.NewInMemory()
	clip := NewClipboard(root)

	history, ok := AsHistory(clip)
	require.True(t, ok, "history clipboard must implement History")
	assert.Equal(t, 0, history.HistoryLen())

	require.NoError(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "first"}))
	require.NoError(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "second"}))
	require.NoError(t, clip.Copy(registerset.ClipboardRegisterID, clipboard.Data{Text: "third"}))

	assert.Equal(t, 3, history.HistoryLen())

	data, ok := history.HistoryAt(0)
	require.True(t, ok)
	assert.Equal(t, "third", data.Text)

	data, ok = history.HistoryAt(1)
	require.True(t, ok)
	assert.Equal(t, "second", data.Text)

	data, ok = history.HistoryAt(2)
	require.True(t, ok)
	assert.Equal(t, "first", data.Text)

	_, ok = history.HistoryAt(3)
	assert.False(t, ok)

	_, ok = history.HistoryAt(-1)
	assert.False(t, ok)
}

func TestNamedRegistersAreNotTracked(t *testing.T) {
	root := clipboard.NewInMemory()
	clip := NewClipboard(root)

	history, ok := AsHistory(clip)
	require.True(t, ok)

	require.NoError(t, clip.Copy("a", clipboard.Data{Text: "named"}))
	require.NoError(t, clip.Copy(registerset.BlackHoleRegisterID, clipboard.Data{Text: "blackhole"}))
	assert.Equal(t, 0, history.HistoryLen())
}

func TestClipboardForwardsCopyAndPaste(t *testing.T) {
	root := clipboard.NewInMemory()
	clip := NewClipboard(root)

	require.NoError(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "copied"}))

	data, err := root.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "copied", data.Text)

	data, err = clip.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "copied", data.Text)
}

func TestFailedCopiesAreNotTracked(t *testing.T) {
	clip := NewClipboard(errorClipboard{})

	history, ok := AsHistory(clip)
	require.True(t, ok)

	require.Error(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "failed"}))
	assert.Equal(t, 0, history.HistoryLen())
}

func TestNewClipboardIsIdempotent(t *testing.T) {
	root := clipboard.NewInMemory()
	clip := NewClipboard(root)
	assert.Same(t, clip, NewClipboard(clip))
}

func TestAsHistoryPlainClipboard(t *testing.T) {
	root := clipboard.NewInMemory()
	_, ok := AsHistory(root)
	assert.False(t, ok, "plain clipboard should not implement History")
}
