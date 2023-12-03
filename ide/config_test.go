package ide

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/command"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/emulator"
)

var sampleConfig = `
extensions:
    fuzzy_file:
        path: "/path/extension_fuzzy_file"
        config:
            command: ag -g ""

log_path: "/tmp/debug.log"
log_level: "trace"
clipboard: memory

editor:
    mode: modal
    modal:
        search_attr:
            bg: red
            fg: 219
        debug: true
        wrap: true
    modeless:
        search_attr:
            bg: red
            fg: 229
        wrap: false
    virtual:
        shell: bash
        editor: vim
        attr:
            bg: red
            fg: 219
        selection_attr:
            bg: green
            fg: 218

input_mode:
  - mouse
  - esc

output_mode: color_256

command:
  show_manual_after: 2s
  aliases:
    cherry: bomb
    todo:
      - e file:///tmp/todo.md
      - jenesaisquoi
    error: 1
  key_bindings:
    f: searchFile
    l: searchText
    <c-x>: closeDoors
    <c-x><c-p>: openAllDoors
    f<c-p>: openDoors small
    <c-x>9:
      - openDoors
      - large
    <c-x>p:
      - invalid
      - smtg:
        - else
    <-x>f: invalidMapping

notifications:
    auto_close: 1s
    progress_bar: false
    attr:
        bg: red
        fg: 219
    background_attr:
        bg: red
        fg: 219
    frame_charset:
        horizontalbottom: '━'
        horizontaltop: '━'
        verticalleft: '┃'
        verticalright: '┃'
        topleft: '┏'
        topright: '┓'
        bottomleft: '┗'
        bottomright: '┛'
browser:
    tabspaces: 4
    prompt:
        width: 20
        height: 10
        text_attr:
            fg: cyan
        highlight_attr:
            bg: red
            fg: 219
    window_manager:
        frame: true
        dim: false
        frame_attr:
            fg: red
        frame_charset:
            horizontalbottom: '━'
            horizontaltop: '━'
            verticalleft: '┃'
            verticalright: '┃'
            topleft: '┏'
            topright: '┓'
            bottomleft: '┗'
            bottomright: '┛'
    frameunion_charset:
        left: '┣'
        right: '┫'
        top: '┫'
        bottom: '┫'
    message_bar_attr:
        fg: white
        bg: cyan
    focus_tab_attr:
        fg: 219
    non_focus_tab_attr:
        fg: white

workspace:
    wallpaper: abc
    wallpaper_attr:
        fg: yellow
        bg: white
    wallpaper_background_attr:
        bg: white

terminal:
    shell: sh
    attr:
        fg: white
        bg: yellow
    selection_attr:
        fg: green
        bg: cyan
`

