// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
	storagePartition  = "idecursor"
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
	WorkspaceURI string
	Entries      []location
	Index        int
	Version      int64
}

func newHistoryDocument(workspaceURI workspaceapi.URI) historyDocument {
	return historyDocument{
		WorkspaceURI: workspaceURI.String(),
		Index:        -1,
		Version:      1,
	}
}

func documentID(workspaceURI workspaceapi.URI) string {
	return base64.RawURLEncoding.EncodeToString([]byte(workspaceURI.String()))
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
