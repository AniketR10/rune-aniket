package extension

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	browserextension "unstable.build/go-tui/api/browser/extension"
	configapi "unstable.build/go-tui/api/config"
	configextension "unstable.build/go-tui/api/config/extension"
	storageextension "unstable.build/go-tui/api/storage/extension"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cmd/extension_ai/backend"
	"unstable.build/go-tui/cmd/extension_ai/backend/openai"
	aiDialogue "unstable.build/go-tui/cmd/extension_ai/dialogue"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/extension"
	plugutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/dialogue"
	"unstable.build/go-tui/handler/input"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/clipboard"
	sysclip "unstable.build/go-tui/text/clipboard/system"
)

const (
	commandQuery      = "?"
	commandChat       = "assistantChat"
	commandResetChat  = "assistantResetChat"
	defaultRPCTimeout = 20 * time.Second
	defaultChatName   = "default"
)

var (
	AIHandlerCommands = []textapi.CommandManual{
		{
			Name: commandQuery,
			Summary: fmt.Sprintf("Send a message to your AI assistant. "+
				"The default model used is configured via extension configuration. "+
				"Available models: %s", availableModelsString()),
			Synopsis: "[message]",
		},
		{
			Name: commandChat,
			Summary: fmt.Sprintf("Open a new conversation tab with your AI assistant. "+
				"If no dialogue ID is provided, a new conversation is started. "+
				"If not passed, the default model used is configured via extension configuration. "+
				"Available models: %s", availableModelsString()),
			Synopsis: "[dialogue_id [model]]",
		},
		{Name: commandResetChat, Summary: "Clear all current chat's history."},
	}
	AIHandlerEvents      = []textapi.EventType{}
	AIHandlerPermissions = []extension.Permission{
		extension.PermissionBrowserWindowManager,
		extension.PermissionBrowserResourceOpener,
		extension.PermissionBrowserNotifications,
		extension.PermissionBrowserEventPublisher,
		extension.PermissionStorage,
		extension.PermissionEditor,
		extension.PermissionConfig,
	}
	defaultComponentCfg = dialogue.ComponentConfig{
		MessagesRowConfig: component.SpanConfig{
			PadHorizontal:    -80,
			PadVertical:      2,
			ContentAlignment: component.SpanAlignmentCentered,
		},
		InputRowColumns: 10,
		InputConfig: input.BoxConfig{
			Placeholder: "Message your assistant...",
			PlaceholderConfig: component.StringConfig{
				Alignment:            component.SpanAlignmentLeft,
				Attributes:           term.Attributes{Fg: 239},
				BackgroundAttributes: term.Attributes{},
			},
			ContentConfig:    term.Attributes{},
			DefaultFrameAttr: term.Attributes{},
		},
		ReceiveMessageStringConfig: component.StringConfig{
			Alignment:            component.SpanAlignmentLeft,
			Attributes:           term.Attributes{Fg: 250},
			BackgroundAttributes: term.Attributes{},
		},
		ReceiveMessageSpanConfig: component.SpanConfig{
			PadVertical:      1,
			ContentAlignment: component.SpanAlignmentLeft,
		},
		SendMessageStringConfig: component.StringConfig{
			Alignment:            component.SpanAlignmentLeft,
			Attributes:           term.Attributes{},
			BackgroundAttributes: term.Attributes{},
		},
		SendMessageSpanConfig: component.SpanConfig{
			PadVertical:      1,
			ContentAlignment: component.SpanAlignmentLeft,
		},
	}
)

