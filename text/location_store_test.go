package text

import (
	"context"
	"strconv"
	"testing"

	"github.com/unstablebuild/tcell/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

func TestLocationStoreCursorIntegrationSortedLocations(t *testing.T) {
	t.Run("sorts by level", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "\na\nb\nc\n", false)

		infoList := LocationSlice([]textapi.Location{
			{
				From:    term.Coordinates{Y: 1},
				To:      term.Coordinates{Y: 1, X: 1},
				Attr:    abcAttr,
				Message: "info",
			},
			{
				From:    term.Coordinates{Y: 11},
				To:      term.Coordinates{Y: 11, X: 11},
				Attr:    abcAttr,
				Message: "info2",
			},
		})
		errList := LocationSlice([]textapi.Location{
			{
				From:    term.Coordinates{Y: 2},
				To:      term.Coordinates{Y: 2, X: 2},
				Attr:    abcAttr,
				Message: "err",
			},
		})
		criticalList := LocationSlice([]textapi.Location{
			{
				From:    term.Coordinates{Y: 3},
				To:      term.Coordinates{Y: 3, X: 3},
				Attr:    abcAttr,
				Message: "critical",
			},
		})
		warnList := LocationSlice([]textapi.Location{
			{
				From:    term.Coordinates{Y: 4},
				To:      term.Coordinates{Y: 4, X: 4},
				Attr:    abcAttr,
				Message: "warn",
			},
		})
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityError, "errList", errList))
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, "infoList", infoList))
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityCritical, "criticalList", criticalList))
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityWarning, "warnList", warnList))

		locations := c.SortedLocations()
		require.Len(t, locations, 5)

		assert.Equal(t, term.Coordinates{Y: 1}, locations[0].From)
		assert.Equal(t, term.Coordinates{Y: 1, X: 1}, locations[0].To)
		assert.Equal(t, "info", locations[0].Message)

		assert.Equal(t, "info2", locations[1].Message)
		assert.Equal(t, "warn", locations[2].Message)
		assert.Equal(t, "err", locations[3].Message)
		assert.Equal(t, "critical", locations[4].Message)
	})

	t.Run("sorts by id if level is the same", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "\na\nb\nc\n", false)
		infoList := LocationSlice([]textapi.Location{
			{
				From:    term.Coordinates{Y: 1},
				To:      term.Coordinates{Y: 1, X: 1},
				Attr:    abcAttr,
				Message: "info",
			},
			{
				From:    term.Coordinates{Y: 11},
				To:      term.Coordinates{Y: 11, X: 11},
				Attr:    abcAttr,
				Message: "info1",
			},
		})
		infoList2 := LocationSlice([]textapi.Location{
			{
				From:    term.Coordinates{Y: 2},
				To:      term.Coordinates{Y: 2, X: 2},
				Attr:    abcAttr,
				Message: "info2",
			},
		})
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, "infoList2", infoList2))
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, "infoList", infoList))

		locations := c.SortedLocations()
		require.Len(t, locations, 3)

		assert.Equal(t, term.Coordinates{Y: 1}, locations[0].From)
		assert.Equal(t, term.Coordinates{Y: 1, X: 1}, locations[0].To)
		assert.Equal(t, "info", locations[0].Message)
		assert.Equal(t, "info1", locations[1].Message)
		assert.Equal(t, "info2", locations[2].Message)
	})

	t.Run("idempotency", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "\na\nb\nc\n", false)
		infoList := LocationSlice([]textapi.Location{
			{
				From:    term.Coordinates{Y: 1},
				To:      term.Coordinates{Y: 1, X: 1},
				Attr:    abcAttr,
				Message: "info",
			},
			{
				From:    term.Coordinates{Y: 11},
				To:      term.Coordinates{Y: 11, X: 11},
				Attr:    abcAttr,
				Message: "info1",
			},
		})
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, "infoList", infoList))

		require.Len(t, c.SortedLocations(), 2)
		require.Len(t, c.SortedLocations(), 2)
		require.Len(t, c.SortedLocations(), 2)
		require.Len(t, c.SortedLocations(), 2)
	})
}

