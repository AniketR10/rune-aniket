package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sync/atomic"
	"time"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/plugin"
	plugutil "github.com/ernestrc/go-tui/plugin/util"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/text"
	"github.com/ernestrc/go-tui/workspace"

	log "github.com/sirupsen/logrus"
)

const (
	commandSed = "sed"
)

var (
	sedHandlerCommands    = []string{commandSed}
	sedHandlerEvents      = []text.EventType{}
	sedHandlerPermissions = []plugin.Permission{
		plugin.PermissionEditor,
		plugin.PermissionBrowserMessenger,
		plugin.PermissionWorkspace,
	}
)

type sedEditorHandler struct {
	ed   text.Editor
	p    browser.EventPublisher
	m    browser.Messenger
	exec workspace.Workspace

	resource     text.Handler
	resourceName string

	exit uint32
}

func newSedHandler(
	ed text.Editor, grants []plugin.Grant,
	broker proto.MuxBroker, pconfig plugin.Config,
) (plugutil.CommandEventHandler, error) {
	ret := new(sedEditorHandler)
	ret.ed = ed

	var err error
	for _, grant := range grants {
		switch grant.Permission {
		case plugin.PermissionWorkspace:
			ret.exec, err = plugin.Workspace(grant.Token, broker)
		case plugin.PermissionBrowserMessenger:
			ret.m, err = plugin.Messenger(grant.Token, broker)
		}
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func (h *sedEditorHandler) execSed(
	command, content string,
) (result string, err error) {
	c, err := h.exec.Command("sed", command)
	if err != nil {
		return "", fmt.Errorf("failed to create command: %v", err)
	}

	stdout, err := h.exec.StdoutPipe(c)
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stdin, err := h.exec.StdinPipe(c)
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stderr, err := h.exec.StderrPipe(c)
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	err = h.exec.Start(c)
	if err != nil {
		return "", fmt.Errorf("failed to start executable: %v", err)
	}

	_, err = io.WriteString(stdin, content)
	if err != nil {
		return "", fmt.Errorf("failed to write content to sed stdin %v", err)
	}

	err = stdin.Close()
	if err != nil {
		return "", fmt.Errorf("failed to close sed stdin: %v", err)
	}

	resultBytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return "", fmt.Errorf("failed to read from sed stdout: %v", err)
	}

	stderrContent, _ := ioutil.ReadAll(stderr)

	err = h.exec.Wait(c)
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, stderrContent)
	}

	return string(resultBytes), nil
}

func (h *sedEditorHandler) setMessage(msg string, args ...interface{}) {
	err := h.m.SetMessage(msg, args...)
	if err != nil {
		log.Errorf("failed to SetMessage: %v", err)
	}
}

func (h *sedEditorHandler) readHandlerContent(hed text.Handler) (int, string, error) {
	view := h.ed.CellView(hed)
	cells, err := view.RawCells()
	if err != nil {
		return 0, "", fmt.Errorf("Reader.RawCells: %v", err)
	}
	return len(cells), cell.CellsToString(cells), nil
}

func (h *sedEditorHandler) writeHandlerContent(
	hed text.Handler, rows int, content string,
) error {
	editor := h.ed.CellEditor(hed)
	_, _, _, err := editor.Edit(term.Coordinates{}, term.Coordinates{Y: rows}, content)
	if err != nil {
		return fmt.Errorf("Writer.Delete: %v", err)
	}
	return nil
}

func (h *sedEditorHandler) HandleCommand(
	ctx context.Context, cmd text.Command,
) (exit bool) {
	var start time.Time
	if log.IsLevelEnabled(log.TraceLevel) {
		start = time.Now()
		log.Tracef("HandleCommand(%#v)", cmd)
	}
	switch cmd.Name {
	case commandSed:
		if len(cmd.Args) != 1 {
			err := errors.New("Usage: sed <script>")
			log.Error(err)
			h.setMessage(err.Error())
			return
		}
		rows, content, err := h.readHandlerContent(cmd.Resource)
		if err != nil {
			log.Error(err)
			return
		}
		result, err := h.execSed(cmd.Args[0], content)
		if err != nil {
			log.Error(err)
			h.setMessage("sed: %v", err.Error())
			return
		}

		err = h.writeHandlerContent(cmd.Resource, rows, result)
		if err != nil {
			log.Error(err)
			return
		}
	}
	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("HandleCommand(%#v) in %s", cmd, time.Since(start))
	}

	return false
}

func (h *sedEditorHandler) Handle(
	ctx context.Context, ev text.Event,
) (exit bool) {
	uexit := atomic.LoadUint32(&h.exit)
	exit = uexit != 0
	return
}

func (h *sedEditorHandler) Close() error {
	atomic.StoreUint32(&h.exit, 1)
	return nil
}
