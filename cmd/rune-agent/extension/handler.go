// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools/webfetch"
	"unstable.build/go-tui/cmd/rune-agent/agent/taskstore"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/clipboard/sysclip"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/handler/inputbox"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/agentshell"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"unstable.build/go-tui/cmd/rune-agent/llm/anthropic"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"
	"unstable.build/go-tui/cmd/rune-agent/llm/openai"
	runemcp "unstable.build/go-tui/cmd/rune-agent/mcp"
	"unstable.build/go-tui/cmd/rune-agent/memory"
	"unstable.build/go-tui/component/markdown"
	mdhandler "unstable.build/go-tui/handler/markdown"
)

const (
	commandQuery = "?"
	commandChat  = "agent"
)

var (
	defaultComponentCfg = dialoguetui.ComponentConfig{
		MessagesRowConfig: component.SpanConfig{
			PadHorizontal:    -80,
			PadVertical:      2,
			ContentAlignment: component.AlignmentCentered,
		},
		InputRowColumns: 10,
		InputBox: dialoguetui.InputBoxConfig{
			Placeholder: "Message your assistant...",
			PlaceholderConfig: component.StringResponsiveConfig{
				NoSplitWords: true,
				StringConfig: component.StringConfig{
					Alignment:            component.AlignmentLeft,
					Attributes:           term.Attributes{Fg: tcell.ColorGray},
					BackgroundAttributes: term.Attributes{},
				},
			},
			ContentAttr: term.Attributes{},
			FrameAttr:   term.Attributes{},
		},
		ReceiveMessageStringConfig: component.StringConfig{
			Alignment:            component.AlignmentLeft,
			Attributes:           term.Attributes{Fg: tcell.ColorSilver},
			BackgroundAttributes: term.Attributes{},
		},
		ReceiveMessageSpanConfig: component.SpanConfig{
			ContentAlignment: component.AlignmentLeft,
		},
		ReasoningStringConfig: component.StringConfig{
			Alignment:            component.AlignmentLeft,
			Attributes:           term.Attributes{Fg: tcell.ColorGray},
			BackgroundAttributes: term.Attributes{},
		},
		ReasoningSpanConfig: component.SpanConfig{
			ContentAlignment: component.AlignmentLeft,
		},
		SendMessageStringConfig: component.StringConfig{
			Alignment:            component.AlignmentLeft,
			Attributes:           term.Attributes{},
			BackgroundAttributes: term.Attributes{},
		},
		SendMessageSpanConfig: component.SpanConfig{
			ContentAlignment: component.AlignmentLeft,
		},
		SendMessageBottomPad: 1,
		ToolCallStringConfig: component.StringConfig{
			Alignment:            component.AlignmentLeft,
			Attributes:           term.Attributes{Fg: tcell.ColorPurple},
			BackgroundAttributes: term.Attributes{},
		},
		ToolCallSpanConfig: component.SpanConfig{
			ContentAlignment: component.AlignmentLeft,
		},
		ToolCallArgsStringConfig: component.StringConfig{
			Alignment:            component.AlignmentLeft,
			Attributes:           term.Attributes{Fg: tcell.ColorGray},
			BackgroundAttributes: term.Attributes{},
		},
		ToolResultStringConfig: component.StringConfig{
			Alignment:            component.AlignmentLeft,
			Attributes:           term.Attributes{Fg: tcell.ColorGray},
			BackgroundAttributes: term.Attributes{},
		},
		ToolResultSpanConfig: component.SpanConfig{
			ContentAlignment: component.AlignmentLeft,
		},
		ErrorStringConfig: component.StringConfig{
			Alignment:            component.AlignmentLeft,
			Attributes:           term.Attributes{Fg: tcell.ColorRed},
			BackgroundAttributes: term.Attributes{},
		},
		ErrorSpanConfig: component.SpanConfig{
			PadVertical:      1,
			ContentAlignment: component.AlignmentLeft,
		},
		WarningStringConfig: component.StringConfig{
			Alignment:            component.AlignmentLeft,
			Attributes:           term.Attributes{Fg: tcell.ColorYellow},
			BackgroundAttributes: term.Attributes{},
		},
		WarningSpanConfig: component.SpanConfig{
			PadVertical:      1,
			ContentAlignment: component.AlignmentLeft,
		},
		CommandOutputSpanConfig: component.SpanConfig{
			PadVertical: 1,
		},
		ToolResultMaxLines: 5,
		StartCollapsed:     true,
		ReasoningAnnotationStringConfig: component.StringConfig{
			Alignment:  component.AlignmentLeft,
			Attributes: term.Attributes{Attrs: tcell.AttrDim},
		},
		MarkdownConfig: defaultMarkdownConfig(),
		PromptToolCallStringConfig: component.StringConfig{
			Alignment:            component.AlignmentLeft,
			Attributes:           term.Attributes{Fg: tcell.ColorAqua},
			BackgroundAttributes: term.Attributes{},
		},
		SelectionConfig: dialoguetui.SelectionConfig{
			TitleStringConfig: component.StringConfig{
				Alignment:  component.AlignmentLeft,
				Attributes: term.Attributes{Fg: tcell.ColorSilver},
			},
			OptionStringConfig: component.StringConfig{
				Alignment:  component.AlignmentLeft,
				Attributes: term.Attributes{Fg: tcell.ColorSilver},
			},
			CursorStringConfig: component.StringConfig{
				Alignment:  component.AlignmentLeft,
				Attributes: term.Attributes{Attrs: tcell.AttrReverse},
			},
			HeaderStringConfig: component.StringConfig{
				Alignment:  component.AlignmentLeft,
				Attributes: term.Attributes{Fg: tcell.ColorYellow},
			},
			DescStringConfig: component.StringConfig{
				Alignment:  component.AlignmentLeft,
				Attributes: term.Attributes{Fg: tcell.ColorDimGray},
			},
		},
	}
)

func defaultMarkdownConfig() *markdown.Config {
	cfg := markdown.DefaultConfig()
	cfg.Paragraph = term.Attributes{Fg: tcell.ColorSilver}
	cfg.HeaderPrefix = false
	return &cfg
}

// clientConstructor builds an llm.Service from a token, config, and
// context-window map. Defaults to openai.NewClient in production;
// tests may substitute a stub.
type clientConstructor func(token string, cfg openai.Config, models map[string]int) llm.Service

// newLLMService creates an LLM service for the given model by reading
// provider API keys and LLM parameters from the current config. This is
// called each time a service is needed so that config changes (e.g. via
// the agentshell config command) take effect immediately.
// If newClient is nil, openai.NewClient is used for non-Anthropic providers.
// If newAnthropicClient is nil, anthropic.NewClient is used for Anthropic.
func newLLMService(
	cfg config.Config,
	reg llmregistry.Registry,
	model string,
	newClient clientConstructor,
	newAnthropicClient anthropic.ClientConstructor,
) (llm.Service, error) {
	entry, ok := reg.Get(context.Background(), model)
	if !ok {
		return nil, fmt.Errorf("model %q not found in registry", model)
	}

	// Read provider API key.
	var apiKey string
	if entry.Provider != "ollama" {
		if pcfg, err := cfg.GetConfig(entry.Provider); err == nil {
			if key, err := pcfg.GetString("api_key"); err == nil {
				apiKey = key
			} else if !errors.Is(err, config.ErrNotFound) {
				return nil, fmt.Errorf("get %q api_key from config: %w", entry.Provider, err)
			}
		} else if !errors.Is(err, config.ErrNotFound) {
			return nil, fmt.Errorf("get %q config section: %w", entry.Provider, err)
		}
		if apiKey == "" {
			return nil, fmt.Errorf("no api_key configured for provider %q; "+
				"set it in the %q config section", entry.Provider, entry.Provider)
		}
	}

	// Read shared LLM parameters.
	var temperature, topP float64
	var maxTokens int
	var debugHTTP bool

	if v, err := cfg.GetFloat("temperature"); err == nil {
		temperature = v
	} else if !errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("get 'temperature' from config: %w", err)
	}
	if v, err := cfg.GetFloat("top_p"); err == nil {
		topP = v
	} else if !errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("get 'top_p' from config: %w", err)
	}
	if v, err := cfg.GetInt("max_tokens"); err == nil {
		maxTokens = v
	} else if !errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("get 'max_tokens' from config: %w", err)
	}
	if v, err := cfg.GetBool("debug_http"); err == nil {
		debugHTTP = v
	} else if !errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("get 'debug_http' from config: %w", err)
	}

	// <provider>.base_url overrides per entry base url and default in config.
	if pcfg, err := cfg.GetConfig(entry.Provider); err == nil {
		if key, err := pcfg.GetString("base_url"); err == nil {
			entry.BaseURL = key
		} else if !errors.Is(err, config.ErrNotFound) {
			return nil, fmt.Errorf("get %q base_url from config: %w", entry.Provider, err)
		}
	} else if !errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("get %q config section: %w", entry.Provider, err)
	}

	contextMap := map[string]int{model: entry.ContextWindow}

	// Dispatch to native Anthropic client for Anthropic models.
	if entry.Provider == anthropic.LLMProvider {
		acfg := anthropic.Config{
			Model:       model,
			MaxTokens:   maxTokens,
			Temperature: temperature,
			TopP:        topP,
			DebugHTTP:   debugHTTP,
		}
		acfg.BaseURL = entry.BaseURL
		if pcfg, err := cfg.GetConfig(entry.Provider); err == nil {
			if v, err := pcfg.GetString("cache_control"); err == nil && v != "" {
				acfg.CacheControl = v
			} else if err != nil && !errors.Is(err, config.ErrNotFound) {
				return nil, fmt.Errorf("get %q cache_control from config: %w", entry.Provider, err)
			}
			if v, err := pcfg.GetString("reasoning_effort"); err == nil && v != "" {
				switch llm.ReasoningEffort(v) {
				case llm.ReasoningEffortLow, llm.ReasoningEffortMedium,
					llm.ReasoningEffortHigh, llm.ReasoningEffortMax:
					acfg.ReasoningEffort = v
				default:
					return nil, fmt.Errorf("invalid %q reasoning_effort value %q: "+
						"must be low, medium, high, or max", entry.Provider, v)
				}
			} else if err != nil && !errors.Is(err, config.ErrNotFound) {
				return nil, fmt.Errorf("get %q reasoning_effort from config: %w", entry.Provider, err)
			}
		}
		// Enable adaptive thinking for models that support it (4.6+).
		acfg.EnableThinking = anthropic.SupportsAdaptiveThinking(model)
		if newAnthropicClient == nil {
			newAnthropicClient = anthropic.NewClient
		}
		return newAnthropicClient(apiKey, acfg, contextMap), nil
	}

	// All other providers go through the OpenAI-compatible client.
	var c openai.Config
	c.Model = model
	c.Temperature = temperature
	c.TopP = topP
	c.MaxTokens = maxTokens
	c.DebugHTTP = debugHTTP
	c.BaseURL = entry.BaseURL

	// Read per-provider OpenAI settings.
	if pcfg, err := cfg.GetConfig(entry.Provider); err == nil {
		if v, err := pcfg.GetBool("force_responses_api"); err == nil {
			c.ForceResponsesAPI = v
		} else if !errors.Is(err, config.ErrNotFound) {
			return nil, fmt.Errorf("get %q force_responses_api from config: %w", entry.Provider, err)
		}
		if v, err := pcfg.GetString("reasoning_effort"); err == nil && v != "" {
			switch llm.ReasoningEffort(v) {
			case llm.ReasoningEffortNone, llm.ReasoningEffortMinimal,
				llm.ReasoningEffortLow, llm.ReasoningEffortMedium,
				llm.ReasoningEffortHigh, llm.ReasoningEffortXHigh:
				c.ReasoningEffort = v
			default:
				return nil, fmt.Errorf("invalid %q reasoning_effort value %q: "+
					"must be none, minimal, low, medium, high or xhigh", entry.Provider, v)
			}
		} else if err != nil && !errors.Is(err, config.ErrNotFound) {
			return nil, fmt.Errorf("get %q reasoning_effort from config: %w", entry.Provider, err)
		}
	} else if !errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("get %q config section: %w", entry.Provider, err)
	}

	if v, err := cfg.GetFloat("frequency_penalty"); err == nil {
		c.FrequencyPenalty = v
	} else if !errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("get 'frequency_penalty' from config: %w", err)
	}
	if v, err := cfg.GetFloat("presence_penalty"); err == nil {
		c.PresencePenalty = v
	} else if !errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("get 'presence_penalty' from config: %w", err)
	}
	if v, err := cfg.GetString("reasoning_summary"); err == nil && v != "" {
		switch llm.ReasoningSummary(v) {
		case llm.ReasoningSummaryAuto, llm.ReasoningSummaryConcise,
			llm.ReasoningSummaryDetailed, llm.ReasoningSummaryDisabled:
			c.ReasoningSummary = v
		default:
			return nil, fmt.Errorf("invalid 'reasoning_summary' value %q: "+
				"must be auto, concise, detailed, or disabled", v)
		}
	} else if err != nil && !errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("get 'reasoning_summary' from config: %w", err)
	}
	if v, err := cfg.GetInt("max_completion_tokens"); err == nil {
		c.MaxCompletionTokens = v
	} else if !errors.Is(err, config.ErrNotFound) {
		return nil, fmt.Errorf("get 'max_completion_tokens' from config: %w", err)
	}

	if newClient == nil {
		newClient = openai.NewClient
	}
	return newClient(apiKey, c, contextMap), nil
}

