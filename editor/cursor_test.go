package editor

import (
	"strconv"
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const locID = "errors"
const sampleSnippet = `
/*
 * Check if the current buffer should be added to or removed from the list of
 * diff buffers.
 */
	void
diff_buf_adjust(win_T *win)
{
	win_T	*wp;
	int		i;

	if (!win->w_p_diff)
	{
	/* When there is no window showing a diff for this buffer, remove
	 * it from the diffs. */
	FOR_ALL_WINDOWS(wp)
		if (wp->w_buffer == win->w_buffer && wp->w_p_diff)
		break;
	if (wp == NULL)
	{
		i = diff_buf_idx(win->w_buffer);
		if (i != DB_COUNT)
		{
		curtab->tp_diffbuf[i] = NULL;
		curtab->tp_diff_invalid = TRUE;
		diff_redraw(TRUE);
		}
	}
	}
	else
	diff_buf_add(win->w_buffer);
} /* { */ `

func setupCursorContent(t *testing.T, width, height int, cont string) (e *Cursor) {
	scroll := component.NewScroll()
	e = NewCursor(scroll)
	_, err := scroll.Buffer().ReadFrom(strings.NewReader(cont))
	require.NoError(t, err)
	scroll.Resize(width, height)
	require.Equal(t, e.scroll.Buffer(), scroll.Buffer())
	require.Equal(t, e.subscriber.c, e)

	return
}

func setupCursor(t *testing.T, width, height int) *Cursor {
	return setupCursorContent(t, width, height, sampleSnippet)
}

func TestCursorSearch(t *testing.T) {
	tsuite := []struct {
		desc          string
		width, height int
		results       int
		searchstring  string
		assertions    func(*testing.T, *Cursor)
		cursor        term.Coordinates
	}{
		{
			"does nothing if search text is not found",
			1000, 1000,
			0,
			"nothing",
			func(t *testing.T, e *Cursor) {
				assert.False(t, e.MoveToNextMatch())
			}, term.Coordinates{},
		},
		{
			"moves to the first result if search text is found",
			1000, 1000,
			2,
			"NULL",
			nil, term.Coordinates{X: 14, Y: 18},
		},
		{
			"MoveToNextMatch does nothing if only one result is found",
			1000, 1000,
			1,
			"else",
			func(t *testing.T, e *Cursor) {
				assert.False(t, e.MoveToNextMatch())
			}, term.Coordinates{X: 4, Y: 29},
		},
		{
			"Seeks to first result if not in window",
			100, 10,
			1,
			"When",
			nil,
			term.Coordinates{X: 7, Y: 8},
		},
		{
			"Seeks to last result upon MoveToPrevMatch",
			1000, 1000,
			2,
			"NULL",
			func(t *testing.T, e *Cursor) {
				assert.True(t, e.MoveToPrevMatch())
			}, term.Coordinates{X: 32, Y: 23},
		},
		{
			"Seeks if MoveToNextMatch result is not in window",
			100, 10,
			2,
			"NULL",
			func(t *testing.T, e *Cursor) {
				assert.True(t, e.MoveToNextMatch())
			}, term.Coordinates{X: 32, Y: 8},
		},
	}

	for _, _tcase := range tsuite {
		tcase := _tcase

		t.Run(tcase.desc, func(t *testing.T) {
			e := setupCursor(t, tcase.width, tcase.height)

			require.Equal(t, tcase.results, e.Search(tcase.searchstring))
			if tcase.assertions != nil {
				tcase.assertions(t, e)
			}

			cursor, _ := e.Cursor()
			assert.Equal(t, tcase.cursor, cursor)

			if tcase.results == 0 {
				return
			}

			require.True(t, e.Select())
			for i := 1; i < len(tcase.searchstring); i++ {
				e.MoveRight()
			}
			assert.Equal(t, tcase.searchstring, e.Selection())
		})
	}
}

func TestCursorMove(t *testing.T) {
	tsuite := []struct {
		desc          string
		width, height int
		sut           func(*testing.T, *Cursor)
		cursor        term.Coordinates
	}{
		{
			"MoveStartLine should do nothing if already at start of line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				assert.False(t, e.MoveStartLine())
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveStartLine should move to start of line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				e.cursor.X = 20
				assert.True(t, e.MoveStartLine())
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveStartLine should seek to start of line if start is out of window",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.scroll.SeekEndLine()
				e.cursor.X = 20

				assert.True(t, e.MoveStartLine())

				assert.Equal(t, 0, e.scroll.Offset().X)
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveEndLine should do nothing if already at end of line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				e.cursor = term.Coordinates{X: 1, Y: 1}
				assert.False(t, e.MoveEndLine())
			},
			term.Coordinates{X: 1, Y: 1},
		},
		{
			"MoveEndLine should move to end of line if past the end of line",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.scroll.SeekEndFile()
				e.cursor.X = 9
				e.cursor.Y = 0
				e.scroll.SeekEndLine()
				assert.True(t, e.MoveEndLine())
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveEndLine should move cursor to end of line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 1
				assert.True(t, e.MoveEndLine())
			},
			term.Coordinates{X: 1, Y: 1},
		},
		{
			"MoveEndLine should seek to end of line if end is out of window",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 2

				assert.True(t, e.MoveEndLine())

				c, _ := e.scroll.Buffer().Cell(e.cursorAtScroll())
				assert.Equal(t, 76, e.scroll.Offset().X+e.cursor.X)
				assert.Equal(t, 'f', c.Ch)
			},
			term.Coordinates{X: 8, Y: 2},
		},
		{
			"MoveFirstLine should do nothing if already on first line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				assert.False(t, e.MoveFirstLine())
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveFirstLine should move to first line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 10
				assert.True(t, e.MoveFirstLine())
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveFirstLine should seek to first line if not in window",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.scroll.SeekEndFile()
				e.cursor.Y = 2
				assert.True(t, e.MoveFirstLine())
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveLastLine should do nothing if already on last line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 31
				assert.False(t, e.MoveLastLine())
			},
			term.Coordinates{X: 0, Y: 31},
		},
		{
			"MoveLastLine should move to first line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				assert.True(t, e.MoveLastLine())
			},
			term.Coordinates{X: 0, Y: 31},
		},
		{
			"MoveLastLine should seek to last line if not in window",
			10, 10,
			func(t *testing.T, e *Cursor) {
				assert.True(t, e.MoveLastLine())

				c, _ := e.scroll.Buffer().Cell(e.cursorAtScroll())
				assert.Equal(t, 31, e.scroll.Offset().Y+e.cursor.Y)
				assert.Equal(t, '}', c.Ch)
			},
			term.Coordinates{X: 0, Y: 9},
		},
		{
			"MoveDown should move the cursor position past the last line until end of window",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 31
				for i := 0; i < e.scroll.Height(); i++ {
					e.MoveDown()
				}
			},
			term.Coordinates{X: 0, Y: 999},
		},
		{
			"MoveDown should seek down if reached last line in window but not at last line",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 9
				assert.True(t, e.MoveDown())
				assert.Equal(t, 1, e.scroll.Offset().Y)
			},
			term.Coordinates{X: 0, Y: 9},
		},
		{
			"MoveDown should NOT seek up if reached last line",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 9
				e.scroll.SeekEndFile()
				offsetY := e.scroll.Offset().Y
				assert.False(t, e.MoveDown())
				assert.Equal(t, offsetY, e.scroll.Offset().Y)
			},
			term.Coordinates{X: 0, Y: 9},
		},
		{
			"MoveUp should do nothing if already on first line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				assert.False(t, e.MoveUp())
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveUp should move the cursor up one line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 10
				assert.True(t, e.MoveUp())
			},
			term.Coordinates{X: 0, Y: 9},
		},
		{
			"MoveUp should seek up if not at first line and cursor is at first line of window",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.scroll.SeekEndFile()
				offsetY := e.scroll.Offset().Y
				assert.True(t, e.MoveUp())
				assert.Equal(t, offsetY-1, e.scroll.Offset().Y)
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveUp should NOT seek up if already at first line",
			10, 10,
			func(t *testing.T, e *Cursor) {
				offsetY := e.scroll.Offset().Y
				assert.False(t, e.MoveUp())
				assert.Equal(t, offsetY, e.scroll.Offset().Y)
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveLeft should do nothing if already at start of line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				assert.False(t, e.MoveLeft())
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveLeft should move cursor left",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				e.cursor.X = 1
				assert.True(t, e.MoveLeft())
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveLeft should seek left if at start of window but not at start of line",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.scroll.SeekEndLine()
				assert.NotZero(t, e.scroll.Offset().X)

				for i := 0; i < 100; i++ {
					e.MoveLeft()
				}
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveRight should move cursor right",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				assert.True(t, e.MoveRight())
			},
			term.Coordinates{X: 1, Y: 0},
		},
		{
			"MoveRight should move cursor right even if at the end of the line",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				e.cursor.X = 1
				assert.True(t, e.MoveRight())
			},
			term.Coordinates{X: 2, Y: 0},
		},
		{
			"MoveRight should seek right if at end of window but not at end of line",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 2
				e.cursor.X = 5
				e.scroll.SeekStartLine()

				for i := 0; i < 100; i++ {
					e.MoveRight()
				}
			},
			term.Coordinates{X: 9, Y: 2},
		},
		{
			"MoveRightStartWord should move to the start of the next word",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 2
				e.cursor.X = 9

				assert.True(t, e.MoveRightStartWord())

				c, _ := e.scroll.Buffer().Cell(e.cursorAtScroll())
				assert.Equal(t, 't', c.Ch)
			},
			term.Coordinates{X: 9, Y: 2},
		},
		{
			"MoveLeftStartWord should move to the start of the next word",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 2
				e.cursor.X = 9
				e.MoveRightStartWord()

				assert.True(t, e.MoveLeftStartWord())

				c, _ := e.scroll.Buffer().Cell(e.cursorAtScroll())
				assert.Equal(t, 'i', c.Ch)
			},
			term.Coordinates{X: 6, Y: 2},
		},
		{
			"MoveRightEndWord should move to the end of the current word",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 2
				e.cursor.X = 9

				e.MoveRightStartWord()
				e.MoveLeftStartWord()

				assert.True(t, e.MoveRightEndWord())

				c, _ := e.scroll.Buffer().Cell(e.cursorAtScroll())
				assert.Equal(t, 'f', c.Ch)
			},
			term.Coordinates{X: 7, Y: 2},
		},
		{
			"MoveToMatchingRune should do nothing if rune is not {,[,(,},],)",
			10, 10,
			func(t *testing.T, e *Cursor) {
				assert.False(t, e.MoveToMatchingRune())
			},
			term.Coordinates{X: 0, Y: 0},
		},
		{
			"MoveToMatchingRune should move to the 'matching rune'",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 7
				assert.True(t, e.MoveToMatchingRune())

				c, _ := e.scroll.Buffer().Cell(e.cursorAtScroll())
				assert.Equal(t, '}', c.Ch)
			},
			term.Coordinates{X: 0, Y: 9},
		},
		{
			"MoveToMatchingRune should return false if current matching rune is not found",
			1000, 1000,
			func(t *testing.T, e *Cursor) {
				e.cursor.Y = 31
				e.cursor.X = 5
				e.scroll.SeekEndFile()
				assert.False(t, e.MoveToMatchingRune())

				c, _ := e.scroll.Buffer().Cell(e.cursorAtScroll())
				assert.Equal(t, '{', c.Ch)
			},
			term.Coordinates{X: 5, Y: 31},
		},
		{
			"MoveToNextChar should do nothing if there is no matches in the line",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.MoveDown()
				e.MoveRight()
				assert.False(t, e.MoveToNextChar('a'))
			},
			term.Coordinates{X: 1, Y: 1},
		},
		{
			"MoveToNextChar should do nothing if are only matches before cursor",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.MoveDown()
				e.MoveRight()
				assert.False(t, e.MoveToNextChar('/'))
			},
			term.Coordinates{X: 1, Y: 1},
		},
		{
			"MoveToNextChar should move the cursor to a matching character",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.MoveDown()
				e.MoveDown()
				assert.True(t, e.MoveToNextChar('o'))

				c, _ := e.scroll.Buffer().Cell(e.cursorAtScroll())
				assert.Equal(t, 'o', c.Ch)
			},
			term.Coordinates{X: 9, Y: 2},
		},
		{
			"MoveToPrevChar should do nothing if there is no matches in the line",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.MoveDown()
				e.MoveRight()
				assert.False(t, e.MoveToPrevChar('a'))
			},
			term.Coordinates{X: 1, Y: 1},
		},
		{
			"MoveToPrevChar should do nothing if are only matches after cursor",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.MoveDown()
				assert.False(t, e.MoveToPrevChar('/'))
			},
			term.Coordinates{X: 0, Y: 1},
		},
		{
			"MoveToPrevChar should move the cursor to a matching character",
			10, 10,
			func(t *testing.T, e *Cursor) {
				e.MoveDown()
				e.MoveDown()
				e.MoveEndLine()
				assert.True(t, e.MoveToPrevChar('C'))

				c, _ := e.scroll.Buffer().Cell(e.cursorAtScroll())
				assert.Equal(t, 'C', c.Ch)
			},
			term.Coordinates{X: 0, Y: 2},
		},
	}

	for _, _tcase := range tsuite {
		tcase := _tcase

		t.Run(tcase.desc, func(t *testing.T) {
			e := setupCursor(t, tcase.width, tcase.height)

			tcase.sut(t, e)

			cursor, _ := e.Cursor()
			assert.Equal(t, tcase.cursor, cursor)
		})
	}
}

