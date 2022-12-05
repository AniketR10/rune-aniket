package main

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

const (
	commandOpenFileCursor = "openFileUnderCursor"
)

var (
	gfHandlerCommands = []string{commandOpenFileCursor}
	gfHandlerEvents   = []text.EventType{
		text.EventTypeOpen,
		text.EventTypeClose,
		text.EventTypeEdit,
		text.EventTypeCursor,
	}
	gfHandlerPermissions = []plugin.Permission{
		plugin.PermissionWorkspace,
		plugin.PermissionBrowserResourceOpener,
		plugin.PermissionBrowserWindowManager,
	}
)

type file struct {
	cell.Buffer
	component.Scroll
	text.Cursor
}

type gfEditorHandler struct {
	ed  text.Editor
	o   browser.ResourceOpener
	wm  browser.WindowManager
	cwd workspace.API

	files map[string]*file
}

func newFile(content string) *file {
	f := new(file)
	f.Buffer.Init()
	f.Scroll.Init(&f.Buffer)
	f.Cursor.Init(&f.Scroll)
	f.Buffer.WriteString(content)
	return f
}

func newGFHandler(
	ed text.Editor, grants []plugin.Grant,
	broker proto.MuxBroker, pconfig config.Config,

) (plugutil.CommandEventHandler, error) {
	ret := new(gfEditorHandler)
	ret.ed = ed
	ret.files = make(map[string]*file)

	var err error
	for _, grant := range grants {
		switch grant.Permission {
		case plugin.PermissionWorkspace:
			ret.cwd, err = plugin.Workspace(grant.Token, broker)
		case plugin.PermissionBrowserWindowManager:
			ret.wm, err = plugin.WindowManager(grant.Token, broker)
		case plugin.PermissionBrowserResourceOpener:
			ret.o, err = plugin.ResourceOpener(grant.Token, broker)
		}
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func (f *file) uriAtCursor() string {
	_, _, uri := f.Scroll.TokenAt(f.CursorAtScroll(), func(r rune) bool {
		return (r >= 'A' && r <= 'Z') ||
			(r >= 'a' && r <= 'z') || r == '_' ||
			(r >= '0' && r <= '9') || r == ':' || r == '@' ||
			r == '+' || r == '/' || r == '.' || r == '-'
	})
	return uri
}

func (h *gfEditorHandler) openFileUnderCursor(win browser.Window, uri workspace.URI) error {
	f, ok := h.files[uri.String()]
	if !ok {
		return fmt.Errorf("could not find buffer for file %s", uri.String())
	}

	word := f.uriAtCursor()
	if word == "" {
		err := fmt.Errorf("word under cursor is empty")
		return err
	}

	log.Infof("trying to parse word %q under cursor", word)

	uri, err := workspace.ParseURI(word)
	if err != nil {
		uri, err = h.cwd.URI(word)
	}
	if err != nil {
		return fmt.Errorf("could not build a URI from word under cursor: %v", err)
	}

	opened, err := h.o.Open(uri)
	if err != nil {
		return fmt.Errorf("could not Open URI: %v", err)
	}

	err = win.SetContent(opened)
	if err != nil && err != browser.ErrTabNotFree {
		return err
	}
	return nil
}

func (h *gfEditorHandler) HandleCommand(ctx context.Context, cmd text.Command) (
	exit bool, err error,
) {
	if cmd.Resource == nil {
		return
	}

	switch cmd.Name {
	case commandOpenFileCursor:
		err = h.openFileUnderCursor(cmd.Window, cmd.URI)
	}

	return
}

func (h *gfEditorHandler) syncBuffers(ev text.Event) error {
	switch ev.Type {
	case text.EventTypeClose:
		delete(h.files, ev.URI.String())
	case text.EventTypeOpen:
		h.files[ev.URI.String()] = newFile(ev.Content)
	case text.EventTypeEdit:
		f, ok := h.files[ev.URI.String()]
		if !ok {
			return fmt.Errorf("could not find buffer for file %s", ev.URI.String())
		}
		f.Edit(ev.Start, ev.End, ev.Content)
	case text.EventTypeCursor:
		f, ok := h.files[ev.URI.String()]
		if !ok {
			return fmt.Errorf("could not find buffer for file %s", ev.URI.String())
		}
		before := f.CursorAtScroll()
		log.Tracef("MoveToScroll(%#v): %s", ev.From, f.String())
		_, ok = f.MoveToScroll(ev.From)
		if !ok && ev.From != before {
			return fmt.Errorf("MoveToScroll(%#v): %v", ev.From, ok)
		}
	}
	return nil
}

func (h *gfEditorHandler) Handle(
	ctx context.Context, ev text.Event,
) (exit bool) {
	var start time.Time
	if log.IsLevelEnabled(log.TraceLevel) {
		start = time.Now()
		log.Tracef("Handle(%#v)", ev.Type)
	}

	err := h.syncBuffers(ev)
	if err != nil {
		err = fmt.Errorf("syncBuffers(%v): %s", ev.Type, err)
		log.Error(err)
	}
	if log.IsLevelEnabled(log.TraceLevel) {
		log.Tracef("Handle(%#v) in %s", ev.Type, time.Since(start))
	}
	return
}

func (h *gfEditorHandler) Close() error {
	return nil
}
