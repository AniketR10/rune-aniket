package main

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	browserplugin "unstable.build/go-tui/api/browser/plugin"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceplugin "unstable.build/go-tui/api/workspace/plugin"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/plugin"
	plugutil "unstable.build/go-tui/plugin/util"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/text"
)

const (
	commandOpenFileCursor = "openFileUnderCursor"
)

var (
	gfHandlerCommands = []string{commandOpenFileCursor}
	gfHandlerEvents   = []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeClose,
		textapi.EventTypeEdit,
		textapi.EventTypeCursor,
	}
	gfHandlerPermissions = []plugin.Permission{
		plugin.Permission(plugin.PermissionFileSystem),
		plugin.Permission(plugin.PermissionBrowserResourceOpener),
		plugin.Permission(plugin.PermissionBrowserWindowManager),
	}
)

type file struct {
	cell.Buffer
	component.Scroll
	text.Cursor
}

type gfEditorHandler struct {
	ed textapi.Editor
	o  browserapi.ResourceOpener
	wm browserapi.WindowManager
	fs workspaceapi.FileSystem

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
	ed textapi.Editor, grants []plugin.Grant,
	broker proto.MuxBroker, pconfig config.Config,

) (plugutil.CommandEventHandler, error) {
	ret := new(gfEditorHandler)
	ret.ed = ed
	ret.files = make(map[string]*file)

	var err error
	for _, grant := range grants {
		switch grant.Permission {
		case plugin.Permission(plugin.PermissionFileSystem):
			ret.fs, err = workspaceplugin.FileSystem(grant, broker)
		case plugin.Permission(plugin.PermissionBrowserWindowManager):
			ret.wm, err = browserplugin.WindowManager(grant, broker)
		case plugin.Permission(plugin.PermissionBrowserResourceOpener):
			ret.o, err = browserplugin.ResourceOpener(grant, broker)
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

func (h *gfEditorHandler) openFileUnderCursor(win browserapi.Window, uri workspaceapi.URI) error {
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

	uri, err := workspaceapi.ParseURI(word)
	if err != nil {
		uri, err = h.fs.URI(word)
	}
	if err != nil {
		return fmt.Errorf("could not build a URI from word under cursor: %v", err)
	}

	opened, err := h.o.Open(uri)
	if err != nil {
		return fmt.Errorf("could not Open URI: %v", err)
	}

	err = win.SetContent(opened)
	if err != nil && err != browserapi.ErrTabNotFree {
		return err
	}
	return nil
}

func (h *gfEditorHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (
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

func (h *gfEditorHandler) syncBuffers(ev textapi.Event) error {
	switch ev.Type {
	case textapi.EventTypeClose:
		delete(h.files, ev.URI.String())
	case textapi.EventTypeOpen:
		h.files[ev.URI.String()] = newFile(ev.Content)
	case textapi.EventTypeEdit:
		f, ok := h.files[ev.URI.String()]
		if !ok {
			return fmt.Errorf("could not find buffer for file %s", ev.URI.String())
		}
		f.Edit(ev.Start, ev.End, ev.Content)
	case textapi.EventTypeCursor:
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
	ctx context.Context, ev textapi.Event,
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
