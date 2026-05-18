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
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

// makeStyledRow builds an extractedRow whose every non-space rune carries
// the given attributes (used to simulate reverse-video status rows).
func makeStyledRow(s string, a term.Attributes) extractedRow {
	runes := []rune(s)
	r := extractedRow{
		runes:      runes,
		attrs:      make([]term.Attributes, len(runes)),
		runeColMap: make([]int, len(runes)),
	}
	for i, ru := range runes {
		r.runeColMap[i] = i
		if ru != ' ' {
			r.attrs[i] = a
		}
	}
	return r
}

func TestDetectChrome(t *testing.T) {
	t.Parallel()

	reverse := term.Attributes(tcell.Style{Attrs: tcell.AttrReverse})

	t.Run("status row at bottom is peeled", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeRow("line 1"),
			makeRow("line 2"),
			makeStyledRow("-- INSERT --", reverse),
		}
		top, bot := detectChrome(rows, nil)
		assert.Equal(t, 0, top)
		assert.Equal(t, 1, bot)
	})

	t.Run("status row at top is peeled", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeStyledRow("file.go +", reverse),
			makeRow("line 1"),
			makeRow("line 2"),
		}
		top, bot := detectChrome(rows, nil)
		assert.Equal(t, 1, top)
		assert.Equal(t, 2, bot)
	})

	t.Run("plain rows: no chrome peeled", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeRow("a"),
			makeRow("b"),
			makeRow("c"),
		}
		top, bot := detectChrome(rows, []string{"a", "b", "c"})
		assert.Equal(t, 0, top)
		assert.Equal(t, 2, bot)
	})

	t.Run("two-row status line at bottom is peeled", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeRow("content"),
			makeStyledRow("file.go [+]", reverse),
			makeStyledRow(":w", reverse),
		}
		top, bot := detectChrome(rows, nil)
		assert.Equal(t, 0, top)
		assert.Equal(t, 0, bot)
	})
}