func newCommandEventHandler(
	ctx context.Context, ed textapi.Editor, w *extensionapi.Workspace,
	pconfig config.Config,
	defaultRegistry llmregistry.Registry,
	defaultModel string,
) (ret *aiEditorHandler, err error) {
	fs := w.FileSystem(ctx)
	executor := w.Executor(ctx)
	terminal := w.Terminal(ctx)
	cwd, err := fs.URI(".")
	if err != nil {
		return nil, fmt.Errorf("get workspace root: %w", err)
	}

	var toolsCfg agentools.Config
	if v, err := pconfig.GetInt("max_line_bytes"); err == nil {
		toolsCfg.MaxLineBytes = v
	} else if err != nil && !errors.Is(err, config.ErrNotFound) {
		slog.Warn("get 'max_line_bytes' from config", "error", err)
	}

	lsp := w.LSP(ctx)
	tools, tracker := agentools.DefaultTools(fs, executor, cwd, lsp, toolsCfg)

	fetcher := webfetch.NewHTTPFetcher(webfetch.DefaultConfig())
	tools = append(tools, agentools.NewWebFetch(fetcher))

	parser := w.Parser(ctx)
	tools = append(tools, agentools.LSPTools(lsp, fs, cwd, tracker)...)
	tools = append(tools, agentools.SyntaxTools(parser, fs, cwd, tracker)...)

	mcpManager := runemcp.NewManager()
	mcpData, mcpReadErr := readWorkspaceFile(fs, filepath.Join(cwd.Path(), ".mcp.json"))
	if mcpReadErr == nil {
		mcpCfg, mcpParseErr := runemcp.LoadConfig(mcpData)
		if mcpParseErr != nil {
			slog.Warn("mcp: failed to parse .mcp.json", "error", mcpParseErr)
		} else {
			mcpTools := mcpManager.Connect(ctx, mcpCfg)
			tools = append(tools, mcpTools...)
		}
	} else if !errors.Is(mcpReadErr, os.ErrNotExist) {
		slog.Warn("mcp: failed to read .mcp.json", "error", mcpReadErr)
	}

	// Discover skills from config dirs.
	var skillDirs []string
	if dirs, err := pconfig.GetSlice("skills"); err == nil {
		for _, d := range dirs {
			if s, ok := d.(string); ok {
				skillDirs = append(skillDirs, s)
			}
		}
	}
	skillRegistry := skills.NewRegistry(fs, cwd, skillDirs, w.Notifications(ctx))
	if loaded := skillRegistry.List(); len(loaded) > 0 {
		slog.Info("loaded skills", "count", len(loaded))
	}
	tools = append(tools, agentools.NewSkillTool(skillRegistry, nil, nil))

	// Always register memory tools — the workspace may be bootstrapped
	// after the extension starts. Tools fail gracefully at runtime.
	memoryPath := filepath.Join(w.DataDir(ctx), "memory")
	tools = append(tools, agentools.MemoryTools(executor, memoryPath)...)

	db := w.Storage(ctx)
	sessionsDir := filepath.Join(w.DataDir(ctx), "sessions")
	dialogueStore := dialoguemanager.NewStore(db, sessionsDir)
	tools = append(tools, agentools.ConversationTools(dialogueStore, fs, sessionsDir)...)

	ret = new(aiEditorHandler)
	ret.defaultEffort = llm.ReasoningEffortHigh
	ret.ctx, ret.cancelCtx = context.WithCancel(context.Background())
	ret.ed = ed
	ret.config = pconfig
	ret.mcpManager = mcpManager
	ret.baseTools = tools
	ret.toolRegistry = agent.NewRegistry(tools...)
	ret.executor = executor
	ret.sessionMgr = agentools.NewSessionManager(ret.ctx, executor, terminal)
	ret.toolRegistry.RegisterOverrides(openai.LLMProvider,
		agentools.NewGrepFiles(fs, cwd, tracker),
		agentools.NewListDir(fs, cwd),
		agentools.NewExecCommand(ret.sessionMgr, cwd),
		agentools.NewWriteStdin(ret.sessionMgr),
	)
	ret.toolRegistry.RegisterExclusions(openai.LLMProvider,
		"search_content", "compact", "bash")
	ret.systemPrompt = agent.DefaultSystemPrompt(cwd)

	// Discover and load project instruction files (e.g. AGENTS.md).
	agentsFile := agent.DefaultAgentsFile
	if v, err := pconfig.GetString("agents_file"); err == nil {
		agentsFile = v
	} else if err != nil && !errors.Is(err, config.ErrNotFound) {
		slog.Warn("get 'agents_file' from config", "error", err)
	}
	if agentsFile != "" {
		paths := agent.DiscoverAgentsFiles(fs, cwd.Path(), agentsFile)
		if section := agent.LoadAgentsFiles(fs, paths); section != "" {
			ret.projectInstructions = section
			slog.Info("loaded project instructions", "file", agentsFile, "count", len(paths))
		}
	}

	ret.skillRegistry = skillRegistry
	ret.plansDir = filepath.Join(w.DataDir(ctx), "plans")
	ret.memoryPath = memoryPath
	ret.cwd = cwd
	ret.fs = fs
	ret.exec = executor
	ret.gitID = resolveGitIdentity(ctx, executor, cwd)
	ret.lsp = lsp
	ret.parser = parser
	ret.memoryDataPath = filepath.Join(w.DataDir(ctx), "memory")
	ret.defaultModel, err = pconfig.GetString("default_model")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("get 'model' from config", "error", err)
		}
		ret.defaultModel = defaultModel
	}
	if v, err := pconfig.GetInt("max_tool_output_bytes"); err == nil {
		ret.maxToolOutputBytes = v
	} else if err != nil && !errors.Is(err, config.ErrNotFound) {
		slog.Warn("get 'max_tool_output_bytes' from config", "error", err)
	}
	if v, err := pconfig.GetFloat("auto_compact_ratio"); err == nil {
		ret.autoCompactRatio = v
	} else if err != nil && !errors.Is(err, config.ErrNotFound) {
		slog.Warn("get 'auto_compact_ratio' from config", "error", err)
	}
	// Check for custom_provider config to add user-defined
	// OpenAI-compatible models to the registry.
	noti := w.Notifications(ctx)
	ret.modelRegistry = defaultRegistry
	if cpCfg, err := pconfig.GetConfig("custom_provider"); err == nil {
		customURL, _ := cpCfg.GetString("url")
		models, modelsErr := config.GetMapInt(cpCfg, "available_models")
		if modelsErr != nil && !errors.Is(modelsErr, config.ErrNotFound) {
			_, _ = noti.Notify(browserapi.LevelWarn,
				"custom_provider: failed to parse available_models: %v", modelsErr)
		}
		if len(models) > 0 {
			customStatic := llmregistry.NewStatic()
			for name, ctxWindow := range models {
				customStatic.Register(llmregistry.ModelEntry{
					Name:          name,
					Provider:      "custom_provider",
					ContextWindow: ctxWindow,
					BaseURL:       customURL,
				})
			}
			ret.modelRegistry = llmregistry.NewComposite(
				customStatic, defaultRegistry,
			)
		}
	} else if !errors.Is(err, config.ErrNotFound) {
		_, _ = noti.Notify(browserapi.LevelWarn,
			"custom_provider: invalid config: %v", err)
	}

	if _, ok := ret.modelRegistry.Get(ctx, ret.defaultModel); !ok {
		slog.Warn("default model not found in registry, falling back",
			"requested", ret.defaultModel,
			"fallback", openai.GPT5Dot4)
		ret.defaultModel = openai.GPT5Dot4
	}

	ret.cfg = defaultComponentCfg
	ret.cfg.MarkdownConfig.Parser = w.Parser(ctx)
	backgroundAttr, err := config.GetAttributes(pconfig, "background_attr")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("get 'background_attr' from extension config", "error", err)
		}
	}
	ret.cfg.ReceiveMessageStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.InputBox.PlaceholderConfig.BackgroundAttributes = backgroundAttr

	ret.cfg.InputBox.FrameAttr.Bg = backgroundAttr.Bg
	ret.cfg.InputBox.PlaceholderConfig.Bg = backgroundAttr.Bg
	ret.cfg.InputBox.ContentAttr.Bg = backgroundAttr.Bg

	ret.cfg.ReceiveMessageStringConfig.Bg = backgroundAttr.Bg

	sendMsgBackgroundAttr, err := config.GetAttributes(pconfig, "user_msg_background_attr")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("get 'user_msg_background_attr' from extension config", "error", err)
		}
		sendMsgBackgroundAttr = term.Attributes{Bg: tcell.ColorGray}
	}
	ret.cfg.SendMessageStringConfig.BackgroundAttributes = sendMsgBackgroundAttr
	ret.cfg.SendMessageStringConfig.Bg = sendMsgBackgroundAttr.Bg
	ret.cfg.ReasoningStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.ReasoningStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.ToolCallStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.ToolCallStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.ToolCallArgsStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.ToolCallArgsStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.ToolResultStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.ToolResultStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.ReasoningAnnotationStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.ReasoningAnnotationStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.ErrorStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.ErrorStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.WarningStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.WarningStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.PromptToolCallStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.PromptToolCallStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.SelectionConfig.TitleStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.SelectionConfig.TitleStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.SelectionConfig.OptionStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.SelectionConfig.OptionStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.SelectionConfig.CursorStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.SelectionConfig.CursorStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.SelectionConfig.HeaderStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.SelectionConfig.HeaderStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.SelectionConfig.DescStringConfig.BackgroundAttributes = backgroundAttr
	ret.cfg.SelectionConfig.DescStringConfig.Bg = backgroundAttr.Bg
	ret.cfg.BackgroundColor = backgroundAttr.Bg
	ret.backgroundAttr = backgroundAttr

	sendMsgAttr, err := config.GetAttributes(pconfig, "user_msg_attr")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("get 'user_msg_attr' from extension config", "error", err)
		}
	} else {
		ret.cfg.SendMessageStringConfig.Attributes = sendMsgAttr
	}
	recvMsgAttr, err := config.GetAttributes(pconfig, "assistant_msg_attr")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("get 'assistant_msg_attr' from extension config", "error", err)
		}
	} else {
		ret.cfg.ReceiveMessageStringConfig.Attributes = recvMsgAttr
	}

	inputBoxAttr, err := config.GetAttributes(pconfig, "input_box_attr")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("get 'input_box_attr' from extension config", "error", err)
		}
	} else {
		ret.cfg.InputBox.ContentAttr = inputBoxAttr
	}
	inputBoxPlaceholderAttr, err := config.GetAttributes(pconfig, "input_box_placeholder_attr")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("Error getting 'input_box_placeholder_attr'"+
				" from extension config", "error", err)
		}
	} else {
		ret.cfg.InputBox.PlaceholderConfig.Attributes = inputBoxPlaceholderAttr
	}

	inputBoxFrameAttr, err := config.GetAttributes(pconfig, "input_box_frame_attr")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("Error getting 'input_box_frame_attr' from extension config", "error", err)
		}
	} else {
		ret.cfg.InputBox.FrameAttr = inputBoxFrameAttr
	}

	ret.contextHintCfg = contextHintConfig{
		labelAttr: term.Attributes{Bg: backgroundAttr.Bg, Attrs: tcell.AttrDim},
		valueAttr: term.Attributes{Bg: backgroundAttr.Bg, Fg: tcell.ColorRed},
	}
	contextHintAttr, err := config.GetAttributes(pconfig, "context_hint_attr")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("get 'context_hint_attr' from extension config", "error", err)
		}
	} else {
		ret.contextHintCfg.valueAttr = contextHintAttr
		ret.contextHintCfg.valueAttr.Bg = backgroundAttr.Bg
		ret.contextHintCfg.labelAttr = contextHintAttr
		ret.contextHintCfg.labelAttr.Bg = backgroundAttr.Bg
		ret.contextHintCfg.labelAttr.Attrs |= tcell.AttrDim
	}

	ret.queryDefaultModel, err = pconfig.GetString("query_default_model")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("get 'query_default_model' from config", "error", err)
		}
		ret.queryDefaultModel = defaultModel
	}
	if _, ok := ret.modelRegistry.Get(ctx, ret.queryDefaultModel); !ok {
		first, hasFirst := firstModel(ret.modelRegistry)
		if !hasFirst {
			return nil, fmt.Errorf("query default model %q not found and registry is empty",
				ret.queryDefaultModel)
		}
		_, _ = noti.Notify(browserapi.LevelWarn,
			"query default model %q not found in registry, falling back to %q",
			ret.queryDefaultModel, first)
		ret.queryDefaultModel = first
	}

	ret.compactModel, err = pconfig.GetString("compact_model")
	if err != nil {
		if err != config.ErrNotFound {
			slog.Warn("get 'compact_model' from config", "error", err)
		}
		// empty string means "use the chat model" — no separate service needed.
	}
	if ret.compactModel != "" {
		if _, ok := ret.modelRegistry.Get(ctx, ret.compactModel); !ok {
			_, _ = noti.Notify(browserapi.LevelWarn,
				"compact model %q not found in registry, compaction will use the chat model",
				ret.compactModel)
			ret.compactModel = ""
		}
	}

	ret.clip, err = sysclip.NewRegister()
	if err != nil {
		slog.Warn("system clipboard unsupported", "error", err)
		ret.clip = clipboard.NewInMemory()
	}

	ret.db = db
	if auditEnabled, _ := pconfig.GetBool("audit_enabled"); auditEnabled {
		ret.auditStore = llm.NewAuditStore(ret.db)
	}
	ret.p = w.Interrupter(ctx)
	ret.o = w.ResourceOpener(ctx)
	ret.wm = w.WindowManager(ctx)
	ret.n = w.Notifications(ctx)
	ret.resources = make(map[string]string)

	ret.dialogueStore = dialogueStore

	if ret.compactModel != "" {
		ret.compactSvc, err = ret.newService(ret.compactModel)
		if err != nil {
			_, _ = noti.Notify(browserapi.LevelWarn,
				"failed to create backend for compact model %q (%v), compaction will use the chat model",
				ret.compactModel, err)
			ret.compactModel = ""
			ret.compactSvc = nil
		}
	}

	queryService, err := ret.newService(ret.queryDefaultModel)
	if err != nil {
		first, hasFirst := firstModel(ret.modelRegistry)
		if !hasFirst || first == ret.queryDefaultModel {
			return nil, fmt.Errorf("new backend for query dialogues: %v", err)
		}
		_, _ = noti.Notify(browserapi.LevelWarn,
			"failed to create backend for model %q (%v), falling back to %q",
			ret.queryDefaultModel, err, first)
		ret.queryDefaultModel = first
		queryService, err = ret.newService(first)
		if err != nil {
			return nil, fmt.Errorf("new backend for query dialogues (fallback %q): %v", first, err)
		}
	}
	queryEntry, _ := ret.modelRegistry.Get(ctx, ret.queryDefaultModel)
	ret.queryAgent = agent.NewAgent(
		queryService, ret.toolRegistry, ret.skillRegistry,
		ret.dialogueStore, agent.NoMemory(), agent.Config{
			MaxToolOutputBytes:  ret.maxToolOutputBytes,
			AutoCompactRatio:    ret.autoCompactRatio,
			CompactSvc:          ret.compactSvc,
			SystemPrompt:        agent.QuerySystemPrompt(ret.cwd) + agent.ProviderToolAddendum(queryEntry.Provider),
			ProjectInstructions: ret.projectInstructions,
			SessionKey:          "query",
			AgentID:             "query",
			Model:               ret.queryDefaultModel,
			Provider:            queryEntry.Provider,
			Workspace:           ret.cwd,
		},
	)

	// Sub-agent configuration.
	ret.agentsConfig = agent.NewConfig(
		[]agent.Definition{
			{
				ID:           "default",
				Name:         "Default Agent",
				Model:        ret.defaultModel,
				SystemPrompt: ret.systemPrompt,
				AllowAny:     true,
			},
		},
	)

	return ret, nil
}