// CommandEventHandler returns a plugutil.CommandEventHandler that manages
// this extension's logic.
func CommandEventHandler(
	ed textapi.Editor, grants []extension.Grant,
	broker proto.MuxBroker, pconfig configapi.Config,
) (hret plugutil.CommandEventHandler, err error) {
	ret := new(aiEditorHandler)
	ret.ctx, ret.cancelCtx = context.WithCancel(context.Background())
	ret.ed = ed
	ret.apiKey, err = pconfig.GetString("api_key")
	if err != nil {
		err = fmt.Errorf("failed to get 'api_key' from config: %w", err)
		return nil, err
	}
	ret.defaultModel, err = pconfig.GetString("model")
	if err != nil {
		if err != configapi.ErrNotFound {
			err = fmt.Errorf("failed to get 'model' from config: %w", err)
			log.Warn(err)
		}
		ret.defaultModel = openai.GPT3Dot5Turbo
	}

	if err := isAvailableModel(ret.defaultModel); err != nil {
		return nil, err
	}

	ret.rpcTimeout, err = configapi.GetDuration(pconfig,
		"rpc_timeout", defaultRPCTimeout)
	if err != nil {
		return nil, err
	}

	ret.cfg = defaultComponentCfg
	backgroundAttr, err := configapi.GetAttributes(pconfig, "background_attr")
	if err != nil {
		if err != configapi.ErrNotFound {
			err = fmt.Errorf("Error getting 'background_attr' from extension config: %v", err)
			log.Warn(err)
		}
	}
	ret.cfg.InputConfig.PlaceholderConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.ReceiveMessageStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.SendMessageStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.InputConfig.DefaultFrameAttr.Bg = backgroundAttr.Bg
	ret.cfg.InputConfig.PlaceholderConfig.Attributes.Bg = backgroundAttr.Bg
	ret.cfg.InputConfig.ContentConfig.Bg = backgroundAttr.Bg
	ret.cfg.ReceiveMessageStringConfig.Attributes.Bg = backgroundAttr.Bg
	ret.cfg.SendMessageStringConfig.Attributes.Bg = backgroundAttr.Bg
	ret.backgroundAttr = backgroundAttr

	sendMsgAttr, err := configapi.GetAttributes(pconfig, "user_msg_attr")
	if err != nil {
		if err != configapi.ErrNotFound {
			err = fmt.Errorf("Error getting 'user_msg_attr' from extension config: %v", err)
			log.Warn(err)
		}
	} else {
		ret.cfg.SendMessageStringConfig.Attributes = sendMsgAttr
	}
	recvMsgAttr, err := configapi.GetAttributes(pconfig, "assistant_msg_attr")
	if err != nil {
		if err != configapi.ErrNotFound {
			err = fmt.Errorf("Error getting 'assistant_msg_attr' from extension config: %v", err)
			log.Warn(err)
		}
	} else {
		ret.cfg.ReceiveMessageStringConfig.Attributes = recvMsgAttr
	}

	inputBoxAttr, err := configapi.GetAttributes(pconfig, "input_box_attr")
	if err != nil {
		if err != configapi.ErrNotFound {
			err = fmt.Errorf("Error getting 'input_box_attr' from extension config: %v", err)
			log.Warn(err)
		}
	} else {
		ret.cfg.InputConfig.ContentConfig = inputBoxAttr
	}
	inputBoxPlaceholderAttr, err := configapi.GetAttributes(pconfig, "input_box_placeholder_attr")
	if err != nil {
		if err != configapi.ErrNotFound {
			err = fmt.Errorf("Error getting 'input_box_placeholder_attr'"+
				" from extension config: %v", err)
			log.Warn(err)
		}
	} else {
		ret.cfg.InputConfig.PlaceholderConfig.Attributes = inputBoxPlaceholderAttr
	}

	inputBoxFrameAttr, err := configapi.GetAttributes(pconfig, "input_box_frame_attr")
	if err != nil {
		if err != configapi.ErrNotFound {
			err = fmt.Errorf("Error getting 'input_box_frame_attr' from extension config: %v", err)
			log.Warn(err)
		}
	} else {
		ret.cfg.InputConfig.DefaultFrameAttr = inputBoxFrameAttr
	}

	ret.clip, err = sysclip.NewRegister()
	if err != nil {
		log.Warnf("system clipboard unsupported: %v", err)
		ret.clip = clipboard.NewInMemory()
	}

	for _, g := range grants {
		switch g.Permission {
		case extension.PermissionStorage:
			ret.db, err = storageextension.Storage(g, broker)
		case extension.PermissionBrowserEventPublisher:
			ret.p, err = browserextension.EventPublisher(g, broker)
		case extension.PermissionBrowserResourceOpener:
			ret.o, err = browserextension.ResourceOpener(g, broker)
		case extension.PermissionBrowserWindowManager:
			ret.wm, err = browserextension.WindowManager(g, broker)
		case extension.PermissionBrowserNotifications:
			ret.n, err = browserextension.Notifications(g, broker)
		case extension.PermissionConfig:
			config, err := configextension.FetchConfig(g, broker)
			if err != nil {
				return nil, err
			}
			ret.editor, err = plugutil.Editor(ret.clip, config)
			if err != nil {
				ret.editor = text.DefaultSimpleEditor(ret.clip)
				log.Warnf("Could not get editor.mode from config: "+
					"%s.. Using 'modeless' editor.", err)
			}
			ret.cfg.InputEditor = ret.editor
		}
		if err != nil {
			return nil, err
		}
	}
	ret.dialogueStore = aiDialogue.NewStore(ret.db)

	return ret, nil
}

