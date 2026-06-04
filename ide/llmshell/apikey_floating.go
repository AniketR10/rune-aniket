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

package llmshell

import (
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/handler/inputbox"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// keyInputFloating wraps an inputbox.Handler as a single-shot floating
// prompt. onDone fires exactly once with the entered text (or aborted=true
// when dismissed via Esc/Ctrl-C).
type keyInputFloating struct {
	ib     *inputbox.Handler
	label  string
	win    browserapi.Window
	onDone func(key string, aborted bool)
	done   bool
}

var _ browserapi.Floating = (*keyInputFloating)(nil)

func (k *keyInputFloating) Handle(ev term.Event) (bool, bool) {
	isEsc := ev.Type == term.EventKey && ev.Key == term.KeyEsc
	exit, handled := k.ib.Handle(ev)
	if !exit {
		return false, handled
	}
	k.finish(isEsc)
	return true, true
}

func (k *keyInputFloating) finish(aborted bool) {
	if k.done {
		return
	}
	k.done = true
	text := k.ib.Text()
	if k.onDone != nil {
		k.onDone(text, aborted)
	}
}

func (k *keyInputFloating) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return k.ib.Cursor()
}

func (k *keyInputFloating) Selection() (string, bool) { return k.ib.Selection() }

func (k *keyInputFloating) Resize(width, height int) { k.ib.Resize(width, height) }

func (k *keyInputFloating) Draw(w term.Writer) { k.ib.Draw(w) }

func (k *keyInputFloating) Dimensions() (int, int) {
	return k.ib.Dimensions()
}

func (k *keyInputFloating) Close() error {
	k.finish(true)
	return nil
}
