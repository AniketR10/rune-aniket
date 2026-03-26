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

package idedebug

import "github.com/unstablebuild/rune-go-sdk/api/textapi"

// EditorEvents returns the events that Manager is
// interested in subscribing to.
func EditorEvents() []textapi.EventType {
	return []textapi.EventType{
		textapi.EventTypeOpen,
	}
}