func TestCursorInsertRow(t *testing.T) {
	e := setupCursor(t, 10, 10)

	e.InsertRowAbove()
	assert.Equal(t, 0, len(e.scroll.Buffer().RawCells()[0]))

	e.scroll.SeekEndLine()
	e.cursor.Y = 2
	e.cursor.X = 9

	e.InsertRowAbove()
	assert.Equal(t, 0, len(e.scroll.Buffer().RawCells()[2]))

	e.MoveLastLine()

	r := e.scroll.Buffer().Rows()
	e.InsertRowBelow()

	assert.Equal(t, r+1, e.scroll.Buffer().Rows())
}

func TestCursorInsertDelete(t *testing.T) {
	e := setupCursor(t, 10, 10)
	str := e.scroll.Buffer().String()

	e.Insert('p')
	e.Insert('a')
	e.Insert('c')
	e.Insert('k')

	for i := 0; i < 4; i++ {
		e.MoveLeft()
		e.Delete()
	}

	assert.Equal(t, str, e.scroll.Buffer().String())
}

func TestCursorBackspace(t *testing.T) {
	e := setupCursor(t, 10, 10)
	n := len(e.scroll.Buffer().String())

	e.MoveLastLine()
	e.MoveEndLine()
	e.MoveRight()

	for i := 0; i < n; i++ {
		e.Backspace()
	}

	assert.Equal(t, "", e.scroll.Buffer().String())
}