// contextHintConfig holds styling for the post-turn context hint.
type contextHintConfig struct {
	labelAttr term.Attributes // separators and labels (·, context:, compacts at)
	valueAttr term.Attributes // numeric values (duration, token counts, percentages)
}

type aiEditorHandler struct {
	exit                atomic.Uint32
	modelRegistry       llmregistry.Registry
	defaultModel        string
	cfg                 dialoguetui.ComponentConfig
	backgroundAttr      term.Attributes
	contextHintCfg      contextHintConfig
	dialogueStore       dialoguemanager.Store
	queryAgent          *agent.Agent
	queryDefaultModel   string
	compactModel        string // empty means use the chat model
	compactSvc          llm.Service
	toolRegistry        *agent.Registry
	systemPrompt        string
	projectInstructions string
	agentsConfig        *agent.Cfg
	baseTools           []agent.Tool
	mcpManager          *runemcp.Manager
	sessionMgr          *agentools.SessionManager

	maxToolOutputBytes int
	autoCompactRatio   float64

	resources map[string]string
	clip      clipboard.Register
	// newClient, if non-nil, replaces openai.NewClient in
	// newLLMService. Intended for testing only.
	newClient          clientConstructor
	newAnthropicClient anthropic.ClientConstructor
	ed                 textapi.Editor
	wm                 browserapi.WindowManager
	n                  browserapi.Notifications
	o                  browserapi.ResourceOpener
	p                  term.Interrupter
	db                 storageapi.Service
	config             config.Config
	skillRegistry      *skills.SkillRegistry
	plansDir           string
	memoryDataPath     string
	exec               workspaceapi.Executor
	lsp                semanticapi.LSP
	parser             syntaxapi.Parser
	memoryPath         string
	cwd                workspaceapi.URI
	fs                 workspaceapi.FileSystem
	executor           workspaceapi.Executor
	gitID              gitIdentity

	auditStore *llm.AuditStore

	// generateDialogueID overrides the spawner's dialogue ID
	// generator. Testing only.
	generateDialogueID func(ctx context.Context, agentID string) string
	// generatePlanPath overrides the plan path generator. Testing only.
	generatePlanPath func(title string) string
	effortMu         sync.Mutex
	defaultEffort    llm.ReasoningEffort // global default applied to new chats/queries

	openChats sync.Map
	ctx       context.Context
	cancelCtx func()
}

func (h *aiEditorHandler) newService(model string) (llm.Service, error) {
	svc, err := newLLMService(h.config, h.modelRegistry, model, h.newClient, h.newAnthropicClient)
	if err != nil {
		return nil, err
	}
	if h.auditStore != nil {
		var provider string
		if entry, ok := h.modelRegistry.Get(h.ctx, model); ok {
			provider = entry.Provider
		}
		svc = llm.NewAuditService(svc, h.auditStore, model, provider)
	}
	return svc, nil
}

func (h *aiEditorHandler) getDefaultEffort() llm.ReasoningEffort {
	h.effortMu.Lock()
	defer h.effortMu.Unlock()
	return h.defaultEffort
}

func (h *aiEditorHandler) setDefaultEffort(e llm.ReasoningEffort) {
	h.effortMu.Lock()
	h.defaultEffort = e
	h.effortMu.Unlock()
	h.queryAgent.SetEffort(e)
}

func (h *aiEditorHandler) Handle(ctx context.Context, ev textapi.Event) (exit bool) {
	uexit := h.exit.Load()
	exit = uexit != 0
	if exit {
		return
	}
	if ev.URI == (workspaceapi.URI{}) {
		return
	}

	var err error
	switch ev.Type {
	case textapi.EventTypeFlush, textapi.EventTypeOpen:
		content, ok := h.resources[ev.URI.String()]
		if ok {
			err = h.queryAgent.AddContextResource(ctx, ev.URI, content)
		}
		h.resources[ev.URI.String()] = ev.Content
	case textapi.EventTypeFocus:
		content, ok := h.resources[ev.URI.String()]
		if !ok {
			slog.Warn("could not find resource on focus event", "uri", ev.URI)
			return
		}
		err = h.queryAgent.AddContextResource(ctx, ev.URI, content)
	case textapi.EventTypeUnfocus:
		err = h.queryAgent.RemoveContextResource(ctx, ev.URI)
	case textapi.EventTypeClose:
		delete(h.resources, ev.URI.String())
	}

	if err != nil {
		slog.Error("query agent context resource", "uri", ev.URI, "error", err)
	}
	return
}

func (h *aiEditorHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (err error) {
	switch cmd.Name {
	case commandQuery:
		return h.handleQuery(cmd)
	case commandChat:
		return h.handleChat(cmd)
	}

	return nil
}

func (h *aiEditorHandler) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], error,
) {
	slog.Debug("complete with", "name", name, "args", args)

	// Strip --all flag from args for length checks, pass it through to the completer.
	showAll := false
	filtered := args
	for i, a := range filtered {
		if a == "--all" {
			showAll = true
			filtered = append(filtered[:i], filtered[i+1:]...)
			break
		}
	}

	switch name {
	case commandChat:
		switch len(filtered) {
		case 0, 1:
			return h.completeWithDialoguesIterator(ctx, showAll)
		case 2:
			return h.completeWithModelsIterator(ctx)
		default:
			return iterator.FromSlice[string](nil), nil
		}
	default:
		return iterator.FromSlice[string](nil), nil
	}
}

func (h *aiEditorHandler) Close() error {
	closing := h.exit.CompareAndSwap(0, 1)
	if !closing {
		return nil
	}
	if h.sessionMgr != nil {
		_ = h.sessionMgr.Close()
	}
	h.cancelCtx()
	return h.mcpManager.Close()
}

func (h *aiEditorHandler) newDialogueComponent() *dialoguetui.Component {
	return dialoguetui.NewComponent(h.cfg)
}

func (h *aiEditorHandler) newAgentShell() textapi.REPLHandler {
	opts := []agentshell.Option{
		agentshell.WithMCPInfo(h.mcpManager),
		agentshell.WithHistorySystemPrompt(true),
		agentshell.WithEffort(h.getDefaultEffort, h.setDefaultEffort),
		agentshell.WithServiceFactory(h.newService),
	}
	if h.auditStore != nil {
		opts = append(opts, agentshell.WithAuditStore(h.auditStore))
	}
	return agentshell.New(
		h.wm,
		nil,
		h.modelRegistry, h.defaultModel,
		h.dialogueStore, h.toolRegistry, h.agentsConfig,
		h.config,
		h.skillRegistry, h.cwd, h.fs,
		h.db, h.exec, h.lsp, h.parser, h.n,
		h.memoryDataPath,
		opts...,
	)
}

