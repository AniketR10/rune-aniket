package text

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

func (c *Component) openAreYouSurePrompt(file workspaceapi.URI) {
	const (
		yesOpt = "Yes"
		noOpt  = "No"
	)

	msg := fmt.Sprintf(`File %s
has been updated since the last back up was created.
Are you sure you want to recover it
and lose all the new updates?`, file)

	c.comp.Prompt(msg, []string{yesOpt, noOpt},
		[]term.KeyComb{{Ch: 'Y'}, {Ch: 'N'}},
		func(i int, opt string) {

			var h browserapi.Handler
			var err error

			switch opt {
			case yesOpt:
				var swapDir, swapFile workspaceapi.URI
				swapDir, err = c.getSwapDir(file)
				if err == nil {
					swapFile, err = workspace.DefaultSwapFile(swapDir, file)
					if err == nil {
						h, err = c.openFileTab(file, swapFile, false, true)
					}
				}
			case noOpt:
			}
			if h != nil {
				err = c.comp.Focus().SetContent(h)
			}
			if err != nil {
				c.log(log.ErrorLevel, "recovery prompt: %v", err)
				c.Notify(notifications.LevelError, "%v", err)
				return
			}
		})
}

func (c *Component) openRecoveryPrompt(file workspaceapi.URI) {
	const (
		recoverOpt  = "Recover"
		readOnlyOpt = "Open Read-Only"
		editOpt     = "Force Edit"
		skipOpt     = "Skip"
	)

	msg := fmt.Sprintf(`File %s is already
open by another process or
an edit session for this file crashed.`, file)

	c.comp.Prompt(msg, []string{recoverOpt, readOnlyOpt, editOpt, skipOpt},
		[]term.KeyComb{{Ch: 'R'}, {Ch: 'O'}, {Ch: 'E'}, {Ch: 'S'}},
		func(i int, opt string) {

			var h browserapi.Handler
			var err error

			switch opt {
			case recoverOpt:
				var swapDir, swapFile workspaceapi.URI
				swapDir, err = c.getSwapDir(file)
				if err == nil {
					swapFile, err = workspace.DefaultSwapFile(swapDir, file)
					if err == nil {
						h, err = c.RecoverFileTab(file, swapFile, false)
						if err == workspaceapi.ErrStaleData {
							c.openAreYouSurePrompt(file)
							return
						}
					}
				}
			case readOnlyOpt:
				h, err = c.OpenFileTab(file, true)
			case editOpt:
				var swapDir, swapFile workspaceapi.URI
				swapDir, err = c.getSwapDir(file)
				if err == nil {
					swapFile, err = workspace.DefaultSwapFile(swapDir, file)
					if err == nil {
						err = c.workspace.Remove(swapFile.Path())
						if err == nil {
							h, err = c.OpenFileTab(file, false)
						}
					}
				}
			case skipOpt:
			}
			if h != nil {
				err = c.comp.Focus().SetContent(h)
			}
			if err != nil {
				c.log(log.ErrorLevel, "recovery prompt: %v", err)
				c.Notify(notifications.LevelError, "%v", err)
				return
			}
		})
}