func TestCursorConflate(t *testing.T) {
	e := setupCursor(t, 10, 10)
	n := strings.Count(e.scroll.Buffer().String(), "\n")
	rows := e.scroll.Buffer().Rows()
	require.Equal(t, n+1, rows)

	for i := 0; i < n; i++ {
		e.Conflate()
	}

	assert.Equal(t, 1, e.scroll.Buffer().Rows())
	assert.Zero(t, strings.Count(e.scroll.Buffer().String(), "\n"))
}

func testCursorSelect(t *testing.T, width, height int) {
	makeSelect := func(t *testing.T) *Cursor {
		e := setupCursor(t, width, height)
		str := e.scroll.Buffer().String()
		assertBufferAttributes(t, e.buffer(), term.Attributes{})
		assert.False(t, e.Unselect())
		require.True(t, e.Select())

		for e.MoveDown() {
		}
		for e.MoveRight() {
		}
		assert.Equal(t, str, e.Selection())
		return e
	}

	makeSelectLine := func(t *testing.T) *Cursor {
		e := setupCursor(t, width, height)
		str := e.scroll.Buffer().String()
		assertBufferAttributes(t, e.buffer(), term.Attributes{})
		require.True(t, e.SelectLine())
		require.True(t, e.MoveLastLine())
		assert.Equal(t, str, e.Selection())
		return e
	}

	makeSelectBlock := func(t *testing.T) *Cursor {
		e := setupCursor(t, width, height)
		assertBufferAttributes(t, e.buffer(), term.Attributes{})
		require.True(t, e.SelectBlock())
		require.True(t, e.MoveLastLine())
		require.True(t, e.MoveEndLine())
		return e
	}

	t.Run("Select then Unselect should reverse all attributes", func(t *testing.T) {
		e := makeSelect(t)
		assert.True(t, e.Unselect())
		assertBufferAttributes(t, e.buffer(), term.Attributes{})
	})

	t.Run("SelectLine then Unselect should reverse all attributes", func(t *testing.T) {
		e := makeSelectLine(t)
		assert.True(t, e.Unselect())
		assertBufferAttributes(t, e.buffer(), term.Attributes{})
	})

	t.Run("SelectBlock then Unselect should reverse all attributes", func(t *testing.T) {
		e := makeSelectBlock(t)
		assert.True(t, e.Unselect())
		assertBufferAttributes(t, e.buffer(), term.Attributes{})
	})

	t.Run("Select selects from start to end", func(t *testing.T) {
		e := makeSelect(t)
		require.True(t, e.DeleteSelection())
		assert.Equal(t, "", e.Selection())
		assertBufferAttributes(t, e.buffer(), term.Attributes{})
	})

	t.Run("SelectLine selects from start line to end line", func(t *testing.T) {
		e := makeSelectLine(t)
		require.True(t, e.DeleteSelection())
		assertBufferAttributes(t, e.buffer(), term.Attributes{})
	})

	t.Run("SelectBlock selects from start to end in block", func(t *testing.T) {
		e := makeSelectBlock(t)
		require.True(t, e.DeleteSelection())
		assertBufferAttributes(t, e.buffer(), term.Attributes{})
	})
}

