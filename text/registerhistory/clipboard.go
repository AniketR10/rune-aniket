// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

// Package registerhistory provides clipboard history on top of clipboard.Register.
package registerhistory

import (
	"sync"

	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/go-tui/text/registerset"
)

var _ clipboard.Register = (*Clipboard)(nil)
var _ History = (*Clipboard)(nil)

// maxHistory is the maximum number of clipboard entries to retain.
const maxHistory = 100

// History provides read access to the clipboard paste history.
type History interface {
	// HistoryLen returns the number of entries in the clipboard history.
	HistoryLen() int
	// HistoryAt returns the clipboard entry at the given index (0 = most recent).
	HistoryAt(index int) (clipboard.Data, bool)
}

// AsHistory returns a History if the clipboard supports history tracking.
func AsHistory(clip clipboard.Register) (History, bool) {
	h, ok := clip.(History)
	return h, ok
}

// NewClipboard returns a clipboard.Register that tracks paste history.
func NewClipboard(clip clipboard.Register) clipboard.Register {
	if history, ok := clip.(*Clipboard); ok {
		return history
	}
	return &Clipboard{clipboard: clip}
}

// Clipboard tracks history for a wrapped clipboard.Register.
type Clipboard struct {
	mu        sync.RWMutex
	clipboard clipboard.Register
	history   []clipboard.Data
}

// Copy satisfies clipboard.Register.
func (c *Clipboard) Copy(registerID string, data clipboard.Data) error {
	if err := c.clipboard.Copy(registerID, data); err != nil {
		return err
	}
	switch registerset.Normalize(registerID) {
	case clipboard.DefaultRegisterID, registerset.ClipboardRegisterID:
		c.mu.Lock()
		c.appendHistory(data)
		c.mu.Unlock()
	}
	return nil
}

// Paste satisfies clipboard.Register.
func (c *Clipboard) Paste(registerID string) (clipboard.Data, error) {
	return c.clipboard.Paste(registerID)
}

// appendHistory adds a clipboard entry to the history ring.
// Must be called with c.mu held.
func (c *Clipboard) appendHistory(data clipboard.Data) {
	if len(c.history) >= maxHistory {
		copy(c.history, c.history[1:])
		c.history[len(c.history)-1] = data
	} else {
		c.history = append(c.history, data)
	}
}

// HistoryLen returns the number of entries in the clipboard history.
func (c *Clipboard) HistoryLen() int {
	c.mu.RLock()
	n := len(c.history)
	c.mu.RUnlock()
	return n
}

// HistoryAt returns the clipboard entry at the given index (0 = most recent).
func (c *Clipboard) HistoryAt(index int) (clipboard.Data, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if index < 0 || index >= len(c.history) {
		return clipboard.Data{}, false
	}
	// Index 0 is the most recent entry (last in slice).
	return c.history[len(c.history)-1-index], true
}
