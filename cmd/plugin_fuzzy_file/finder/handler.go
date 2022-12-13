package finder

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	browserplugin "unstable.build/go-tui/api/browser/plugin"
	textapi "unstable.build/go-tui/api/text"
	textplugin "unstable.build/go-tui/api/text/plugin"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceplugin "unstable.build/go-tui/api/workspace/plugin"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/handler/search"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
)

const (
	readerBufferSize    = 64 * 1024
	defaultStoreTimeout = 5 * time.Second
	defaultMaxHistory   = 2000
)

func Permissions() []plugin.Permission {
	return []plugin.Permission{
		plugin.Permission(browserplugin.PermissionBrowserResourceOpener),
		plugin.Permission(browserplugin.PermissionBrowserEventPublisher),
		plugin.Permission(browserplugin.PermissionBrowserMessenger),
		plugin.PermissionStorage,
		plugin.Permission(textplugin.PermissionEditor),
		plugin.Permission(workspaceplugin.PermissionWorkspace),
	}
}

type fuzzyFinderHandler struct {
	s                    document.Service
	f                    browserapi.ResourceOpener
	p                    browserapi.EventPublisher
	m                    browserapi.Messenger
	ed                   textapi.Editor
	workspace            workspaceapi.Workspace
	invokeWindow         browserapi.Window
	historyKey           term.KeyComb
	mu                   sync.Mutex
	cmdStr               string
	getResource          func(workspaceapi.Workspace, string) (workspaceapi.URI, term.Coordinates)
	workspaceFallback    func(workspaceapi.Workspace, context.Context) (iterator.Iterator[string], error)
	pid                  workspaceapi.Pid
	quitChan             chan struct{}
	height               int
	list                 search.List
	background           tui.Component
	listHandler          tui.Handler
	killed               bool
	useWorkspaceFallback bool
	cancelScan           func()

	history search.History
}

func (h *fuzzyFinderHandler) execCommand(command string) (workspaceapi.Pid, error) {
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}
	return h.execCommandWith(shell, command)
}

// ExecCommandWith executes the given command with the specified shell
func (h *fuzzyFinderHandler) execCommandWith(shell string, command string) (workspaceapi.Pid, error) {
	cmd, err := h.workspace.Command(shell, "-c", command)
	if err != nil {
		return 0, fmt.Errorf("failed to create command: %w", err)
	}
	return cmd, nil
}

// KillCommand kills the process for the given command
func (h *fuzzyFinderHandler) killCommand() error {
	return h.workspace.Signal(h.pid, syscall.SIGKILL)
}

func (h *fuzzyFinderHandler) readCommand(ctx context.Context, datachan chan<- []byte, src io.Reader) {
	reader := bufio.NewReaderSize(src, readerBufferSize)
	for {
		data, err := reader.ReadBytes('\n')
		if len(data) > 0 {
			select {
			case datachan <- data[:len(data)-1]:
			case <-ctx.Done():
				return
			case <-h.quitChan:
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Error(err)
			}
			break
		}
	}
}

func (h *fuzzyFinderHandler) addSearchHistory(searchQuery string) {
	if h.s == nil {
		log.Debug("Storage permission not granted; ignoring history feature")
		return
	}

	if searchQuery == "" {
		log.Trace("skipping persisting of empty query")
		return
	}

	h.mu.Unlock()
	defer h.mu.Lock()

	err := h.history.Add(searchQuery)
	if err != nil {
		log.Errorf("error adding search history: %v", err)
	} else {
		log.Debugf("added %q to query history: %v", searchQuery, h.history)
	}
}

func (h *fuzzyFinderHandler) open(resource workspaceapi.URI) (browser.Handler, error) {
	h.mu.Unlock()
	defer h.mu.Lock()
	return h.f.Open(resource)
}

func (h *fuzzyFinderHandler) setContent(
	resource workspaceapi.URI, b browser.Handler, pos term.Coordinates,
) error {
	h.mu.Unlock()
	defer h.mu.Lock()
	err := h.invokeWindow.SetContent(b)
	if err != nil && !errors.Is(err, browserapi.ErrTabNotFree) {
		return err
	}
	if h.ed == nil {
		log.Info("could not set cursor position because host did not grant textplugin.PermissionEditor")
		return nil
	}

	hed, err := h.ed.Editor(resource)
	if err != nil {
		return err
	}

	return h.ed.SetCursor(hed, pos)
}

func (h *fuzzyFinderHandler) setMessage(msg string, args ...interface{}) error {
	// allow browser messenger permission to be denied
	if h.m == nil {
		return nil
	}

	h.mu.Unlock()
	defer h.mu.Lock()

	return h.m.SetMessage(msg, args...)
}

