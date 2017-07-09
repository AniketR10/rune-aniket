package less

import termbox "github.com/nsf/termbox-go"

type Config struct {
	tabspaces    int
	fg           termbox.Attribute
	bg           termbox.Attribute
	msgwidth     int8 // 0 - 100%
	cmdBarHeight int  // in cells
	wrap         bool
	debug        bool
	resfg        termbox.Attribute
	resbg        termbox.Attribute
	// TODO keyMap *KeyMap
}

var defaultConfig = Config{
	tabspaces:    4,
	fg:           termbox.ColorDefault,
	bg:           termbox.ColorDefault,
	msgwidth:     70,
	cmdBarHeight: 1,
	wrap:         false,
	debug:        false,
	resfg:        termbox.AttrReverse,
	resbg:        termbox.ColorDefault,
}
