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

package text

import (
	"context"
)

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type ctxKey int

// pKey is the key for sync.Payload values in Contexts. It is
// unexported; clients use workspace.ContextWithPayload and
// PayloadFromContext instead of using this key directly.
var pKey ctxKey

// barsKey signals that a text.Editor.Edit caller wants the editor
// to install its full auxiliary chrome (status / icons / aux bars,
// command bar) on top of the bare buffer view. It is unexported;
// callers use withAuxiliaryBars to opt in and editors use
// BarsFromContext to read the flag.
var barsKey ctxKey = 1

func contextWithAlias(ctx context.Context, cmd string) context.Context {
	return context.WithValue(ctx, pKey, cmd)
}

// IsAliasContext can be called from a dispatched command's handler
// to know if the command was dispatched through an alias.
func IsAliasContext(ctx context.Context) (string, bool) {
	locker, ok := ctx.Value(pKey).(string)
	return locker, ok
}

// withAuxiliaryBars returns a context that signals to text.Editor
// implementations that the caller wants the full auxiliary chrome
// (status / icons / aux bars, command bar) wrapped around the
// returned handler.
//
// Only text.Component, which owns the surrounding browser tab and
// reserves screen space for those bars, should set this; other
// callers (input boxes, command prompts, finders, file explorers)
// embed the editor in their own layout and want a bare view.
func withAuxiliaryBars(ctx context.Context) context.Context {
	return context.WithValue(ctx, barsKey, true)
}

// BarsFromContext reports whether the context was prepared by
// withAuxiliaryBars. text.Editor implementations call it to decide
// whether to wrap the returned text.Handler with status / icons /
// aux bars.
func BarsFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(barsKey).(bool)
	return v
}
