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

package ide

// funcCommandObserver adapts a plain callback to commandObserver for
// subscribers that only care that a command ran, not which one.
type funcCommandObserver func()

func (f funcCommandObserver) observeCommand(_, _ string, _ []string, _ error) {
	f()
}

type commandObserverRegistry struct {
	subscribers []commandObserver
}

func newCommandObserverRegistry() *commandObserverRegistry {
	return &commandObserverRegistry{}
}

func (r *commandObserverRegistry) subscribe(obs commandObserver) {
	if obs == nil {
		return
	}
	r.subscribers = append(r.subscribers, obs)
}

func (r *commandObserverRegistry) observeCommand(
	typed, resolved string, args []string, err error,
) {
	for _, s := range r.subscribers {
		s.observeCommand(typed, resolved, args, err)
	}
}
