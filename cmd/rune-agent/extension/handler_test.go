// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

package extension

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

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
