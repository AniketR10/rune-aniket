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
	Config
	browser *editor.Ex
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

func (i *IDE) init(cfgfilename, recfilename string, filenames ...string) error {
	cfg, err := loadYamlConfig(cfgfilename)
	if err != nil {
		return err
	}

	opts := make([]browser.Option, 0)
	viOpts := make([]vi.Option, 0)
	pluginOpts := make([]plugin.Option, 0)

	if recfilename != "" {
		opts = append(opts, browser.WithRecoveryFile(recfilename))
	}

	for _, filename := range filenames {
		opts = append(opts, browser.WithFilepath(filename))
	}

	opts = append(opts,
		browser.WithTabspaces(cfg.Browser.Tabspaces),
		browser.WithStartText(cfg.Browser.StartText),
		// browser.WithWindowManagerConfig(cfg.WindowManager),
		// browser.WithCommandEvent(term.Event{Type: term.EventKey, Ch: ':'}),
	)

	if cfg.Browser.SwapDir != "" {
		opts = append(opts, browser.WithSwapDir(cfg.Browser.SwapDir))
	}

	viOpts = append(viOpts,
		vi.WithResAttr(term.Attributes{Bg: term.ColorYellow, Fg: term.ColorBlack}),
	)

	var l *log.Logger
	if cfg.LogOutputPath != "" {
		f, err := os.OpenFile(cfg.LogOutputPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		level, err := log.ParseLevel(cfg.LogLevel)
		if err != nil {
			return err
		}

		plugin.SetLoggingOutput(f)
		plugin.SetLoggingLevel(level)

		l = log.New()
		l.SetOutput(f)
		l.SetLevel(level)
		l.SetFormatter(&log.TextFormatter{
			DisableColors:   true,
			TimestampFormat: time.StampMilli,
		})
		opts = append(opts, browser.WithLogger(l))
		viOpts = append(viOpts, vi.WithLogger(l))
		pluginOpts = append(pluginOpts, plugin.WithLogger(l))
	}

	vi := vi.Editor(viOpts...)
	i.browser, err = editor.NewEx(vi, opts...)
	if err != nil {
		return err
	}

	res := plugin.BrowserResources(i.browser)
	i.manager = plugin.NewManager(plugin.GrantAll(res), pluginOpts...)

	for id, p := range cfg.Plugins {
		err := i.manager.Run(id, p.Path, plugin.NewConfig(p.Config))
		if err != nil {
			return fmt.Errorf("could not run plugin with id '%s': %v", id, err)
		}
	}
	return nil
}

// Run initialzes the underlying terminal environment and runs
// it with this tui.Handler.
func (i *IDE) Run() error {
	err := tui.Init()
	if err != nil {
		return err
	}

	term.SetOutputMode(strToOutput(i.Config.OutputMode))
	term.SetInputMode(strArrayToInput(i.Config.InputMode))

	err = tui.RunWithLocker(i.browser, i.manager.ResourceLocker())
	if err != nil {
		return err
	}

	return nil
}

func (i *IDE) closeResources() error {
	err1 := i.manager.Close()
	err2 := i.browser.Close()

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
