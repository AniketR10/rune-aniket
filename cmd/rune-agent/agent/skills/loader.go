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
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// loadDir scans dir for subdirectories containing SKILL.md using the
// registry's FileSystem. Returns all successfully parsed skills; logs
// and skips failures.
func (r *SkillRegistry) loadDir(dir string) []Skill {
	entries, err := r.fs.ReadDir(dir)
	if err != nil {
		slog.Debug("skills: cannot read directory", "dir", dir, "error", err)
		return nil
	}

	var result []Skill
	for _, e := range entries {
		if !e.IsDir() {
			if e.Type()&os.ModeSymlink == 0 {
				continue
			}
			info, err := r.fs.Stat(filepath.Join(dir, e.Name()))
			if err != nil || !info.IsDir() {
				continue
			}
		}
		skillPath := filepath.Join(dir, e.Name(), "SKILL.md")
		data, err := r.readFile(skillPath)
		if err != nil {
			continue // no SKILL.md in this subdirectory
		}
		skillDir := filepath.Join(dir, e.Name())
		skill, err := Parse(data, skillDir)
		if err != nil {
			slog.Warn("skills: failed to parse",
				"path", skillPath, "error", err)
			continue
		}
		result = append(result, skill)
	}
	return result
}

// readFile reads the entire contents of path via r.fs.
func (r *SkillRegistry) readFile(path string) ([]byte, error) {
	f, err := r.fs.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}
