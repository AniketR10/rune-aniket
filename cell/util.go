package cell

import "github.com/ernestrc/go-tui/term"

// SortFromToBlock sorts a pair of coordinates (from/to) such that:
//
//  				cases
//
//  	┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
//  	│ f    ││ t    ││    f ││    t ││ t  f ││ f  t │
//  	│    t ││    f ││ t    ││ f    ││      ││      │
//  	└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
//         |       |        |       |       |       |
//         v       v        v       v       v       v
//  	┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
//  	│ f    ││ f    ││ f    ││ f    ││ f  t ││ f  t │
//  	│    t ││    t ││    t ││    t ││      ││      │
//  	└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
//
func SortFromToBlock(from term.Coordinates, to term.Coordinates) (
	term.Coordinates, term.Coordinates,
) {
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

// SortFromTo sorts a pair of coordinates (from/to) such that:
//
// 				cases
//
// 	┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
// 	│ f    ││ t    ││    f ││    t ││ t  f ││ f  t │
// 	│    t ││    f ││ t    ││ f    ││      ││      │
// 	└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
//        |       |        |       |       |       |
//        v       v        v       v       v       v
// 	┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
// 	│ f    ││ f    ││    f ││    f ││ f  t ││ f  t │
// 	│    t ││    t ││ t    ││ t    ││      ││      │
// 	└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
func SortFromTo(from term.Coordinates, to term.Coordinates) (
	term.Coordinates, term.Coordinates,
) {
	if from.Y > to.Y {
		temp := from.Y
		from.Y = to.Y
		to.Y = temp

		temp = from.X
		from.X = to.X
		to.X = temp
	} else if from.Y == to.Y && from.X > to.X {
		temp := from.X
		from.X = to.X
		to.X = temp
	}
	return from, to
}
