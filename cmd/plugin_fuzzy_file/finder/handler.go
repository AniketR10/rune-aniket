package finder

import (
	"bufio"
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/component/search"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

const (
	readerBufferSize        = 64 * 1024
	defaultStoreTimeout     = 5 * time.Second
	searchHistoryDocumentID = "search-history"
	defaultMaxHistory       = 20
)

func Permissions() []plugin.Permission {
	return []plugin.Permission{
		plugin.PermissionBrowserResourceOpener,
		plugin.PermissionBrowserEventPublisher,
		plugin.PermissionBrowserMessenger,
		plugin.PermissionBrowserStorage,
		plugin.PermissionEditor,
	}
}

type fuzzyFinderHandler struct {
	s            browser.Storage
	f            browser.ResourceOpener
	p            browser.EventPublisher
	m            browser.Messenger
	ed           editor.Editor
	invokeWindow browser.Window
	historyKey   term.Event
	mu           sync.Mutex
	cmdStr       string
	getResource  func(string) (string, term.Coordinates)
	exec         *exec.Cmd
	quitChan     chan struct{}
	height       int
	list         search.List
	listHandler  tui.Handler
	killed       bool

	history struct {
		max     int
		Queries []string
	}
}

func execCommand(command string, setpgid bool) *exec.Cmd {
	shell := os.Getenv("SHELL")
	if len(shell) == 0 {
		shell = "sh"
	}
	return execCommandWith(shell, command, setpgid)
}

// ExecCommandWith executes the given command with the specified shell
func execCommandWith(shell string, command string, setpgid bool) *exec.Cmd {
	cmd := exec.Command(shell, "-c", command)
	if setpgid {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}
	return cmd
}

// KillCommand kills the process for the given command
func (h *fuzzyFinderHandler) killCommand() error {
	return syscall.Kill(-h.exec.Process.Pid, syscall.SIGKILL)
}

func (h *fuzzyFinderHandler) readCommand(src io.Reader) {
	h.mu.Lock()
	datachan := h.list.Push()
	h.mu.Unlock()

	defer close(datachan)

	reader := bufio.NewReaderSize(src, readerBufferSize)
	for {
		data, err := reader.ReadBytes(byte('\n'))
		if len(data) > 0 {
			select {
			case datachan <- data:
			case <-h.quitChan:
				return
			}
		}
		if err != nil {
			break
		}
	}
}

func (h *fuzzyFinderHandler) getSearchHistory() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultStoreTimeout)
	defer cancel()

	err := h.s.Get(ctx, searchHistoryDocumentID, &h.history)
	if err == document.ErrNotFound {
		err = nil
	}
	if err == nil {
		log.Debugf("retrieved query history; %#v", h.history.Queries)
	}

	return err
}

func (h *fuzzyFinderHandler) addSearchHistory(searchQuery string) {
	if h.s == nil {
		log.Debugf("Storage permission not granted; ignoring history feature")
		return
	}

	h.history.Queries = append(h.history.Queries, searchQuery)
	if len(h.history.Queries) > h.history.max {
		h.history.Queries = h.history.Queries[1:]
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultStoreTimeout)
	defer cancel()

	h.mu.Unlock()
	defer h.mu.Lock()

	err := h.s.Set(ctx, searchHistoryDocumentID, &h.history)
	if err != nil {
		log.Errorf("error adding search history: %v", err)
	} else {
		log.Debugf("added %s to query history", searchQuery)
	}
}

func (h *fuzzyFinderHandler) open(resource string) (browser.Handler, error) {
	h.mu.Unlock()
	defer h.mu.Lock()
	return h.f.Open(resource)
}

