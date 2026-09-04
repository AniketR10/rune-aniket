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

package registerset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
)

func TestRegisterSet(t *testing.T) {
	root := clipboard.NewInMemory()
	registers := New(root)

	require.NoError(t, registers.Copy("a", clipboard.Data{Text: "named"}))
	require.NoError(t, registers.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "unnamed"}))
	require.NoError(t, registers.Copy(ClipboardRegisterID, clipboard.Data{Text: "system"}))
	require.NoError(t, registers.Copy(BlackHoleRegisterID, clipboard.Data{Text: "ignored"}))

	data, err := registers.Paste("a")
	require.NoError(t, err)
	assert.Equal(t, "named", data.Text)

	data, err = registers.Paste("A")
	require.NoError(t, err)
	assert.Equal(t, "named", data.Text)

	data, err = registers.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "system", data.Text)

	data, err = registers.Paste(UnnamedRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "system", data.Text)

	data, err = root.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "system", data.Text)

	data, err = registers.Paste(ClipboardRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "system", data.Text)

	data, err = registers.Paste(BlackHoleRegisterID)
	require.NoError(t, err)
	assert.Empty(t, data.Text)
}

func TestRegisterSetDefaultRegisterIsSystemClipboard(t *testing.T) {
	root := clipboard.NewInMemory()
	registers := New(root)

	require.NoError(t, registers.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "from vi"}))

	data, err := root.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "from vi", data.Text)

	require.NoError(t, root.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: "from system"}))

	data, err = registers.Paste(clipboard.DefaultRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "from system", data.Text)

	data, err = registers.Paste(UnnamedRegisterID)
	require.NoError(t, err)
	assert.Equal(t, "from system", data.Text)
}
