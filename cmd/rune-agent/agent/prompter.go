// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agent

import "context"

// PromptOption describes a single selectable choice.
type PromptOption struct {
	Label       string
	Description string
	Value       string
	// RequiresInput indicates that selecting this option should
	// open a text input box so the user can type a response.
	RequiresInput bool
}

// PromptRequest describes a prompt to show to the user.
type PromptRequest struct {
	Title       string
	Header      string // short chip label (max 12 chars)
	Body        string // optional markdown content shown before options
	Options     []PromptOption
	MultiSelect bool
}

// PromptResponse holds the user's selection(s).
type PromptResponse struct {
	Values    []string // selected option Value(s)
	TextInput string   // text typed by the user (when a RequiresInput option was selected)
}

// Prompter allows tools to block and wait for user input.
type Prompter interface {
	Prompt(ctx context.Context, req PromptRequest) (PromptResponse, error)
}
