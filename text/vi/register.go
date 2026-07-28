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
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/go-tui/text/registerset"
)

const (
	unnamedRegister   = '"'
	lastYankRegister  = '0'
	clipboardRegister = '+'
	blackHoleRegister = '_'
)

func validRegisterName(name rune) bool {
	return name == unnamedRegister || name == lastYankRegister ||
		name == clipboardRegister || name == blackHoleRegister ||
		name == '/' || name == '.' || name == '-' ||
		('a' <= name && name <= 'z') ||
		('A' <= name && name <= 'Z')
}

// validMarkName reports whether name is accepted as a mark identifier
// for `'{mark}` / “ `{mark} “ motions. Vim recognizes alphabetic
// marks (a-z, A-Z) plus the special marks `.` (last change), `<` and
// `>` (visual selection start/end). Numeric or other special marks
// (`'`, `^`, `[`, `]`, etc.) are not recognized here yet.
func validMarkName(name rune) bool {
	if name == '.' || name == '<' || name == '>' {
		return true
	}
	return ('a' <= name && name <= 'z') ||
		('A' <= name && name <= 'Z')
}

func registerNameToID(name rune) string {
	if name == 0 || name == unnamedRegister {
		return clipboard.DefaultRegisterID
	}
	return registerset.Normalize(string(name))
}

func normalizedRegisterName(name rune) rune {
	registerID := registerNameToID(name)
	if registerID == clipboard.DefaultRegisterID {
		return unnamedRegister
	}
	names := []rune(registerID)
	if len(names) > 0 {
		return names[0]
	}
	return unnamedRegister
}