func (h *aiEditorHandler) handleChat(cmd textapi.Command) error {
	cmd.Args = filterAllFlag(cmd.Args)
	if len(cmd.Args) > 0 {
		if _, ok := h.modelRegistry.Get(h.ctx, cmd.Args[0]); ok {
			return errors.New("model must be passed as a second argument to a dialogue ID, " +
				"check command manual for more details")
		}
	}
	model := h.defaultModel
	if len(cmd.Args) > 1 {
		model = cmd.Args[1]
		if _, ok := h.modelRegistry.Get(h.ctx, model); !ok {
			return fmt.Errorf("model '%s' is not supported. Available models: %s",
				model, availableModelsString(h.modelRegistry))
		}
	}

	backendService, err := h.newService(model)
	if err != nil {
		return fmt.Errorf("new backend: %v", err)
	}

	mu := new(sync.Mutex)
	ctx, cancel := context.WithCancel(h.ctx)

	d, err := h.getDialogue(ctx, h.dialogueStore, cmd)
	if err != nil {
		cancel()
		return err
	}

	// Create per-session spawner for sub-agent support.
	sessionKey := d.ID
	agentID := "default"
	serviceFactory := func(
		model string,
	) (llm.Service, string, error) {
		entry, ok := h.modelRegistry.Get(h.ctx, model)
		if !ok {
			return nil, "", fmt.Errorf("model %q not found", model)
		}
		svc, err := h.newService(model)
		if err != nil {
			return nil, "", err
		}
		return svc, entry.Provider, nil
	}
	// Build a separate agentshell for the command adapter. It only needs
	// the base tools for display (e.g. /tools); it carries no mutable state.
	cmdRegistry := agent.NewRegistry(h.baseTools...)
	cmdShellOpts := []agentshell.Option{
		agentshell.WithMCPInfo(h.mcpManager),
		agentshell.WithServiceFactory(h.newService),
	}
	if h.auditStore != nil {
		cmdShellOpts = append(cmdShellOpts, agentshell.WithAuditStore(h.auditStore))
	}
	cmdShell := agentshell.New(
		h.wm, backendService,
		h.modelRegistry, model,
		h.dialogueStore, cmdRegistry, h.agentsConfig,
		h.config,
		h.skillRegistry, h.cwd, h.fs,
		h.db, h.exec, h.lsp, h.parser, h.n,
		h.memoryDataPath,
		cmdShellOpts...,
	)

	// Build command adapter with session-aware overrides.
	var comp *dialoguetui.Component
	hs := &hintSlot{} // shared with syncComponent for hint preservation during compaction
	adapter := &commandAdapter{
		handler:            cmdShell,
		dialogueID:         d.ID,
		wm:                 h.wm,
		store:              h.dialogueStore,
		modelRegistry:      h.modelRegistry,
		config:             h.config,
		newClient:          h.newClient,
		newAnthropicClient: h.newAnthropicClient,
		currentModel:       model,
		skillRegistry:      h.skillRegistry,
		auditStore:         h.auditStore,
		resetFn: func() {
			mu.Lock()
			comp.Reset()
			mu.Unlock()
		},
		compactFn: func(msgs []llm.Message) {
			mu.Lock()
			comp.Reset()
			pending := make(map[string]llm.ToolCall)
			for _, msg := range msgs {
				addMessage(comp, msg, pending)
			}
			// Re-add the status hint if one is active. Reset and
			// AddSendMessageMarkdown (called during replay) both
			// remove it, so we restore it after all messages are
			// replayed to keep the progress animation visible
			// during compaction.
			if hs.comp != nil {
				comp.AddReceiveMessageHint(hs.comp, hs.conf)
			}
			mu.Unlock()
			_ = h.p.Interrupt(context.Background())
		},
	}

	// Create component with tab-completion wired to the adapter.
	cfg := h.cfg
	cfg.InputBox.WordCompleter = makeCommandCompleter(ctx, adapter)
	comp = dialoguetui.NewComponent(cfg)

	// Replay dialogue history.
	pendingTools := make(map[string]llm.ToolCall)
	for _, msg := range d.Messages {
		addMessage(comp, msg, pendingTools)
	}

	dhandler, tx, rx := dialoguetui.Handler(ctx, mu, comp, h.p,
		dialoguetui.WithCommands(adapter),
		dialoguetui.WithCloseFunc(cancel),
	)

	// Build the agent registry now that tx is available for ask_user.
	prompter := &tuiPrompter{tx: tx, noti: h.n}
	askUser := agentools.NewAskUser(prompter)
	requestSkill := agentools.NewRequestSkill(prompter)
	exitPlan := agentools.NewExitPlan(h.plansDir, prompter)
	if h.generatePlanPath != nil {
		exitPlan.GeneratePlanPath = h.generatePlanPath
	}
	memRecaller := memory.NewRecaller(h.fs, h.executor, h.memoryPath)
	spawner := agent.NewGoroutineSpawner(
		h.dialogueStore, serviceFactory,
		h.agentsConfig,
		h.skillRegistry,
		memRecaller,
		h.projectInstructions,
		sessionKey, agentID,
	)
	spawner.GenerateDialogueID = h.generateDialogueID
	childEvents := make(chan agent.ChildEvent, 64)
	sessionTools := agentools.SessionTools(spawner, spawner.ListAgents(), childEvents, h.skillRegistry)
	skillTool := agentools.NewSkillTool(h.skillRegistry, spawner, childEvents)
	taskStore := taskstore.New()
	progressUpdater := &tuiProgressUpdater{tx: tx}
	taskTools := agentools.NewTaskTools(taskStore, progressUpdater)
	allTools := make([]agent.Tool, 0, len(h.baseTools)+len(sessionTools)+len(taskTools)+4)
	allTools = append(allTools, h.baseTools...)
	allTools = append(allTools, sessionTools...)
	allTools = append(allTools, askUser)
	allTools = append(allTools, requestSkill)
	allTools = append(allTools, exitPlan)
	allTools = append(allTools, skillTool) // overrides nil-spawner skill tool from baseTools
	allTools = append(allTools, taskTools...)
	chatRegistry := agent.NewRegistry(allTools...)
	chatRegistry.CopyOverridesFrom(h.toolRegistry)
	chatRegistry.RegisterOverrides(openai.LLMProvider,
		agentools.NewUpdatePlan(progressUpdater),
		agentools.NewRequestUserInput(prompter),
	)
	chatRegistry.RegisterExclusions(openai.LLMProvider,
		"TaskCreate", "TaskUpdate", "TaskGet", "TaskList", "ask_user_question",
	)
	spawner.SetRegistry(chatRegistry)

	syncComp := syncComponent{mu: mu, comp: comp, h: h, hintSlot: hs}
	h.openChats.Store(d.ID, syncComp)

	chatEntry, _ := h.modelRegistry.Get(h.ctx, model)
	chatAgent := agent.NewAgent(
		backendService, chatRegistry, h.skillRegistry,
		h.dialogueStore, memRecaller, agent.Config{
			MaxToolOutputBytes:  h.maxToolOutputBytes,
			AutoCompactRatio:    h.autoCompactRatio,
			CompactSvc:          h.compactSvc,
			SystemPrompt:        h.systemPrompt + agent.ProviderToolAddendum(chatEntry.Provider),
			ProjectInstructions: h.projectInstructions,
			SessionKey:          sessionKey,
			AgentID:             agentID,
			Model:               model,
			Provider:            chatEntry.Provider,
			Workspace:           h.cwd,
		},
	)
	if effort := h.getDefaultEffort(); effort != "" {
		chatAgent.SetEffort(effort)
	}
	adapter.agent = chatAgent

	handler, msgRx := h.wrapDialogueHandler(ctx, syncComp, dhandler, rx)
	go createAgentCompletions(ctx, cancel, tx, msgRx, chatAgent, spawner, childEvents, h.skillRegistry, d.ID, syncComp, h.n,
		makeOnCompacted(h.dialogueStore, adapter.compactFn))

	bhandler := browserapi.FuncHandler(handler, func() error {
		cancel()
		h.openChats.Delete(d.ID)
		return nil
	})
	uri, err := getModelUri(d.ID, model)
	if err != nil {
		return err
	}
	const icon = '󱫆'
	tab, err := h.wm.Tab(uri, icon, uri.String(), bhandler)
	if err != nil {
		return fmt.Errorf("create tab: %v", err)
	}

	if err := h.wm.SetWindowContent(cmd.Window, tab); err != nil {
		return fmt.Errorf("window set content: %v", err)
	}
	return nil
}

func getModelUri(id, model string) (workspaceapi.URI, error) {
	model = strings.ReplaceAll(model, "/", "_") // i.e. hf.co/org/model
	model = strings.ReplaceAll(model, ":", "_") // i.e. llama4:scout
	model = url.PathEscape(model)
	uriStr := fmt.Sprintf("rune-agent://%s/%s", model, id)
	return workspaceapi.ParseURI(uriStr)
}

