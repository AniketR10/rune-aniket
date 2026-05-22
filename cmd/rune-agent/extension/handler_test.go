// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

package extension

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"

	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/configedit"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmtest"
)

// recordingWindowManager is a stub browserapi.WindowManager that records the
// arguments passed to Tab so tests can verify the visible label.
type recordingWindowManager struct {
	gotURI  workspaceapi.URI
	gotIcon rune
	gotName string
}

func (m *recordingWindowManager) Focus() (browserapi.Window, error) { return nil, nil }
func (m *recordingWindowManager) Split(
	_ browserapi.Orientation, _ browserapi.Window, _ browserapi.Handler,
) (browserapi.Window, error) {
	return nil, nil
}
func (m *recordingWindowManager) Floating(
	_ browserapi.Floating, _ browserapi.FloatingConfig,
) (browserapi.Window, error) {
	return nil, nil
}
func (m *recordingWindowManager) Bar(_ browserapi.BarConfig, _ tui.Handler) error { return nil }
func (m *recordingWindowManager) Tab(
	uri workspaceapi.URI, icon rune, name string, h browserapi.Handler,
) (browserapi.Handler, error) {
	m.gotURI = uri
	m.gotIcon = icon
	m.gotName = name
	return h, nil
}
func (m *recordingWindowManager) SetWindowContent(_ browserapi.Window, _ browserapi.Handler) error {
	return nil
}
func (m *recordingWindowManager) CloseWindow(_ browserapi.Window) error { return nil }

// TestOpenChatTabUsesDialogueIDAsLabel verifies that the visible tab name is
// the dialogue's petname ID (e.g. "rolling-fox") and not the internal
// "rune-agent://<model>/<id>" URI.
func TestOpenChatTabUsesDialogueIDAsLabel(t *testing.T) {
	const dialogueID = "rolling-fox"
	uri, err := getModelUri(dialogueID, "gpt-5")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(uri.String(), "rune-agent://"),
		"precondition: URI must use the rune-agent scheme")

	wm := &recordingWindowManager{}
	_, err = openChatTab(wm, uri, dialogueID, nil)
	require.NoError(t, err)

	assert.Equal(t, dialogueID, wm.gotName,
		"visible tab label must be the dialogue ID, not the internal URI")
	assert.False(t, strings.HasPrefix(wm.gotName, "rune-agent://"),
		"tab label must not be the internal rune-agent:// URI")
	assert.Equal(t, uri, wm.gotURI, "URI must still be passed unchanged as tab identity")
}

// -----------------------------------------------------------------------------
// E2E fixture for Ctrl-C-dismisses-active-prompt
// -----------------------------------------------------------------------------

// stubNotifications is a no-op browserapi.Notifications. wrapDialogueHandler
// calls Notify(LevelInfo, "canceled completion request") when Ctrl-C cancels
// an in-flight completion; the test does not assert on it.
type stubNotifications struct{}

func (stubNotifications) Notify(_ browserapi.NotificationLevel, _ string, _ ...any) (string, error) {
	return "", nil
}
func (stubNotifications) NotifyOnce(_ browserapi.NotificationLevel, _ string, _ ...any) (string, error) {
	return "", nil
}
func (stubNotifications) UpdateNotificationProgress(_, _ string, _, _ int64) error { return nil }

// memDialogueStore is the minimum dialoguemanager.Store needed to drive the
// agent loop in tests.
type memDialogueStore struct {
	mu        sync.Mutex
	dialogues map[string]dialoguemanager.Dialogue
}

func newMemDialogueStore() *memDialogueStore {
	return &memDialogueStore{dialogues: make(map[string]dialoguemanager.Dialogue)}
}

func (s *memDialogueStore) Health(context.Context) error { return nil }

func (s *memDialogueStore) Create(_ context.Context, d dialoguemanager.Dialogue) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.dialogues[d.ID]; ok {
		return storageapi.ErrAlreadyExists
	}
	s.dialogues[d.ID] = d
	return nil
}

func (s *memDialogueStore) Get(_ context.Context, id string) (dialoguemanager.Dialogue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.dialogues[id]
	if !ok {
		return dialoguemanager.Dialogue{}, storageapi.ErrNotFound
	}
	return d, nil
}

func (s *memDialogueStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.dialogues, id)
	return nil
}

