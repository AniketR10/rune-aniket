package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

type grid struct {
	width, height int
	matrix        [][]tui.Component
}

// Grid renders matrix as an equaly sized grid of components.
// Note that each []tui.Component in matrix is treated as a row.
func Grid(matrix [][]tui.Component) tui.Component {
	return &grid{matrix: matrix}
}

func (g *grid) Resize(width, height int) {
	g.width, g.height = width, height
}

func (g *grid) Draw(w term.Writer) {
	var v Virtual

	if len(g.matrix) == 0 {
		return
	}

	heightPerComp := g.height / len(g.matrix)
	for y, row := range g.matrix {
		yComp := y * heightPerComp
		if yComp+heightPerComp > g.height {
			break
		}
		if len(row) == 0 {
			continue
		}
		widthPerComp := g.width / len(row)
		for x, comp := range row {
			xComp := x * widthPerComp
			if xComp+widthPerComp > g.width {
				break
			}

			v.C = comp
			v.Resize(widthPerComp, heightPerComp)
			v.Move(term.Coordinates{X: xComp, Y: yComp})
			v.Draw(w)
		}
	}
}
