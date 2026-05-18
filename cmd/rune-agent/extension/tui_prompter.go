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

package extension

import (
	"context"
	"errors"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
)

// tuiPrompter bridges agent.Prompter to the dialoguetui event channel
// so prompts surface as inline selection UI in the chat tab.
type tuiPrompter struct {
	tx   chan<- dialoguetui.MessageEvent
	noti browserapi.Notifications
}

func (tp *tuiPrompter) Prompt(ctx context.Context, req agent.PromptRequest) (agent.PromptResponse, error) {
	resultCh := make(chan []string, 1)

	// Build label→value mapping so we can translate the TUI's
	// label-based selections back into the option Values that
	// callers compare against.
	labelToValue := make(map[string]string, len(req.Options))
	options := make([]dialoguetui.PromptEventOption, len(req.Options))
	for i, o := range req.Options {
		options[i] = dialoguetui.PromptEventOption{
			Label:         o.Label,
			Description:   o.Description,
			RequiresInput: o.RequiresInput,
		}
		if o.Value != "" {
			labelToValue[o.Label] = o.Value
		}
	}

	ev := dialoguetui.MessageEvent{
		Type:              dialoguetui.MessageEventPrompt,
		PromptTitle:       req.Title,
		PromptHeader:      req.Header,
		PromptBody:        req.Body,
		PromptOptions:     options,
		PromptMultiSelect: req.MultiSelect,
		PromptResult:      resultCh,
	}
	select {
	case tp.tx <- ev:
	case <-ctx.Done():
		return agent.PromptResponse{}, ctx.Err()
	}
	_, _ = tp.noti.Notify(browserapi.LevelWarn, "Input required: %s", req.Title)
	select {
	case vals := <-resultCh:
		if vals == nil {
			return agent.PromptResponse{}, errors.New("prompt dismissed")
		}

		// Free-form prompt (zero options): the TUI sends [text].
		// Return as TextInput only; Values stays nil.
		if len(req.Options) == 0 {
			return agent.PromptResponse{TextInput: vals[0]}, nil
		}

		// Map labels back to values where a mapping exists.
		var textInput string
		// When the TUI sends [label, text], the second element
		// is free-form text from a RequiresInput option.
		if len(vals) == 2 {
			textInput = vals[1]
			vals = vals[:1] // keep only the label for value mapping
		}
		for i, v := range vals {
			if mapped, ok := labelToValue[v]; ok {
				vals[i] = mapped
			}
		}
		return agent.PromptResponse{Values: vals, TextInput: textInput}, nil
	case <-ctx.Done():
		select {
		case tp.tx <- dialoguetui.MessageEvent{Type: dialoguetui.MessageEventPromptDismiss}:
		default:
		}
		return agent.PromptResponse{}, ctx.Err()
	}
}
