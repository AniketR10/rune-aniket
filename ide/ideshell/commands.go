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

package ideshell

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/sh"
)

// New creates a repl.Handler wired with a CommandRegistry,
// sh layer, and the built-in help command. The returned
// registry can be used to register additional commands.
func New(
	scheduleNextTick func(func()) bool,
	interrupter term.Interrupter,
	opts ...repl.Option,
) (*repl.Handler, *CommandRegistry) {
	r := NewRegistry()
	registerBaseCommands(r)
	h := repl.New(sh.New(r), scheduleNextTick, interrupter, opts...)
	return h, r
}

func registerBaseCommands(r *CommandRegistry) {
	r.Register("help", "Show available commands", &helpHandler{r: r})
}

type helpHandler struct {
	r *CommandRegistry
}

func (h *helpHandler) HandleCommand(
	ctx context.Context, cmd repl.Command,
) (iterator.Iterator[component.Responsive], error) {
	return h.r.Help(ctx, cmd.Args)
}

func (h *helpHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	if len(args) == 0 {
		return iterator.Empty[string](), nil
	}
	return h.r.Complete(ctx, args[len(args)-1], nil)
}

func (h *helpHandler) Help(
	_ context.Context, _ []string,
) (iterator.Iterator[component.Responsive], error) {
	return toLines(
		"Show available commands or help for a specific command",
	), nil
}

func toLines(ss ...string) iterator.Iterator[component.Responsive] {
	out := make([]component.Responsive, len(ss))
	for i, s := range ss {
		out[i] = toResponsive(s)
	}
	return iterator.FromSlice(out)
}

func toResponsive(s string) component.Responsive {
	return component.NewResponsiveString(
		s, component.StringResponsiveConfig{},
	)
}