func (s *memDialogueStore) AppendMessages(
	_ context.Context, d dialoguemanager.Dialogue, msgs []llmapi.Message, _ llmapi.DialogueUsage,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing := s.dialogues[d.ID]
	existing.Messages = append(existing.Messages, msgs...)
	s.dialogues[d.ID] = existing
	return nil
}

func (s *memDialogueStore) List(
	context.Context,
) (iterator.Iterator[dialoguemanager.DialogueHeader], error) {
	return iterator.FromSlice[dialoguemanager.DialogueHeader](nil), nil
}

func (s *memDialogueStore) ArchiveAndReplace(
	context.Context, dialoguemanager.ArchiveAndReplaceParams,
) error {
	return nil
}

// promptFlusher wraps a wrapped chat tui.Handler with an async barrier
// that drains the dialogue handler's consumeIncoming goroutine and the
// scripted agent loop after every Handle. This lets
// handlertest.RunHandlerSequence drive the public handler interface
// (Handle/Draw/Cursor/Resize) and assert against the rendered frame the
// user would see — no private fields or component methods are touched.
type promptFlusher struct {
	t           *testing.T
	inner       tui.Handler
	interruptCh <-chan struct{}
	settle      time.Duration
}

func (a *promptFlusher) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = a.inner.Handle(ev)
	a.flush()
	return
}

func (a *promptFlusher) Resize(w, h int)    { a.inner.Resize(w, h) }
func (a *promptFlusher) Draw(w term.Writer) { a.inner.Draw(w) }
func (a *promptFlusher) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return a.inner.Cursor()
}
func (a *promptFlusher) Selection() (string, bool) { return a.inner.Selection() }

// flush waits for the dialogue tx consumer to publish at least one
// interrupt and then for it to stay quiet for the settle window, so
// any cascading agent-loop events triggered by the most recent input
// have rendered before the next assertion runs.
//
// The first interrupt may arrive after the agent loop reaches the
// prompter and the prompter sends the prompt event on tx, which can
// involve several goroutine hops. The first-event grace window is
// therefore generous; subsequent events use the shorter settle window
// to detect quiescence.
func (a *promptFlusher) flush() {
	const firstEventGrace = 500 * time.Millisecond
	deadline := time.After(3 * time.Second)
	gotAny := false
	for {
		grace := a.settle
		if !gotAny {
			grace = firstEventGrace
		}
		select {
		case <-a.interruptCh:
			gotAny = true
		case <-time.After(grace):
			return
		case <-deadline:
			a.t.Logf("promptFlusher: timed out draining interrupt channel")
			return
		}
	}
}

// nopFileSystem mirrors the helper from the agent package; redeclared here
// so the extension package does not depend on agent test files.
type nopFileSystem struct{}

func (nopFileSystem) URI(string) (workspaceapi.URI, error) {
	return workspaceapi.URI{}, nil
}
func (nopFileSystem) OpenFile(string, int, os.FileMode) (workspaceapi.File, error) {
	return nil, os.ErrNotExist
}
func (nopFileSystem) Remove(string) error                   { return nil }
func (nopFileSystem) Stat(string) (os.FileInfo, error)      { return nil, os.ErrNotExist }
func (nopFileSystem) ReadDir(string) ([]os.DirEntry, error) { return nil, nil }
func (nopFileSystem) MkdirAll(string, os.FileMode) error    { return nil }

func dirURI(dir string) workspaceapi.URI {
	u, _ := workspaceapi.ParseURI("file://" + dir)
	return u
}

// promptHandlerOpts configures the e2e handler fixture.
type promptHandlerOpts struct {
	// toolArgs is the JSON payload the scripted llmapi.Service emits as
	// the ask_user_question arguments. When empty the agent loop is
	// never started (used by tests that drive prompt events directly).
	toolArgs string
	// extraPrompt, when set, is sent on the dialogue tx channel during
	// setup. Tests that exercise the PromptInputMode branch use this
	// to install a RequiresInput option that ask_user_question itself
	// never produces.
	extraPrompt *dialoguetui.MessageEvent
}

