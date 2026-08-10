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
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/audit"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/agentshell"
	"unstable.build/go-tui/cmd/rune-agent/configedit"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmarg"
	"unstable.build/go-tui/component/markdown"
	mdhandler "unstable.build/go-tui/handler/markdown"
)

// commandAdapter wraps a repl.CommandHandler into a dialoguetui.CommandHandler
// with session-aware overrides for commands like /clear, /history, and /model.
type commandAdapter struct {
	handler    repl.CommandHandler
	dialogueID string
	resetFn    func() // visual reset (Component.Reset under lock)
	wm         browserapi.WindowManager
	store      dialoguemanager.Store
	// compactFn replaces messages in the component after a successful compact.
	compactFn func(msgs []llmapi.Message)

	// Model switching support. When agent is non-nil, the /model command
	// can switch the backing LLM service mid-conversation.
	agent        *agent.Agent
	llmSvc       llmapi.Service
	config       configedit.Config
	ctx          context.Context
	storage      storageapi.Service
	currentModel string
	auditStore   *audit.Store

	// Skill resolution. When a /name command matches a skill, the
	// formatted skill content is returned as UserMessage.
	skillRegistry *skills.SkillRegistry
}

// commandAdapterDeps groups the values commandAdapter borrows from its
// owning aiEditorHandler. Keeping them in a struct lets handleChat thread
// a single argument to newCommandAdapter instead of a long parameter list.
type commandAdapterDeps struct {
	handler       repl.CommandHandler
	dialogueID    string
	wm            browserapi.WindowManager
	store         dialoguemanager.Store
	llmSvc        llmapi.Service
	config        configedit.Config
	ctx           context.Context
	storage       storageapi.Service
	currentModel  string
	skillRegistry *skills.SkillRegistry
	auditStore    *audit.Store

	// mu guards comp / hintSlot writes; the closures below take it.
	mu          *sync.Mutex
	comp        **dialoguetui.Component // late-bound: caller assigns *comp later
	hintSlot    *hintSlot
	interrupter term.Interrupter
}

// newCommandAdapter builds the commandAdapter and its resetFn / compactFn
// closures. The closures dereference *deps.comp at call time, so the
// caller can construct the adapter before instantiating the component
// and assign *deps.comp afterwards.
func newCommandAdapter(deps commandAdapterDeps) *commandAdapter {
	return &commandAdapter{
		handler:       deps.handler,
		dialogueID:    deps.dialogueID,
		wm:            deps.wm,
		store:         deps.store,
		llmSvc:        deps.llmSvc,
		config:        deps.config,
		ctx:           deps.ctx,
		storage:       deps.storage,
		currentModel:  deps.currentModel,
		skillRegistry: deps.skillRegistry,
		auditStore:    deps.auditStore,
		resetFn: func() {
			deps.mu.Lock()
			(*deps.comp).Reset()
			deps.mu.Unlock()
		},
		compactFn: func(msgs []llmapi.Message) {
			deps.mu.Lock()
			comp := *deps.comp
			comp.Reset()
			pending := make(map[string]llmapi.ToolCall)
			for _, msg := range msgs {
				addMessage(comp, msg, pending)
			}
			// Re-add the status hint if one is active. Reset and
			// AddSendMessageMarkdown (called during replay) both
			// remove it, so we restore it after all messages are
			// replayed to keep the progress animation visible
			// during compaction.
			if deps.hintSlot.comp != nil {
				comp.AddReceiveMessageHint(deps.hintSlot.comp, deps.hintSlot.conf)
			}
			deps.mu.Unlock()
			_ = deps.interrupter.Interrupt(context.Background())
		},
	}
}

