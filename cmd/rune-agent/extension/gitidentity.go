// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package extension

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/gitenv"
)

const gitTimeout = 10 * time.Second

// gitIdentity captures the identity of a git repository and its worktrees.
// Two worktrees of the same repo share the same commonDir.
type gitIdentity struct {
	commonDir string            // absolute path from `git rev-parse --git-common-dir`
	worktrees map[string]string // workspace URI string → filepath.Base(path)
}

// resolveGitIdentity discovers the git common directory and all worktrees
// for the repository at cwd. The worktrees map is keyed by full URI strings
// (matching the scheme/host of cwdURI) so it can be compared directly
// against Dialogue.WorkspaceURI. Returns an empty gitIdentity if cwd is not
// inside a git repo or on any error.
func resolveGitIdentity(ctx context.Context, exec workspaceapi.Executor, cwdURI workspaceapi.URI) gitIdentity {
	cwd := cwdURI.Path()
	commonDir, err := runGitOutput(ctx, exec, cwd, "rev-parse", "--git-common-dir")
	if err != nil {
		return gitIdentity{}
	}
	commonDir = strings.TrimSpace(commonDir)
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(cwd, commonDir)
	}
	commonDir = filepath.Clean(commonDir)

	porcelain, err := runGitOutput(ctx, exec, cwd, "worktree", "list", "--porcelain")
	if err != nil {
		return gitIdentity{}
	}

	wts := make(map[string]string)
	for _, line := range strings.Split(porcelain, "\n") {
		if wtPath, ok := strings.CutPrefix(line, "worktree "); ok {
			wtPath = strings.TrimSpace(wtPath)
			if wtPath == "" {
				continue
			}
			// Build a URI with the same scheme/host as cwdURI but the
			// worktree's filesystem path, so it matches Dialogue.WorkspaceURI.
			wtURI, err := workspaceapi.ParseURI(
				cwdURI.Scheme() + "://" + cwdURI.Host() + wtPath,
			)
			if err != nil {
				continue
			}
			wts[wtURI.String()] = filepath.Base(wtPath)
		}
	}

	return gitIdentity{commonDir: commonDir, worktrees: wts}
}

// runGitOutput runs a git command in the given directory and returns its stdout.
func runGitOutput(ctx context.Context, exec workspaceapi.Executor, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	watcher := workspaceapi.ChanProcessWatcher(make(chan error, 1))

	_, err := exec.Start(ctx, workspaceapi.Cmd{
		Path:    "git",
		Args:    args,
		Dir:     dir,
		Env:     gitenv.Environ(),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Watcher: watcher,
	})
	if err != nil {
		return "", err
	}

	select {
	case procErr := <-watcher.WatchProcess():
		if procErr != nil {
			return "", procErr
		}
	case <-ctx.Done():
		return "", ctx.Err()
	}

	return stdout.String(), nil
}
