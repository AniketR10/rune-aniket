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
