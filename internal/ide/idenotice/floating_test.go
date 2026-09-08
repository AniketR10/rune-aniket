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
