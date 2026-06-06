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
	"log/slog"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/debug"
	tterm "unstable.build/go-tui/term"
)

// SubmitMessage is a user message sent through the tx channel.
type SubmitMessage struct {
	Text      string
	SkillName string // non-empty when the message was triggered by a slash-command skill
}

// CommandResult is the outcome of a handled /command.
type CommandResult struct {
	// Display is the output to render in the chat. Nil means no display output.
	Display iterator.Iterator[component.Responsive]
	// UserMessage, when non-empty, is sent through the tx channel as a user
	// message to the LLM instead of being displayed.
	UserMessage string
	// SkillName, when non-empty, identifies the skill that generated UserMessage.
	SkillName string
	// Exit, when true, signals the handler to close the chat.
	Exit bool
}

// CommandHandler handles /commands typed in the chat input.
// Any repl.CommandHandler can be adapted to this interface.
type CommandHandler interface {
	HandleCommand(ctx context.Context, name string, args []string) (CommandResult, error)
	Complete(ctx context.Context, name string, args []string) (iterator.Iterator[string], error)
}

// HandlerOption configures optional behavior of Handler.
type HandlerOption func(*dialogueHandler)

// WithCommands sets a CommandHandler for intercepting /commands.
func WithCommands(h CommandHandler) HandlerOption {
	return func(dh *dialogueHandler) { dh.commands = h }
}

// WithCloseFunc sets a function called when a command returns
// CommandResult.Exit == true.
func WithCloseFunc(fn func()) HandlerOption {
	return func(dh *dialogueHandler) { dh.closeFn = fn }
}

// Handler wraps a dialogue.Component and provides a simple-to-use
// tui.Handler which sends input messages via rx and can
// receive messages into the dialogue history via tx.
//
// Closing the tx channel effecively closes the returned handler.
//
// locker is used to synchronize access to c.
func Handler(
	ctx context.Context,
	locker sync.Locker, c *Component,
	interrupter term.Interrupter,
	opts ...HandlerOption,
) (h tui.Handler, tx chan<- MessageEvent, rx <-chan SubmitMessage) {
	c.interrupter = interrupter
	c.mu = locker
	ch1 := make(chan SubmitMessage)
	ch2 := make(chan MessageEvent)
	sh := &dialogueHandler{
		interrupter:  interrupter,
		comp:         c,
		rx:           ch2,
		tx:           ch1,
		mu:           locker,
		ctx:          ctx,
		inputFocused: true,
	}
	for _, o := range opts {
		o(sh)
	}
	sh.mouseDelegate = newMouseDelegate(&sh.grid, &c.messages)
	mouse := mouse.New(sh.mouseDelegate)
	sh.mouse = mouse
	go debug.CapturePanicReport(sh.consumeIncoming)

	return sh, ch2, ch1
}

type dialogueHandler struct {
	grid          tterm.SelectionWriter
	mouseDelegate *mouseDelegate
	comp          *Component
	interrupter   term.Interrupter
	mouse         *mouse.Mouse
	tx            chan SubmitMessage // user messages
	rx            chan MessageEvent  // assistant messages
	mu            sync.Locker
	commands      CommandHandler
	closeFn       func()
	ctx           context.Context
	inputFocused  bool // true when last mouse interaction was in the input area

	// busy is true while an agent completion is active. When busy,
	// follow-up user messages are queued instead of being sent directly.
	busy bool

	// queue holds follow-up messages submitted while the agent is busy.
	// Messages are injected FIFO when the handler becomes idle.
	queue []SubmitMessage

	// history holds the text of previously submitted messages (sent,
	// queued, and command lines), oldest first. ArrowUp/ArrowDown walk
	// it to recall earlier input once the compose editor's cursor is at
	// the top/bottom edge and the queue is exhausted.
	history []string
	// historyIdx is the cursor into history for recall. It equals
	// len(history) when not actively browsing; ArrowUp decrements it and
	// ArrowDown increments it.
	historyIdx int
}

func (h *dialogueHandler) publishInterrupt(ctx context.Context) {
	err := h.interrupter.Interrupt(ctx)
	if err != nil {
		slog.Error("publish interrupt",
			"struct", "dialogue.handler",
			"error", err,
		)
	}
}

func (s *dialogueHandler) Draw(w term.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.grid.Clear()
	s.grid.SetContext(w.Context())
	s.comp.Draw(&s.grid)
	s.mouseDelegate.offset = s.comp.MessagesPosition()

	var sel *tterm.SelRange
	if s.mouseDelegate.sel.Active {
		sel = &tterm.SelRange{
			Start:  term.CoordinatesSum(s.mouseDelegate.sel.Start, s.mouseDelegate.offset),
			End:    term.CoordinatesSum(s.mouseDelegate.sel.End, s.mouseDelegate.offset),
			Active: true,
		}
	}
	s.grid.Dump(w, sel)
}

