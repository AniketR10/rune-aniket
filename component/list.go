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

func (l *List) NewList( /*rowHeight,*/ width, height int, factory fractal.Factory) (c *List) {
	c = new(List)
	return
}

func (l *List) Clear() (err error) {
	l.rows = l.rows[:0]
	return l.Resize(l.width, l.height)
}

func (l *List) Set(rows []*fractal.Buffer) (err error) {
	l.rows = rows
	return l.Resize(l.width, l.height)
}

func (l *List) Add(content *fractal.Buffer) (err error) {
	l.rows = append(l.rows, content)
	return l.Resize(l.width, l.height)
}

func (l *List) Pop() (row *fractal.Buffer) {
	l.rows, row = l.rows[1:], l.rows[0]
	return
}

func (l *List) Rows() []*fractal.Buffer {
	return l.rows
}

func (l *List) Resize(width, height int) (err error) {
	if l.children == nil {
		l.children = make([]fractal.Component, height)
	}
	pchildren := l.children
	pheight := l.height
	l.children = l.children[:0]
	l.width, l.height = width, height

	var row fractal.Component
	for i := 0; i < l.height && i < len(l.rows); i++ {
		if pheight > i {
			// reuse component
			row = pchildren[i]
			if err = row.Resize(width, 1); err != nil {
				return
			}
		} else {
			// create new coponent
			row = l.factory(l.rows[i])
			if err = row.Move(l.X, l.Y+i); err != nil {
				return
			}
		}
		l.children = append(l.children, row)
	}

	return
}

func (l *List) Move(x, y int) (err error) {
	l.X, l.Y = x, y

	for i, row := range l.children {
		if err = row.Move(l.X, l.Y+i); err != nil {
			return
		}
	}

	return
}

func (l *List) Draw(w fractal.Writer) (err error) {
	for _, row := range l.children {
		if err = row.Draw(w); err != nil {
			return
		}
	}

	return
}

func (l *List) Height() int {
	return l.height
}

func (l *List) Width() int {
	return l.width
}

func (l *List) Position() (int, int) {
	return l.X, l.Y
}
