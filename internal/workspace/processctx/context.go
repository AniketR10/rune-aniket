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

// Package processctx carries logical process metadata through context.
package processctx

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

type parentPidContextKey struct{}
type extensionIDContextKey struct{}

// ContextWithParentPid returns a context carrying the given logical parent PID.
func ContextWithParentPid(ctx context.Context, pid workspaceapi.Pid) context.Context {
	return context.WithValue(ctx, parentPidContextKey{}, pid)
}

// ParentPidFromContext extracts a logical parent PID from ctx.
func ParentPidFromContext(ctx context.Context) (workspaceapi.Pid, bool) {
	pid, ok := ctx.Value(parentPidContextKey{}).(workspaceapi.Pid)
	return pid, ok && pid != 0
}

// ContextWithExtensionID returns a context carrying the logical extension ID
// associated with a process or RPC call.
func ContextWithExtensionID(ctx context.Context, extensionID string) context.Context {
	return context.WithValue(ctx, extensionIDContextKey{}, extensionID)
}

// ExtensionIDFromContext extracts a logical extension ID from ctx.
func ExtensionIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(extensionIDContextKey{}).(string)
	return id, ok && id != ""
}

// DeriveCommandContext copies logical process metadata from callerCtx into
// commandCtx. The returned context keeps commandCtx cancellation/deadline while
// carrying caller metadata needed by process tracking.
func DeriveCommandContext(commandCtx, callerCtx context.Context) context.Context {
	if parent, ok := ParentPidFromContext(callerCtx); ok {
		commandCtx = ContextWithParentPid(commandCtx, parent)
	}
	if extensionID, ok := ExtensionIDFromContext(callerCtx); ok {
		commandCtx = ContextWithExtensionID(commandCtx, extensionID)
	}
	return commandCtx
}
