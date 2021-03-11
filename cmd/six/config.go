package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/editor"
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
	legacyDefaultStartText = `
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
	defaultStartText = `
███████╗██╗██╗ ██╗
██╔════╝██║██████║
███████╗██║╚═██╔═╝
╚════██║██║██████╗
███████║██║██╔═██║
╚══════╝╚═╝╚═╝ ╚═╝`
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

func (c ideConfig) commandKeyMappings() map[term.Event]string {
	ret := make(map[term.Event]string)
	if c.cfg == nil {
		return ret
	}
	m, err := plugin.MapConfig(c.cfg).GetMap("command_key_bindings")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["command_key_bindings"] = err
		}
		return ret
	}

	for k, v := range m {
		ev, err := term.ParseKey(k)
		if err != nil {
			c.errors["command_key_bindings."+k] = err
			continue
		}
		strValue, ok := v.(string)
		if !ok {
			c.errors["command_key_bindings."+k] = errors.New("expected string found unknown type")
			continue
		}
		ret[ev] = strValue
	}

	return ret
}

func (c ideConfig) getConfig(cfg plugin.Config, key string) (plugin.Config, bool) {
	cfg, err := cfg.GetConfig(key)
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors[key] = err
		}
		return nil, false
	}
	return cfg, true
}

func (c ideConfig) windowManager() (plugin.Config, bool) {
	b, ok := c.browser()
	if !ok {
		return nil, false
	}
	return c.getConfig(b, "window_manager")
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

func (c ideConfig) getBrowserAttr(
	key string, def term.Attributes,
) (attr term.Attributes) {
	attr = def
	cfg, ok := c.browser()
	if !ok {
		return
	}
	cfgAttr, err := cfg.GetAttributes(key)
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["window_manager."+key] = err
		}
		return
	}
	attr = cfgAttr
	return
}

func (c ideConfig) messageBarAttr() term.Attributes {
	return c.getBrowserAttr("message_bar_attr",
		browser.DefaultConfig().MessageBarAttr)
}

func (c ideConfig) focusTabAttr() term.Attributes {
	return c.getBrowserAttr("focus_tab_attr",
		browser.DefaultConfig().FocusTabAttr)
}

func (c ideConfig) nonFocusTabAttr() term.Attributes {
	return c.getBrowserAttr("non_focus_tab_attr",
		browser.DefaultConfig().NonFocusTabAttr)
}

func (c ideConfig) startTextAttr() term.Attributes {
	return c.getBrowserAttr("start_text_attr",
		browser.DefaultConfig().StartTextAttr)
}

func (c ideConfig) startTextBackgroundAttr() term.Attributes {
	return c.getBrowserAttr("start_text_background_attr",
		browser.DefaultConfig().StartTextBackgroundAttr)
}

func (c ideConfig) dirtyTabAttr() term.Attributes {
	return c.getBrowserAttr("dirty_tab_attr",
		editor.DefaultConfig().DirtyTabAttr)
}

func (c ideConfig) windowFrameCharset() (cs component.FrameCharSet) {
	cs = defaultWindowManagerConfig.FrameCharSet
	cfg, ok := c.windowManager()
	if !ok {
		return
	}
	cfgCs, err := cfg.GetFrameCharset("frame_charset", cs)
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["window_manager.frame_charset"] = err
		}
		return
	}
	cs = cfgCs
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

func (c ideConfig) frameUnionCharset() (cs component.FrameUnionCharSet) {
	cs = component.DefaultFrameUnionCharSet()
	cfg, ok := c.browser()
	if !ok {
		return
	}

	cfg, err := cfg.GetConfig("frameunion_charset")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["browser.frameunion_charset"] = err
		}
		return
	}

	left, err := cfg.GetRune("left")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["browser.frameunion_charset.left"] = err
		}
	} else {
		cs.Left = left
	}

	right, err := cfg.GetRune("right")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["browser.frameunion_charset.right"] = err
		}
	} else {
		cs.Right = right
	}

	top, err := cfg.GetRune("top")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["browser.frameunion_charset.top"] = err
		}
	} else {
		cs.Top = top
	}

	bottom, err := cfg.GetRune("bottom")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["browser.frameunion_charset.bottom"] = err
		}
	} else {
		cs.Bottom = bottom
	}

	return
}

func (c ideConfig) windowManagerConfig() component.WindowManagerConfig {
	return component.WindowManagerConfig{
		Frame:        c.frame(),
		FrameAttr:    c.windowFrameAttr(),
		FrameCharSet: c.windowFrameCharset(),
	}
}

func (c ideConfig) browser() (plugin.Config, bool) {
	if c.cfg == nil {
		return nil, false
	}
	return c.getConfig(plugin.MapConfig(c.cfg), "browser")
}

func (c ideConfig) vi() (plugin.Config, bool) {
	if c.cfg == nil {
		return nil, false
	}
	return c.getConfig(plugin.MapConfig(c.cfg), "vi")
}

func (c ideConfig) viResultAttr() (attr term.Attributes) {
	attr = term.Attributes{Bg: term.ColorYellow, Fg: term.ColorBlack}
	cfg, ok := c.vi()
	if !ok {
		return
	}
	attr, err := cfg.GetAttributes("search_attr")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["vi.search_attr"] = err
		}
	}
	return attr
}

func (c ideConfig) viDebug() (ret bool) {
	cfg, ok := c.vi()
	if !ok {
		return
	}
	ret, err := cfg.GetBool("debug")
	if err != nil {
		if err != plugin.ErrNotFound {
			c.errors["vi.debug"] = err
		}
	}
	return ret
}

func (c ideConfig) browserTabspaces() (tabs int) {
	tabs = editor.DefaultConfig().Tabspaces
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
	dir = editor.DefaultConfig().SwapDir
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
