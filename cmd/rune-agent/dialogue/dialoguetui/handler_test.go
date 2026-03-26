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

package dialoguetui

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

var _ tui.Handler = (*dialogueHandler)(nil)

func typeText(h tui.Handler, text string) {
	for _, ch := range text {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
}

func TestHandlerIntegration(t *testing.T) {
	interrupt := make(chan struct{})
	h, tx, rx := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(20, 9)

	t.Run("interrupt should be called after tx channel send msg", func(t *testing.T) {
		for _, ch := range "Hello assistant!" {
			_, handled := h.Handle(term.Event{Type: term.EventKey, Ch: ch})
			assert.True(t, handled)
		}
		go func() {
			msg := <-rx
			assert.Equal(t, "Hello assistant!", msg.Text)
		}()
		_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
		assert.True(t, handled)

		tx <- MessageEvent{Type: MessageEventText, Text: "Well, hello Sir."}

		<-interrupt

		w := term.NewStringWriter(20, 9)
		h.Draw(w)
		err := w.Flush()
		require.NoError(t, err)

		out := w.String()
		assert.Equal(t, `Hello assistant!    
Well, hello Sir.    
                    
                    
                    
                    
 ┌──────────────┐   
 │              │   
 └──────────────┘   `, out)

		cursor, _, ok := h.Cursor()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{Y: 7, X: 2}, cursor)
	})
}

func TestHandlerCtrlOToggle(t *testing.T) {
	interrupt := make(chan struct{}, 10)
	comp := NewComponent(ComponentConfig{})
	h, tx, _ := Handler(context.Background(), new(sync.Mutex), comp, term.FuncInterrupter(func(context.Context) error {
		interrupt <- struct{}{}
		return nil
	}))
	defer close(tx)
	h.Resize(30, 10)

	// Send reasoning event
	tx <- MessageEvent{Type: MessageEventReasoning, Text: "Deep thought"}
	<-interrupt

	// Ctrl+O should be handled and toggle to collapsed
	_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'o'})
	assert.True(t, handled)

	w := term.NewStringWriter(31, 11)
	h.Draw(w)
	err := w.Flush()
	require.NoError(t, err)
	assert.Contains(t, w.String(), "ctrl-o to expand")
	assert.NotContains(t, w.String(), "Deep thought")

	// Toggle to expanded: reasoning reappears
	_, handled = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'o'})
	assert.True(t, handled)

	err = w.Clear(term.Attributes{})
	require.NoError(t, err)
	h.Draw(w)
	err = w.Flush()
	require.NoError(t, err)
	assert.Contains(t, w.String(), "ctrl-o to collapse")
	assert.Contains(t, w.String(), "Deep thought")
}

func TestHandlerCtrlOToggleCollapsesTools(t *testing.T) {
	interrupt := make(chan struct{}, 10)
	comp := NewComponent(ComponentConfig{})
	h, tx, _ := Handler(context.Background(), new(sync.Mutex), comp, term.FuncInterrupter(func(context.Context) error {
		interrupt <- struct{}{}
		return nil
	}))
	defer close(tx)
	h.Resize(40, 12)

	// Send tool call events
	tx <- MessageEvent{Type: MessageEventToolCall, ToolCallID: "c1", ToolName: "read_file", ToolArgs: `{}`, ToolSummary: "a.go"}
	<-interrupt
	tx <- MessageEvent{Type: MessageEventToolResult, ToolCallID: "c1", ToolName: "read_file", ToolArgs: `{}`, ToolSummary: "a.go", ToolOutput: "file data"}
	<-interrupt

	// Expanded: should show full tool output
	w := term.NewStringWriter(41, 13)
	h.Draw(w)
	_ = w.Flush()
	assert.Contains(t, w.String(), "✓ read_file a.go")
	assert.Contains(t, w.String(), "file data")
	assert.NotContains(t, w.String(), "└─ ✓")

	// Ctrl+O: collapse
	_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'o'})
	assert.True(t, handled)

	_ = w.Clear(term.Attributes{})
	h.Draw(w)
	_ = w.Flush()
	assert.Contains(t, w.String(), "└─ ✓ read_file a.go")
	assert.NotContains(t, w.String(), "file data")

	// Ctrl+O: expand back
	_, handled = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'o'})
	assert.True(t, handled)

	_ = w.Clear(term.Attributes{})
	h.Draw(w)
	_ = w.Flush()
	assert.Contains(t, w.String(), "✓ read_file a.go")
	assert.Contains(t, w.String(), "file data")
}

