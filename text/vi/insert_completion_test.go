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

package vi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/handler/handlertest"
)

var _ browserapi.Floating = (*insertCompletionFloating)(nil)

func TestInsertCompletionFloating(t *testing.T) {
	t.Run("render and navigation", func(t *testing.T) {
		f := newInsertCompletionFloating([]string{"alpha", "beta", "gamma"}, 0, nil)
		w, h := f.Dimensions()
		writer := term.NewStringWriter(w, h)
		writer.ForegroundCh = '#'
		handlertest.RunHandlerSequenceWriter(t, writer, f, w, h, []handlertest.SequenceTestCase{
			{InputSequence: "", Expected: " ##### \n ####  \n ##### "},
			{InputSequence: "<down>", Expected: " ##### \n ##### \n ##### "},
			{InputSequence: "<up>", Expected: " ##### \n ####  \n ##### "},
			{InputSequence: "<c-j>", Expected: " ##### \n ##### \n ##### "},
			{InputSequence: "<c-k>", Expected: " ##### \n ####  \n ##### "},
			{InputSequence: "<up>", Expected: " ##### \n ####  \n ##### "},
			{InputSequence: "<down>", Expected: " ##### \n ##### \n ##### "},
			{InputSequence: "<down>", Expected: " ##### \n ####  \n ##### "},
			{InputSequence: "<down>", Expected: " ##### \n ####  \n ##### "},
		})
	})

	t.Run("handle returns", func(t *testing.T) {
		tests := []struct {
			name        string
			event       term.Event
			wantExit    bool
			wantHandled bool
		}{
			{
				name:        "esc exits",
				event:       term.Event{Type: term.EventKey, Key: term.KeyEsc},
				wantExit:    true,
				wantHandled: true,
			},
			{
				name:        "enter exits",
				event:       term.Event{Type: term.EventKey, Key: term.KeyEnter},
				wantExit:    true,
				wantHandled: true,
			},
			{
				name:        "tab exits",
				event:       term.Event{Type: term.EventKey, Key: term.KeyTab},
				wantExit:    true,
				wantHandled: true,
			},
			{
				name:        "down is handled",
				event:       term.Event{Type: term.EventKey, Key: term.KeyArrowDown},
				wantExit:    false,
				wantHandled: true,
			},
			{
				name:        "up is handled",
				event:       term.Event{Type: term.EventKey, Key: term.KeyArrowUp},
				wantExit:    false,
				wantHandled: true,
			},
			{
				name:        "ctrl+j is handled",
				event:       term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'j'},
				wantExit:    false,
				wantHandled: true,
			},
			{
				name:        "ctrl+k is handled",
				event:       term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'k'},
				wantExit:    false,
				wantHandled: true,
			},
			{
				name:        "unrelated key not handled",
				event:       term.Event{Type: term.EventKey, Ch: 'x'},
				wantExit:    false,
				wantHandled: false,
			},
			{
				name:        "mouse event not handled",
				event:       term.Event{Type: term.EventMouse},
				wantExit:    false,
				wantHandled: false,
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				f := newInsertCompletionFloating([]string{"alpha", "beta"}, 0, nil)
				exit, handled := f.Handle(tc.event)
				assert.Equal(t, tc.wantExit, exit)
				assert.Equal(t, tc.wantHandled, handled)
			})
		}
	})

	t.Run("enter applies focused candidate", func(t *testing.T) {
		var applied string
		f := newInsertCompletionFloating([]string{"alpha", "beta", "gamma"}, 0, func(s string) {
			applied = s
		})
		// Focus down to "beta", then press Enter
		f.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		f.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
		assert.Equal(t, "beta", applied)
	})

	t.Run("tab applies focused candidate", func(t *testing.T) {
		var applied string
		f := newInsertCompletionFloating([]string{"alpha", "beta"}, 0, func(s string) {
			applied = s
		})
		f.Handle(term.Event{Type: term.EventKey, Key: term.KeyTab})
		assert.Equal(t, "alpha", applied)
	})

	t.Run("esc does not apply", func(t *testing.T) {
		var applied string
		f := newInsertCompletionFloating([]string{"alpha", "beta"}, 0, func(s string) {
			applied = s
		})
		f.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
		assert.Equal(t, "", applied)
	})

	t.Run("focus offset tracks navigation", func(t *testing.T) {
		f := newInsertCompletionFloating([]string{"alpha", "beta", "gamma"}, 0, nil)
		assert.Equal(t, 0, f.list.FocusOffset())

		f.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		assert.Equal(t, 1, f.list.FocusOffset())

		f.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'j'})
		assert.Equal(t, 2, f.list.FocusOffset())

		f.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowUp})
		assert.Equal(t, 1, f.list.FocusOffset())

		f.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'k'})
		assert.Equal(t, 0, f.list.FocusOffset())
	})

	t.Run("dimensions", func(t *testing.T) {
		f := newInsertCompletionFloating([]string{"ab", "cdef"}, 0, nil)
		w, h := f.Dimensions()
		assert.Equal(t, 6, w) // len("cdef") + 2
		assert.Equal(t, 2, h)
	})
}
