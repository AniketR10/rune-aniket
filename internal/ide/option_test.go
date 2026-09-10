// Copyright (C) 2017-2026 The Rune Authors
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

package ide

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTutorialPlaylistNextItem(t *testing.T) {
	t.Parallel()
	playlist := []TutorialPlaylistItem{
		{Name: "basics", Description: "Learn the basics."},
		{Name: "navigation", Description: "Navigate code."},
	}
	opts := newOptions(WithTutorialPlaylist(playlist...))
	playlist[1].Name = "changed"

	next, ok := opts.nextTutorialPlaylistItem("basics")
	assert.True(t, ok)
	assert.Equal(t, TutorialPlaylistItem{
		Name: "navigation", Description: "Navigate code.",
	}, next)

	_, ok = opts.nextTutorialPlaylistItem("navigation")
	assert.False(t, ok)
	_, ok = opts.nextTutorialPlaylistItem("unknown")
	assert.False(t, ok)
}
