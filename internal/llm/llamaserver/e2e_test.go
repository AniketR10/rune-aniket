// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

//go:build e2e

package llamaserver_test

// End-to-end suite that drives llamaserver.Service through the
// llmapi.Service interface against a real `llama-server` binary and a
// real, small GGUF model. It verifies that our config→flag mapping, the
// managed-subprocess pool lifecycle, and the OpenAI-compatible streaming
// contract (text, usage, finish reasons, tool calls, cancellation) all
// behave as they did in the in-process llamacpp.Service.
//
// The suite is opt-in. It runs only when:
//
//	RUNE_LLAMASERVER_E2E=1   — opt in, and
//	`llama-server` is resolvable on $PATH (or via
//	RUNE_LLAMASERVER_E2E_BIN pointing at the binary).
//
// Environment overrides:
//
//	RUNE_LLAMASERVER_E2E_BIN    — absolute path to the llama-server binary.
//	                              Defaults to `llama-server` on $PATH.
//	RUNE_LLAMASERVER_E2E_MODEL  — absolute path to a local .gguf file. When
//	                              set, the download step is skipped.
//	RUNE_LLAMASERVER_E2E_OFFLINE=1
//	                            — never hit the network; only use a cached
//	                              model, skip otherwise.
//
// The default model is a Q4_K_M quantization of a small instruction-tuned
// model with a chat template that supports tool calling, pulled once via
// our own ociregistry client and cached under
// ${UserCacheDir}/rune/llamaserver-e2e/.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/internal/llm/llamacpp/ociregistry"
	"unstable.build/rune/internal/llm/llamaserver"
)

const (
	// e2eReference is a small instruction-tuned GGUF whose chat template
	// supports tool calling. Pinned to a specific quant so download size
	// and behaviour stay stable across runs. bartowski's repo is public
	// (no HuggingFace auth) and ships a tool-calling chat template.
	e2eReference = "bartowski/Qwen2.5-3B-Instruct-GGUF:Q4_K_M"
	// e2eContextWindow keeps the KV cache small enough to load on a modest
	// machine while leaving room for the tool-calling prompts below.
	e2eContextWindow = 4096
)

var errSkipE2E = errors.New("llamaserver e2e: skipped")

// e2eEnabled reports whether the opt-in flag is set.
func e2eEnabled() bool {
	return os.Getenv("RUNE_LLAMASERVER_E2E") == "1" ||
		os.Getenv("RUNE_LLAMASERVER_E2E_MODEL") != ""
}

// e2eServerBin resolves the llama-server binary, honouring the override.
func e2eServerBin(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("RUNE_LLAMASERVER_E2E_BIN"); p != "" {
		if _, err := os.Stat(p); err != nil {
			t.Skipf("RUNE_LLAMASERVER_E2E_BIN=%q: %v", p, err)
		}
		return p
	}
	p, err := exec.LookPath("llama-server")
	if err != nil {
		t.Skip("llamaserver e2e: llama-server not found on $PATH " +
			"(set RUNE_LLAMASERVER_E2E_BIN to override)")
	}
	return p
}

// e2eModelPath returns the GGUF path, pulling e2eReference into the
// per-user cache when RUNE_LLAMASERVER_E2E_MODEL is unset.
func e2eModelPath(t *testing.T) string {
	t.Helper()
	if p := os.Getenv("RUNE_LLAMASERVER_E2E_MODEL"); p != "" {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("RUNE_LLAMASERVER_E2E_MODEL=%q: %v", p, err)
		}
		return p
	}

	baseDir, err := os.UserCacheDir()
	require.NoError(t, err)
	cacheDir := filepath.Join(baseDir, "rune", "llamaserver-e2e")
	cache, err := ociregistry.OpenCache(cacheDir)
	require.NoError(t, err)

	ref, err := ociregistry.ParseReferenceWithDefault(e2eReference, "huggingface.co")
	require.NoError(t, err)

	if pr, err := ociregistry.Resolve(cache, ref); err == nil && pr.ModelPath != "" {
		if _, err := os.Stat(pr.ModelPath); err == nil {
			return pr.ModelPath
		}
	}

	if os.Getenv("RUNE_LLAMASERVER_E2E_OFFLINE") == "1" {
		t.Skipf("%v: offline and no cached model at %s", errSkipE2E, cacheDir)
	}

	t.Logf("llamaserver e2e: downloading %s into %s (first-run only)",
		ref.String(), cacheDir)
	client := ociregistry.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	pr, err := client.Pull(ctx, cache, ref, nil)
	if err != nil {
		t.Skipf("%v: pull %s: %v", errSkipE2E, ref.String(), err)
	}
	if pr.ModelPath == "" {
		t.Skipf("%v: no model layer in manifest for %s", errSkipE2E, ref.String())
	}
	return pr.ModelPath
}

