package ide

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding/toml"
	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/go-multierror"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/storage"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/vi"
	"unstable.build/go-tui/workspace"
)

const (
	cmdSwitchToWorkspace = "switchToWorkspace"
	cmdCloseWorkspace    = "closeWorkspace"
	cmdReloadWorkspace   = "reloadWorkspace"
	cmdAddWorkspace      = "addWorkspace"
)

var (
	workspaceCommands = map[string]func(*workspaceManagerHandler, ...string) error{
		cmdAddWorkspace:      (*workspaceManagerHandler).commandAddWorkspace,
		cmdCloseWorkspace:    (*workspaceManagerHandler).commandCloseWorkspace,
		cmdReloadWorkspace:   (*workspaceManagerHandler).commandReloadWorkspace,
		cmdSwitchToWorkspace: (*workspaceManagerHandler).commandSwitchToWorkspace,
		"quit":               (*workspaceManagerHandler).commandQuit,
		"forceQuit!":         (*workspaceManagerHandler).commandQuit,
	}
	defaultCommandKey = term.KeyComb{Ch: ':'}
)

type workspaceManagerHandler struct {
	mu               sync.Locker
	exit             bool
	cfg              ideConfig
	ctxWithLocker    context.Context
	storage          document.Service
	workspace        workspace.WorkspaceManager
	publishEvent     func(term.Event) bool
	pluginRunner     PluginsRunner
	sixDir           string
	builtinPlugins   map[string]Plugin
	configFilename   string
	addWorkspacePath bool
	userHome         string
	history          *history

	union          handler.FrameUnion
	bar            handler.Tabs
	focusProxy     handler.Proxy
	width, height  int
	workspaces     []*workspaceHandler
	workspaceCount int
	focus          int
	empty          *ex
}

