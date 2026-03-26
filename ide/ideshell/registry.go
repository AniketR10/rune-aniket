// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

// Package ideshell provides an extensible REPL/shell for managing
// IDE internal resources. Commands are registered via a
// CommandRegistry and executed through an sh-aware repl.Handler.
package ideshell

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
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

// HandleCommand looks up cmd.Name in the registry and
// delegates to the matching handler. Returns
// repl.ErrNotFound for unknown commands so the sh layer
// can fall back to PATH executables.
func (r *CommandRegistry) HandleCommand(
	ctx context.Context, cmd repl.Command,
) (iterator.Iterator[component.Responsive], error) {
	r.mu.RLock()
	e, ok := r.commands[cmd.Name]
	r.mu.RUnlock()
	if !ok {
		return nil, repl.ErrNotFound
	}
	return e.handler.HandleCommand(ctx, cmd)
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
		name    string
		summary string
	}
	cmds := make([]cmd, 0, len(r.commands))
	for name, e := range r.commands {
		cmds = append(cmds, cmd{name: name, summary: e.summary})
	}
	r.mu.RUnlock()
	sort.Slice(cmds, func(i, j int) bool {
		return cmds[i].name < cmds[j].name
	})
	lines := make([]component.Responsive, len(cmds))
	for i, c := range cmds {
		lines[i] = component.NewResponsiveString(
			fmt.Sprintf("  %-12s %s", c.name, c.summary),
			component.StringResponsiveConfig{},
		)
	}
	return iterator.FromSlice(lines)
}

type entry struct {
	summary string
	handler CommandHandler
}
