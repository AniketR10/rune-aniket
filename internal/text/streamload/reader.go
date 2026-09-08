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

package streamload

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/workspace/walkdir"
)

// pageReader streams a file's content line-by-line off a workspace's
// walkdir.Reader. Reads are synchronous; one call to readLines pulls
// up to n lines from the underlying scanner. The scanner is owned by
// the reader: Close releases the file.
//
// pageReader is not safe for concurrent use. Callers (notably
// *Handler) must drive it from a single goroutine — typically the
// event-loop goroutine via Handler.Handle.
type pageReader struct {
	file workspaceapi.File
	scan *bufio.Scanner
	eof  bool
}

// newPageReader opens path on reader and prepares a line-buffered
// scanner. It returns the reader plus the initial bufio.Scanner so the
// caller can pull pages of lines from it.
func newPageReader(r walkdir.Reader, path string) (*pageReader, error) {
	if r == nil {
		return nil, errors.New("streamload: nil walkdir.Reader")
	}
	f, err := r.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	pr := &pageReader{file: f}
	pr.scan = bufio.NewScanner(f)
	// Use a generous buffer so very long lines don't fail the scanner.
	// The default bufio.MaxScanTokenSize (64KiB) is fine for most code
	// files but can be too small for minified bundles or long-data
	// configs. Cap at 1MiB which matches what other Rune scanners use.
	const maxLine = 1 << 20
	pr.scan.Buffer(make([]byte, 0, 4096), maxLine)
	return pr, nil
}

// ready reports whether the underlying file is usable without
// blocking. Files opened through asyncOpenReader report false while
// their deferred open is in flight; reading them earlier would
// block on the open.
func (r *pageReader) ready() bool {
	if rf, ok := r.file.(interface{ Ready() bool }); ok {
		return rf.Ready()
	}
	return true
}

// readLines pulls up to n lines from the scanner and returns them
// joined by '\n' followed by a trailing '\n' if at least one line was
// read. The boolean reports whether any content was read; false means
// EOF (or a scanner error, also setting r.eof).
//
// readLines guarantees that the returned string ends with '\n' when
// non-empty, so callers appending pages via cell.Buffer.WriteString do
// not need to manage row terminators themselves.
func (r *pageReader) readLines(n int) (string, bool) {
	if r.eof || n <= 0 {
		return "", false
	}
	var sb strings.Builder
	read := 0
	for read < n && r.scan.Scan() {
		if read > 0 {
			sb.WriteByte('\n')
		}
		sb.Write(r.scan.Bytes())
		read++
	}
	if read == 0 {
		// either EOF or scanner error — treat both as terminal.
		r.eof = true
		return "", false
	}
	if read < n {
		// Scanner returned fewer than n lines; check for hard EOF /
		// error before declaring EOF, so the next page-read attempt
		// short-circuits.
		if err := r.scan.Err(); err == nil {
			r.eof = true
		} else if errors.Is(err, io.EOF) {
			r.eof = true
		}
	}
	// Trailing newline so the caller can simply append to a cell.Buffer
	// without worrying about row terminators.
	sb.WriteByte('\n')
	return sb.String(), true
}

// atEOF reports whether the reader has exhausted the file.
func (r *pageReader) atEOF() bool {
	return r.eof
}

// Close releases the underlying file. Safe to call multiple times.
func (r *pageReader) Close() error {
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	r.scan = nil
	r.eof = true
	return err
}
