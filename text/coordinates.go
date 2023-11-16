package text

import (
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// ScrollToWindowCoordinates translates scroll content Coordinates to window Coordinates,
// taking into consideration scroll offsets and wrapped lines.
func ScrollToWindowCoordinates(scroll *component.Scroll, pos term.Coordinates) term.Coordinates {
	offset := scroll.Offset()
	ret := cell.CoordinatesDiff(pos, offset)
	if !scroll.Wrap {
		return ret
	}
	wraps := scroll.Wraps()
	for y, count := range wraps {
		if y >= pos.Y {
			break
		}
		ret.Y += count
	}
	diff := pos.X / scroll.Width()
	ret.X = pos.X%scroll.Width() - offset.X
	ret.Y += diff
	return ret
}

// WindowToScrollCoordinates translates window Coordinates to scroll content Coordinates,
// taking into consideration scroll offsets and wrapped lines.
func WindowToScrollCoordinates(scroll *component.Scroll, pos term.Coordinates) term.Coordinates {
	offset := scroll.Offset()
	ret := cell.CoordinatesSum(pos, offset)
	if !scroll.Wrap {
		return ret
	}
	wraps := scroll.Wraps()
	var sum, count int
	for _, count = range wraps {
		if pos.Y+offset.Y >= sum+count+1 {
			ret.Y -= count
			sum += count + 1
			continue
		}
		diff := pos.Y + offset.Y - sum
		ret.X = pos.X + diff*scroll.Width() + offset.X
		ret.Y -= diff
		return ret
	}
	if len(wraps) == 0 {
		return ret
	}
	// always treat pos.Y >= rows as a continuation of the last line
	// this is important for support of insert at x=width
	rowsPastLastLine := ret.Y + 1 - len(wraps)
	ret.Y -= rowsPastLastLine
	ret.X = scroll.Buffer().Columns(ret.Y) + (rowsPastLastLine-1)*scroll.Width()
	return ret
}
