// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
	"testing"

	"github.com/stretchr/testify/assert"
)

// captureHandler records InputRun and Input calls so tests can assert
// how the parser split a stream between the batched and per-rune paths.
type captureHandler struct {
	nopHandler
	runs    [][]byte
	singles []rune
}

func (h *captureHandler) Input(c rune) {
	h.singles = append(h.singles, c)
}

func (h *captureHandler) InputRun(run []byte) {
	cp := make([]byte, len(run))
	copy(cp, run)
	h.runs = append(h.runs, cp)
}

func TestAdvanceBytesBatchesPrintableRuns(t *testing.T) {
	h := &captureHandler{}
	p := NewParser(h, new(StdTimeout))

	p.AdvanceBytes([]byte("hello"))

	assert.Equal(t, [][]byte{[]byte("hello")}, h.runs)
	assert.Empty(t, h.singles)
	assert.Equal(t, 'o', p.state.precedingChar)
}

func TestAdvanceBytesSplitsOnControlBytes(t *testing.T) {
	h := &captureHandler{}
	p := NewParser(h, new(StdTimeout))

	// "ab\rcd": the CR is executed per-byte, the two printable runs batch.
	p.AdvanceBytes([]byte("ab\rcd"))

	assert.Equal(t, [][]byte{[]byte("ab"), []byte("cd")}, h.runs)
	assert.Empty(t, h.singles)
}

// TestLoggingHandlerForwardsInputRun pins that the trace-logging
// wrapper forwards batched runs; a no-op override would silently drop
// bulk output whenever trace logging is enabled.
func TestLoggingHandlerForwardsInputRun(t *testing.T) {
	h := &captureHandler{}
	wrapped := HandlerWithLogging("test", h)

	wrapped.InputRun([]byte("world"))

	assert.Equal(t, [][]byte{[]byte("world")}, h.runs)
}
