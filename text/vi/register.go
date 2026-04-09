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

package vi

import (
	"sync"
	"unicode"

	"github.com/unstablebuild/rune-go-sdk/clipboard"
)

var _ clipboard.Register = (*registerSet)(nil)

const (
	unnamedRegister   = '"'
	lastYankRegister  = '0'
	clipboardRegister = '+'
	blackHoleRegister = '_'
)

type registerSet struct {
	mu        sync.RWMutex
	clipboard clipboard.Register
	data      map[rune]clipboard.Data
}

func newRegisterSet(clip clipboard.Register) *registerSet {
	if registers, ok := clip.(*registerSet); ok {
		return registers
	}
	return &registerSet{
		clipboard: clip,
		data:      make(map[rune]clipboard.Data),
	}
}

func (r *registerSet) Copy(registerID string, data clipboard.Data) error {
	return r.copy(registerIDToName(registerID), data)
}

func (r *registerSet) Paste(registerID string) (clipboard.Data, error) {
	return r.paste(registerIDToName(registerID))
}

func registerIDToName(registerID string) rune {
	if registerID == clipboard.DefaultRegisterID {
		return unnamedRegister
	}
	for _, name := range registerID {
		return name
	}
	return unnamedRegister
}

func registerNameToID(name rune) string {
	name = normalRegisterName(name)
	if name == unnamedRegister {
		return clipboard.DefaultRegisterID
	}
	return string(name)
}

func validRegisterName(name rune) bool {
	return name == unnamedRegister || name == lastYankRegister ||
		name == clipboardRegister || name == blackHoleRegister ||
		name == '/' || name == '.' || name == '-' ||
		('a' <= name && name <= 'z') ||
		('A' <= name && name <= 'Z')
}

func normalRegisterName(name rune) rune {
	if name == 0 {
		return unnamedRegister
	}
	return unicode.ToLower(name)
}

func (r *registerSet) copy(name rune, data clipboard.Data) error {
	name = normalRegisterName(name)
	switch name {
	case blackHoleRegister:
		return nil
	case clipboardRegister:
		return r.clipboard.Copy(clipboard.DefaultRegisterID, data)
	}

	r.mu.Lock()
	r.data[name] = data
	r.mu.Unlock()
	if name == unnamedRegister {
		return r.clipboard.Copy(clipboard.DefaultRegisterID, data)
	}
	return nil
}

func (r *registerSet) paste(name rune) (clipboard.Data, error) {
	name = normalRegisterName(name)
	if name == clipboardRegister {
		return r.clipboard.Paste(clipboard.DefaultRegisterID)
	}

	r.mu.RLock()
	data := r.data[name]
	r.mu.RUnlock()
	return data, nil
}
