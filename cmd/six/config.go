package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/plugin"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
	yaml "gopkg.in/yaml.v3"
)

const (
	outputNormal     = "normal"
	output256        = "color_256"
	outputGrayscale  = "grayscale"
	inputEsc         = "esc"
	inputAlt         = "alt"
	inputMouse       = "mouse"
	inputCurrent     = "current"
	defaultStartText = `
         __       
        /\ \      
       /  \ \     
      / /\ \_\    
     / / /\/_/    
    / /_/_        
   / /___/\       
  / /\__ \ \      
 / / /__\ \ \     
/ / /____\ \ \    
\/__________\/    `
)

var (
	defaultWindowManagerConfig = handler.DefaultWindowManagerConfig()
)

type pluginConfig struct {
	id     string
	parent *ideConfig
	cfg    plugin.Config
}

type ideConfig struct {
	cfg    map[string]interface{}
	errors map[string]error
}

func initConfig(c *ideConfig, cfg map[string]interface{}) {
	c.cfg = cfg
	c.errors = make(map[string]error)
}

func initDefaultConfig(c *ideConfig) {
	cfg := make(map[string]interface{})
	initConfig(c, cfg)
}

func (c ideConfig) windowManager() (plugin.Config, bool) {
	b, ok := c.browser()
	if !ok {
		return nil, false
	}
	cfg, err := b.GetConfig("window_manager")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["window_manager"] = err
		}
		return nil, false
	}
	return cfg, true
}

func (c ideConfig) windowFrameAttr() (attr term.Attributes) {
	attr = defaultWindowManagerConfig.FrameAttr
	cfg, ok := c.windowManager()
	if !ok {
		return
	}
	cfgAttr, err := cfg.GetAttributes("frame_attr")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["window_manager.frame_attr"] = err
		}
		return
	}
	attr = cfgAttr
	return
}

func (c ideConfig) focusWindowFrameAttr() (attr term.Attributes) {
	attr = defaultWindowManagerConfig.FocusFrameAttr
	cfg, ok := c.windowManager()
	if !ok {
		return
	}
	cfgAttr, err := cfg.GetAttributes("focus_frame_attr")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["window_manager.focus_frame_attr"] = err
		}
		return
	}
	attr = cfgAttr
	return
}

func (c ideConfig) frame() (frame bool) {
	frame = defaultWindowManagerConfig.Frame
	cfg, ok := c.windowManager()
	if !ok {
		return
	}
	cfgFrame, err := cfg.GetBool("frame")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["window_manager.frame"] = err
		}
		return
	}
	frame = cfgFrame
	return
}

func (c ideConfig) windowManagerConfig() handler.WindowManagerConfig {
	defaults := handler.DefaultWindowManagerConfig()
	return handler.WindowManagerConfig{
		WindowManagerConfig: component.WindowManagerConfig{
			Frame:        c.frame(),
			FrameAttr:    c.windowFrameAttr(),
			FrameCharSet: defaults.FrameCharSet,
		},
		FocusFrameAttr:    c.focusWindowFrameAttr(),
		FocusFrameCharSet: defaults.FocusFrameCharSet,
	}
}

func (c ideConfig) browser() (plugin.Config, bool) {
	if c.cfg == nil {
		return nil, false
	}
	b, err := plugin.MapConfig(c.cfg).GetConfig("browser")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["browser"] = err
		}
		return nil, false
	}
	return b, true
}

func (c ideConfig) browserTabspaces() (tabs int) {
	tabs = browser.DefaultConfig().Tabspaces
	cfg, ok := c.browser()
	if !ok {
		return
	}
	cfgTabs, err := cfg.GetInt("tabspaces")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["browser.tabspaces"] = err
		}
		return
	}
	tabs = cfgTabs
	return
}

func (c ideConfig) browserStartText() (text string) {
	text = defaultStartText
	cfg, ok := c.browser()
	if !ok {
		return
	}
	cfgText, err := cfg.GetString("start_text")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["browser.start_text"] = err
		}
		return
	}
	text = cfgText
	return
}

func (c ideConfig) browserSwapDir() (dir string) {
	dir = browser.DefaultConfig().SwapDir
	cfg, ok := c.browser()
	if !ok {
		return
	}
	cfgDir, err := cfg.GetString("swap_dir")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["browser.swap_dir"] = err
		}
		return
	}
	dir = cfgDir
	return
}