func (h *aiEditorHandler) handleQuery(cmd textapi.Command) error {
	mu := new(sync.Mutex)
	comp := h.newDialogueComponent()
	queryID := strconv.Itoa(rand.Int())
	ctx, cancel := context.WithCancel(h.ctx)
	dhandler, tx, rx := dialoguetui.Handler(ctx, mu, comp, h.p)

	query := strings.Join(cmd.Args, " ")
	if query == "" {
		query = " " // avoid library omiting Content field when zero-valued
	}
	msg := llm.Message{Content: query, Role: llm.RoleUser}
	addMessage(comp, msg, nil)

	syncComp := syncComponent{mu: mu, comp: comp, h: h, hintSlot: &hintSlot{}}

	qrx := make(chan dialoguetui.SubmitMessage)
	handler, msgRx := h.wrapDialogueHandler(ctx, syncComp, dhandler, qrx)

	// wrap rx to enable sending query and so get
	// context cancelation for free
	go func() {
		// do not store queries in store after user is done
		defer h.dialogueStore.Delete(ctx, queryID) //nolint:errcheck

		select {
		case qrx <- dialoguetui.SubmitMessage{Text: query}:
		case <-ctx.Done():
			return
		}
		for {
			select {
			case msg := <-rx:
				select {
				case qrx <- msg:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	serviceFactory := func(model string) (llm.Service, string, error) {
		entry, ok := h.modelRegistry.Get(h.ctx, model)
		if !ok {
			return nil, "", fmt.Errorf("model %q not found", model)
		}
		svc, err := h.newService(model)
		if err != nil {
			return nil, "", err
		}
		return svc, entry.Provider, nil
	}
	spawner := agent.NewGoroutineSpawner(
		h.dialogueStore, serviceFactory,
		h.agentsConfig,
		h.skillRegistry,
		agent.NoMemory(),
		h.projectInstructions,
		queryID, "query",
	)
	spawner.SetRegistry(agent.NewRegistry(h.baseTools...))
	spawner.GenerateDialogueID = h.generateDialogueID
	childEvents := make(chan agent.ChildEvent, 64)

	go createAgentCompletions(ctx, cancel, tx, msgRx,
		h.queryAgent, spawner, childEvents, h.skillRegistry, queryID, syncComp, h.n, nil)

	var err error
	var win browserapi.Window
	bhandler := browserapi.FuncHandler(handler, func() error {
		cancel()
		if win != nil {
			return h.wm.CloseWindow(win)
		}
		return nil
	})
	floating := browserapi.FuncFloating(bhandler, func() (int, int) {
		mu.Lock()
		defer mu.Unlock()
		const width = 100
		return width, comp.Height(width)
	})
	floatingConfig := browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	}
	win, err = h.wm.Floating(floating, floatingConfig)
	if err != nil {
		return fmt.Errorf("floating window: %v", err)
	}
	return nil
}

func (h *aiEditorHandler) getDialogue(
	ctx context.Context, dialogueStore dialoguemanager.Store, cmd textapi.Command,
) (dialoguemanager.Dialogue, error) {
	dialogueID := parseDialogueID(cmd)
	if dialogueID == "" {
		// No user-provided ID — generate a unique one.
		dialogueID = dialoguemanager.GenerateUniqueID(ctx, dialogueStore, "")
	}
	d, err := dialogueStore.Get(ctx, dialogueID)
	if err != nil {
		if !errors.Is(err, storageapi.ErrNotFound) {
			return dialoguemanager.Dialogue{}, fmt.Errorf("get dialogue from store: %w", err)
		}
		d.ID = dialogueID
	}
	return d, nil
}

func (h *aiEditorHandler) completeWithDialoguesIterator(ctx context.Context, showAll bool) (
	iterator.Iterator[string], error,
) {
	listIt, err := h.dialogueStore.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("dialogue store list: %w", err)
	}

	all, err := iterator.ToSlice(ctx, listIt)
	if err != nil {
		return nil, fmt.Errorf("dialogue store list collect: %w", err)
	}

	// dialogueTier assigns a sort bucket:
	//   0 = current workspace or known worktree (local)
	//   1 = empty workspace (legacy)
	//   2 = foreign workspace
	dialogueTier := func(d dialoguemanager.DialogueHeader) int {
		ws, hasWS := d.Workspace()
		switch {
		case hasWS && ws.Equal(h.cwd):
			return 0
		case hasWS && h.gitID.worktrees[ws.String()] != "":
			return 0
		case !hasWS:
			return 1
		default:
			return 2
		}
	}

	// Filter: drop empty IDs, sub-agents, and (unless showAll)
	// foreign-workspace dialogues.
	filtered := all[:0]
	for _, d := range all {
		if d.ID == "" || d.SubAgent {
			continue
		}
		if !showAll && dialogueTier(d) == 2 {
			continue
		}
		filtered = append(filtered, d)
	}

	// Sort: local first, then legacy, then foreign.
	// Within each tier, preserve the store's UpdatedAt-descending order
	// via a stable sort.
	sort.SliceStable(filtered, func(i, j int) bool {
		return dialogueTier(filtered[i]) < dialogueTier(filtered[j])
	})

	// Map to display strings.
	out := make([]string, len(filtered))
	for i, d := range filtered {
		ws, hasWS := d.Workspace()
		switch {
		case !hasWS || ws.Equal(h.cwd):
			// Legacy dialogues are tagged so the user can tell they
			// have no workspace association.
			if !hasWS {
				out[i] = "<legacy>:" + d.ID
			} else {
				out[i] = d.ID
			}
		case h.gitID.worktrees[ws.String()] != "":
			out[i] = h.gitID.worktrees[ws.String()] + ":" + d.ID
		default:
			out[i] = ws.String() + ":" + d.ID
		}
	}
	return iterator.FromSlice(out), nil
}

func (h *aiEditorHandler) completeWithModelsIterator(ctx context.Context) (
	iterator.Iterator[string], error,
) {
	return iterator.Map(h.modelRegistry.Models(), func(e llmregistry.ModelEntry) string {
		return e.Name
	}), nil
}

func (h *aiEditorHandler) wrapDialogueHandler(
	ctx context.Context, comp syncComponent,
	dhandler tui.Handler, rx <-chan dialoguetui.SubmitMessage,
) (tui.Handler, <-chan completionRequest) {
	ret := make(chan completionRequest)

	// wrap dialogue.Handler's rx chan to add adhoc
	// cancelation of completion requests
	var cancel func()
	mu := new(sync.Mutex)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-rx:
				reqCtx, cancelFn := context.WithCancel(ctx)
				mu.Lock()
				cancel = cancelFn
				mu.Unlock()
				select {
				case ret <- completionRequest{msg: msg.Text, skillName: msg.SkillName, ctx: reqCtx}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// wrap it for ctrl-c cancelation of context
	return handler.Wrap(dhandler, func(ev term.Event) (exit bool, handled bool) {
		if ev.Ch == 'c' && ev.Mod == term.ModCtrl {
			mu.Lock()
			cancelFn := cancel
			cancel = nil
			mu.Unlock()
			if cancelFn != nil {
				cancelFn()
				_, _ = comp.h.n.Notify(browserapi.LevelInfo, "canceled completion request")
			}
			handled = true
			return
		}
		return dhandler.Handle(ev)
	}), ret
}

// parseDialogueID extracts a user-provided dialogue ID from the
// command args. Returns empty string when no ID was provided,
// signaling that a new unique ID should be generated.
func parseDialogueID(cmd textapi.Command) string {
	var id string
	for _, arg := range cmd.Args {
		if arg == "--all" || arg == "" {
			continue
		}
		id = arg
		break
	}
	if id == "" {
		return ""
	}
	// Strip workspace prefix (e.g. "worktree-name:myid" → "myid").
	if i := strings.IndexByte(id, ':'); i >= 0 {
		id = id[i+1:]
	}
	return id
}

// filterAllFlag returns args with "--all" removed.
func filterAllFlag(args []string) (out []string) {
	for _, a := range args {
		if a != "--all" {
			out = append(out, a)
		}
	}
	return out
}

// baseDialogueID strips the archive suffix from an archived dialogue ID.
// Handles both "foo-archived" and "foo-archived-2" formats produced by NextArchivedID.
func baseDialogueID(archivedID string) string {
	if i := strings.LastIndex(archivedID, "-archived"); i >= 0 {
		return archivedID[:i]
	}
	return archivedID
}

func replayMessages(d dialoguemanager.Dialogue) []llm.Message {
	msgs := make([]llm.Message, 0, len(d.Messages)+1)
	if len(d.Messages) > 0 {
		msgs = append(msgs, d.Messages[0])
	}
	if d.ApprovedPlan != nil {
		msgs = append(msgs, llm.Message{
			Role:    llm.RoleUser,
			Content: fmt.Sprintf("Plan approved. Saved to %s\n\n%s", d.ApprovedPlan.Path, d.ApprovedPlan.Body),
		})
	}
	if len(d.Messages) > 1 {
		msgs = append(msgs, d.Messages[1:]...)
	}
	return msgs
}

func addMessage(c *dialoguetui.Component, msg llm.Message, pendingTools map[string]llm.ToolCall) {
	switch msg.Role {
	case llm.RoleAssistant:
		if msg.ReasoningContent != "" {
			c.AddReasoningChunk(msg.ReasoningContent)
		}
		if msg.Content != "" {
			c.AddReceiveMessageChunk(msg.Content)
			c.AddReceiveMessageBreak()
		}
		for _, call := range msg.ToolCalls {
			if pendingTools != nil {
				pendingTools[call.ID] = call
			}
		}
	case llm.RoleUser:
		if replayed, ok := parseStoredCommandMessage(msg.Content); ok {
			c.AddSendMessage(replayed)
		} else if strings.HasPrefix(msg.Content, agent.CompactSummaryPrefix) ||
			strings.HasPrefix(msg.Content, "Plan approved. Saved to ") {
			c.AddSendMessageMarkdown(msg.Content)
		} else {
			c.AddSendMessage(msg.Content)
		}
	case llm.RoleSystem:
	case llm.RoleTool:
		id := msg.ToolCallID
		name := "tool"
		args := ""
		if pendingTools != nil {
			if call, ok := pendingTools[id]; ok {
				name = call.Function.Name
				args = call.Function.Arguments
				delete(pendingTools, id)
			}
		}
		if name == "update_plan" {
			var parsed struct {
				Plan []struct {
					Step   string `json:"step"`
					Status string `json:"status"`
				} `json:"plan"`
			}
			if err := json.Unmarshal([]byte(args), &parsed); err == nil {
				for i, step := range parsed.Plan {
					c.UpdateTaskProgress(dialoguetui.ProgressTaskEntry{
						ID:      fmt.Sprintf("plan:%d", i),
						Subject: step.Step,
						Status:  step.Status,
					})
				}
				for i := len(parsed.Plan); i < 32; i++ {
					c.UpdateTaskProgress(dialoguetui.ProgressTaskEntry{
						ID:     fmt.Sprintf("plan:%d", i),
						Status: "deleted",
					})
				}
				return
			}
		}
		c.AddToolCall(id, name, args, "")
		c.CompleteToolCall(id, name, args, "", msg.Content, false)
	}
}

func firstModel(reg llmregistry.Registry) (string, bool) {
	it := reg.Models()
	defer func() { _ = it.Close() }()
	e, ok := it.Next(context.Background())
	if !ok {
		return "", false
	}
	return e.Name, true
}

func availableModelsString(reg llmregistry.Registry) string {
	it := reg.Models()
	defer func() { _ = it.Close() }()
	var names []string
	for {
		e, ok := it.Next(context.Background())
		if !ok {
			break
		}
		names = append(names, e.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

type completionRequest struct {
	msg       string
	skillName string
	ctx       context.Context
}

const truncatedTurnHint = "At high and max/xhigh effort levels, models may think more extensively and can be more likely to exhaust the max_tokens budget. Consider increasing max_tokens to give the model more room (/max_tokens 64000), or lowering the effort level (/effort medium)."

type turnOutcome uint8

const (
	turnOutcomeNone turnOutcome = iota
	turnOutcomeCompleted
	turnOutcomeTruncated
	turnOutcomeCanceled
	turnOutcomeError
)

func notifyTurnOutcome(noti browserapi.Notifications, outcome turnOutcome, reason string) {
	var level browserapi.NotificationLevel
	var msg string
	switch outcome {
	case turnOutcomeCompleted:
		level = browserapi.LevelSuccess
		msg = "Turn completed"
	case turnOutcomeTruncated:
		level = browserapi.LevelWarn
		msg = "Turn stopped after reaching the token limit"
	case turnOutcomeCanceled:
		return
	case turnOutcomeError:
		level = browserapi.LevelError
		if reason == "" {
			msg = "Turn failed"
		} else {
			msg = fmt.Sprintf("Turn failed: %s", reason)
		}
	default:
		return
	}
	if _, err := noti.Notify(level, "%s", msg); err != nil {
		slog.Error("notify", "error", err)
	}
}

func outcomeAndReasonForFinishReason(reason llm.FinishReason) (turnOutcome, string) {
	switch reason {
	case llm.FinishReasonStop:
		return turnOutcomeCompleted, ""
	case llm.FinishReasonLength:
		return turnOutcomeTruncated, ""
	case llm.FinishReasonToolCall:
		return turnOutcomeError, "the model stopped while requesting tool calls"
	case llm.FinishReasonContentFilter:
		return turnOutcomeError, "the response was blocked by a content filter"
	case llm.FinishReasonNull:
		return turnOutcomeError, "the response ended unexpectedly"
	default:
		return turnOutcomeError, fmt.Sprintf("unexpected finish reason: %s", reason)
	}
}

// makeOnCompacted returns a callback that fetches the compacted dialogue
// from the store and replays it in the TUI via compactFn. It is used by
// createAgentCompletions to handle EventCompacted from both the main
// agent loop and child (sub-agent) events.
func makeOnCompacted(store dialoguemanager.Store, compactFn func([]llm.Message)) func(string) {
	return func(dialogueID string) {
		compacted, err := store.Get(context.Background(), dialogueID)
		if err != nil {
			slog.Error("compacted: fetch dialogue", "id", dialogueID, "error", err)
			return
		}
		compactFn(replayMessages(compacted))
	}
}

func createAgentCompletions(
	ctx context.Context, cancel func(),
	tx chan<- dialoguetui.MessageEvent, rx <-chan completionRequest,
	ag *agent.Agent, spawner agent.Spawner,
	childEvents <-chan agent.ChildEvent,
	skillRegistry *skills.SkillRegistry,
	id string,
	syncComp syncComponent,
	noti browserapi.Notifications,
	onCompacted func(dialogueID string),
) {
	// Forward child events from sub-agents to the TUI.
	// The goroutine must exit before we close tx to avoid
	// sending on a closed channel.
	var childWg sync.WaitGroup
	childWg.Go(func() {
		for {
			select {
			case <-ctx.Done():
				return
			case cev, ok := <-childEvents:
				if !ok {
					return
				}
				var msg dialoguetui.MessageEvent
				switch cev.Event.Type {
				case agent.EventToolCall:
					msg = dialoguetui.MessageEvent{
						Type:             dialoguetui.MessageEventToolCall,
						ToolCallID:       cev.Event.ToolCallID,
						ToolName:         cev.Event.ToolName,
						ToolArgs:         cev.Event.ToolArgs,
						ToolSummary:      cev.Event.ToolSummary,
						ToolStartTime:    cev.Event.ToolStartTime,
						ParentToolCallID: cev.ParentToolCallID,
					}
				case agent.EventToolResult:
					msg = dialoguetui.MessageEvent{
						Type:             dialoguetui.MessageEventToolResult,
						ToolCallID:       cev.Event.ToolCallID,
						ToolName:         cev.Event.ToolName,
						ToolArgs:         cev.Event.ToolArgs,
						ToolSummary:      cev.Event.ToolSummary,
						ToolOutput:       cev.Event.ToolOutput,
						IsError:          cev.Event.IsError,
						ToolDuration:     cev.Event.ToolDuration,
						ParentToolCallID: cev.ParentToolCallID,
					}
				case agent.EventToolsDropped:
					msg = dialoguetui.MessageEvent{
						Type:               dialoguetui.MessageEventToolsDropped,
						DroppedToolCallIDs: cev.Event.DroppedToolCallIDs,
					}
				case agent.EventMemoryRecall:
					msg = agentMemoriesToTUI(cev.Event)
				case agent.EventCompacted:
					if cev.Event.ArchivedDialogueID != "" && onCompacted != nil {
						dialogueID := baseDialogueID(cev.Event.ArchivedDialogueID)
						onCompacted(dialogueID)
					}
					continue
				case agent.EventDone:
					msg = dialoguetui.MessageEvent{
						Type:             dialoguetui.MessageEventChildResult,
						ParentToolCallID: cev.ParentToolCallID,
						ToolOutput:       cev.Event.Text,
						IsError:          cev.Event.IsError,
					}
				default:
					continue
				}
				select {
				case tx <- msg:
				case <-ctx.Done():
					return
				}
			}
		}
	})
	defer func() {
		cancel()
		childWg.Wait()
		close(tx)
	}()

	for {
		var req completionRequest
		select {
		case <-ctx.Done():
			return
		case req = <-rx:
		}
		// Signal the dialogue handler that the agent is busy so
		// follow-up messages are queued instead of sent directly.
		select {
		case tx <- dialoguetui.MessageEvent{Type: dialoguetui.MessageEventBusy, Busy: true}:
		case <-ctx.Done():
			return
		}
		hint := syncComp.addStatusHint()

		// Agent-type skills: spawn a sub-agent whose events are
		// rendered as top-level (tool calls, text, reasoning all
		// visible in the TUI).
		var it iterator.Iterator[agent.Event]
		if req.skillName != "" {
			if skill, ok := skillRegistry.Get(req.skillName); ok && skill.Type == "agent" {
				var allowedTools []string
				if skill.AllowedTools != "" {
					allowedTools = strings.Fields(skill.AllowedTools)
				}
				handle, skillErr := spawner.Run(req.ctx, agent.RunRequest{
					Label:        skill.Name,
					Model:        ag.Model(),
					Message:      req.msg,
					AllowedTools: allowedTools,
					SystemPrompt: skill.Body,
				})
				if skillErr != nil {
					select {
					case tx <- dialoguetui.MessageEvent{Type: dialoguetui.MessageEventError, Text: skillErr.Error()}:
					case <-ctx.Done():
					}
					select {
					case tx <- dialoguetui.MessageEvent{Type: dialoguetui.MessageEventBreak}:
					case <-ctx.Done():
					}
					_, _ = noti.Notify(browserapi.LevelError, "skill %s: %v", skill.Name, skillErr)
					syncComp.removeStatusHint(hint)
					continue
				}
				it = handle.Events
			}
		}
		if it == nil {
			var runOpts []agent.RunOption
			if req.skillName != "" {
				runOpts = append(runOpts, agent.WithSkillName(req.skillName))
			}
			it = ag.Run(req.ctx, id, req.msg, runOpts...)
		}
		var lastUsage agent.Event // track last usage event for post-turn hint
		func() {
			defer it.Close() //nolint:errcheck
			breakSent := false
			// Always signal turn completion to the TUI when we
			// exit, even on context cancellation or abnormal agent
			// exit. AddReceiveMessageBreak is idempotent; a
			// duplicate after EventDone is harmless. This ensures
			// the turn's spinner animation stops and any dropped
			// tool-result events are cleaned up by completeRunning.
			defer func() {
				if breakSent {
					return
				}
				select {
				case tx <- dialoguetui.MessageEvent{Type: dialoguetui.MessageEventBreak}:
				case <-ctx.Done():
				}
			}()
			outcome := turnOutcomeNone
			outcomeReason := ""
			defer func() {
				if outcome == turnOutcomeNone {
					switch {
					case errors.Is(ctx.Err(), context.Canceled), errors.Is(req.ctx.Err(), context.Canceled):
						outcome = turnOutcomeCanceled
						outcomeReason = string(llm.FinishReasonNull)
					}
				}
				notifyTurnOutcome(noti, outcome, outcomeReason)
			}()
			for {
				ev, ok := it.Next(req.ctx)
				if !ok {
					break
				}
				switch ev.Type {
				case agent.EventMemoryRecall:
					select {
					case tx <- agentMemoriesToTUI(ev):
					case <-ctx.Done():
						return
					}
				case agent.EventInferenceStart:
					hint.setPhase(phaseSending)
				case agent.EventInferenceReady:
					hint.setPhase(phaseThinking)
				case agent.EventFirstContent:
					hint.setPhase(phaseReceiving)
				case agent.EventReasoning:
					if ev.Reasoning == "" {
						continue
					}
					select {
					case tx <- dialoguetui.MessageEvent{
						Type: dialoguetui.MessageEventReasoning,
						Text: ev.Reasoning,
					}:
					case <-ctx.Done():
						return
					}
				case agent.EventText:
					if ev.Text == "" {
						continue
					}
					select {
					case tx <- dialoguetui.MessageEvent{
						Type: dialoguetui.MessageEventText,
						Text: ev.Text,
					}:
					case <-ctx.Done():
						return
					}
				case agent.EventToolsStart:
					hint.setPhase(phaseToolCalling)
				case agent.EventToolCall:
					select {
					case tx <- dialoguetui.MessageEvent{
						Type:          dialoguetui.MessageEventToolCall,
						ToolCallID:    ev.ToolCallID,
						ToolName:      ev.ToolName,
						ToolArgs:      ev.ToolArgs,
						ToolSummary:   ev.ToolSummary,
						ToolStartTime: ev.ToolStartTime,
					}:
					case <-ctx.Done():
						return
					}
				case agent.EventToolResult:
					select {
					case tx <- dialoguetui.MessageEvent{
						Type:         dialoguetui.MessageEventToolResult,
						ToolCallID:   ev.ToolCallID,
						ToolName:     ev.ToolName,
						ToolArgs:     ev.ToolArgs,
						ToolSummary:  ev.ToolSummary,
						ToolOutput:   ev.ToolOutput,
						IsError:      ev.IsError,
						ToolDuration: ev.ToolDuration,
					}:
					case <-ctx.Done():
						return
					}
				case agent.EventToolsDropped:
					select {
					case tx <- dialoguetui.MessageEvent{
						Type:               dialoguetui.MessageEventToolsDropped,
						DroppedToolCallIDs: ev.DroppedToolCallIDs,
					}:
					case <-ctx.Done():
						return
					}
				case agent.EventRateLimitWarning:
					hint.setPhase(phaseRateLimited)
					if ev.RateLimit != nil {
						select {
						case tx <- dialoguetui.MessageEvent{
							Type: dialoguetui.MessageEventWarning,
							Text: ev.RateLimit.Message,
						}:
						case <-ctx.Done():
							return
						}
					}
				case agent.EventUsageUpdate:
					lastUsage = ev
					u := ev.Usage
					hint.setTokens(u.TokensSent, u.TokensReceived)
				case agent.EventCompacting:
					hint.setPhase(phaseCompacting)
				case agent.EventCompacted:
					hint.setPhase(phaseSending)
					onCompacted(id)
					if ev.ArchivedDialogueID != "" {
						select {
						case tx <- dialoguetui.MessageEvent{
							Type: dialoguetui.MessageEventWarning,
							Text: fmt.Sprintf(
								"Conversation compacted. Old conversation stored as **%s**.",
								ev.ArchivedDialogueID,
							),
						}:
						case <-ctx.Done():
							return
						}
					}
				case agent.EventDone:
					outcome, outcomeReason = outcomeAndReasonForFinishReason(ev.FinishReason)
					if ev.FinishReason == llm.FinishReasonLength {
						select {
						case tx <- dialoguetui.MessageEvent{
							Type: dialoguetui.MessageEventWarning,
							Text: truncatedTurnHint,
						}:
						case <-ctx.Done():
							return
						}
					}
					select {
					case tx <- dialoguetui.MessageEvent{
						Type: dialoguetui.MessageEventBreak,
					}:
						breakSent = true
					case <-ctx.Done():
						return
					}
				case agent.EventError:
					if ev.Error != nil && !errors.Is(ev.Error, context.Canceled) {
						outcome = turnOutcomeError
						outcomeReason = ev.Error.Error()
						errMsg := fmt.Sprintf("agent: %v", ev.Error)
						select {
						case tx <- dialoguetui.MessageEvent{Type: dialoguetui.MessageEventError, Text: errMsg}:
						case <-ctx.Done():
							return
						}
					}
				}
			}
			if err := it.Err(); err != nil {
				if !errors.Is(err, context.Canceled) {
					if outcome == turnOutcomeNone || outcome == turnOutcomeCompleted {
						outcome = turnOutcomeError
						outcomeReason = err.Error()
					}
					errMsg := fmt.Sprintf("agent stream: %v", err)
					select {
					case tx <- dialoguetui.MessageEvent{Type: dialoguetui.MessageEventError, Text: errMsg}:
					case <-ctx.Done():
						return
					}
				} else if outcome == turnOutcomeNone {
					outcome = turnOutcomeCanceled
					outcomeReason = string(llm.FinishReasonNull)
				}
			}
			if outcome == turnOutcomeNone {
				switch {
				case errors.Is(ctx.Err(), context.Canceled), errors.Is(req.ctx.Err(), context.Canceled):
					outcome = turnOutcomeCanceled
					outcomeReason = ""
				default:
					outcome = turnOutcomeError
					outcomeReason = "the response ended unexpectedly"
				}
			}
		}()
		syncComp.removeStatusHint(hint)
		if lastUsage.Context.TokensSent > 0 {
			syncComp.setContextHint(lastUsage)
		}
		// Signal that the agent is idle; the handler will drain
		// any queued follow-up messages at this point.
		select {
		case tx <- dialoguetui.MessageEvent{Type: dialoguetui.MessageEventBusy, Busy: false}:
		case <-ctx.Done():
			return
		}
	}
}

var _ agent.Prompter = (*tuiPrompter)(nil)

// tuiPrompter bridges agent.Prompter to the TUI by sending
// MessageEventPrompt events and blocking for user input.
type tuiPrompter struct {
	tx   chan<- dialoguetui.MessageEvent
	noti browserapi.Notifications
}

func (tp *tuiPrompter) Prompt(ctx context.Context, req agent.PromptRequest) (agent.PromptResponse, error) {
	resultCh := make(chan []string, 1)

	// Build label→value mapping so we can translate the TUI's
	// label-based selections back into the option Values that
	// callers compare against.
	labelToValue := make(map[string]string, len(req.Options))
	options := make([]dialoguetui.PromptEventOption, len(req.Options))
	for i, o := range req.Options {
		options[i] = dialoguetui.PromptEventOption{
			Label:         o.Label,
			Description:   o.Description,
			RequiresInput: o.RequiresInput,
		}
		if o.Value != "" {
			labelToValue[o.Label] = o.Value
		}
	}

	ev := dialoguetui.MessageEvent{
		Type:              dialoguetui.MessageEventPrompt,
		PromptTitle:       req.Title,
		PromptHeader:      req.Header,
		PromptBody:        req.Body,
		PromptOptions:     options,
		PromptMultiSelect: req.MultiSelect,
		PromptResult:      resultCh,
	}
	select {
	case tp.tx <- ev:
	case <-ctx.Done():
		return agent.PromptResponse{}, ctx.Err()
	}
	_, _ = tp.noti.Notify(browserapi.LevelWarn, "Input required: %s", req.Title)
	select {
	case vals := <-resultCh:
		if vals == nil {
			return agent.PromptResponse{}, errors.New("prompt dismissed")
		}

		// Free-form prompt (zero options): the TUI sends [text].
		// Return as TextInput only; Values stays nil.
		if len(req.Options) == 0 {
			return agent.PromptResponse{TextInput: vals[0]}, nil
		}

		// Map labels back to values where a mapping exists.
		var textInput string
		// When the TUI sends [label, text], the second element
		// is free-form text from a RequiresInput option.
		if len(vals) == 2 {
			textInput = vals[1]
			vals = vals[:1] // keep only the label for value mapping
		}
		for i, v := range vals {
			if mapped, ok := labelToValue[v]; ok {
				vals[i] = mapped
			}
		}
		return agent.PromptResponse{Values: vals, TextInput: textInput}, nil
	case <-ctx.Done():
		select {
		case tp.tx <- dialoguetui.MessageEvent{Type: dialoguetui.MessageEventPromptDismiss}:
		default:
		}
		return agent.PromptResponse{}, ctx.Err()
	}
}

// tuiProgressUpdater implements agent.ProgressUpdater by sending task
// progress events to the TUI channel.
type tuiProgressUpdater struct {
	tx chan<- dialoguetui.MessageEvent
}

func (u *tuiProgressUpdater) UpdateTaskProgress(ctx context.Context, task taskstore.Task) {
	select {
	case u.tx <- dialoguetui.MessageEvent{
		Type: dialoguetui.MessageEventTaskProgress,
		TaskProgress: dialoguetui.ProgressTaskEntry{
			ID:          task.ID,
			Subject:     task.Subject,
			Description: task.Description,
			ActiveForm:  task.ActiveForm,
			Status:      task.Status,
		},
	}:
	case <-ctx.Done():
	}
}

// commandAdapter wraps a repl.CommandHandler into a dialoguetui.CommandHandler
// with session-aware overrides for commands like /clear, /history, and /model.
type commandAdapter struct {
	handler    repl.CommandHandler
	dialogueID string
	resetFn    func() // visual reset (Component.Reset under lock)
	wm         browserapi.WindowManager
	store      dialoguemanager.Store
	// compactFn replaces messages in the component after a successful compact.
	compactFn func(msgs []llm.Message)

	// Model switching support. When agent is non-nil, the /model command
	// can switch the backing LLM service mid-conversation.
	agent              *agent.Agent
	modelRegistry      llmregistry.Registry
	config             config.Config
	newClient          clientConstructor
	newAnthropicClient anthropic.ClientConstructor
	currentModel       string
	auditStore         *llm.AuditStore

	// Skill resolution. When a /name command matches a skill, the
	// formatted skill content is returned as UserMessage.
	skillRegistry *skills.SkillRegistry
}

func (a *commandAdapter) newService(model string) (llm.Service, error) {
	svc, err := newLLMService(a.config, a.modelRegistry, model, a.newClient, a.newAnthropicClient)
	if err != nil {
		return nil, err
	}
	if a.auditStore != nil {
		var provider string
		if entry, ok := a.modelRegistry.Get(context.Background(), model); ok {
			provider = entry.Provider
		}
		svc = llm.NewAuditService(svc, a.auditStore, model, provider)
	}
	return svc, nil
}

func (a *commandAdapter) HandleCommand(
	ctx context.Context, name string, args []string,
) (dialoguetui.CommandResult, error) {
	// Check if the command matches a skill before dispatching as a shell command.
	if skill, ok := a.skillRegistry.Get(name); ok {
		return dialoguetui.CommandResult{
			UserMessage: formatSkillMessage(skill, strings.Join(args, " ")),
			SkillName:   skill.Name,
		}, nil
	}

	switch name {
	case "clear":
		if len(args) == 0 {
			return a.handleClear(ctx)
		}
		// /clear [id] → chats clear <id>
		name = "chats"
		args = append([]string{"clear"}, args...)
	case "chats":
		// /chats <subcmd> → inject current dialogue ID when missing
		if len(args) >= 1 && a.dialogueID != "" {
			switch args[0] {
			case "show", "log", "clear", "compact":
				if len(args) < 2 {
					args = append(args, a.dialogueID)
				}
			case "export":
				// export accepts flags like --audit, so check
				// for a non-flag positional argument.
				hasID := false
				for _, arg := range args[1:] {
					if !strings.HasPrefix(arg, "--") {
						hasID = true
						break
					}
				}
				if !hasID {
					args = append(args, a.dialogueID)
				}
			}
		}
		if len(args) >= 1 && args[0] == "compact" {
			return a.handleCompact(ctx, args[1:])
		}
	case "history":
		// /history [id] → chats show <id>
		if len(args) == 0 && a.dialogueID != "" {
			args = []string{a.dialogueID}
		}
		name = "chats"
		args = append([]string{"show"}, args...)
	case "compact":
		// /compact [id] → chats compact <id>
		if len(args) == 0 && a.dialogueID != "" {
			args = []string{a.dialogueID}
		}
		return a.handleCompact(ctx, args)
	case "export":
		// /export [--audit] [id] → chats export [--audit] <id>
		var flags []string
		var positional []string
		for _, arg := range args {
			if strings.HasPrefix(arg, "--") {
				flags = append(flags, arg)
			} else {
				positional = append(positional, arg)
			}
		}
		if len(positional) == 0 && a.dialogueID != "" {
			positional = []string{a.dialogueID}
		}
		name = "chats"
		args = append([]string{"export"}, flags...)
		args = append(args, positional...)
	case "log":
		// /log [id] → chats log <id>
		if len(args) == 0 && a.dialogueID != "" {
			args = []string{a.dialogueID}
		}
		name = "chats"
		args = append([]string{"log"}, args...)
	case "model":
		return a.handleModel(ctx, args)
	case "effort":
		return a.handleEffort(args)
	case "max_tokens":
		return a.handleMaxTokens(args)
	case "fork":
		// /fork [id] → chats fork <dialogueID>
		if len(args) == 0 && a.dialogueID != "" {
			args = []string{a.dialogueID}
		}
		name = "chats"
		args = append([]string{"fork"}, args...)
	}
	it, err := a.handler.HandleCommand(
		ctx, repl.Command{Name: name, Args: args}, repl.NopProgressWriter(),
	)
	if errors.Is(err, agentshell.ErrExit) {
		return dialoguetui.CommandResult{Exit: true}, nil
	}
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}

	items, drainErr := iterator.ToSlice(ctx, it)
	_ = it.Close()
	if drainErr != nil {
		return dialoguetui.CommandResult{}, drainErr
	}
	if len(items) == 0 {
		return dialoguetui.CommandResult{}, nil
	}

	a.openCommandFloating(items)
	return dialoguetui.CommandResult{}, nil
}

// formatSkillMessage formats the user-visible portion of a slash-command
// invocation. The full skill body is injected as a system message by
// Agent.run via WithSkillName; this function only emits the
// command envelope and any user-supplied arguments.
func formatSkillMessage(skill skills.Skill, args string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<command-message>%s</command-message>\n", skill.Name)
	fmt.Fprintf(&b, "<command-name>/%s</command-name>", skill.Name)
	if args != "" {
		b.WriteString("\n")
		b.WriteString(args)
	}
	return b.String()
}

func parseStoredCommandMessage(content string) (string, bool) {
	const (
		messageOpen  = "<command-message>"
		messageClose = "</command-message>"
		nameOpen     = "<command-name>"
		nameClose    = "</command-name>"
	)

	if !strings.HasPrefix(content, messageOpen) {
		return "", false
	}
	rest := strings.TrimPrefix(content, messageOpen)
	messageEnd := strings.Index(rest, messageClose)
	if messageEnd < 0 {
		return "", false
	}
	messageName := rest[:messageEnd]
	rest = rest[messageEnd+len(messageClose):]
	if !strings.HasPrefix(rest, "\n"+nameOpen) {
		return "", false
	}
	rest = strings.TrimPrefix(rest, "\n"+nameOpen)
	nameEnd := strings.Index(rest, nameClose)
	if nameEnd < 0 {
		return "", false
	}
	commandName := rest[:nameEnd]
	rest = rest[nameEnd+len(nameClose):]

	if messageName == "" || commandName == "" || !strings.HasPrefix(commandName, "/") {
		return "", false
	}
	if strings.TrimPrefix(commandName, "/") != messageName {
		return "", false
	}
	if rest == "" {
		return commandName, true
	}
	if !strings.HasPrefix(rest, "\n") {
		return "", false
	}
	args := strings.TrimPrefix(rest, "\n")
	if args == "" {
		return commandName, true
	}
	if strings.Contains(args, "\n") {
		return commandName + "\n" + args, true
	}
	return commandName + " " + args, true
}

// handleModel shows the current model or switches to a new one.
func (a *commandAdapter) handleModel(
	ctx context.Context, args []string,
) (dialoguetui.CommandResult, error) {
	if len(args) == 0 {
		md, err := markdown.New(a.currentModel)
		if err != nil {
			return dialoguetui.CommandResult{}, err
		}
		return dialoguetui.CommandResult{
			Display: iterator.FromSlice([]component.Responsive{md}),
		}, nil
	}
	model := args[0]
	entry, ok := a.modelRegistry.Get(ctx, model)
	if !ok {
		return dialoguetui.CommandResult{}, fmt.Errorf("model %q is not available. Available models: %s",
			model, availableModelsString(a.modelRegistry))
	}
	svc, err := a.newService(model)
	if err != nil {
		return dialoguetui.CommandResult{}, fmt.Errorf("create service for model %q: %w", model, err)
	}
	a.agent.SwapService(svc, model, entry.Provider)
	a.currentModel = model
	md, err := markdown.New(fmt.Sprintf("Switched to model **%s**", model))
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}
	return dialoguetui.CommandResult{
		Display: iterator.FromSlice([]component.Responsive{md}),
	}, nil
}

// validEffortLevels lists the allowed reasoning effort values.
var validEffortLevels = []llm.ReasoningEffort{
	llm.ReasoningEffortNone,
	llm.ReasoningEffortMinimal,
	llm.ReasoningEffortLow,
	llm.ReasoningEffortMedium,
	llm.ReasoningEffortHigh,
	llm.ReasoningEffortXHigh,
	llm.ReasoningEffortMax,
}

// handleEffort shows the current effort level or sets a new one.
func (a *commandAdapter) handleEffort(args []string) (dialoguetui.CommandResult, error) {
	if len(args) == 0 {
		current := a.agent.Effort()
		md, err := markdown.New(fmt.Sprintf("Current effort level: **%s**", current))
		if err != nil {
			return dialoguetui.CommandResult{}, err
		}
		return dialoguetui.CommandResult{
			Display: iterator.FromSlice([]component.Responsive{md}),
		}, nil
	}

	level := llm.ReasoningEffort(args[0])
	valid := false
	for _, v := range validEffortLevels {
		if level == v {
			valid = true
			break
		}
	}
	if !valid {
		return dialoguetui.CommandResult{}, fmt.Errorf(
			"invalid effort level %q: must be none, minimal, low, medium, high, xhigh, or max", args[0])
	}

	a.agent.SetEffort(level)
	md, err := markdown.New(fmt.Sprintf("Set effort level to **%s**", level))
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}
	return dialoguetui.CommandResult{
		Display: iterator.FromSlice([]component.Responsive{md}),
	}, nil
}

// handleMaxTokens shows or sets the per-session max-output-token override.
func (a *commandAdapter) handleMaxTokens(args []string) (dialoguetui.CommandResult, error) {
	if len(args) == 0 {
		current := a.agent.MaxOutputTokens()
		text := "Current max output tokens: provider/config default"
		if current > 0 {
			text = fmt.Sprintf("Current max output tokens: **%d**", current)
		}
		md, err := markdown.New(text)
		if err != nil {
			return dialoguetui.CommandResult{}, err
		}
		return dialoguetui.CommandResult{
			Display: iterator.FromSlice([]component.Responsive{md}),
		}, nil
	}

	n, err := strconv.Atoi(args[0])
	if err != nil || n <= 0 {
		return dialoguetui.CommandResult{}, fmt.Errorf(
			"invalid max_tokens value %q: must be a positive integer", args[0])
	}

	a.agent.SetMaxOutputTokens(n)
	md, err := markdown.New(fmt.Sprintf("Set max output tokens to **%d**", n))
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}
	return dialoguetui.CommandResult{
		Display: iterator.FromSlice([]component.Responsive{md}),
	}, nil
}

// handleClear resets the UI, runs the archive+clear via the shell, and
// returns the confirmation message inline instead of in a floating window.
func (a *commandAdapter) handleClear(ctx context.Context) (dialoguetui.CommandResult, error) {
	if a.resetFn != nil {
		a.resetFn()
	}
	it, err := a.handler.HandleCommand(ctx, repl.Command{
		Name: "chats", Args: []string{"clear", a.dialogueID},
	}, repl.NopProgressWriter())
	if err != nil {
		return dialoguetui.CommandResult{}, err
	}
	items, drainErr := iterator.ToSlice(ctx, it)
	_ = it.Close()
	if drainErr != nil {
		return dialoguetui.CommandResult{}, drainErr
	}
	if len(items) == 0 {
		return dialoguetui.CommandResult{}, nil
	}
	return dialoguetui.CommandResult{
		Display: iterator.FromSlice(items),
	}, nil
}

// handleCompact returns a lazy iterator that blocks during the LLM
// summarisation call so AddCommand's animation plays. After the iterator
// is fully drained, its Close method (which runs AFTER AddCommand removes
// the animation node) resets the component and replays the compacted
// messages. This ordering avoids the panic from Remove-after-Reset.
func (a *commandAdapter) handleCompact(
	_ context.Context, args []string,
) (dialoguetui.CommandResult, error) {
	return dialoguetui.CommandResult{
		Display: &compactIterator{
			handler:    a.handler,
			args:       args,
			store:      a.store,
			dialogueID: a.dialogueID,
			compactFn:  a.compactFn,
		},
	}, nil
}

// compactIterator is a lazy iterator for the /compact command.
// The first call to Next blocks while the agentshell runs the LLM compact
// call, then returns false (no items are yielded to AddCommand). Close
// triggers the visual reset and replay of compacted messages.
//
// AddCommand's deferred cleanup order is:
//  1. Remove(animNode) — safe because Reset has not been called yet
//  2. it.Close()       — calls compactFn which does Reset + replay
type compactIterator struct {
	handler    repl.CommandHandler
	args       []string
	store      dialoguemanager.Store
	dialogueID string
	compactFn  func(msgs []llm.Message)

	called bool
	closed bool
	err    error
}

func (c *compactIterator) Next(ctx context.Context) (component.Responsive, bool) {
	if c.called {
		return nil, false
	}
	c.called = true

	// This call blocks while the LLM summarises the conversation.
	it, err := c.handler.HandleCommand(ctx, repl.Command{
		Name: "chats", Args: append([]string{"compact"}, c.args...),
	}, repl.NopProgressWriter())
	if err != nil {
		c.err = err
		return nil, false
	}
	drainErr := it.Close() // discard UI elements
	if drainErr != nil {
		c.err = drainErr
		return nil, false
	}
	return nil, false
}

func (c *compactIterator) Err() error { return c.err }

func (c *compactIterator) Close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	if c.err != nil || c.compactFn == nil || c.store == nil {
		return nil
	}
	d, err := c.store.Get(context.Background(), c.dialogueID)
	if err != nil {
		slog.Error("compact: fetch compacted dialogue", "id", c.dialogueID, "error", err)
		return nil
	}
	c.compactFn(d.Messages)
	return nil
}

func (a *commandAdapter) openCommandFloating(items []component.Responsive) {
	// When the single item is a markdown component, use the markdown handler
	// which provides rich navigation (vi keys, search, mouse selection).
	if len(items) == 1 {
		if md, ok := items[0].(*markdown.Component); ok {
			a.openMarkdownFloating(md)
			return
		}
	}

	list := component.NewResponsiveList()
	for _, item := range items {
		list.PushBack(item)
	}

	listHandler := handler.NopFromComponent(list)
	scrollHandler := handler.Wrap(listHandler, func(ev term.Event) (exit, handled bool) {
		if ev.Type != term.EventKey {
			return
		}
		switch ev.Key {
		case term.KeyEsc:
			return true, true
		case term.KeyArrowUp:
			list.SeekUp()
			return false, true
		case term.KeyArrowDown:
			list.SeekDown()
			return false, true
		}
		return
	})

	const floatingWidth = 90
	var win browserapi.Window
	bhandler := browserapi.FuncHandler(scrollHandler, func() error {
		return a.wm.CloseWindow(win)
	})
	floating := browserapi.FuncFloating(bhandler, func() (int, int) {
		h := list.Height(floatingWidth)
		return floatingWidth, min(h, 45)
	})
	var err error
	win, err = a.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		slog.Error("command floating window", "error", err)
	}
}