func assertDefaultConfig(t *testing.T, cfg *ideConfig) {
	assert.Len(t, cfg.extensions(), 0)
	assert.Equal(t, 4, cfg.browserTabspaces())
	assert.NotZero(t, cfg.wallpaper())
	defWmConfig := handler.DefaultWindowManagerConfig()
	defWmConfig.FocusFrameAttr = defWmConfig.FrameAttr
	defWmConfig.FocusFrameCharSet = defWmConfig.FrameCharSet
	assert.Equal(t, defWmConfig, cfg.windowManagerConfig())
	assert.Equal(t, "", cfg.logOutputPath())
	assert.Equal(t, logrus.ErrorLevel, cfg.logLevel())
	assert.Equal(t, term.Output256, cfg.outputMode())
	assert.Equal(t, term.InputCurrent, cfg.inputMode())
	assert.Equal(t, component.DefaultFrameUnionCharSet(), cfg.frameUnionCharset())

	actualNotifications := cfg.notificationsConfig()
	expectedNotifications := browser.DefaultConfig().Notifications
	assert.NotNil(t, actualNotifications.Interrupter)
	actualNotifications.Interrupter = nil
	expectedNotifications.Interrupter = nil
	assert.Equal(t, expectedNotifications, actualNotifications)

	assert.Equal(t, browser.DefaultConfig().FocusTabAttr, cfg.focusTabAttr())
	assert.Equal(t, browser.DefaultConfig().NonFocusTabAttr, cfg.nonFocusTabAttr())
	assert.Equal(t, browser.DefaultConfig().WallpaperAttr, cfg.workspaceWallpaperAttr())
	assert.Equal(t, browser.DefaultConfig().WallpaperBackgroundAttr, cfg.workspaceWallpaperBackgroundAttr())
	selectAttr := term.Attributes{Bg: term.AttrReverse, Fg: term.AttrReverse}
	assert.Equal(t, emulator.Config{SelectionAttributes: selectAttr}, cfg.terminalConfig())
	assert.Equal(t, command.DefaultConfig().ShowManualAfter, cfg.commandOverlayShowManualAfter())

	assert.Equal(t, term.Attributes{Fg: term.ColorBlack, Bg: term.ColorYellow},
		cfg.modelessResultAttr())
	assert.False(t, cfg.modelessWrap())

	assert.Equal(t, "modal", cfg.editorMode())
	os.Setenv("SHELL", "fish")
	assert.Equal(t, "", cfg.virtualEditorEditor())
	assert.Equal(t, "fish", cfg.virtualEditorShell())
	assert.Equal(t, term.Attributes{}, cfg.virtualEditorAttr())
	assert.Equal(t, term.Attributes{Fg: term.AttrReverse, Bg: term.AttrReverse},
		cfg.virtualEditorSelectionAttr())
}

func TestConfigDefault(t *testing.T) {
	ret := new(ideConfig)
	initDefaultConfig(ret, "myWallpaper")
	assertDefaultConfig(t, ret)
}

func TestConfigDecodeError(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	_, err = f.WriteString("||\\n\x00{'BABY':'$$'}")
	require.NoError(t, err)

	var ret ideConfig
	_, err = loadConfig(&ret, f.Name(), "myWallpaper")
	assert.Error(t, err)
	assertDefaultConfig(t, &ret)
}

