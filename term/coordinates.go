// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package term

// Coordinates represent a point in a 2-D space.
type Coordinates struct {
	X, Y int
}

// CoordinatesBlockSort sorts a pair of coordinates (from/to) such that:
//
//		┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
//		│ f    ││ t    ││    f ││    t ││ t  f ││ f  t │
//		│    t ││    f ││ t    ││ f    ││      ││      │
//		└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
//	      |       |        |       |       |       |
//	      v       v        v       v       v       v
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
//		┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
//		│ f    ││ t    ││    f ││    t ││ t  f ││ f  t │
//		│    t ││    f ││ t    ││ f    ││      ││      │
//		└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
//	      |       |        |       |       |       |
//	      v       v        v       v       v       v
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

// CoordinatesIntersection calculates the intersection a ∩ b in a 2D space,
// defined as the set of all those cells which are common to both
// a and b. Both a and b are expected to be right-exclusive ranges.
//
//	┌──────┐     ┌──────┐     ┌──────┐
//	│ AA   │  ∩  │      │  =  │      │
//	│      │     │   BB │     │      │
//	└──────┘     └──────┘     └──────┘
//	┌──────┐     ┌──────┐     ┌──────┐
//	│ AAAAA│  ∩  │      │  =  │      │
//	│AAA   │     │BBBBB │     │CCC   │
//	└──────┘     └──────┘     └──────┘
//	┌──────┐     ┌──────┐     ┌──────┐
//	│  BBBB│  ∩  │      │  =  │      │
//	│BBB   │     │AAAAA │     │CCC   │
//	└──────┘     └──────┘     └──────┘
//	┌──────┐     ┌──────┐     ┌──────┐
//	│     A│  ∩  │      │  =  │      │
//	│AAAAAA│     │BBB   │     │CCC   │
//	└──────┘     └──────┘     └──────┘
func CoordinatesIntersection(
	startA, endA, startB, endB Coordinates,
) (intersectionStart Coordinates, intersectionEnd Coordinates, ok bool) {
	startA, endA = CoordinatesSort(startA, endA)
	startB, endB = CoordinatesSort(startB, endB)

	// conflate non-sorted cases into sorted
	actualStartA, actualStartB := CoordinatesSort(startA, startB)
	if actualStartA != startA {
		temp := endB
		endB = endA
		endA = temp
		startA = actualStartA
		startB = actualStartB
	}

	if startB.Y > endA.Y || (startB.Y == endA.Y && startB.X >= endA.X) ||
		startA == endA || startB == endB {
		return
	}

	intersectionStart = startB
	intersectionEnd, _ = CoordinatesSort(endA, endB)
	ok = true
	return
}

// CoordinatesInBounds returns true if the given position is
// within the given right-exclusive bounds.
func CoordinatesInBounds(pos Coordinates, bounds Coordinates) bool {
	return pos.X >= 0 && pos.Y >= 0 && pos.X < bounds.X && pos.Y < bounds.Y
}