func (h *fuzzyFinderHandler) openResource(searchQuery, data string) {
	resource, pos := h.getResource(h.workspace, data)
	handler, err := h.open(resource)
	if err != nil {
		merr := h.setMessage("Open: %v", err)
		if merr != nil {
			log.Errorf("error setting message: %v", merr)
		}
		log.Errorf("error opening new resource: %v", err)
		return
	}

	err = h.setContent(resource, handler, pos)
	if err != nil {
		log.Errorf("error SetContent: %v", err)
		return
	}
}

func (h *fuzzyFinderHandler) scanDataViaWorkspaceAPI(
	ctx context.Context, datachan chan<- []byte,
) {
	log.Debugf("using workspace API to get resource iterator")

	it, err := h.workspaceFallback(h.workspace, ctx)
	if err != nil {
		log.Error(err)
		return
	}

	for {
		resource, ok := it.Next()
		if !ok {
			if err := it.Err(); err != nil {
				log.Error(err)
			} else {
				log.Infof("Done iterating over data: EOF")
			}
			break
		}
		select {
		case datachan <- []byte(resource):
		case <-ctx.Done():
			log.Infof("Done iterating over data: %s", ctx.Err())
			return
		case <-h.quitChan:
			log.Infof("Done iterating over data: closed")
			return
		}
	}
}

func (h *fuzzyFinderHandler) scanData() {
	start := time.Now()
	defer func() {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.Debugf("Done iterating over data in %s", time.Since(start))
		}
	}()

	ctx := context.Background()
	ctx, cancelScan := context.WithCancel(ctx)
	defer cancelScan()

	h.mu.Lock()
	h.cancelScan = cancelScan
	datachan := h.list.Push(ctx)
	defer close(datachan)
	h.mu.Unlock()

	if h.useWorkspaceFallback {
		h.scanDataViaWorkspaceAPI(ctx, datachan)
		return
	}

	log.Infof("using resource list command: %s", h.cmdStr)

	exec, err := h.execCommand(h.cmdStr)
	h.mu.Lock()
	h.pid = exec
	h.mu.Unlock()
	if err != nil {
		log.Debug(err)
		h.scanDataViaWorkspaceAPI(ctx, datachan)
		return
	}

	out, err := h.workspace.StdoutPipe(h.pid)
	if err != nil {
		log.Errorf("command stdout failed; %v", err)
		return
	}
	err = h.workspace.Start(h.pid)
	if err != nil {
		log.Errorf("command start failed; %v", err)
		return
	}

	h.readCommand(ctx, datachan, out)

	err = h.workspace.Wait(h.pid)

	h.mu.Lock()
	defer h.mu.Unlock()

	killed := h.killed
	h.pid = 0

	if !killed && err != nil {
		merr := h.setMessage("failed to execute '%s': %v", h.cmdStr, err)
		if merr != nil {
			log.Errorf("error setting message: %v", merr)
		}
	}

}

func (h *fuzzyFinderHandler) initGrants(
	broker proto.MuxBroker, grants []plugin.Grant,
	historyDocumentID string, maxHistory int,
) (err error) {
	for _, grant := range grants {
		switch grant.Permission {
		case plugin.Permission(workspaceplugin.PermissionWorkspace):
			h.workspace, err = workspaceplugin.Workspace(grant.Token, broker)
		case plugin.Permission(textplugin.PermissionEditor):
			h.ed, err = textplugin.Editor(grant.Token, broker)
		case plugin.Permission(browserplugin.PermissionBrowserMessenger):
			h.m, err = browserplugin.Messenger(grant.Token, broker)
		case plugin.Permission(browserplugin.PermissionBrowserEventPublisher):
			h.p, err = browserplugin.EventPublisher(grant.Token, broker)
		case plugin.Permission(browserplugin.PermissionBrowserResourceOpener):
			h.f, err = browserplugin.ResourceOpener(grant.Token, broker)
		case plugin.PermissionStorage:
			h.s, err = plugin.Storage(grant.Token, broker)
			if err == nil {
				h.history.Init(h.s, historyDocumentID, maxHistory)
				err = h.history.Load()
			}
		}
		if err != nil {
			return
		}
	}
	if h.f == nil || h.p == nil {
		log.Fatalf("This plugin cannot function with granted permissions. Exiting now.")
	}
	return
}

