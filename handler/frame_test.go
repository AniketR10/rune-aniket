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

package handler

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

func TestFrameProxyMan(t *testing.T) {
	myManual := tui.Manual{
		Summary: "sup",
		Keys: tui.KeyMap{
			term.KeyComb{Ch: 'j'}: {
				ID:          "wow",
				Description: "now",
			},
		},
	}
	handler := &TestHandler{Manual: myManual}
	if !reflect.DeepEqual(handler.Man(), NewFrame(handler).Man()) {
		t.Errorf("did not proxy Man correctly")
	}
}

func TestFrameProxyCursor(t *testing.T) {
	handler := &TestHandler{
		CursorPos:   term.Coordinates{X: 1},
		CursorStyle: term.CursorStyleBlinkingBar,
	}
	proxy := NewFrame(handler)
	proxy.Resize(4, 4)
	offsetCursor, style, ok := handler.Cursor()
	require.True(t, ok)
	offsetCursor.X++
	offsetCursor.Y++

	newCursor, style, ok := proxy.Cursor()
	require.True(t, ok)
	assert.Equal(t, offsetCursor, newCursor)
	assert.Equal(t, term.CursorStyleBlinkingBar, style)
}