func assertBufferAttributes(t *testing.T, b *cell.Buffer, attr term.Attributes) {
	for y, row := range b.RawCells() {
		for x, c := range row {
			assert.Equal(t, attr.Bg, c.Bg, "at y=%d;x=%d", y, x)
			assert.Equal(t, attr.Fg, c.Fg, "at y=%d;x=%d", y, x)
		}
	}
}

func TestCursorSelect10(t *testing.T) {
	testCursorSelect(t, 10, 100)
}

func TestCursorSelect20(t *testing.T) {
	testCursorSelect(t, 20, 100)
}

func TestCursorSelect50(t *testing.T) {
	testCursorSelect(t, 50, 100)
}

func TestCursorSelect100(t *testing.T) {
	testCursorSelect(t, 100, 100)
}

func testCursorUndoRedo(t *testing.T, moveBefore, moveAfter func(c *Cursor) bool, width, height int) {
	const input = "Aleda"
	e := setupCursor(t, width, height)
	str := e.scroll.Buffer().String()

	moveBefore(e)
	cBefore, ok := e.Cursor()
	require.True(t, ok)

	e.InsertString(input)
	str2 := e.scroll.Buffer().String()

	require.True(t, e.Undo())
	require.False(t, e.Undo())

	c, ok := e.Cursor()
	require.True(t, ok)
	assert.Equal(t, cBefore, c)
	assert.Equal(t, str, e.scroll.Buffer().String())

	moveAfter(e)

	require.True(t, e.Redo())
	assert.Equal(t, str2, e.scroll.Buffer().String())
	require.False(t, e.Redo())

	require.True(t, e.Undo())
	require.False(t, e.Undo())

	c, ok = e.Cursor()
	require.True(t, ok)
	assert.Equal(t, cBefore, c)
	assert.Equal(t, str, e.scroll.Buffer().String())
}

