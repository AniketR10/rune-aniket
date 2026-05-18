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

package vte

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term/vte/vteparser"
)

// Replay feeds raw ANSI bytes through a headless VTE sized at width x height
// and returns a snapshot of the resulting primary buffer plus the final
// cursor position in scroll-relative coordinates (matching the row indices
// of the returned [][]term.Cell).
//
// The returned buffer is independent from the internal VTE state: its cells
// are cloned, so callers may retain it without holding any locks.
//
// Replay is intended for offline, deterministic analysis of recorded ANSI
// output (for example, terminal screen captures used by vteprobe tests).
// It does not start a pty, does not read from a master, and does not touch
// the clipboard or any tab manager.
func Replay(width, height int, data []byte) (*cell.Buffer, term.Coordinates, error) {
	if width <= 0 || height <= 0 {
		return nil, term.Coordinates{}, fmt.Errorf(
			"vte.Replay: width and height must be > 0 (got %dx%d)",
			width, height)
	}
	uri, err := workspaceapi.ParseURI("memory:///replay")
	if err != nil {
		return nil, term.Coordinates{}, fmt.Errorf("vte.Replay: parse uri: %w", err)
	}

	pty := workspaceapi.Pty{Master: replayFile{}, Slave: replayFile{}}
	ph := newParserHandler(
		new(sync.Mutex), pty, replayTabManager{}, clipboard.NewInMemory(),
		func() {}, uri, term.Attributes{},
		false, // useTitleAsTabname=false: SetTitle becomes a no-op
		0,     // maxScrollLength: caller controls history via screen size
		0,     // minWidth: width is the authoritative dimension
	)
	// Match the convention used by parser_handler tests so empty cells
	// render as visible spaces rather than the implementation-detail
	// zero rune. Callers (e.g. vteprobe) treat the cell grid as text.
	ph.sync.primBuf.SetDefaultChar(' ')
	ph.sync.altBuf.SetDefaultChar(' ')
	ph.Resize(width, height)

	parser := vteparser.NewParser(ph, new(vteparser.StdTimeout))
	for _, b := range data {
		parser.Advance(b)
	}

	cells := term.CloneCells(ph.sync.primBuf.Cells.RawCells())
	cursor := ph.sync.primBuf.CursorAtScroll()
	return cell.CellsToBuffer(cells), cursor, nil
}

// replayTabManager satisfies browser.TabManager with no side effects.
// It is only used during Replay, which keeps the parser handler in focus
// so the tab-name path is never reached.
type replayTabManager struct{}

func (replayTabManager) Tab(
	workspaceapi.URI, rune, string, browserapi.Handler,
) (browserapi.Handler, error) {
	return nil, nil
}

func (replayTabManager) SetTabName(
	workspaceapi.URI, string, term.Attributes,
) error {
	return nil
}

// replayFile satisfies workspaceapi.File for the replay pty.
// Writes are dropped (the parser only writes responses to queries we do not
// care about during replay); reads return EOF.
type replayFile struct{}

func (replayFile) Name() string                      { return "" }
func (replayFile) Stat() (os.FileInfo, error)        { return nil, io.EOF }
func (replayFile) Sync() error                       { return nil }
func (replayFile) Truncate(int64) error              { return nil }
func (replayFile) Fd() uintptr                       { return 0 }
func (replayFile) Seek(int64, int) (int64, error)    { return 0, nil }
func (replayFile) Read([]byte) (int, error)          { return 0, io.EOF }
func (replayFile) ReadAt([]byte, int64) (int, error) { return 0, io.EOF }
func (replayFile) Write(p []byte) (int, error)       { return len(p), nil }
func (replayFile) Close() error                      { return nil }