func TestHandlerErrorEvent(t *testing.T) {
	interrupt := make(chan struct{}, 10)
	h, tx, _ := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(20, 9)

	tx <- MessageEvent{Type: MessageEventError, Text: "max iterations"}
	<-interrupt

	w := term.NewStringWriter(20, 9)
	h.Draw(w)
	err := w.Flush()
	require.NoError(t, err)
	assert.Contains(t, w.String(), "! max iterations")
}

func TestHandlerWarningEvent(t *testing.T) {
	interrupt := make(chan struct{}, 10)
	h, tx, _ := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(50, 9)

	tx <- MessageEvent{Type: MessageEventWarning, Text: "Rate limited. Waiting 5s before retrying (attempt 1/3)."}
	<-interrupt

	w := term.NewStringWriter(50, 9)
	h.Draw(w)
	err := w.Flush()
	require.NoError(t, err)
	out := w.String()
	assert.Contains(t, out, "Rate limited")
	assert.Contains(t, out, "Waiting 5s")
	// Warnings should NOT have the "! " prefix that errors have.
	assert.NotContains(t, out, "! Rate limited")
}

func newPromptHandler(t *testing.T) (tui.Handler, chan<- MessageEvent, chan struct{}) {
	t.Helper()
	interrupt := make(chan struct{}, 10)
	h, tx, _ := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	t.Cleanup(func() { close(tx) })
	h.Resize(40, 12)
	return h, tx, interrupt
}

func TestHandlerPromptRendersSelection(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "Pick a DB",
		PromptHeader:  "Database",
		PromptOptions: []PromptEventOption{{Label: "Postgres"}, {Label: "SQLite"}},
		PromptResult:  resultCh,
	}
	<-interrupt

	w := term.NewStringWriter(40, 12)
	h.Draw(w)
	_ = w.Flush()
	assert.Contains(t, w.String(), "Pick a DB")
	assert.Contains(t, w.String(), "Postgres")
	assert.Contains(t, w.String(), "SQLite")
}

func TestHandlerPromptArrowKeysMoveCursor(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "Choose",
		PromptOptions: []PromptEventOption{{Label: "A"}, {Label: "B"}, {Label: "C"}},
		PromptResult:  resultCh,
	}
	<-interrupt

	// Move down and select
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
	assert.True(t, handled)
	_, handled = h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)

	vals := <-resultCh
	assert.Equal(t, []string{"B"}, vals)
}

func TestHandlerPromptEnterSelectsAndSendsResult(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "Choose",
		PromptOptions: []PromptEventOption{{Label: "First"}, {Label: "Second"}},
		PromptResult:  resultCh,
	}
	<-interrupt

	// Press Enter immediately (selects first option)
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)

	vals := <-resultCh
	assert.Equal(t, []string{"First"}, vals)
}

func TestHandlerPromptEscDismissesSendsNil(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "Choose",
		PromptOptions: []PromptEventOption{{Label: "A"}},
		PromptResult:  resultCh,
	}
	<-interrupt

	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
	assert.True(t, handled)

	vals := <-resultCh
	assert.Nil(t, vals)
}

