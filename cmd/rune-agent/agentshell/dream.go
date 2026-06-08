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

package agentshell

import (
	"context"
	"fmt"
	"time"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmarg"
	"unstable.build/go-tui/cmd/rune-agent/memory/dream"
	"unstable.build/go-tui/cmd/rune-agent/memory/dream/dreamcomponent"
)

func (s *shell) handleDream(
	ctx context.Context, args []string, pw repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	model := s.dreamModel
	if model == "" {
		model = s.defaultModel
	}
	debug := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--model":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--model requires a value")
			}
			i++
			model = args[i]
		case "--debug":
			debug = true
		default:
			return nil, fmt.Errorf(
				"unknown argument: %s\nusage: dream [--model MODEL] [--debug]", args[i])
		}
	}

	svc, err := s.serviceForModel(model)
	if err != nil {
		return nil, fmt.Errorf("create llm service: %w", err)
	}
	entry, err := llmarg.Resolve(ctx, svc, model)
	if err != nil {
		return nil, err
	}

	deps := dream.Deps{
		LLM:           svc,
		Store:         s.store,
		Storage:       s.storage,
		FS:            s.fs,
		Exec:          s.exec,
		LSP:           s.lsp,
		Parser:        s.parser,
		Notifications: s.notifications,
		DataPath:      s.memoryDataPath,
		Model:         entry,
	}

	it, err := dream.Dream(ctx, deps)
	if err != nil {
		return nil, err
	}

	return iterator.FromFunc(
		func(ctx context.Context) (component.Responsive, bool, error) {
			for {
				p, ok := it.Next(ctx)
				if !ok {
					return nil, false, it.Err()
				}
				if p.Total > 0 && pw != nil {
					pw.Progress(int64(p.Progress)+1, int64(p.Total), p.Units)
				}
				if debug {
					return responsiveString(formatDebugProgress(p)), true, nil
				}
				if p.Type == dream.ProgressToolCall {
					continue
				}
				return dreamcomponent.New(p), true, nil
			}
		},
		func() error { return it.Close() },
	), nil
}

func responsiveString(s string) component.Responsive {
	return component.NewResponsiveString(s, component.StringResponsiveConfig{})
}

func formatDebugProgress(p dream.Progress) string {
	ts := time.Now().Format("15:04:05.000")
	out := fmt.Sprintf("[%s] %s", ts, progressTypeName(p.Type))
	if p.DialogueID != "" {
		out += " dlg=" + p.DialogueID
	}
	if p.ToolName != "" {
		out += " tool=" + p.ToolName
	}
	if p.Total > 0 {
		if p.Units != "" {
			out += fmt.Sprintf(" %d/%d %s", p.Progress+1, p.Total, p.Units)
		} else {
			out += fmt.Sprintf(" %d/%d", p.Progress+1, p.Total)
		}
	}
	if p.IsError {
		out += " error=true"
	}
	if p.Message != "" {
		out += " | " + p.Message
	}
	return out
}

func progressTypeName(t dream.ProgressType) string {
	switch t {
	case dream.ProgressBootstrap:
		return "bootstrap"
	case dream.ProgressAnalyzing:
		return "analyzing"
	case dream.ProgressWriting:
		return "writing"
	case dream.ProgressVerifying:
		return "verifying"
	case dream.ProgressFixing:
		return "fixing"
	case dream.ProgressMigrating:
		return "migrating"
	case dream.ProgressReprocessing:
		return "reprocessing"
	case dream.ProgressToolCall:
		return "tool-call"
	case dream.ProgressToolResult:
		return "tool-result"
	case dream.ProgressError:
		return "error"
	case dream.ProgressPhaseStart:
		return "phase-start"
	case dream.ProgressPhaseFinish:
		return "phase-finish"
	case dream.ProgressDone:
		return "done"
	}
	return fmt.Sprintf("type(%d)", int(t))
}
