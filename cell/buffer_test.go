package cell

import (
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/term"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const str = "hello\n\tworld\n"
const longStr = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`

func newBufferWithContent(t *testing.T, str string) *Buffer {
	b := NewBuffer()
	_, err := b.ReadFrom(strings.NewReader(str))
	require.NoError(t, err)

	return b
}

func TestBufferInsert(t *testing.T) {
	buf := NewBuffer()
	var next term.Coordinates

	next = buf.Insert(next, 'h')
	next = buf.Insert(next, 'e')
	next = buf.Insert(next, 'l')
	next = buf.Insert(next, 'l')
	next = buf.Insert(next, 'o')
	next = buf.Insert(next, '\n')
	next = buf.Insert(next, 'w')
	next = buf.Insert(next, 'o')
	next = buf.Insert(next, 'r')
	next = buf.Insert(next, 'l')
	buf.Insert(next, 'd')
	next = buf.Insert(term.Coordinates{X: 4, Y: 0}, '\n')

	str := "hell\no\nworld"
	assert.Equal(t, 3, buf.Rows())

	cols := buf.Columns(0)
	assert.Equal(t, 4, cols)

	cols = buf.Columns(1)
	assert.Equal(t, 1, cols)

	cols = buf.Columns(2)
	assert.Equal(t, 5, cols)

	assert.Equal(t, str, buf.String())

	assert.Equal(t, term.Coordinates{Y: 1}, next)

	next = buf.Insert(term.Coordinates{Y: 1}, '\t')
	assert.Equal(t, term.Coordinates{Y: 1, X: 4}, next)
}

func TestBufferDeleteRow(t *testing.T) {
	str := "hello\nworld"
	buf := newBufferWithContent(t, str)

	assert.True(t, buf.DeleteRow(0))
	assert.Equal(t, "world", buf.String())

	assert.True(t, buf.DeleteRow(0))
	assert.Equal(t, "", buf.String())
}

func TestBufferDeleteRow2(t *testing.T) {
	str := "\nworld"
	buf := newBufferWithContent(t, str)

	assert.True(t, buf.DeleteRow(0))
	assert.Equal(t, "world", buf.String())
}

func TestBufferDeleteRowNotPanic(t *testing.T) {
	str := "1234"
	buf := newBufferWithContent(t, str)

	assert.False(t, buf.DeleteRow(1))
	assert.Equal(t, "1234", buf.String())
}

func TestBufferTruncateRowFrom1(t *testing.T) {
	str := "hello\nworld"
	buf := newBufferWithContent(t, str)

	buf.TruncateRowFrom(term.Coordinates{X: 2, Y: 0})

	assert.Equal(t, "he\nworld", buf.String())
}

func TestBufferTruncateRowFrom2(t *testing.T) {
	str := "hello\nworld"
	buf := newBufferWithContent(t, str)

	buf.TruncateRowFrom(term.Coordinates{X: 2, Y: 1})

	assert.Equal(t, "hello\nwo", buf.String())
}

func TestBufferDeleteCell(t *testing.T) {
	str := "hello\nworld"
	buf := newBufferWithContent(t, str)

	_, r, ok := buf.DeleteCell(term.Coordinates{X: 100, Y: 0})
	require.False(t, ok)

	_, r, ok = buf.DeleteCell(term.Coordinates{X: 0, Y: 100})
	require.False(t, ok)

	_, r, ok = buf.DeleteCell(term.Coordinates{X: 0, Y: 1})
	require.True(t, ok)
	assert.Equal(t, 'w', r)
	_, r, ok = buf.DeleteCell(term.Coordinates{X: 1, Y: 1})
	require.True(t, ok)
	assert.Equal(t, 'r', r)
	buf.ConflateRow(0)
	_, r, ok = buf.DeleteCell(term.Coordinates{X: 7, Y: 0})
	require.True(t, ok)
	assert.Equal(t, 'd', r)

	assert.Equal(t, "hellool", buf.String())
}

func TestBufferDeleteCellAtTab(t *testing.T) {
	str := "!\t\t!\t"
	buf := newBufferWithContent(t, str)
	require.Equal(t, 14, buf.Columns(0))

	start, r, ok := buf.DeleteCell(term.Coordinates{X: 3, Y: 0})
	assert.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 1}, start)
	assert.Equal(t, "!\t!\t", buf.String())
	assert.Equal(t, 10, buf.Columns(0))
	assert.Equal(t, '\t', r)

	start, r, ok = buf.DeleteCell(term.Coordinates{X: 2, Y: 0})
	assert.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 1}, start)
	assert.Equal(t, "!!\t", buf.String())
	assert.Equal(t, 6, buf.Columns(0))
	assert.Equal(t, '\t', r)

	start, r, ok = buf.DeleteCell(term.Coordinates{X: 2, Y: 0})
	assert.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 2}, start)
	assert.Equal(t, "!!", buf.String())
	assert.Equal(t, 2, buf.Columns(0))
	assert.Equal(t, '\t', r)

	str = "\t\t>>>>"
	buf = newBufferWithContent(t, str)
	start, r, ok = buf.DeleteCell(term.Coordinates{X: 3, Y: 0})
	assert.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 0}, start)
	assert.Equal(t, "\t>>>>", buf.String())
	assert.Equal(t, 8, buf.Columns(0))
	assert.Equal(t, '\t', r)
}

type selectCase struct {
	from     term.Coordinates
	to       term.Coordinates
	expected [][]term.Cell
}

func assertCellProperties(t *testing.T, cell term.Cell, attr term.Attributes) {
	assert.Equal(t, attr.Fg, cell.Fg)
	assert.Equal(t, attr.Bg, cell.Bg)
}

func TestBufferTruncateFrom(t *testing.T) {

	tsuite := []struct {
		contents string
		input    term.Coordinates
		ok       bool
		expected string
	}{
		{"hello\nworld\n", term.Coordinates{X: 4, Y: 0}, true, "hell"},
		{"hello\nworld\n", term.Coordinates{X: 0, Y: 1}, true, "hello\n"},
		{longStr, term.Coordinates{X: 6, Y: 0}, true, "Love i"},
	}

	for _, tcase := range tsuite {
		buf := newBufferWithContent(t, tcase.contents)
		ok := buf.TruncateFrom(tcase.input)
		if tcase.ok {
			assert.True(t, ok)
			assert.Equal(t, tcase.expected, buf.String())
		} else {
			assert.False(t, ok)
		}
	}
}

func TestBufferDelete(t *testing.T) {
	t.Run("calls underlying writer Delete", func(t *testing.T) {
		b := NewBuffer()
		_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
		require.NoError(t, err)

		start, end, str := b.Delete(term.Coordinates{}, term.Coordinates{Y: 1})
		assert.Equal(t, term.Coordinates{}, start)
		assert.Equal(t, term.Coordinates{Y: 1}, end)
		assert.Equal(t, "bla\n", str)
	})

	t.Run("does not return last rawcells newline on delete last line", func(t *testing.T) {
		b := NewBuffer()
		_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
		require.NoError(t, err)

		start, end, str := b.Delete(term.Coordinates{Y: 1}, term.Coordinates{Y: 2})
		assert.Equal(t, term.Coordinates{Y: 1}, start)
		assert.Equal(t, term.Coordinates{Y: 2}, end)
		assert.Equal(t, "bleh", str)
	})

	t.Run("panics if coordinates are negative", func(t *testing.T) {
		b := NewBuffer()
		_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
		require.NoError(t, err)

		assert.Panics(t, func() {
			b.Delete(term.Coordinates{X: 10}, term.Coordinates{X: -1})
		})
	})

	t.Run("does not panic if coordinates are partially out of bounds (x)", func(t *testing.T) {
		b := NewBuffer()
		_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
		require.NoError(t, err)

		start, end, str := b.Delete(term.Coordinates{X: 10}, term.Coordinates{X: 11})
		assert.Equal(t, term.Coordinates{X: 3}, start)
		assert.Equal(t, term.Coordinates{X: 3}, end)
		assert.Equal(t, "", str)
	})

	t.Run("does not panic if coordinates are partially out of bounds (y)", func(t *testing.T) {
		b := NewBuffer()
		_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
		require.NoError(t, err)

		start, end, str := b.Delete(term.Coordinates{Y: 1}, term.Coordinates{Y: 1, X: 11})
		assert.Equal(t, term.Coordinates{Y: 1}, start)
		assert.Equal(t, term.Coordinates{Y: 1, X: 4}, end)
		assert.Equal(t, "bleh", str)
	})

	t.Run("does not panic if coordinates are completely out of bounds", func(t *testing.T) {
		b := NewBuffer()
		_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
		require.NoError(t, err)

		start, end, str := b.Delete(term.Coordinates{Y: 1, X: 11}, term.Coordinates{Y: 2, X: 10})
		assert.Equal(t, term.Coordinates{}, start)
		assert.Equal(t, term.Coordinates{}, end)
		assert.Equal(t, "", str)
	})
}

func TestBufferInsertString(t *testing.T) {
	b := NewBuffer()

	from, until := b.InsertString(term.Coordinates{X: 1}, "hello\n")
	assert.Equal(t, term.Coordinates{}, from)
	assert.Equal(t, term.Coordinates{Y: 1}, until)
	b.InsertString(until, "world")
	b.InsertString(until, "")

	assert.Equal(t, " hello\nworld", b.String())
}

type testSubscriber struct {
	onDidInsert, onDidDelete, onWillInsert, onWillDelete int
}

func (t *testSubscriber) OnWillInsert(at term.Coordinates, str string) {
	t.onWillInsert++
}

func (t *testSubscriber) OnDidInsert(from, to term.Coordinates) {
	t.onDidInsert++
}

func (t *testSubscriber) OnWillDelete(from, to term.Coordinates) {
	t.onWillDelete++
}

func (t *testSubscriber) OnDidDelete(start, end term.Coordinates, str string) {
	t.onDidDelete++
}

func (t *testSubscriber) Unsubscribe() {
}

func TestBufferReset(t *testing.T) {
	t.Run("resets the contents of the buffer", func(t *testing.T) {
		var b Buffer
		b.InitWithTabspaces(4)
		b.ReadFrom(strings.NewReader("a\tb"))

		assert.Equal(t, "a\tb", b.String())

		b.Reset()
		assert.Equal(t, "", b.String())
		assert.Equal(t, 4, b.Tabspaces())
	})

	t.Run("does not reset subscribers", func(t *testing.T) {
		b := NewBuffer()
		sub := testSubscriber{}
		b.Subscribe(&sub)

		b.Reset()
		b.InsertRowAt(0)
		b.DeleteRow(0)
		assert.Equal(t, 1, sub.onDidInsert)
		assert.Equal(t, 1, sub.onWillInsert)
		// 1 reset + 1 delete
		assert.Equal(t, 2, sub.onWillDelete)
		assert.Equal(t, 2, sub.onWillDelete)
	})

	t.Run("does reset undo", func(t *testing.T) {
		b := NewBuffer()
		sub := testSubscriber{}
		b.Subscribe(&sub)

		b.InsertRowAt(0)
		b.Reset()

		ok, _ := b.Undo()
		assert.False(t, ok)
	})
}

func TestBufferDeleteLine(t *testing.T) {
	tsuite := []struct {
		from, to   term.Coordinates
		input      string
		output     string
		start, end term.Coordinates
	}{
		{
			from:   term.Coordinates{X: 1},
			to:     term.Coordinates{Y: 1, X: 2},
			input:  "bla\nbleh",
			output: "bla\nbleh",
			start:  term.Coordinates{},
			end:    term.Coordinates{Y: 2},
		},
		{
			// inverted from/to
			to:     term.Coordinates{X: 1},
			from:   term.Coordinates{Y: 1, X: 2},
			input:  "bla\nbleh",
			output: "bla\nbleh",
			start:  term.Coordinates{},
			end:    term.Coordinates{Y: 2},
		},
		{
			from:   term.Coordinates{X: 1}, // should not matter that is oob
			to:     term.Coordinates{Y: 2},
			input:  "\nbla\n\nbleh\n",
			output: "\nbla\n\n",
			start:  term.Coordinates{},
			end:    term.Coordinates{Y: 3},
		},
		{
			from:   term.Coordinates{Y: 1},
			to:     term.Coordinates{Y: 2},
			input:  "{\n\tb\n\tc\n}",
			output: "\tb\n\tc\n",
			start:  term.Coordinates{Y: 1},
			end:    term.Coordinates{Y: 3},
		},
	}

	for _, tcase := range tsuite {
		b := NewBuffer()
		b.WriteString(tcase.input)

		start, end, str := b.DeleteLine(tcase.from, tcase.to)
		assert.Equal(t, tcase.start, start)
		assert.Equal(t, tcase.end, end)
		assert.Equal(t, tcase.output, str)
	}
}

func TestBufferDeleteBlock(t *testing.T) {
	tsuite := []struct {
		from, to   term.Coordinates
		input      string
		start, end term.Coordinates
		str        string
	}{
		{
			from:  term.Coordinates{X: 1},
			to:    term.Coordinates{Y: 1, X: 2},
			input: "bla\nbleh",
			start: term.Coordinates{X: 1},
			end:   term.Coordinates{Y: 1, X: 2},
			str:   "l\nl",
		},
		{ // inverted
			to:    term.Coordinates{X: 1},
			from:  term.Coordinates{Y: 1, X: 2},
			input: "bla\nbleh",
			start: term.Coordinates{X: 1},
			end:   term.Coordinates{Y: 1, X: 2},
			str:   "l\nl",
		},
		{
			from:  term.Coordinates{},
			to:    term.Coordinates{Y: 2},
			input: "\nbla\n\nbleh\n",
			start: term.Coordinates{},
			end:   term.Coordinates{Y: 2},
			str:   "\n\n",
		},
	}

	for _, tcase := range tsuite {
		b := NewBuffer()
		b.WriteString(tcase.input)

		start, end, str := b.DeleteBlock(tcase.from, tcase.to)
		assert.Equal(t, tcase.start, start)
		assert.Equal(t, tcase.end, end)
		assert.Equal(t, tcase.str, str)
	}

	const str = `/*
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
} /* {                                                                                 */`

	t.Run("does not OOB for lines that are shorter than to", func(t *testing.T) {
		b := NewBuffer()
		b.WriteString(str)

		assert.Equal(t, str, b.String())
		to := term.Coordinates{X: 89, Y: 30}
		start, end, _ := b.DeleteBlock(term.Coordinates{}, to)
		assert.Equal(t, term.Coordinates{}, start)
		assert.Equal(t, to, end)
	})

	t.Run("does not OOB for lines that are shorter than from", func(t *testing.T) {
		expected := `/
 
 
 

d
{























}`

		b := NewBuffer()
		b.WriteString(str)

		assert.Equal(t, str, b.String())
		to := term.Coordinates{X: 1, Y: 0}
		from := term.Coordinates{X: 89, Y: 30}
		start, end, _ := b.DeleteBlock(from, to)
		assert.Equal(t, to, start)
		assert.Equal(t, from, end)

		assert.Equal(t, expected, b.String())
	})
}

func TestBufferShiftRowTabs(t *testing.T) {
	stringNoTab := "the_3T_ring_idea_is_fucking_cool\n:D"
	origString := "\t" + stringNoTab
	buf := NewBuffer()
	buf.ReadFrom(strings.NewReader(origString))

	shifted := buf.ShiftRowLeft(0)
	assert.Equal(t, 4, shifted)
	assert.Equal(t, stringNoTab, buf.String())

	shifted = buf.ShiftRowLeft(0)
	assert.Equal(t, 0, shifted)
	assert.Equal(t, stringNoTab, buf.String())

	shifted = buf.ShiftRowRight(0)
	assert.Equal(t, 4, shifted)
	assert.Equal(t, origString, buf.String())
}

func TestBufferShiftRowSpaces(t *testing.T) {
	origString := "     "
	buf := NewBuffer()
	buf.ReadFrom(strings.NewReader(origString))

	shifted := buf.ShiftRowLeft(0)
	assert.Equal(t, 4, shifted)
	assert.Equal(t, " ", buf.String())

	shifted = buf.ShiftRowLeft(0)
	assert.Equal(t, 1, shifted)
	assert.Equal(t, "", buf.String())
}

func TestBufferUnsubscribe(t *testing.T) {
	buf := NewBuffer()
	one := &testSubscriber{}
	two := &testSubscriber{}
	buf.Subscribe(one)
	buf.Subscribe(two)

	assert.Panics(t, func() {
		buf.Unsubscribe(&testSubscriber{})
	})

	assert.NotPanics(t, func() {
		buf.Unsubscribe(two)
		buf.Unsubscribe(one)
	})
}

func TestBufferSubscribe(t *testing.T) {
	buf := NewBuffer()
	one := &testSubscriber{}
	two := &testSubscriber{}
	buf.Subscribe(one)
	buf.Subscribe(two)

	buf.WriteString("\n")
	assert.Equal(t, 1, one.onWillInsert)
	assert.Equal(t, 1, one.onDidInsert)
	assert.Equal(t, 1, two.onWillInsert)
	assert.Equal(t, 1, two.onDidInsert)
}

func TestBufferInsertWithAttr(t *testing.T) {
	buf := NewBuffer()
	fg := term.ColorRed
	bg := term.ColorYellow

	buf.InsertWithAttr(term.Coordinates{}, 'A',
		term.Attributes{Fg: fg, Bg: bg})
	cell := buf.RawCells()[0][0]
	assert.Equal(t, term.Cell{Ch: 'A', Fg: fg, Bg: bg}, cell)

	buf.InsertWithAttr(term.Coordinates{X: 1}, '\n',
		term.Attributes{Fg: fg, Bg: bg})

	buf.InsertWithAttr(term.Coordinates{Y: 1}, 'E',
		term.Attributes{Fg: fg, Bg: bg})
	cell = buf.RawCells()[1][0]
	assert.Equal(t, term.Cell{Ch: 'E', Fg: fg, Bg: bg}, cell)
}

func TestBufferInsertStringWithAttr(t *testing.T) {
	fg := term.ColorRed
	bg := term.ColorYellow
	t.Run("insert single line string", func(t *testing.T) {
		buf := NewBuffer()
		buf.InsertStringWithAttr(term.Coordinates{}, "Atza",
			term.Attributes{Fg: fg, Bg: bg})
		row := buf.RawCells()[0]
		assert.Equal(t, []term.Cell{
			term.Cell{Ch: 'A', Fg: fg, Bg: bg},
			term.Cell{Ch: 't', Fg: fg, Bg: bg},
			term.Cell{Ch: 'z', Fg: fg, Bg: bg},
			term.Cell{Ch: 'a', Fg: fg, Bg: bg},
		}, row)
	})
	t.Run("insert multi line string", func(t *testing.T) {
		buf := NewBuffer()
		buf.InsertStringWithAttr(term.Coordinates{}, "Lola\nGranola",
			term.Attributes{Fg: fg, Bg: bg})
		cells := buf.RawCells()
		assert.Equal(t, [][]term.Cell{
			[]term.Cell{
				term.Cell{Ch: 'L', Fg: fg, Bg: bg},
				term.Cell{Ch: 'o', Fg: fg, Bg: bg},
				term.Cell{Ch: 'l', Fg: fg, Bg: bg},
				term.Cell{Ch: 'a', Fg: fg, Bg: bg},
			},
			[]term.Cell{
				term.Cell{Ch: 'G', Fg: fg, Bg: bg},
				term.Cell{Ch: 'r', Fg: fg, Bg: bg},
				term.Cell{Ch: 'a', Fg: fg, Bg: bg},
				term.Cell{Ch: 'n', Fg: fg, Bg: bg},
				term.Cell{Ch: 'o', Fg: fg, Bg: bg},
				term.Cell{Ch: 'l', Fg: fg, Bg: bg},
				term.Cell{Ch: 'a', Fg: fg, Bg: bg},
			},
		}, cells)
	})
}

func TestBufferHeightWidth(t *testing.T) {
	buf := NewBuffer()
	buf.WriteString("aaaaaaaaaaaaaaaaaaaa\naaa\naaaaaaa\naaa")
	assert.Equal(t, 4, buf.Height())
	assert.Equal(t, 20, buf.Width())
}

func TestBufferMaxColumns(t *testing.T) {
	buf := newBufferWithContent(t, longStr)
	assert.Equal(t, buf.MaxColumns(), 44)

	buf = newBufferWithContent(t, str)
	assert.Equal(t, buf.MaxColumns(), 9)
}

func testBufferSelect(t *testing.T, fn func(b *Buffer, from, to term.Coordinates) ([][]term.Cell, bool)) {
	t.Run("does not panic if oob", func(t *testing.T) {
		b := NewBuffer()
		b.WriteString("a")
		_, ok := fn(b, term.Coordinates{X: 2}, term.Coordinates{Y: 1})
		assert.False(t, ok)
	})
}

func TestBufferSelect(t *testing.T) {
	testBufferSelect(t, (*Buffer).Select)
}

func TestBufferSelectLine(t *testing.T) {
	testBufferSelect(t, (*Buffer).SelectLine)
}

func TestBufferSelectBlock(t *testing.T) {
	testBufferSelect(t, (*Buffer).SelectBlock)
}

func TestBufferVersion(t *testing.T) {
	b := NewBuffer()
	assert.Equal(t, 0, b.Version())

	b.WriteString("bla")
	assert.Equal(t, 1, b.Version())

	b.DeleteRow(0)
	assert.Equal(t, 2, b.Version())

	b.Undo()
	assert.Equal(t, 1, b.Version())

	b.Undo()
	assert.Equal(t, 0, b.Version())

	b.Undo()
	assert.Equal(t, 0, b.Version())

	for i := 0; i < 5; i++ {
		b.Redo()
	}
	assert.Equal(t, 2, b.Version())

	b.Reset()
	assert.Equal(t, 0, b.Version())
}

func TestBufferWriteStringRawCells(t *testing.T) {
	b := NewBuffer()
	b.WriteString("a\nb\nc")
	assert.Equal(t, [][]term.Cell{{{Ch: 'a'}}, {{Ch: 'b'}}, {{Ch: 'c'}}}, b.RawCells())

	b.WriteString("xyz")
	assert.Equal(t, [][]term.Cell{{{Ch: 'a'}}, {{Ch: 'b'}}, {{Ch: 'c'}, {Ch: 'x'}, {Ch: 'y'}, {Ch: 'z'}}}, b.RawCells())
}

func TestBufferInsertRowAt(t *testing.T) {
	tsuite := []struct {
		desc    string
		inStr   string
		inY     int
		wantOut string
	}{
		{"first row", "", 0, "\n"},
		{"first row above last row", "a", 0, "\na"},
		{"last row", "a", 1, "a\n"},
		{"past last row", "a", 2, "a\n\n"},
		{"in the middle of buffer", "a\nb\nc", 1, "a\n\nb\nc"},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			b := NewBuffer()
			b.WriteString(tcase.inStr)

			b.InsertRowAt(tcase.inY)
			assert.Equal(t, tcase.wantOut, b.String())
		})
	}
}
