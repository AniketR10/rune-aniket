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
	"errors"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

const (
	yesOpt = "Yes"
	noOpt  = "No"
)

var yesNoKeyCombs = []term.KeyComb{{Ch: 'y'}, {Ch: 'n'}}

func (h *workspaceManagerHandler) openRestorePrompt(
	ex *ex,
	workspaceURI workspaceapi.URI,
	cache []file,
) {
	// use the window before prompt was open
	invokeWindow := ex.invokeWindow()

	promptHandler := newOpenRestorePromptHandler(
		h, ex, workspaceURI, invokeWindow, cache, yesOpt, noOpt,
	).(*openRestorePromptHandler)
	promptWindow := ex.comp.Prompt(
		"Do you want to restore the previous session?",
		[]string{yesOpt, noOpt},
		yesNoKeyCombs,
		promptHandler,
	)
	promptHandler.promptWindow = promptWindow
}

func newOpenRestorePromptHandler(
	wm *workspaceManagerHandler,
	ex *ex,
	workspaceURI workspaceapi.URI,
	invokeWindow browser.Window,
	cache []file,
	restoreCwdOption, noRestoreOption string,
) handler.PromptHandler {
	return &openRestorePromptHandler{
		wm:               wm,
		ex:               ex,
		workspaceURI:     workspaceURI,
		invokeWindow:     invokeWindow,
		cache:            cache,
		restoreCwdOption: restoreCwdOption,
		noRestoreOption:  noRestoreOption,
	}
}

type openRestorePromptHandler struct {
	wm               *workspaceManagerHandler
	ex               *ex
	workspaceURI     workspaceapi.URI
	invokeWindow     browser.Window
	promptWindow     browser.Window
	cache            []file
	restoreCwdOption string
	noRestoreOption  string
}

func (h *openRestorePromptHandler) OnSelect(idx int, option string) {
	// close so if invokeWindow is Closed (called from another prompt)
	// Focus() does not return the Prompt window
	if h.promptWindow != nil {
		_ = h.promptWindow.Close()
	}

	var err error

	switch option {
	case h.restoreCwdOption:
		err = h.wm.openPrevSessionFiles(h.ex, h.cache, h.invokeWindow)
	case h.noRestoreOption:
		h.wm.history.resetWorkspaceCache(h.workspaceURI)
	}

	if err != nil {
		_ = h.wm.empty.Browser().Notify(notifications.LevelError, err.Error())
	}
}

func (h *openRestorePromptHandler) OnClose() error {
	return nil
}

func (h *workspaceManagerHandler) openConfirmExitPrompt(ex *ex, hasDirtyFilesOpen bool) {

	promptText := "Are you sure you want to exit?"

	if hasDirtyFilesOpen {
		promptText = "There are open files with changes pending to be written. " +
			promptText
	}

	promptHandler := &openConfirmExitPromptHandler{wm: h}
	promptWindow := ex.comp.Prompt(
		promptText,
		[]string{yesOpt, noOpt},
		yesNoKeyCombs,
		promptHandler,
	)
	promptHandler.promptWindow = promptWindow
}

func (h *workspaceManagerHandler) openCreateWorkspacePrompt(ex *ex, uri workspaceapi.URI) {
	promptText := fmt.Sprintf(
		"workspace with URI %s does not exist. Do you want to create it?",
		uri.String())

	promptHandler := &createWorkspaceHandler{ex: ex, uri: uri, wm: h}
	ex.comp.Prompt(
		promptText,
		[]string{yesOpt, noOpt},
		yesNoKeyCombs,
		promptHandler,
	)
}

type openConfirmExitPromptHandler struct {
	wm           *workspaceManagerHandler
	promptWindow browser.Window
}

func (h *openConfirmExitPromptHandler) OnSelect(
	idx int, option string,
) {
	switch option {
	case yesOpt:
		h.wm.confirmedForceExit = true
		h.wm.publishEvent(term.Event{Type: term.EventNone})
	case noOpt:
		h.wm.shaderRunner.cancel()
		h.wm.confirmedForceExit = false
		h.promptWindow.Close()
	}
}

func (h *openConfirmExitPromptHandler) OnClose() error {
	h.wm.exitPromptOpen = false
	h.wm.shaderRunner.cancel()
	return nil
}

type createWorkspaceHandler struct {
	ex  *ex
	wm  *workspaceManagerHandler
	uri workspaceapi.URI
}

func (h *createWorkspaceHandler) OnSelect(
	idx int, option string,
) {
	switch option {
	case yesOpt:
		path := h.uri.Path()
		// NOTE: if we just use the default workspaceLoader, we might
		// be using the wrong scheme (or on the wrong host). Recursively,
		// attempt to create a workspace on some parent directory of path,
		// and then proceed to MkdirAll from that root.
		parent, err := h.createParentWorkspace(h.uri)
		if err != nil {
			_ = h.ex.Browser().Notify(notifications.LevelError,
				"create parent workspace: %v", err.Error())
			log.Errorf("create parent to mkdirall of %s: %v", path, err)
			return
		}
		err = parent.MkdirAll(path, 0755)
		if err != nil {
			_ = h.ex.Browser().Notify(notifications.LevelError, "mkdirall: %v", err)
			log.Errorf("mkdirall %s: %v", path, err)
			return
		}

		log.Tracef("mkdirall %s: ok", path)

		err = h.wm.addWorkspace(h.uri, "", nil, true, true, -1)
		if err != nil {
			_ = h.ex.Browser().Notify(notifications.LevelError, err.Error())
			log.Errorf("addWorkspace %s: %v", h.uri, err)
			return
		}
		log.Tracef("addWorkspace %s: ok", h.uri)
	case noOpt:
	}
}

func (h *createWorkspaceHandler) OnClose() error {
	return nil
}

func (h *createWorkspaceHandler) createParentWorkspace(uri workspaceapi.URI) (workspaceLoader, error) {
	parent := workspaceapi.Dir(uri)
	if parent.Equal(uri) {
		return nil, errors.New("reached root dir")
	}

	cwd, err := h.wm.workspace.AddWorkspace(h.wm.ctxWithLocker, uri)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		return h.createParentWorkspace(parent)
	}
	return cwd, err
}
