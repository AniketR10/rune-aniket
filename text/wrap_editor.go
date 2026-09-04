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

package text

import "github.com/unstablebuild/rune-go-sdk/term"

// wraps Editor returned in calls to Edit
// to auto-delete in calls to Close or Handle(exit=true)
type wrapEditor struct {
	parent *Component
	Handler
}

func (w wrapEditor) Close() error {
	delete(w.parent.editors, w.Resource().String())
	return w.Handler.Close()
}

func (w wrapEditor) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = w.Handler.Handle(ev)
	if exit {
		delete(w.parent.editors, w.Resource().String())
	}
	return
}