type aiEditorHandler struct {
	apiKey         string
	defaultModel   string
	rpcTimeout     time.Duration
	editor         text.Editor
	cfg            dialogue.ComponentConfig
	backgroundAttr term.Attributes
	dialogueStore  aiDialogue.Store

	clip clipboard.Register
	ed   textapi.Editor
	wm   browserapi.WindowManager
	n    browserapi.Notifications
	o    browserapi.ResourceOpener
	p    browserapi.EventPublisher
	db   document.Service

	ctx       context.Context
	cancelCtx func()
	exit      bool
}

func (h *aiEditorHandler) Handle(ctx context.Context, ev textapi.Event) (exit bool) {
	exit = h.exit
	if exit {
		return
	}
	return
}

func (h *aiEditorHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (exit bool, err error) {
	switch cmd.Name {
	case commandQuery:
		return h.handleQuery(cmd)
	case commandChat:
		return h.handleChat(cmd)
	case commandResetChat:
		return h.handleResetChat(cmd)
	}

	return false, nil
}

func (h *aiEditorHandler) Complete(ctx context.Context, args []string) (
	iterator.Iterator[string], string, error,
) {
	return iterator.FromSlice[string](nil), "", nil
}

func (h *aiEditorHandler) Close() error {
	h.exit = true
	h.cancelCtx()
	return nil
}

func (h *aiEditorHandler) newDialogueComponent() *dialogue.Component {
	return dialogue.NewComponent(h.cfg)
}

func (h *aiEditorHandler) handleChat(cmd textapi.Command) (bool, error) {
	if len(cmd.Args) > 0 {
		if err := isAvailableModel(cmd.Args[0]); err == nil {
			return false, errors.New("Model must be passed as a second argument to a dialogue ID. " +
				"Check command manual for more details.")
		}
	}
	model := h.defaultModel
	if len(cmd.Args) > 1 {
		model = cmd.Args[1]
		if err := isAvailableModel(model); err != nil {
			return false, err
		}
	}

	comp := h.newDialogueComponent()
	backendService := openai.NewClient(h.apiKey, openai.Config{
		Model: model,
	})
	dialogueManager := aiDialogue.NewManager(backendService, h.dialogueStore)
	dhandler, tx, rx := dialogue.Handler(comp, h.p, h.clip)

	ctx, cancel := context.WithCancel(h.ctx)
	d, err := h.getDialogue(ctx, h.dialogueStore, cmd)
	if err != nil {
		cancel()
		return false, err
	}

	// dialogue history
	for _, msg := range d.Messages {
		addMessage(comp, msg)
	}

	go createCompletions(ctx, cancel, tx, rx, dialogueManager, d.ID)

	background := component.WithBackground(comp, term.Cell{
		Bg: h.backgroundAttr.Bg,
		Fg: h.backgroundAttr.Fg,
	})
	bhandler := browserapi.FuncHandler(handler.WithComponent(dhandler, background),
		func() error {
			cancel()
			return nil
		})
	uri, err := workspaceapi.ParseURI(fmt.Sprintf("assistant://%s/%s", model, d.ID))
	if err != nil {
		panic(err)
	}
	tab, err := h.wm.Tab(uri, uri.String(), bhandler)
	if err != nil {
		return true, fmt.Errorf("create tab: %v", err)
	}

	if err := cmd.Window.SetContent(tab); err != nil {
		return false, fmt.Errorf("window set content: %v", err)
	}
	return false, nil
}

func (h *aiEditorHandler) handleQuery(cmd textapi.Command) (bool, error) {
	comp := h.newDialogueComponent()
	backendService := openai.NewClient(h.apiKey, openai.Config{
		Model: h.defaultModel,
	})
	dialogueManager := aiDialogue.NewManager(backendService, h.dialogueStore)
	dhandler, tx, rx := dialogue.Handler(comp, h.p, h.clip)

	id := strconv.Itoa(rand.Int())
	ctx, cancel := context.WithCancel(h.ctx)

	query := strings.Join(cmd.Args, " ")
	msg := backend.ChatCompletionMessage{Content: query, Role: openai.RoleUser}
	addMessage(comp, msg)

	go func() {
		// manually add input and returned completion
		it, err := dialogueManager.CreateCompletion(ctx, id, []string{query})
		if err != nil {
			cancel()
			if !errors.Is(err, context.Canceled) {
				err := h.n.Notify(notifications.LevelError,
					"create chat completion: %v", err)
				if err != nil {
					log.Errorf("notify: %v", err)
				}
			}
			return
		}
		drawMessage(ctx, it, tx)

		// resume creating completions upon further user input
		createCompletions(ctx, cancel, tx, rx, dialogueManager, id)
	}()

	var win browserapi.Window
	background := component.WithBackground(comp, term.Cell{
		Bg: h.backgroundAttr.Bg,
		Fg: h.backgroundAttr.Fg,
	})
	bhandler := browserapi.FuncHandler(handler.WithComponent(dhandler, background),
		func() error {
			cancel()
			if win != nil {
				return win.Close()
			}
			return nil
		})
	floating := browserapi.FuncFloating(bhandler, func() (int, int) {
		const width = 100
		return width, comp.Height(width)
	})
	floatingConfig := component.FloatingConfig{
		Alignment: component.SpanAlignmentCentered,
	}
	win, err := h.wm.Floating(floating, floatingConfig)
	if err != nil {
		return false, fmt.Errorf("floating window: %v", err)
	}
	return false, nil
}

func (h *aiEditorHandler) getDialogue(
	ctx context.Context, dialogueStore aiDialogue.Store, cmd textapi.Command,
) (aiDialogue.Dialogue, error) {
	dialogueID := getDialogueID(cmd)
	d, err := dialogueStore.Get(ctx, dialogueID)
	if err != nil {
		if !errors.Is(err, document.ErrNotFound) {
			return aiDialogue.Dialogue{}, fmt.Errorf("get dialogue from store: %w", err)
		}
		d.ID = dialogueID
	}
	return d, nil
}

func (h *aiEditorHandler) handleResetChat(cmd textapi.Command) (bool, error) {
	dialogueStore := aiDialogue.NewStore(h.db)
	dialogueID := getDialogueID(cmd)
	err := dialogueStore.Delete(h.ctx, dialogueID)
	if err != nil {
		return false, fmt.Errorf("remove dialogue store: %w", err)
	}
	return false, nil
}

func drawMessage(
	ctx context.Context,
	it iterator.Iterator[string], tx chan<- string,
) {
	for {
		response, ok := it.Next()
		if !ok {
			break
		}
		select {
		case tx <- response:
		case <-ctx.Done():
			return
		}
	}
	if it.Err() != nil {
		if !errors.Is(it.Err(), context.Canceled) {
			log.Errorf("stream completion: %v", it.Err())
		}
	}

	// signal end of message
	select {
	case tx <- dialogue.EOM:
	case <-ctx.Done():
		return
	}
}

func getDialogueID(cmd textapi.Command) string {
	id := defaultChatName
	if len(cmd.Args) > 0 {
		id = cmd.Args[0]
	}
	return id
}

func addMessage(c *dialogue.Component, msg backend.ChatCompletionMessage) {
	switch openai.Role(msg.Role) {
	case openai.RoleAssistant:
		c.AddReceiveMessageChunk(msg.Content)
		c.AddReceiveMessageBreak()
	case openai.RoleUser:
		c.AddSendMessage(msg.Content)
	// case openai.RoleSystem, openai.RoleTool:
	default:
		/* do not render */
	}
}

func availableModelsString() string {
	var availableStr strings.Builder
	var i int
	for k := range openai.AvailableModels() {
		if i != 0 {
			availableStr.WriteString(", ")
		}
		availableStr.WriteString(k)
		i++
	}
	return availableStr.String()
}

func isAvailableModel(model string) error {
	available := openai.AvailableModels()
	if _, ok := available[model]; !ok {
		availableStr := availableModelsString()
		return fmt.Errorf("Model '%s' is not supported. Available models: %s",
			model, availableStr)
	}
	return nil
}

func createCompletions(
	ctx context.Context, cancel func(),
	tx chan<- string, rx <-chan string,
	dialogueManager aiDialogue.Manager,
	id string,
) {
	defer close(tx)
	defer cancel()
	for {
		var msg string
		select {
		case <-ctx.Done():
			return
		case msg = <-rx:
		}

		it, err := dialogueManager.CreateCompletion(ctx, id, []string{msg})
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Errorf("dialogue manager create completion: %v", err)
			}
			return
		}
		drawMessage(ctx, it, tx)
	}
}