func TestConfigSetting(t *testing.T) {
	m, err := decodeConfig(strings.NewReader(sampleConfig))
	require.NoError(t, err)

	var cfg ideConfig
	initConfig(&cfg, m, "myWallpaper")

	assert.Equal(t, 4, cfg.browserTabspaces())
	assert.Equal(t, "abc", cfg.wallpaper())
	assert.Equal(t, "/tmp/debug.log", cfg.logOutputPath())
	assert.Equal(t, logrus.TraceLevel, cfg.logLevel())
	assert.Equal(t, term.Output256, cfg.outputMode())
	assert.True(t, term.InputMouse&cfg.inputMode() != 0)
	assert.True(t, term.InputEsc&cfg.inputMode() != 0)

	expectedConfig := handler.WindowManagerConfig{
		WindowManagerConfig: component.WindowManagerConfig{
			Frame:        true,
			FrameAttr:    term.Attributes{Fg: term.ColorRed},
			FrameCharSet: component.FrameCharSetHighlight(),
		},
		Dim:               false,
		FocusFrameAttr:    handler.DefaultWindowManagerConfig().FrameAttr,
		FocusFrameCharSet: handler.DefaultWindowManagerConfig().FrameCharSet,
	}
	assert.Equal(t, expectedConfig, cfg.windowManagerConfig())

	assert.Len(t, cfg.extensions(), 1)
	extensionCfgStruct := cfg.extensions()["fuzzy_file"]
	assert.Equal(t, "fuzzy_file", extensionCfgStruct.id)
	assert.Equal(t, &cfg, extensionCfgStruct.parent)

	extensionCfg, ok := extensionCfgStruct.config()
	require.True(t, ok)

	cmd, err := extensionCfg.GetString("command")
	require.NoError(t, err)
	assert.Equal(t, "ag -g \"\"", cmd)

	expectedCommandAliases := map[string][]string{
		"todo":   {"e file:///tmp/todo.md", "jenesaisquoi"},
		"cherry": {"bomb"},
	}
	assert.Equal(t, expectedCommandAliases, cfg.commandAliases())
	assert.Equal(t, 2*time.Second, cfg.commandOverlayShowManualAfter())

	expectedFUCs := component.FrameUnionCharSet{Left: '┣', Right: '┫', Top: '┫', Bottom: '┫'}
	assert.Equal(t, expectedFUCs, cfg.frameUnionCharset())

	notifications := cfg.notificationsConfig()
	assert.False(t, notifications.ProgressBar)
	assert.Equal(t, 1*time.Second, notifications.AutoClose)
	assert.Equal(t, component.FrameCharSetHighlight(), notifications.FrameCharSet)
	assert.Equal(t, term.Attributes{Fg: term.Attribute(219), Bg: term.ColorRed},
		notifications.Attributes)
	assert.Equal(t, term.Attributes{Fg: term.Attribute(219), Bg: term.ColorRed},
		notifications.BackgroundAttributes)

	assert.Equal(t, term.Attributes{Fg: term.Attribute(219)}, cfg.focusTabAttr())
	assert.Equal(t, term.Attributes{Fg: term.ColorWhite}, cfg.nonFocusTabAttr())
	assert.Equal(t, term.Attributes{Fg: term.ColorYellow, Bg: term.ColorWhite}, cfg.workspaceWallpaperAttr())
	assert.Equal(t, term.Attributes{Bg: term.ColorWhite}, cfg.workspaceWallpaperBackgroundAttr())
	expectedEmulatorConfig := emulator.Config{
		Shell:               "sh",
		Attributes:          term.Attributes{Fg: term.ColorWhite, Bg: term.ColorYellow},
		SelectionAttributes: term.Attributes{Fg: term.ColorGreen, Bg: term.ColorCyan},
	}
	assert.Equal(t, expectedEmulatorConfig, cfg.terminalConfig())

	expectedPrompt := browser.PromptConfig{
		Width:         20,
		Height:        10,
		TextAttr:      term.Attributes{Fg: term.ColorCyan},
		HighlightAttr: term.Attributes{Fg: 219, Bg: term.ColorRed},
	}
	assert.Equal(t, expectedPrompt, cfg.promptConfig())

	assert.Equal(t, term.Attributes{Bg: term.ColorRed,
		Fg: term.Attribute(219)}, cfg.modalResultAttr())
	assert.True(t, cfg.modalDebug())
	assert.True(t, cfg.modalWrap())

	assert.Equal(t, term.Attributes{Bg: term.ColorRed,
		Fg: term.Attribute(229)}, cfg.modelessResultAttr())
	assert.False(t, cfg.modelessWrap())

	assert.Equal(t, "modal", cfg.editorMode())
	os.Setenv("SHELL", "")
	assert.Equal(t, "vim", cfg.virtualEditorEditor())
	assert.Equal(t, "bash", cfg.virtualEditorShell())
	assert.Equal(t, term.Attributes{Fg: term.Attribute(219), Bg: term.ColorRed},
		cfg.virtualEditorAttr())
	assert.Equal(t, term.Attributes{Fg: term.Attribute(218), Bg: term.ColorGreen},
		cfg.virtualEditorSelectionAttr())

	wantMappings := map[handler.Sequence][]string{
		{First: term.KeyComb{Ch: 'f'}}:            {"searchFile"},
		{First: term.KeyComb{Ch: 'l'}}:            {"searchText"},
		{First: term.KeyComb{Key: term.KeyCtrlX}}: {"closeDoors"},
		{
			First: term.KeyComb{Key: term.KeyCtrlX},
			Last:  term.KeyComb{Key: term.KeyCtrlP},
		}: {"openAllDoors"},
		{
			First: term.KeyComb{Ch: 'f'},
			Last:  term.KeyComb{Key: term.KeyCtrlP},
		}: {"openDoors", "small"},
		{
			First: term.KeyComb{Key: term.KeyCtrlX},
			Last:  term.KeyComb{Ch: '9'},
		}: {"openDoors", "large"},
	}
	assert.Equal(t, wantMappings, cfg.commandKeyMappings())
}
