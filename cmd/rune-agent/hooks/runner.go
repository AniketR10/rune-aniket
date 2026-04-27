// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// maxOutputBytes caps stdout / response-body / additionalContext to
// avoid unbounded growth from a misbehaving hook.
const maxOutputBytes = 10_000

// Notifier is a minimal subset of browserapi.Notifications used by the
// hook runner to surface non-fatal warnings to the user.
type Notifier interface {
	Notify(level browserapi.NotificationLevel, msg string, args ...any) (string, error)
}

// Runner dispatches configured hooks for the given event. It is safe
// for concurrent use.
type Runner struct {
	cfg        Config
	logger     *slog.Logger
	noti       Notifier
	httpClient *http.Client
	executor   workspaceapi.Executor

	// projectDir is the workspace root, propagated to command hooks
	// via the RUNE_PROJECT_DIR environment variable.
	projectDir string
}

// NewRunner returns a Runner that dispatches hooks defined in cfg.
// executor is required to dispatch command-type hooks; passing nil
// causes those hooks to fail with a non-blocking warning. noti may be
// nil.
func NewRunner(
	cfg Config, exec workspaceapi.Executor,
	noti Notifier, projectDir string,
) *Runner {
	return &Runner{
		cfg:        cfg,
		logger:     slog.With("struct", "hooks.Runner"),
		noti:       noti,
		executor:   exec,
		projectDir: projectDir,
	}
}

// Result is the merged outcome of dispatching all hooks for an event.
// Decision == "block" means at least one hook returned a block; the
// last block reason wins.
type Result struct {
	Continue          bool
	StopReason        string
	Decision          string
	Reason            string
	AdditionalContext string
	SystemMessage     string
	SuppressOutput    bool
}

// Blocked reports whether the result indicates the caller should
// block the corresponding action.
func (r Result) Blocked() bool { return r.Decision == "block" }

// hookResult carries one hook's parsed output plus any non-fatal
// warning produced while running it.
type hookResult struct {
	output HookOutput
	stdout string
	stderr string
	warn   error
}

// Run dispatches every matching hook for the given event in parallel
// and merges their results. A nil receiver is a no-op (returns the
// zero Result with Continue=true).
func (r *Runner) Run(ctx context.Context, payload Payload) Result {
	if r == nil {
		return Result{Continue: true}
	}
	groups := r.cfg.groupsFor(payload.HookEventName)
	if len(groups) == 0 {
		return Result{Continue: true}
	}
	disc := payload.matcherDiscriminator()

	// Collect every matching hook before dispatch so we can size the
	// fan-in channel accurately.
	var jobs []Hook
	for _, g := range groups {
		if !matchValue(g.Matcher, disc) {
			continue
		}
		jobs = append(jobs, g.Hooks...)
	}
	if len(jobs) == 0 {
		return Result{Continue: true}
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		r.logger.Warn("hooks: marshal payload", "error", err, "event", payload.HookEventName)
		return Result{Continue: true}
	}

	env := r.commandEnv(payload)

	results := make([]hookResult, len(jobs))
	var wg sync.WaitGroup
	for i, h := range jobs {
		wg.Add(1)
		go func(i int, h Hook) {
			defer wg.Done()
			switch h.Type {
			case HookTypeCommand:
				results[i] = r.runCommand(ctx, h, env, payload, payloadJSON)
			case HookTypeHTTP:
				results[i] = r.runHTTP(ctx, h, payloadJSON)
			}
		}(i, h)
	}
	wg.Wait()

	return r.merge(payload.HookEventName, results)
}

func (r *Runner) merge(ev Event, results []hookResult) Result {
	out := Result{Continue: true}
	var ctxBuf, sysBuf strings.Builder
	for _, res := range results {
		if res.warn != nil {
			r.logger.Warn("hooks: handler warning",
				"event", ev,
				"error", res.warn,
				"stderr", trim(res.stderr, 256))
			if r.noti != nil {
				_, _ = r.noti.Notify(browserapi.LevelWarn,
					"hook %s: %v", ev, res.warn)
			}
			continue
		}
		o := res.output
		if o.Continue != nil && !*o.Continue {
			out.Continue = false
			if o.StopReason != "" {
				out.StopReason = o.StopReason
			}
		}
		if o.SuppressOutput {
			out.SuppressOutput = true
		}
		if o.Decision == "block" {
			out.Decision = "block"
			if o.Reason != "" {
				out.Reason = o.Reason
			}
		}
		if o.SystemMessage != "" {
			appendBounded(&sysBuf, o.SystemMessage)
		}
		if o.HookSpecificOutput != nil && o.HookSpecificOutput.AdditionalContext != "" {
			appendBounded(&ctxBuf, o.HookSpecificOutput.AdditionalContext)
		}
	}
	out.AdditionalContext = ctxBuf.String()
	out.SystemMessage = sysBuf.String()
	return out
}

// commandEnv builds the env-var slice passed to command hooks.
func (r *Runner) commandEnv(p Payload) []string {
	env := []string{
		"RUNE_HOOK_EVENT=" + string(p.HookEventName),
		"RUNE_DIALOGUE_ID=" + p.SessionID,
	}
	if r.projectDir != "" {
		env = append(env, "RUNE_PROJECT_DIR="+r.projectDir)
	}
	return env
}

func appendBounded(b *strings.Builder, s string) {
	if b.Len() >= maxOutputBytes {
		return
	}
	if b.Len() > 0 {
		b.WriteString("\n")
	}
	remaining := maxOutputBytes - b.Len()
	if len(s) > remaining {
		s = s[:remaining]
		slog.Warn("hooks: output truncated to maxOutputBytes", "limit", maxOutputBytes)
	}
	b.WriteString(s)
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// EnvelopeJSON returns the JSON encoding of payload, useful for
// tests.
func EnvelopeJSON(p Payload) ([]byte, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}
	return b, nil
}