func (a *commandAdapter) openMarkdownFloating(md *markdown.Component) {
	mdh := mdhandler.New(md)

	var win browserapi.Window
	bhandler := browserapi.FuncHandler(mdh, func() error {
		return a.wm.CloseWindow(win)
	})
	floating := browserapi.FuncFloating(bhandler, mdh.Dimensions)
	var err error
	win, err = a.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		slog.Error("command floating window", "error", err)
	}
}

func (a *commandAdapter) Complete(
	ctx context.Context, name string, args []string,
) (iterator.Iterator[string], error) {
	if name == "model" && a.modelRegistry != nil {
		return iterator.Map(a.modelRegistry.Models(), func(e llmregistry.ModelEntry) string {
			return e.Name
		}), nil
	}
	if name == "effort" {
		levels := make([]string, len(validEffortLevels))
		for i, l := range validEffortLevels {
			levels[i] = string(l)
		}
		return iterator.FromSlice(levels), nil
	}
	return a.handler.Complete(ctx, name, args)
}

// makeCommandCompleter builds a WordCompleter that translates
// /command-style input into calls to the CommandHandler's Complete method.
func makeCommandCompleter(ctx context.Context, ch dialoguetui.CommandHandler) inputbox.WordCompleter {
	return func(line string, pos int) (string, []string, string) {
		head := line[:pos]
		tail := line[pos:]
		if len(head) < 1 || head[0] != '/' {
			return head, nil, tail
		}
		cmdLine := head[1:]
		parts := strings.Fields(cmdLine)
		var cmd string
		var args []string
		if len(parts) > 0 {
			cmd = parts[0]
			if len(parts) > 1 || strings.HasSuffix(cmdLine, " ") {
				if len(parts) > 1 {
					args = parts[1:]
				}
				if strings.HasSuffix(cmdLine, " ") {
					args = append(args, "")
				}
			}
		}
		lastSpace := strings.LastIndex(head, " ")
		if lastSpace >= 0 {
			head = head[:lastSpace+1]
		} else {
			head = "/"
		}
		it, err := ch.Complete(ctx, cmd, args)
		if err != nil {
			return line[:pos], nil, tail
		}
		all, err := iterator.ToSlice(ctx, it)
		_ = it.Close()
		if err != nil {
			return line[:pos], nil, tail
		}

		// Filter candidates by the prefix the user has already
		// typed (the text after head, before cursor).
		prefix := line[len([]rune(head)):pos]
		var candidates []string
		if prefix == "" {
			candidates = all
		} else {
			lower := strings.ToLower(prefix)
			for _, c := range all {
				if strings.HasPrefix(strings.ToLower(c), lower) {
					candidates = append(candidates, c)
				}
			}
		}
		return head, candidates, tail
	}
}

