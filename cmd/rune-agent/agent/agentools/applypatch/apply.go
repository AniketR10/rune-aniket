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
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func resolve(cwd workspaceapi.URI, path string) string {
	expanded, err := workspaceapi.ExpandPathWithURI(path, cwd)
	if err != nil {
		return path
	}
	return expanded
}

// Result summarises which operations succeeded.
type Result struct {
	Applied int
	Total   int
	Errors  []string
}

// Apply executes every operation in patch against the filesystem rooted
// at cwd.
func Apply(fs workspaceapi.FileSystem, cwd workspaceapi.URI, patch Patch) Result {
	res := Result{Total: len(patch.Ops)}
	for _, op := range patch.Ops {
		var err error
		switch op.Type {
		case OpAdd:
			err = applyAdd(fs, cwd, op)
		case OpDelete:
			err = applyDelete(fs, cwd, op)
		case OpUpdate:
			err = applyUpdate(fs, cwd, op)
		}
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", op.Path, err))
		} else {
			res.Applied++
		}
	}
	return res
}

func applyAdd(fs workspaceapi.FileSystem, cwd workspaceapi.URI, op FileOp) error {
	path := resolve(cwd, op.Path)

	if _, err := fs.Stat(path); err == nil {
		return fmt.Errorf("file already exists")
	}

	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	var content strings.Builder
	for i, l := range op.Lines {
		if i > 0 {
			content.WriteByte('\n')
		}
		content.WriteString(l.Content)
	}

	return writeFileFS(fs, path, []byte(content.String()))
}

func applyDelete(fs workspaceapi.FileSystem, cwd workspaceapi.URI, op FileOp) error {
	path := resolve(cwd, op.Path)

	if _, err := fs.Stat(path); err != nil {
		return fmt.Errorf("file does not exist")
	}

	return fs.Remove(path)
}

func applyUpdate(fs workspaceapi.FileSystem, cwd workspaceapi.URI, op FileOp) error {
	path := resolve(cwd, op.Path)

	data, err := readFileFS(fs, path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	content := string(data)
	// Split preserving the trailing newline status.
	fileLines := splitLines(content)

	// Collect resolved hunk positions so we can apply bottom-to-top.
	type resolvedHunk struct {
		pos  int
		hunk Hunk
	}
	var resolved []resolvedHunk
	searchStart := 0
	for i, hunk := range op.Hunks {
		pattern := hunkPattern(hunk)
		pos := seekSequence(fileLines, pattern, searchStart)
		if pos < 0 {
			return hunkMatchError(i+1, fileLines, pattern, searchStart, hunk.ContextHint)
		}
		resolved = append(resolved, resolvedHunk{pos: pos, hunk: hunk})
		searchStart = pos + len(pattern)
	}

	// Apply hunks bottom-to-top to avoid index shifting.
	sort.Slice(resolved, func(i, j int) bool {
		return resolved[i].pos > resolved[j].pos
	})

	for _, rh := range resolved {
		fileLines = spliceHunk(fileLines, rh.pos, rh.hunk)
	}

	result := joinLines(fileLines)

	// Handle move-to: write to new path, remove old.
	if op.MoveTo != "" {
		newPath := resolve(cwd, op.MoveTo)
		if err := fs.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
			return fmt.Errorf("mkdir move target: %w", err)
		}
		if err := writeFileFS(fs, newPath, []byte(result)); err != nil {
			return err
		}
		return fs.Remove(path)
	}

	return writeFileFS(fs, path, []byte(result))
}

// hunkPattern extracts the lines that must be present in the file
// for the hunk to match: context lines and remove lines.
func hunkPattern(h Hunk) []string {
	var pat []string
	for _, l := range h.Lines {
		if l.Kind == LineContext || l.Kind == LineRemove {
			pat = append(pat, l.Content)
		}
	}
	return pat
}

// hunkMatchError produces a detailed error when a hunk fails to match.
// It finds the best partial match and reports the first diverging line
// so the caller can understand why matching failed.
func hunkMatchError(hunkNum int, fileLines, pattern []string, searchStart int, contextHint string) error {
	bm := bestPartialMatch(fileLines, pattern, searchStart)

	var b strings.Builder
	fmt.Fprintf(&b, "hunk %d: no match found", hunkNum)
	if contextHint != "" {
		fmt.Fprintf(&b, " (near %q)", contextHint)
	}

	if bm.Matched == 0 && len(pattern) > 0 {
		fmt.Fprintf(&b, "\n  could not match any context lines")
		fmt.Fprintf(&b, "\n  first expected line: %q", pattern[0])
		if searchStart < len(fileLines) {
			fmt.Fprintf(&b, "\n  search started at line %d: %q", searchStart+1, fileLines[searchStart])
		}
		return fmt.Errorf("%s", b.String())
	}

	fmt.Fprintf(&b, "\n  best partial match at line %d: %d/%d lines matched",
		bm.Pos+1, bm.Matched, bm.Total)
	if bm.PastEOF {
		fmt.Fprintf(&b, "\n  line %d: expected %q but reached end of file",
			bm.Pos+bm.Matched+1, bm.ExpectedLine)
	} else {
		fmt.Fprintf(&b, "\n  line %d: expected %q",
			bm.Pos+bm.Matched+1, bm.ExpectedLine)
		fmt.Fprintf(&b, "\n  line %d:      got %q",
			bm.Pos+bm.Matched+1, bm.ActualLine)
	}
	return fmt.Errorf("%s", b.String())
}

// spliceHunk replaces the matched region at pos with the hunk's
// context + add lines.
func spliceHunk(fileLines []string, pos int, h Hunk) []string {
	patLen := 0
	for _, l := range h.Lines {
		if l.Kind == LineContext || l.Kind == LineRemove {
			patLen++
		}
	}

	var replacement []string
	for _, l := range h.Lines {
		if l.Kind == LineContext || l.Kind == LineAdd {
			replacement = append(replacement, l.Content)
		}
	}

	result := make([]string, 0, len(fileLines)-patLen+len(replacement))
	result = append(result, fileLines[:pos]...)
	result = append(result, replacement...)
	result = append(result, fileLines[pos+patLen:]...)
	return result
}

// splitLines splits content into lines. An empty string yields a
// single empty-string element. A trailing newline produces a trailing
// empty element to preserve round-trip fidelity.
func splitLines(s string) []string {
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "\n")
}

// joinLines is the inverse of splitLines.
func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

func readFileFS(fs workspaceapi.FileSystem, path string) ([]byte, error) {
	f, err := fs.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

func writeFileFS(fs workspaceapi.FileSystem, path string, data []byte) error {
	f, err := fs.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
