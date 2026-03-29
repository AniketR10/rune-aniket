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

package applypatch

import (
	"fmt"
	"strings"
)

// OpType describes the kind of file operation in a patch.
type OpType int

const (
	// OpAdd creates a new file.
	OpAdd    OpType = iota // Create a new file
	// OpDelete deletes an existing file.
	OpDelete               // Delete an existing file
	// OpUpdate modifies an existing file.
	OpUpdate               // Modify an existing file
)

// LineKind describes whether a diff line is context, added, or removed.
type LineKind int

const (
	// LineContext marks an unchanged line (prefix " ").
	LineContext LineKind = iota // Unchanged line (prefix " ")
	// LineAdd marks an added line (prefix "+").
	LineAdd                    // Added line (prefix "+")
	// LineRemove marks a removed line (prefix "-").
	LineRemove                 // Removed line (prefix "-")
)

// Line is a single diff line within a hunk.
type Line struct {
	Kind    LineKind
	Content string
}

// Hunk is a section of changes within a file update.
type Hunk struct {
	ContextHint string // text after @@ (optional section label)
	Lines       []Line
}

// FileOp is a single file operation within a patch.
type FileOp struct {
	Type   OpType
	Path   string
	MoveTo string // only for Update with "*** Move to:"
	Lines  []Line // for Add: lines to write
	Hunks  []Hunk // for Update: diff hunks
}

// Patch is a collection of file operations.
type Patch struct {
	Ops []FileOp
}

const (
	prefixBegin  = "*** Begin Patch"
	prefixEnd    = "*** End Patch"
	prefixAdd    = "*** Add File:"
	prefixDelete = "*** Delete File:"
	prefixUpdate = "*** Update File:"
	prefixMove   = "*** Move to:"
	prefixHunk   = "@@"
)

// Parse parses a Codex-style patch from the input string.
func Parse(input string) (Patch, error) {
	lines := strings.Split(input, "\n")
	p := parser{lines: lines}
	return p.parse()
}

type parser struct {
	lines []string
	pos   int
}

func (p *parser) peek() string {
	if p.pos >= len(p.lines) {
		return ""
	}
	return p.lines[p.pos]
}

func (p *parser) next() string {
	s := p.peek()
	p.pos++
	return s
}

func (p *parser) done() bool {
	return p.pos >= len(p.lines)
}

func (p *parser) parse() (Patch, error) {
	// Skip leading blank lines and preamble text, find "*** Begin Patch"
	// or a file directive. Models sometimes omit delimiters or emit
	// commentary before the patch.
	for !p.done() {
		if strings.TrimSpace(p.peek()) == prefixBegin {
			break
		}
		trimmed := strings.TrimSpace(p.peek())
		if trimmed == "" {
			p.pos++
			continue
		}
		// If we hit a file directive before *** Begin Patch, treat it
		// as an implicit start — the model omitted the begin marker.
		if isFileDirective(trimmed) {
			break
		}
		// Otherwise skip preamble text (model commentary).
		p.pos++
	}
	if p.done() {
		return Patch{}, fmt.Errorf("empty patch: no file operations found")
	}
	// Consume *** Begin Patch if present; otherwise we're already on
	// a file directive.
	if strings.TrimSpace(p.peek()) == prefixBegin {
		p.pos++
	}

	var patch Patch
	for !p.done() {
		line := strings.TrimSpace(p.peek())
		switch {
		case line == prefixEnd:
			p.pos++
			// Ignore any trailing text after *** End Patch.
			return patch, nil
		case strings.HasPrefix(line, prefixAdd):
			op, err := p.parseAdd()
			if err != nil {
				return Patch{}, err
			}
			patch.Ops = append(patch.Ops, op)
		case strings.HasPrefix(line, prefixDelete):
			op := p.parseDelete()
			patch.Ops = append(patch.Ops, op)
		case strings.HasPrefix(line, prefixUpdate):
			op, err := p.parseUpdate()
			if err != nil {
				return Patch{}, err
			}
			patch.Ops = append(patch.Ops, op)
		default:
			return Patch{}, fmt.Errorf("unexpected line %q at line %d", p.peek(), p.pos+1)
		}
	}

	// All lines consumed without *** End Patch. If we parsed at least
	// one operation, treat as valid — the model omitted the end marker.
	if len(patch.Ops) > 0 {
		return patch, nil
	}
	return Patch{}, fmt.Errorf("empty patch: no file operations found")
}