func TestHandlerPromptAbsorbsRegularKeys(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "Choose",
		PromptOptions: []PromptEventOption{{Label: "A"}},
		PromptResult:  resultCh,
	}
	<-interrupt

	// Regular character should be absorbed (handled=true but no effect)
	_, handled := h.Handle(term.Event{Type: term.EventKey, Ch: 'x'})
	assert.True(t, handled)

	// Prompt should still be active — select to clean up
	_, handled = h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)
	<-resultCh
}

func TestHandlerPromptCursorHidden(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "Choose",
		PromptOptions: []PromptEventOption{{Label: "A"}},
		PromptResult:  resultCh,
	}
	<-interrupt

	_, _, ok := h.Cursor()
	assert.False(t, ok, "cursor should be hidden during active prompt")

	// Clean up
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	<-resultCh
}

func TestHandlerPromptMultiSelectSpaceToggles(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:              MessageEventPrompt,
		PromptTitle:       "Features",
		PromptOptions:     []PromptEventOption{{Label: "Logging"}, {Label: "Metrics"}, {Label: "Tracing"}},
		PromptMultiSelect: true,
		PromptResult:      resultCh,
	}
	<-interrupt

	// Toggle first option
	h.Handle(term.Event{Type: term.EventKey, Ch: ' '})
	// Move down and toggle second
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
	h.Handle(term.Event{Type: term.EventKey, Ch: ' '})
	// Confirm
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	vals := <-resultCh
	assert.Equal(t, []string{"Logging", "Metrics"}, vals)
}

func TestHandlerPromptDismissEvent(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "Choose",
		PromptOptions: []PromptEventOption{{Label: "A"}},
		PromptResult:  resultCh,
	}
	<-interrupt

	// Verify prompt active
	_, _, ok := h.Cursor()
	assert.False(t, ok)

	// Send dismiss event
	tx <- MessageEvent{Type: MessageEventPromptDismiss}
	<-interrupt

	// Prompt should be gone, cursor visible again
	_, _, ok = h.Cursor()
	assert.True(t, ok)

	vals := <-resultCh
	assert.Nil(t, vals)
}

func TestHandlerToolEvents(t *testing.T) {
	interrupt := make(chan struct{}, 10)
	h, tx, _ := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(20, 9)

	tx <- MessageEvent{Type: MessageEventToolCall, ToolCallID: "c1", ToolName: "read_file"}
	<-interrupt

	w := term.NewStringWriter(20, 9)
	h.Draw(w)
	err := w.Flush()
	require.NoError(t, err)
	assert.Equal(t, `⚙ read_file         
                    
                    
                    
                    
                    
 ┌──────────────┐   
 │              │   
 └──────────────┘   `, w.String())

	tx <- MessageEvent{Type: MessageEventToolResult, ToolCallID: "c1", ToolName: "read_file", ToolOutput: "file content", IsError: false}
	<-interrupt

	err = w.Clear(term.Attributes{})
	require.NoError(t, err)
	h.Draw(w)
	err = w.Flush()
	require.NoError(t, err)
	assert.Equal(t, `✓ read_file         
file content        
                    
                    
                    
                    
 ┌──────────────┐   
 │              │   
 └──────────────┘   `, w.String())
}

func TestIsCommand(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"/help", true},
		{"/model default", true},
		{"/ space", false},
		{"/", false},
		{"hello", false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, isCommand(tc.input))
		})
	}
}

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantArgs []string
	}{
		{"/help", "help", []string{}},
		{"/model default", "model", []string{"default"}},
		{"/tools a b", "tools", []string{"a", "b"}},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			name, args := parseCommand(tc.input)
			assert.Equal(t, tc.wantName, name)
			assert.Equal(t, tc.wantArgs, args)
		})
	}
}

// mockCommandHandler is a test stub for CommandHandler.
type mockCommandHandler struct {
	handleFunc   func(ctx context.Context, name string, args []string) (CommandResult, error)
	completeFunc func(ctx context.Context, name string, args []string) (iterator.Iterator[string], error)
}

