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
}

// newCmdGitService creates and initializes a cmdGitService.
func newCmdGitService(exec workspaceapi.Executor) gitService {
	c := new(cmdGitService)
	c.exec = exec
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
func (c *cmdGitService) git(args []string) (string, error) {
	var stdout, stderr bytes.Buffer
	ch := make(chan error)
	cmd := workspaceapi.Cmd{
		Path:    "git",
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

var (
	errDiffNoChanges = errors.New("diff no changes")
)

// diff satisfies gitService.
func (c *cmdGitService) diff(workPath string) (*diff.FileDiff, error) {
	if workPath == "" {
		return nil, errors.New("empty git rel file path")
	}

	out, err := c.git([]string{"diff", "-U0", workPath})
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
