package main

import (
	"fmt"
	"os"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/editor/vi"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

// IDE binds together a text editor/browser with a plugin manager.
type IDE struct {
	ideConfig
	ex      *editor.Ex
	manager *plugin.Manager
}

// New allocates storage for a new IDE and initializes it with config
// at cfgfilename and filename. Note that if filename is empty, a default inmutable
// buffer will be loaded.
func New(cfgfilename string, filenames ...string) (i *IDE, err error) {
	i = new(IDE)
	err = i.init(cfgfilename, "", filenames...)
	return
}

// NewRecovery allocates storage for a new IDe and initializes in recovery mode.
// The underlying editor will use recfilename to try to recover file at filename.
// Note that this function panics if either filename or recfilename are empty.
func NewRecovery(cfgfilename, filename string, recfilename string) (i *IDE, err error) {
	if filename == "" || recfilename == "" {
		panic(fmt.Sprintf("invalid input: filename='%s', recfilename='%s'",
			filename, recfilename))
	}
	i = new(IDE)
	err = i.init(cfgfilename, recfilename, filename)
	return
}

func (i *IDE) initPlugins() (ret []error) {
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
			ret = append(ret, fmt.Errorf("could not run plugin with id '%s': %v", id, err))
		}
	}

	return ret
}

func (i *IDE) init(cfgfilename, recfilename string, filenames ...string) error {
	configErr := loadConfig(&i.ideConfig, cfgfilename)

	opts := make([]editor.Option, 0)
	viOpts := make([]vi.Option, 0)
	pluginOpts := make([]plugin.Option, 0)

	if recfilename != "" {
		opts = append(opts, editor.WithRecoveryFile(recfilename))
	}

	for _, filename := range filenames {
		opts = append(opts, editor.WithFilepath(filename))
	}

	opts = append(opts,
		editor.WithTabspaces(i.ideConfig.browserTabspaces()),
		editor.WithStartText(i.ideConfig.browserStartText()),
		editor.WithWindowManagerConfig(i.ideConfig.windowManagerConfig()),
		editor.WithFrameUnionCharSet(i.ideConfig.frameUnionCharset()),
		editor.WithCommandEvent(term.Event{Type: term.EventKey, Ch: ':'}),
		editor.WithMessageBarAttr(i.ideConfig.messageBarAttr()),
		editor.WithFocusTabAttr(i.ideConfig.focusTabAttr()),
		editor.WithNonFocusTabAttr(i.ideConfig.nonFocusTabAttr()),
		editor.WithStartTextAttr(i.ideConfig.startTextAttr()),
		editor.WithStartTextBackgroundAttr(i.ideConfig.startTextBackgroundAttr()),
	)

	if i.ideConfig.browserSwapDir() != "" {
		opts = append(opts, editor.WithSwapDir(i.ideConfig.browserSwapDir()))
	}

	viOpts = append(viOpts,
		vi.WithResAttr(term.Attributes{Bg: term.ColorYellow, Fg: term.ColorBlack}),
	)

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
		l.SetFormatter(&log.TextFormatter{
			DisableColors:   true,
			TimestampFormat: time.StampMilli,
		})
		opts = append(opts, editor.WithLogger(l))
		viOpts = append(viOpts, vi.WithLogger(l))
		pluginOpts = append(pluginOpts, plugin.WithLogger(l))
	}

	vi := vi.Editor(viOpts...)
	ex, err := editor.NewEx(vi, opts...)
	if err != nil {
		return err
	}
	i.ex = ex

	res := plugin.BrowserResources(i.ex.Browser())
	res = plugin.MergeResourceMap(res, plugin.EditorResources(i.ex.Editor()))
	i.manager, err = plugin.NewManager(plugin.GrantAll(res), pluginOpts...)
	if err != nil {
		return fmt.Errorf("error initializing plugin manager: %v", err)
	}
	plugErrs := i.initPlugins()

	err = tui.Init()
	if err != nil {
		return err
	}

	term.SetOutputMode(i.ideConfig.outputMode())
	term.SetInputMode(i.ideConfig.inputMode())

	reportNonFatalErrs(i.ex.Browser(), l, configErr, i.ideConfig.errors, plugErrs)
	return nil
}

func reportNonFatalErrs(
	b browser.Browser, l *log.Logger, configErr error,
	configErrs map[string]error, plugErrs []error,
) {
	if l != nil {
		for key, err := range configErrs {
			l.Warnf("error with config %s: %v", key, err)
		}
		for _, err := range plugErrs {
			l.Errorf("failed to run plugin: %#v", err)
		}
		if configErr != nil {
			l.Errorf("failed to load configuration: %v", configErr)
		}
	}

	// it's good UX to report this immediately to the user
	for _, err := range plugErrs {
		b.SetMessage("Error running plugin: %v", err)
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

	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}

	return nil
}

// Close satisfies io.Closer by closing this all IDE's resources, including
// the environment terminal state.
func (i *IDE) Close() error {
	tui.Close()
	return i.closeResources()
}