func (a *commandAdapter) HandleCommand(
	ctx context.Context, name string, args []string,
) (dialoguetui.CommandResult, error) {
	// Check if the command matches a skill before dispatching as a shell command.
	if skill, ok := a.skillRegistry.Get(name); ok {
		return dialoguetui.CommandResult{
			UserMessage: formatSkillMessage(skill, strings.Join(args, " ")),
			SkillName:   skill.Name,
		}, nil
	}

	switch name {
	case "clear":
		if err := rejectPositionalID(name, args); err != nil {
			return dialoguetui.CommandResult{}, err
		}
		return a.handleClear(ctx)
	case "history":
		if err := rejectPositionalID(name, args); err != nil {
			return dialoguetui.CommandResult{}, err
		}
		name = "chats"
		args = []string{"show", a.dialogueID}
	case "compact":
		if len(args) > 1 {
			return dialoguetui.CommandResult{}, errors.New("usage: /compact [<model>]")
		}
		return a.handleCompact(ctx, args)
	case "export":
		if err := rejectPositionalID(name, args); err != nil {
			return dialoguetui.CommandResult{}, err
		}
		name = "chats"
		args = append([]string{"export"}, args...)
		args = append(args, a.dialogueID)
	case "log":
		if err := rejectPositionalID(name, args); err != nil {
			return dialoguetui.CommandResult{}, err
		}
		name = "chats"
		args = []string{"log", a.dialogueID}
	case "model":
		return a.handleModel(ctx, args)
	case "effort":
		return a.handleEffort(args)
	case "max_tokens":
		return a.handleMaxTokens(args)
	case "fork":
		if err := rejectPositionalID(name, args); err != nil {
			return dialoguetui.CommandResult{}, err
		}
		name = "chats"
		args = []string{"fork", a.dialogueID}
	}
	it, err := a.handler.HandleCommand(
		ctx, repl.Command{Name: name, Args: args}, repl.NopProgressWriter(),
	)
	if errors.Is(err, agentshell.ErrExit) {
		return dialoguetui.CommandResult{Exit: true}, nil
	}
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}

	items, drainErr := iterator.ToSlice(ctx, it)
	_ = it.Close()
	if drainErr != nil {
		return dialoguetui.CommandResult{}, drainErr
	}
	if len(items) == 0 {
		return dialoguetui.CommandResult{}, nil
	}

	a.openCommandFloating(items)
	return dialoguetui.CommandResult{}, nil
}

// rejectPositionalID returns an error if args contains a non-flag
// positional argument. Chat commands act on the open chat only, so a
// dialogue id is no longer accepted.
func rejectPositionalID(name string, args []string) error {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			return fmt.Errorf("/%s does not take a dialogue id; it acts on the open chat", name)
		}
	}
	return nil
}

// formatSkillMessage formats the user-visible portion of a slash-command
// invocation. The full skill body is injected as a system message by
// Agent.run via WithSkillName; this function only emits the
// command envelope and any user-supplied arguments.
//
// A <command-hint> envelope is appended so the model is told, inline
// in the user turn, to invoke the skill tool. Without this cue the
// model frequently fails to notice the system-message skill body and
// either ignores the command or falls back to request_skill.
func formatSkillMessage(skill skills.Skill, args string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<command-message>%s</command-message>\n", skill.Name)
	fmt.Fprintf(&b, "<command-name>/%s</command-name>", skill.Name)
	if args != "" {
		b.WriteString("\n")
		b.WriteString(args)
	}
	fmt.Fprintf(
		&b,
		"\n<command-hint>Call the skill tool with name=%q to load and run this skill.</command-hint>",
		skill.Name,
	)
	return b.String()
}

func parseStoredCommandMessage(content string) (string, bool) {
	const (
		messageOpen  = "<command-message>"
		messageClose = "</command-message>"
		nameOpen     = "<command-name>"
		nameClose    = "</command-name>"
		hintOpen     = "<command-hint>"
		hintClose    = "</command-hint>"
	)

	if !strings.HasPrefix(content, messageOpen) {
		return "", false
	}
	rest := strings.TrimPrefix(content, messageOpen)
	messageEnd := strings.Index(rest, messageClose)
	if messageEnd < 0 {
		return "", false
	}
	messageName := rest[:messageEnd]
	rest = rest[messageEnd+len(messageClose):]
	if !strings.HasPrefix(rest, "\n"+nameOpen) {
		return "", false
	}
	rest = strings.TrimPrefix(rest, "\n"+nameOpen)
	nameEnd := strings.Index(rest, nameClose)
	if nameEnd < 0 {
		return "", false
	}
	commandName := rest[:nameEnd]
	rest = rest[nameEnd+len(nameClose):]

	if messageName == "" || commandName == "" || !strings.HasPrefix(commandName, "/") {
		return "", false
	}
	if strings.TrimPrefix(commandName, "/") != messageName {
		return "", false
	}
	if hintIdx := strings.LastIndex(rest, "\n"+hintOpen); hintIdx >= 0 {
		if strings.HasSuffix(rest, hintClose) {
			rest = rest[:hintIdx]
		}
	}
	if rest == "" {
		return commandName, true
	}
	if !strings.HasPrefix(rest, "\n") {
		return "", false
	}
	args := strings.TrimPrefix(rest, "\n")
	if args == "" {
		return commandName, true
	}
	if strings.Contains(args, "\n") {
		return commandName + "\n" + args, true
	}
	return commandName + " " + args, true
}