func (s *dialogueHandler) Resize(width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.grid.Resize(width, height)
	s.comp.Resize(width, height)
}

func (s *dialogueHandler) Handle(ev term.Event) (exit, handled bool) {
	switch ev.Mod {
	case 0, term.ModCtrl, term.ModAlt, term.ModShift, term.ModCtrlAlt:
	default:
		return
	}
	if ev.Type == term.EventMouse {
		pos := s.comp.InputPosition()
		if ev.MouseY >= pos.Y {
			if !s.inputFocused {
				s.inputFocused = true
				s.mouseDelegate.ClearSelection()
			}
			ev.MouseY -= pos.Y
			ev.MouseX -= pos.X
			return s.comp.Input().Handle(ev)
		}
		if s.inputFocused {
			s.inputFocused = false
		}
		pos = s.comp.MessagesPosition()
		ev.MouseY -= pos.Y
		ev.MouseX -= pos.X
		return s.mouse.Handle(ev)
	}

	if ev.Type != term.EventKey {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.comp.PromptInputMode() {
		// Text input mode: route keys to the prompt inputbox.
		handled = true
		switch ev.Key {
		case term.KeyEnter:
			ch, label, text := s.comp.PreparePromptInputSubmit()
			s.mu.Unlock()
			if ch != nil {
				ch <- []string{label, text}
			}
			s.mu.Lock()
		case term.KeyEsc:
			s.comp.CancelPromptInput()
		default:
			if ev.Mod == term.ModCtrl && ev.Ch == 'c' {
				// Ctrl-C dismisses the entire prompt while in text input mode.
				ch := s.comp.PreparePromptDismiss()
				s.mu.Unlock()
				if ch != nil {
					ch <- nil
				}
				s.mu.Lock()
				return
			}
			if ib := s.comp.PromptInput(); ib != nil {
				ib.Handle(ev)
			}
		}
		return
	} else if s.comp.HasActivePrompt() {
		handled = true
		switch ev.Key {
		case term.KeyArrowUp:
			s.comp.PromptMoveUp()
		case term.KeyArrowDown:
			s.comp.PromptMoveDown()
		case term.KeyEnter:
			// If the selected option requires input, transition to text input mode.
			if s.comp.StartPromptInput() {
				// Switched to input mode; wait for user to type.
			} else {
				ch, vals := s.comp.PreparePromptSelect()
				s.mu.Unlock()
				if ch != nil {
					ch <- vals
				}
				s.mu.Lock()
			}
		case term.KeyEsc:
			ch := s.comp.PreparePromptDismiss()
			s.mu.Unlock()
			if ch != nil {
				ch <- nil
			}
			s.mu.Lock()
		default:
			if ev.Mod == term.ModCtrl && ev.Ch == 'c' {
				// Ctrl-C dismisses the active selection prompt.
				ch := s.comp.PreparePromptDismiss()
				s.mu.Unlock()
				if ch != nil {
					ch <- nil
				}
				s.mu.Lock()
				return
			}
			if ev.Ch == ' ' {
				s.comp.PromptToggle()
			}
			// absorb all other keys
		}
		return
	} else if s.comp.HasFreeInputPrompt() {
		// Free-form prompt: the user types into the main inputbox.
		// Enter submits the text; Esc dismisses.
		switch ev.Key {
		case term.KeyEnter:
			ch, text := s.comp.PrepareFreeInputSubmit()
			if ch != nil {
				handled = true
				s.mu.Unlock()
				ch <- []string{text}
				s.mu.Lock()
			}
			// If text was empty, do nothing (don't submit empty).
		case term.KeyEsc:
			handled = true
			ch := s.comp.PreparePromptDismiss()
			s.mu.Unlock()
			if ch != nil {
				ch <- nil
			}
			s.mu.Lock()
		default:
			if ev.Mod == term.ModCtrl && ev.Ch == 'c' {
				// Ctrl-C dismisses the active free-form prompt without
				// submitting any typed text.
				handled = true
				ch := s.comp.PreparePromptDismiss()
				s.mu.Unlock()
				if ch != nil {
					ch <- nil
				}
				s.mu.Lock()
				return
			}
			// Route all other keys to the main inputbox.
			_, handled = s.comp.Input().Handle(ev)
		}
		return
	}

	switch {
	case ev.Key == term.KeyEnter && (ev.Mod == 0 || ev.Mod == term.ModShift):
		// The compose input decides whether this Enter submits or just
		// inserts a newline. When it inserts, the event is already
		// consumed by the input, so we must not submit or re-forward it.
		handled = true
		if !s.comp.Input().EnterSubmits(ev) {
			return
		}
		text := s.comp.Input().Text()
		if s.commands != nil && isCommand(text) {
			item, ok := s.comp.InputSubmit()
			if ok {
				s.appendHistory(item)
				name, args := parseCommand(item)
				go debug.CapturePanicReport(func() {
					s.executeCommand(name, args)
				})
			}
			return
		}
		if len(text) == 0 {
			return
		}
		s.comp.Input().Clear()
		s.submitMessage(SubmitMessage{Text: text}, text, true)
		return
	case ev.Mod == term.ModCtrl && ev.Ch == 'c':
		// Ctrl-C clears the compose input. The editor backend consumes
		// Ctrl-C itself, so this must run before the event is forwarded
		// to the input handler below.
		handled = true
		s.comp.Input().Clear()
		return
	}

	if !handled {
		_, handled = s.comp.Input().Handle(ev)
	}

	if handled {
		return
	}

	// Up/down navigation is delegated to the compose editor first; only
	// when it leaves the event unhandled (cursor at the top/bottom edge)
	// do we recall queued/history messages, then fall back to scrolling
	// the messages view.
	switch ev.Mod {
	case 0:
		switch ev.Key {
		case term.KeyArrowDown:
			if s.recallDown() {
				handled = true
				return
			}
			handled = s.comp.SeekDown()
			return
		case term.KeyArrowUp:
			if s.recallUp() {
				handled = true
				return
			}
			handled = s.comp.SeekUp()
			return
		}
	case term.ModCtrl:
		switch ev.Ch {
		case 'k':
			if s.recallUp() {
				handled = true
				return
			}
			handled = s.comp.SeekUp()
		case 'j':
			if s.recallDown() {
				handled = true
				return
			}
			handled = s.comp.SeekDown()
		case 'o':
			handled = true
			s.comp.ToggleContracted()
		}
	}

	return
}

func (s *dialogueHandler) Selection() (string, bool) {
	if s.inputFocused {
		return s.comp.Input().Selection()
	}
	return s.mouseDelegate.Selection()
}

func (s *dialogueHandler) Cursor() (
	cursor term.Coordinates, style term.CursorStyle, ok bool,
) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.comp.PromptInputMode() {
		// Show cursor inside the prompt input box.
		cursor, style, ok = s.comp.PromptInputCursor()
		if !ok {
			return
		}
		cursor = term.CoordinatesSum(cursor, s.comp.MessagesPosition())
		return
	} else if s.comp.HasActivePrompt() {
		return cursor, style, false
	}
	cursor, style, ok = s.comp.Cursor()
	if !ok {
		return
	}
	cursor = term.CoordinatesSum(cursor, s.comp.InputPosition())
	return
}

