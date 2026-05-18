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

package dreamcomponent

import (
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/cmd/rune-agent/memory/dream"
)

// New returns a one-line component.Responsive that renders the given
// dream.Progress event as a styled row, suitable for streaming over
// the REPL→extension RPC bridge where each yielded Responsive is
// snapshotted at yield time and cannot mutate later. Stream one
// New(p) per dream.Progress event.
func New(p dream.Progress) component.Responsive {
	return &rowResponsive{e: entryFromProgress(p)}
}

// entryFromProgress builds the entry that renders a single
// dream.Progress event as one row.
func entryFromProgress(p dream.Progress) *entry {
	switch p.Type {
	case dream.ProgressBootstrap:
		return &entry{
			kind:   kindBootstrap,
			status: statusSuccess,
			label:  fallback(p.Message, "bootstrapping memory workspace"),
		}
	case dream.ProgressMigrating:
		return &entry{
			kind:   kindMigrating,
			status: statusSuccess,
			label:  fallback(p.Message, "migrating"),
		}
	case dream.ProgressPhaseStart:
		return &entry{
			kind:   kindPhase,
			status: statusRunning,
			label:  phaseLabel(p.Message),
		}
	case dream.ProgressPhaseFinish:
		return &entry{
			kind:   kindPhase,
			status: statusSuccess,
			label:  phaseLabel(p.Message),
		}
	case dream.ProgressReprocessing,
		dream.ProgressAnalyzing,
		dream.ProgressWriting,
		dream.ProgressVerifying,
		dream.ProgressFixing:
		return &entry{
			kind:     kindDialogue,
			status:   statusRunning,
			label:    dialogueLabel(p),
			progress: p.Progress,
			total:    p.Total,
			units:    p.Units,
		}
	case dream.ProgressToolCall:
		return &entry{
			kind:   kindTool,
			status: statusRunning,
			name:   p.ToolName,
			label:  toolDurationSuffix(p),
			indent: 1,
		}
	case dream.ProgressToolResult:
		status := statusSuccess
		if p.IsError {
			status = statusError
		}
		return &entry{
			kind:    kindTool,
			status:  status,
			name:    p.ToolName,
			label:   toolDurationSuffix(p),
			indent:  1,
			isError: p.IsError,
		}
	case dream.ProgressError:
		return &entry{
			kind:    kindError,
			status:  statusError,
			label:   fallback(p.Message, "error"),
			indent:  1,
			isError: true,
		}
	case dream.ProgressDone:
		return &entry{
			kind:   kindDone,
			status: statusSuccess,
			label:  fallback(p.Message, "Dream complete"),
		}
	}
	return &entry{
		kind:   kindBootstrap,
		status: statusRunning,
		label:  p.Message,
	}
}

// rowResponsive is a tiny component.Responsive that draws a single
// dream.Progress event as one styled row. It is stateless and
// self-contained so it survives the textrpc snapshot boundary
// unchanged.
type rowResponsive struct {
	e     *entry
	width int
}

var _ component.Responsive = (*rowResponsive)(nil)

// Height satisfies component.Responsive.
func (r *rowResponsive) Height(int) int { return 1 }

// Resize satisfies component.Responsive.
func (r *rowResponsive) Resize(width, _ int) {
	r.width = width
}

// Draw satisfies component.Responsive.
func (r *rowResponsive) Draw(w term.Writer) {
	if r.width <= 0 {
		return
	}
	drawEntry(w, 0, r.width, r.e)
}
