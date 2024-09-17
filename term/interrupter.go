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

package term

import (
	"context"
	"time"
)

// Interrupter wraps the basic method Interrupt, which
// sends an interrupt event to the main loop, forcing a redraw
// of all components.
//
// The given context is piped back into the next loop iteration
// so callers can use it to distinguish between an interrupt-driven
// call to Draw or just the next tick.
type Interrupter interface {
	Interrupt(context.Context) error
}

// NopInterrupter is an interrupter that does nothing when Interrupt
// is called.
func NopInterrupter() Interrupter {
	return FuncInterrupter(func(context.Context) error { return nil })
}

// FuncInterrupter returns an Interrupter that calls fn
// every time Interrupt is called.
func FuncInterrupter(fn func(context.Context) error) Interrupter {
	return fnInterrupter{fn: fn}
}

type fnInterrupter struct {
	fn func(context.Context) error
}

func (i fnInterrupter) Interrupt(ctx context.Context) error {
	return i.fn(ctx)
}

// InterruptAt interrupts the main event loop at the given fps, using the
// given interrupter. This function only returns when context is canceled.
func InterruptAt(ctx context.Context, interrupter Interrupter, fps int) {
	cadence := time.Duration(int(time.Second) / fps)
	ticker := time.NewTicker(cadence)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = interrupter.Interrupt(ctx)
		case <-ctx.Done():
			return
		}
	}
}
