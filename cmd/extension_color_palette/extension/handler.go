package extension

import (
	"context"
	"fmt"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
)

var colorPaletteCmd = textapi.CommandManual{
	Name: "colorPalette",
	Summary: "Opens a new window and displays all the color codes available " +
		"to customize the UI via configuration.",
}

// Greantee returns this extension's extension.Grantee, and it required permissions.
func Grantee() (extension.Grantee, []extension.Permission) {
	return extutil.NewCommandSplitHandler(extutil.CommandSplitHandlerConfig{
		SplitOrientation: browserapi.OrientationRight,
		Handler: func(ctx context.Context, _ textapi.Command,
			grants []extension.Grant, broker rpc.MuxBroker,
			invokeWindow browserapi.Window, config config.Config) (browserapi.Handler, error) {
			return new(colorPaletteHandler), nil
		},
		Command: colorPaletteCmd,
	})
}

type colorPaletteHandler struct {
	width, height int
	grid          tui.Component
	dim           bool
	dirty         bool
}

func (h *colorPaletteHandler) Resize(width, height int) {
	if h.grid != nil {
		h.grid.Resize(width, height)
	}
	h.width = width
	h.height = height
}

func makeColorGrid(dim bool) tui.Component {
	ret := make([][]tui.Component, 16)
	var nameNum int
	for y := 0; y < 16; y++ {
		ret[y] = make([]tui.Component, 16)
		for x := 0; x < 16; x++ {
			var attrs tcell.AttrMask
			color := tcell.PaletteColor(nameNum)
			name := color.Name(true)
			if dim {
				attrs = tcell.AttrDim
				name = fmt.Sprintf("D%s", name)
			}
			ret[y][x] = component.NewStringWithConfig(name,
				component.StringConfig{
					Attributes:           term.Attributes{Bg: color, Attrs: attrs},
					BackgroundAttributes: term.Attributes{Bg: color, Attrs: attrs},
				},
			)
			nameNum++
		}
	}
	nextGridOf := 12
	nextGrid := make([][]tui.Component, 0, nextGridOf)
	var i, x int
	y := -1
	for name := range tcell.ColorNames {
		if i%nextGridOf == 0 {
			y++
			x = 0
			nextGrid = append(nextGrid, make([]tui.Component, nextGridOf))
		}
		var attrs tcell.AttrMask
		color := tcell.GetColor(name)
		if dim {
			attrs = tcell.AttrDim
			name = fmt.Sprintf("D%s", name)
		}
		nextGrid[y][x] = component.NewStringWithConfig(name,
			component.StringConfig{
				Attributes:           term.Attributes{Bg: color, Attrs: attrs},
				BackgroundAttributes: term.Attributes{Bg: color, Attrs: attrs},
			},
		)
		i++
		x++
	}
	ret = append(ret, nextGrid...)
	return component.Grid(ret)
}

func (h *colorPaletteHandler) Draw(w term.Writer) {
	if h.grid == nil || h.dirty {
		h.dirty = false
		h.grid = makeColorGrid(h.dim)
		h.grid.Resize(h.width, h.height)
	}
	h.grid.Draw(w)
}

func (h *colorPaletteHandler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return
	}
	if ev.Key == term.KeyCtrlD {
		h.dim = !h.dim
		h.dirty = true
		return
	}
	exit = ev.Key == term.KeyEsc
	return
}

func (h *colorPaletteHandler) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	return
}

func (h *colorPaletteHandler) Man() tui.Manual {
	return tui.Manual{}
}

func (h *colorPaletteHandler) Close() error {
	return nil
}
