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

package text

import (
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
)

// LocationList is the interface that groups Prev and Next
// location methods to fetch the previous and next item respectively.
//
// Both Prev/Next return false if reached either end or beginning of list.
//
// Current returns the current result. If there are no results in the list
// then Current returns false.
type LocationList interface {
	Current() (textapi.Location, bool)
	Prev() (textapi.Location, bool)
	Next() (textapi.Location, bool)
}

// LocationSlice returns a LocationList based on in
func LocationSlice(in []textapi.Location) LocationList {
	return &sliceLocations{in: in}
}

type sliceLocations struct {
	curr int
	in   []textapi.Location
}

func (s *sliceLocations) Current() (textapi.Location, bool) {
	if s.curr >= len(s.in) || s.curr < 0 {
		return textapi.Location{}, false
	}

	return s.in[s.curr], true
}

func (s *sliceLocations) Prev() (textapi.Location, bool) {
	if s.curr-1 < 0 {
		return textapi.Location{}, false
	}
	s.curr--
	return s.in[s.curr], true
}

func (s *sliceLocations) Next() (textapi.Location, bool) {
	if s.curr+1 >= len(s.in) {
		return textapi.Location{}, false
	}
	s.curr++
	return s.in[s.curr], true
}
