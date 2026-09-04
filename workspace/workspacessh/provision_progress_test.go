// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
		Phase:   ProvisionPhaseDownloading,
		Done:    128,
		Of:      512,
		Units:   "KiB",
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

// TestParseProvisionProgressOldShapeStillParses asserts that a line emitted by
// a remote built before the byte sub-progress and finalizing/downloading
// phases existed (no d/o/u fields) still parses cleanly, so the local notifier
// stays backward-compatible with older remotes.
func TestParseProvisionProgressOldShapeStillParses(t *testing.T) {
	const oldLine = `{"rune":"provision","i":1,"n":2,"pkg":"rune-go","ver":"1.2.3","phase":"installing"}`
	got, ok := ParseProvisionProgressLine([]byte(oldLine))
	require.True(t, ok, "old-shape line must parse")
	assert.Equal(t, ProvisionPhaseInstalling, got.Phase)
	assert.Equal(t, 0, got.Done)
	assert.Equal(t, 0, got.Of)
	assert.Empty(t, got.Units)
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

func TestServerReadyRoundTrip(t *testing.T) {
	line, err := encodeServerReady()
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(line, "\n"), "encoded line must end with newline")
	assert.True(t, parseServerReadyLine([]byte(strings.TrimRight(line, "\n"))),
		"sentinel-tagged ready line must parse")
}

func TestParseServerReadyLineRejectsNonReady(t *testing.T) {
	cases := []struct {
		desc string
		line string
	}{
		{"plain text", "ready to serve"},
		{"unrelated json", `{"level":"info","msg":"hello"}`},
		{"wrong sentinel", `{"rune":"other"}`},
		{"provision sentinel is not ready", `{"rune":"provision","phase":"done"}`},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.False(t, parseServerReadyLine([]byte(tc.line)))
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
			desc:    "downloading with total",
			p:       ProvisionProgress{Index: 1, Total: 2, Package: "rune-go", Version: "1.2.3", Phase: ProvisionPhaseDownloading, Done: 128, Of: 512, Units: "KiB"},
			wantMsg: "Downloading rune-go@1.2.3 (128/512 KiB)",
			wantLvl: NotificationInfo,
		},
		{
			desc:    "downloading without total",
			p:       ProvisionProgress{Index: 1, Total: 2, Package: "rune-go", Version: "1.2.3", Phase: ProvisionPhaseDownloading, Done: 128, Units: "KiB"},
			wantMsg: "Downloading rune-go@1.2.3 (128 KiB)",
			wantLvl: NotificationInfo,
		},
		{
			desc:    "activating",
			p:       ProvisionProgress{Package: "rune-go", Version: "1.2.3", Phase: ProvisionPhaseActivating},
			wantMsg: "Activated rune-go@1.2.3",
			wantLvl: NotificationInfo,
		},
		{
			desc:    "finalizing",
			p:       ProvisionProgress{Index: 2, Total: 2, Phase: ProvisionPhaseFinalizing},
			wantMsg: "Finalizing workspace…",
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