func (m *mockCommandHandler) HandleCommand(ctx context.Context, name string, args []string) (CommandResult, error) {
	return m.handleFunc(ctx, name, args)
}

func (m *mockCommandHandler) Complete(ctx context.Context, name string, args []string) (iterator.Iterator[string], error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, name, args)
	}
	return iterator.Empty[string](), nil
}

func TestHandlerCommandIntercepted(t *testing.T) {
	interrupt := make(chan struct{}, 20)
	mock := &mockCommandHandler{
		handleFunc: func(_ context.Context, name string, _ []string) (CommandResult, error) {
			r := component.NewResponsiveString("output-"+name, component.StringResponsiveConfig{})
			return CommandResult{Display: iterator.FromSlice([]component.Responsive{r})}, nil
		},
	}
	h, tx, rx := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}),
		WithCommands(mock),
	)
	defer close(tx)
	h.Resize(30, 9)

	// Type /help and press enter.
	for _, ch := range "/help" {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)

	// rx should NOT receive the message (it was intercepted).
	select {
	case msg := <-rx:
		t.Fatalf("expected no message on rx, got %q", msg.Text)
	case <-time.After(50 * time.Millisecond):
	}

	// Wait for the goroutine to drain and interrupt.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-interrupt:
		case <-deadline:
			t.Fatal("timed out waiting for command output")
		}
		w := term.NewStringWriter(30, 9)
		h.Draw(w)
		_ = w.Flush()
		if assert.ObjectsAreEqual(true, containsStr(w.String(), "output-help")) {
			break
		}
	}
}

func TestHandlerNonCommandGoesToLLM(t *testing.T) {
	interrupt := make(chan struct{}, 10)
	mock := &mockCommandHandler{
		handleFunc: func(context.Context, string, []string) (CommandResult, error) {
			t.Fatal("HandleCommand should not be called")
			return CommandResult{}, nil
		},
	}
	h, tx, rx := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}),
		WithCommands(mock),
	)
	defer close(tx)
	h.Resize(20, 9)

	for _, ch := range "hello" {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	go func() {
		msg := <-rx
		assert.Equal(t, "hello", msg.Text)
	}()
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)
}

func TestHandlerBusyQueuesUserMessageUntilIdle(t *testing.T) {
	mu := new(sync.Mutex)
	comp := NewComponent(ComponentConfig{})
	interrupt := make(chan struct{}, 10)
	h, tx, rx := Handler(context.Background(), mu, comp,
		term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(20, 9)

	tx <- MessageEvent{Type: MessageEventBusy, Busy: true}
	<-interrupt

	typeText(h, "hello")
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)

	select {
	case msg := <-rx:
		t.Fatalf("expected queued message to wait until idle, got %q", msg.Text)
	case <-time.After(50 * time.Millisecond):
	}

	w := term.NewStringWriter(20, 9)
	h.Draw(w)
	_ = w.Flush()
	assert.Contains(t, w.String(), "hello")
	assert.Contains(t, w.String(), "⏳")

	go func() {
		tx <- MessageEvent{Type: MessageEventBusy, Busy: false}
	}()

	select {
	case msg := <-rx:
		assert.Equal(t, "hello", msg.Text)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for queued message to drain")
	}
	<-interrupt

	w = term.NewStringWriter(20, 9)
	h.Draw(w)
	_ = w.Flush()
	assert.NotContains(t, w.String(), "⏳ hello")
	assert.Contains(t, w.String(), "hello")

	mu.Lock()
	assert.Equal(t, 0, comp.QueueLen())
	mu.Unlock()
}

