package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	browserplugin "unstable.build/go-tui/api/browser/plugin"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceplugin "unstable.build/go-tui/api/workspace/plugin"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
)

const (
	commandSed = "sed"
)

var (
	sedHandlerCommands    = []string{commandSed}
	sedHandlerEvents      = []textapi.EventType{}
	sedHandlerPermissions = []plugin.Permission{
		plugin.Permission(plugin.PermissionEditor),
		plugin.Permission(plugin.PermissionBrowserMessenger),
		plugin.Permission(plugin.PermissionExecute),
	}
)

type sedEditorHandler struct {
	ed   textapi.Editor
	m    browserapi.Messenger
	exec workspaceapi.Executor

	resource     textapi.Handler
	resourceName string

	exit uint32
}

func newSedHandler(
	ed textapi.Editor, grants []plugin.Grant,
	broker proto.MuxBroker, pconfig config.Config,
) (plugutil.CommandEventHandler, error) {
	ret := new(sedEditorHandler)
	ret.ed = ed

	var err error
	for _, grant := range grants {
		switch grant.Permission {
		case plugin.Permission(plugin.PermissionExecute):
			ret.exec, err = workspaceplugin.Executor(grant, broker)
		case plugin.Permission(plugin.PermissionBrowserMessenger):
			ret.m, err = browserplugin.Messenger(grant, broker)
		}
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func (h *sedEditorHandler) execSed(
	command, content string,
) (result string, ret error) {
	waitch := make(chan error)
	var stdout, stderr bytes.Buffer
	cmd := workspaceapi.Cmd{
		Path:    "sed",
		Args:    []string{command},
		Stdin:   strings.NewReader(content),
		Watcher: workspaceapi.ChanWatcher(waitch),
		Stdout:  &stdout,
		Stderr:  &stderr,
	}
	_, err := h.exec.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to start command: %v", err)
	}

	ret = <-waitch
	result = stdout.String()
	stderrContent := stderr.String()
	if ret != nil {
		ret = fmt.Errorf("sed: %v: stderr: %s", ret, stderrContent)
	}
	return
}

func (h *sedEditorHandler) setMessage(msg string, args ...interface{}) {
	err := h.m.SetMessage(msg, args...)
	if err != nil {
		log.Errorf("failed to SetMessage: %v", err)
	}
}

func (h *sedEditorHandler) readHandlerContent(hed textapi.Handler) (int, string, error) {
	view := h.ed.CellView(hed)
	cells, err := view.RawCells()
	if err != nil {
		return 0, "", fmt.Errorf("CellView.RawCells: %v", err)
	}
	return len(cells), cell.CellsToString(cells), nil
}

func (h *sedEditorHandler) writeHandlerContent(
	hed textapi.Handler, rows int, content string,
) error {
	editor := h.ed.CellEditor(hed)
	_, _, _, err := editor.Edit(term.Coordinates{}, term.Coordinates{Y: rows}, content)
	if err != nil {
		return fmt.Errorf("CellEditor.Delete: %v", err)
	}
	return nil
}

func (h *sedEditorHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (bool, error) {
	var start time.Time
	if log.IsLevelEnabled(log.TraceLevel) {
		start = time.Now()
		log.Tracef("HandleCommand(%#v)", cmd)
	}
	switch cmd.Name {
	case commandSed:
		if len(cmd.Args) != 1 {
			err := errors.New("Usage: sed <script>")
			return false, err
		}
		if cmd.Resource == nil {
			return false, errors.New("cannot run sed here")
		}
		rows, content, err := h.readHandlerContent(cmd.Resource)
		if err != nil {
			return false, err
		}
		result, err := h.execSed(cmd.Args[0], content)
		if err != nil {
			return false, err
		}

		err = h.writeHandlerContent(cmd.Resource, rows, result)
		if err != nil {
			return false, err
		}
	}
	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("HandleCommand(%#v) in %s", cmd, time.Since(start))
	}

	return false, nil
}

func (h *sedEditorHandler) Handle(
	ctx context.Context, ev textapi.Event,
) (exit bool) {
	uexit := atomic.LoadUint32(&h.exit)
	exit = uexit != 0
	return
}

func (h *sedEditorHandler) Close() error {
	atomic.StoreUint32(&h.exit, 1)
	return nil
}
