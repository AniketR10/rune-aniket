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

package headless

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/debug"
)

// Options configures a single headless run.
type Options struct {
	// InstructionsFile is the path to the file whose contents are sent
	// to the agent as the task prompt.
	InstructionsFile string
	// Model is a model argument resolved host-side. Empty means
	// llmapi.DefaultModel.
	Model string
	// Effort overrides the reasoning effort. Empty uses the
	// provider/config default.
	Effort string
	// Stdout receives the JSONL event stream. Defaults to os.Stdout.
	Stdout io.Writer
}

// Result reports the outcome of a headless run.
type Result struct {
	// Trajectory is the ATIF document written as the final stdout line.
	Trajectory Trajectory
	// AgentError is non-nil when the agent loop itself failed. It is
	// distinct from the error returned by Run, which reports a failure
	// to start.
	AgentError error
}

// Run executes one non-interactive agent run against the Rune host that
// launched this process. A non-nil error means the run could not start;
// a failure inside the agent loop is reported through Result.AgentError
// so the trajectory is still emitted.
func Run(ctx context.Context, opts Options) (Result, error) {
	if opts.InstructionsFile == "" {
		return Result{}, errors.New("instructions file is required")
	}
	if opts.Model == "" {
		opts.Model = llmapi.DefaultModel
	}
	out := opts.Stdout
	if out == nil {
		out = os.Stdout
	}

	instructions, err := os.ReadFile(opts.InstructionsFile)
	if err != nil {
		return Result{}, fmt.Errorf("read instructions: %w", err)
	}
	sum := sha256.Sum256(instructions)

	w, err := Connect()
	if err != nil {
		return Result{}, err
	}

	runID := uuid.NewString()
	sess, err := bootstrap(ctx, w, opts, runID)
	if err != nil {
		return Result{}, err
	}
	defer sess.cleanup()

	startedAt := time.Now().UTC()
	builder := NewBuilder(BuilderConfig{
		AgentVersion:    debug.Tag,
		SessionID:       runID,
		ModelName:       sess.model.Name,
		ReasoningEffort: opts.Effort,
		ToolDefinitions: sess.registry.AllTools(),
		Extra: map[string]any{
			"provider":            sess.model.Provider,
			"reasoning_effort":    opts.Effort,
			"workspace":           sess.cwd.String(),
			"commit":              debug.Commit,
			"instructions_file":   opts.InstructionsFile,
			"instructions_sha256": hex.EncodeToString(sum[:]),
			"started_at":          startedAt.Format(time.RFC3339Nano),
		},
	})
	stream := newStreamer(out, builder)

	res := Result{}
	if err := stream.instructions(string(instructions)); err != nil {
		return res, fmt.Errorf("write instructions event: %w", err)
	}
	res.AgentError = drive(ctx, sess.agent, runID, string(instructions), stream)

	builder.cfg.Extra["ended_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	traj, err := stream.finish()
	res.Trajectory = traj
	if err != nil {
		return res, err
	}
	return res, nil
}

// drive consumes the agent event stream to completion, forwarding every
// event to the JSONL stream and the ATIF builder.
func drive(
	ctx context.Context, ag *agent.Agent,
	dialogueID, message string, stream *streamer,
) error {
	it := ag.Run(ctx, dialogueID, message)
	defer func() { _ = it.Close() }()

	var agentErr error
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		if err := stream.event(ev); err != nil {
			return fmt.Errorf("write event: %w", err)
		}
		if ev.Type == agent.EventError && agentErr == nil {
			agentErr = ev.Error
		}
	}
	if err := it.Err(); err != nil && agentErr == nil {
		agentErr = err
	}
	return agentErr
}