// newPromptHandler wires a stub llmapi.Service through agent.Agent →
// ask_user_question → tuiPrompter → dialoguetui → wrapDialogueHandler
// and returns the resulting tui.Handler wrapped in promptFlusher so
// handlertest.RunHandlerSequence can drive it deterministically.
func newPromptHandler(t *testing.T, opts promptHandlerOpts) tui.Handler {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	// Stub llmapi.Service: one scripted tool-call response, followed by
	// a stop reply so the agent loop terminates after the tool returns
	// "prompt dismissed".
	svc := llmtest.New(
		[]llmapi.ModelEntry{{Provider: "test", Name: "test-model", ContextWindow: 128_000}},
		llmtest.Response{
			ToolCalls: []llmapi.ToolCall{{
				ID:   "call-1",
				Type: llmapi.ToolTypeFunction,
				Function: llmapi.FunctionCall{
					Name:      "ask_user_question",
					Arguments: opts.toolArgs,
				},
			}},
			FinishReason: llmapi.FinishReasonToolCall,
		},
		llmtest.Response{
			Chunks:       []string{"done"},
			FinishReason: llmapi.FinishReasonStop,
		},
	)

	// The interrupter doubles as a barrier: every time the dialogue
	// consumer publishes one (after applying a MessageEvent), the
	// promptFlusher sees it and continues.
	interruptCh := make(chan struct{}, 64)
	interrupter := term.FuncInterrupter(func(context.Context) error {
		select {
		case interruptCh <- struct{}{}:
		default:
		}
		return nil
	})

	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	mu := new(sync.Mutex)
	dhandler, tx, rx := dialoguetui.Handler(ctx, mu, comp, interrupter)

	prompter := &tuiPrompter{tx: tx, noti: stubNotifications{}}
	askUser := agentools.NewAskUser(prompter)
	registry := agent.NewRegistry(askUser)
	skillReg := skills.NewRegistry(nopFileSystem{}, dirURI(""), nil, nil)
	store := newMemDialogueStore()
	ag := agent.NewAgent(svc, registry, skillReg, store, agent.NoMemory(), agent.Config{
		SystemPrompt: "test",
		Model:        llmapi.ModelEntry{Provider: "test", Name: "test-model", ContextWindow: 128_000},
		Prompter:     prompter,
	})

	// Minimal aiEditorHandler: wrapDialogueHandler only reads h.n via
	// the Ctrl-C notify path.
	owner := &aiEditorHandler{n: stubNotifications{}}
	syncComp := syncComponent{mu: mu, comp: comp, h: owner, hintSlot: &hintSlot{}}
	wrapped, msgRx := owner.wrapDialogueHandler(ctx, syncComp, dhandler, rx)

	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case req, ok := <-msgRx:
				if !ok {
					return
				}
				it := ag.Run(req.ctx, "test-dialogue", req.msg)
				for {
					if _, more := it.Next(req.ctx); !more {
						break
					}
				}
				_ = it.Close()
			}
		}
	})

	if opts.extraPrompt != nil {
		tx <- *opts.extraPrompt
		// Wait for the dialogue handler's consumer to apply the event
		// before returning. Without this, the first key the test sends
		// may race ahead of the prompt becoming active.
		select {
		case <-interruptCh:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for extra prompt to be consumed")
		}
	}

	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	return &promptFlusher{
		t:           t,
		inner:       wrapped,
		interruptCh: interruptCh,
		settle:      50 * time.Millisecond,
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}


// dismissedFrame is the rendered frame after a prompt is dismissed:
// the agent's prior tool spinner clears, the user message remains on
// the first line (when a message was submitted), and the empty
// inputbox is back at the bottom with a cursor.
const (
	frameWidth  = 40
	frameHeight = 10
)

// blanks builds a row of trailing spaces of frameWidth length.
func blanks() string { return strings.Repeat(" ", frameWidth) }

func pad(s string) string {
	if len(s) >= frameWidth {
		return s
	}
	return s + strings.Repeat(" ", frameWidth-len(s))
}

func frame(lines ...string) string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = pad(l)
	}
	return strings.Join(out, "\n")
}

