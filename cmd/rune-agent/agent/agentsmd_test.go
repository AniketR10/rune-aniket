// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverAgentsFiles(t *testing.T) {
	t.Run("finds file in cwd", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "AGENTS.md"), "instructions here")

		paths := DiscoverAgentsFiles(osFileSystem{}, dir, "AGENTS.md")

		assert.Equal(t, []string{filepath.Join(dir, "AGENTS.md")}, paths)
	})

	t.Run("walks up to find files in ancestors", func(t *testing.T) {
		root := t.TempDir()
		child := filepath.Join(root, "a", "b")
		require.NoError(t, os.MkdirAll(child, 0o755))
		writeFile(t, filepath.Join(root, "AGENTS.md"), "root instructions")
		writeFile(t, filepath.Join(root, "a", "AGENTS.md"), "mid instructions")

		paths := DiscoverAgentsFiles(osFileSystem{}, child, "AGENTS.md")

		// Closest first, then ancestors.
		assert.Equal(t, []string{
			filepath.Join(root, "a", "AGENTS.md"),
			filepath.Join(root, "AGENTS.md"),
		}, paths)
	})

	t.Run("returns nil when no files found", func(t *testing.T) {
		dir := t.TempDir()

		paths := DiscoverAgentsFiles(osFileSystem{}, dir, "AGENTS.md")

		assert.Nil(t, paths)
	})

	t.Run("empty filename returns nil", func(t *testing.T) {
		paths := DiscoverAgentsFiles(osFileSystem{}, "/tmp", "")

		assert.Nil(t, paths)
	})

	t.Run("custom filename", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "CLAUDE.md"), "claude instructions")

		paths := DiscoverAgentsFiles(osFileSystem{}, dir, "CLAUDE.md")

		assert.Equal(t, []string{filepath.Join(dir, "CLAUDE.md")}, paths)
	})
}

func TestLoadAgentsFiles(t *testing.T) {
	t.Run("returns empty for nil paths", func(t *testing.T) {
		result := LoadAgentsFiles(osFileSystem{}, nil)

		assert.Equal(t, "", result)
	})

	t.Run("loads single file", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "AGENTS.md")
		writeFile(t, p, "# My Instructions\nDo stuff.")

		result := LoadAgentsFiles(osFileSystem{}, []string{p})

		expected := "Contents of " + p + " (project instructions):\n\n# My Instructions\nDo stuff."
		assert.Equal(t, expected, result)
	})

	t.Run("loads multiple files separated by blank lines", func(t *testing.T) {
		dir := t.TempDir()
		p1 := filepath.Join(dir, "child", "AGENTS.md")
		p2 := filepath.Join(dir, "AGENTS.md")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "child"), 0o755))
		writeFile(t, p1, "child instructions")
		writeFile(t, p2, "root instructions")

		result := LoadAgentsFiles(osFileSystem{}, []string{p1, p2})

		expected := "Contents of " + p1 + " (project instructions):\n\nchild instructions" +
			"\n\nContents of " + p2 + " (project instructions):\n\nroot instructions"
		assert.Equal(t, expected, result)
	})

	t.Run("skips empty files", func(t *testing.T) {
		dir := t.TempDir()
		p1 := filepath.Join(dir, "empty.md")
		p2 := filepath.Join(dir, "real.md")
		writeFile(t, p1, "   \n  \n  ")
		writeFile(t, p2, "real content")

		result := LoadAgentsFiles(osFileSystem{}, []string{p1, p2})

		expected := "Contents of " + p2 + " (project instructions):\n\nreal content"
		assert.Equal(t, expected, result)
	})

	t.Run("skips unreadable files", func(t *testing.T) {
		dir := t.TempDir()
		missing := filepath.Join(dir, "missing.md")
		p := filepath.Join(dir, "real.md")
		writeFile(t, p, "content")

		result := LoadAgentsFiles(osFileSystem{}, []string{missing, p})

		expected := "Contents of " + p + " (project instructions):\n\ncontent"
		assert.Equal(t, expected, result)
	})

	t.Run("returns empty when all files fail", func(t *testing.T) {
		result := LoadAgentsFiles(osFileSystem{}, []string{"/nonexistent/AGENTS.md"})

		assert.Equal(t, "", result)
	})
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}
