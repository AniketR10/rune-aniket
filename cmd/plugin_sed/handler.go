package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/plugin"
	plugutil "github.com/ernestrc/go-tui/plugin/util"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"

	log "github.com/sirupsen/logrus"
)

const (
	commandSed = "sed"
)

var (
	sedHandlerCommands    = []string{commandSed}
	sedHandlerEvents      = []editor.EventType{}
	sedHandlerPermissions = []plugin.Permission{
		plugin.PermissionEditor,
		plugin.PermissionBrowserMessenger,
	}
)

type sedEditorHandler struct {
	ed editor.Editor
	p  browser.EventPublisher
	m  browser.Messenger

	resource     editor.Handler
	resourceName string

	exit uint32
}

func newSedHandler(
	ed editor.Editor, grants []plugin.Grant,
	broker proto.MuxBroker, pconfig plugin.Config,
) (plugutil.CommandEventHandler, error) {
	ret := new(sedEditorHandler)
	ret.ed = ed

	var err error
	for _, grant := range grants {
		switch grant.Permission {
		case plugin.PermissionBrowserMessenger:
			ret.m, err = plugin.Messenger(grant.Token, broker)
			if err != nil {
				return nil, err
			}
		}
	}

	return ret, nil
}

func (h *sedEditorHandler) execSed(
	command, content string,
) (result string, err error) {
	c := exec.Command("sed", command)

	stdout, err := c.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stdin, err := c.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stderr, err := c.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	err = c.Start()
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

	err = c.Wait()
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

func (h *sedEditorHandler) readHandlerContent(hed editor.Handler) (int, string, error) {
	reader := h.ed.Reader(hed)
	cells, err := reader.RawCells()
	if err != nil {
		return 0, "", fmt.Errorf("Reader.RawCells: %v", err)
	}
	return len(cells), cell.CellsToString(cells), nil
}

func (h *sedEditorHandler) writeHandlerContent(
	hed editor.Handler, rows int, content string,
) error {
	writer := h.ed.Writer(hed)
	_, _, _, err := writer.Delete(term.Coordinates{}, term.Coordinates{Y: rows})
	if err != nil {
		return fmt.Errorf("Writer.Delete: %v", err)
	}
	_, _, err = writer.Insert(term.Coordinates{}, content)
	if err != nil {
		return fmt.Errorf("Writer.Insert: %v", err)
	}
	return nil
}

func (h *sedEditorHandler) HandleCommand(cmd editor.Command) (exit bool) {
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
	ctx context.Context, ev editor.Event,
) (exit bool) {
	uexit := atomic.LoadUint32(&h.exit)
	exit = uexit != 0
	return
}

func (h *sedEditorHandler) Close() error {
	atomic.StoreUint32(&h.exit, 1)
	return nil
}