func TestHandlerBusyQueueDrainsFIFOAcrossTurns(t *testing.T) {
	mu := new(sync.Mutex)
	comp := NewComponent(ComponentConfig{})
	interrupt := make(chan struct{}, 10)
	h, tx, rx := Handler(context.Background(), mu, comp,
		term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(30, 10)

	tx <- MessageEvent{Type: MessageEventBusy, Busy: true}
	<-interrupt

	typeText(h, "first")
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	typeText(h, "second")
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	mu.Lock()
	assert.Equal(t, 2, comp.QueueLen())
	mu.Unlock()

	go func() {
		tx <- MessageEvent{Type: MessageEventBusy, Busy: false}
	}()

	select {
	case msg := <-rx:
		assert.Equal(t, "first", msg.Text)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first queued message")
	}
	<-interrupt

	tx <- MessageEvent{Type: MessageEventBusy, Busy: true}
	<-interrupt

	go func() {
		tx <- MessageEvent{Type: MessageEventBusy, Busy: false}
	}()

	select {
	case msg := <-rx:
		assert.Equal(t, "second", msg.Text)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second queued message")
	}
	<-interrupt

	mu.Lock()
	assert.Equal(t, 0, comp.QueueLen())
	mu.Unlock()
}

func TestHandlerArrowUpRecallsNewestQueuedMessage(t *testing.T) {
	mu := new(sync.Mutex)
	comp := NewComponent(ComponentConfig{})
	interrupt := make(chan struct{}, 10)
	h, tx, _ := Handler(context.Background(), mu, comp,
		term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(30, 10)

	tx <- MessageEvent{Type: MessageEventBusy, Busy: true}
	<-interrupt

	typeText(h, "first")
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	typeText(h, "second")
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowUp})
	assert.True(t, handled)

	mu.Lock()
	assert.Equal(t, "second", comp.Input().Text())
	assert.Equal(t, 1, comp.QueueLen())
	mu.Unlock()
}

func TestHandlerArrowUpAfterDiscardRecallsPreviousQueuedMessage(t *testing.T) {
	mu := new(sync.Mutex)
	comp := NewComponent(ComponentConfig{})
	interrupt := make(chan struct{}, 10)
	h, tx, _ := Handler(context.Background(), mu, comp,
		term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(30, 10)

	tx <- MessageEvent{Type: MessageEventBusy, Busy: true}
	<-interrupt

	typeText(h, "first")
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	typeText(h, "second")
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowUp})
	assert.True(t, handled)
	_, handled = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'c'})
	assert.True(t, handled)
	_, handled = h.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowUp})
	assert.True(t, handled)

	mu.Lock()
	assert.Equal(t, "first", comp.Input().Text())
	assert.Equal(t, 0, comp.QueueLen())
	mu.Unlock()
}

func TestHandlerCommandError(t *testing.T) {
	interrupt := make(chan struct{}, 20)
	mock := &mockCommandHandler{
		handleFunc: func(context.Context, string, []string) (CommandResult, error) {
			return CommandResult{}, errors.New("bad command")
		},
	}
	h, tx, _ := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}),
		WithCommands(mock),
	)
	defer close(tx)
	h.Resize(30, 9)

	for _, ch := range "/bad" {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	deadline := time.After(2 * time.Second)
	for {
		select {
		case <-interrupt:
		case <-deadline:
			t.Fatal("timed out waiting for error")
		}
		w := term.NewStringWriter(30, 9)
		h.Draw(w)
		_ = w.Flush()
		if containsStr(w.String(), "! bad command") {
			return
		}
	}
}

func TestHandlerCommandExit(t *testing.T) {
	interrupt := make(chan struct{}, 10)
	closeCalled := make(chan struct{})
	mock := &mockCommandHandler{
		handleFunc: func(context.Context, string, []string) (CommandResult, error) {
			return CommandResult{Exit: true}, nil
		},
	}
	h, tx, _ := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}),
		WithCommands(mock),
		WithCloseFunc(func() { close(closeCalled) }),
	)
	defer close(tx)
	h.Resize(20, 9)

	for _, ch := range "/exit" {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	select {
	case <-closeCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("closeFn was not called")
	}
}

