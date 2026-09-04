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

package command

import "github.com/unstablebuild/rune-go-sdk/component"

// Dispatcher abstracts the ability to dispatch commands.
type Dispatcher interface {
	Dispatch(cmd string, args ...string) bool
	// Preview allows implementations to provide a dynamic substitute
	// for the command manual via the returned tui.Component, and
	// if there are mutable effects, these can be reversed via the returned
	// cancel function. The final boolean is to indicate that either
	// tui.Component or the returned cancel function are non nil.
	Preview(cmd string, args ...string) (component.Responsive, func(), bool)
}

// FuncDispatcher returns a Dispatcher that calls fn every time Dispatch is called.
// It ignores calls to Preview.
func FuncDispatcher(fn func(string, ...string) bool) Dispatcher {
	return fnDispatcher{fn: fn}
}

// FuncDispatcherWithPreview returns a Dispatcher that calls fn every time Dispatch is called.
// It ignores calls to Preview.
func FuncDispatcherWithPreview(
	fn func(string, ...string) bool,
	preview func(string, ...string) (component.Responsive, func(), bool),
) Dispatcher {
	return fnDispatcher{fn: fn, preview: preview}
}

type fnDispatcher struct {
	fn      func(string, ...string) bool
	preview func(string, ...string) (component.Responsive, func(), bool)
}

func (d fnDispatcher) Dispatch(cmd string, args ...string) bool {
	return d.fn(cmd, args...)
}

func (d fnDispatcher) Preview(cmd string, args ...string) (component.Responsive, func(), bool) {
	if d.preview == nil {
		return nil, nil, false
	}
	return d.preview(cmd, args...)
}
