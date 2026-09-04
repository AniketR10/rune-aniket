// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package agentshell

import (
	"context"
	"fmt"
	"time"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/cmd/rune-agent/llm/llmarg"
	"unstable.build/rune/cmd/rune-agent/memory/dream"
	"unstable.build/rune/cmd/rune-agent/memory/dream/dreamcomponent"
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
