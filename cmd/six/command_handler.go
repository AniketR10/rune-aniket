package main

import (
	"context"
	"math"
	"strings"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler/search"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

type commandHandlerMode uint

const (
	modeCommandHandlerCommand commandHandlerMode = iota
	// modeCommandHandlerArgs1
	// modeCommandHandlerArgs2
	// ...
)

type commandListHandler struct {
	mode         commandHandlerMode
	commandKey   term.KeyComb
	overlayCfg   text.CommandOverlayConfig
	dispatchFunc func(string, ...string) bool
	completeFunc func(context.Context, string, ...string) iterator.Iterator[string]

	height, width int
	// overlay buffer over the search list so we can
	// stop the search for multiple argument commands
	// but we can display arguments
	buf        cell.Buffer
	responsive component.Responsive
	list       search.List
	history    search.History

	commandAndArgs []string
	commandsBackup []string

	// used to signal across Handle calls that user
	// is cyclying through commands, in particular
	// when command key is a character that could be interpreted
	// as a character to be inserted in command input buffer
	prevCommandCycle bool
}

func newCommandListHandler(
	b document.Service, max int, overlayCfg text.CommandOverlayConfig,
	commandKey term.KeyComb,
	completeFunc func(context.Context, string, ...string) iterator.Iterator[string],
	dispatchFunc func(string, ...string) bool,
	interrupt func(),
) *commandListHandler {
	ret := new(commandListHandler)
	ret.init(b, max, overlayCfg, commandKey,
		completeFunc, dispatchFunc, interrupt)
	return ret
}

func (h *commandListHandler) init(
	store document.Service, max int, overlayCfg text.CommandOverlayConfig,
	commandKey term.KeyComb,
	completeFunc func(context.Context, string, ...string) iterator.Iterator[string],
	dispatchFunc func(string, ...string) bool,
	interrupt func(),
) {
	h.mode = modeCommandHandlerCommand
	h.commandKey = commandKey
	h.overlayCfg = overlayCfg
	h.dispatchFunc = dispatchFunc
	h.completeFunc = completeFunc

	h.buf.Init()
	h.responsive = component.BufferResponsive(&h.buf,
		component.StringConfig{
			Attributes:           overlayCfg.ElementAttr,
			BackgroundAttributes: overlayCfg.ElementAttr,
		})

	cfg := search.ListConfig{
		Algo:             search.FuzzyMatch,
		Interrupt:        interrupt,
		CaseSensitive:    false,
		MatchedTextAttr:  &overlayCfg.MatchedTextAttr,
		FocusElementAttr: &overlayCfg.FocusElementAttr,
		ElementAttr:      &overlayCfg.ElementAttr,
	}

	h.list.Init(cfg)

	for {
		h.history.Init(store, commandHistoryDocumentID, max)
		err := h.history.Load()
		if err == nil {
			break
		}
		h.log(log.ErrorLevel, "load history: %v", err)
		store = document.NewInMemoryService()
	}
}

func (h *commandListHandler) commandOverlayDimensions() (width, height int) {
	width = int(math.Min(float64(h.width), float64(h.overlayCfg.Width)))
	height = int(math.Min(float64(h.height), float64(h.overlayCfg.Height)))
	return
}

func (h *commandListHandler) resizeCommandOverlay() {
	// propagate local cmd+args buffer height to
	// search list, which only has cmd, in case args alone span
	// multiple lines
	cmdWidth, cmdHeight := h.commandOverlayDimensions()
	height := h.responsive.Height(cmdWidth)
	// set to min 1, as it's being used as input field
	// and max to the height of the overlayed component
	height = int(math.Min(math.Max(1, float64(height)), float64(cmdHeight)))
	// the list is very short so waiting is not a significant
	// perf penalty and it makes tests easier to make deterministic
	h.list.SetMinInputHeight(height)
	h.responsive.Resize(cmdWidth, height)
}

func (h *commandListHandler) Resize(width, height int) {
	h.width = width
	h.height = height
	h.list.Resize(width, height)
}

func (h *commandListHandler) Draw(w term.Writer) {
	// resize on every draw because search.List uses a responsive
	// input so local buffer changes must consider potential resize
	// of search.List
	h.resizeCommandOverlay()
	// the list is very short so waiting is not a significant
	// perf penalty and it makes tests easier to make deterministic
	h.list.Draw(w)
	h.responsive.Draw(w)
}

func (h *commandListHandler) handleLastCommand() {
	cmd := h.history.Next()
	if cmd == "" {
		h.log(log.TraceLevel, "ignoring next command in history: empty")
		return
	}
	h.reset()
	for _, ch := range cmd {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
}

func (h *commandListHandler) dispatchCommand() (
	quit, handled bool,
) {
	match, ok := h.list.Focus()

	var commandString string
	if len(h.commandAndArgs) != 0 {
		if ok {
			h.commandAndArgs = append(h.commandAndArgs, string(match.Data()))
		} else if buf := h.list.Buffer().String(); buf != "" {
			h.commandAndArgs = append(h.commandAndArgs, buf)
		}
		// trim empty args (i.e. client added more spaces than required between args)
		trimmedCmdAndArgs := make([]string, 0, len(h.commandAndArgs))
		for _, argi := range h.commandAndArgs {
			if argi != "" {
				trimmedCmdAndArgs = append(trimmedCmdAndArgs, argi)
			}
		}
		h.commandAndArgs = trimmedCmdAndArgs
		commandString = strings.Join(h.commandAndArgs, " ")

		h.log(log.TraceLevel, "dispatching command and args %#v", h.commandAndArgs)
		quit = h.dispatchFunc(h.commandAndArgs[0], h.commandAndArgs[1:]...)
	} else {
		commandString = string(match.Data())
		// no match, use what's in buffer
		if commandString == "" {
			commandString = h.buf.String()
		}

		h.log(log.TraceLevel, "dispatching command %#v", commandString)
		quit = h.dispatchFunc(commandString)
	}

	if commandString != "" {
		err := h.history.Add(commandString)
		if err != nil {
			h.log(log.ErrorLevel, "add history %q: %v", commandString, err)
		} else {
			h.log(log.TraceLevel, "add history %q: ok", commandString)
		}
	}

	if quit {
		// hack to signal exit process
		return true, false
	}
	return true, true
}

func (h *commandListHandler) Handle(ev term.Event) (quit, handled bool) {
	switch h.mode {
	case modeCommandHandlerCommand:
		return h.handleCommand(ev)
	default:
		return h.handleCompleteArgs(ev)
	}
}

func (h *commandListHandler) handleCommon(ev *term.Event) (quit, handled bool) {
	key := ev.KeyComb()
	if (key == h.commandKey && h.commandKey.Ch == 0) ||
		(key == h.commandKey && h.buf.Columns(0) == 0) ||
		(key == h.commandKey && h.prevCommandCycle) {
		h.handleLastCommand()
		h.prevCommandCycle = true
		return false, true
	}

	h.prevCommandCycle = false
	handled = true
	switch ev.Key {
	case term.KeyEnter:
		quit, handled = h.dispatchCommand()
		h.reset()
	case term.KeyEsc:
		quit = true
	case term.KeyArrowDown, term.KeyCtrlJ:
		h.list.FocusDown()
	case term.KeyArrowUp, term.KeyCtrlK:
		h.list.FocusUp()
	case term.KeyTab:
		if h.list.MatchCount() > 1 {
			if !h.list.FocusDown() {
				h.list.FocusStart()
			}
			return
		}
		// if only one match, then select that
		fallthrough
	case term.KeySpace:
		ev.Ch = ' '
		handled = false
	default:
		handled = false
	}
	return
}

func (h *commandListHandler) handleCommand(ev term.Event) (quit, handled bool) {
	quit, handled = h.handleCommon(&ev)
	if handled {
		return
	}

	switch ev.Key {
	case term.KeyBackspace, term.KeyBackspace2:
		cols := h.buf.Columns(0)
		if cols == 0 {
			handled = true
			quit = true
			return
		}
		h.buf.DeleteCell(term.Coordinates{X: cols - 1})
		h.list.Buffer().Replace(h.buf.String())
		handled = true
		return
	}

	if ev.Ch == 0 {
		return
	}

	handled = true
	h.buf.WriteString(string(ev.Ch))
	if ev.Ch == ' ' {
		h.incArgsCompleteMode()
		return
	}
	h.list.Buffer().WriteString(string(ev.Ch))
	return
}

func (h *commandListHandler) handleCompleteArgs(ev term.Event) (quit, handled bool) {
	quit, handled = h.handleCommon(&ev)
	if handled {
		return
	}
	switch ev.Key {
	case term.KeyBackspace, term.KeyBackspace2:
		handled = true
		h.buf.DeleteCell(term.Coordinates{X: h.buf.Columns(0) - 1})
		if h.list.Buffer().Size() != 0 {
			h.list.Buffer().DeleteCell(
				term.Coordinates{X: h.list.Buffer().Columns(0) - 1},
			)
			return
		}
		if !h.decArgsCompleteMode() {
			h.setCommandMode()
		}
		return
	}

	if ev.Ch == 0 {
		return
	}

	handled = true
	h.buf.WriteString(string(ev.Ch))
	if ev.Ch == ' ' {
		h.incArgsCompleteMode()
		return
	}
	h.list.Buffer().WriteString(string(ev.Ch))
	return
}

func (h *commandListHandler) completeTopList() bool {
	command, ok := h.list.Focus()
	if !ok {
		h.log(log.TraceLevel, "completeTopList: no matches on search list")
		return false
	}
	h.commandAndArgs = append(h.commandAndArgs, string(command.Data()))
	newCmdAndArgs := strings.Join(h.commandAndArgs, " ")
	h.buf.Replace(newCmdAndArgs + " ")

	return true
}

func (h *commandListHandler) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "commandListHandler").
		Logf(level, msg, args...)
}

