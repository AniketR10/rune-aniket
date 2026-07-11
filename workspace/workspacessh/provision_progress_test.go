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

package workspacessh

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProvisionProgressRoundTrip(t *testing.T) {
	in := ProvisionProgress{
		Index:   2,
		Total:   5,
		Package: "rune-go",
		Version: "1.2.3",
		Phase:   ProvisionPhaseFailed,
		Detail:  "dial tcp: connection refused",
	}
	line, err := EncodeProvisionProgress(in)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(line, "\n"), "encoded line must end with newline")

	got, ok := ParseProvisionProgressLine([]byte(strings.TrimRight(line, "\n")))
	require.True(t, ok, "sentinel-tagged line must parse")
	assert.Equal(t, provisionSentinel, got.Rune)
	in.Rune = provisionSentinel
	assert.Equal(t, in, got)
}

func TestParseProvisionProgressLineRejectsNonProgress(t *testing.T) {
	cases := []struct {
		desc string
		line string
	}{
		{"plain text", "provision: install rune-go@1.2.3"},
		{"unrelated json", `{"level":"info","msg":"hello"}`},
		{"wrong sentinel", `{"rune":"other","pkg":"rune-go"}`},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			_, ok := ParseProvisionProgressLine([]byte(tc.line))
			assert.False(t, ok)
		})
	}
}

func TestProvisionProgressMessageAndLevel(t *testing.T) {
	cases := []struct {
		desc    string
		p       ProvisionProgress
		wantMsg string
		wantLvl NotificationLevel
	}{
		{
			desc:    "installing",
			p:       ProvisionProgress{Index: 2, Total: 5, Package: "rune-go", Version: "1.2.3", Phase: ProvisionPhaseInstalling},
			wantMsg: "Installing toolchain (2/5): rune-go@1.2.3",
			wantLvl: NotificationInfo,
		},
		{
			desc:    "activating",
			p:       ProvisionProgress{Package: "rune-go", Version: "1.2.3", Phase: ProvisionPhaseActivating},
			wantMsg: "Activated rune-go@1.2.3",
			wantLvl: NotificationInfo,
		},
		{
			desc:    "done",
			p:       ProvisionProgress{Index: 5, Total: 5, Phase: ProvisionPhaseDone},
			wantMsg: "Installed 5/5 toolchain packages",
			wantLvl: NotificationInfo,
		},
		{
			desc:    "failed",
			p:       ProvisionProgress{Package: "rune-go", Version: "1.2.3", Phase: ProvisionPhaseFailed},
			wantMsg: "Failed to install rune-go@1.2.3",
			wantLvl: NotificationWarning,
		},
		{
			desc:    "failed with detail",
			p:       ProvisionProgress{Package: "rune-go", Version: "1.2.3", Phase: ProvisionPhaseFailed, Detail: "dial tcp: connection refused"},
			wantMsg: "Failed to install rune-go@1.2.3: dial tcp: connection refused",
			wantLvl: NotificationWarning,
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.wantMsg, tc.p.Message())
			assert.Equal(t, tc.wantLvl, tc.p.Level())
		})
	}
}
