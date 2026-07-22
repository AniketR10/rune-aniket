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

// Package gitenv builds environments for spawning git against a
// specific working directory.
package gitenv

import (
	"os"
	"strings"
)

// perRepoVars are git's per-repository environment overrides — the
// list printed by `git rev-parse --local-env-vars`, plus the hook
// quarantine and discovery-ceiling vars. They pin git to a
// repository regardless of the command's working directory, and git
// exports several of them to hook subprocesses: git spawned from a
// pre-commit hook (or `git rebase -x`, GUI clients, ...) with this
// environment intact operates on the hook's repository instead of
// the directory it was pointed at. That once re-initialized a
// developer checkout as bare when a test suite ran under pre-commit.
var perRepoVars = map[string]struct{}{
	"GIT_ALTERNATE_OBJECT_DIRECTORIES": {},
	"GIT_CONFIG":                       {},
	"GIT_CONFIG_PARAMETERS":            {},
	"GIT_CONFIG_COUNT":                 {},
	"GIT_OBJECT_DIRECTORY":             {},
	"GIT_DIR":                          {},
	"GIT_WORK_TREE":                    {},
	"GIT_IMPLICIT_WORK_TREE":           {},
	"GIT_GRAFT_FILE":                   {},
	"GIT_INDEX_FILE":                   {},
	"GIT_NO_REPLACE_OBJECTS":           {},
	"GIT_REPLACE_REF_BASE":             {},
	"GIT_PREFIX":                       {},
	"GIT_SHALLOW_FILE":                 {},
	"GIT_COMMON_DIR":                   {},
	"GIT_INTERNAL_SUPER_PREFIX":        {},
	"GIT_QUARANTINE_PATH":              {},
	"GIT_CEILING_DIRECTORIES":          {},
}

// Environ returns os.Environ() with git's per-repository overrides
// removed, so a spawned git command resolves the repository from its
// working directory. User-level configuration (GIT_SSH_COMMAND,
// GIT_AUTHOR_*, GIT_CONFIG_GLOBAL, ...) is preserved.
func Environ() []string {
	return Sanitize(os.Environ())
}

// Sanitize returns env with git's per-repository overrides removed.
// Use it when the base environment is built by other means (e.g.
// exec.Cmd.Environ, which adjusts PWD for the command's Dir).
func Sanitize(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, _, _ := strings.Cut(kv, "=")
		if _, ok := perRepoVars[name]; ok {
			continue
		}
		// The numbered companions of GIT_CONFIG_COUNT.
		if strings.HasPrefix(name, "GIT_CONFIG_KEY_") ||
			strings.HasPrefix(name, "GIT_CONFIG_VALUE_") {
			continue
		}
		out = append(out, kv)
	}
	return out
}
