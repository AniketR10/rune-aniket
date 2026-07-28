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

package idenotice

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component/comptest"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestBuildNoticeHandlerDimensionsAddPadding(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "header", content: "# Hi"},
		{name: "paragraph", content: "hi"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			span, err := buildNoticeHandler(tc.content, nopParser{}, syncTick, nopLinkClick)
			require.NoError(t, err)
			require.NotNil(t, span)

			innerW, innerH := span.Content().(interface {
				Dimensions() (int, int)
			}).Dimensions()
			outerW, outerH := span.Dimensions()
			assert.Equal(t, innerW+2, outerW,
				"span dimensions must add 2 horizontal cells (1 left + 1 right)")
			assert.Equal(t, innerH, outerH,
				"span must not add vertical padding")
		})
	}
}

func TestBuildNoticeHandlerPlainTextLayout(t *testing.T) {
	span, err := buildNoticeHandler("hi", nopParser{}, syncTick, nopLinkClick)
	require.NoError(t, err)

	w := term.NewStringWriter(8, 3)
	comptest.TestComponent(t, span, w, []comptest.TestCase{
		{
			Action: func() { span.Resize(8, 3) },
			Expected: `
 hi     
        
        `,
		},
	})
}

func TestBuildNoticeHandlerMarkdownHeader(t *testing.T) {
	span, err := buildNoticeHandler("# Hello", nopParser{}, syncTick, nopLinkClick)
	require.NoError(t, err)

	w := term.NewStringWriter(12, 3)
	comptest.TestComponent(t, span, w, []comptest.TestCase{
		{
			Action: func() { span.Resize(12, 3) },
			Expected: `
            
  Hello     
            `,
		},
	})
}

func TestBuildNoticeHandlerCenteredInWideCanvas(t *testing.T) {
	span, err := buildNoticeHandler("hi", nopParser{}, syncTick, nopLinkClick)
	require.NoError(t, err)

	w := term.NewStringWriter(20, 1)
	comptest.TestComponent(t, span, w, []comptest.TestCase{
		{
			Action: func() { span.Resize(20, 1) },
			Expected: `
 hi                 `,
		},
	})
}

func TestBuildNoticeHandlerLinkClickInvokesCallback(t *testing.T) {
	var clicked *url.URL
	onClick := func(u *url.URL) bool {
		clicked = u
		return true
	}
	span, err := buildNoticeHandler(
		"[docs](https://docs.rune.build)", nopParser{}, syncTick, onClick)
	require.NoError(t, err)
	span.Resize(40, 3)

	ev := term.Event{
		Type:   term.EventMouse,
		Key:    term.MouseLeft,
		MouseX: 2,
		MouseY: 0,
	}
	_, handled := span.Handle(ev)
	assert.True(t, handled)
	require.NotNil(t, clicked, "onLinkClick must fire for clicked link")
	assert.Equal(t, "https://docs.rune.build", clicked.String())
}