func TestCursorUndoRedo10(t *testing.T) {
	testCursorUndoRedo(t, (*Cursor).MoveFirstLine, (*Cursor).MoveLastLine, 10, 10)
}
func TestCursorUndoRedo20(t *testing.T) {
	testCursorUndoRedo(t, (*Cursor).MoveFirstLine, (*Cursor).MoveLastLine, 20, 20)
}
func TestCursorUndoRedo100(t *testing.T) {
	testCursorUndoRedo(t, (*Cursor).MoveFirstLine, (*Cursor).MoveLastLine, 100, 100)
}

func TestCursorUndoRedo10Backwards(t *testing.T) {
	testCursorUndoRedo(t, (*Cursor).MoveLastLine, (*Cursor).MoveFirstLine, 10, 10)
}
func TestCursorUndoRedo20Backwards(t *testing.T) {
	testCursorUndoRedo(t, (*Cursor).MoveLastLine, (*Cursor).MoveFirstLine, 20, 20)
}
func TestCursorUndoRedo100Backwards(t *testing.T) {
	testCursorUndoRedo(t, (*Cursor).MoveLastLine, (*Cursor).MoveFirstLine, 100, 100)
}

func testCursorDeleteSelection(t *testing.T, width, height int, typeSelect int) {
	tsuite := []struct {
		initialBuf  string
		initialPos  func(*Cursor)
		selected    bool
		finalPos    func(*Cursor)
		deleted     bool
		finalBuf    string
		skipForMode []int
	}{
		{
			initialBuf: "",
			selected:   false,
			finalPos:   func(*Cursor) {},
			deleted:    false,
			finalBuf:   "",
		},
		{
			initialBuf: "a",
			selected:   true,
			finalPos:   func(*Cursor) {},
			deleted:    true,
			finalBuf:   "",
		},
		{
			initialBuf:  "a\nb",
			selected:    true,
			finalPos:    func(c *Cursor) { c.MoveRight() },
			deleted:     true,
			finalBuf:    "b",
			skipForMode: []int{lineSelection, blockSelection},
		},
		{
			initialBuf: "a\nb",
			selected:   true,
			finalPos: func(c *Cursor) {
				for i := 0; i < 3; i++ {
					c.MoveRight()
				}
			},
			finalBuf:    "b",
			deleted:     true,
			skipForMode: []int{lineSelection, blockSelection},
		},
		{
			initialBuf: "a\nb",
			initialPos: func(c *Cursor) {
				for i := 0; i < 3; i++ {
					c.MoveRight()
				}
			},
			finalPos:    func(*Cursor) {},
			selected:    true,
			deleted:     true,
			finalBuf:    "ab",
			skipForMode: []int{lineSelection, blockSelection},
		},
		{
			initialBuf: "a\nb",
			selected:   true,
			finalPos: func(c *Cursor) {
				for i := 0; i < 100; i++ {
					c.MoveDown()
				}
			},
			deleted: true,
		},
		{
			initialBuf: "a\nb",
			initialPos: func(c *Cursor) {
				for i := 0; i < 100; i++ {
					c.MoveDown()
				}
			},
			finalPos:    func(*Cursor) {},
			selected:    true,
			deleted:     true,
			finalBuf:    "a\nb",
			skipForMode: []int{lineSelection},
		},
		{
			initialBuf: "a\nb",
			selected:   true,
			finalPos: func(c *Cursor) {
				c.MoveLastLine()
				c.MoveEndLine()
			},
			deleted:  true,
			finalBuf: "",
		},
		{
			initialBuf: "type Writer {\n\ta int\n\tb int\n}\n",
			selected:   true,
			initialPos: func(c *Cursor) {
				c.MoveLastLine()
			},
			finalPos: func(c *Cursor) {
				c.MoveFirstLine()
			},
			deleted:     true,
			finalBuf:    "",
			skipForMode: []int{standardSelection, blockSelection},
		},
	}

	for _, tcase := range tsuite {
		var skip bool
		for _, mode := range tcase.skipForMode {
			if mode == typeSelect {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		c := setupCursorContent(t, width, height, tcase.initialBuf)
		if tcase.initialPos != nil {
			tcase.initialPos(c)
		}
		switch typeSelect {
		case noSelection:
			panic("hmm...")
		case blockSelection:
			require.Equal(t, tcase.selected, c.SelectBlock())
		case lineSelection:
			require.Equal(t, tcase.selected, c.SelectLine())
		case standardSelection:
			require.Equal(t, tcase.selected, c.Select())
		}
		if !tcase.selected {
			continue
		}
		tcase.finalPos(c)
		require.Equal(t, tcase.deleted, c.DeleteSelection())
		if !tcase.deleted {
			continue
		}
		assert.Equal(t, tcase.finalBuf, c.scroll.Buffer().String())
	}
}

func TestCursorDeleteSelection10(t *testing.T) {
	testCursorDeleteSelection(t, 10, 10, standardSelection)
}
func TestCursorDeleteSelection20(t *testing.T) {
	testCursorDeleteSelection(t, 20, 20, standardSelection)
}
func TestCursorDeleteSelection1000(t *testing.T) {
	testCursorDeleteSelection(t, 1000, 1000, standardSelection)
}
func TestCursorDeleteSelectionLine10(t *testing.T) {
	testCursorDeleteSelection(t, 10, 10, lineSelection)
}
func TestCursorDeleteSelectionLine20(t *testing.T) {
	testCursorDeleteSelection(t, 20, 20, lineSelection)
}
func TestCursorDeleteSelectionLine1000(t *testing.T) {
	testCursorDeleteSelection(t, 1000, 1000, lineSelection)
}
func TestCursorDeleteSelectionBlock10(t *testing.T) {
	testCursorDeleteSelection(t, 10, 10, blockSelection)
}
func TestCursorDeleteSelectionBlock20(t *testing.T) {
	testCursorDeleteSelection(t, 20, 20, blockSelection)
}
func TestCursorDeleteSelectionBlock1000(t *testing.T) {
	testCursorDeleteSelection(t, 1000, 1000, blockSelection)
}

func TestCursorMoveToBounds(t *testing.T) {
	e := setupCursor(t, 100, 100)

	pos, ok := e.Cursor()
	require.True(t, ok)
	assert.Equal(t, term.Coordinates{}, pos)

	assert.True(t, e.MoveRight())
	assert.True(t, e.MoveRight())
	assert.True(t, e.MoveRight())

	e.MoveToBounds(2)

	pos, ok = e.Cursor()
	require.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 1}, pos)

	pos, _ = e.Cursor()
	assert.True(t, e.MoveDown())

	e.MoveEndLine()
	e.MoveRight()
	e.MoveRight()

	e.MoveToBounds(1)

	pos, _ = e.Cursor()
	assert.Equal(t, term.Coordinates{X: 2, Y: 1}, pos)

	e.MoveToBounds(0)

	pos, _ = e.Cursor()
	assert.Equal(t, term.Coordinates{X: 1, Y: 1}, pos)

	e.MoveLastLine()
	e.MoveDown()

	e.MoveToBounds(0)

	pos, _ = e.Cursor()
	assert.Equal(t, term.Coordinates{X: 0, Y: 31}, pos)
}

