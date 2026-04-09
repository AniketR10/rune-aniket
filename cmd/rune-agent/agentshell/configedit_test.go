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

package agentshell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func dirURI(dir string) workspaceapi.URI {
	u, _ := workspaceapi.ParseURI("file://" + dir)
	return u
}

// osFS adapts the real filesystem for ConfigFS in tests.
type osFS struct{}

func (osFS) OpenFile(path string, flag int, perm os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(path, flag, perm)
}

func (osFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func writeConfig(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".rune")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644))
}

func readConfig(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".rune", "config.yaml"))
	require.NoError(t, err)
	return string(data)
}

func TestAddSkillDir(t *testing.T) {
	t.Run("no config file creates it", func(t *testing.T) {
		root := t.TempDir()
		err := AddSkillDir(osFS{}, dirURI(root), ".rune/skills")
		require.NoError(t, err)

		content := readConfig(t, root)
		assert.Contains(t, content, ".rune/skills")
		assert.Contains(t, content, "extensions")
	})

	t.Run("no skills key creates it", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, `extensions:
  rune-agent:
    config:
      background_attr:
        bg: default
`)
		err := AddSkillDir(osFS{}, dirURI(root), ".rune/skills")
		require.NoError(t, err)

		content := readConfig(t, root)
		assert.Contains(t, content, ".rune/skills")
	})

	t.Run("existing list appends", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, `extensions:
  rune-agent:
    config:
      skills:
        - .rune/skills
`)
		err := AddSkillDir(osFS{}, dirURI(root), "~/.rune/skills")
		require.NoError(t, err)

		content := readConfig(t, root)
		assert.Contains(t, content, ".rune/skills")
		assert.Contains(t, content, "~/.rune/skills")
	})

	t.Run("duplicate returns error", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, `extensions:
  rune-agent:
    config:
      skills:
        - .rune/skills
`)
		err := AddSkillDir(osFS{}, dirURI(root), ".rune/skills")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already in config")
	})

	t.Run("other keys preserved", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, `extensions:
  rune-agent:
    config:
      background_attr:
        bg: default
      skills:
        - .rune/skills
`)
		err := AddSkillDir(osFS{}, dirURI(root), "custom/skills")
		require.NoError(t, err)

		content := readConfig(t, root)
		assert.Contains(t, content, "background_attr")
		assert.Contains(t, content, "bg: default")
		assert.Contains(t, content, "custom/skills")
	})

	t.Run("empty config creates structure", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "")

		err := AddSkillDir(osFS{}, dirURI(root), ".rune/skills")
		require.NoError(t, err)

		content := readConfig(t, root)
		assert.Contains(t, content, ".rune/skills")
		assert.Contains(t, content, "extensions")
		assert.Contains(t, content, "rune-agent")
	})
}

func TestRemoveSkillDir(t *testing.T) {
	t.Run("no config file returns not found error", func(t *testing.T) {
		root := t.TempDir()
		err := RemoveSkillDir(osFS{}, dirURI(root), ".rune/skills")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in config")
	})

	t.Run("removes target", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, `extensions:
  rune-agent:
    config:
      skills:
        - .rune/skills
        - ~/.rune/skills
`)
		err := RemoveSkillDir(osFS{}, dirURI(root), ".rune/skills")
		require.NoError(t, err)

		content := readConfig(t, root)
		assert.NotContains(t, content, "- .rune/skills")
		assert.Contains(t, content, "~/.rune/skills")
	})

	t.Run("not found returns error", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, `extensions:
  rune-agent:
    config:
      skills:
        - .rune/skills
`)
		err := RemoveSkillDir(osFS{}, dirURI(root), "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not in config")
	})

	t.Run("removing last entry leaves empty sequence", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, `extensions:
  rune-agent:
    config:
      skills:
        - .rune/skills
`)
		err := RemoveSkillDir(osFS{}, dirURI(root), ".rune/skills")
		require.NoError(t, err)

		content := readConfig(t, root)
		assert.Contains(t, content, "skills:")
		// Should not contain any list items
		for _, line := range strings.Split(content, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") && strings.Contains(trimmed, "skills") {
				t.Errorf("expected no skill dirs remaining, found: %s", line)
			}
		}
	})
}

func TestSetMaxTokens(t *testing.T) {
	t.Run("no config file creates it", func(t *testing.T) {
		root := t.TempDir()
		err := SetMaxTokens(osFS{}, dirURI(root), 1234)
		require.NoError(t, err)

		content := readConfig(t, root)
		assert.Contains(t, content, "extensions")
		assert.Contains(t, content, "rune-agent")
		assert.Contains(t, content, "max_tokens: 1234")
	})

	t.Run("existing value is updated", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, `extensions:
  rune-agent:
    config:
      max_tokens: 1000
      skills:
        - .rune/skills
`)
		err := SetMaxTokens(osFS{}, dirURI(root), 2000)
		require.NoError(t, err)

		content := readConfig(t, root)
		assert.NotContains(t, content, "max_tokens: 1000")
		assert.Contains(t, content, "max_tokens: 2000")
		assert.Contains(t, content, ".rune/skills")
	})

	t.Run("empty config creates structure", func(t *testing.T) {
		root := t.TempDir()
		writeConfig(t, root, "")

		err := SetMaxTokens(osFS{}, dirURI(root), 3000)
		require.NoError(t, err)

		content := readConfig(t, root)
		assert.Contains(t, content, "max_tokens: 3000")
	})
}