func TestLocationStoreCursorIntegrationSetLocationListMessages(t *testing.T) {
	content := "\naaa\nbbb\nccc\n"
	messageLocations := []textapi.Location{
		{
			From:    term.Coordinates{Y: 1},
			To:      term.Coordinates{Y: 1, X: 2},
			Attr:    abcAttr,
			Message: "1",
		},
		{
			From:    term.Coordinates{Y: 2},
			To:      term.Coordinates{Y: 2, X: 2},
			Attr:    abcAttr,
			Message: "2",
		},
		{
			From:    term.Coordinates{Y: 3},
			To:      term.Coordinates{Y: 3, X: 2},
			Attr:    abcAttr,
			Message: "3",
		},
	}

	assertMessages := func(t *testing.T, c *Cursor) {
		for i := 0; i < 3; i++ {
			locsByID, ok := c.LocationsAtCursor()
			require.True(t, ok)
			require.Len(t, locsByID, 1)
			assert.Equal(t, locsByID[locID].Message, strconv.Itoa(i+1))
			c.MoveDown()
		}
	}

	t.Run("returns nil/false if cursor is not in from, to or in between", func(t *testing.T) {
		c := setupCursorContent(t, 10, 2, content, false)
		abcList := LocationSlice(messageLocations)
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, locID, abcList))

		_, ok := c.LocationsAtCursor()
		assert.False(t, ok)
	})

	t.Run("return messages if cursor is at From", func(t *testing.T) {
		c := setupCursorContent(t, 10, 2, content, false)
		abcList := LocationSlice(messageLocations)
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, locID, abcList))
		require.True(t, c.MoveDown())

		assertMessages(t, c)
	})

	t.Run("return messages if cursor between From/To", func(t *testing.T) {
		c := setupCursorContent(t, 10, 2, content, false)

		abcList := LocationSlice(messageLocations)

		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, locID, abcList))
		require.True(t, c.MoveDown())
		require.True(t, c.MoveRight())

		assertMessages(t, c)
	})

	t.Run("return messages if cursor is at To", func(t *testing.T) {
		c := setupCursorContent(t, 10, 2, content, false)

		abcList := LocationSlice(messageLocations)

		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, locID, abcList))
		require.True(t, c.MoveDown())
		c.MoveRight()
		c.MoveRight()

		assertMessages(t, c)
	})
}