func (h *fuzzyFinderHandler) setContent(name string, b browser.Handler, pos term.Coordinates) error {
	h.mu.Unlock()
	defer h.mu.Lock()
	err := h.invokeWindow.SetContent(b)
	if err != nil {
		return err
	}
	if h.ed == nil {
		log.Info("could not set cursor position because host did not grant plugin.PermissionEditor")
		return nil
	}

	hed, err := h.ed.Editor(name)
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
	resource, pos := h.getResource(data)
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

func (h *fuzzyFinderHandler) publishInterrupt() {
	err := h.p.PublishInterrupt()
	if err != nil {
		log.Printf("failed to publish interrupt: %v", err)
	}
}

func (h *fuzzyFinderHandler) scanData() {
	h.mu.Lock()
	h.exec = execCommand(h.cmdStr, true)
	exec := h.exec
	h.mu.Unlock()

	out, err := exec.StdoutPipe()
	if err != nil {
		log.Errorf("command stdout failed; %v", err)
		return
	}
	err = exec.Start()
	if err != nil {
		log.Errorf("command start failed; %v", err)
		return
	}

	h.readCommand(out)

	err = exec.Wait()

	h.mu.Lock()
	defer h.mu.Unlock()

	killed := h.killed
	h.exec = nil

	if !killed && err != nil {
		merr := h.setMessage("failed to execute '%s': %v", h.cmdStr, err)
		if merr != nil {
			log.Errorf("error setting message: %v", merr)
		}
	}

}

func (h *fuzzyFinderHandler) initGrants(
	broker proto.MuxBroker, grants []plugin.Grant,
) (err error) {
	for _, grant := range grants {
		switch grant.Permission {
		case plugin.PermissionEditor:
			h.ed, err = plugin.Editor(grant.Token, broker)
		case plugin.PermissionBrowserMessenger:
			h.m, err = plugin.Messenger(grant.Token, broker)
		case plugin.PermissionBrowserEventPublisher:
			h.p, err = plugin.EventPublisher(grant.Token, broker)
		case plugin.PermissionBrowserResourceOpener:
			h.f, err = plugin.ResourceOpener(grant.Token, broker)
		case plugin.PermissionBrowserStorage:
			h.s, err = plugin.Storage(grant.Token, broker)
			if err == nil {
				err = h.getSearchHistory()
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
	invokeWindow browser.Window, config plugin.Config,
	historyKey term.Event, command string,
	getResource func(line string) (string, term.Coordinates),
) (tui.Handler, error) {
	h := new(fuzzyFinderHandler)
	err := h.initGrants(broker, grants)
	if err != nil {
		return nil, err
	}

	h.invokeWindow = invokeWindow
	h.historyKey = historyKey
	h.getResource = getResource
	h.cmdStr = command
	log.Printf("using resource list command: %s", h.cmdStr)

	h.quitChan = make(chan struct{})

	listConfig := h.getListConfig(config)
	h.list.Init(listConfig)
	h.listHandler = search.Handler(&h.list, func(item string) {
		searchQuery := h.list.Buffer().String()
		h.openResource(searchQuery, item)
		h.addSearchHistory(searchQuery)
	})

	h.history.max, err = config.GetInt("history")
	if err != nil {
		if err != plugin.ErrNotFound {
			log.Errorf("failed to load 'history' from config: %v", err)
		}
		h.history.max = defaultMaxHistory
	} else {
		log.Tracef("loaded 'history' from config: %v", h.history.max)
	}

	go h.scanData()

	return h, nil
}

func (h *fuzzyFinderHandler) getListConfig(config plugin.Config) search.ListConfig {
	caseSensitive, err := config.GetBool("case_sensitive")
	if err != nil {
		if err != plugin.ErrNotFound {
			log.Errorf("failed to load 'case_sensitive' from config: %v", err)
		}
		caseSensitive = true
	}
	algoStr, err := config.GetString("algo")
	if err != nil && err != plugin.ErrNotFound {
		log.Errorf("failed to load 'algo' from config: %v", err)
	}
	algo := search.FuzzyMatch
	if algoStr == "equal" {
		algo = search.EqualMatch
	}

	cfg := search.ListConfig{
		Algo:          algo,
		Interrupt:     h.publishInterrupt,
		CaseSensitive: caseSensitive,
	}

	matchedTextAttr, err := config.GetAttributes("match_text_attr")
	if err != nil && err != plugin.ErrNotFound {
		log.Errorf("failed to load 'match_base_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'match_base_attr' from config: %v", matchedTextAttr)
		cfg.MatchedTextAttr = &matchedTextAttr
	}

	countAttr, err := config.GetAttributes("count_attr")
	if err != nil && err != plugin.ErrNotFound {
		log.Errorf("failed to load 'count_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'count_attr' from config: %v", countAttr)
		cfg.CountAttr = &countAttr
	}

	textAttr, err := config.GetAttributes("element_attr")
	if err != nil && err != plugin.ErrNotFound {
		log.Errorf("failed to load 'element_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'element_attr' from config: %v", textAttr)
		cfg.ElementAttr = &textAttr
	}

	focusAttr, err := config.GetAttributes("focus_element_attr")
	if err != nil && err != plugin.ErrNotFound {
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
	h.listHandler.Resize(width, height)
}

func (h *fuzzyFinderHandler) Draw(w term.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.listHandler.Draw(w)
}

func (h *fuzzyFinderHandler) writeLastSearchQuery() {
	if len(h.history.Queries) == 0 {
		log.Debugf("no search queries stored")
		return
	}
	search := h.history.Queries[0]
	h.list.Buffer().Reset()
	h.list.Buffer().WriteString(search)
}

func (h *fuzzyFinderHandler) Handle(ev term.Event) (exit, handled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ev.Type != term.EventKey {
		return
	}

	if ev == h.historyKey {
		h.writeLastSearchQuery()
		handled = true
		return
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

	log.Tracef("fuzzyFinderHandler.Close(): %#v", h.exec)

	if h.exec != nil {
		h.killed = true
		_ = h.killCommand()
	}
	close(h.quitChan)
	h.list.Close()
	return nil
}
