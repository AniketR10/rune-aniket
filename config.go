package less

import termbox "github.com/nsf/termbox-go"

type Config struct {
	Tabspaces    int
	Fg           termbox.Attribute
	Bg           termbox.Attribute
	Msgwidth     int8 // 0 - 100%
	CmdBarHeight int  // in cells
	Wrap         bool
	Debug        bool
	Resfg        termbox.Attribute
	Resbg        termbox.Attribute
	// TODO keyMap *KeyMap
}

var defaultConfig = Config{
	Tabspaces:    4,
	Fg:           termbox.ColorDefault,
	Bg:           termbox.ColorDefault,
	Msgwidth:     70,
	CmdBarHeight: 1,
	Wrap:         false,
	Debug:        false,
	Resfg:        termbox.AttrReverse,
	Resbg:        termbox.ColorDefault,
}

func NewConfig() *Config {
	cfg := new(Config)
	*cfg = defaultConfig
	return cfg
}
