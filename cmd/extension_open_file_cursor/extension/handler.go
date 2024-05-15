package extension

import (
	"context"
	"fmt"
	"time"
	"unicode"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/logging"
	browserapi "unstable.build/go-tui/api/browser"
	browserextension "unstable.build/go-tui/api/browser/extension"
	"unstable.build/go-tui/api/config"
	configextension "unstable.build/go-tui/api/config/extension"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceextension "unstable.build/go-tui/api/workspace/extension"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
)

var (
	commandOpenFileCursor = textapi.CommandManual{
		Name: "openFileUnderCursor",
		Summary: "Edit the file whose name is under the cursor. If the file doesn't exist " +
			"then this command opens a new file.",
	}
)

// Grantee returns this extension's Grantee and the permissions required to run it.
func Grantee() (extension.Grantee, []extension.Permission) {
	return extutil.NewEditorEventHandler(gfHandlerCommands, newGFHandler,
		gfHandlerEvents, gfHandlerPermissions...)
}

var (
	gfHandlerCommands = []textapi.CommandManual{commandOpenFileCursor}
	gfHandlerEvents   = []textapi.EventType{
		textapi.EventTypeOpen,
		textapi.EventTypeClose,
		textapi.EventTypeEdit,
		textapi.EventTypeCursor,
		textapi.EventTypeFocus, // needed in case wrap mode is set
	}
	gfHandlerPermissions = []extension.Permission{
		extension.PermissionFileSystem,
		extension.PermissionConfig,
		extension.PermissionBrowserResourceOpener,
	}
)

type gfEditorHandler struct {
	ed textapi.Editor
	o  browserapi.ResourceOpener
	fs workspaceapi.FileSystem

	tracker extutil.ResourceTracker
}

func newGFHandler(
	ctx context.Context, ed textapi.Editor, grants []extension.Grant,
	broker rpc.MuxBroker, pconfig config.Config,
) (extutil.CommandEventHandler, error) {
	ret := new(gfEditorHandler)
	ret.ed = ed

	var err error
	for _, grant := range grants {
		switch grant.Permission {
		case extension.PermissionFileSystem:
			ret.fs, err = workspaceextension.FileSystem(ctx, grant, broker)
		case extension.PermissionBrowserResourceOpener:
			ret.o, err = browserextension.ResourceOpener(ctx, grant, broker)
		case extension.PermissionConfig:
			cfg, err := configextension.FetchConfig(ctx, grant, broker)
			if err != nil {
				return nil, fmt.Errorf("fetch config: %v", err)
			}
			tabspaces, err := extutil.Tabspaces(cfg)
			if err != nil {
				return nil, fmt.Errorf("get configured tabspaces: %v", err)
			}
			wrap, err := extutil.Wrap(cfg)
			if err != nil {
				return nil, fmt.Errorf("get configured wrap mode: %v", err)
			}
			ret.tracker.Init(tabspaces, wrap)
			ret.log(log.DebugLevel,
				"initialized content tracker with tabspaces: %d and wrap mode: %v",
				tabspaces, wrap)
		}
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func (h *gfEditorHandler) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], error,
) {
	return iterator.FromSlice[string](nil), nil
}

func (h *gfEditorHandler) HandleCommand(ctx context.Context, cmd textapi.Command) (
	exit bool, err error,
) {
	if cmd.Resource == nil {
		return
	}

	switch cmd.Name {
	case commandOpenFileCursor.Name:
		err = h.openFileUnderCursor(cmd.Window, cmd.URI)
	}

	return
}

func (h *gfEditorHandler) Handle(
	ctx context.Context, ev textapi.Event,
) (exit bool) {
	var start time.Time
	if log.IsLevelEnabled(log.TraceLevel) {
		start = time.Now()
		h.log(log.TraceLevel, "handle %v", ev.Type)
	}

	h.tracker.Handle(ctx, ev)

	if log.IsLevelEnabled(log.TraceLevel) {
		h.log(log.TraceLevel, "handle %v in %s", ev.Type, time.Since(start))
	}
	return
}

func (h *gfEditorHandler) Close() error {
	return nil
}

type resource interface {
	TokenAt(term.Coordinates, func(rune) bool) (term.Coordinates, term.Coordinates, string)
	Cursor() term.Coordinates
}

func uriAtCursor(res resource) string {
	_, _, uri := res.TokenAt(res.Cursor(), isAllowedURI)
	return uri
}

func isAllowedURI(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsNumber(r) {
		return true
	}

	switch r {
	case '-', '.', '_', '~', ':', '/', '?', '#', '[', ']', '@',
		'!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', '%':
		return true
	default:
		return false
	}
}

func (h *gfEditorHandler) openFileUnderCursor(win browserapi.Window, uri workspaceapi.URI) error {
	res, ok := h.tracker.Resource(uri)
	if !ok {
		return fmt.Errorf("could not find resource for uri %s", uri.String())
	}
	word := uriAtCursor(res)
	if word == "" {
		return fmt.Errorf("no word under cursor")
	}

	h.log(log.DebugLevel, "attempting to parse word %q under cursor", word)

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

func (h *gfEditorHandler) log(level log.Level, msg string, args ...any) {
	log.WithFields(log.Fields{
		logging.KeyClass: "extension.gfEditorHandler",
	}).Logf(level, msg, args...)
}
