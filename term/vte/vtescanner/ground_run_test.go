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

package vtescanner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroundRun(t *testing.T) {
	cases := []struct {
		name  string
		state State
		buf   string
		want  int
	}{
		{"full printable run", Ground, "hello world", 11},
		{"stops at control byte", Ground, "abc\ndef", 3},
		{"stops at escape", Ground, "abc\x1b[31m", 3},
		{"stops at DEL", Ground, "abc\x7fdef", 3},
		{"stops at high byte", Ground, "abc\xc3\xa9", 3},
		{"leading control", Ground, "\rabc", 0},
		{"empty buffer", Ground, "", 0},
		{"non-ground state", CsiParam, "hello", 0},
		{"utf8 state", Utf8, "hello", 0},
		{"space and tilde boundary", Ground, " ~", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := NewScanner(&testDispatcher{})
			s.state = tc.state
			assert.Equal(t, tc.want, s.GroundRun([]byte(tc.buf)))
		})
	}
}
