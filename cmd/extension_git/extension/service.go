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
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/go-diff/diff"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

// gitService is an interface that wraps methods to perform Git operations.
type gitService interface {
	// diff returns the file diff for the given file.
	diff(workPath string) (*diff.FileDiff, error)
}

// cmdGitService implements gitService using the local Git CLI installation.
type cmdGitService struct {
	exec workspaceapi.Executor
	cwd  workspaceapi.URI
}

// newCmdGitService creates and initializes a cmdGitService.
func newCmdGitService(exec workspaceapi.Executor, cwd workspaceapi.URI) gitService {
	c := new(cmdGitService)
	c.exec = exec
	c.cwd = cwd
	return c
}

// gitExecError represents a failed git command execution.
type gitExecError struct {
	exit   error
	stderr string
}

func (e *gitExecError) Error() string {
	return fmt.Sprintf(
		"process exit with non-zero status (%s) %v", e.exit.Error(), e.stderr,
	)
}

// git executes commands on the Git CLI.
func (c *cmdGitService) git(workDir string, args []string) (string, error) {
	if err := validateWorkDir(workDir); err != nil {
		return "", err
	}

	var stdout, stderr bytes.Buffer
	ch := make(chan error)
	cmd := workspaceapi.Cmd{
		Path:    "git",
		Dir:     workDir,
		Args:    args,
		Watcher: workspaceapi.ChanWatcher(ch),
		Stdout:  &stdout,
		Stderr:  &stderr,
	}
	if _, err := c.exec.Start(cmd); err != nil {
		return "", fmt.Errorf("start process: %v", err)
	}
	if err := <-ch; err != nil {
		return "", &gitExecError{exit: err, stderr: stderr.String()}
	}

	return strings.TrimSpace(stdout.String()), nil
}

// repoPath satisfies gitService.
func (c *cmdGitService) repoPath(workPath string) (string, error) {
	p, err := c.git(workPath, []string{"rev-parse", "--show-toplevel"})
	if err != nil {
		return "", fmt.Errorf("git cmd: %w", err)
	}

	return p, err
}

var (
	errDiffNoChanges = errors.New("diff no changes")
)

// diff satisfies gitService.
func (c *cmdGitService) diff(workPath string) (*diff.FileDiff, error) {
	if workPath == "" {
		return nil, errors.New("empty git rel file path")
	}
	fileRelPath, repoPath, err := c.extractRelPath(workPath)
	if err != nil {
		return nil, fmt.Errorf("extract file rel path: %w", err)
	}

	out, err := c.git(repoPath, []string{"diff", "-U0", fileRelPath})
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

// extractGitRelPath gives the file path relative to the repository root
func (c *cmdGitService) extractRelPath(file string) (
	fileRelPath string, repoPath string, err error,
) {
	if !filepath.IsAbs(file) {
		file = path.Join(c.cwd.Path(), file)
	}

	// process file path since usually on macOS temp folders on
	// `/var/folders/...` are living really under `/private/var/folders/...`
	// (the former is symlinked).
	file = filepath.Clean(file)

	// if it's a directory already, `fileDir` will be set to `filePath`.
	fileDir := filepath.Dir(file)

	repoPath, err = c.repoPath(fileDir)
	if err != nil {
		return fileRelPath, repoPath, fmt.Errorf("repo path: %w", err)
	}

	fileRelPath, err = filepath.Rel(repoPath, file)
	if err != nil {
		return fileRelPath, repoPath, fmt.Errorf("file path rel: %w", err)
	}

	if fileRelPath == "" {
		// this would be very strange... even filepath.Rel("/a/dir", "/a/dir")
		// would give us `"."` and not `""` but better to be safe
		return fileRelPath, repoPath, errors.New("file path rel empty result")
	}

	return fileRelPath, repoPath, nil
}

// validateWorkDir protects from problematic work directories.
func validateWorkDir(workDir string) error {
	if workDir == "" {
		return errors.New("empty workDir")
	}

	return nil
}
