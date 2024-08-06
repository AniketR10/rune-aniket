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

package ide

import (
	browserapi "unstable.build/go-tui/api/browser"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/term"
)

func (h *workspaceManagerHandler) openRestorePrompt(
	ex *ex,
	workspaceURI workspaceapi.URI,
	cache []file,
) {
	const (
		restoreCwd = "Yes"
		noRestore  = "No"
	)

	// use the window before prompt was open
	invokeWindow := ex.invokeWindow()

	var promptWindow browserapi.Window
	promptWindow = ex.comp.Prompt(
		"Do you want to restore the previous session?",
		[]string{restoreCwd, noRestore},
		[]term.KeyComb{{Ch: 'y'}, {Ch: 'n'}},
		func(i int, option string) {
			// close so if invokeWindow is Closed (called from another prompt)
			// Focus() does not return the Prompt window
			if promptWindow != nil {
				_ = promptWindow.Close()
			}
			var err error

			switch option {
			case restoreCwd:
				err = h.openPrevSessionFiles(ex, cache, invokeWindow)
			case noRestore:
				h.history.resetWorkspaceCache(workspaceURI)
			}
			if err != nil {
				_ = h.empty.Browser().Notify(notifications.LevelError, err.Error())
			}
		},
	)
}

func (h *workspaceManagerHandler) openExitPrompt(ex *ex) {
	const (
		yes = "Yes"
		no  = "No"
	)

	var promptWindow browserapi.Window
	promptWindow = ex.comp.Prompt(
		"There are open files with changes pending to be written. Are you sure you want to exit?",
		[]string{yes, no},
		[]term.KeyComb{{Ch: 'y'}, {Ch: 'n'}},
		func(i int, option string) {
			switch option {
			case yes:
				h.promptForceExit = true
				h.publishEvent(term.Event{Type: term.EventNone})
			case no:
				h.promptForceExit = false
				if promptWindow != nil {
					promptWindow.Close()
				}
			}
		},
	)
}
