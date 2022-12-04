package command

import (
	"context"
	"math"
	"strings"
	"sync"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/blue/logging"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler/search"
	"unstable.build/go-tui/term"
)

// NewHandler allocates storage for a new Handler and initializes it.
func NewHandler(
	storage document.Service, completer Completer,
	dispatcher Dispatcher, interrupter term.Interrupter,
	commands []string, config Config,
) *Handler {
	ret := new(Handler)
	ret.Init(storage, completer, dispatcher, interrupter, commands, config)
	return ret
}

// Handler is a tui.Handler that presents a command browsing prompt
// with history scrolling and argument completion.
type Handler struct {
	mode       commandHandlerMode
	config     Config
	dispatcher Dispatcher
	completer  Completer

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

	sync      bool
	mu        sync.Mutex
	cancelFn  func()
	cancelCtx context.Context
}

type commandHandlerMode uint

const (
	modeCommandHandlerCommand commandHandlerMode = iota
	// modeCommandHandlerArgs1
	// modeCommandHandlerArgs2
	// ...
)

// Init initializes this handler with the given storage, completer,
// dispatcher, interrupter and config.
func (h *Handler) Init(
	storage document.Service, completer Completer,
	dispatcher Dispatcher, interrupter term.Interrupter,
	commands []string, config Config,
) {
	cfg := search.ListConfig{
		Algo:             search.FuzzyMatch,
		Interrupter:      interrupter,
		CaseSensitive:    false,
		MatchedTextAttr:  &config.MatchedTextAttr,
		FocusElementAttr: &config.FocusElementAttr,
		ElementAttr:      &config.ElementAttr,
	}
	h.doInit(storage, completer, dispatcher, commands, config, cfg)
}

func (h *Handler) doInit(
	storage document.Service, completer Completer,
	dispatcher Dispatcher, commands []string, config Config,
	listCfg search.ListConfig,
) {
	h.mode = modeCommandHandlerCommand
	h.config = config
	h.dispatcher = dispatcher
	h.completer = completer

	h.buf.Init()
	h.responsive = component.BufferResponsive(&h.buf,
		component.StringConfig{
			Attributes:           config.ElementAttr,
			BackgroundAttributes: config.ElementAttr,
		})

	h.list.Init(listCfg)

	for {
		h.history.Init(storage, config.DocumentID, config.MaxHistory)
		err := h.history.Load()
		if err == nil {
			break
		}
		h.log(log.ErrorLevel, "load history: %v", err)
		storage = document.NewInMemoryService()
	}

	// add a canceled cancelCtx so Wait never needs to check if cancelFn is nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h.cancelCtx = ctx
	h.cancelFn = func() {}

	h.Reset(commands)
}

func (h *Handler) resizeCommandOverlay() {
	// propagate local cmd+args buffer height to
	// search list, which only has cmd, in case args alone span
	// multiple lines
	height := h.responsive.Height(h.width)
	// set to min 1, as it's being used as input field
	// and max to the height of the overlayed component
	height = int(math.Min(math.Max(1, float64(height)), float64(h.height)))
	// the list is very short so waiting is not a significant
	// perf penalty and it makes tests easier to make deterministic
	h.list.SetMinInputHeight(height)
	h.responsive.Resize(h.width, height)
}

// Resize satisfies tui.Handler.
func (h *Handler) Resize(width, height int) {
	h.width = width
	h.height = height
	h.list.Resize(width, height)
}

// Draw satisfies tui.Handler.
func (h *Handler) Draw(w term.Writer) {
	// resize on every draw because search.List uses a responsive
	// input so local buffer changes must consider potential resize
	// of search.List
	h.resizeCommandOverlay()
	// the list is very short so waiting is not a significant
	// perf penalty and it makes tests easier to make deterministic
	h.list.Draw(w)
	h.responsive.Draw(w)
}