// New returns a tui.Handler that employs a search.List
// to interactively search the command's stdout lines.
func New(
	grants []plugin.Grant, broker proto.MuxBroker,
	invokeWindow browserapi.Window, cfg config.Config,
	historyKey term.KeyComb, historyDocumentID string, command string,
	fallback func(workspaceapi.Workspace, context.Context) (iterator.Iterator[string], error),
	getResource func(exec workspaceapi.Workspace, line string) (workspaceapi.URI, term.Coordinates),
) (tui.Handler, error) {
	h := new(fuzzyFinderHandler)
	maxHistory, err := cfg.GetInt("history")
	if err != nil {
		if err != config.ErrNotFound {
			log.Errorf("failed to load 'history' from config: %v", err)
		}
		maxHistory = defaultMaxHistory
	} else {
		log.Tracef("loaded 'history' from config: %v", maxHistory)
	}
	err = h.initGrants(broker, grants, historyDocumentID, maxHistory)
	if err != nil {
		return nil, err
	}

	h.invokeWindow = invokeWindow
	h.historyKey = historyKey
	h.getResource = getResource
	h.cmdStr = command
	h.useWorkspaceFallback = command == ""
	h.workspaceFallback = fallback

	h.quitChan = make(chan struct{})

	listConfig := h.getListConfig(cfg)
	h.list.Init(listConfig)
	h.listHandler = search.Handler(&h.list, func(item string) {
		searchQuery := h.list.Buffer().String()
		h.openResource(searchQuery, item)
		h.addSearchHistory(searchQuery)
	})

	var defCell term.Cell
	if listConfig.ElementAttr != nil {
		defCell.Bg = listConfig.ElementAttr.Bg
		defCell.Fg = listConfig.ElementAttr.Fg
	}
	h.background = component.WithBackground(h.listHandler, defCell)

	log.Debugf("useWorkspaceFallback set to %v", h.useWorkspaceFallback)

	go h.scanData()

	return h, nil
}

func (h *fuzzyFinderHandler) getListConfig(c config.Config) search.ListConfig {
	caseSensitive, err := c.GetBool("case_sensitive")
	if err != nil {
		if err != config.ErrNotFound {
			log.Errorf("failed to load 'case_sensitive' from config: %v", err)
		}
		caseSensitive = true
	}
	algoStr, err := c.GetString("algo")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("failed to load 'algo' from config: %v", err)
	}
	algo := search.FuzzyMatch
	if algoStr == "equal" {
		algo = search.EqualMatch
	} else if algoStr == "contains" {
		algo = search.ContainsMatch
	}

	cfg := search.ListConfig{
		Algo:          algo,
		Interrupter:   h.p,
		CaseSensitive: caseSensitive,
	}

	matchedTextAttr, err := config.GetAttributes(c, "matched_text_attr")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("failed to load 'matched_text_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'matched_text_attr' from config: %v", matchedTextAttr)
		cfg.MatchedTextAttr = &matchedTextAttr
	}

	countAttr, err := config.GetAttributes(c, "count_attr")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("failed to load 'count_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'count_attr' from config: %v", countAttr)
		cfg.CountAttr = &countAttr
	}

	textAttr, err := config.GetAttributes(c, "element_attr")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("failed to load 'element_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'element_attr' from config: %v", textAttr)
		cfg.ElementAttr = &textAttr
	}

	focusAttr, err := config.GetAttributes(c, "focus_element_attr")
	if err != nil && err != config.ErrNotFound {
		log.Errorf("failed to load 'focus_element_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'focus_element_attr' from config: %v", focusAttr)
		cfg.FocusElementAttr = &focusAttr
	}

	return cfg
}

func (h *fuzzyFinderHandler) Resize(width, height int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.height = height
	h.background.Resize(width, height)
}

func (h *fuzzyFinderHandler) Draw(w term.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.background.Draw(w)
}

func (h *fuzzyFinderHandler) writeLastSearchQuery() {
	search := h.history.Next()
	if search == "" {
		log.Debugf("no search queries stored")
		return
	}
	h.list.Buffer().Reset()
	h.list.Buffer().WriteString(search)
}

func (h *fuzzyFinderHandler) Handle(ev term.Event) (exit, handled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ev.Type != term.EventKey {
		return
	}

	if ev.KeyComb() == h.historyKey {
		h.writeLastSearchQuery()
		handled = true
		return
	}

	if ev.KeyComb().Key == term.KeyCtrlC {
		if h.cancelScan != nil {
			h.cancelScan()
		}
		if h.pid != 0 {
			_ = h.killCommand()
		}
	}

	exit, handled = h.listHandler.Handle(ev)

	log.Tracef("fuzzyFinderHandler.Handle(%#v): %v", ev, handled)

	return
}

func (h *fuzzyFinderHandler) Cursor() (pos term.Coordinates, show bool) {
	return h.listHandler.Cursor()
}

func (h *fuzzyFinderHandler) Man() tui.Manual {
	return h.listHandler.Man()
}

func (h *fuzzyFinderHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	log.Tracef("fuzzyFinderHandler.Close(): %#v", h.pid)

	if h.pid != 0 {
		h.killed = true
		_ = h.killCommand()
	}
	close(h.quitChan)
	h.list.Close()
	return nil
}
