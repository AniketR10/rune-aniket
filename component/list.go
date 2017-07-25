package component

import "github.com/ernestrc/fractal"

const resfg, resbg = fractal.AttrReverse, fractal.AttrReverse

type List struct {
	factory       fractal.Factory
	children      []fractal.Component
	rows          []*fractal.Buffer
	width, height int
	fractal.Coordinates
}

func (n *List) NewList(rowHeight, width, height int) (c *List) {
	c = new(List)
	c.children = make([]fractal.Component, height)
	return
}

func (n *List) Clear() (err error) {
	n.rows = n.rows[:0]
	return n.Resize(n.width, n.height)
}

func (n *List) Set(rows []*fractal.Buffer) (err error) {
	n.rows = rows
	return n.Resize(n.width, n.height)
}

func (n *List) Add(content *fractal.Buffer) (err error) {
	n.rows = append(n.rows, content)
	return n.Resize(n.width, n.height)
}

func (n *List) Pop() (row *fractal.Buffer) {
	n.rows, row = n.rows[1:], n.rows[0]
	return
}

func (n *List) Rows() []*fractal.Buffer {
	return n.rows
}

func (n *List) Resize(width, height int) (err error) {
	children := n.children[:0]

	var row fractal.Component
	for i := 0; i < height && i < len(n.rows); i++ {
		if n.height > i {
			row = n.children[i]
			if err = row.Resize(width, 1); err != nil {
				return
			}
		} else {
			row = n.factory(n.rows[i])
			if err = row.Move(n.X, n.Y+i); err != nil {
				return
			}
		}
		children = append(children, row)
	}

	n.children = children
	n.width, n.height = width, height

	return
}

func (n *List) Move(x, y int) (err error) {
	n.X, n.Y = x, y

	for i, row := range n.children {
		if err = row.Move(n.X, n.Y+i); err != nil {
			return
		}
	}

	return
}

func (n *List) Draw(w fractal.Writer) (err error) {
	for _, row := range n.children {
		if err = row.Draw(w); err != nil {
			return
		}
	}

	return
}

func (n *List) Height() int {
	return n.height
}

func (n *List) Width() int {
	return n.width
}

func (n *List) Position() (int, int) {
	return n.X, n.Y
}