func (h *Handler) handleLastCommand() {
	cmd := h.history.Next()
	if cmd == "" {
		h.log(log.TraceLevel, "ignoring next command in history: empty")
		return
	}
	h.reset()
	for _, ch := range cmd {
		h.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	h.log(log.TraceLevel, "done pushing history events")
}

func (h *Handler) trimmedCommandAndArgs(cmd string, args ...string) []string {
	ret := make([]string, 0, len(args)+1)
	ret = append(ret, cmd)
	for _, argi := range args {
		if argi != "" {
			ret = append(ret, argi)
		}
	}
	return ret
}

func (h *Handler) dispatchCommand() (
	quit, handled bool,
) {

	var commandString string
	if len(h.commandAndArgs) != 0 {
		// add what's currently in the buffer; if user wanted to auto-complete
		// then Tab should be expected first
		if buf := h.list.Buffer().String(); buf != "" {
			h.commandAndArgs = append(h.commandAndArgs, buf)
		}
		// trim empty args (i.e. client added more spaces than required between args)
		h.commandAndArgs = h.trimmedCommandAndArgs(h.commandAndArgs[0], h.commandAndArgs[1:]...)
		commandString = strings.Join(h.commandAndArgs, " ")

		h.log(log.TraceLevel, "dispatching command and args %#v", h.commandAndArgs)
		quit = h.dispatcher.Dispatch(h.commandAndArgs[0], h.commandAndArgs[1:]...)
	} else {
		match, _ := h.list.Focus()
		// if no args, then it means that we are in command mode, in which case
		// what's in the match list takes preference.
		commandString = string(match.Data())
		// no match, use what's in buffer
		if commandString == "" {
			commandString = h.buf.String()
		}

		h.log(log.TraceLevel, "dispatching command %#v", commandString)
		quit = h.dispatcher.Dispatch(commandString)
	}

	if commandString != "" {
		err := h.history.Add(commandString)
		if err != nil {
			h.log(log.ErrorLevel, "add history %q: %v", commandString, err)
		} else {
			h.log(log.TraceLevel, "add history %q: ok", commandString)
		}
	}

	return true, true
}

// Handle satisfies tui.Handler.
func (h *Handler) Handle(ev term.Event) (quit, handled bool) {
	switch h.mode {
	case modeCommandHandlerCommand:
		return h.handleCommand(ev)
	default:
		return h.handleCompleteArgs(ev)
	}
}

func (h *Handler) handleCommon(ev *term.Event) (quit, handled bool) {
	key := ev.KeyComb()
	if (key == h.config.HistoryKey && h.config.HistoryKey.Ch == 0) ||
		(key == h.config.HistoryKey && h.buf.Columns(0) == 0) ||
		(key == h.config.HistoryKey && h.prevCommandCycle) {
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
		h.Cancel()
	case term.KeyArrowDown, term.KeyCtrlJ:
		h.list.FocusDown()
	case term.KeyArrowUp, term.KeyCtrlK:
		h.list.FocusUp()
	case term.KeyCtrlC:
		h.cancelCompletionPush("received ctrl-c")
	case term.KeyTab:
		h.incArgsCompleteMode(true)
	case term.KeySpace:
		ev.Ch = ' '
		handled = false
	default:
		handled = false
	}
	return
}

func (h *Handler) handleCommand(ev term.Event) (quit, handled bool) {
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
			h.Cancel()
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
		// wait as commands are finite and muscle memory could beat
		// the completing logic
		h.Wait()
		h.incArgsCompleteMode(true)
		return
	}
	h.list.Buffer().WriteString(string(ev.Ch))
	return
}

func (h *Handler) handleCompleteArgs(ev term.Event) (quit, handled bool) {
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
			h.setCompletionList(h.commandAndArgs[0], h.completionArgs()...)
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
		// do not wait here, as args are expected to be dynamic
		// and fuzzy search is a guide for user to complete
		h.incArgsCompleteMode(false)
		return
	}
	h.list.Buffer().WriteString(string(ev.Ch))
	h.setCompletionList(h.commandAndArgs[0], h.completionArgs()...)
	return
}

func (h *Handler) completionArgs() []string {
	args := make([]string, 0, len(h.commandAndArgs))
	args = append(args, h.commandAndArgs[1:]...)
	return append(args, h.list.Buffer().String())
}

func (h *Handler) completeTopList() bool {
	match, ok := h.list.Focus()
	if !ok {
		h.log(log.TraceLevel, "completeTopList: no matches on search list with %q",
			h.list.Buffer().String())
		return false
	}
	part := string(match.Data())
	h.log(log.TraceLevel, "completeTopList: matched %q with %q",
		h.list.Buffer().String(), part)
	h.commandAndArgs = append(h.commandAndArgs, part)
	newCmdAndArgs := strings.Join(h.commandAndArgs, " ")
	h.buf.Replace(newCmdAndArgs + " ")

	return true
}

func (h *Handler) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "command.Handler").
		Logf(level, msg, args...)
}

func (h *Handler) incArgsCompleteMode(complete bool) {
	h.mode++
	if !complete || !h.completeTopList() {
		h.commandAndArgs = append(h.commandAndArgs, h.list.Buffer().String())
	}
	h.list.Buffer().Reset()
	h.setCompletionList(h.commandAndArgs[0], h.commandAndArgs[1:]...)
}

