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

package schemedoc

import "sync"

// closeGroup is a sync.WaitGroup in which
// wg.Add and wg.Wait can be called concurrently from
// different goroutines.
type closeGroup struct {
	mu         sync.Mutex
	wg         sync.WaitGroup
	waitClosed bool
}

func (wg *closeGroup) AddOne() (func(), bool) {
	wg.mu.Lock()
	if wg.waitClosed {
		wg.mu.Unlock()
		return nil, false
	}
	wg.wg.Add(1)
	wg.mu.Unlock()
	return wg.wg.Done, true
}

func (wg *closeGroup) Close() bool {
	wg.mu.Lock()
	if wg.waitClosed {
		wg.mu.Unlock()
		return false
	}
	wg.waitClosed = true
	wg.mu.Unlock()
	wg.wg.Wait()
	return true
}