func TestCursorDrawLocationListsIntegration(t *testing.T) {
	t.Run("sets location list attrs", func(t *testing.T) {
		c := setupCursorContent(t, 1, 5, "\na\nb\nc\n", false)

		expected := [][]term.Cell{
			{{}},
			{{Attributes: abcAttr}},
			{{Attributes: abcAttr}},
			{{Attributes: abcAttr}},
			{{}},
		}

		abcList := LocationSlice(abcLocations)

		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, locID, abcList))

		w := cell.NewBufferWriter(context.Background(), 1, 5)
		DrawLocations(c.SortedLocations(), c.scroll, w)
		assert.Equal(t, expected, w.RawCells())
	})

	t.Run("clears location lists", func(t *testing.T) {
		c := setupCursorContent(t, 1, 5, "\na\nb\nc\n", false)
		abcList := LocationSlice(abcLocations)

		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, locID, abcList))
		assert.NotNil(t, c.SetLocationList(textapi.LocationPriorityInfo, locID, nil))

		expected := [][]term.Cell{
			{{}},
			{{}},
			{{}},
			{{}},
			{{}},
		}

		w := cell.NewBufferWriter(context.Background(), 1, 5)
		DrawLocations(c.SortedLocations(), c.scroll, w)
		assert.Equal(t, expected, w.RawCells())
	})

	t.Run("lower priority lists do not override higher priority list attrs", func(t *testing.T) {
		infoLocations := []textapi.Location{
			{
				From: term.Coordinates{Y: 1},
				To:   term.Coordinates{Y: 1, X: 1},
				Attr: term.Attributes{Fg: tcell.ColorRed, Bg: tcell.ColorGreen},
			},
			{
				From: term.Coordinates{Y: 2},
				To:   term.Coordinates{Y: 2, X: 1},
				Attr: term.Attributes{Fg: tcell.ColorRed, Bg: tcell.ColorGreen},
			},
			{
				From: term.Coordinates{Y: 3},
				To:   term.Coordinates{Y: 3, X: 1},
				Attr: term.Attributes{Fg: tcell.ColorRed, Bg: tcell.ColorGreen},
			},
		}
		criticalLocations := []textapi.Location{
			{
				From: term.Coordinates{Y: 1},
				To:   term.Coordinates{Y: 1, X: 1},
				Attr: term.Attributes{Attrs: tcell.AttrUnderline, Bg: tcell.ColorBlack},
			},
			{
				From: term.Coordinates{Y: 2},
				To:   term.Coordinates{Y: 2, X: 1},
				Attr: term.Attributes{Attrs: tcell.AttrUnderline, Bg: tcell.ColorBlack},
			},
			{
				From: term.Coordinates{Y: 3},
				To:   term.Coordinates{Y: 3, X: 1},
				Attr: term.Attributes{Attrs: tcell.AttrUnderline, Bg: tcell.ColorBlack},
			},
		}
		c := setupCursorContent(t, 1, 5, "\na\nb\nc\n", false)

		expected := [][]term.Cell{
			{{}},
			{{Attributes: term.Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrUnderline, Bg: tcell.ColorBlack}}},
			{{Attributes: term.Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrUnderline, Bg: tcell.ColorBlack}}},
			{{Attributes: term.Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrUnderline, Bg: tcell.ColorBlack}}},
			{{}},
		}

		criticalList := LocationSlice(criticalLocations)
		infoList := LocationSlice(infoLocations)
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityCritical, "list1", criticalList))
		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, "list2", infoList))

		w := cell.NewBufferWriter(context.Background(), 1, 5)
		DrawLocations(c.SortedLocations(), c.scroll, w)
		assert.Equal(t, expected, w.RawCells())
	})

	t.Run("trims to fit location To line if From is in bounds", func(t *testing.T) {
		c := setupCursorContent(t, 1, 5, "\na\nb\nc\n", false)

		expected := [][]term.Cell{
			{{}},
			{{}},
			{{Attributes: abcAttr}},
			{{Attributes: abcAttr}},
			{{Attributes: abcAttr}},
		}
		locations := []textapi.Location{
			{
				From: term.Coordinates{Y: 2},
				To:   term.Coordinates{Y: 6, X: 1},
				Attr: abcAttr,
			},
		}

		abcList := LocationSlice(locations)

		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, locID, abcList))

		w := cell.NewBufferWriter(context.Background(), 1, 5)
		DrawLocations(c.SortedLocations(), c.scroll, w)
		assert.Equal(t, expected, w.RawCells())
	})

	t.Run("does not panic if width == 0 in wrap mode", func(t *testing.T) {
		c := setupCursorContent(t, 0, 5, "\na\nb\nc\n", false)
		c.scroll.Wrap = true

		expected := [][]term.Cell{
			{{}},
			{{}},
			{{}},
			{{}},
			{{}},
		}
		locations := []textapi.Location{
			{
				From: term.Coordinates{Y: 0},
				To:   term.Coordinates{Y: 1, X: 1},
				Attr: abcAttr,
			},
		}

		abcList := LocationSlice(locations)

		assert.Nil(t, c.SetLocationList(textapi.LocationPriorityInfo, locID, abcList))

		w := cell.NewBufferWriter(context.Background(), 1, 5)
		DrawLocations(c.SortedLocations(), c.scroll, w)
		assert.Equal(t, expected, w.RawCells())
	})
}
