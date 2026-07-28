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

// Package dreamcomponent renders dream.Progress events as styled
// single-line component.Responsive rows suitable for streaming over
// the REPL→extension RPC bridge.
//
// The package intentionally exposes no in-place "live" component:
// textrpc.streamResponsive snapshots each yielded Responsive at yield
// time, so a long-lived component that mutates after yield would never
// render later state changes over RPC. Callers stream one New(p) per
// dream.Progress event instead.
package dreamcomponent

import (
	"fmt"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/cmd/rune-agent/memory/dream"
)

// status describes the lifecycle of an entry.
type status int

const (
	statusRunning status = iota
	statusSuccess
	statusError
)

// entryKind classifies entries so rendering can pick colors and glyphs.
type entryKind int

const (
	kindBootstrap entryKind = iota + 1
	kindMigrating
	kindPhase
	kindDialogue
	kindTool
	kindError
	kindDone
)

// entry is the rendered state for a single dream.Progress event.
type entry struct {
	kind     entryKind
	status   status
	label    string
	name     string // tool name rendered before label (bold, fuchsia)
	progress int
	total    int
	units    string
	indent   int
	isError  bool
}

const (
	glyphSuccess     = "✓"
	glyphError       = "✗"
	glyphWorking     = "⚙"
	glyphPhaseFinish = "✓"
	glyphDone        = "■"
	glyphSubStep     = "└─"
)

var (
	successAttr   = term.Attributes{Fg: term.ColorGreen}
	errorAttr     = term.Attributes{Fg: term.ColorRed}
	phaseAttr     = term.Attributes{Fg: term.ColorFuchsia, Attrs: term.AttrBold}
	migrateAttr   = term.Attributes{Fg: term.ColorOlive}
	bootstrapAttr = term.Attributes{Fg: term.ColorAqua}
	doneAttr      = term.Attributes{Fg: term.ColorGreen, Attrs: term.AttrBold}
	toolAttr      = term.Attributes{Fg: term.ColorGray}
	toolNameAttr  = term.Attributes{Fg: term.ColorFuchsia, Attrs: term.AttrBold}
	counterAttr   = term.Attributes{Fg: term.ColorGray}
	subStepAttr   = term.Attributes{Fg: term.ColorGray}
	labelAttr     = term.Attributes{}
	workingAttr   = term.Attributes{}
)

// drawEntry paints one entry on row y of writer w. Returns no value;
// the caller knows the entry occupies a single row.
func drawEntry(w term.Writer, y, width int, e *entry) {
	x := 0
	if e.indent > 0 {
		lead := strings.Repeat("  ", e.indent-1) + glyphSubStep + " "
		x = writeRuneLineAttr(w, x, y, lead, width, subStepAttr)
	}

	glyph, glyphAttr := glyphForEntry(e)
	x = writeRuneLineAttr(w, x, y, glyph+" ", width, glyphAttr)

	// Tool entries render as `<name in bold fuchsia> <suffix in gray>`
	// mirroring the collapsed tool view in dialoguetui. Other kinds
	// render their entire label in a single attribute.
	if e.kind == kindTool {
		if e.name != "" {
			x = writeRuneLineAttr(w, x, y, e.name, width, toolNameAttr)
			if e.label != "" {
				x = writeRuneLineAttr(w, x, y, " ", width, toolAttr)
			}
		}
		if e.label != "" {
			x = writeRuneLineAttr(w, x, y, e.label, width, toolAttr)
		}
	} else {
		x = writeRuneLineAttr(w, x, y, e.label, width, labelAttrForEntry(e))
	}

	if e.total > 0 {
		counter := fmt.Sprintf(" (%d/%d", e.progress+1, e.total)
		if e.units != "" {
			counter += " " + e.units
		}
		counter += ")"
		writeRuneLineAttr(w, x, y, counter, width, counterAttr)
	}
}

func glyphForEntry(e *entry) (string, term.Attributes) {
	switch e.status {
	case statusRunning:
		return glyphWorking, workingAttr
	case statusError:
		return glyphError, errorAttr
	}
	// statusSuccess
	switch e.kind {
	case kindBootstrap:
		return glyphSuccess, bootstrapAttr
	case kindMigrating:
		return glyphSuccess, migrateAttr
	case kindPhase:
		return glyphPhaseFinish, phaseAttr
	case kindDone:
		return glyphDone, doneAttr
	case kindError:
		return glyphError, errorAttr
	}
	return glyphSuccess, successAttr
}

func labelAttrForEntry(e *entry) term.Attributes {
	switch e.kind {
	case kindPhase:
		return phaseAttr
	case kindMigrating:
		return migrateAttr
	case kindBootstrap:
		return bootstrapAttr
	case kindDone:
		return doneAttr
	case kindError:
		return errorAttr
	case kindTool:
		return toolAttr
	}
	return labelAttr
}

// writeRuneLineAttr writes s at (x, y) with the given attributes,
// stopping at maxWidth. Returns the new x position.
func writeRuneLineAttr(w term.Writer, x, y int, s string, maxWidth int, attr term.Attributes) int {
	for _, ch := range s {
		if x >= maxWidth {
			break
		}
		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
			Attributes: attr,
			Ch:         ch,
			Width:      1,
		})
		x++
	}
	return x
}

// --- label helpers ---

func phaseName(msg string) string {
	for _, prefix := range []string{"Starting phase: ", "Completed phase: "} {
		if rest, ok := strings.CutPrefix(msg, prefix); ok {
			return rest
		}
	}
	return msg
}

func phaseLabel(msg string) string {
	name := phaseName(msg)
	if name == "" {
		return "Phase"
	}
	return "Phase: " + name
}

func dialogueLabel(p dream.Progress) string {
	if p.Message != "" {
		return p.Message
	}
	if p.DialogueID != "" {
		return "Conversation " + p.DialogueID
	}
	return "Conversation"
}

func fallback(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// toolDurationSuffix returns "(in 1.2s)" when p.Duration > 0, otherwise
// the empty string. Tool rows render as "<tool_name> (in 1.2s)" instead
// of the verbose tool summary so the lifecycle is concise.
func toolDurationSuffix(p dream.Progress) string {
	if p.Duration <= 0 {
		return ""
	}
	return fmtDuration(p.Duration)
}

// fmtDuration renders a tool duration like "(in 1.2s)" or "(in 250ms)".
// Sub-millisecond durations collapse to "(in <1ms)" to avoid noisy
// "0s" suffixes.
func fmtDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return ""
	case d < time.Millisecond:
		return "(in <1ms)"
	case d < time.Second:
		return fmt.Sprintf("(in %dms)", d.Milliseconds())
	default:
		return fmt.Sprintf("(in %.1fs)", d.Seconds())
	}
}
