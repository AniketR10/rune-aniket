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

package memory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
)

const recallTimeout = 30 * time.Second

// Available reports whether the memory workspace at dataPath is
// bootstrapped (go.mod exists) and ready to query.
func Available(fs workspaceapi.FileSystem, dataPath string) bool {
	_, err := fs.Stat(filepath.Join(dataPath, "go.mod"))
	return err == nil
}

// Recaller implements agent.MemoryRecaller by delegating to the
// memory query engine. It checks Available on each call so it
// picks up a newly bootstrapped workspace without restart.
type Recaller struct {
	fs       workspaceapi.FileSystem
	exec     workspaceapi.Executor
	dataPath string
}

// NewRecaller creates a Recaller for the memory workspace at dataPath.
func NewRecaller(
	fs workspaceapi.FileSystem, exec workspaceapi.Executor, dataPath string,
) *Recaller {
	return &Recaller{fs: fs, exec: exec, dataPath: dataPath}
}

// Recall returns relevant memories for the given context, or an error
// if the workspace is not bootstrapped or the query engine fails.
func (r *Recaller) Recall(ctx context.Context, files []string, task string) ([]agent.Memory, error) {
	if !Available(r.fs, r.dataPath) {
		return nil, errors.New("memory module not available")
	}
	raw, err := Recall(ctx, r.exec, r.dataPath, RecallInput{
		Files: files,
		Task:  task,
	})
	if err != nil {
		return nil, err
	}
	return parseMemories(raw), nil
}

// parseMemories parses the output of the memory recall engine into
// structured Memory entries. Each line has the format:
//
//	[id] (categories) content
//
// Lines that don't match the expected format are skipped.
func parseMemories(raw string) []agent.Memory {
	if raw == "" {
		return nil
	}
	var memories []agent.Memory
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "[") {
			continue
		}
		closeBracket := strings.Index(line, "] ")
		if closeBracket < 0 {
			continue
		}
		id := line[1:closeBracket]
		rest := line[closeBracket+2:]
		memories = append(memories, agent.Memory{ID: id, Content: rest})
	}
	return memories
}

// RecallInput holds parameters for Recall.
type RecallInput struct {
	Files  []string
	Errors []string
	Task   string
	Budget int // 0 uses default (10).
}

// Recall runs the memory query engine (`go run .`) with the given
// context flags and returns the formatted output. Returns ("", nil)
// if the engine produces no output (no memories matched).
func Recall(
	ctx context.Context, exec workspaceapi.Executor,
	dataPath string, input RecallInput,
) (string, error) {
	budget := input.Budget
	if budget <= 0 {
		budget = 10
	}

	args := []string{"run", "."}
	if len(input.Files) > 0 {
		args = append(args, "-files", strings.Join(input.Files, ","))
	}
	if len(input.Errors) > 0 {
		args = append(args, "-errors", strings.Join(input.Errors, ","))
	}
	if input.Task != "" {
		args = append(args, "-task", input.Task)
	}
	args = append(args, "-budget", strconv.Itoa(budget))

	out, err := runGoCommand(ctx, exec, dataPath, args)
	if err != nil {
		return "", fmt.Errorf("memory recall: %w", err)
	}
	return out, nil
}

// FetchConversation runs the memory query engine with
// `-conversation <id>` and returns the original conversation
// transcript for the given memory.
func FetchConversation(
	ctx context.Context, exec workspaceapi.Executor,
	dataPath string, memoryID string,
) (string, error) {
	args := []string{"run", ".", "-conversation", memoryID}
	out, err := runGoCommand(ctx, exec, dataPath, args)
	if err != nil {
		return "", fmt.Errorf("fetch conversation %s: %w", memoryID, err)
	}
	return out, nil
}

// runGoCommand executes `go <args>` in the memory workspace directory,
// captures stdout, and returns the trimmed output. Returns ("", nil)
// when stdout is empty.
func runGoCommand(
	ctx context.Context, exec workspaceapi.Executor,
	dataPath string, args []string,
) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, recallTimeout)
	defer cancel()

	var stdout, stderr bytes.Buffer
	watcher := workspaceapi.ChanProcessWatcher(make(chan error, 1))

	_, err := exec.Start(ctx, workspaceapi.Cmd{
		Path:    "go",
		Args:    args,
		Dir:     dataPath,
		Env:     os.Environ(),
		Stdout:  &stdout,
		Stderr:  &stderr,
		Watcher: watcher,
	})
	if err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	select {
	case procErr := <-watcher.WatchProcess():
		if procErr != nil {
			return "", fmt.Errorf("%w\n%s", procErr, stderr.String())
		}
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("timed out after %v", recallTimeout)
		}
		return "", ctx.Err()
	}

	out := strings.TrimSpace(stdout.String())
	if out == "" {
		return "", nil
	}
	return out, nil
}
