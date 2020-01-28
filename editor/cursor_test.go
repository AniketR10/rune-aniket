package editor

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func setupCursorContent(t *testing.T, width, height int, cont string) (e Cursor) {
	scroll := component.NewScroll()
	e.Init(scroll)
	_, err := scroll.ReadFrom(strings.NewReader(cont))
	require.NoError(t, err)
	scroll.Resize(width, height)

	return
}

func setupCursor(t *testing.T, width, height int) Cursor {
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
				assert.False(t, e.MoveNextSearchResult())
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
			"MoveNextSearchResult does nothing if only one result is found",
			1000, 1000,
			1,
			"else",
			func(t *testing.T, e *Cursor) {
				assert.True(t, e.MoveNextSearchResult())
			}, term.Coordinates{X: 4, Y: 29},
		},
		{
			"Seeks to first result if not in window",
			100, 10,
			1,
			"When",
			nil,
			term.Coordinates{X: 7, Y: 0},
		},
		{
			"Seeks to last result upon MovePrevSearchResult",
			1000, 1000,
			2,
			"NULL",
			func(t *testing.T, e *Cursor) {
				assert.True(t, e.MovePrevSearchResult())
			}, term.Coordinates{X: 32, Y: 23},
		},
		{
			"Seeks if MoveNextSearchResult result is not in window",
			100, 10,
			2,
			"NULL",
			func(t *testing.T, e *Cursor) {
				assert.True(t, e.MoveNextSearchResult())
			}, term.Coordinates{X: 32, Y: 1},
		},
	}

	for _, _tcase := range tsuite {
		tcase := _tcase

		t.Run(tcase.desc, func(t *testing.T) {
			e := setupCursor(t, tcase.width, tcase.height)

			require.Equal(t, tcase.results, e.Search(tcase.searchstring))
			if tcase.assertions != nil {
				tcase.assertions(t, &e)
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

				c, _ := e.scroll.Cell(e.cursorAtScroll())
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

				c, _ := e.scroll.Cell(e.cursorAtScroll())
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

				c, _ := e.scroll.Cell(e.cursorAtScroll())
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

				c, _ := e.scroll.Cell(e.cursorAtScroll())
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

				c, _ := e.scroll.Cell(e.cursorAtScroll())
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

				c, _ := e.scroll.Cell(e.cursorAtScroll())
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

				c, _ := e.scroll.Cell(e.cursorAtScroll())
				assert.Equal(t, '{', c.Ch)
			},
			term.Coordinates{X: 5, Y: 31},
		},
	}

	for _, _tcase := range tsuite {
		tcase := _tcase

		t.Run(tcase.desc, func(t *testing.T) {
			e := setupCursor(t, tcase.width, tcase.height)

			tcase.sut(t, &e)

			cursor, _ := e.Cursor()
			assert.Equal(t, tcase.cursor, cursor)
		})
	}
}

func TestCursorInsertRow(t *testing.T) {
	e := setupCursor(t, 10, 10)

	e.InsertRowAbove()
	assert.Equal(t, 0, len(e.scroll.RawCells()[0]))

	e.scroll.SeekEndLine()
	e.cursor.Y = 2
	e.cursor.X = 9

	e.InsertRowAbove()
	assert.Equal(t, 0, len(e.scroll.RawCells()[2]))

	e.MoveLastLine()

	r := e.scroll.Rows()
	e.InsertRowBelow()

	assert.Equal(t, r+1, e.scroll.Rows())
}

func TestCursorInsertDelete(t *testing.T) {
	e := setupCursor(t, 10, 10)
	str := e.scroll.String()

	e.Insert('p')
	e.Insert('a')
	e.Insert('c')
	e.Insert('k')

	for i := 0; i < 4; i++ {
		e.MoveLeft()
		e.Delete()
	}

	assert.Equal(t, str, e.scroll.String())
}

func TestCursorBackspace(t *testing.T) {
	e := setupCursor(t, 10, 10)
	n := len(e.scroll.String())

	e.MoveLastLine()
	e.MoveEndLine()
	e.MoveRight()

	for i := 0; i < n; i++ {
		e.Backspace()
	}

	assert.Equal(t, "", e.scroll.String())
}

