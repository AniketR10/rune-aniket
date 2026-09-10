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

//go:build unix && !darwin

package font

import (
	"os"
	"path/filepath"
)

func emojiFontPaths() []string {
	var paths []string
	if dataPath := os.Getenv("XDG_DATA_HOME"); dataPath != "" {
		paths = append(paths, filepath.Join(expandUser(dataPath), "fonts", "NotoColorEmoji.ttf"))
	}
	return append(paths,
		expandUser("~/.local/share/fonts/NotoColorEmoji.ttf"),
		expandUser("~/.fonts/NotoColorEmoji.ttf"),
		"/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/google-noto-emoji/NotoColorEmoji.ttf",
		"/usr/local/share/fonts/NotoColorEmoji.ttf",
	)
}
