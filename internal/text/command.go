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

package text

import (
	"context"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// CommandHandler wraps the basic methods HandleCommand and Complete.
type CommandHandler interface {
	// HandleCommand is called when user issued a command previously registered via SubscribeCommand.
	HandleCommand(context.Context, textapi.Command) (err error)

	// Complete takes command args and returns a list of expanded options for them.
	// It also returns a expanded version of the last arg, or an empty string
	// if the last arg could/should not be automatically expanded.
	Complete(ctx context.Context, cmd textapi.Command) (
		iterator.Iterator[string], string, error,
	)
}

// FallbackPrompter handles a command that was invoked but has no
// registered handler (e.g. its providing extension is not installed).
type FallbackPrompter interface {
	ShowFallbackPrompt(ctx context.Context, command string, args ...string)
}

// WorkspaceCommandRegistry abstracts the ability to subscribe to commands
// for a particular workspace.
type WorkspaceCommandRegistry interface {
	SubscribeCommandForWorkspace(
		workspace workspaceapi.URI, cmd textapi.CommandManual, handler CommandHandler) error
	UnsubscribeCommandForWorkspace(workspace workspaceapi.URI, name string) error
}

// FileCommandRegistry abstracts the ability to subscribe to commands
// for a particular file.
type FileCommandRegistry interface {
	SubscribeCommandForFile(
		file workspaceapi.URI, cmd textapi.CommandManual, handler CommandHandler) error
	UnsubscribeCommandForFile(file workspaceapi.URI, name string) error
}

// FuncCommandHandler returns an CommandHandler that calls fn
// every time HandleCommand is invoked. If completeFn is not nil,
// then it is called when Complete is invoked.
func FuncCommandHandler(
	fn func(context.Context, textapi.Command) error,
	completeFn func(context.Context, textapi.Command) (
		iterator.Iterator[string], string, error,
	),
) CommandHandler {
	return fnCommandHandler{
		cb:         fn,
		completeFn: completeFn,
	}
}

type fnCommandHandler struct {
	cb         func(context.Context, textapi.Command) error
	completeFn func(context.Context, textapi.Command) (
		iterator.Iterator[string], string, error,
	)
}

func (f fnCommandHandler) HandleCommand(ctx context.Context, c textapi.Command) error {
	return f.cb(ctx, c)
}

func (f fnCommandHandler) Complete(ctx context.Context, cmd textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	if f.completeFn != nil {
		return f.completeFn(ctx, cmd)
	}
	return iterator.FromSlice[string](nil), "", nil
}
