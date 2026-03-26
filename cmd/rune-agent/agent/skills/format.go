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