func newWorkspaceManagerHandler(
	initial workspaceapi.URI, manager workspace.WorkspaceManager,
	cfg ideConfig, recfilename string, filenames []string,
	sixDir string, publishEvent func(term.Event) bool,
	pluginRunner PluginsRunner, locker sync.Locker,
	builtinPlugins map[string]Plugin, configFilename string,
) (*workspaceManagerHandler, error) {
	ret := new(workspaceManagerHandler)

	err := ret.init(initial, manager,
		cfg, recfilename, filenames, sixDir,
		publishEvent, pluginRunner, locker, builtinPlugins,
		configFilename)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (h *workspaceManagerHandler) newEditor(cfg ideConfig) text.Editor {
	viOpts := append([]vi.Option{},
		vi.WithResAttr(cfg.viResultAttr()),
		vi.WithDebug(cfg.viDebug()),
		vi.WithWrap(cfg.viWrap()),
		vi.WithClipboard(cfg.clipboard()),
	)
	return vi.Editor(viOpts...)
}

func (h *workspaceManagerHandler) init(
	cwd workspaceapi.URI, manager workspace.WorkspaceManager,
	cfg ideConfig, recfilename string, filenames []string,
	sixDir string, publishEvent func(term.Event) bool,
	pluginRunner PluginsRunner, locker sync.Locker,
	builtinPlugins map[string]Plugin, configFilename string,
) error {
	h.mu = locker
	h.workspaces = make([]*workspaceHandler, 10)
	h.cfg = cfg
	h.configFilename = configFilename
	h.publishEvent = publishEvent
	h.workspace = manager
	h.sixDir = sixDir
	h.pluginRunner = pluginRunner
	h.builtinPlugins = builtinPlugins
	h.ctxWithLocker = workspace.ContextWithLocker(context.Background(), h.mu)
	storage, err := storage.New(h.ctxWithLocker, sixDir, toml.Marshaler())
	if err != nil {
		storage = document.NewInMemoryService()
		log.Warnf("Could not setup fs-backed storage: %v. Using ephemeral.", err)
	}
	h.storage = storage

	globalOpts := h.textOpts(h.cfg)
	h.empty, _ = newEx(h.newEditor(cfg), nil, h.storage, h.publishEvent, globalOpts...)
	err = h.subscribeAllWorkspaceCommands(h.empty)
	if err != nil {
		return err
	}

	h.bar.Init()
	h.bar.OnClick = h.switchToWorkspace
	h.bar.SetAttr(cfg.focusTabAttr(), cfg.nonFocusTabAttr(),
		cfg.windowFrameAttr(), cfg.windowFrameAttr())
	h.bar.SetFrameCharSet(cfg.windowFrameCharset())
	h.bar.SetBorder(cfg.frame())

	h.union.Init(&h.focusProxy)
	h.union.Attributes = cfg.windowFrameAttr()
	h.union.Frame = cfg.frame()

	charset := cfg.frameUnionCharset()
	h.union.Right = charset.Right
	h.union.Left = charset.Left
	h.union.Top = charset.Top
	h.union.Bottom = charset.Bottom
	h.addWorkspacePath = cfg.workspacePath()
	h.history = newHistory(h.storage)

	// best effort
	user, err := user.Current()
	if err == nil {
		h.userHome = user.HomeDir
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// AddWorkspace is idempotent, so it should be fine to here and later when
	// actually creating the workspace handler.
	tempcwd, err := h.workspace.AddWorkspace(h.ctxWithLocker, cwd)
	if err != nil {
		return fmt.Errorf("Failed to create new workspace for %q: %s", cwd, err)
	}
	var uris []workspaceapi.URI
	for _, filename := range filenames {
		uri, err := tempcwd.URI(filename)
		if err != nil {
			return err
		}
		uris = append(uris, uri)
	}

	shouldRestore := len(uris) == 0
	// TODO err = h.addWorkspace(cwd, recfilename, uris, shouldRestore, h.cfg.autoRestore())
	err = h.addWorkspace(cwd, recfilename, uris, shouldRestore, shouldRestore)
	if err != nil {
		return err
	}
	h.focusProxy.Target = h.focusHandler()
	h.union.UnionBottom(&h.bar, h.barSize())

	return nil
}

func (h *workspaceManagerHandler) initWithRestorePrompt(
	ex *ex,
	workspaceURI workspaceapi.URI,
	cache map[string]file,
) error {
	const (
		restoreCwd = "Yes"
		noRestore  = "No"
	)

	ex.comp.Prompt(
		"Do you want to restore the previous session?",
		[]string{restoreCwd, noRestore},
		[]term.KeyComb{{Ch: 'y'}, {Ch: 'n'}},
		func(i int, option string) {
			var err error

			switch option {
			case restoreCwd:
				for uriStr := range cache {
					uri, uerr := workspaceapi.ParseURI(uriStr)
					if uerr != nil {
						err = multierror.Append(err, uerr)
						continue
					}
					ferr := ex.editFileURI(uri)
					if ferr != nil {
						err = multierror.Append(err, ferr)
					}
				}
			case noRestore:
				h.history.resetWorkspaceCache(workspaceURI)
			}
			if err != nil {
				h.empty.Browser().SetMessage(err.Error())
			}
		},
	)
	return nil
}

func (h *workspaceManagerHandler) subscribeAllWorkspaceCommands(ex *ex) error {
	return h.subscribeCommands(ex, workspaceCommands)
}

func (h *workspaceManagerHandler) subscribeActiveWorkspaceCommands(ex *ex) (ret error) {
	workspaceActiveCommands := map[string]func(*workspaceManagerHandler, ...string) error{
		cmdAddWorkspace:      (*workspaceManagerHandler).commandAddWorkspace,
		cmdCloseWorkspace:    (*workspaceManagerHandler).commandCloseWorkspace,
		cmdReloadWorkspace:   (*workspaceManagerHandler).commandReloadWorkspace,
		cmdSwitchToWorkspace: (*workspaceManagerHandler).commandSwitchToWorkspace,
	}
	return h.subscribeCommands(ex, workspaceActiveCommands)
}

func (h *workspaceManagerHandler) subscribeCommands(
	ex *ex,
	commands map[string]func(*workspaceManagerHandler, ...string) error,
) (ret error) {
	for cmd, fn := range commands {
		fn := fn
		cmd := cmd
		err := ex.comp.SubscribeCommand(cmd,
			text.FuncCommandCompleter(func(ctx context.Context, cmd textapi.Command) (bool, error) {
				return false, fn(h, cmd.Args...)
			}, func(ctx context.Context, args []string) (
				iterator.Iterator[string], string, error,
			) {
				return h.completeCommand(ctx, cmd, args)
			}))
		if err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return ret
}

func (h *workspaceManagerHandler) completeCommand(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], string, error) {
	switch cmd {
	case cmdSwitchToWorkspace:
		if len(args) == 0 {
			nums := [10]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
			return iterator.FromSlice(nums[:]), "", nil
		}
		return iterator.FromSlice[string](nil), "", nil
	default:
		return iterator.FromSlice[string](nil), "", nil
	}
}

func (h *workspaceManagerHandler) focusHandler() tui.Handler {
	if handler := h.workspaces[h.focus]; handler != nil {
		return handler
	}

	return h.empty
}

func (h *workspaceManagerHandler) drawBar() bool {
	return h.workspaceCount > 1 || h.focusHandler() == h.empty
}

func (h *workspaceManagerHandler) barSize() int {
	frame := h.cfg.frame()
	ret := 1
	if frame {
		ret += 2
	}
	return ret
}

func (h *workspaceManagerHandler) makeWorkspaceTabName(
	i int, w *workspaceHandler,
) string {
	if w == nil || !h.addWorkspacePath {
		return strconv.Itoa(i + 1)
	}
	if w.uri.Scheme() == workspace.FileScheme && h.userHome != "" {
		path := w.uri.Path()
		path = strings.ReplaceAll(path, h.userHome, "~")
		return path
	}
	return w.uri.String()
}

func (h *workspaceManagerHandler) Resize(width, height int) {
	h.width, h.height = width, height
	h.bar.RemoveAll()

	drawBar := h.drawBar()
	if drawBar {
		height -= h.barSize()
	}
	var barFocusIdx int
	for i, w := range h.workspaces {
		if w != nil {
			w.Resize(width, height)
			idx := h.bar.Add(h.makeWorkspaceTabName(i, w))
			if i == h.focus {
				barFocusIdx = idx
			}
		} else if i == h.focus {
			idx := h.bar.Add(h.makeWorkspaceTabName(i, w))
			barFocusIdx = idx
		}
	}
	h.bar.SetFocus(barFocusIdx)
	h.empty.Resize(width, height)
	// bar needs to be drawn last so frame union characters
	// are drawn last
	if drawBar {
		h.union.Resize(h.width, h.height)
	}
}

func (h *workspaceManagerHandler) Draw(w term.Writer) {
	target := h.focusHandler()
	h.focusProxy.Target = target
	if h.drawBar() {
		h.union.Draw(w)
	} else {
		target.Draw(w)
	}
}

func (h *workspaceManagerHandler) switchToWorkspace(i int) {
	h.focus = i
	h.focusProxy.Target = h.focusHandler()
	// resize so disappearing bar feature can be implemented
	h.Resize(h.width, h.height)
}

func (h *workspaceManagerHandler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type == term.EventMouse && h.drawBar() && ev.MouseY >= h.height-h.barSize() {
		exit, handled = h.union.Handle(ev)
	} else {
		exit, handled = h.focusHandler().Handle(ev)
	}
	return exit || h.exit, handled
}

func (h *workspaceManagerHandler) Cursor() (pos term.Coordinates, show bool) {
	return h.focusHandler().Cursor()
}

func (h *workspaceManagerHandler) Man() tui.Manual {
	return h.focusHandler().Man()
}

func (h *workspaceManagerHandler) initPlugins(manager plugin.Runner, cfg ideConfig) {
	var wg sync.WaitGroup
	userPlugins := cfg.plugins()

	wg.Add(len(userPlugins) + len(h.builtinPlugins))

	for id, p := range h.builtinPlugins {
		pconfig := p.Config
		if pconfig == nil {
			pconfig = config.MapConfig(make(map[string]interface{}))
		}
		go func(id, path string, config config.Config) {
			defer wg.Done()
			err := manager.Run(id, path, pconfig)
			if err != nil {
				log.Errorf("failed to run built-in plugin with id %q: %v", id, err)
			}
		}(id, p.Path, pconfig)
	}

	for id, p := range userPlugins {
		path, _ := p.path()
		pconfig, ok := p.config()
		if !ok {
			pconfig = config.MapConfig(make(map[string]interface{}))
		}
		go func(id, path string, config config.Config) {
			defer wg.Done()
			err := manager.Run(id, path, pconfig)
			if err != nil {
				log.Errorf("failed to run plugin with id %q: %v", id, err)
			}
		}(id, path, pconfig)
	}

	wg.Wait()
}

func (h *workspaceManagerHandler) textOpts(cfg ideConfig) []text.Option {
	ret := []text.Option{
		text.WithTabspaces(cfg.browserTabspaces()),
		text.WithWindowManagerConfig(cfg.windowManagerConfig()),
		text.WithFrameUnionCharSet(cfg.frameUnionCharset()),
		text.WithCommandKey(cfg.commandKey()),
		text.WithCommandMaxHistory(cfg.commandMaxHistory()),
		text.WithMessageBarAttr(cfg.messageBarAttr()),
		text.WithFocusTabAttr(cfg.focusTabAttr()),
		text.WithNonFocusTabAttr(cfg.nonFocusTabAttr()),
		text.WithWallpaperAttr(cfg.workspaceWallpaperAttr()),
		text.WithWallpaperBackgroundAttr(cfg.workspaceWallpaperBackgroundAttr()),
		text.WithWallpaper(cfg.wallpaper()),
		text.WithDirtyTabAttr(cfg.dirtyTabAttr()),
		text.WithCommandOverlayConfig(cfg.commandOverlayConfig()),
		text.WithCommandAliases(cfg.commandAliases()),
		text.WithPromptConfig(cfg.promptConfig()),
		text.WithInterrupter(term.FuncInterrupter(func() error {
			if !h.publishEvent(term.Event{Type: term.EventInterrupt}) {
				return errEventStreamNotReady
			}
			return nil
		})),
		text.WithSendNone(func() {
			h.publishEvent(term.Event{Type: term.EventNone})
		}),
	}

	for seq, cmd := range cfg.commandKeyMappings() {
		if seq.Last != (term.KeyComb{}) {
			ret = append(ret, text.WithCommandSequenceBinding(seq, cmd))
		} else {
			ret = append(ret, text.WithCommandKeyBinding(seq.First, cmd))
		}
	}

	return ret
}

// we have no conrol over what plugins are defining in configuration;
// it could be secret keys or anything worth stealing for a malicious plugin
// that gets granted plugin.PermissionConfig.
func cleanedPluginConfig(cfg map[string]interface{}) map[string]interface{} {
	m := make(map[string]interface{}, len(cfg))
	for k, v := range cfg {
		if k != "plugins" {
			m[k] = v
		}
	}
	return m
}

func (h *workspaceManagerHandler) addWorkspace(
	uri workspaceapi.URI, recfilename string, filenames []workspaceapi.URI,
	shouldRestore, shouldPromptRestore bool,
) error {
	for i, w := range h.workspaces {
		if w == nil {
			continue
		}
		if w.uri.Equal(uri) {
			h.switchToWorkspace(i)
			return nil
		}
	}
	cwd, err := h.workspace.AddWorkspace(h.ctxWithLocker, uri)
	if err != nil {
		return fmt.Errorf("Failed to create new workspace for %q: %s", uri, err)
	}

	cfg := cloneConfig(h.cfg)

	isConfigErr, configErr := loadWorkspaceConfig(h.configFilename, cwd, uri, &cfg)
	if configErr != nil && !isConfigErr {
		return configErr
	}

	textOpts := h.textOpts(cfg)
	if recfilename != "" {
		recFile, err := cwd.URI(recfilename)
		if err != nil {
			return err
		}
		textOpts = append(textOpts, text.WithRecoveryFile(recFile))
	}

	for _, uri := range filenames {
		textOpts = append(textOpts, text.WithFile(uri))
	}

	// workspace capable of opening URIs other than the workspaceapi.URI
	multicwd := workspace.Multi(h.ctxWithLocker, h.workspace, cwd, uri)
	ex, err := newEx(h.newEditor(cfg), multicwd, h.storage,
		h.publishEvent, textOpts...)
	if err != nil {
		return err
	}
	err = ex.subscribeCommands()
	if err != nil {
		return err
	}
	err = h.subscribeActiveWorkspaceCommands(ex)
	if err != nil {
		return err
	}

	res := plugin.BrowserResources(ex.Browser())
	res = plugin.MergeResourceMap(res, plugin.EditorResources(ex.Editor()))
	res = plugin.MergeResourceMap(res, plugin.WorkspaceResources(cwd))
	// NOTE: plugins that register new schemes will fail for subsequent workspaces
	res = plugin.MergeResourceMap(res, plugin.SchemeManagerResources(h.workspace))
	res = plugin.MergeResourceMap(res, plugin.StorageResources(h.sixDir))
	res = plugin.MergeResourceMap(res, plugin.ConfigResources(
		config.MapConfig(cleanedPluginConfig(cfg.cfg))))

	dataDir := filepath.Join(h.sixDir, ".plugin")
	if err := os.MkdirAll(dataDir, 0777); err != nil {
		return fmt.Errorf("mkdir .plugin: %v", err)
	}
	runner, err := h.pluginRunner.WorkspacePluginsRunner(h.mu, uri, res, dataDir)
	if err != nil {
		return fmt.Errorf("error initializing plugin manager: %v", err)
	}

	h.initPlugins(runner, cfg)

	i, ok := h.nextAvailableWorkspace()
	if !ok {
		return fmt.Errorf("no available workspaces")
	}

	h.workspaces[i] = &workspaceHandler{
		uri:     uri,
		ex:      ex,
		Plugins: runner,
	}
	h.workspaceCount++
	h.switchToWorkspace(i)

	logNonFatalErrs(configErr, cfg.errors)

	prevSessionFiles := h.history.recordAddWorkspace(uri, ex.Editor(), shouldRestore)
	if !shouldRestore || len(prevSessionFiles) == 0 {
		return nil
	}

	if shouldPromptRestore {
		h.initWithRestorePrompt(ex, uri, prevSessionFiles)
		return nil
	}

	for uri := range prevSessionFiles {
		uri, uerr := workspaceapi.ParseURI(uri)
		if uerr != nil {
			err = multierror.Append(err, uerr)
			continue
		}
		ferr := ex.editFileURI(uri)
		if ferr != nil {
			err = multierror.Append(err, ferr)
		}
	}

	return err
}

func (h *workspaceManagerHandler) nextAvailableWorkspace() (idx int, ok bool) {
	for i := h.focus; i >= 0; i++ {
		if h.workspaces[i] == nil {
			ok = true
			idx = i
			return
		}
	}
	for i := 0; i < h.focus; i++ {
		if h.workspaces[i] == nil {
			ok = true
			idx = i
			return
		}
	}
	return
}

func logNonFatalErrs(
	configErr error,
	configErrs map[string]error,
) {
	all := configErr
	for key, err := range configErrs {
		err = fmt.Errorf("Failed to load %q: %v", key, err)
		all = multierr.Append(all, err)
	}
	if all != nil {
		log.Warn(all)
	}
}

func (h *workspaceManagerHandler) commandReloadWorkspace(args ...string) error {
	workspaceURI, _, err := h.closeWorkspace()
	if err != nil {
		return err
	}
	return h.addWorkspace(workspaceURI, "", nil, true, false)
}

func (h *workspaceManagerHandler) commandAddWorkspace(args ...string) error {
	if len(args) == 0 {
		args = append(args, os.TempDir())
	}
	path := args[0]

	// try to use literal URI
	uri, parseErr := workspaceapi.ParseURI(path)
	if parseErr == nil {
		return h.addWorkspace(uri, "", nil, true, true)
	}

	uri, pathErr := workspaceapi.CurrentUserHostURI(path)
	if pathErr != nil {
		err := multierr.Append(pathErr, parseErr)
		return err
	}
	return h.addWorkspace(uri, "", nil, true, true)
}

func (h *workspaceManagerHandler) closeWorkspace() (workspaceapi.URI, []workspaceapi.URI, error) {
	if h.focusHandler() == h.empty {
		return workspaceapi.URI{}, nil, errors.New("workspace tab is empty")
	}

	hm := h.workspaces[h.focus]
	uri := hm.uri
	tabs := hm.ex.comp.Tabs()
	var files []workspaceapi.URI
	for _, tab := range tabs {
		// only reload with tabs that were created by workspace
		_, ok := tab.Closer().(workspace.FlusherCloser)
		uri := tab.URI()
		if ok && uri != (workspaceapi.URI{}) && uri.Scheme() != "" {
			files = append(files, uri)
		}
	}

	h.history.recordCloseWorkspace(uri)
	err := hm.Close()
	if err != nil {
		log.Error(err)
	} else {
		log.Debugf("Closed all workspace resources successfully")
	}

	h.workspaces[h.focus] = nil
	h.workspaceCount--

	for i := h.focus; i >= 0; i-- {
		if h.workspaces[i] != nil {
			h.switchToWorkspace(i)
			return workspaceapi.URI{}, nil, err
		}
	}

	// for resize of current workspace with empty
	h.switchToWorkspace(h.focus)

	return uri, files, nil
}

func (h *workspaceManagerHandler) commandCloseWorkspace(args ...string) error {
	_, _, err := h.closeWorkspace()
	return err
}

func (h *workspaceManagerHandler) commandQuit(args ...string) error {
	h.exit = true
	return nil
}

func (h *workspaceManagerHandler) commandSwitchToWorkspace(args ...string) error {
	if len(args) == 0 {
		return errors.New("invalid arguments. " +
			"Expecting 1 argument with workspace number")
	}
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid workspace: %s", err)
	}
	n-- // UI does not use 0-based indexing
	if n < 0 || n >= len(h.workspaces) {
		return fmt.Errorf("invalid workspace: there's only %d workspaces",
			len(h.workspaces))
	}
	h.switchToWorkspace(n)
	return nil
}

func (h *workspaceManagerHandler) Close() (ret error) {
	// TODO
	//if h.history.dirtyFilesOpen() {
	//}

	for _, hm := range h.workspaces {
		if hm == nil {
			continue
		}
		if err := hm.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	if err := h.empty.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := h.storage.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return
}

type workspaceHandler struct {
	*ex
	uri     workspaceapi.URI
	Plugins plugin.Runner
}

func (hm *workspaceHandler) Close() (ret error) {
	if err := hm.ex.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := hm.Plugins.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return
}
