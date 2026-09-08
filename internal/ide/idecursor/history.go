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

package idecursor

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const (
	commandName       = "cursorhistory"
	documentIDPrefix  = "cursor-history:"
	documentKind      = "cursor-history"
	defaultMaxEntries = 100
	minJumpLines      = 10
)

type location struct {
	URI          string
	Cursor       term.Coordinates
	WindowCursor term.Coordinates
	Timestamp    time.Time
}

type historyDocument struct {
	Kind         string
	WorkspaceURI string
	Entries      []location
	Index        int
	Version      int64
}

func newHistoryDocument(workspaceURI workspaceapi.URI) historyDocument {
	return historyDocument{
		Kind:         documentKind,
		WorkspaceURI: workspaceURI.String(),
		Index:        -1,
		Version:      1,
	}
}

func documentID(workspaceURI workspaceapi.URI) string {
	return documentIDPrefix + base64.RawURLEncoding.EncodeToString([]byte(workspaceURI.String()))
}

func sameLocation(a, b location) bool {
	return a.URI == b.URI && a.Cursor == b.Cursor
}

func shouldRecord(current, next location) bool {
	if sameLocation(current, next) {
		return false
	}
	if current.URI != next.URI {
		return true
	}
	diff := current.Cursor.Y - next.Cursor.Y
	if diff < 0 {
		diff = -diff
	}
	return diff >= minJumpLines
}

func (d *historyDocument) current() (location, bool) {
	if d.Index < 0 || d.Index >= len(d.Entries) {
		return location{}, false
	}
	return d.Entries[d.Index], true
}

func (d *historyDocument) record(next location) bool {
	if current, ok := d.current(); ok && !shouldRecord(current, next) {
		return false
	}
	if d.Index >= 0 && d.Index < len(d.Entries)-1 {
		d.Entries = d.Entries[:d.Index+1]
	}
	d.Entries = append(d.Entries, next)
	if len(d.Entries) > defaultMaxEntries {
		overflow := len(d.Entries) - defaultMaxEntries
		d.Entries = d.Entries[overflow:]
	}
	d.Index = len(d.Entries) - 1
	return true
}

func (d *historyDocument) recordJump(from, to location) bool {
	changed := false
	current, ok := d.current()
	if !ok {
		changed = d.record(from) || changed
	} else if !sameLocation(current, from) {
		d.Entries[d.Index] = from
		if d.Index >= 0 && d.Index < len(d.Entries)-1 {
			d.Entries = d.Entries[:d.Index+1]
		}
		changed = true
	}
	return d.record(to) || changed
}

func (d *historyDocument) prev() (location, error) {
	if d.Index <= 0 {
		return location{}, fmt.Errorf("no previous cursor history entry")
	}
	d.Index--
	return d.Entries[d.Index], nil
}

func (d *historyDocument) next() (location, error) {
	if d.Index < 0 || d.Index >= len(d.Entries)-1 {
		return location{}, fmt.Errorf("no next cursor history entry")
	}
	d.Index++
	return d.Entries[d.Index], nil
}

func (d *historyDocument) jump(index int) (location, error) {
	if index < 0 || index >= len(d.Entries) {
		return location{}, fmt.Errorf("cursor history entry out of range")
	}
	d.Index = index
	return d.Entries[index], nil
}
