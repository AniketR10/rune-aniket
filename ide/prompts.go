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
