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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/tcell/v3"
)

func TestParserIntegration(t *testing.T) {
	suite := []struct {
		description string
		input       []byte
		assert      func(t *testing.T, handler mockHandler)
	}{
		{
			"parse control attribute",
			[]byte{0x1b, '[', '1', 'm'},
			func(t *testing.T, handler mockHandler) {
				assert.Equal(t, &Attr{Type: BoldAttr}, handler.attr)
			},
		},
		{
			"parse_terminal_identity_csi standard",
			[]byte{0x1b, '[', '1', 'c'},
			func(t *testing.T, handler mockHandler) {
				assert.False(t, handler.identityReported)
			},
		},
		{
			"parse_terminal_identity_csi",
			[]byte{0x1b, '[', 'c'},
			func(t *testing.T, handler mockHandler) {
				assert.True(t, handler.identityReported)
			},
		},
		{
			"parse_terminal_identity_csi with 0",
			[]byte{0x1b, '[', '0', 'c'},
			func(t *testing.T, handler mockHandler) {
				assert.True(t, handler.identityReported)
			},
		},
		{
			"parse_terminal_identity_esc",
			[]byte{0x1b, 'Z'},
			func(t *testing.T, handler mockHandler) {
				assert.True(t, handler.identityReported)
			},
		},
		{
			"parse_terminal_identity_esc no skip params",
			[]byte{0x1b, '#', 'Z'},
			func(t *testing.T, handler mockHandler) {
				assert.False(t, handler.identityReported)
			},
		},
		{
			"parse truecolor attr",
			[]byte{
				0x1b, '[', '3', '8', ';', '2', ';', '1', '2', '8', ';', '6', '6', ';',
				'2', '5', '5', 'm',
			},
			func(t *testing.T, handler mockHandler) {
				expected := tcell.NewRGBColor(128, 66, 255)
				assert.Equal(t, &Attr{Type: ForegroundAttr, Color: expected}, handler.attr)
			},
		},
		{
			"parsing ForegroundAttr must not parse blue component also as separate attr",
			[]byte{0x1b, '[', '3', '8', ';', '2', ';', '0', ';', '2', '5', '5', ';', '0', 'm'},
			func(t *testing.T, handler mockHandler) {
				expected := tcell.NewRGBColor(0, 255, 0)
				// The blue component was being parsed as iota's 0 value (ResetAttr) wiping the RGB attr.
				assert.NotEqual(t, &Attr{Type: ResetAttr}, handler.attr)
				assert.Equal(t, &Attr{Type: ForegroundAttr, Color: expected}, handler.attr)
				assert.Len(t, handler.attrs, 1)
			},
		},
		{
			"parsing BackgroundAttr must not parse blue component also as separate attr",
			[]byte{0x1b, '[', '4', '8', ';', '2', ';', '0', ';', '2', '5', '5', ';', '0', 'm'},
			func(t *testing.T, handler mockHandler) {
				expected := tcell.NewRGBColor(0, 255, 0)
				// The blue component was being parsed as iota's 0 value (ResetAttr) wiping the RGB attr.
				assert.NotEqual(t, &Attr{Type: ResetAttr}, handler.attr)
				assert.Equal(t, &Attr{Type: BackgroundAttr, Color: expected}, handler.attr)
				assert.Len(t, handler.attrs, 1)
			},
		},
		{
			"parse designate G0 as line drawing",
			[]byte{0x1b, '(', '0'},
			func(t *testing.T, handler mockHandler) {
				assert.Equal(t, CharsetIndexG0, handler.index)
				assert.Equal(t, StandardCharsetSpecialCharacterAndLineDrawing, handler.charset)
			},
		},
		{
			"parse designate G1 as line drawing and invoke",
			[]byte{0x1b, ')', '0', 0x0e},
			func(t *testing.T, handler mockHandler) {
				assert.Equal(t, CharsetIndexG1, handler.index)
				assert.Equal(t, StandardCharsetSpecialCharacterAndLineDrawing, handler.charset)
			},
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			var handler mockHandler
			handler.init()

			parser := NewParser(&handler, testSyncHandler{})

			for _, b := range test.input {
				parser.Advance(b)
			}

			test.assert(t, handler)
			handler.resetState()
		})
	}
}

var _ Handler = (*mockHandler)(nil)

type mockHandler struct {
	nopHandler
	index            CharsetIndex
	charset          StandardCharset
	attr             *Attr
	attrs            []*Attr
	identityReported bool
}

func (m *mockHandler) init() {
	m.index = CharsetIndexG0
	m.charset = StandardCharsetASCII
	m.attr = nil
	m.attrs = make([]*Attr, 0)
	m.identityReported = false
}

func (m *mockHandler) TerminalAttribute(attr Attr) {
	m.attr = new(Attr)
	*m.attr = attr
	m.attrs = append(m.attrs, &attr)
}

func (m *mockHandler) ConfigureCharset(index CharsetIndex, charset StandardCharset) {
	m.index = index
	m.charset = charset
}

func (m *mockHandler) SetActiveCharset(index CharsetIndex) {
	m.index = index
}

func (m *mockHandler) IdentifyTerminal(identifySecondary bool) {
	m.identityReported = true
}

func (m *mockHandler) resetState() {
	m.init()
}

type testSyncHandler struct {
}

func (t testSyncHandler) SetTimeout(duration time.Duration) {
	panic("unreachable")
}

func (t testSyncHandler) ClearTimeout() {
	panic("unreachable")
}

func (t testSyncHandler) PendingTimeout() bool {
	return false
}
