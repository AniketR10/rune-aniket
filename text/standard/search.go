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

package standard

import (
	"errors"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/handler/searchbox"
	"unstable.build/go-tui/text"
)

// searchHandler routes the standard editor's find and replace keys to a
// floating searchbox.Box.
type searchHandler struct {
	text.Handler
	box *searchbox.Box
}

func newSearchHandler(
	inner text.Handler, controller *standardHandler, cfg SearchConfig,
) *searchHandler {
	boxConfig := searchbox.Config{
		WindowManager: cfg.WindowManager,
		Editor:        Editor(),
		Title:         "Find / Replace",
		FindKey:       cfg.FindKey,
		ReplaceKey:    cfg.ReplaceKey,
		// ctrl-f is a legacy alias of the find key in text editors; it must
		// not be aliased in terminals, where it belongs to the shell.
		FindKeyAliases:  []term.KeyComb{{Mod: term.ModCtrl, Ch: 'f'}},
		PaddingTop:      1,
		Attr:            cfg.Attr,
		InputAttr:       cfg.InputAttr,
		PlaceholderAttr: cfg.PlaceholderAttr,
		FrameAttr:       cfg.FrameAttr,
		FocusFrameAttr:  cfg.FocusFrameAttr,
		ButtonAttr:      cfg.ButtonAttr,
		ButtonHoverAttr: cfg.ButtonHoverAttr,
		OnOpenError: func(mode searchbox.Mode, err error) {
			what := "find"
			if mode == searchbox.ModeReplace {
				what = "replace"
			}
			controller.log(log.WarnLevel, "open floating %s: %v", what, err)
			_, _ = controller.cfg.notifications.Notify(browserapi.LevelWarn,
				"Unable to open floating %s: %v", what, err)
			if mode == searchbox.ModeFind {
				controller.startFind()
			}
		},
	}
	return &searchHandler{Handler: inner, box: searchbox.New(controller, boxConfig)}
}

func (h *searchHandler) Handle(ev term.Event) (bool, bool) {
	if h.box.HandleKey(ev) {
		return false, true
	}
	return h.Handler.Handle(ev)
}

func (h *searchHandler) Close() error {
	return errors.Join(h.box.Close(), h.Handler.Close())
}