// e2eService builds a llamaserver.Service backed by a real os/exec
// executor, the resolved llama-server binary, and the pulled GGUF. The
// service (and its process pool) is closed at test end. cfg lets a test
// tune server flags (e.g. MaxOutputTokens for the length test).
func e2eService(t *testing.T, cfg llamaserver.Config) (*llamaserver.Service, llmapi.ModelEntry) {
	t.Helper()
	if !e2eEnabled() {
		t.Skip("llamaserver e2e: set RUNE_LLAMASERVER_E2E=1 " +
			"(or RUNE_LLAMASERVER_E2E_MODEL=/path/to/model.gguf) to run")
	}
	bin := e2eServerBin(t)
	modelPath := e2eModelPath(t)

	if cfg.StartupTimeout == 0 {
		cfg.StartupTimeout = 3 * time.Minute
	}
	if cfg.IdleTimeout == 0 {
		cfg.IdleTimeout = time.Hour
	}
	cfg.MaxServers = 2

	exec := &realExecutor{}
	t.Cleanup(exec.wait)
	svc := llamaserver.New(
		cfg, exec, llamaserver.NewFixedLocator(bin), nopNotifications{},
	)
	t.Cleanup(func() { _ = svc.Close() })

	entry := llmapi.ModelEntry{
		Name:          "qwen2.5-3b-e2e",
		Provider:      llamaserver.LLMProvider,
		BaseURL:       modelPath,
		ContextWindow: e2eContextWindow,
	}
	return svc, entry
}

// collectStream drains it to completion and returns the accumulated text,
// tool calls, and the single DoneData.
func collectStream(
	t *testing.T, ctx context.Context, it iterator.Iterator[llmapi.Event],
) (text string, tools []llmapi.ToolCall, done *llmapi.DoneData) {
	t.Helper()
	var (
		sb      strings.Builder
		numDone int
	)
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		switch ev.Type {
		case llmapi.EventTextDelta:
			sb.WriteString(ev.Text)
		case llmapi.EventToolCallDone:
			require.NotNil(t, ev.ToolCall, "EventToolCallDone without ToolCall")
			tools = append(tools, *ev.ToolCall)
		case llmapi.EventStreamDone:
			numDone++
			done = ev.DoneData
		case llmapi.EventStreamError:
			t.Fatalf("stream error: %v", ev.Error)
		}
	}
	require.NoError(t, it.Err())
	require.Equal(t, 1, numDone, "expected exactly one EventStreamDone")
	require.NotNil(t, done, "EventStreamDone without DoneData")
	return sb.String(), tools, done
}

// drainText drains it and returns the accumulated text, surfacing any stream
// error (from an EventStreamError, the iterator Err, or a non-completing
// stream) instead of failing the test. Used where a model-template quirk
// should skip rather than fail.
func drainText(ctx context.Context, it iterator.Iterator[llmapi.Event]) (string, error) {
	var sb strings.Builder
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		switch ev.Type {
		case llmapi.EventTextDelta:
			sb.WriteString(ev.Text)
		case llmapi.EventStreamError:
			return sb.String(), ev.Error
		}
	}
	if err := it.Err(); err != nil {
		return sb.String(), err
	}
	return sb.String(), nil
}

// isTemplateError reports whether err is a llama-server chat-template
// rejection (e.g. a Jinja role-alternation exception) rather than a
// transport or client-side failure.
func isTemplateError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "jinja") ||
		strings.Contains(msg, "roles must alternate") ||
		strings.Contains(msg, "template")
}

// nopNotifications is a no-op browserapi.Notifications for the e2e service.
type nopNotifications struct{}

func (nopNotifications) Notify(
	browserapi.NotificationLevel, string, ...any,
) (string, error) {
	return "", nil
}

func (nopNotifications) NotifyOnce(
	browserapi.NotificationLevel, string, ...any,
) (string, error) {
	return "", nil
}

func (nopNotifications) UpdateNotificationProgress(
	string, string, int64, int64,
) error {
	return nil
}

// e2eRecordingNotifications captures notification level+message so the e2e
// load-failure test can assert on the surfaced error text.
type e2eRecordingNotifications struct {
	mu       sync.Mutex
	notifies []e2eNotifyRecord
}

type e2eNotifyRecord struct {
	level browserapi.NotificationLevel
	msg   string
}

func (r *e2eRecordingNotifications) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifies = append(r.notifies, e2eNotifyRecord{
		level: level, msg: fmt.Sprintf(msg, args...),
	})
	return "id", nil
}

func (r *e2eRecordingNotifications) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return r.Notify(level, msg, args...)
}

func (r *e2eRecordingNotifications) UpdateNotificationProgress(
	string, string, int64, int64,
) error {
	return nil
}

// errorMessages returns the formatted message of every error-level Notify.
func (r *e2eRecordingNotifications) errorMessages() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, n := range r.notifies {
		if n.level == browserapi.LevelError {
			out = append(out, n.msg)
		}
	}
	return out
}

// assertJSONObject fails unless s parses as a JSON object. A surrounding
// markdown code fence is stripped first: weaker quants sometimes wrap
// structured output in ```json … ``` even under response_format, which is a
// model quirk rather than a client contract violation.
func assertJSONObject(t *testing.T, s string) {
	t.Helper()
	payload := stripCodeFence(strings.TrimSpace(s))
	var obj map[string]any
	assert.NoErrorf(t, json.Unmarshal([]byte(payload), &obj),
		"expected JSON object, got %q", s)
}

// stripCodeFence removes a single surrounding ``` or ```json markdown fence.
func stripCodeFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[i+1:] // drop the ```json language tag line
	}
	return strings.TrimSuffix(strings.TrimSpace(s), "```")
}