func TestCursorConflate(t *testing.T) {
	e := setupCursor(t, 10, 10)
	n := strings.Count(e.scroll.String(), "\n")
	rows := e.scroll.Rows()
	require.Equal(t, n+1, rows)

	for i := 0; i < n; i++ {
		e.Conflate()
	}

	assert.Equal(t, 1, e.scroll.Rows())
	assert.Zero(t, strings.Count(e.scroll.String(), "\n"))
}

func testCursorSelect(t *testing.T, width, height int) {
	t.Run("Select", func(t *testing.T) {
		e := setupCursor(t, width, height)
		str := e.scroll.String()
		e.Unselect()
		require.True(t, e.Select())

		e.MoveLastLine()
		e.MoveEndLine()
		assert.Equal(t, str, e.Selection())

		require.True(t, e.DeleteSelection())
		assert.Equal(t, "", e.Selection())
	})

	t.Run("SelectLine", func(t *testing.T) {
		e := setupCursor(t, width, height)
		str := e.scroll.String()
		require.True(t, e.SelectLine())
		for e.MoveDown() {
		}
		assert.Equal(t, str, e.Selection())
		assert.False(t, e.DeleteSelection())
	})

	t.Run("SelectBlock", func(t *testing.T) {
		e := setupCursor(t, width, height)
		str := e.scroll.String()
		require.True(t, e.SelectBlock())
		e.MoveLastLine()
		for e.MoveRight() {
		}
		assert.Equal(t, str, e.Selection())
		assert.False(t, e.DeleteSelection())
	})
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
	str := e.scroll.String()

	moveBefore(&e)
	cBefore, ok := e.Cursor()
	require.True(t, ok)

	for _, c := range input {
		e.Insert(c)
	}
	str2 := e.scroll.String()

	for range input {
		require.True(t, e.Undo())
	}
	require.False(t, e.Undo())

	c, ok := e.Cursor()
	require.True(t, ok)
	assert.Equal(t, cBefore, c)
	assert.Equal(t, str, e.scroll.String())

	moveAfter(&e)

	for range input {
		require.True(t, e.Redo())
	}
	assert.Equal(t, str2, e.scroll.String())
	require.False(t, e.Redo())

	for range input {
		require.True(t, e.Undo())
	}
	require.False(t, e.Undo())

	c, ok = e.Cursor()
	require.True(t, ok)
	assert.Equal(t, cBefore, c)
	assert.Equal(t, str, e.scroll.String())
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

func testCursorDeleteSelection(t *testing.T, width, height int) {
	tsuite := []struct {
		initialBuf string
		initialPos func(*Cursor)
		selected   bool
		finalPos   func(*Cursor)
		deleted    bool
		finalBuf   string
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
			initialBuf: "a\nb",
			selected:   true,
			finalPos:   func(c *Cursor) { c.MoveRight() },
			deleted:    true,
			finalBuf:   "b",
		},
		{
			initialBuf: "a\nb",
			selected:   true,
			finalPos: func(c *Cursor) {
				for i := 0; i < 3; i++ {
					c.MoveRight()
				}
			},
			deleted: false,
		},
		{
			initialBuf: "a\nb",
			initialPos: func(c *Cursor) {
				for i := 0; i < 3; i++ {
					c.MoveRight()
				}
			},
			selected: false,
		},
		{
			initialBuf: "a\nb",
			selected:   true,
			finalPos: func(c *Cursor) {
				for i := 0; i < 100; i++ {
					c.MoveDown()
				}
			},
			deleted: false,
		},
		{
			initialBuf: "a\nb",
			initialPos: func(c *Cursor) {
				for i := 0; i < 100; i++ {
					c.MoveDown()
				}
			},
			selected: false,
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
	}

	for _, tcase := range tsuite {
		c := setupCursorContent(t, width, height, tcase.initialBuf)
		if tcase.initialPos != nil {
			tcase.initialPos(&c)
		}
		require.Equal(t, tcase.selected, c.Select())
		if !tcase.selected {
			continue
		}
		tcase.finalPos(&c)
		require.Equal(t, tcase.deleted, c.DeleteSelection())
		if !tcase.deleted {
			continue
		}
		assert.Equal(t, tcase.finalBuf, c.scroll.String())
	}
}
func TestCursorDeleteSelection10(t *testing.T) {
	testCursorDeleteSelection(t, 10, 10)
}
func TestCursorDeleteSelection20(t *testing.T) {
	testCursorDeleteSelection(t, 20, 20)
}
func TestCursorDeleteSelection1000(t *testing.T) {
	testCursorDeleteSelection(t, 1000, 1000)
}
