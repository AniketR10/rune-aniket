package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sync"
	"time"

	"github.com/ernestrc/blue/retry"
	multierr "github.com/ernestrc/go-multierror"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	browserplugin "unstable.build/go-tui/api/browser/plugin"
	"unstable.build/go-tui/api/config"
	configplugin "unstable.build/go-tui/api/config/plugin"
	textapi "unstable.build/go-tui/api/text"
	textplugin "unstable.build/go-tui/api/text/plugin"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/search"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
)

const (
	cmdLogs = "logs"
)

var (
	requiredPermissions = []plugin.Permission{
		plugin.Permission(plugin.PermissionBrowserWindowManager),
		plugin.Permission(plugin.PermissionBrowserEventPublisher),
		plugin.Permission(plugin.PermissionBrowserMessenger),
		plugin.PermissionConfig,
		plugin.Permission(plugin.PermissionEditor),
	}
	commands = []string{
		cmdLogs,
	}

	retryStrategy = retry.ExponentialStrategy(10*time.Millisecond, 1*time.Second)
)

type logsGrantee struct {
	mu     sync.Mutex
	broker proto.MuxBroker
	quitCh chan struct{}

	wm browserapi.WindowManager
	m  browserapi.Messenger
	p  browserapi.EventPublisher
	c  config.Config

	logFile string
	cfg     search.ListConfig

	discardedLogs bool
}

func (e *logsGrantee) Connected(broker proto.MuxBroker, pconfig config.Config) {
	e.mu.Lock()
	defer e.mu.Unlock()

	log.Infof("plugin connected; config: %#v", pconfig)
	e.broker = broker

	caseSensitive, err := pconfig.GetBool("case_sensitive")
	if err != nil {
		if err != config.ErrNotFound {
			log.Errorf("failed to load 'case_sensitive' from config: %v", err)
		}
		caseSensitive = true
	}

	debug, err := pconfig.GetBool("debug")
	if err != nil {
		if err != config.ErrNotFound {
			log.Errorf("failed to load 'case_sensitive' from config: %v", err)
		}
	} else {
		log.Debugf("set debug to %v", debug)
	}

	// set to true to avoid discarding all logs
	e.discardedLogs = debug

	algoStr, err := pconfig.GetString("algo")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("failed to load 'algo' from config: %v", err)
	}
	algo := search.FuzzyMatch
	if algoStr == "equal" {
		algo = search.EqualMatch
	} else if algoStr == "contains" {
		algo = search.ContainsMatch
	}

	e.cfg = search.ListConfig{
		Algo:            algo,
		Interrupter:     e.p,
		CaseSensitive:   caseSensitive,
		BottomSearchBar: true,
	}

	matchedTextAttr, err := config.GetAttributes(pconfig, "matched_text_attr")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("failed to load 'matched_text_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'matched_text_attr' from config: %v", matchedTextAttr)
		e.cfg.MatchedTextAttr = &matchedTextAttr
	}

	countAttr, err := config.GetAttributes(pconfig, "count_attr")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("failed to load 'count_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'count_attr' from config: %v", countAttr)
		e.cfg.CountAttr = &countAttr
	}

	textAttr, err := config.GetAttributes(pconfig, "element_attr")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("failed to load 'element_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'element_attr' from config: %v", textAttr)
		e.cfg.ElementAttr = &textAttr
	}

	focusAttr, err := config.GetAttributes(pconfig, "focus_element_attr")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("failed to load 'focus_element_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'focus_element_attr' from config: %v", focusAttr)
		e.cfg.FocusElementAttr = &focusAttr
	} else if err == config.ErrNotFound {
		e.cfg.FocusElementAttr = &defaultPinAttr
	}

}

func (e *logsGrantee) PermissionGranted(grants []plugin.Grant) {
	log.Infof("permissions granted: %v", grants)

	var err error
	for _, g := range grants {
		switch g.Permission {
		case plugin.Permission(plugin.PermissionBrowserMessenger):
			e.m, err = browserplugin.Messenger(g, e.broker)
		case plugin.Permission(plugin.PermissionBrowserEventPublisher):
			e.p, err = browserplugin.EventPublisher(g, e.broker)
		case plugin.Permission(plugin.PermissionBrowserWindowManager):
			e.wm, err = browserplugin.WindowManager(g, e.broker)
		case plugin.PermissionConfig:
			e.c, err = configplugin.FetchConfig(g, e.broker)
		case plugin.Permission(plugin.PermissionEditor):
			var ed textapi.Editor
			ed, err = textplugin.Editor(g, e.broker)
			if err == nil {
				for _, cmd := range commands {
					subsErr := ed.SubscribeCommand(cmd, e)
					if subsErr != nil {
						err = multierr.Append(err, subsErr)
					}
				}
			}
		}
		if err != nil {
			log.Fatalf("PermissionGranted: %+v: %s", g.Permission, err)
		}
	}

	e.logFile, err = e.c.GetString("log_path")
	if err != nil {
		log.Fatalf("Could not get 'log_path' from config: %s", err)
	}
}

