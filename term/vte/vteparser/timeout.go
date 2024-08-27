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

package vteparser

import "time"

// Timeout provides an interface for creating timeouts and checking their expiry.
type Timeout interface {
	// SetTimeout sets the timeout for the next synchronized update.
	//
	// The duration parameter specifies the duration of the timeout. Once the
	// specified duration has elapsed, the synchronized update routine can be
	// performed.
	SetTimeout(duration time.Duration)

	// ClearTimeout clears the current timeout.
	ClearTimeout()

	// PendingTimeout returns whether a timeout is currently active and has not yet expired.
	PendingTimeout() bool
}

// StdTimeout represents a std library's Timeout implementation.
// A zero value is ready to be used.
type StdTimeout struct {
	timeout time.Time
}

// SyncTimeout returns the synchronized update expiration time.
func (h *StdTimeout) SyncTimeout() time.Time {
	return h.timeout
}

// SetTimeout sets the timeout for the next synchronized update.
func (h *StdTimeout) SetTimeout(duration time.Duration) {
	h.timeout = time.Now().Add(duration)
}

// ClearTimeout clears the current timeout.
func (h *StdTimeout) ClearTimeout() {
	h.timeout = time.Time{}
}

// PendingTimeout returns whether a timeout is currently active and has not yet expired.
func (h *StdTimeout) PendingTimeout() bool {
	return time.Now().Before(h.timeout)
}
