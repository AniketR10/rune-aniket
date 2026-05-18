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

package vteprobe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandTabs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		line    string
		tabstop int
		want    string
	}{
		{"no tabs", "hello world", 4, "hello world"},
		{"single leading tab tab=4", "\tfoo", 4, "    foo"},
		{"two leading tabs tab=4", "\t\tfoo", 4, "        foo"},
		{"mid-line tab tab=4", "ab\tcd", 4, "ab  cd"},
		{"tabstop=2", "\tfoo", 2, "  foo"},
		{"tabstop=8", "\tfoo", 8, "        foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, expandTabs(tt.line, tt.tabstop))
		})
	}
}

func TestVisualToRawCol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		line       string
		runeOffset int
		tabstop    int
		want       int
	}{
		{"no tabs offset 0", "abc", 0, 4, 1},
		{"no tabs offset 1", "abc", 1, 4, 2},
		{"no tabs offset 2", "abc", 2, 4, 3},
		{"single tab offset 4 ts4 -> col 2", "\tabc", 4, 4, 2},
		{"single tab offset 5 ts4 -> col 2 (a)", "\tabc", 4, 4, 2},
		{"tab tab offset 8 ts4 -> col 3 (after both tabs)", "\t\tabc", 8, 4, 3},
		{"past end", "abc", 100, 4, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := visualToRawCol(tt.line, tt.runeOffset, tt.tabstop)
			assert.Equal(t, tt.want, got)
		})
	}
}
