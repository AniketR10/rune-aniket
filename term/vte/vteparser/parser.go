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

package vteparser

import (
	"bytes"
	"math"
	"time"
	"unicode/utf8"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/term/vte/vtescanner"
)

const (
	// Maximum number of bytes read in one synchronized update (2MiB).
	syncBufferSize = 0x20_0000
	// Maximum time before a synchronized update is aborted.
	syncUpdateTimeout time.Duration = 150 * time.Millisecond

	// Number of bytes in the BSU/ESU CSI sequences.
	syncEscapeLen = 8
)

var (
	// BSU CSI sequence for beginning or extending synchronized updates.
	bsuCSI = []byte("\x1b[?2026h")

	// ESU CSI sequence for terminating synchronized updates.
	esuCSI = []byte("\x1b[?2026l")
)

// SyncState represents the state for synchronized terminal updates.
type syncState struct {
	// Timeout handler for synchronized updates.
	timeout Timeout
	// Bytes read during the synchronized update.
	buffer []byte
}

// parserState represents the internal state for the VTE parser.
type parserState struct {
	// Last processed character for repetition.
	precedingChar rune

	// State for synchronized terminal updates.
	syncState syncState
}

// Parser wraps a Parser to ultimately call methods on a Handler.
type Parser struct {
	state   parserState
	scanner *vtescanner.Scanner
	handler Handler
}

// NewParser returns a new Parser instance.
func NewParser(handler Handler, timeout Timeout) *Parser {
	ret := new(Parser)
	ret.Init(handler, timeout)
	return ret
}

// Init initializes this parser with the given Handler
// and Timeout implementations.
func (p *Parser) Init(handler Handler, timeout Timeout) {
	p.handler = handler
	p.state = parserState{
		syncState: syncState{
			buffer:  make([]byte, 0, syncBufferSize),
			timeout: timeout,
		},
	}
	var driver vtescanner.Driver = newDriver(&p.state, handler)
	if log.IsLevelEnabled(log.TraceLevel) {
		driver = vtescanner.WithLoggingDriver(driver)
	}
	p.scanner = vtescanner.NewScanner(driver)
}

// SyncTimeout returns the synchronized update timeout.
func (p *Parser) SyncTimeout() Timeout {
	return p.state.syncState.timeout
}

// Advance processes a new byte from the PTY.
func (p *Parser) Advance(ch byte) {
	if p.state.syncState.timeout.PendingTimeout() {
		p.advanceSync(ch)
	} else {
		p.scanner.Advance(ch)
	}
}

// AdvanceBytes processes a batch of bytes from the PTY. Runs of
// printable text encountered while the scanner is in ground state are
// delivered to the handler in one InputRun call instead of per-byte
// dispatch, which dominates bulk output streams.
func (p *Parser) AdvanceBytes(buf []byte) {
	for len(buf) > 0 {
		if p.state.syncState.timeout.PendingTimeout() {
			p.advanceSync(buf[0])
			buf = buf[1:]
			continue
		}
		if n := p.scanner.GroundRun(buf); n > 0 {
			p.handler.InputRun(buf[:n])
			p.state.precedingChar, _ = utf8.DecodeLastRune(buf[:n])
			buf = buf[n:]
			continue
		}
		p.scanner.Advance(buf[0])
		buf = buf[1:]
	}
}

// StopSync ends a synchronized update.
func (p *Parser) StopSync() {
	// Process all synchronized bytes.
	for _, ch := range p.state.syncState.buffer {
		p.scanner.Advance(ch)
	}

	// Report that update ended, since we could end due to timeout.
	p.handler.UnsetPrivateMode(PrivateModeSyncUpdate)
	// Resetting state after processing makes sure we don't interpret buffered sync escapes.
	p.state.syncState.buffer = p.state.syncState.buffer[:0]
	p.state.syncState.timeout.ClearTimeout()
}

// SyncBytesCount returns the number of bytes in the synchronization buffer.
func (p *Parser) SyncBytesCount() int {
	return len(p.state.syncState.buffer)
}

// AdvanceSync processes a new byte during a synchronized update.
func (p *Parser) advanceSync(ch byte) {
	p.state.syncState.buffer = append(p.state.syncState.buffer, ch)

	// Handle sync CSI escape sequences.
	p.advanceSyncCSI()
}

// AdvanceSyncCSI handles BSU/ESU CSI sequences during synchronized update.
func (p *Parser) advanceSyncCSI() {
	// Get the last few bytes for comparison.
	len := len(p.state.syncState.buffer)
	offset := int(math.Max(float64(len-syncEscapeLen), 0))
	end := p.state.syncState.buffer[offset:]

	// NOTE: It is technically legal to specify multiple private modes in the same
	// escape, but we only allow EXACTLY `\e[?2026h`/`\e[?2026l` to keep the parser
	// reasonable.
	//
	// Check for extension/termination of the synchronized update.
	if bytes.Equal(end, bsuCSI) {
		p.state.syncState.timeout.SetTimeout(syncUpdateTimeout)
	} else if bytes.Equal(end, esuCSI) || len >= syncBufferSize-1 {
		p.StopSync()
	}
}
