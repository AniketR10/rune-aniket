package main

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/component/search"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

const (
	defaultCommand   = `set -o pipefail; command find -L . -mindepth 1 \( -path '*/\.*' -o -fstype 'sysfs' -o -fstype 'devfs' -o -fstype 'devtmpfs' -o -fstype 'proc' \) -prune -o -type f -print -o -type l -print 2> /dev/null | cut -b3-`
	readerBufferSize = 64 * 1024
)

type fuzzyFinderHandler struct {
	f            browser.ResourceOpener
	p            browser.EventPublisher
	m            browser.Messenger
	invokeWindow browser.Window
	mu           sync.Mutex
	cmdStr       string
	exec         *exec.Cmd
	quitChan     chan struct{}
	height       int
	list         search.List
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

func (h *fuzzyFinderHandler) openFile(file string) {
	buf, err := h.f.Open(file)
	if err != nil {
		merr := h.m.SetMessage("Open: %v", err)
		if merr != nil {
			log.Errorf("error setting message: %v", merr)
		}
		log.Errorf("error opening new file buffer: %v", err)
		return
	}

	err = h.invokeWindow.SetContent(buf)
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

func (h *fuzzyFinderHandler) scanForFiles() {
	h.mu.Lock()
	h.exec = execCommand(h.cmdStr, true)
	exec := h.exec

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
	h.mu.Unlock()

	h.readCommand(out)

	h.mu.Lock()
	defer h.mu.Unlock()

	h.exec = nil
}

func (h *fuzzyFinderHandler) initGrants(
	broker proto.MuxBroker, grants []plugin.Grant,
) (err error) {
	for _, grant := range grants {
		switch grant.Permission {
		case plugin.PermissionBrowserMessenger:
			h.m, err = plugin.Messenger(grant.Token, broker)
		case plugin.PermissionBrowserEventPublisher:
			h.p, err = plugin.EventPublisher(grant.Token, broker)
		case plugin.PermissionBrowserResourceOpener:
			h.f, err = plugin.ResourceOpener(grant.Token, broker)
		}
		if err != nil {
			return
		}
	}
	return
}

func newFuzzyFinderHandler(
	grants []plugin.Grant, broker proto.MuxBroker,
	invokeWindow browser.Window, config plugin.Config,
) (tui.Handler, error) {
	h := new(fuzzyFinderHandler)
	err := h.initGrants(broker, grants)
	if err != nil {
		return nil, err
	}

	h.invokeWindow = invokeWindow

	cmdStr, err := config.GetString("command")
	if err != nil {
		if err != plugin.ErrNotFound {
			log.Printf("failed to load 'command' config: %v", err)
		}
		h.cmdStr = defaultCommand
	} else {
		h.cmdStr = cmdStr
	}
	log.Printf("using file list command: %s", h.cmdStr)

	h.quitChan = make(chan struct{})

	listConfig := h.getListConfig(config)
	h.list.Init(listConfig)

	go h.scanForFiles()

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

	searchBaseAttr, err := config.GetAttributes("search_base_attr")
	if err != nil && err != plugin.ErrNotFound {
		log.Errorf("failed to load 'search_base_attr' from config: %v", err)
	} else if err == nil {
		log.Tracef("loaded 'search_base_attr' from config: %v", searchBaseAttr)
		cfg.SearchBaseAttr = &searchBaseAttr
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
	h.list.Resize(width, height)
}

func (h *fuzzyFinderHandler) Draw(w term.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.list.Draw(w)
}

func (h *fuzzyFinderHandler) Handle(ev term.Event) (exit, handled bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ev.Type != term.EventKey {
		return
	}

	switch ev.Key {
	case term.KeyEnter:
		filename, ok := h.list.Focus()
		if ok {
			handled = true
			exit = true
			h.openFile(string(filename))
		}
	case term.KeyEsc:
		exit = true
	case term.KeyArrowDown:
		handled = h.list.FocusDown()
	case term.KeyArrowUp:
		handled = h.list.FocusUp()
	case term.KeyBackspace:
		fallthrough
	case term.KeyBackspace2:
		handled = h.list.SearchQueryDelete()
	}

	if ev.Ch != 0 {
		h.list.SearchQueryWrite(ev.Ch)
		handled = true
	}

	return
}

func (h *fuzzyFinderHandler) Cursor() (pos term.Coordinates, show bool) {
	return term.Coordinates{X: h.list.SearchQueryLen()}, true
}

func (h *fuzzyFinderHandler) Man() tui.Manual {
	return tui.Manual{}
}

func (h *fuzzyFinderHandler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.exec != nil {
		_ = h.killCommand()
	}
	close(h.quitChan)
	h.list.Close()
	return nil
}
