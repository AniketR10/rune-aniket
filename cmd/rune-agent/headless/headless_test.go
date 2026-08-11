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

package headless

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunRequiresInstructionsFile(t *testing.T) {
	_, err := Run(context.Background(), Options{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "instructions file is required")
}

func TestRunRejectsUnreadableInstructions(t *testing.T) {
	_, err := Run(context.Background(), Options{
		InstructionsFile: filepath.Join(t.TempDir(), "missing.md"),
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "read instructions")
}

func TestRunRequiresPluginEnvironment(t *testing.T) {
	t.Setenv("RUNE_SOCKET", "")
	t.Setenv("RUNE_DATADIR", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "instructions.md")
	require.NoError(t, os.WriteFile(path, []byte("do it"), 0o600))

	_, err := Run(context.Background(), Options{InstructionsFile: path})

	assert.ErrorIs(t, err, ErrNotInRune)
}

// The agentbench runner evaluates `git diff`, so a headless run must
// never write into the workspace it is pointed at.
func TestRunLeavesWorkspaceUnmodified(t *testing.T) {
	t.Setenv("RUNE_SOCKET", "")
	t.Setenv("RUNE_DATADIR", "")
	workspace := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(workspace, "main.go"), []byte("package main\n"), 0o600))
	instructions := filepath.Join(t.TempDir(), "instructions.md")
	require.NoError(t, os.WriteFile(instructions, []byte("do it"), 0o600))

	before := treeDigest(t, workspace)
	chdir(t, workspace)

	var out strings.Builder
	_, err := Run(context.Background(), Options{
		InstructionsFile: instructions, Stdout: &out,
	})

	require.Error(t, err)
	assert.Equal(t, before, treeDigest(t, workspace))
	assert.Empty(t, out.String())
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { require.NoError(t, os.Chdir(prev)) })
}

// treeDigest fingerprints every path and file body under root.
func treeDigest(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			entries = append(entries, "d "+rel)
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		entries = append(entries,
			fmt.Sprintf("f %s %x", rel, sha256.Sum256(body)))
		return nil
	})
	require.NoError(t, err)
	sort.Strings(entries)
	sum := sha256.Sum256([]byte(strings.Join(entries, "\n")))
	return hex.EncodeToString(sum[:])
}
