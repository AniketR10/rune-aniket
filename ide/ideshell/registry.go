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

// Package ideshell provides an extensible REPL/shell for managing
// IDE internal resources. Commands are registered via a
// CommandRegistry and executed through an sh-aware repl.Handler.
package ideshell

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/component/markdown"
)

// CommandHandler extends repl.CommandHandler with recursive
// help resolution. Every registered command implements this
// interface, enabling "help foo bar" to delegate through
// nested registries.
type CommandHandler interface {
	repl.CommandHandler
	Help(ctx context.Context, args []string) (
		iterator.Iterator[component.Responsive], error,
	)
}

// CommandRegistry stores commands keyed by name and implements
// CommandHandler by dispatching to registered handlers.
type CommandRegistry struct {
	mu       sync.RWMutex
	commands map[string]entry
}

// NewRegistry returns an empty CommandRegistry.
func NewRegistry() *CommandRegistry {
	return &CommandRegistry{
		commands: make(map[string]entry),
	}
}

// Register adds a command with the given name, summary,
// and handler.
func (r *CommandRegistry) Register(
	name, summary string, h CommandHandler,
) {
	r.mu.Lock()
	r.commands[name] = entry{summary: summary, handler: h}
	r.mu.Unlock()
}

// RegisterREPLCommand registers an extension-provided REPL
// command with the registry.
func (r *CommandRegistry) RegisterREPLCommand(
	man textapi.CommandManual, h textapi.REPLHandler,
) error {
	if man.Name == "" {
		return errors.New("command name is required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.commands[man.Name]; ok {
		return errors.New("command already registered")
	}

	r.commands[man.Name] = entry{
		summary:  man.Summary,
		synopsis: man.Synopsis,
		handler:  h,
	}
	return nil
}

// UnregisterREPLCommand removes an extension-provided REPL
// command from the registry.
func (r *CommandRegistry) UnregisterREPLCommand(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.commands[name]; !ok {
		return errors.New("command not registered")
	}

	delete(r.commands, name)
	return nil
}

// HandleCommand looks up cmd.Name in the registry and
// delegates to the matching handler. Returns
// repl.ErrNotFound for unknown commands so the sh layer
// can fall back to PATH executables.
func (r *CommandRegistry) HandleCommand(
	ctx context.Context, cmd repl.Command, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	r.mu.RLock()
	e, ok := r.commands[cmd.Name]
	r.mu.RUnlock()
	if !ok {
		return nil, repl.ErrNotFound
	}
	return e.handler.HandleCommand(ctx, cmd, pw)
}

// Complete returns command name completions when args is
// nil, or delegates to the matching handler's Complete when
// args is non-nil.
func (r *CommandRegistry) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	if args != nil {
		r.mu.RLock()
		e, ok := r.commands[cmd]
		r.mu.RUnlock()
		if ok {
			return e.handler.Complete(ctx, cmd, args)
		}
		return iterator.Empty[string](), nil
	}
	r.mu.RLock()
	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		if strings.HasPrefix(name, cmd) {
			names = append(names, name)
		}
	}
	r.mu.RUnlock()
	sort.Strings(names)
	return iterator.FromSlice(names), nil
}

// lookup returns the entry registered under name, if any.
func (r *CommandRegistry) lookup(name string) (entry, bool) {
	r.mu.RLock()
	e, ok := r.commands[name]
	r.mu.RUnlock()
	return e, ok
}

// Help returns help output. With no args it lists all
// commands and their summaries. With args it looks up the
// first arg and delegates to that handler's Help with the
// remaining args.
func (r *CommandRegistry) Help(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	if len(args) == 0 {
		return r.listCommands(), nil
	}
	r.mu.RLock()
	e, ok := r.commands[args[0]]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown command: %s", args[0])
	}
	return e.handler.Help(ctx, args[1:])
}

func (r *CommandRegistry) listCommands() iterator.Iterator[component.Responsive] {
	r.mu.RLock()
	type cmd struct {
		name     string
		synopsis string
		summary  string
	}
	cmds := make([]cmd, 0, len(r.commands))
	for name, e := range r.commands {
		cmds = append(cmds, cmd{
			name:     name,
			synopsis: e.synopsis,
			summary:  e.summary,
		})
	}
	r.mu.RUnlock()
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].name < cmds[j].name
	})
	var b strings.Builder
	for _, c := range cmds {
		fmt.Fprintf(&b, "- **%s**", c.name)
		if c.synopsis != "" {
			b.WriteString(" `")
			b.WriteString(c.synopsis)
			b.WriteByte('`')
		}
		if c.summary != "" {
			b.WriteString(" — ")
			b.WriteString(c.summary)
		}
		b.WriteByte('\n')
	}
	return markdownList(b.String())
}

func markdownList(content string) iterator.Iterator[component.Responsive] {
	md, err := markdown.New(content)
	if err != nil {
		r := component.NewResponsiveString(content, component.StringResponsiveConfig{})
		return iterator.FromSlice([]component.Responsive{r})
	}
	return iterator.FromSlice([]component.Responsive{md})
}

type entry struct {
	summary  string
	synopsis string
	handler  CommandHandler
}

// registryFallback dispatches first to a CommandRegistry and, when the
// registry reports repl.ErrNotFound, falls back to a secondary handler.
// It backs Config.DisableShellInterpreter for REPL surfaces where every
// unmatched input line is a fragment of a hosted language rather than a
// discrete command.
type registryFallback struct {
	registry *CommandRegistry
	fallback repl.CommandHandler
}

var _ repl.CommandHandler = (*registryFallback)(nil)

func (f *registryFallback) HandleCommand(
	ctx context.Context, cmd repl.Command, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	iter, err := f.registry.HandleCommand(ctx, cmd, pw)
	if errors.Is(err, repl.ErrNotFound) {
		return f.fallback.HandleCommand(ctx, cmd, pw)
	}
	return iter, err
}

func (f *registryFallback) Complete(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	// A registered command's own argument completion always wins.
	if args != nil {
		if _, ok := f.registry.lookup(cmd); ok {
			return f.registry.Complete(ctx, cmd, args)
		}
		return f.fallback.Complete(ctx, cmd, args)
	}
	// args == nil is command-name completion in a system shell, but in a
	// language REPL the first token is itself a fragment of the hosted
	// language (e.g. "fmt.Pri"). Offer the fallback's completions for
	// the fragment, and only fall back to registered command names when
	// the fallback has nothing to add.
	fb, err := f.fallback.Complete(ctx, cmd, args)
	if err != nil {
		return nil, err
	}
	items, err := iterator.ToSlice(ctx, fb)
	if err != nil {
		return nil, err
	}
	if len(items) > 0 {
		return iterator.FromSlice(items), nil
	}
	return f.registry.Complete(ctx, cmd, args)
}

// CompletionPrefix forwards to the fallback handler when it is a
// PrefixCompleter, so a language REPL's identifier-boundary prefix is
// honored. Registry commands keep the default whitespace-token prefix.
func (f *registryFallback) CompletionPrefix(line string) string {
	if pc, ok := f.fallback.(PrefixCompleter); ok {
		return pc.CompletionPrefix(line)
	}
	return completionPrefix(line, nil)
}
