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

package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatSkillContent(t *testing.T) {
	t.Run("basic skill without resources", func(t *testing.T) {
		skill := Skill{
			Name: "commit",
			Body: "Create a git commit.",
			Dir:  "/skills/commit",
		}
		got := FormatSkillContent(skill)
		assert.Contains(t, got, `<skill_content name="commit">`)
		assert.Contains(t, got, "Create a git commit.")
		assert.Contains(t, got, "Skill directory: /skills/commit")
		assert.Contains(t, got, "</skill_content>")
		assert.NotContains(t, got, "<skill_resources>")
	})

	t.Run("skill with resources", func(t *testing.T) {
		dir := t.TempDir()
		scriptsDir := filepath.Join(dir, "scripts")
		require.NoError(t, os.Mkdir(scriptsDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(scriptsDir, "run.sh"), []byte("#!/bin/sh"), 0o644))

		skill := Skill{
			Name: "deploy",
			Body: "Deploy the app.",
			Dir:  dir,
		}
		got := FormatSkillContent(skill)
		assert.Contains(t, got, `<skill_content name="deploy">`)
		assert.Contains(t, got, "<skill_resources>")
		assert.Contains(t, got, "<file>scripts/run.sh</file>")
		assert.Contains(t, got, "</skill_resources>")
	})
}

func TestEnumerateResources(t *testing.T) {
	t.Run("no resource dirs", func(t *testing.T) {
		dir := t.TempDir()
		files := enumerateResources(dir)
		assert.Empty(t, files)
	})

	t.Run("with files in resource dirs", func(t *testing.T) {
		dir := t.TempDir()
		for _, sub := range []string{"scripts", "references", "assets"} {
			require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0o755))
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "a.sh"), nil, 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "b.txt"), nil, 0o644))
		// subdirectories are skipped
		require.NoError(t, os.Mkdir(filepath.Join(dir, "scripts", "nested"), 0o755))

		files := enumerateResources(dir)
		assert.Len(t, files, 2)
		joined := strings.Join(files, ",")
		assert.Contains(t, joined, "scripts/a.sh")
		assert.Contains(t, joined, "assets/b.txt")
	})
}