func TestCursorSkipNulls(t *testing.T) {
	e := setupCursor(t, 100, 100)

	assert.True(t, e.MoveRight())

	e.MoveToNextNonNull()

	pos, ok := e.Cursor()
	require.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 0}, pos)

	e.MoveLastLine()
	e.MoveDown()

	e.MoveToNextNonNull()

	pos, _ = e.Cursor()
	assert.Equal(t, term.Coordinates{X: 0, Y: 32}, pos)

	e.MoveFirstLine()
	for i := 0; i < 16; i++ {
		e.MoveDown()
	}
	e.MoveRight()

	e.MoveToNextNonNull()

	pos, _ = e.Cursor()
	assert.Equal(t, term.Coordinates{X: 3, Y: 16}, pos)

	e.MoveRight()
	e.MoveToNextNonNull()

	pos, _ = e.Cursor()
	assert.Equal(t, term.Coordinates{X: 7, Y: 16}, pos)
}

func TestCursorShiftLine(t *testing.T) {
	c := setupCursorContent(t, 10, 1, " blabla\nbleble")
	assert.True(t, c.ShiftLineLeft())
	assert.False(t, c.ShiftLineLeft())
	assert.Equal(t, term.Coordinates{}, c.cursor)

	c.ShiftLineRight()
	assert.Equal(t, term.Coordinates{X: 4}, c.cursor)
	c.ShiftLineRight()
	assert.Equal(t, term.Coordinates{X: 8}, c.cursor)
	assert.True(t, c.ShiftLineLeft())
	assert.Equal(t, term.Coordinates{X: 4}, c.cursor)
	assert.True(t, c.ShiftLineLeft())
	assert.Equal(t, term.Coordinates{}, c.cursor)
	assert.True(t, c.MoveDown())
	c.ShiftLineRight()
	assert.Equal(t, term.Coordinates{Y: 0, X: 4}, c.cursor)
	assert.True(t, c.ShiftLineLeft())
	assert.Equal(t, term.Coordinates{Y: 0, X: 0}, c.cursor)
}