func (s *dialogueHandler) consumeIncoming() {
	ctx := context.Background()
	for ev := range s.rx {
		s.mu.Lock()
		switch ev.Type {
		case MessageEventReasoning:
			s.comp.AddReasoningChunk(ev.Text)
		case MessageEventText:
			s.comp.AddReceiveMessageChunk(ev.Text)
		case MessageEventBreak:
			s.comp.AddReceiveMessageBreak()
		case MessageEventToolCall:
			if IsTaskTool(ev.ToolName) {
				break
			}
			if ev.ParentToolCallID != "" {
				s.comp.AddChildToolCall(ev.ParentToolCallID, ev.ToolCallID, ev.ToolName, ev.ToolArgs, ev.ToolSummary)
			} else {
				s.comp.AddToolCall(ev.ToolCallID, ev.ToolName, ev.ToolArgs, ev.ToolSummary)
			}
			if !ev.ToolStartTime.IsZero() {
				s.comp.SetToolStartTime(ev.ToolCallID, ev.ToolStartTime)
			}
		case MessageEventToolResult:
			if IsTaskTool(ev.ToolName) {
				break
			}
			if ev.ToolDuration > 0 {
				s.comp.SetToolDuration(ev.ToolCallID, ev.ToolDuration)
			}
			if ev.ParentToolCallID != "" {
				s.comp.CompleteChildToolCall(ev.ParentToolCallID, ev.ToolCallID, ev.ToolName, ev.ToolArgs, ev.ToolSummary, ev.ToolOutput, ev.IsError)
			} else {
				s.comp.CompleteToolCall(ev.ToolCallID, ev.ToolName, ev.ToolArgs, ev.ToolSummary, ev.ToolOutput, ev.IsError)
			}
		case MessageEventChildResult:
			s.comp.AddChildResult(ev.ParentToolCallID, ev.ToolOutput, ev.IsError)
		case MessageEventError:
			s.comp.AddErrorMessage(ev.Text)
		case MessageEventWarning:
			s.comp.AddWarningMessage(ev.Text)
		case MessageEventToolsDropped:
			s.comp.MarkToolsDropped(ev.DroppedToolCallIDs)
		case MessageEventMemoryRecall:
			s.comp.AddMemoryRecall(ev.Memories, ev.MemoryDuration)
		case MessageEventPrompt:
			oldCh := s.comp.AddPrompt(ev.PromptTitle, ev.PromptHeader, ev.PromptBody,
				ev.PromptOptions, ev.PromptMultiSelect, ev.PromptResult)
			if oldCh != nil {
				s.mu.Unlock()
				oldCh <- nil
				s.mu.Lock()
			}
		case MessageEventTaskProgress:
			s.comp.UpdateTaskProgress(ev.TaskProgress)
		case MessageEventPromptDismiss:
			ch := s.comp.PreparePromptDismiss()
			if ch != nil {
				s.mu.Unlock()
				ch <- nil
				s.mu.Lock()
			}
		case MessageEventBusy:
			s.busy = ev.Busy
			if !s.busy {
				s.drainQueue()
			}
		}
		s.mu.Unlock()
		s.publishInterrupt(ctx)
	}
}

