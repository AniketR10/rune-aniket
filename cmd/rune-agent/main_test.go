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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/headless"
)

func TestParseHeadlessArgs(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		want    headless.Options
		wantErr string
	}{
		{
			name: "instructions only",
			args: []string{"--headless", "task.md"},
			want: headless.Options{InstructionsFile: "task.md"},
		},
		{
			name: "model and effort",
			args: []string{"--headless", "task.md",
				"--model", "anthropic/claude-opus-4-6", "--effort", "high"},
			want: headless.Options{
				InstructionsFile: "task.md",
				Model:            "anthropic/claude-opus-4-6",
				Effort:           "high",
			},
		},
		{
			name:    "missing instructions",
			args:    []string{"--headless"},
			wantErr: "usage: rune-agent --headless",
		},
		{
			name:    "flag in place of instructions",
			args:    []string{"--headless", "--model", "openai/gpt-5"},
			wantErr: "usage: rune-agent --headless",
		},
		{
			name:    "headless not first",
			args:    []string{"--model", "openai/gpt-5", "--headless", "task.md"},
			wantErr: "usage: rune-agent --headless",
		},
		{
			name:    "value-less flag",
			args:    []string{"--headless", "task.md", "--model"},
			wantErr: `--model requires a value`,
		},
		{
			name:    "unknown flag",
			args:    []string{"--headless", "task.md", "--memory"},
			wantErr: `unknown argument "--memory"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseHeadlessArgs(tc.args)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRunHeadlessExitCodes(t *testing.T) {
	t.Run("bad usage exits 1", func(t *testing.T) {
		assert.Equal(t, 1, runHeadless([]string{"--headless"}))
	})

	t.Run("no plugin environment exits 1", func(t *testing.T) {
		t.Setenv("RUNE_SOCKET", "")
		t.Setenv("RUNE_DATADIR", "")
		path := filepath.Join(t.TempDir(), "task.md")
		require.NoError(t, os.WriteFile(path, []byte("do it"), 0o600))

		assert.Equal(t, 1, runHeadless([]string{"--headless", path}))
	})
}
