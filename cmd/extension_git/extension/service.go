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
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
	"unstable.build/go-tui/api/workspaceapi"
)

var _ gitService = (*cmdGitService)(nil)

// gitService is an interface that wraps methods to perform Git operations.
type gitService interface {
	// diff returns the file diff for the given file.
	diff(ctx context.Context, workPath string) (*diff.FileDiff, error)

	// currentCommit returns the current commit hash.
	currentCommit(ctx context.Context, workPath string) (string, error)

	// remoteURL returns the remote URL given a remote name.
	remoteURL(ctx context.Context, workPath string, remoteName string) (string, error)

	// relPath extracts the path relative to the repository.
	relPath(ctx context.Context, workPath string) (string, error)
}

type cmdGitService struct {
	exec workspaceapi.Executor
	cwd  workspaceapi.URI
	fs   workspaceapi.FileSystem
}

func newCmdGitService(exec workspaceapi.Executor, cwd workspaceapi.URI, fs workspaceapi.FileSystem) *cmdGitService {
	c := new(cmdGitService)
	c.exec = exec
	c.cwd = cwd
	c.fs = fs
	return c
}

var (
	errDiffNoChanges = errors.New("diff no changes")
)

type gitExecError struct {
	exit   error
	stderr string
}

func (e *gitExecError) Error() string {
	return fmt.Sprintf(
		"process exit with non-zero status (%s) %v", e.exit.Error(), e.stderr,
	)
}

// git executes commands using the Git CLI.
func (c *cmdGitService) git(ctx context.Context, workPath string, args []string) (
	string, error,
) {
	if workPath == "" {
		return "", errors.New("call git cmd on empty path")
	}

	if !filepath.IsAbs(workPath) {
		workPath = path.Join(c.cwd.Path(), workPath)
	}

	// ensure workDir is a directory and not a file path
	workDir := workPath
	isDir, err := c.isDir(workDir)
	if err != nil {
		return "", fmt.Errorf("check if is dir: %w", err)
	}
	if !isDir {
		workDir = filepath.Dir(workDir)
	}

	var stdout, stderr bytes.Buffer
	ch := make(chan error)
	cmd := workspaceapi.Cmd{
		Path:    "git",
		Dir:     workDir,
		Args:    args,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
		Stdout:  &stdout,
		Stderr:  &stderr,
	}
	if _, err := c.exec.Start(ctx, cmd); err != nil {
		return "", fmt.Errorf("start process: %v", err)
	}
	if err := <-ch; err != nil {
		// clean any new line there might be
		return "", &gitExecError{
			exit:   err,
			stderr: strings.ReplaceAll(stderr.String(), "\n", " "),
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}

// repoPath provides the local file path of the repository.
func (c *cmdGitService) repoPath(ctx context.Context, workPath string) (string, error) {
	p, err := c.git(ctx, workPath, []string{"rev-parse", "--show-toplevel"})
	if err != nil {
		return "", fmt.Errorf("git cmd: %w", err)
	}

	return p, err
}

// diff satisfies gitService.
func (c *cmdGitService) diff(ctx context.Context, workPath string) (*diff.FileDiff, error) {
	relFile, err := c.relPath(ctx, workPath)
	if err != nil {
		return nil, fmt.Errorf("rel path: %w", err)
	}

	repoPath, err := c.repoPath(ctx, workPath)
	if err != nil {
		return nil, fmt.Errorf("repo path: %w", err)
	}

	// reminder: the `relFile` must exist relative to `repoPath` for the `git
	// diff` to work. It could be `dir1/file1.sh` and `/a/b/c/my-repo`
	// respectively or `/a/b/c/my-repo/dir` and `file1.sh`. At the moment we
	// relativize around repo root path, so it's the former.
	out, err := c.git(ctx, repoPath, []string{"diff", "-U0", "--no-ext-diff", relFile})
	if err != nil {
		return nil, fmt.Errorf("git cmd: %w", err)
	}

	if out == "" {
		return nil, errDiffNoChanges
	}

	r := diff.NewFileDiffReader(bytes.NewBufferString(out))
	diff, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("file diff reader: %w", err)
	}
	if diff == nil {
		return nil, errors.New("parse nil diff")
	}

	return diff, nil
}

// currentCommit satisfies gitService.
func (c *cmdGitService) currentCommit(
	ctx context.Context, workPath string,
) (string, error) {
	_, err := c.relPath(ctx, workPath)
	if err != nil {
		return "", fmt.Errorf("rel path: %w", err)
	}
	out, err := c.git(ctx, workPath, []string{"rev-parse", "HEAD"})
	if err != nil {
		return "", err
	}
	return out, err
}

// remoteURL satisfies gitService.
func (c *cmdGitService) remoteURL(
	ctx context.Context, workDir string, remoteName string,
) (string, error) {
	if remoteName == "" {
		return "", errors.New("must pass remote name")
	}
	out, err := c.git(ctx, workDir, []string{"remote", "get-url", remoteName})
	if err != nil {
		return "", err
	}
	return out, err
}

// isDir detects if the passed file is a directory or not in the file system.
func (c *cmdGitService) isDir(file string) (bool, error) {
	info, err := c.fs.Stat(file)
	if err != nil {
		return false, err
	}
	return info.IsDir(), err
}

// relPath satisfies gitService
func (c *cmdGitService) relPath(ctx context.Context, workPath string) (
	relFile string, err error,
) {
	repo, err := c.repoPath(ctx, workPath)
	if err != nil {
		return relFile, fmt.Errorf("repo path: %w", err)
	}

	if !filepath.IsAbs(workPath) {
		workPath = path.Join(c.cwd.Path(), workPath)
	}

	// process file path since usually on macOS temp folders on
	// `/var/folders/...` are living really under `/private/var/folders/...`
	// (the former is symlinked).
	workPath = filepath.Clean(workPath)

	relFile, err = filepath.Rel(repo, workPath)
	if err != nil {
		return relFile, fmt.Errorf("file path rel: %w", err)
	}

	return relFile, nil
}