func (h *Handler) decArgsCompleteMode() bool {
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

func (h *Handler) setCommandMode() {
	h.mode = modeCommandHandlerCommand
	h.list.Buffer().Replace(h.buf.String())
	h.commandAndArgs = h.commandAndArgs[:0]
	h.resetListWith(h.commandsBackup)
}

func (h *Handler) setCompletionList(cmd string, args ...string) {
	h.log(log.TraceLevel, "setCompletionList: %s %#v", cmd, args)

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	// cancel prev if there's any
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cancelCompletionPush("re set completion list")
	h.cancelFn = cancel
	h.cancelCtx = ctx

	// if user added any extra spaces, do not pass to completer
	cmdAndArgs := h.trimmedCommandAndArgs(cmd, args...)

	h.list.DataReset()
	it := h.completer.Complete(ctx, cmdAndArgs[0], cmdAndArgs[1:]...)

	if h.sync {
		pushIteratorSync(ctx, &h.list, cancel, it, h.log)
	} else {
		ch := h.list.Push(ctx)
		go pushIterator(ctx, ch, cancel, it, h.log)
	}
}

func pushIteratorSync(
	ctx context.Context,
	list *search.List, cancel func(),
	it iterator.Iterator[string],
	l func(level log.Level, msg string, args ...interface{}),
) {
	defer cancel()
	for {
		next, ok := it.Next()
		if !ok {
			break
		}
		list.PushSync([]byte(next))
	}
}

func pushIterator(
	ctx context.Context,
	ch chan<- []byte, cancel func(),
	it iterator.Iterator[string],
	l func(level log.Level, msg string, args ...interface{}),
) {
	defer close(ch)
	defer cancel()

	for {
		next, ok := it.Next()
		if !ok {
			break
		}
		select {
		case <-ctx.Done():
			l(log.TraceLevel, "context canceled for ch %p before completed push", ch)
			return
		case ch <- []byte(next):
			l(log.TraceLevel, "pushed %q onto search list for ch %p", next, ch)
		}
	}

	err := it.Err()
	if err != nil {
		l(log.ErrorLevel, "completion iterator error: %v", err)
	}
}

// Reset resets the commands listed in this Handler.
// It should be called after initialization and every time
// new commands are available.
func (h *Handler) Reset(commands []string) {
	h.setCommandMode()
	h.buf.Reset()
	h.list.Buffer().Reset()
	h.commandsBackup = commands
	h.resetListWith(commands)
}

// assumes holding lock
func (h *Handler) cancelCompletionPush(reason string) {
	h.log(log.DebugLevel, "completion push to search list: %s", reason)
	h.cancelFn()
	h.list.Cancel()
}

func (h *Handler) resetListWith(items []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cancelCompletionPush("reset list")

	h.list.DataReset()
	for _, item := range items {
		h.list.PushSync([]byte(item))
	}
}

func (h *Handler) reset() {
	h.Cancel()
	h.setCommandMode()
	h.buf.Reset()
	h.list.Buffer().Reset()
}

// Cursor satisfies tui.Handler.
func (h *Handler) Cursor() (term.Coordinates, bool) {
	if h.width == 0 {
		return term.Coordinates{}, false
	}
	var pos term.Coordinates
	x := len(h.buf.String()) % h.width
	y := len(h.buf.String()) / h.width
	if y >= h.height {
		pos.X += h.width - 1
		pos.Y += h.height - 1
	} else {
		pos.X += x
		pos.Y += y
	}
	return pos, true
}

// Man satisfies tui.Handler
func (h *Handler) Man() tui.Manual {
	panic("TODO")
}

// Wait waits for any asynchronous completion
// or search to finish before it returns.
func (h *Handler) Wait() {
	h.mu.Lock()
	cancelCtx := h.cancelCtx
	h.mu.Unlock()

	<-cancelCtx.Done()
	h.list.Wait()
}

// Cancel cancels any asynchronous completion or search currently ongoing
// or does nothing if there's currently no ongoing completion or search.
func (h *Handler) Cancel() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cancelCompletionPush("cancel")
	h.list.Cancel()
}

func (h *Handler) Dimensions() (width, height int) {
	maxWidthItems := 20
	h.list.IterateVisible(func(m search.Match) {
		if mlen := len(m.Data()); mlen > maxWidthItems {
			maxWidthItems = mlen
		}
	})
	width = int(math.Max(float64(h.list.Buffer().MaxColumns()), float64(maxWidthItems)))
	width = int(math.Min(math.Max(float64(width), 50), 100))
	height = int(math.Min(math.Max(float64(h.list.MatchCount()), 3), 40))
	return
}

// Close closes all resources associated with this Handler.
func (h *Handler) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.cancelCompletionPush("close")
	return h.list.Close()
}