func (h *commandListHandler) incArgsCompleteMode() {
	h.mode++
	if !h.completeTopList() {
		h.commandAndArgs = append(h.commandAndArgs, h.list.Buffer().String())
	}
	h.list.Buffer().Reset()
	h.setCompletionList(h.commandAndArgs[0], h.commandAndArgs[1:]...)
}

func (h *commandListHandler) decArgsCompleteMode() bool {
	if h.mode == 1 {
		return false
	}
	h.mode--
	lastIdx := len(h.commandAndArgs) - 1
	last := h.commandAndArgs[lastIdx]
	h.commandAndArgs = h.commandAndArgs[:lastIdx]
	h.list.Buffer().Replace(last)
	h.setCompletionList(h.commandAndArgs[0], h.commandAndArgs[1:]...)
	return true
}

func (h *commandListHandler) setCommandMode() {
	h.mode = modeCommandHandlerCommand
	h.list.Buffer().Replace(h.buf.String())
	h.commandAndArgs = h.commandAndArgs[:0]
	h.resetListWith(h.commandsBackup)
}

func (h *commandListHandler) setCompletionList(cmd string, args ...string) {
	h.log(log.TraceLevel, "setCompletionList: %v", h.commandAndArgs)

	ctx := context.Background()
	it := h.completeFunc(ctx, cmd, args...)

	var completionItems []string
	for {
		next, ok := it.Next()
		if !ok {
			break
		}
		completionItems = append(completionItems, next)
	}
	err := it.Err()
	if err != nil {
		h.log(log.ErrorLevel, "completion iterator error: %v", err)
		return
	}
	h.resetListWith(completionItems)
}

func (h *commandListHandler) dataReset(items []string) {
	h.setCommandMode()
	h.buf.Reset()
	h.list.Buffer().Reset()
	h.commandsBackup = items
	h.resetListWith(items)
}

func (h *commandListHandler) resetListWith(items []string) {
	h.list.DataReset()
	for _, item := range items {
		h.list.PushSync([]byte(item))
	}
}

func (h *commandListHandler) reset() {
	h.setCommandMode()
	h.buf.Reset()
	h.list.Buffer().Reset()
}

func (h *commandListHandler) Cursor() (term.Coordinates, bool) {
	var pos term.Coordinates
	cmdWidth, cmdHeight := h.commandOverlayDimensions()
	x := len(h.buf.String()) % cmdWidth
	y := len(h.buf.String()) / cmdWidth
	if y >= cmdHeight {
		pos.X += cmdWidth - 1
		pos.Y += cmdHeight - 1
	} else {
		pos.X += x
		pos.Y += y
	}
	return pos, true
}

func (h *commandListHandler) Man() tui.Manual {
	panic("TODO")
}

func (h *commandListHandler) Close() error {
	return h.list.Close()
}
