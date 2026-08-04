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

package ide

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

// watchedFilesTelemetryLSP reports the number of files carried by each
// workspace/didChangeWatchedFiles notification before delegating. Only the
// agent-facing RPC seam is wrapped, so filesystem watcher events — which
// reach the language servers through idelsp's own event handler — are never
// reported.
type watchedFilesTelemetryLSP struct {
	semanticapi.LSP
	onChange func(n int)
}

func (w watchedFilesTelemetryLSP) DidChangeWatchedFiles(
	ctx context.Context, params semanticapi.DidChangeWatchedFilesParams,
) error {
	if w.onChange != nil {
		w.onChange(len(params.Changes))
	}
	return w.LSP.DidChangeWatchedFiles(ctx, params)
}
