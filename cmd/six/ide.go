package main

import (
	"fmt"
	"os"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/text"
	"github.com/ernestrc/go-tui/text/vi"
	"github.com/ernestrc/go-tui/workspace"
	log "github.com/sirupsen/logrus"
)

// IDE binds together a text editor/browser with a plugin manager.
type IDE struct {
	ideConfig
	ex               *Ex
	manager          *plugin.Manager
	workspaceManager *workspace.Manager
	clipboard        *plugin.ClipboardManager
}

// New allocates storage for a new IDE and initializes it with config
// at cfgfilename and filename. Note that if filename is empty, a default inmutable
// buffer will be loaded.
func New(cwd, cfgfilename string, filenames ...string) (
	i *IDE, err error,
) {
	i = new(IDE)
	err = i.init(cwd, cfgfilename, "", filenames...)
	return
}

// NewRecovery allocates storage for a new IDe and initializes in recovery mode.
// The underlying editor will use recfilename to try to recover file at filename.
// Note that this function panics if either filename or recfilename are empty.
func NewRecovery(
	cwd, cfgfilename, filename string, recfilename string,
) (i *IDE, err error) {
	if filename == "" || recfilename == "" {
		panic(fmt.Sprintf("invalid input: filename='%s', recfilename='%s'",
			filename, recfilename))
	}
	i = new(IDE)
	err = i.init(cwd, cfgfilename, recfilename, filename)
	return
}

func (i *IDE) initPlugins(l *log.Logger) {
	for id, p := range i.ideConfig.plugins() {
		path, ok := p.path()
		if !ok {
			continue
		}
		config, ok := p.config()
		if !ok {
			config = plugin.MapConfig(make(map[string]interface{}))
		}
		err := i.manager.Run(id, path, config)
		if err != nil {
			l.Errorf("failed to run plugin: could not run plugin with id '%s': %v", id, err)
		}
	}
}