func TestHandlerCommandUserMessage(t *testing.T) {
	mock := &mockCommandHandler{
		handleFunc: func(_ context.Context, name string, args []string) (CommandResult, error) {
			if name == "commit" {
				return CommandResult{
					UserMessage: "<skill_content>" + strings.Join(args, " ") + "</skill_content>",
				}, nil
			}
			return CommandResult{}, errors.New("unknown")
		},
	}
	h, tx, rx := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			return nil
		}),
		WithCommands(mock),
	)
	defer close(tx)
	h.Resize(40, 9)

	// Type /commit -m "fix" and press enter.
	for _, ch := range `/commit -m "fix"` {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	go func() {
		msg := <-rx
		assert.Equal(t, `<skill_content>-m "fix"</skill_content>`, msg.Text)
	}()
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)
}

func TestHandlerCommandUserMessageQueuedWhileBusy(t *testing.T) {
	interrupt := make(chan struct{}, 10)
	mock := &mockCommandHandler{
		handleFunc: func(_ context.Context, name string, _ []string) (CommandResult, error) {
			return CommandResult{
				UserMessage: "<skill>" + name + "</skill>",
				SkillName:   name,
			}, nil
		},
	}
	h, tx, rx := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}),
		WithCommands(mock),
	)
	defer close(tx)
	h.Resize(40, 9)

	tx <- MessageEvent{Type: MessageEventBusy, Busy: true}
	<-interrupt

	typeText(h, "/review")
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)

	select {
	case msg := <-rx:
		t.Fatalf("expected command user message to queue while busy, got %q", msg.Text)
	case <-time.After(50 * time.Millisecond):
	}

	go func() {
		tx <- MessageEvent{Type: MessageEventBusy, Busy: false}
	}()

	select {
	case msg := <-rx:
		assert.Equal(t, "<skill>review</skill>", msg.Text)
		assert.Equal(t, "review", msg.SkillName)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for queued command message")
	}
	<-interrupt
}

func TestHandlerCommandUserMessageNoArgs(t *testing.T) {
	mock := &mockCommandHandler{
		handleFunc: func(_ context.Context, name string, _ []string) (CommandResult, error) {
			return CommandResult{UserMessage: "<skill>" + name + "</skill>"}, nil
		},
	}
	h, tx, rx := Handler(context.Background(), new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
			return nil
		}),
		WithCommands(mock),
	)
	defer close(tx)
	h.Resize(40, 9)

	for _, ch := range "/review" {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	go func() {
		msg := <-rx
		assert.Equal(t, "<skill>review</skill>", msg.Text)
	}()
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)
}

// TestHandlerPromptHintRestoredAfterSelect verifies that when a prompt
// event arrives through the handler's tx channel, the receive-message hint
// is hidden while the prompt is active, and restored after the user selects.
func TestHandlerPromptHintRestoredAfterSelect(t *testing.T) {
	mu := new(sync.Mutex)
	comp := NewComponent(ComponentConfig{})
	interrupt := make(chan struct{}, 10)
	h, tx, _ := Handler(context.Background(), mu, comp,
		term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(20, 10)

	// Add hint under lock (simulates syncComponent.addStatusHint).
	mu.Lock()
	comp.AddReceiveMessageHint(component.NewString("$"), component.SpanConfig{
		PadHorizontal:    -1,
		ContentAlignment: component.AlignmentLeft,
	})
	mu.Unlock()

	w := term.NewStringWriter(21, 11)

	// Hint should be visible.
	h.Draw(w)
	_ = w.Flush()
	hintVisible := "$                    \n" +
		"                     \n" +
		"                     \n" +
		"                     \n" +
		"                     \n" +
		"                     \n" +
		"                     \n" +
		" ┌──────────────┐    \n" +
		" │              │    \n" +
		" └──────────────┘    \n" +
		"                     "
	assert.Equal(t, hintVisible, w.String(), "hint visible before prompt")

	// Send prompt — hint should be hidden.
	resultCh := make(chan []string, 1)
	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "OK?",
		PromptOptions: []PromptEventOption{{Label: "Yes"}, {Label: "No"}},
		PromptResult:  resultCh,
	}
	<-interrupt

	_ = w.Clear(term.Attributes{})
	h.Draw(w)
	_ = w.Flush()
	assert.False(t, containsStr(w.String(), "$"), "hint hidden during prompt")
	assert.True(t, containsStr(w.String(), "OK?"), "prompt visible")

	// Select first option — hint should be restored.
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)
	vals := <-resultCh
	assert.Equal(t, []string{"Yes"}, vals)

	_ = w.Clear(term.Attributes{})
	h.Draw(w)
	_ = w.Flush()
	assert.Equal(t, hintVisible, w.String(), "hint restored after prompt select")
}

