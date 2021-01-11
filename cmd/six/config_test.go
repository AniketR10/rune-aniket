package main

import (
	"io/ioutil"
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertDefaultConfig(t *testing.T, cfg *ideConfig) {
	assert.Len(t, cfg.plugins(), 0)
	assert.Equal(t, 4, cfg.browserTabspaces())
	assert.NotZero(t, cfg.browserStartText())
	assert.Equal(t, handler.DefaultWindowManagerConfig(), cfg.windowManagerConfig())
	assert.Equal(t, "", cfg.browserSwapDir())
	assert.Equal(t, "", cfg.logOutputPath())
	assert.Equal(t, logrus.ErrorLevel, cfg.logLevel())
	assert.Equal(t, term.OutputCurrent, cfg.outputMode())
	assert.Equal(t, term.InputCurrent, cfg.inputMode())
}

func TestDefaultConfig(t *testing.T) {
	ret := new(ideConfig)
	initDefaultConfig(ret)
	assertDefaultConfig(t, ret)
}

func TestDecodeConfigError(t *testing.T) {
	f, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	_, err = f.WriteString("||\\n\x00{'BABY':'$$'}")
	require.NoError(t, err)

	var ret ideConfig
	err = loadConfig(&ret, f.Name())
	assert.Error(t, err)
	assertDefaultConfig(t, &ret)
}

func TestConfigSetting(t *testing.T) {
	input := `
plugins:
    fuzzy_file:
        path: "/home/ernestrc/src/go-tui/bin/plugin_fuzzy_file"
        config:
            command: ag -g ""

log_path: "/tmp/debug.log"
log_level: "trace"

input_mode:
  - mouse
  - esc

output_mode: color_256

browser:
    tabspaces: 4
    swap_dir: /tmp/util
    window_manager:
        border: true
        border_attr:
            fg: red
        focus_border_attr:
            fg: magenta
            bg:
              - cyan
              - bold
    start_text: abc

`
	m, err := decodeConfig(strings.NewReader(input))
	require.NoError(t, err)

	var cfg ideConfig
	initConfig(&cfg, m)

	assert.Equal(t, 4, cfg.browserTabspaces())
	assert.Equal(t, "abc", cfg.browserStartText())
	assert.Equal(t, "/tmp/util", cfg.browserSwapDir())
	assert.Equal(t, "/tmp/debug.log", cfg.logOutputPath())
	assert.Equal(t, logrus.TraceLevel, cfg.logLevel())
	assert.Equal(t, term.Output256, cfg.outputMode())
	assert.True(t, term.InputMouse&cfg.inputMode() != 0)
	assert.True(t, term.InputEsc&cfg.inputMode() != 0)

	expectedConfig := handler.WindowManagerConfig{
		WindowManagerConfig: component.WindowManagerConfig{
			Border:       true,
			BorderAttr:   term.Attributes{Fg: term.ColorRed},
			FrameCharSet: handler.DefaultWindowManagerConfig().FrameCharSet,
		},
		FocusBorderAttr:   term.Attributes{Fg: term.ColorMagenta, Bg: term.AttrBold | term.ColorCyan},
		FocusFrameCharSet: handler.DefaultWindowManagerConfig().FocusFrameCharSet,
	}
	assert.Equal(t, expectedConfig, cfg.windowManagerConfig())

	assert.Len(t, cfg.plugins(), 1)
	pluginCfgStruct := cfg.plugins()["fuzzy_file"]
	assert.Equal(t, "fuzzy_file", pluginCfgStruct.id)
	assert.Equal(t, &cfg, pluginCfgStruct.parent)

	pluginCfg, ok := pluginCfgStruct.config()
	require.True(t, ok)

	cmd, err := pluginCfg.GetString("command")
	require.NoError(t, err)
	assert.Equal(t, "ag -g \"\"", cmd)
}
