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

// Package registerset provides a register-aware clipboard.Register implementation.
package registerset

import (
	"sync"
	"unicode"

	"github.com/unstablebuild/rune-go-sdk/clipboard"
)

var _ clipboard.Register = (*RegisterSet)(nil)

const (
	// UnnamedRegisterID identifies the Vim-compatible unnamed register.
	UnnamedRegisterID = `"`
	// LastYankRegisterID identifies the Vim-compatible last-yank register.
	LastYankRegisterID = "0"
	// ClipboardRegisterID identifies the system clipboard register.
	ClipboardRegisterID = "+"
	// BlackHoleRegisterID identifies the black-hole register.
	BlackHoleRegisterID = "_"
)

// RegisterSet stores independent clipboard data for each register ID.
type RegisterSet struct {
	mu        sync.RWMutex
	clipboard clipboard.Register
	data      map[string]clipboard.Data
}

// New returns a register-aware clipboard backed by clip.
func New(clip clipboard.Register) clipboard.Register {
	if registers, ok := clip.(*RegisterSet); ok {
		return registers
	}
	return &RegisterSet{
		clipboard: clip,
		data:      make(map[string]clipboard.Data),
	}
}

// Copy satisfies clipboard.Register.
func (r *RegisterSet) Copy(registerID string, data clipboard.Data) error {
	registerID = Normalize(registerID)
	switch registerID {
	case BlackHoleRegisterID:
		return nil
	case ClipboardRegisterID:
		return r.clipboard.Copy(clipboard.DefaultRegisterID, data)
	}

	r.mu.Lock()
	r.data[registerID] = data
	r.mu.Unlock()
	if registerID == clipboard.DefaultRegisterID {
		return r.clipboard.Copy(clipboard.DefaultRegisterID, data)
	}
	return nil
}

// Paste satisfies clipboard.Register.
func (r *RegisterSet) Paste(registerID string) (clipboard.Data, error) {
	registerID = Normalize(registerID)
	if registerID == ClipboardRegisterID {
		return r.clipboard.Paste(clipboard.DefaultRegisterID)
	}

	r.mu.RLock()
	data := r.data[registerID]
	r.mu.RUnlock()
	return data, nil
}

// Normalize converts aliases and letter case to canonical register IDs.
func Normalize(registerID string) string {
	if registerID == "" || registerID == clipboard.DefaultRegisterID || registerID == UnnamedRegisterID {
		return clipboard.DefaultRegisterID
	}
	for _, name := range registerID {
		return string(unicode.ToLower(name))
	}
	return clipboard.DefaultRegisterID
}