func (p *parser) parseAdd() (FileOp, error) {
	path := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p.next()), prefixAdd))
	op := FileOp{Type: OpAdd, Path: path}

	for !p.done() {
		if isDirective(p.peek()) {
			break
		}
		line := p.next()
		if len(line) == 0 {
			// Empty line in an add block: treat as empty content line.
			op.Lines = append(op.Lines, Line{Kind: LineAdd, Content: ""})
			continue
		}
		if line[0] != '+' {
			return FileOp{}, fmt.Errorf("expected '+' prefix in add block, got %q at line %d", line, p.pos)
		}
		op.Lines = append(op.Lines, Line{Kind: LineAdd, Content: line[1:]})
	}
	return op, nil
}

func (p *parser) parseDelete() FileOp {
	path := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p.next()), prefixDelete))
	return FileOp{Type: OpDelete, Path: path}
}

func (p *parser) parseUpdate() (FileOp, error) {
	path := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p.next()), prefixUpdate))
	op := FileOp{Type: OpUpdate, Path: path}

	// Optional "*** Move to:"
	if !p.done() && strings.HasPrefix(strings.TrimSpace(p.peek()), prefixMove) {
		op.MoveTo = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(p.next()), prefixMove))
	}

	// Parse hunks
	for !p.done() {
		trimmed := strings.TrimSpace(p.peek())
		if isFileDirective(trimmed) || trimmed == prefixEnd {
			break
		}
		if !strings.HasPrefix(trimmed, prefixHunk) {
			return FileOp{}, fmt.Errorf("expected @@ hunk header, got %q at line %d", p.peek(), p.pos+1)
		}
		hunk, err := p.parseHunk()
		if err != nil {
			return FileOp{}, err
		}
		op.Hunks = append(op.Hunks, hunk)
	}
	return op, nil
}

func (p *parser) parseHunk() (Hunk, error) {
	header := strings.TrimSpace(p.next())
	// Extract optional context hint after @@
	hint := strings.TrimPrefix(header, prefixHunk)
	hint = strings.TrimSpace(hint)

	hunk := Hunk{ContextHint: hint}

	for !p.done() {
		if isDirective(p.peek()) {
			break
		}
		line := p.next()
		if len(line) == 0 {
			// Empty line in diff: treat as context with empty content.
			hunk.Lines = append(hunk.Lines, Line{Kind: LineContext, Content: ""})
			continue
		}
		switch line[0] {
		case ' ':
			hunk.Lines = append(hunk.Lines, Line{Kind: LineContext, Content: line[1:]})
		case '+':
			hunk.Lines = append(hunk.Lines, Line{Kind: LineAdd, Content: line[1:]})
		case '-':
			hunk.Lines = append(hunk.Lines, Line{Kind: LineRemove, Content: line[1:]})
		default:
			return Hunk{}, fmt.Errorf("unexpected diff line prefix %q at line %d", line, p.pos)
		}
	}
	return hunk, nil
}

func isDirective(line string) bool {
	trimmed := strings.TrimSpace(line)
	return isFileDirective(trimmed) ||
		trimmed == prefixEnd ||
		strings.HasPrefix(trimmed, prefixHunk)
}

func isFileDirective(trimmed string) bool {
	return strings.HasPrefix(trimmed, prefixAdd) ||
		strings.HasPrefix(trimmed, prefixDelete) ||
		strings.HasPrefix(trimmed, prefixUpdate)
}
