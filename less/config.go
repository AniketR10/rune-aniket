package less

import fractal "github.com/ernestrc/fractal"

type Config struct {
	Tabspaces    int
	FG           fractal.Attribute
	BG           fractal.Attribute
	Msgwidth     int8 // 0 - 100%
	CmdBarHeight int  // in cells
	Wrap         bool
	Debug        bool
	ResFG        fractal.Attribute
	ResBG        fractal.Attribute
	WindowBorder fractal.Attribute
	// TODO keyMap *KeyMap
}

var defaultConfig = Config{
	Tabspaces:    8,
	FG:           fractal.ColorDefault,
	BG:           fractal.ColorDefault,
	Msgwidth:     70,
	CmdBarHeight: 1,
	Wrap:         false,
	Debug:        false,
	ResFG:        fractal.AttrReverse,
	ResBG:        fractal.ColorDefault,
}

func DefaultConfig() *Config {
	cfg := new(Config)
	*cfg = defaultConfig
	return cfg
}