// handleModel shows the current model or switches to a new one.
func (a *commandAdapter) handleModel(
	ctx context.Context, args []string,
) (dialoguetui.CommandResult, error) {
	if len(args) == 0 {
		md, err := markdown.New(a.resolvedModelLabel(ctx))
		if err != nil {
			return dialoguetui.CommandResult{}, err
		}
		return dialoguetui.CommandResult{
			Display: iterator.FromSlice([]component.Responsive{md}),
		}, nil
	}
	arg := args[0]
	entry, err := llmarg.Resolve(ctx, a.llmSvc, arg)
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}
	svc := a.llmSvc
	if a.auditStore != nil {
		svc = audit.NewService(svc, a.auditStore, entry)
	}
	a.agent.SwapService(svc, entry)
	qualified := entry.Provider + "/" + entry.Name
	a.currentModel = qualified
	md, err := markdown.New(fmt.Sprintf("Switched to model **%s**", qualified))
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}
	return dialoguetui.CommandResult{
		Display: iterator.FromSlice([]component.Responsive{md}),
	}, nil
}

// resolvedModelLabel returns the qualified provider/model the session
// currently uses. currentModel may be a router alias (e.g. "default"),
// so it is resolved through the service rather than displayed verbatim.
func (a *commandAdapter) resolvedModelLabel(ctx context.Context) string {
	if entry, err := llmarg.Resolve(ctx, a.llmSvc, a.currentModel); err == nil {
		return entry.Provider + "/" + entry.Name
	}
	return a.currentModel
}

// validEffortLevels lists the allowed reasoning effort values.
var validEffortLevels = []llmapi.ReasoningEffort{
	llmapi.ReasoningEffortNone,
	llmapi.ReasoningEffortMinimal,
	llmapi.ReasoningEffortLow,
	llmapi.ReasoningEffortMedium,
	llmapi.ReasoningEffortHigh,
	llmapi.ReasoningEffortXHigh,
	llmapi.ReasoningEffortMax,
	llmapi.ReasoningEffortUltra,
}

// handleEffort shows the current effort level or sets a new one.
func (a *commandAdapter) handleEffort(args []string) (dialoguetui.CommandResult, error) {
	if len(args) == 0 {
		current := string(a.agent.Effort())
		if current == "" {
			current = "model default"
		}
		md, err := markdown.New(fmt.Sprintf("Current effort level: **%s**", current))
		if err != nil {
			return dialoguetui.CommandResult{}, err
		}
		return dialoguetui.CommandResult{
			Display: iterator.FromSlice([]component.Responsive{md}),
		}, nil
	}

	level := llmapi.ReasoningEffort(args[0])
	valid := false
	for _, v := range validEffortLevels {
		if level == v {
			valid = true
			break
		}
	}
	if !valid {
		return dialoguetui.CommandResult{}, fmt.Errorf(
			"invalid effort level %q: must be none, minimal, low, medium, high, xhigh, max, or ultra",
			args[0])
	}

	a.agent.SetEffort(level)
	md, err := markdown.New(fmt.Sprintf("Set effort level to **%s**.", level))
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}
	return dialoguetui.CommandResult{
		Display: iterator.FromSlice([]component.Responsive{md}),
	}, nil
}

// handleMaxTokens shows or sets the per-session max-output-token override.
func (a *commandAdapter) handleMaxTokens(args []string) (dialoguetui.CommandResult, error) {
	if len(args) == 0 {
		current := a.agent.MaxOutputTokens()
		text := "Current max output tokens: provider/config default"
		if current > 0 {
			text = fmt.Sprintf("Current max output tokens: **%d**", current)
		}
		md, err := markdown.New(text)
		if err != nil {
			return dialoguetui.CommandResult{}, err
		}
		return dialoguetui.CommandResult{
			Display: iterator.FromSlice([]component.Responsive{md}),
		}, nil
	}

	n, err := strconv.Atoi(args[0])
	if err != nil || n <= 0 {
		return dialoguetui.CommandResult{}, fmt.Errorf(
			"invalid max_tokens value %q: must be a positive integer", args[0])
	}

	if err := llmarg.ValidateMaxOutputTokens(a.agent.ModelEntry(), n); err != nil {
		return dialoguetui.CommandResult{}, err
	}

	a.agent.SetMaxOutputTokens(n)
	md, err := markdown.New(fmt.Sprintf("Set max output tokens to **%d**.", n))
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}
	return dialoguetui.CommandResult{
		Display: iterator.FromSlice([]component.Responsive{md}),
	}, nil
}