func TestCursorShiftSelection(t *testing.T) {
	c := setupCursorContent(t, 10, 10, " blabla\nbleble")
	require.True(t, c.Select())
	require.True(t, c.MoveDown())

	c.ShiftSelectionRight()
	assert.Equal(t, "\t blabla\n\tbleble", c.scroll.Buffer().String())

	require.True(t, c.SelectBlock())
	require.True(t, c.MoveUp())
	assert.True(t, c.ShiftSelectionLeft())
	assert.Equal(t, " blabla\nbleble", c.scroll.Buffer().String())
}

var (
	abcAttr      = term.Attributes{Fg: term.AttrUnderline, Bg: term.ColorBlack}
	abcLocations = []Location{
		{
			From:    term.Coordinates{Y: 1},
			To:      term.Coordinates{Y: 1},
			Attr:    abcAttr,
			Message: "blabla",
		},
		{
			From: term.Coordinates{Y: 2},
			To:   term.Coordinates{Y: 2},
			Attr: abcAttr,
		},
		{
			From: term.Coordinates{Y: 3},
			To:   term.Coordinates{Y: 3},
			Attr: abcAttr,
		},
	}
)

func TestCursorMoveLocationList(t *testing.T) {
	t.Run("MoveToPrevLocation should return false and do nothing if location list is nil", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "")
		assert.False(t, c.MoveToPrevLocation(locID))
	})

	t.Run("MoveToNextLocation should return false and do nothing if location list is nil", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "")
		assert.False(t, c.MoveToNextLocation(locID))
	})

	t.Run("MoveToPrevLocation should return false and do nothing if already at start of location list", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "")
		locations := []Location{Location{}}
		assert.Nil(t, c.SetLocationList(locID, &testLocationList{locations: locations}))
		assert.False(t, c.MoveToPrevLocation(locID))
	})

	t.Run("MoveToNextLocation should return false and do nothing if already at end of location list", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "")
		locations := []Location{Location{}}
		assert.Nil(t, c.SetLocationList(locID, &testLocationList{locations: locations}))
		assert.False(t, c.MoveToNextLocation(locID))
	})

	t.Run("MoveToPrevLocation should return true and move cursor to earlier location", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, " X")
		require.True(t, c.MoveRight())

		locations := []Location{Location{}}
		assert.Nil(t, c.SetLocationList(locID, &testLocationList{locations: locations}))
		assert.True(t, c.MoveToNextLocation(locID))

		pos, ok := c.Cursor()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{}, pos)
	})

	t.Run("MoveToNextLocation should wrap around to first location", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "\na\nb\nc \n")
		require.True(t, c.MoveLastLine())
		require.True(t, c.MoveEndLine())

		assert.Nil(t, c.SetLocationList(locID, &testLocationList{locations: abcLocations}))
		assert.True(t, c.MoveToNextLocation(locID))

		pos, ok := c.Cursor()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{Y: 1}, pos)
	})

	t.Run("MoveToPrevLocation should wrap around to last location", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "\na\nb\nc\n")

		assert.Nil(t, c.SetLocationList(locID, &testLocationList{locations: abcLocations}))
		assert.True(t, c.MoveToPrevLocation(locID))

		pos, ok := c.Cursor()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{Y: 3}, pos)
	})

	t.Run("MoveToNextLocation should wrap around to first location (special case)", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, " X")
		require.True(t, c.MoveRight())

		locations := []Location{Location{}, Location{From: term.Coordinates{X: 1}}}
		assert.Nil(t, c.SetLocationList(locID, &testLocationList{locations: locations}))
		assert.True(t, c.MoveToNextLocation(locID))

		pos, ok := c.Cursor()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{}, pos)
	})

	t.Run("MoveToPrevLocation should wrap around to last location (special case)", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, " X")

		locations := []Location{Location{}, Location{From: term.Coordinates{X: 1}}}
		assert.Nil(t, c.SetLocationList(locID, &testLocationList{locations: locations}))
		assert.True(t, c.MoveToPrevLocation(locID))

		pos, ok := c.Cursor()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 1}, pos)
	})

	t.Run("MoveToNextLocation should go to next location after cursor", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "\na\nb\nc \n")
		require.True(t, c.MoveDown())
		require.True(t, c.MoveDown())

		assert.Nil(t, c.SetLocationList(locID, &testLocationList{locations: abcLocations}))
		assert.True(t, c.MoveToNextLocation(locID))

		pos, ok := c.Cursor()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{Y: 3}, pos)
	})

	t.Run("MoveToPrevLocation should go to prev location before location", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, "\na\nb\nc\n")
		require.True(t, c.MoveDown())
		require.True(t, c.MoveDown())

		assert.Nil(t, c.SetLocationList(locID, &testLocationList{locations: abcLocations}))
		assert.True(t, c.MoveToPrevLocation(locID))

		pos, ok := c.Cursor()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{Y: 1}, pos)
	})
}

