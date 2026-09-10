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

// Package idetutorial implements the interactive tutorial overlay used
// by the IDE.
package idetutorial

import (
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

// Tutorial is the in-process state machine driving a single tutorial run.
// Implementations draw an overlay on top of the IDE root via direct
// [term.Writer] writes and return exit=true from Handle when the tutorial
// finishes or is dismissed. Reset zeroes runtime progress so the same
// instance can be dispatched again; host services and parsed content are
// retained.
type Tutorial interface {
	tui.Handler
	Reset()

	// Stop tears down any background work the tutorial owns (the
	// Starlark goroutine, an installed Prompt overlay, etc.) without
	// implying that the tutorial will be re-run. Stop must be safe
	// to call multiple times and on a tutorial that never ran.
	Stop()

	// Completed reports whether the most recent run reached a normal return.
	Completed() bool

	// ObserveCommand reports a dispatched IDE command to the tutorial.
	// typed is the user-typed name (possibly an alias), resolved is the
	// alias-expanded target, args are positional arguments, and err is
	// the dispatch result. When err is non-nil the tutorial state must
	// not advance. Returns exit=true when the tutorial finishes as a
	// result of this observation.
	ObserveCommand(typed, resolved string, args []string, err error) (exit bool)

	// ObserveEvent reports an observed editor event to the tutorial.
	// eventType is the lowercase event-type name (e.g. "open") and uri
	// is the affected document URI. Returns exit=true when the tutorial
	// finishes as a result of this observation.
	ObserveEvent(eventType, uri string) (exit bool)

	// Shader returns the background shader the tutorial wants installed
	// over the IDE root, and ok=true when one is desired. ok=false means
	// no shader should be installed. Returning a different Shader value
	// triggers a tear-down and rebuild of the underlying shader
	// component by value equality.
	Shader() (Shader, bool)

	// SetDefaultAttributes updates the tutorial's view of the current
	// default terminal attributes so per-step shaders read live theme
	// colors. Implementations must restage any active step's shader spec
	// so the next Draw rebuilds it with the new attributes.
	SetDefaultAttributes(defAttr term.Attributes)

	// ComponentAt returns the tutorial handler drawn at pos, ok=false
	// when the tutorial's overlay does not cover pos. The composing
	// [Handler] hides the root's terminal cursor when the overlay
	// covers it so the cursor does not bleed through; a cursor outside
	// the overlay stays visible.
	ComponentAt(pos term.Coordinates) (tui.Handler, bool)
}