// handleClear resets the UI, runs the archive+clear via the shell, and
// returns the confirmation message inline instead of in a floating window.
func (a *commandAdapter) handleClear(ctx context.Context) (dialoguetui.CommandResult, error) {
	if a.resetFn != nil {
		a.resetFn()
	}
	it, err := a.handler.HandleCommand(ctx, repl.Command{
		Name: "chats", Args: []string{"clear", a.dialogueID},
	}, repl.NopProgressWriter())
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}
	items, drainErr := iterator.ToSlice(ctx, it)
	_ = it.Close()
	if drainErr != nil {
		return dialoguetui.CommandResult{}, drainErr
	}
	if len(items) == 0 {
		return dialoguetui.CommandResult{}, nil
	}
	return dialoguetui.CommandResult{
		Display: iterator.FromSlice(items),
	}, nil
}

// handleCompact returns a lazy iterator that blocks during the LLM
// summarisation call so AddCommand's animation plays. After the iterator
// is fully drained, its Close method (which runs AFTER AddCommand removes
// the animation node) resets the component and replays the compacted
// messages. This ordering avoids the panic from Remove-after-Reset.
func (a *commandAdapter) handleCompact(
	_ context.Context, args []string,
) (dialoguetui.CommandResult, error) {
	var model string
	if len(args) > 0 {
		model = args[0]
	}
	return dialoguetui.CommandResult{
		Display: &compactIterator{
			handler:    a.handler,
			store:      a.store,
			dialogueID: a.dialogueID,
			model:      model,
			compactFn:  a.compactFn,
		},
	}, nil
}

func (a *commandAdapter) openCommandFloating(items []component.Responsive) {
	// When the single item is a markdown component, use the markdown handler
	// which provides rich navigation (vi keys, search, mouse selection).
	if len(items) == 1 {
		if md, ok := items[0].(*markdown.Component); ok {
			a.openMarkdownFloating(md)
			return
		}
	}

	list := component.NewResponsiveList()
	for _, item := range items {
		list.PushBack(item)
	}

	listHandler := handler.NopFromComponent(list)
	scrollHandler := handler.Wrap(listHandler, func(ev term.Event) (exit, handled bool) {
		if ev.Type != term.EventKey {
			return
		}
		switch ev.Key {
		case term.KeyEsc:
			return true, true
		case term.KeyArrowUp:
			list.SeekUp()
			return false, true
		case term.KeyArrowDown:
			list.SeekDown()
			return false, true
		}
		if ev.Mod == term.ModCtrl {
			switch ev.Ch {
			case 'k', 'p':
				list.SeekUp()
				return false, true
			case 'j', 'n':
				list.SeekDown()
				return false, true
			}
		}
		return
	})

	const floatingWidth = 90
	var win browserapi.Window
	bhandler := browserapi.FuncHandler(scrollHandler, func() error {
		return a.wm.CloseWindow(win)
	})
	floating := browserapi.FuncFloating(bhandler, func() (int, int) {
		h := list.Height(floatingWidth)
		return floatingWidth, min(h, 45)
	})
	var err error
	win, err = a.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		slog.Error("command floating window", "error", err)
	}
}

func (a *commandAdapter) openMarkdownFloating(md *markdown.Component) {
	mdh := mdhandler.New(md)

	var win browserapi.Window
	bhandler := browserapi.FuncHandler(mdh, func() error {
		return a.wm.CloseWindow(win)
	})
	floating := browserapi.FuncFloating(bhandler, mdh.Dimensions)
	var err error
	win, err = a.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		slog.Error("command floating window", "error", err)
	}
}

func (a *commandAdapter) Complete(
	ctx context.Context, name string, args []string,
) (iterator.Iterator[string], error) {
	if name == "compact" && len(args) > 1 {
		return iterator.FromSlice[string](nil), nil
	}
	if (name == "model" || name == "compact") && a.llmSvc != nil {
		// Qualified to disambiguate name collisions across providers
		// (e.g. openai/gpt-5.5 vs codex/gpt-5.5).
		return iterator.Map(a.llmSvc.Models(), func(e llmapi.ModelEntry) string {
			return e.Provider + "/" + e.Name
		}), nil
	}
	if name == "effort" {
		levels := make([]string, len(validEffortLevels))
		for i, l := range validEffortLevels {
			levels[i] = string(l)
		}
		return iterator.FromSlice(levels), nil
	}
	return a.handler.Complete(ctx, name, args)
}
