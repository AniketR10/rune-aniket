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

package dialogue

import (
	"context"

	"unstable.build/go-tui/cmd/extension_ai/backend"
)

// Completer wraps the Complete method.
type Completer interface {
	// Complete is called by a Manager when a chat completion has streamed
	// successfully. It enables clients of Manager to access the
	// complete Metadata field returned by backend.Service at the end
	// of the stream and also allows them to handle backend.FinishReason.
	Complete(ctx context.Context, dialogueID, completionID string,
		reason backend.FinishReason, msg backend.ChatCompletionMessage)
}

// FuncCompleter satisfies Completer by wrapping a function.
type FuncCompleter func(context.Context, string, string,
	backend.FinishReason, backend.ChatCompletionMessage)

// Complete satisfies Completer.
func (f FuncCompleter) Complete(ctx context.Context, dialogueID, completionID string,
	reason backend.FinishReason, msg backend.ChatCompletionMessage) {
	f(ctx, dialogueID, completionID, reason, msg)
}
