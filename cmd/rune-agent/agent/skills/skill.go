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
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill holds a parsed SKILL.md.
type Skill struct {
	Name          string            // from frontmatter (required)
	Description   string            // from frontmatter (required)
	Body          string            // markdown below frontmatter
	Dir           string            // absolute path to skill directory
	License       string            // optional
	Compatibility string            // optional
	Metadata      map[string]string // optional
	AllowedTools  string            // optional, space-delimited
	Type          string            // "" for prompt-skill, "agent" for agent-skill
	Model         string            // optional model override for agent-type skills
	// ParentContext, when true, opts an agent skill into receiving the
	// parent dialogue's prior user/assistant messages as initial
	// context when the skill is invoked via a slash command. Default
	// false keeps the sub-agent isolated.
	ParentContext bool
}

type frontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  string            `yaml:"allowed-tools"`
	Type          string            `yaml:"type"`
	Model         string            `yaml:"model"`
	ParentContext bool              `yaml:"parent-context"`
}

var nameRegexp = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// Parse splits YAML frontmatter from body, validates fields, and returns a Skill.
// Validation is lenient: name format issues are logged as warnings but still load.
// Only missing name, missing description, or unparseable YAML cause errors.
func Parse(data []byte, dir string) (Skill, error) {
	fm, body, err := splitFrontmatter(data)
	if err != nil {
		return Skill{}, err
	}

	var meta frontmatter
	if err := yaml.Unmarshal(fm, &meta); err != nil {
		return Skill{}, fmt.Errorf("parse frontmatter: %w", err)
	}

	if meta.Name == "" {
		return Skill{}, fmt.Errorf("missing required field: name")
	}
	if meta.Description == "" {
		return Skill{}, fmt.Errorf("missing required field: description")
	}

	// Lenient validation: warn but load anyway.
	warnName(meta.Name, dir)

	return Skill{
		Name:          meta.Name,
		Description:   meta.Description,
		Body:          body,
		Dir:           dir,
		License:       meta.License,
		Compatibility: meta.Compatibility,
		Metadata:      meta.Metadata,
		AllowedTools:  meta.AllowedTools,
		Type:          meta.Type,
		Model:         meta.Model,
		ParentContext: meta.ParentContext,
	}, nil
}

// warnName logs warnings for name issues without blocking loading.
func warnName(name, dir string) {
	log := slog.With("name", name, "dir", dir)
	if len(name) > 64 {
		log.Warn("skill name exceeds 64 characters")
	}
	if !nameRegexp.MatchString(name) {
		log.Warn("skill name does not match recommended format (lowercase alphanumeric with single hyphens)")
	}
	if strings.Contains(name, "--") {
		log.Warn("skill name contains consecutive hyphens")
	}
	if dirBase := filepath.Base(dir); dir != "" && dirBase != name {
		log.Warn("skill name does not match parent directory", "directory", dirBase)
	}
}

func splitFrontmatter(data []byte) (fm []byte, body string, err error) {
	s := string(data)
	if !strings.HasPrefix(s, "---") {
		return nil, "", fmt.Errorf("missing frontmatter delimiters")
	}
	rest := s[3:]
	fmStr, after, ok := strings.Cut(rest, "\n---")
	if !ok {
		return nil, "", fmt.Errorf("missing frontmatter delimiters")
	}
	// Strip leading newline from body
	after = strings.TrimPrefix(after, "\n")
	return []byte(fmStr), after, nil
}
