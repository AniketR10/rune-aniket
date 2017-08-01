package less

import (
	termbox "github.com/nsf/termbox-go"
)

type Config struct {
	Tabspaces    int
	FG           termbox.Attribute
	BG           termbox.Attribute
	Msgwidth     int8 // 0 - 100%
	CmdBarHeight int  // in cells
	Wrap         bool
	Debug        bool
	ResFG        termbox.Attribute
	ResBG        termbox.Attribute
	ScrollBorder termbox.Attribute
	// TODO keyMap *KeyMap
}

var defaultConfig = Config{
	Tabspaces:    8,
	FG:           termbox.ColorDefault,
	BG:           termbox.ColorDefault,
	Msgwidth:     70,
	CmdBarHeight: 1,
	Wrap:         false,
	Debug:        false,
	ResFG:        termbox.AttrReverse,
	ResBG:        termbox.ColorDefault,
}

func DefaultConfig() *Config {
	cfg := new(Config)
	*cfg = defaultConfig
	return cfg
}
