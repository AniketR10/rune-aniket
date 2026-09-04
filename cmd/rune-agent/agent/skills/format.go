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

package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FormatSkillContent produces the canonical <skill_content> block for a skill,
// including its body, directory hint, and resource listing.
func FormatSkillContent(skill Skill) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<skill_content name=%q>\n", skill.Name)
	b.WriteString(skill.Body)
	fmt.Fprintf(&b, "\n\nSkill directory: %s\nRelative paths in this skill are relative to the skill directory.", skill.Dir)

	if resources := enumerateResources(skill.Dir); len(resources) > 0 {
		b.WriteString("\n\n<skill_resources>")
		for _, r := range resources {
			fmt.Fprintf(&b, "\n  <file>%s</file>", r)
		}
		b.WriteString("\n</skill_resources>")
	}

	b.WriteString("\n</skill_content>")
	return b.String()
}

// ResourceDirs are the subdirectories scanned for skill resources.
var ResourceDirs = []string{"scripts", "references", "assets"}

// enumerateResources lists files in the skill's resource subdirectories.
func enumerateResources(skillDir string) []string {
	var files []string
	for _, sub := range ResourceDirs {
		entries, err := os.ReadDir(filepath.Join(skillDir, sub))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			files = append(files, filepath.Join(sub, e.Name()))
		}
	}
	return files
}
