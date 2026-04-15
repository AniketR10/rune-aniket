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

package vi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/text"
)

func TestKeyBindingsSpecialMarks(t *testing.T) {
	bindings := KeyBindings()

	tests := []struct {
		name    string
		first   rune
		last    rune
		wantCmd [][]string
	}{
		{
			name:    "backtick-dot jumps to last change position",
			first:   '`',
			last:    '.',
			wantCmd: [][]string{{text.CommandLocationJump, "next", lastChangeLocationListID}},
		},
		{
			name:    "apostrophe-dot jumps to last change line",
			first:   '\'',
			last:    '.',
			wantCmd: [][]string{{text.CommandLocationJumpLine, "next", lastChangeLocationListID}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			seq := handler.Sequence{
				First: term.KeyComb{Ch: tc.first},
				Last:  term.KeyComb{Ch: tc.last},
			}
			cmds, ok := bindings[seq]
			require.True(t, ok, "binding for %c%c should exist", tc.first, tc.last)
			assert.Equal(t, tc.wantCmd, cmds)
		})
	}
}

func TestKeyBindingsLetterMarks(t *testing.T) {
	bindings := KeyBindings()

	// Verify all a-z mark bindings exist.
	for ch := 'a'; ch <= 'z'; ch++ {
		// m{ch} — set mark
		seq := handler.Sequence{
			First: term.KeyComb{Ch: 'm'},
			Last:  term.KeyComb{Ch: ch},
		}
		cmds, ok := bindings[seq]
		require.True(t, ok, "m%c should exist", ch)
		assert.Equal(t, [][]string{
			{text.CommandDeleteAllLocations, string(ch)},
			{text.CommandCreateLocation, string(ch)},
		}, cmds)

		// `{ch} — exact position jump
		seq = handler.Sequence{
			First: term.KeyComb{Ch: '`'},
			Last:  term.KeyComb{Ch: ch},
		}
		cmds, ok = bindings[seq]
		require.True(t, ok, "`%c should exist", ch)
		assert.Equal(t, [][]string{
			{text.CommandLocationJump, "next", string(ch)},
		}, cmds)

		// '{ch} — linewise jump
		seq = handler.Sequence{
			First: term.KeyComb{Ch: '\''},
			Last:  term.KeyComb{Ch: ch},
		}
		cmds, ok = bindings[seq]
		require.True(t, ok, "'%c should exist", ch)
		assert.Equal(t, [][]string{
			{text.CommandLocationJumpLine, "next", string(ch)},
		}, cmds)
	}
}