// TestE2ECtrlCDismissesSelectionPrompt drives the chat tab handler
// end-to-end through the public handlertest.RunHandlerSequence API:
// the user types "hi" and presses Enter, the scripted LLM emits an
// ask_user_question tool call which renders a selection prompt, and
// Ctrl-C must clear that prompt. The two cases compare the rendered
// frame before and after Ctrl-C, with no access to private dialogue
// component state.
func TestE2ECtrlCDismissesSelectionPrompt(t *testing.T) {
	args := mustJSON(t, map[string]any{
		"questions": []map[string]any{{
			"question": "Pick one", "header": "Choice",
			"options": []map[string]any{
				{"label": "A", "description": ""},
				{"label": "B", "description": ""},
			},
			"multiSelect": false,
		}},
	})
	h := newPromptHandler(t, promptHandlerOpts{toolArgs: args})
	handlertest.RunHandlerSequence(t, h, frameWidth, frameHeight, []handlertest.SequenceTestCase{
		{
			// Submit the user message; the agent loop emits the
			// tool call and the selection prompt becomes visible.
			InputSequence: "hi<enter>",
			Expected: frame(
				"hi",
				"Pick one                        [Choice]",
				blanks(),
				"> A",
				"  B",
				"  Other",
				"      None of the above",
				"   ┌───────────────────────────────┐    ",
				"   │                               │    ",
				"   └───────────────────────────────┘    ",
			),
		},
		{
			// Ctrl-C through the wrapped chat handler must clear
			// the prompt and return focus to the empty input box.
			InputSequence: "<c-c>",
			Expected: frame(
				"hi",
				blanks(), blanks(), blanks(), blanks(), blanks(), blanks(),
				"   ┌───────────────────────────────┐    ",
				"   │▐                              │    ",
				"   └───────────────────────────────┘    ",
			),
		},
	})
}

// -----------------------------------------------------------------------------
// E2E fixture for RUNE-179 (UTF-8 / binary safety across the agent loop)
// -----------------------------------------------------------------------------

// utf8E2EHandler wires the real chat tab handler to a scripted
// llmapi.Service that emits a sequence of read_file / grep_files tool
// calls covering each Layer of the RUNE-179 fix. It returns the
// wrapped tui.Handler plus the scripted service so the test can
// inspect the captured request log after the sequence completes.
func utf8E2EHandler(t *testing.T, svc *llmtest.Service, workspaceDir string) tui.Handler {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	interruptCh := make(chan struct{}, 64)
	interrupter := term.FuncInterrupter(func(context.Context) error {
		select {
		case interruptCh <- struct{}{}:
		default:
		}
		return nil
	})

	comp := dialoguetui.NewComponent(dialoguetui.ComponentConfig{})
	mu := new(sync.Mutex)
	dhandler, _, rx := dialoguetui.Handler(ctx, mu, comp, interrupter)

	cwd, err := workspaceapi.ParseURI("file://" + workspaceDir)
	require.NoError(t, err)

	fs := testLocalFS{root: workspaceDir}
	tools, _ := agentools.DefaultTools(
		fs, testLocalExec{}, cwd, nil, agentools.Config{}, configedit.NopConfig(),
	)
	// DefaultTools does not include grep_files (the host-side ripgrep
	// integration covers that path in production). RUNE-179 fixed the
	// in-process grep_files implementation, so register it explicitly
	// for this e2e fixture.
	tools = append(tools, agentools.NewGrepFiles(fs, cwd, agentools.NewFileTracker()))
	registry := agent.NewRegistry(tools...)
	skillReg := skills.NewRegistry(fs, cwd, nil, nil)
	store := newMemDialogueStore()
	ag := agent.NewAgent(svc, registry, skillReg, store, agent.NoMemory(), agent.Config{
		SystemPrompt: "test",
		Model:        llmapi.ModelEntry{Provider: "test", Name: "test-model", ContextWindow: 128_000},
		Workspace:    cwd,
	})

	owner := &aiEditorHandler{n: stubNotifications{}}
	syncComp := syncComponent{mu: mu, comp: comp, h: owner, hintSlot: &hintSlot{}}
	wrapped, msgRx := owner.wrapDialogueHandler(ctx, syncComp, dhandler, rx)

	var wg sync.WaitGroup
	wg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case req, ok := <-msgRx:
				if !ok {
					return
				}
				it := ag.Run(req.ctx, "test-dialogue", req.msg)
				for {
					if _, more := it.Next(req.ctx); !more {
						break
					}
				}
				_ = it.Close()
			}
		}
	})

	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	return &promptFlusher{
		t:           t,
		inner:       wrapped,
		interruptCh: interruptCh,
		// Tool execution takes longer than ask_user prompts; bump the
		// quiescence window so the multi-tool scripted sequence
		// finishes before the test asserts on captured requests.
		settle: 150 * time.Millisecond,
	}
}

// findToolResult scans messages for the tool-role message that carries
// the result of the given tool-call ID. Returns ("", false) if absent.
func findToolResult(msgs []llmapi.Message, toolCallID string) (string, bool) {
	for _, m := range msgs {
		if m.Role == llmapi.RoleTool && m.ToolCallID == toolCallID {
			return m.Content, true
		}
	}
	return "", false
}

