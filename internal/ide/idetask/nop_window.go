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

package idetask

import (
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

type noopWindow struct{}

func (w noopWindow) Content() (browserapi.Handler, error)     { return nil, nil }
func (w noopWindow) SetContent(h browserapi.Handler) error    { return nil }
func (w noopWindow) Close() error                             { return nil }
func (w noopWindow) WindowID() uint64                         { return 0 }
func (w noopWindow) Focus() (bool, error)                     { return false, nil }
func (w noopWindow) Closed() bool                             { return false }
func (w noopWindow) IsFloating() bool                         { return false }
func (w noopWindow) IsMinimized() (component.Alignment, bool) { return 0, false }
func (w noopWindow) MinimizeUp(padding int) bool              { return false }
func (w noopWindow) MinimizeDown(padding int) bool            { return false }
func (w noopWindow) MinimizeLeft(padding int) bool            { return false }
func (w noopWindow) MinimizeRight(padding int) bool           { return false }
func (w noopWindow) Unminimize() bool                         { return false }
func (w noopWindow) SetFrameAttr(term.Attributes) (term.Attributes, bool) {
	return term.Attributes{}, false
}
func (w noopWindow) Position() term.Coordinates { return term.Coordinates{} }
func (w noopWindow) Width() int                 { return 0 }
func (w noopWindow) Height() int                { return 0 }
