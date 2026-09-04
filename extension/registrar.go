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

package extension

import (
	"io"
	"maps"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"unstable.build/rune/rpc"
)

// ResourceRegistrar wraps the basic Serve method, to serve resources over a mux broker.
type ResourceRegistrar interface {
	Register(registar rpc.ServiceRegistrar, locker sync.Locker) (
		io.Closer, error,
	)
}

// MergeResourceMap merges m1 with mn.
// If permissions are overlapping, the last of passed prevails.
func MergeResourceMap(
	m1 map[extensionapi.Permission]ResourceRegistrar,
	mn ...map[extensionapi.Permission]ResourceRegistrar,
) map[extensionapi.Permission]ResourceRegistrar {
	ret := make(map[extensionapi.Permission]ResourceRegistrar)
	maps.Copy(ret, m1)
	for _, m := range mn {
		maps.Copy(ret, m)
	}
	return ret
}