func isCommand(text string) bool {
	return len(text) >= 2 && text[0] == '/' && text[1] != ' '
}

func parseCommand(text string) (string, []string) {
	parts := strings.Fields(text[1:])
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

func (s *dialogueHandler) executeCommand(name string, args []string) {
	result, err := s.commands.HandleCommand(s.ctx, name, args)
	defer s.publishInterrupt(s.ctx)
	if err != nil {
		s.mu.Lock()
		s.comp.AddErrorMessage(err.Error())
		s.mu.Unlock()
		return
	}
	if result.Exit {
		if s.closeFn != nil {
			s.closeFn()
		}
		return
	}
	if result.UserMessage != "" {
		s.mu.Lock()
		s.submitMessage(
			SubmitMessage{Text: result.UserMessage, SkillName: result.SkillName},
			"/"+name,
			false,
		)
		s.mu.Unlock()
		return
	}
	if result.Display != nil {
		s.comp.AddCommand(s.ctx, result.Display)
	}
}

// submitMessage either queues a message while busy or forwards it immediately.
// Must be called with s.mu held. If displayNow is true and the handler is idle,
// the message is rendered as a sent message before being forwarded. queuedLabel is
// used for the visible queued indicator when the message is deferred.
func (s *dialogueHandler) submitMessage(msg SubmitMessage, queuedLabel string, displayNow bool) {
	s.appendHistory(msg.Text)
	if s.busy {
		s.queue = append(s.queue, msg)
		s.comp.AddQueuedMessage(queuedLabel)
		return
	}
	if displayNow {
		s.comp.AddSendMessage(msg.Text)
	}
	s.mu.Unlock()
	s.tx <- msg
	s.mu.Lock()
}

// appendHistory records text as the most recent recall entry and resets
// the recall cursor so the next ArrowUp starts from the newest entry.
// Consecutive duplicates are collapsed to avoid stuttering recall.
func (s *dialogueHandler) appendHistory(text string) {
	if text == "" {
		return
	}
	if n := len(s.history); n == 0 || s.history[n-1] != text {
		s.history = append(s.history, text)
	}
	s.historyIdx = len(s.history)
}

// recallUp is the editor-edge fallback for upward navigation: it first
// pops the newest queued message into the compose input, then walks back
// through the submit history. It reports whether anything was recalled.
func (s *dialogueHandler) recallUp() bool {
	if s.comp.Input().Text() == "" && len(s.queue) > 0 {
		msg := s.queue[len(s.queue)-1]
		s.queue = s.queue[:len(s.queue)-1]
		s.comp.RemoveLastQueuedMessage()
		s.comp.Input().SetText(msg.Text)
		return true
	}
	if s.historyIdx <= 0 {
		return false
	}
	s.historyIdx--
	s.comp.Input().SetText(s.history[s.historyIdx])
	return true
}

// recallDown is the editor-edge fallback for downward navigation: it
// walks forward through the submit history, clearing the input once it
// moves past the newest entry. It reports whether anything was recalled.
func (s *dialogueHandler) recallDown() bool {
	if s.historyIdx >= len(s.history) {
		return false
	}
	s.historyIdx++
	if s.historyIdx == len(s.history) {
		s.comp.Input().Clear()
	} else {
		s.comp.Input().SetText(s.history[s.historyIdx])
	}
	return true
}

// drainQueue sends the first queued message (FIFO) through tx and
// promotes its visual representation from "queued" to "sent".
// Must be called with s.mu held. It unlocks around the channel send
// and re-locks on return.
func (s *dialogueHandler) drainQueue() {
	if len(s.queue) == 0 {
		return
	}
	msg := s.queue[0]
	s.queue = s.queue[1:]
	s.comp.PromoteFirstQueuedMessage()
	s.comp.AddSendMessage(msg.Text)
	s.mu.Unlock()
	s.tx <- msg
	s.mu.Lock()
}
