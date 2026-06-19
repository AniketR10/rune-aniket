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

package textrpc

import "context"

// Waiter lets a command dispatcher learn when an asynchronously handled
// command finishes. A caller attaches it to the dispatch context with
// ContextWithWaiter; a handler whose HandleCommand returns before the
// command is actually done (e.g. the out-of-process extension command
// stream) reports completion on Ch instead.
//
// Claimed is the contract between such a handler and the caller: the
// handler MUST set Claimed before HandleCommand returns when it takes
// responsibility for delivering the result on Ch. The caller reads
// Claimed only after dispatch returns, so a plain bool is sufficient —
// no synchronization is needed. When Claimed is false the command
// completed synchronously and nothing is sent on Ch.
type Waiter struct {
	Ch      chan error
	Claimed bool
}

type waiterKey struct{}

// ContextWithWaiter returns a context carrying w so a command stream can
// report asynchronous completion back to the caller through w.Ch.
func ContextWithWaiter(ctx context.Context, w *Waiter) context.Context {
	return context.WithValue(ctx, waiterKey{}, w)
}

// WaiterFromContext returns the Waiter attached by ContextWithWaiter, if
// any.
func WaiterFromContext(ctx context.Context) (*Waiter, bool) {
	w, ok := ctx.Value(waiterKey{}).(*Waiter)
	return w, ok
}