// TestE2EUTF8SafetyAcrossAgentLoop is the RUNE-179 end-to-end
// regression. The scripted LLM walks a workspace that contains:
//
//   - utf8.txt: valid UTF-8 text
//   - latin1.log: text with a stray 0xff byte
//   - bin.dat: binary blob with a NUL in the first 8 KiB
//
// It issues a read_file call against each path and a final grep_files
// over the workspace before replying with "done". The test then
// inspects the captured llmapi.Request log to verify:
//
//  1. Layer 1 — every outgoing string field is valid UTF-8.
//  2. Layer 3 — read_file on the binary file returns a BinaryStub that
//     steers the model toward bash inspection.
//  3. Layer 3 — read_file on the latin-1 file ends with the U+FFFD
//     marker and the bad byte is replaced.
//  4. Layer 3 — read_file on the UTF-8 file is passed through with no
//     marker.
//  5. Layer 4 — grep_files for "needle" returns the UTF-8 file but
//     not the binary blob, even though both contain the literal bytes.
func TestE2EUTF8SafetyAcrossAgentLoop(t *testing.T) {
	dir := t.TempDir()

	// utf8.txt: valid multi-byte text containing the literal "needle"
	// so grep_files has at least one positive hit.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "utf8.txt"),
		[]byte("héllo · 世界\nfind the needle in the haystack\n"), 0o644))
	// latin1.log: ASCII with one stray 0xff byte sliced into a line.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "latin1.log"),
		[]byte("normal line\nbad byte here: \xff oops\nmore text\n"), 0o644))
	// bin.dat: looks binary by both heuristics (NUL in first 8 KiB)
	// and embeds the same "needle" token between binary bytes so the
	// grep filter is the only thing keeping it out of results.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bin.dat"),
		[]byte("\x7fELF\x02\x01\x01\x00\x00\x00needle\x00data\x01\x02"), 0o644))

	// Scripted LLM: four tool-calls then stop. The arguments include a
	// stray invalid UTF-8 byte in a path field on purpose so the
	// wire-level sanitiser in agent.go (Layer 1) has to scrub it
	// before CreateCompletion is invoked on the next turn.
	svc := llmtest.New(
		[]llmapi.ModelEntry{{Provider: "test", Name: "test-model", ContextWindow: 128_000}},
		llmtest.Response{
			ToolCalls: []llmapi.ToolCall{{
				ID:   "call-read-utf8",
				Type: llmapi.ToolTypeFunction,
				Function: llmapi.FunctionCall{
					Name:      "read_file",
					Arguments: `{"path":"utf8.txt","offset":0,"limit":0}`,
				},
			}},
			FinishReason: llmapi.FinishReasonToolCall,
		},
		llmtest.Response{
			ToolCalls: []llmapi.ToolCall{{
				ID:   "call-read-latin1",
				Type: llmapi.ToolTypeFunction,
				Function: llmapi.FunctionCall{
					Name: "read_file",
					// Embed a stray 0xff byte to exercise Layer 1's
					// tool-call Arguments sanitisation.
					Arguments: "{\"path\":\"latin1.log\",\"offset\":0,\"limit\":0,\"_pad\":\"\xff\"}",
				},
			}},
			FinishReason: llmapi.FinishReasonToolCall,
		},
		llmtest.Response{
			ToolCalls: []llmapi.ToolCall{{
				ID:   "call-read-bin",
				Type: llmapi.ToolTypeFunction,
				Function: llmapi.FunctionCall{
					Name:      "read_file",
					Arguments: `{"path":"bin.dat","offset":0,"limit":0}`,
				},
			}},
			FinishReason: llmapi.FinishReasonToolCall,
		},
		llmtest.Response{
			ToolCalls: []llmapi.ToolCall{{
				ID:   "call-grep",
				Type: llmapi.ToolTypeFunction,
				Function: llmapi.FunctionCall{
					Name:      "grep_files",
					Arguments: `{"pattern":"needle"}`,
				},
			}},
			FinishReason: llmapi.FinishReasonToolCall,
		},
		llmtest.Response{
			Chunks:       []string{"done"},
			FinishReason: llmapi.FinishReasonStop,
		},
	)

	h := utf8E2EHandler(t, svc, dir)

	// Drive the handler through its public interface. The user types
	// "hi" and presses Enter; the scripted agent loop runs all four
	// tool calls and the final stop. We only assert on the final
	// frame (assistant text "done" appears) — the per-tool assertions
	// run after the sequence against the captured request log.
	// The chat area shows the user's message at the top; the
	// assistant's "done" reply and tool-spinner lines are written
	// into the dialogue component asynchronously by the agent loop
	// and may not be flushed by the time this frame is captured.
	// The RUNE-179 assertions below run after polling for the full
	// scripted sequence to complete.
	handlertest.RunHandlerSequence(t, h, frameWidth, frameHeight, []handlertest.SequenceTestCase{{
		InputSequence: "hi<enter>",
		Expected: frame(
			"hi",
			blanks(), blanks(), blanks(), blanks(), blanks(), blanks(),
			"   ┌───────────────────────────────┐    ",
			"   │▐                              │    ",
			"   └───────────────────────────────┘    ",
		),
	}})

	// Give the agent loop a final moment to flush the post-tool
	// CreateCompletion calls into the recorder. The promptFlusher
	// settle window covers most of this, but the stop reply itself
	// arrives after the last interrupt so we wait once more.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if svc.CallCount() >= 5 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.GreaterOrEqualf(t, svc.CallCount(), 5,
		"expected scripted agent loop to issue all 5 CreateCompletion calls, got %d", svc.CallCount())

	requests := svc.Requests()

	// (1) Layer 1: every outgoing string field is valid UTF-8.
	for k, r := range requests {
		for i, m := range r.Request.Messages {
			assert.Truef(t, utf8.ValidString(m.Content),
				"request %d message %d Content has invalid UTF-8: %q", k, i, m.Content)
			assert.Truef(t, utf8.ValidString(m.ReasoningContent),
				"request %d message %d ReasoningContent has invalid UTF-8", k, i)
			for j, tc := range m.ToolCalls {
				assert.Truef(t, utf8.ValidString(tc.Function.Arguments),
					"request %d message %d toolcall %d Arguments has invalid UTF-8: %q",
					k, i, j, tc.Function.Arguments)
				assert.Truef(t, utf8.ValidString(tc.Function.Name),
					"request %d message %d toolcall %d Name has invalid UTF-8", k, i, j)
			}
		}
	}

	// The Nth scripted response is processed during the Nth call; the
	// next call (N+1) is the first one whose Messages slice carries
	// the tool-role result from that response. So the message log on
	// request[i+1] is where we can inspect tool result i.
	require.GreaterOrEqual(t, len(requests), 5)
	utf8Msgs := requests[1].Request.Messages
	latin1Msgs := requests[2].Request.Messages
	binMsgs := requests[3].Request.Messages
	grepMsgs := requests[4].Request.Messages

	// (2) Layer 3: binary file → BinaryStub with bash hint.
	binResult, ok := findToolResult(binMsgs, "call-read-bin")
	require.True(t, ok, "expected tool result for call-read-bin in request 3")
	header := regexp.MustCompile(`^<binary file: bin\.dat, \d+ bytes, sha256=[0-9a-f]{64}`)
	assert.Regexp(t, header, binResult, "binary file result must start with the stub header")
	assert.Contains(t, binResult, "use bash",
		"binary stub must instruct the model to fall back to bash")
	assert.Contains(t, binResult, "xxd",
		"binary stub must mention xxd / hexdump / strings as inspection options")

	// (3) Layer 3: latin-1 file → U+FFFD marker, bad byte replaced.
	latin1Result, ok := findToolResult(latin1Msgs, "call-read-latin1")
	require.True(t, ok, "expected tool result for call-read-latin1 in request 2")
	assert.Truef(t, utf8.ValidString(latin1Result),
		"latin-1 tool result must be valid UTF-8: %q", latin1Result)
	assert.Contains(t, latin1Result, "\ufffd",
		"latin-1 tool result must replace bad bytes with U+FFFD")
	assert.Contains(t, latin1Result, "(1 invalid UTF-8 byte replaced with U+FFFD)",
		"latin-1 tool result must carry the byte-count marker")

	// (4) Layer 3: valid UTF-8 file → verbatim, no marker, no stub.
	utf8Result, ok := findToolResult(utf8Msgs, "call-read-utf8")
	require.True(t, ok, "expected tool result for call-read-utf8 in request 1")
	assert.Contains(t, utf8Result, "héllo · 世界",
		"utf-8 tool result must round-trip multi-byte characters")
	assert.NotContains(t, utf8Result, "invalid UTF-8",
		"valid utf-8 file must not be tagged with a sanitisation marker")
	assert.NotContains(t, utf8Result, "<binary file:",
		"valid utf-8 file must not be tagged as binary")

	// (5) Layer 4: grep_files matches the utf-8 text file but skips
	// the binary blob even though both contain the literal "needle".
	grepResult, ok := findToolResult(grepMsgs, "call-grep")
	require.True(t, ok, "expected tool result for call-grep in request 4")
	assert.Contains(t, grepResult, "utf8.txt", "grep must find the text file")
	assert.NotContains(t, grepResult, "bin.dat",
		"grep must skip binary files even when their bytes contain the pattern")
}

