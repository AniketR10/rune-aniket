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

package debug

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// WriteTagsFile writes the given file to a random file prefixed by DEBUG
// on the default os temp dir. It returns a cleanup function to remove
// the file.
func WriteTagsFile(tags ...string) func() {
	dir := os.TempDir()
	filename := "DEBUG" + strings.Join(append(tags, strconv.Itoa(rand.Int())), "_")
	filename = strings.ReplaceAll(filename, "/", "_")
	tempfile := filepath.Join(dir, filename)
	err := os.WriteFile(tempfile, fmt.Appendf(nil, "%#v", tags), 0777)
	if err != nil {
		panic(err)
	}
	return func() { os.Remove(tempfile) }
}