func (i *IDE) init(cwd, cfgfilename, recfilename string, filenames ...string) error {
	configErr := loadConfig(&i.ideConfig, cfgfilename)

	var (
		opts          []text.Option
		viOpts        []vi.Option
		pluginOpts    []plugin.Option
		workspaceOpts []workspace.Option
	)

	if recfilename != "" {
		recFile, err := workspace.LocalURI(recfilename)
		if err != nil {
			return err
		}
		opts = append(opts, text.WithRecoveryFile(recFile))
	}

	for _, filename := range filenames {
		file, err := workspace.LocalURI(filename)
		if err != nil {
			return err
		}
		opts = append(opts, text.WithFile(file))
	}

	opts = append(opts,
		text.WithTabspaces(i.ideConfig.browserTabspaces()),
		text.WithStartText(i.ideConfig.browserStartText()),
		text.WithWindowManagerConfig(i.ideConfig.windowManagerConfig()),
		text.WithFrameUnionCharSet(i.ideConfig.frameUnionCharset()),
		text.WithCommandEvent(term.Event{Type: term.EventKey, Ch: ':'}),
		text.WithMessageBarAttr(i.ideConfig.messageBarAttr()),
		text.WithFocusTabAttr(i.ideConfig.focusTabAttr()),
		text.WithNonFocusTabAttr(i.ideConfig.nonFocusTabAttr()),
		text.WithStartTextAttr(i.ideConfig.startTextAttr()),
		text.WithStartTextBackgroundAttr(i.ideConfig.startTextBackgroundAttr()),
		text.WithDirtyTabAttr(i.ideConfig.dirtyTabAttr()),
		text.WithCommandOverlayConfig(i.commandOverlayConfig()),
		text.WithPromptConfig(i.promptConfig()),
	)

	for seq, cmd := range i.ideConfig.commandKeyMappings() {
		if seq.Last != (term.Event{}) {
			opts = append(opts, text.WithCommandSequenceBinding(seq, cmd))
		} else {
			opts = append(opts, text.WithCommandKeyBinding(seq.First, cmd))
		}
	}

	if i.ideConfig.browserSwapDir() != nil {
		opts = append(opts, text.WithSwapDir(*i.ideConfig.browserSwapDir()))
	}

	viOpts = append(viOpts,
		vi.WithResAttr(i.ideConfig.viResultAttr()),
		vi.WithDebug(i.ideConfig.viDebug()),
		vi.WithWrap(i.ideConfig.viWrap()),
	)

	i.clipboard = plugin.NewClipboardManager()
	viOpts = append(viOpts, vi.WithClipboard(i.clipboard))

	var l *log.Logger
	if i.ideConfig.logOutputPath() != "" {
		f, err := os.OpenFile(i.ideConfig.logOutputPath(), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}

		level := i.ideConfig.logLevel()
		plugin.SetLoggingOutput(f)
		plugin.SetLoggingLevel(level)

		l = log.New()
		l.SetOutput(f)
		l.SetLevel(level)
		l.SetFormatter(&logging.LogrusFormatter{})
		opts = append(opts, text.WithLogger(l))
		viOpts = append(viOpts, vi.WithLogger(l))
		pluginOpts = append(pluginOpts, plugin.WithLogger(l))
	}

	for _, key := range i.ideConfig.workspaceSSHPrivateKeys() {
		workspaceOpts = append(workspaceOpts, workspace.WithSSHPrivateKey(key))
	}
	workspaceOpts = append(workspaceOpts,
		workspace.WithSSHTimeout(i.ideConfig.workspaceSSHTimeout()))

	cwdURI, err := workspace.ParseURI(cwd)
	if err != nil {
		return err
	}

	i.workspaceManager, err = workspace.NewManager(cwdURI, workspaceOpts...)
	if err != nil {
		return err
	}

	vi := vi.Editor(viOpts...)
	ex, err := NewEx(vi, i.workspaceManager, opts...)
	if err != nil {
		return err
	}
	i.ex = ex

	res := plugin.BrowserResources(i.ex.Browser())
	res = plugin.MergeResourceMap(res, plugin.EditorResources(i.ex.Editor()))
	res = plugin.MergeResourceMap(res, plugin.WorkspaceResources(i.workspaceManager))
	res[plugin.PermissionClipboard] = i.clipboard

	i.manager, err = plugin.NewManager(plugin.GrantAll(res), pluginOpts...)
	if err != nil {
		return fmt.Errorf("error initializing plugin manager: %v", err)
	}

	go i.initPlugins(l)

	err = tui.Init()
	if err != nil {
		return err
	}

	term.SetOutputMode(i.ideConfig.outputMode())
	term.SetInputMode(i.ideConfig.inputMode())

	reportNonFatalErrs(i.ex.Browser(), l, configErr, i.ideConfig.errors)
	return nil
}

func reportNonFatalErrs(
	b browser.Browser, l *log.Logger, configErr error,
	configErrs map[string]error,
) {
	if l != nil {
		for key, err := range configErrs {
			l.Warnf("error with config %s: %v", key, err)
		}
		if configErr != nil {
			l.Errorf("failed to load configuration: %v", configErr)
		}
	}
	if configErr != nil {
		b.SetMessage("Error loading config: %v", configErr)
	}
}

// Run initialzes the underlying terminal environment and runs
// it with this tui.Handler.
func (i *IDE) Run() error {
	err := tui.RunWithLocker(i.ex, i.manager.ResourceLocker())
	if err != nil {
		return err
	}

	return nil
}

func (i *IDE) closeResources() error {
	err1 := i.manager.Close()
	err2 := i.ex.Close()
	err3 := i.clipboard.Close()
	err4 := i.workspaceManager.Close()

	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	if err3 != nil {
		return err3
	}
	if err4 != nil {
		return err4
	}

	return nil
}

// Close satisfies io.Closer by closing this all IDE's resources, including
// the environment terminal state.
func (i *IDE) Close() error {
	tui.Close()
	return i.closeResources()
}