// TestE2ECtrlCDismissesFreeFormPrompt drives the same handler with an
// ask_user_question call that has no options (free-form text input).
// The user types "Ali" into the main inputbox; Ctrl-C must dismiss
// the prompt without submitting the typed text. The third frame
// asserts that the typed text remains in the inputbox for the user
// to edit or send as a regular chat message.
func TestE2ECtrlCDismissesFreeFormPrompt(t *testing.T) {
	args := mustJSON(t, map[string]any{
		"questions": []map[string]any{{
			"question":    "What is your name?",
			"header":      "Name",
			"options":     []map[string]any{},
			"multiSelect": false,
		}},
	})
	h := newPromptHandler(t, promptHandlerOpts{toolArgs: args})
	handlertest.RunHandlerSequence(t, h, frameWidth, frameHeight, []handlertest.SequenceTestCase{
		{
			InputSequence: "hi<enter>",
			Expected: frame(
				"hi",
				"Name: What is your name?",
				blanks(), blanks(), blanks(), blanks(), blanks(),
				"   ┌───────────────────────────────┐    ",
				"   │▐                              │    ",
				"   └───────────────────────────────┘    ",
			),
		},
		{
			InputSequence: "Ali",
			Expected: frame(
				"hi",
				"Name: What is your name?",
				blanks(), blanks(), blanks(), blanks(), blanks(),
				"   ┌───────────────────────────────┐    ",
				"   │Ali▐                           │    ",
				"   └───────────────────────────────┘    ",
			),
		},
		{
			InputSequence: "<c-c>",
			Expected: frame(
				"hi",
				blanks(), blanks(), blanks(), blanks(), blanks(), blanks(),
				"   ┌───────────────────────────────┐    ",
				"   │Ali▐                           │    ",
				"   └───────────────────────────────┘    ",
			),
		},
	})
}

