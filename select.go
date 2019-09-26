package fractal

import (
	"math"

	"github.com/nsf/termbox-go"
)

/*
*					cases
*
*		┌──────┐┌──────┐┌──────┐┌──────┐
*		│ f    ││ t    ││    f ││    t │
*		│    t ││    f ││ t    ││ f    │
*		└──────┘└──────┘└──────┘└──────┘
*          |       |        |       |
*          v       v        v       v
*		┌──────┐┌──────┐┌──────┐┌──────┐
*		│ f    ││ f    ││ f    ││ f    │
*		│    t ││    t ││    t ││    t │
*		└──────┘└──────┘└──────┘└──────┘
 */
func sortFromTo(from Coordinates, to Coordinates) (Coordinates, Coordinates) {
	if from.X > to.X {
		temp := from.X
		from.X = to.X
		to.X = temp
	}
	if from.Y > to.Y {
		temp := from.Y
		from.Y = to.Y
		to.Y = temp
	}
	return from, to
}

// Select returns the cells inside the given coordinates or nil if coordinates are out of bounds.
func Select(cells [][]termbox.Cell, from Coordinates, to Coordinates) (res [][]termbox.Cell) {
	res = make([][]termbox.Cell, 0)

	from, to = sortFromTo(from, to)

	for from.Y < to.Y && from.Y < len(cells) {
		x := int(math.Min(float64(from.X), float64(len(cells[from.Y]))))
		res = append(res, cells[from.Y][x:])
		from.X = 0
		from.Y++
	}

	if from.Y >= len(cells) {
		return
	}

	fromX := int(math.Min(float64(from.X), float64(len(cells[from.Y]))))
	toX := int(math.Min(float64(to.X+1), float64(len(cells[from.Y]))))
	res = append(res, cells[from.Y][fromX:toX])
	return
}

// SelectLine returns the lines inside the given coordinates or nil if coordinates are out of bounds.
func SelectLine(cells [][]termbox.Cell, from Coordinates, to Coordinates) (res [][]termbox.Cell) {
	res = make([][]termbox.Cell, 0)

	from, to = sortFromTo(from, to)

	for from.Y <= to.Y && from.Y < len(cells) {
		res = append(res, cells[from.Y][:])
		from.Y++
	}

	return
}

// SelectBlock returns the block of cells inside the given coordinates or nil if coordinates are out of bounds.
func SelectBlock(cells [][]termbox.Cell, from Coordinates, to Coordinates) (res [][]termbox.Cell) {
	res = make([][]termbox.Cell, 0)

	from, to = sortFromTo(from, to)

	for from.Y <= to.Y && from.Y < len(cells) {
		maxy := float64(len(cells[from.Y]))
		xfrom := int(math.Min(maxy, float64(from.X)))
		xto := int(math.Min(maxy, float64(to.X+1)))
		res = append(res, cells[from.Y][xfrom:xto])
		from.Y++
	}

	return
}