// TestHandlerPromptHintRestoredAfterDismiss verifies that the hint is
// restored when the prompt is dismissed via Esc through the handler.
func TestHandlerPromptHintRestoredAfterDismiss(t *testing.T) {
	mu := new(sync.Mutex)
	comp := NewComponent(ComponentConfig{})
	interrupt := make(chan struct{}, 10)
	h, tx, _ := Handler(context.Background(), mu, comp,
		term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(20, 10)

	// Add hint.
	mu.Lock()
	comp.AddReceiveMessageHint(component.NewString("$"), component.SpanConfig{
		PadHorizontal:    -1,
		ContentAlignment: component.AlignmentLeft,
	})
	mu.Unlock()

	// Send prompt.
	resultCh := make(chan []string, 1)
	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "OK?",
		PromptOptions: []PromptEventOption{{Label: "Yes"}, {Label: "No"}},
		PromptResult:  resultCh,
	}
	<-interrupt

	// Dismiss via Esc.
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
	assert.True(t, handled)
	vals := <-resultCh
	assert.Nil(t, vals)

	// Hint should be restored.
	w := term.NewStringWriter(21, 11)
	h.Draw(w)
	_ = w.Flush()
	assert.Equal(t, "$                    \n"+
		"                     \n"+
		"                     \n"+
		"                     \n"+
		"                     \n"+
		"                     \n"+
		"                     \n"+
		" ┌──────────────┐    \n"+
		" │              │    \n"+
		" └──────────────┘    \n"+
		"                     ", w.String(), "hint restored after Esc dismiss")
}

// TestHandlerFreeFormPromptRendersQuestion verifies that when a prompt
// with empty options is sent, the question text appears in the messages area
// and the cursor remains visible (unlike selection prompts which hide it).
func TestHandlerFreeFormPromptRendersQuestion(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "What is your name?",
		PromptHeader:  "Name",
		PromptOptions: nil, // no options → free-form
		PromptResult:  resultCh,
	}
	<-interrupt

	w := term.NewStringWriter(40, 12)
	h.Draw(w)
	_ = w.Flush()
	assert.Contains(t, w.String(), "Name: What is your name?")

	// Cursor should be visible (pointing to the main inputbox).
	_, _, ok := h.Cursor()
	assert.True(t, ok, "cursor should be visible during free-form prompt")

	// Clean up — dismiss the prompt.
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
	<-resultCh
}

// TestHandlerFreeFormPromptEnterSubmitsText verifies that typing text
// and pressing Enter during a free-form prompt sends the text on the
// result channel.
func TestHandlerFreeFormPromptEnterSubmitsText(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "What is your name?",
		PromptOptions: nil,
		PromptResult:  resultCh,
	}
	<-interrupt

	// Type "Alice"
	for _, ch := range "Alice" {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.True(t, handled)

	vals := <-resultCh
	assert.Equal(t, []string{"Alice"}, vals)
}