// TestE2ECtrlCDismissesRequiresInputPrompt covers the PromptInputMode
// branch of the dialogue handler. ask_user_question never marks an
// option as RequiresInput, so the fixture installs a synthetic prompt
// with RequiresInput=true before any user input. Enter selects the
// only option and transitions to text-input mode; Ctrl-C must dismiss
// the entire prompt instead of merely leaving text-input mode.
func TestE2ECtrlCDismissesRequiresInputPrompt(t *testing.T) {
	h := newPromptHandler(t, promptHandlerOpts{
		extraPrompt: &dialoguetui.MessageEvent{
			Type:         dialoguetui.MessageEventPrompt,
			PromptTitle:  "Choose",
			PromptHeader: "Choice",
			PromptOptions: []dialoguetui.PromptEventOption{
				{Label: "Other", RequiresInput: true},
			},
			PromptResult: make(chan []string, 1),
		},
	})
	handlertest.RunHandlerSequence(t, h, frameWidth, frameHeight, []handlertest.SequenceTestCase{
		{
			// Enter selects "Other" which has RequiresInput=true,
			// switching the dialogue handler into PromptInputMode.
			InputSequence: "<enter>",
			Expected: frame(
				"▐ype your feedback and press Enter to   ",
				blanks(), blanks(), blanks(), blanks(), blanks(), blanks(),
				"   ┌───────────────────────────────┐    ",
				"   │                               │    ",
				"   └───────────────────────────────┘    ",
			),
		},
		{
			// Ctrl-C in PromptInputMode must dismiss the whole
			// prompt, not just exit text input.
			InputSequence: "<c-c>",
			Expected: frame(
				blanks(), blanks(), blanks(), blanks(), blanks(), blanks(), blanks(),
				"   ┌───────────────────────────────┐    ",
				"   │▐                              │    ",
				"   └───────────────────────────────┘    ",
			),
		},
	})
}