type syncComponent struct {
	mu       *sync.Mutex
	comp     *dialoguetui.Component
	h        *aiEditorHandler
	hintSlot *hintSlot // shared across copies, protected by mu
}

// hintSlot stores the active hint component so that compactFn can
// re-add it after Reset + message replay during compaction.
type hintSlot struct {
	comp tui.Component
	conf component.SpanConfig
}

func (s syncComponent) addStatusHint() *statusHint {
	hint := newStatusHint(s.h.p, s.h.backgroundAttr, s.h.cfg.DurationPrecision, s.comp.TaskActiveForm)
	bg := component.WithBackground(hint, term.Cell{
		Attributes: s.h.backgroundAttr,
		Ch:         ' ',
		Width:      1,
	})
	conf := component.SpanConfig{
		ContentAlignment: component.AlignmentLeft,
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.comp.AddReceiveMessageHint(bg, conf)
	s.hintSlot.comp = bg
	s.hintSlot.conf = conf

	return hint
}

func (s syncComponent) removeStatusHint(hint *statusHint) {
	_ = hint.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.comp.RemoveReceiveMessageHint()
	s.hintSlot.comp = nil
}

// statusPhase describes the current phase of the agent turn.
type statusPhase int

const (
	phaseSending     statusPhase = iota // sending request to LLM
	phaseThinking                       // waiting for first content token
	phaseReceiving                      // receiving streamed content
	phaseToolCalling                    // tools are executing
	phaseCompacting                     // conversation is being compacted
	phaseRateLimited                    // waiting due to rate limit / retry
)

func (p statusPhase) String() string {
	switch p {
	case phaseSending:
		return "sending"
	case phaseThinking:
		return "thinking"
	case phaseReceiving:
		return "receiving"
	case phaseToolCalling:
		return "tool calling"
	case phaseCompacting:
		return "compacting"
	case phaseRateLimited:
		return "rate limited"
	default:
		return ""
	}
}

func (p statusPhase) arrow() rune {
	switch p {
	case phaseSending, phaseToolCalling:
		return '↑'
	default:
		return '↓'
	}
}

var statusSpinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

// statusHint is a tui.Component that displays a dynamic status line
// with a spinner, phase label, elapsed time, and token count.
type statusHint struct {
	mu                sync.Mutex
	interrupter       term.Interrupter
	phase             statusPhase
	startTime         time.Time
	durationPrecision time.Duration // when positive, truncates elapsed to this granularity
	tokensSent        int           // cumulative input tokens
	tokensReceived    int           // cumulative output tokens
	width             int
	drawCount         int
	attr              term.Attributes // background attr; text uses Fg from it
	cancel            context.CancelFunc
	activeFormFn      func() string // optional; returns activeForm from progress widget
}

func newStatusHint(interrupter term.Interrupter, attr term.Attributes, durationPrecision time.Duration, activeFormFn func() string) *statusHint {
	ctx, cancel := context.WithCancel(context.Background())
	h := &statusHint{
		interrupter:       interrupter,
		startTime:         time.Now(),
		durationPrecision: durationPrecision,
		attr:              attr,
		cancel:            cancel,
		activeFormFn:      activeFormFn,
	}
	go h.tick(ctx)
	return h
}

func (h *statusHint) tick(ctx context.Context) {
	ticker := time.NewTicker(125 * time.Millisecond) // 8 Hz
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = h.interrupter.Interrupt(ctx)
		}
	}
}

