package term

// Coordinates represent a point in a 2-D space.
type Coordinates struct {
	X, Y int
}

// CoordinatesBlockSort sorts a pair of coordinates (from/to) such that:
//
//					cases
//
//		┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
//		│ f    ││ t    ││    f ││    t ││ t  f ││ f  t │
//		│    t ││    f ││ t    ││ f    ││      ││      │
//		└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
//	       |       |        |       |       |       |
//	       v       v        v       v       v       v
//		┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
//		│ f    ││ f    ││ f    ││ f    ││ f  t ││ f  t │
//		│    t ││    t ││    t ││    t ││      ││      │
//		└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
func CoordinatesBlockSort(from Coordinates, to Coordinates) (
	Coordinates, Coordinates,
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

// CoordinatesSort sorts a pair of coordinates (from/to) such that:
//
//					cases
//
//		┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
//		│ f    ││ t    ││    f ││    t ││ t  f ││ f  t │
//		│    t ││    f ││ t    ││ f    ││      ││      │
//		└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
//	       |       |        |       |       |       |
//	       v       v        v       v       v       v
//		┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
//		│ f    ││ f    ││    f ││    f ││ f  t ││ f  t │
//		│    t ││    t ││ t    ││ t    ││      ││      │
//		└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
func CoordinatesSort(from Coordinates, to Coordinates) (
	Coordinates, Coordinates,
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

// CoordinatesDiff subtracts a from b.
func CoordinatesDiff(a, b Coordinates) Coordinates {
	return Coordinates{
		Y: a.Y - b.Y,
		X: a.X - b.X,
	}
}

// CoordinatesSum adds a to b.
func CoordinatesSum(a, b Coordinates) Coordinates {
	return Coordinates{
		Y: a.Y + b.Y,
		X: a.X + b.X,
	}
}
