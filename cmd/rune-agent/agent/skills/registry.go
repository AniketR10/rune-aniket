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
	"os/user"
	"slices"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

	builtins "unstable.build/go-tui/cmd/rune-agent/skills"
)

// SkillRegistry is a thread-safe, mutable registry of skills.
// Created once at startup and shared by the skill tool and agentshell.
type SkillRegistry struct {
	mu     sync.RWMutex
	fs     workspaceapi.FileSystem
	byName map[string]Skill
	dirs   []string // tracked directories (absolute paths)
	cwd    workspaceapi.URI
	n      browserapi.Notifications
}

// NewRegistry creates a registry pre-populated from the given directories.
// fs is used to read skill directories and SKILL.md files.
// cwd is used to resolve relative dirs. n may be nil (warnings
// are silently dropped).
func NewRegistry(fs workspaceapi.FileSystem, cwd workspaceapi.URI, dirs []string, n browserapi.Notifications) *SkillRegistry {
	r := &SkillRegistry{
		fs:     fs,
		byName: make(map[string]Skill),
		cwd:    cwd,
		n:      n,
	}
	for _, dir := range dirs {
		abs := r.resolve(dir)
		r.dirs = append(r.dirs, abs)
		for _, s := range r.loadDir(abs) {
			r.warnDescription(s)
			if existing, exists := r.byName[s.Name]; !exists {
				r.byName[s.Name] = s
			} else {
				r.notify("skill %q shadowed: keeping %s, ignoring %s",
					s.Name, existing.Dir, s.Dir)
			}
		}
	}
	r.registerBuiltins()
	return r
}

// Get returns the skill with the given name, or false.
func (r *SkillRegistry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byName[name]
	return s, ok
}

// GetFold returns the skill whose name matches case-insensitively,
// or false. Exact match is tried first; on miss it scans all names.
func (r *SkillRegistry) GetFold(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if s, ok := r.byName[name]; ok {
		return s, true
	}
	for k, s := range r.byName {
		if strings.EqualFold(k, name) {
			return s, true
		}
	}
	return Skill{}, false
}

// List returns a snapshot of all registered skills, sorted by name.
func (r *SkillRegistry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Skill, 0, len(r.byName))
	for _, s := range r.byName {
		result = append(result, s)
	}
	slices.SortFunc(result, func(a, b Skill) int {
		return strings.Compare(a.Name, b.Name)
	})
	return result
}

// Dirs returns a snapshot of tracked directories.
func (r *SkillRegistry) Dirs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.dirs)
}

// AddDir adds a skill directory. Resolves relative paths against
// cwd (stored at construction). Scans the directory and
// registers all found skills. Returns the list of newly added skills.
// Returns error if directory is already tracked.
func (r *SkillRegistry) AddDir(dir string) ([]Skill, error) {
	abs := r.resolve(dir)

	r.mu.Lock()
	defer r.mu.Unlock()

	if slices.Contains(r.dirs, abs) {
		return nil, fmt.Errorf("directory already tracked: %s", abs)
	}

	loaded := r.loadDir(abs)
	var added []Skill
	for _, s := range loaded {
		r.warnDescription(s)
		if existing, exists := r.byName[s.Name]; !exists {
			r.byName[s.Name] = s
			added = append(added, s)
		} else {
			r.notify("skill %q shadowed: keeping %s, ignoring %s",
				s.Name, existing.Dir, s.Dir)
		}
	}
	r.dirs = append(r.dirs, abs)
	return added, nil
}

// RemoveDir removes a skill directory and unregisters all skills
// whose Dir is under that directory. Returns error if dir not tracked.
func (r *SkillRegistry) RemoveDir(dir string) error {
	abs := r.resolve(dir)

	r.mu.Lock()
	defer r.mu.Unlock()

	idx := slices.Index(r.dirs, abs)
	if idx < 0 {
		return fmt.Errorf("directory not tracked: %s", abs)
	}

	r.dirs = slices.Delete(r.dirs, idx, idx+1)
	for name, s := range r.byName {
		if strings.HasPrefix(s.Dir, abs) {
			delete(r.byName, name)
		}
	}
	return nil
}

// Reload re-scans all tracked directories and re-registers builtins.
// New or updated skills become visible; skills whose SKILL.md was
// removed are dropped. This is safe to call from the agent loop on
// every turn so that out-of-band skill installations are picked up.
func (r *SkillRegistry) Reload() {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Reset to empty, then re-load from tracked dirs + builtins.
	r.byName = make(map[string]Skill, len(r.byName))
	for _, abs := range r.dirs {
		for _, s := range r.loadDir(abs) {
			r.warnDescription(s)
			if existing, exists := r.byName[s.Name]; !exists {
				r.byName[s.Name] = s
			} else {
				r.notify("skill %q shadowed: keeping %s, ignoring %s",
					s.Name, existing.Dir, s.Dir)
			}
		}
	}
	r.registerBuiltins()
}

func (r *SkillRegistry) resolve(dir string) string {
	expanded, err := workspaceapi.ExpandPath(dir, user.Current, func() (string, error) {
		return r.cwd.Path(), nil
	})
	if err != nil {
		return dir
	}
	return expanded
}

func (r *SkillRegistry) warnDescription(s Skill) {
	if len(s.Description) > 1024 {
		r.notify("skill %q: description exceeds 1024 characters (%d)",
			s.Name, len(s.Description))
	}
}

func (r *SkillRegistry) registerBuiltins() {
	for _, entry := range []struct {
		data []byte
		name string
	}{
		{builtins.Explore, "explore"},
		{builtins.Plan, "plan"},
	} {
		s, err := Parse(entry.data, "<builtin>/"+entry.name)
		if err != nil {
			panic("builtin skill " + entry.name + ": " + err.Error())
		}
		r.byName[s.Name] = s
	}
}

func (r *SkillRegistry) notify(format string, args ...any) {
	if r.n == nil {
		return
	}
	_, _ = r.n.Notify(browserapi.LevelWarn, format, args...)
}
