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

package vtescanner

type params struct {
	subparams [MaxParams]uint16
	params    [MaxParams]uint16
	// slices backs slice() so a CSI dispatch costs no allocation in
	// steady state. Its contents are only valid until the next dispatch.
	slices           [MaxParams][]uint16
	currentSubparams uint16
	len              uint16
}

func (p *params) isFull() bool {
	return p.len == MaxParams
}

func (p *params) push(item uint16) {
	p.subparams[p.len-p.currentSubparams] = p.currentSubparams + 1
	p.params[p.len] = item
	p.currentSubparams = 0
	p.len += 1
}

func (p *params) extend(item uint16) {
	p.subparams[p.len-p.currentSubparams] = p.currentSubparams + 1
	p.params[p.len] = item
	p.currentSubparams += 1
	p.len += 1
}

// slice groups the parsed parameters by subparameter run. The returned
// slice, and the slices it holds, alias storage the scanner reuses: a
// driver that needs them past the dispatch call must copy.
func (p *params) slice() [][]uint16 {
	n := 0
	index := uint16(0)
	for index < p.len {
		numSubparams := p.subparams[index]
		p.slices[n] = p.params[index : index+numSubparams]
		n++
		index += numSubparams
	}
	return p.slices[:n]
}

func (p *params) reset() {
	p.currentSubparams = 0
	p.len = 0
}