func (h *statusHint) Close() error {
	h.cancel()
	return nil
}

func (h *statusHint) setPhase(p statusPhase) {
	h.mu.Lock()
	h.phase = p
	h.mu.Unlock()
}

func (h *statusHint) setTokens(sent, received int) {
	h.mu.Lock()
	h.tokensSent = sent
	h.tokensReceived = received
	h.mu.Unlock()
}

func (h *statusHint) Resize(width, height int) {
	h.mu.Lock()
	h.width = width
	h.mu.Unlock()
}

func (h *statusHint) Draw(w term.Writer) {
	h.mu.Lock()
	phase := h.phase
	tokensSent := h.tokensSent
	tokensReceived := h.tokensReceived
	width := h.width
	h.drawCount++
	dc := h.drawCount
	h.mu.Unlock()

	spinner := statusSpinnerFrames[dc%len(statusSpinnerFrames)]
	elapsed := time.Since(h.startTime)
	if h.durationPrecision > 0 {
		elapsed = elapsed.Truncate(h.durationPrecision)
	}

	phaseLabel := phase.String()
	if h.activeFormFn != nil {
		if af := h.activeFormFn(); af != "" {
			phaseLabel = af
		}
	}

	var sb strings.Builder
	sb.WriteRune(spinner)
	sb.WriteByte(' ')
	sb.WriteString(phaseLabel)
	sb.WriteString(" (")
	sb.WriteString(formatStatusDuration(elapsed))
	if tokensSent > 0 || tokensReceived > 0 {
		sb.WriteString(" · ↑ ")
		sb.WriteString(formatTokenCount(tokensSent))
		sb.WriteString(" · ↓ ")
		sb.WriteString(formatTokenCount(tokensReceived))
	}
	sb.WriteByte(')')

	cellAttr := term.Attributes{Bg: h.attr.Bg}
	text := sb.String()
	x := 0
	for _, r := range text {
		if x >= width {
			break
		}
		w.SetCell(term.Coordinates{X: x, Y: 0}, term.Cell{
			Ch:         r,
			Width:      1,
			Attributes: cellAttr,
		})
		x++
	}
}

// formatStatusDuration formats a duration for the status line.
func formatStatusDuration(d time.Duration) string {
	d = d.Truncate(time.Second)
	s := int(d.Seconds())
	if s < 60 {
		return fmt.Sprintf("%ds", s)
	}
	m := s / 60
	s %= 60
	if m < 60 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := m / 60
	m %= 60
	return fmt.Sprintf("%dh %dm", h, m)
}

// formatTokenCount formats a token count for the status line.
func formatTokenCount(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d tokens", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.1fk tokens", float64(n)/1000)
	default:
		return fmt.Sprintf("%.1fm tokens", float64(n)/1_000_000)
	}
}

// formatTokenNumber formats a token count without the "tokens" suffix and
// without decimal places.
func formatTokenNumber(n int) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d", n)
	case n < 1_000_000:
		return fmt.Sprintf("%.0fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%.0fm", float64(n)/1_000_000)
	}
}

// contextHintSegment is a styled span of text within the context hint.
type contextHintSegment struct {
	text string
	attr term.Attributes
}

// contextHint is a static tui.Component that renders a one-line
// summary of context usage after a turn completes.
type contextHint struct {
	segments []contextHintSegment
	width    int
}

func (c *contextHint) Resize(width, height int) { c.width = width }

func (c *contextHint) Draw(w term.Writer) {
	x := 0
	for _, seg := range c.segments {
		for _, r := range seg.text {
			if x >= c.width {
				return
			}
			w.SetCell(term.Coordinates{X: x, Y: 0}, term.Cell{
				Ch:         r,
				Width:      1,
				Attributes: seg.attr,
			})
			x++
		}
	}
}

// buildContextHintSegments produces styled segments for post-turn display.
func buildContextHintSegments(ev agent.Event, cfg contextHintConfig, durationPrecision time.Duration) []contextHintSegment {
	u := ev.Usage
	dur := u.TotalDuration
	if durationPrecision > 0 {
		dur = dur.Truncate(durationPrecision)
	} else {
		dur = dur.Truncate(time.Second)
	}
	label := cfg.labelAttr
	value := cfg.valueAttr

	segs := []contextHintSegment{
		{text: formatStatusDuration(dur), attr: value},
		{text: " · ", attr: label},
		{text: formatTokenCount(u.TokensSent), attr: value},
		{text: " sent", attr: label},
	}

	if u.TokensCached > 0 {
		hitPct := float64(u.TokensCached) / float64(u.TokensSent) * 100
		segs = append(segs,
			contextHintSegment{text: " · cache: ", attr: label},
			contextHintSegment{text: fmt.Sprintf("%.0f%%", hitPct), attr: value},
			contextHintSegment{text: fmt.Sprintf(" (%s/%s)", formatTokenNumber(u.TokensCached), formatTokenNumber(u.TokensSent)), attr: label},
		)
	}

	if ev.Context.Window > 0 {
		contextTokens := ev.Context.TokensSent + ev.Context.TokensReceived
		pct := float64(contextTokens) / float64(ev.Context.Window) * 100
		segs = append(segs,
			contextHintSegment{text: " · context: ", attr: label},
			contextHintSegment{text: formatTokenNumber(contextTokens), attr: value},
			contextHintSegment{text: fmt.Sprintf(" (%.0f%%)", pct), attr: value},
		)
		if ev.Context.AutoCompactAt > 0 {
			segs = append(segs,
				contextHintSegment{text: " · compacts at ", attr: label},
				contextHintSegment{text: formatTokenNumber(ev.Context.AutoCompactAt), attr: value},
			)
		}
	}

	return segs
}

func (s syncComponent) setContextHint(ev agent.Event) {
	hint := &contextHint{
		segments: buildContextHintSegments(ev, s.h.contextHintCfg, s.h.cfg.DurationPrecision),
	}
	bg := component.WithBackground(hint, term.Cell{
		Attributes: s.h.backgroundAttr,
		Ch:         ' ',
		Width:      1,
	})
	s.mu.Lock()
	defer s.mu.Unlock()

	s.comp.AddReceiveMessageHint(bg, component.SpanConfig{
		ContentAlignment: component.AlignmentLeft,
	})
}

func readWorkspaceFile(wfs workspaceapi.FileSystem, path string) ([]byte, error) {
	f, err := wfs.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// agentMemoriesToTUI converts an agent EventMemoryRecall into a TUI
// MessageEvent, mapping []agent.Memory to []dialoguetui.MemoryRecallEntry.
func agentMemoriesToTUI(ev agent.Event) dialoguetui.MessageEvent {
	entries := make([]dialoguetui.MemoryRecallEntry, len(ev.Memories))
	for i, m := range ev.Memories {
		entries[i] = dialoguetui.MemoryRecallEntry{ID: m.ID, Content: m.Content}
	}
	return dialoguetui.MessageEvent{
		Type:           dialoguetui.MessageEventMemoryRecall,
		Memories:       entries,
		MemoryDuration: ev.MemoryDuration,
	}
}
