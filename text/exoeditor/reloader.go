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

package exoeditor

import "github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

// Reloader routes an FS-watcher-driven reload for the file at uri
// into the IDE's canonical async reload pipeline — the same path
// :reloadfile uses. Implementations live in the ide package and
// resolve uri to the open tab's workspace.FlusherCloser internally.
//
// Reload is non-blocking: it starts the async reload and returns
// either nil or a start-failure error (e.g. workspace.ErrFlushInProgress
// when another op is already in flight). The actual disk I/O,
// buffer reset, and dirty-tab attribute clear happen on the IDE's
// own awaiter goroutine + UI scheduler — callers do not wait for
// completion.
//
// Reload must be called on the host UI goroutine: it touches the
// open-tab map and the FlusherCloser swap-worker state, which
// cooperate with subscribers that mutate UI-owned cell.Buffer state.
type Reloader interface {
	Reload(uri workspaceapi.URI) error
}