func (c ideConfig) logOutputPath() string {
	if c.cfg == nil {
		return ""

	}
	path, err := plugin.MapConfig(c.cfg).GetString("log_path")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["log_path"] = err
		}
		return ""
	}
	return path
}

func (c ideConfig) logLevel() log.Level {
	if c.cfg == nil {
		return log.ErrorLevel

	}
	levelStr, err := plugin.MapConfig(c.cfg).GetString("log_level")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["log_level"] = err
		}
		return log.ErrorLevel
	}

	level, err := log.ParseLevel(levelStr)
	if err != nil {
		c.errors["log_level"] = err
		return log.ErrorLevel
	}
	return level
}

func (c ideConfig) outputMode() (out term.OutputMode) {
	out = term.Output256

	outputModeIfc, ok := c.cfg["output_mode"]
	if !ok {
		return
	}

	outputModeStr, ok := outputModeIfc.(string)
	if !ok {
		c.errors["output_mode"] = errors.New("invalid type")
		return
	}

	switch outputModeStr {
	case outputNormal:
		out = term.OutputNormal
	case output256:
		out = term.Output256
	case outputGrayscale:
		out = term.OutputGrayscale
	default:
		c.errors["output_mode"] = fmt.Errorf("unknown output mode: %s", outputModeStr)
	}

	return
}

func (c ideConfig) inputMode() term.InputMode {
	inputModeIfc, ok := c.cfg["input_mode"]
	if !ok {
		return term.InputCurrent
	}

	inputModeSlice, ok := inputModeIfc.([]interface{})
	if !ok {
		inputModeStr, ok := inputModeIfc.(string)
		if !ok {
			c.errors["input_mode"] = errors.New("invalid type")
			return term.InputCurrent
		}

		inputModeSlice = []interface{}{inputModeStr}
	}

	var ret term.InputMode
	for _, inputMode := range inputModeSlice {
		switch inputMode {
		case inputEsc:
			ret |= term.InputEsc
		case inputAlt:
			ret |= term.InputAlt
		case inputMouse:
			ret |= term.InputMouse
		case inputCurrent:
			return term.InputCurrent
		default:
			c.errors["input_mode"] = fmt.Errorf("unknown input mode: %s", inputMode)
		}
	}

	return ret
}

func (c ideConfig) plugins() map[string]pluginConfig {
	pConfigIfc, ok := c.cfg["plugins"]
	if !ok {
		return nil
	}
	pConfigMap, ok := pConfigIfc.(map[string]interface{})
	if !ok {
		c.errors["plugins"] = errors.New("invalid type")
		return nil
	}

	ret := make(map[string]pluginConfig)
	for id, pConfig := range pConfigMap {
		pcfg, ok := pConfig.(map[string]interface{})
		if !ok {
			c.errors["plugins."+id] = errors.New("invalid type")
			continue
		}
		ret[id] = pluginConfig{
			id:     id,
			parent: &c,
			cfg:    plugin.MapConfig(pcfg),
		}
	}

	return ret
}

func (c pluginConfig) path() (string, bool) {
	path, err := c.cfg.GetString("path")
	if err != nil {
		// path is non-optional
		errorID := fmt.Sprintf("plugin.%s.path", c.id)
		c.parent.errors[errorID] = err
		return "", false
	}
	return path, true
}

func (c pluginConfig) config() (plugin.Config, bool) {
	cfg, err := c.cfg.GetConfig("config")
	if err != nil {
		if err != plugin.ErrNotFound {
			errorID := fmt.Sprintf("plugin.%s.config", c.id)
			c.parent.errors[errorID] = err
		}
		return nil, false
	}
	return cfg, true
}

func decodeConfig(r io.Reader) (cfg map[string]interface{}, err error) {
	d := yaml.NewDecoder(r)

	cfg = make(map[string]interface{})
	err = d.Decode(&cfg)
	return
}

func loadConfig(c *ideConfig, configpath string) error {
	f, err := os.Open(configpath)
	if err != nil {
		initDefaultConfig(c)
		if err != os.ErrNotExist {
			return err
		}
		return nil
	}

	cfg, err := decodeConfig(f)
	if err != nil {
		initDefaultConfig(c)
		return err
	}

	initConfig(c, cfg)

	return nil
}
