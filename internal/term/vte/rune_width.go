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

package vte

import (
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
)

//go:generate go run ./runewidthgen

const (
	// widthTableLimit covers the Basic Multilingual Plane plus the
	// Supplementary Multilingual Plane, which together hold every script
	// and every emoji a terminal realistically renders.
	widthTableLimit = 0x20000
	widthBlockShift = 6
	widthBlockSize  = 1 << widthBlockShift
)

// runeWidth returns the number of cells c occupies. The generated table
// shortcuts graphemecluster.StringWidth, which costs two Unicode table
// binary searches on every call.
func runeWidth(c rune) int {
	if c < 0x7F {
		return 1
	}
	if c < widthTableLimit {
		block := int(runeWidthBlockIndex[c>>widthBlockShift])
		return int(runeWidthBlocks[block<<widthBlockShift|int(c)&(widthBlockSize-1)] - '0')
	}
	return graphemecluster.StringWidth(string(c))
}