// TestHandlerFreeFormPromptEscDismisses verifies that pressing Esc
// during a free-form prompt sends nil on the result channel.
func TestHandlerFreeFormPromptEscDismisses(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "What is your name?",
		PromptOptions: nil,
		PromptResult:  resultCh,
	}
	<-interrupt

	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
	assert.True(t, handled)

	vals := <-resultCh
	assert.Nil(t, vals)
}

// TestHandlerFreeFormPromptEmptyEnterIgnored verifies that pressing Enter
// with no text typed does not submit or dismiss the prompt.
func TestHandlerFreeFormPromptEmptyEnterIgnored(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "What is your name?",
		PromptOptions: nil,
		PromptResult:  resultCh,
	}
	<-interrupt

	// Press Enter with empty input — should NOT submit.
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.False(t, handled, "empty Enter should not be handled")

	// Prompt should still be active — type and submit to clean up.
	for _, ch := range "Bob" {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	vals := <-resultCh
	assert.Equal(t, []string{"Bob"}, vals)
}

// TestHandlerFreeFormPromptDismissEvent verifies that a
// MessageEventPromptDismiss event correctly dismisses a free-form prompt.
func TestHandlerFreeFormPromptDismissEvent(t *testing.T) {
	h, tx, interrupt := newPromptHandler(t)
	resultCh := make(chan []string, 1)

	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "What is your name?",
		PromptOptions: nil,
		PromptResult:  resultCh,
	}
	<-interrupt

	// Verify cursor is visible (free-form prompt uses main inputbox).
	_, _, ok := h.Cursor()
	assert.True(t, ok)

	// Send dismiss event.
	tx <- MessageEvent{Type: MessageEventPromptDismiss}
	<-interrupt

	// Prompt should be gone.
	vals := <-resultCh
	assert.Nil(t, vals)

	// Normal input should work after dismiss — cursor visible, typing works.
	_, _, ok = h.Cursor()
	assert.True(t, ok, "cursor visible after dismiss")
}

// TestHandlerPromptDismissEventRestoresHint verifies that the hint is
// restored when a MessageEventPromptDismiss event dismisses the prompt
// (e.g. context cancellation path).
func TestHandlerPromptDismissEventRestoresHint(t *testing.T) {
	mu := new(sync.Mutex)
	comp := NewComponent(ComponentConfig{})
	interrupt := make(chan struct{}, 10)
	h, tx, _ := Handler(context.Background(), mu, comp,
		term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(20, 10)

	// Add hint.
	mu.Lock()
	comp.AddReceiveMessageHint(component.NewString("$"), component.SpanConfig{
		PadHorizontal:    -1,
		ContentAlignment: component.AlignmentLeft,
	})
	mu.Unlock()

	// Send prompt.
	resultCh := make(chan []string, 1)
	tx <- MessageEvent{
		Type:          MessageEventPrompt,
		PromptTitle:   "OK?",
		PromptOptions: []PromptEventOption{{Label: "Yes"}, {Label: "No"}},
		PromptResult:  resultCh,
	}
	<-interrupt

	// Dismiss via event (context cancellation path).
	tx <- MessageEvent{Type: MessageEventPromptDismiss}
	<-interrupt
	vals := <-resultCh
	assert.Nil(t, vals)

	// Hint should be restored.
	w := term.NewStringWriter(21, 11)
	h.Draw(w)
	_ = w.Flush()
	assert.Equal(t, "$                    \n"+
		"                     \n"+
		"                     \n"+
		"                     \n"+
		"                     \n"+
		"                     \n"+
		"                     \n"+
		" ┌──────────────┐    \n"+
		" │              │    \n"+
		" └──────────────┘    \n"+
		"                     ", w.String(), "hint restored after dismiss event")
}

func containsStr(haystack, needle string) bool {
	return len(haystack) >= len(needle) && searchStr(haystack, needle)
}

func searchStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