func (e *logsGrantee) PermissionDenied(perms []plugin.Permission) {
	log.Fatalf("Could not start plugin due to missing permissions: "+
		"denied: %v; required: %v", perms, requiredPermissions)
}

func (e *logsGrantee) Shutdown(reason string) error {
	log.Debugf("plugin being shutdown: %s", reason)
	close(e.quitCh)
	return nil
}

func (e *logsGrantee) Health() error {
	return nil
}

func consumeAvailableData(
	reader *bufio.Reader, l *search.List,
	ch chan<- []byte, quit chan struct{},
) error {
	for {
		data, err := reader.ReadBytes('\n')
		if len(data) != 0 {
			select {
			case ch <- data:
			case <-quit:
				return nil
			}
		}
		if err == io.EOF {
			l.Pause()
			return nil
		}
		if err != nil {
			log.Warnf("read error: %s", err)
			return err
		}
	}
}

func consumeData(
	ctx context.Context, file workspaceapi.File, l *search.List,
	quit chan struct{}, watcher *fsnotify.Watcher,
) {
	reader := bufio.NewReader(file)

	ch := l.Push(ctx)
	defer close(ch)

	if err := consumeAvailableData(reader, l, ch, quit); err != nil {
		log.Warnf("error reading initial data on log file: %s", err)
	}

	for {
		// retry reads and collect watcher errors
		err := retry.Retry(ctx, retryStrategy, func(ctx context.Context) (bool, error) {
			select {
			case ev := <-watcher.Events:
				switch ev.Op {
				case fsnotify.Write:
					if err := consumeAvailableData(reader, l, ch, quit); err != nil {
						return true, fmt.Errorf("read error: %s", err)
					}
				}
				return false, nil
			case err, ok := <-watcher.Errors:
				if !ok {
					return false, nil
				}
				return true, fmt.Errorf("notify error: %s", err)
			case <-quit:
				return false, nil
			}
		})

		select {
		case <-quit:
			return
		case <-ctx.Done():
			return
		default:
			if err != nil {
				log.Warn(err)
			}
		}
	}
}

func (e *logsGrantee) showLogs(win browserapi.Window, args []string) (bool, error) {
	if log.IsLevelEnabled(log.TraceLevel) && len(args) == 0 {
		return false, errors.New("Cannot show own logs in Trace level to avoid " +
			"an infinite loop. Check Manually.")
	}
	if e.logFile == "" && len(args) == 0 {
		return false, errors.New("Cannot show logs if 'log_path' in config is empty " +
			"and no arguments were supplied to 'logs' command.")
	}

	logFile := e.logFile
	if len(args) != 0 {
		logFile = args[0]
	}

	log.Debugf("Opening log file %q", logFile)

	uri, err := workspaceapi.CurrentUserHostURI(logFile)
	if err != nil {
		return false, err
	}

	// NOTE: log_path is always referencing a local path so until
	// we containerize plugins, it's safe to call os.Open
	file, err := workspace.OpenFile(logFile, os.O_RDONLY, 0)
	if err != nil {
		return false, fmt.Errorf("could not open logs file: %s", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		_ = file.Close()
		return false, fmt.Errorf("notify: %s", err)
	}

	err = watcher.Add(file.Name())
	if err != nil {
		_ = watcher.Close()
		_ = file.Close()
		return false, fmt.Errorf("notify.Add: %s", err)
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	l := search.NewList(e.cfg)

	cleanup := func() (err error) {
		cancel()
		if cerr := file.Close(); cerr != nil {
			err = multierr.Append(err, cerr)
		}
		if cerr := watcher.Close(); cerr != nil {
			err = multierr.Append(err, cerr)
		}
		if cerr := l.Close(); cerr != nil {
			err = multierr.Append(err, cerr)
		}
		return
	}

	logsHandler := handler.Sync(new(sync.Mutex), newLogsHandler(l, e.cfg.ElementAttr,
		e.cfg.MatchedTextAttr, e.cfg.FocusElementAttr))
	h := browserapi.FuncHandler(logsHandler, cleanup)

	go consumeData(ctx, file, l, e.quitCh, watcher)

	t, err := e.wm.Tab(uri, "logs:"+uri.Name(), h)
	if err != nil {
		_ = cleanup()
		return false, fmt.Errorf("Tab: %s", err)
	}

	log.Tracef("HandleCommand: created new logs handler: %p", h)

	err = win.SetContent(t)
	if err != nil {
		_ = cleanup()
		_ = t.Close()
		return false, err
	}

	return false, nil
}

func (e *logsGrantee) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (bool, error) {
	if cmd.Name != cmdLogs {
		panic("extraneous command")
	}

	if !e.discardedLogs {
		// disable all logs to avoid creating infinite I/O loops
		log.SetOutput(io.Discard)
		log.SetLevel(log.FatalLevel)
		plugin.SetLoggingOutput(io.Discard)
		plugin.SetLoggingLevel(log.FatalLevel)
		e.discardedLogs = true
	}

	return e.showLogs(cmd.Window, cmd.Args)
}

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6068", nil))
	}()

	s := logsGrantee{quitCh: make(chan struct{})}
	plugin.Serve(&s, requiredPermissions...)
}
