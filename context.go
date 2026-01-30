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

package tui

import (
	"context"
	"strconv"

	"unstable.build/go-tui/term"
)

// ContextWithIteration returns a new Context that holds locker.
//
// Deprecated: use term.Event.Context to propagate this.
func ContextWithIteration(ctx context.Context, i int64) context.Context {
	return term.ContextWithPayload(ctx, []byte(strconv.FormatInt(i, 10)))
}

// IterationFromContext returns the ID value stored in ctx, if any.
//
// Deprecated: use term.Event.Context to propagate this.
func IterationFromContext(ctx context.Context) (int64, bool) {
	payload, ok := term.PayloadFromContext(ctx)
	if !ok {
		return 0, false
	}
	return IterationFromRawBytes(payload)
}

// IterationFromRawBytes parses the given payload into an iteration number,
// as formatted by ContextWithIteration.
//
// Deprecated: use term.Event.Context to propagate this.
func IterationFromRawBytes(payload []byte) (int64, bool) {
	i, err := strconv.ParseInt(string(payload), 10, 64)
	if err != nil {
		return 0, false
	}
	return i, true
}
