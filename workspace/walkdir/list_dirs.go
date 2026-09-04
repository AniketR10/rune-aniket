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

package walkdir

import (
	"context"

	"github.com/unstablebuild/blue/iterator"
)

// ListDirs traverses the workspace directory and returns
// an iterator that returns all directories under root. If root is
// a partial or full file name, it will be ignored and its base
// directory, will be used. If errors are encountered while reading
// the contents of directories those errors will be aggregated and
// reported by the iterator's Err method.
func ListDirs(
	ctx context.Context, w Reader, root string,
) (iterator.Iterator[string], error) {
	return listPaths(ctx, w, root, true)
}