func TestCursorSetLocationListAttr(t *testing.T) {
	c := setupCursorContent(t, 10, 10, "\na\nb\nc\n")
	buf := c.scroll.Buffer()

	expected := [][]term.Cell{
		{},
		{{Ch: 'a', Bg: abcAttr.Bg, Fg: abcAttr.Fg}},
		{{Ch: 'b', Bg: abcAttr.Bg, Fg: abcAttr.Fg}},
		{{Ch: 'c', Bg: abcAttr.Bg, Fg: abcAttr.Fg}},
	}

	abcList := &testLocationList{locations: abcLocations}

	assert.Nil(t, c.SetLocationList(locID, abcList))
	assert.Equal(t, expected, buf.RawCells())

	buf.RawCells()[2][0].Bg = term.AttrUnderline
	buf.RawCells()[2][0].Fg = term.AttrBold

	newLocations := []Location{
		{
			From: term.Coordinates{Y: 2},
			To:   term.Coordinates{Y: 2},
			Attr: term.Attributes{Bg: term.ColorRed},
		},
	}

	expected = [][]term.Cell{
		{},
		{{Ch: 'a'}},
		{{Ch: 'b',
			Fg: term.AttrBold, Bg: term.AttrUnderline | term.ColorRed}},
		{{Ch: 'c'}},
	}
	oldLocList := c.SetLocationList(locID, &testLocationList{locations: newLocations})
	assert.Equal(t, abcList, oldLocList)
	assert.Equal(t, expected, buf.RawCells())

	require.True(t, buf.DeleteRow(0))

	// make sure it doesn't remove the wrong one
	buf.RawCells()[0][0].Bg = term.ColorRed
	newLocations = []Location{
		{
			From: term.Coordinates{Y: 2},
			To:   term.Coordinates{Y: 2},
			Attr: term.Attributes{Bg: term.ColorRed},
		},
	}
	expected = [][]term.Cell{
		{{Ch: 'a', Bg: term.ColorRed}},
		{{Ch: 'b', Fg: term.AttrBold, Bg: term.AttrUnderline}},
		{{Ch: 'c', Bg: term.ColorRed}},
	}
	newL := c.SetLocationList(locID, &testLocationList{locations: newLocations})
	assert.Equal(t, expected, buf.RawCells())
	assert.Nil(t, newL)

	// clear location list
	c.SetLocationList(locID, nil)
	expected = [][]term.Cell{
		{{Ch: 'a', Bg: term.ColorRed}},
		{{Ch: 'b', Fg: term.AttrBold, Bg: term.AttrUnderline}},
		{{Ch: 'c'}},
	}
	assert.Equal(t, expected, buf.RawCells())
}

func TestCursorSetLocationListMessages(t *testing.T) {
	content := "\naaa\nbbb\nccc\n"
	messageLocations := []Location{
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
			locsByID, ok := c.Locations()
			require.True(t, ok, c.messages)
			require.Len(t, locsByID, 1)
			assert.Equal(t, locsByID[locID].Message, strconv.Itoa(i+1))
			require.True(t, c.MoveDown())
		}
	}

	t.Run("returns nil/false if cursor is not in from, to or in between", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, content)
		abcList := &testLocationList{locations: messageLocations}
		assert.Nil(t, c.SetLocationList(locID, abcList))

		_, ok := c.Locations()
		assert.False(t, ok)
	})

	t.Run("return messages if cursor is at From", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, content)
		abcList := &testLocationList{locations: messageLocations}
		assert.Nil(t, c.SetLocationList(locID, abcList))
		require.True(t, c.MoveDown())

		assertMessages(t, c)
	})

	t.Run("return messages if cursor between From/To", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, content)

		abcList := &testLocationList{locations: messageLocations}

		assert.Nil(t, c.SetLocationList(locID, abcList))
		require.True(t, c.MoveDown())
		require.True(t, c.MoveRight())

		assertMessages(t, c)
	})

	t.Run("return messages if cursor is at To", func(t *testing.T) {
		c := setupCursorContent(t, 10, 10, content)

		abcList := &testLocationList{locations: messageLocations}

		assert.Nil(t, c.SetLocationList(locID, abcList))
		require.True(t, c.MoveDown())
		c.MoveRight()
		c.MoveRight()

		assertMessages(t, c)
	})
}
