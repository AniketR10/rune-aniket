package cell

import (
	"fmt"
	"strings"
	"testing"

	"unstable.build/go-tui/term"

	"github.com/ernestrc/tcell/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	tsuite := []struct {
		content     string
		in          int
		wantOk      bool
		wantContent string
	}{
		{"hello\nworld", 0, true, "world"},
		{"world", 0, true, ""},
		{"\nworld", 0, true, "world"},
		{"1234", 1, false, "1234"},
		{"ya-basic\n", 1, true, "ya-basic"},
		{"ya-basic\na", 1, true, "ya-basic"},
		{"a\nb\nc", 1, true, "a\nc"},
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			buf := newBufferWithContent(t, tcase.content)
			assert.Equal(t, tcase.wantOk, buf.DeleteRow(tcase.in))
			assert.Equal(t, tcase.wantContent, buf.String())
		})
	}
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
	tsuite := []struct {
		description string
		content     string
		pos         term.Coordinates
		wantOk      bool
		wantRune    rune
		wantStart   term.Coordinates
	}{
		{"delete cell out of X bounds returns false",
			"a", term.Coordinates{X: 2}, false, 0, term.Coordinates{}},
		{"delete cell exactly out of X bounds returns false",
			"a", term.Coordinates{X: 1}, false, 0, term.Coordinates{}},
		{"delete cell out of Y bounds returns false",
			"a\nb", term.Coordinates{Y: 2}, false, 0, term.Coordinates{}},
		{"delete last cell of buffer",
			"a\nb", term.Coordinates{Y: 1}, true, 'b', term.Coordinates{Y: 1}},
		{"delete first cell of buffer",
			"a\nb", term.Coordinates{}, true, 'a', term.Coordinates{}},
		{"delete start of the line tab at tab",
			"\ta", term.Coordinates{X: 3}, true, '\t', term.Coordinates{}},
		{"delete start of the line tab at null 0",
			"\ta", term.Coordinates{}, true, '\t', term.Coordinates{}},
		{"delete start of the line tab at null 1",
			"\ta", term.Coordinates{X: 1}, true, '\t', term.Coordinates{}},
		{"delete start of the line tab at null 2",
			"\ta", term.Coordinates{X: 2}, true, '\t', term.Coordinates{}},
		{"delete end of the line tab at tab",
			"a\t", term.Coordinates{X: 4}, true, '\t', term.Coordinates{X: 1}},
		{"delete end of the line tab at null 0",
			"a\t", term.Coordinates{X: 1}, true, '\t', term.Coordinates{X: 1}},
		{"delete end of the line tab at null 1",
			"a\t", term.Coordinates{X: 2}, true, '\t', term.Coordinates{X: 1}},
		{"delete end of the line tab at null 2",
			"a\t", term.Coordinates{X: 3}, true, '\t', term.Coordinates{X: 1}},
		{"handle ending null with no tab (by silently cleaning up nulls)",
			"a\x00\x00", term.Coordinates{X: 1}, false, 0, term.Coordinates{}},
		{"delete next character if starting null not part of tab expansion (and nulls)",
			"\x00\x00a", term.Coordinates{X: 1}, false, 0, term.Coordinates{}},
		{"delete start and end of the line tab at tab",
			"\t", term.Coordinates{X: 3}, true, '\t', term.Coordinates{X: 0}},
		{"delete start and end of the line tab at null 0",
			"\t", term.Coordinates{X: 0}, true, '\t', term.Coordinates{X: 0}},
		{"delete start and end of the line tab at null 1",
			"\t", term.Coordinates{X: 1}, true, '\t', term.Coordinates{X: 0}},
		{"delete start and end of the line tab at null 2",
			"\t", term.Coordinates{X: 2}, true, '\t', term.Coordinates{X: 0}},
		{"delete >1 width character before >1 width character",
			"💥💥", term.Coordinates{X: 0}, true, '💥', term.Coordinates{}},
		{"delete >1 width character after >1 width character",
			"💥💥", term.Coordinates{X: 2}, true, '💥', term.Coordinates{X: 2}},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.description, func(t *testing.T) {
			buf := newBufferWithContent(t, tcase.content)
			actualPos, actualR, actualOk := buf.DeleteCell(tcase.pos)
			require.Equal(t, tcase.wantOk, actualOk, buf.RawCells())
			assert.Equal(t, tcase.wantRune, actualR, buf.RawCells())
			assert.Equal(t, tcase.wantStart, actualPos, buf.RawCells())
		})
	}
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

func TestBufferTruncateFromWithUnixView(t *testing.T) {

	tsuite := []struct {
		contents               string
		input                  term.Coordinates
		ok                     bool
		expectedInternalString string
	}{
		{"hello\nworld", term.Coordinates{X: 4, Y: 0}, true, "hell"},
		{"hello\nworld", term.Coordinates{X: 0, Y: 1}, true, "hello\n"},
		{"hello\nworld\n", term.Coordinates{X: 4, Y: 0}, true, "hell\n"},
		{"hello\nworld\n", term.Coordinates{X: 0, Y: 1}, true, "hello\n\n"},
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			buf := newBufferWithContent(t, tcase.contents)
			buf.WithView(&testView{reader: buf.View()})

			ok := buf.TruncateFrom(tcase.input)
			if tcase.ok {
				assert.True(t, ok)
				assert.Equal(t, tcase.expectedInternalString, buf.cells.String())
			} else {
				assert.False(t, ok)
			}
		})
	}
}

func TestBufferDelete(t *testing.T) {
	t.Run("calls underlying writer Delete", func(t *testing.T) {
		b := NewBuffer()
		_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
		require.NoError(t, err)

		start, str := b.Delete(term.Coordinates{}, term.Coordinates{Y: 1})
		assert.Equal(t, term.Coordinates{}, start)
		assert.Equal(t, "bla\n", str)
	})

	t.Run("does not return last rawcells newline on delete last line", func(t *testing.T) {
		b := NewBuffer()
		_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
		require.NoError(t, err)

		start, str := b.Delete(term.Coordinates{Y: 1}, term.Coordinates{Y: 2})
		assert.Equal(t, term.Coordinates{Y: 1}, start)
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

		start, str := b.Delete(term.Coordinates{X: 10}, term.Coordinates{X: 11})
		assert.Equal(t, term.Coordinates{X: 3}, start)
		assert.Equal(t, "", str)
	})

	t.Run("does not panic if coordinates are partially out of bounds (y)", func(t *testing.T) {
		b := NewBuffer()
		_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
		require.NoError(t, err)

		start, str := b.Delete(term.Coordinates{Y: 1}, term.Coordinates{Y: 1, X: 11})
		assert.Equal(t, term.Coordinates{Y: 1}, start)
		assert.Equal(t, "bleh", str)
	})

	t.Run("does not panic if coordinates are completely out of bounds", func(t *testing.T) {
		b := NewBuffer()
		_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
		require.NoError(t, err)

		start, str := b.Delete(term.Coordinates{Y: 1, X: 11}, term.Coordinates{Y: 2, X: 10})
		assert.Equal(t, term.Coordinates{}, start)
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
	onDidEdit, onWillEdit int
}

func (t *testSubscriber) OnWillEdit(from, to term.Coordinates, str string) {
	t.onWillEdit++
}

func (t *testSubscriber) OnDidEdit(start, end term.Coordinates, old string) {
	t.onDidEdit++
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
		// 1 reset + 1 delete + 1 insert
		assert.Equal(t, 3, sub.onDidEdit)
		assert.Equal(t, 3, sub.onWillEdit)
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
		from, to term.Coordinates
		input    string
		output   string
		start    term.Coordinates
	}{
		{
			from:   term.Coordinates{X: 1},
			to:     term.Coordinates{Y: 1, X: 2},
			input:  "bla\nbleh",
			output: "bla\nbleh",
			start:  term.Coordinates{},
		},
		{
			// inverted from/to
			to:     term.Coordinates{X: 1},
			from:   term.Coordinates{Y: 1, X: 2},
			input:  "bla\nbleh",
			output: "bla\nbleh",
			start:  term.Coordinates{},
		},
		{
			from:   term.Coordinates{X: 1}, // should not matter that is oob
			to:     term.Coordinates{Y: 2},
			input:  "\nbla\n\nbleh\n",
			output: "\nbla\n\n",
			start:  term.Coordinates{},
		},
		{
			from: term.Coordinates{Y: 1},
			to:   term.Coordinates{Y: 2},
			input: `{
	b
	c
}`,
			output: "\tb\n\tc\n",
			start:  term.Coordinates{Y: 1},
		},
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			b := NewBuffer()
			b.WriteString(tcase.input)

			start, str := b.DeleteLine(tcase.from, tcase.to)
			assert.Equal(t, tcase.start, start)
			assert.Equal(t, tcase.output, str)
		})
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

		start, str := b.DeleteBlock(tcase.from, tcase.to)
		assert.Equal(t, tcase.start, start)
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
		start, _ := b.DeleteBlock(term.Coordinates{}, to)
		assert.Equal(t, term.Coordinates{}, start)
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
		start, _ := b.DeleteBlock(from, to)
		assert.Equal(t, to, start)

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

func TestBufferShiftRowTabsWithMultiWidthChar(t *testing.T) {
	stringNoTab := "💥the_3T_ring_idea_is_fucking_cool\n:D"
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
	assert.Equal(t, 1, one.onWillEdit)
	assert.Equal(t, 1, one.onDidEdit)
}

func TestBufferInsertWithAttr(t *testing.T) {
	buf := NewBuffer()
	attr := term.Attributes{Fg: tcell.ColorRed, Bg: tcell.ColorYellow, Attrs: tcell.AttrBold}

	buf.InsertWithAttr(term.Coordinates{}, 'A', attr)
	cell := buf.RawCells()[0][0]
	assert.Equal(t, term.Cell{Ch: 'A', Attributes: attr, Width: 1}, cell)

	buf.InsertWithAttr(term.Coordinates{X: 1}, '\n', attr)

	buf.InsertWithAttr(term.Coordinates{Y: 1}, 'E', attr)
	cell = buf.RawCells()[1][0]
	assert.Equal(t, term.Cell{Ch: 'E', Attributes: attr, Width: 1}, cell)
}

func TestBufferInsertStringWithAttr(t *testing.T) {
	attr := term.Attributes{Fg: tcell.ColorRed, Bg: tcell.ColorYellow, Attrs: tcell.AttrItalic}
	t.Run("insert single line string", func(t *testing.T) {
		buf := NewBuffer()
		buf.InsertStringWithAttr(term.Coordinates{}, "Atza", attr)
		row := buf.RawCells()[0]
		assert.Equal(t, []term.Cell{
			{Ch: 'A', Attributes: attr, Width: 1},
			{Ch: 't', Attributes: attr, Width: 1},
			{Ch: 'z', Attributes: attr, Width: 1},
			{Ch: 'a', Attributes: attr, Width: 1},
		}, row)
	})
	t.Run("insert multi line string", func(t *testing.T) {
		buf := NewBuffer()
		buf.InsertStringWithAttr(term.Coordinates{}, "Lola\nGranola", attr)
		cells := buf.RawCells()
		assert.Equal(t, [][]term.Cell{
			{
				{Ch: 'L', Attributes: attr, Width: 1},
				{Ch: 'o', Attributes: attr, Width: 1},
				{Ch: 'l', Attributes: attr, Width: 1},
				{Ch: 'a', Attributes: attr, Width: 1},
			},
			{
				{Ch: 'G', Attributes: attr, Width: 1},
				{Ch: 'r', Attributes: attr, Width: 1},
				{Ch: 'a', Attributes: attr, Width: 1},
				{Ch: 'n', Attributes: attr, Width: 1},
				{Ch: 'o', Attributes: attr, Width: 1},
				{Ch: 'l', Attributes: attr, Width: 1},
				{Ch: 'a', Attributes: attr, Width: 1},
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
	const str = "hello\n\tworld\n"
	buf := newBufferWithContent(t, longStr)
	assert.Equal(t, buf.MaxColumns(), 44)

	buf = newBufferWithContent(t, str)
	assert.Equal(t, buf.MaxColumns(), 9)
}

func testBufferSelect(t *testing.T, fn func(b *Buffer, from, to term.Coordinates) ([][]term.Cell, []Selection, bool)) {
	t.Run("does not panic if oob", func(t *testing.T) {
		b := NewBuffer()
		b.WriteString("a")
		_, _, ok := fn(b, term.Coordinates{X: 2}, term.Coordinates{Y: 1})
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
	assert.Equal(t, [][]term.Cell{{{Ch: 'a', Width: 1}}, {{Ch: 'b', Width: 1}}, {{Ch: 'c', Width: 1}}}, b.RawCells())

	b.WriteString("xyz")
	assert.Equal(t, [][]term.Cell{{{Ch: 'a', Width: 1}}, {{Ch: 'b', Width: 1}}, {{Ch: 'c', Width: 1}, {Ch: 'x', Width: 1}, {Ch: 'y', Width: 1}, {Ch: 'z', Width: 1}}}, b.RawCells())
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
		{"last row only one row", "a", 1, "a\n"},
		{"last row with prev rows", "z\na", 2, "z\na\n"},
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

type testView struct {
	reader View
}

func newUnixFileReader(r View) *testView {
	b := new(testView)
	b.reader = r
	return b
}

func (b *testView) endsWithEOL() bool {
	cells := b.reader.RawCells()
	return len(cells) > 1 && len(cells[len(cells)-1]) == 0
}

func (b *testView) Rows() (rows int) {
	rows = b.reader.Rows()
	if !b.endsWithEOL() {
		return
	}
	rows--
	return

}

func (b *testView) Columns(row int) int {
	return b.reader.Columns(row)
}

func (b *testView) Cell(pos term.Coordinates) (term.Cell, bool) {
	return b.reader.Cell(pos)
}

func (b *testView) RawCells() (cells [][]term.Cell) {
	cells = b.reader.RawCells()
	if !b.endsWithEOL() {
		return
	}
	cells = cells[:len(cells)-1]
	return
}

func (b *testView) String() string {
	if !b.endsWithEOL() {
		return b.reader.String()
	}
	return CellsToString(b.RawCells())
}

func TestBufferInsertRowAtWithUnixView(t *testing.T) {
	tsuite := []struct {
		desc    string
		inStr   string
		inY     int
		wantOut string
	}{
		{"(with EOL) first row", "\n", 0, "\n"},
		{"(with EOL) first row above last row", "a\n", 0, "\na"},
		{"(with EOL) last row only one row", "a\n", 1, "a\n"},
		{"(with EOL) last row with prev rows", "z\na\n", 2, "z\na\n"},
		{"(with EOL) past last row", "a\n", 2, "a\n"},
		{"(with EOL) in the middle of buffer", "a\nb\nc\n", 1, "a\n\nb\nc"},
		{"(no EOL) first row", "", 0, ""},
		{"(no EOL) first row above last row", "a", 0, "\na"},
		{"(no EOL) last row only one row", "a", 1, "a"},
		{"(no EOL) last row with prev rows", "z\na", 2, "z\na"},
		{"(no EOL) past last row", "a", 2, "a\n"},
		{"(no EOL) in the middle of buffer", "a\nb\nc", 1, "a\n\nb\nc"},
	}

	for _, tcase := range tsuite {
		t.Run("WriteString "+tcase.desc, func(t *testing.T) {
			b := NewBuffer()
			b.WithView(&testView{reader: b.View()})
			b.WriteString(tcase.inStr)

			b.InsertRowAt(tcase.inY)
			assert.Equal(t, tcase.wantOut, b.String())
		})

		t.Run("ReadFrom "+tcase.desc, func(t *testing.T) {
			b := NewBuffer()
			b.WithView(&testView{reader: b.View()})
			b.ReadFrom(strings.NewReader(tcase.inStr))

			b.InsertRowAt(tcase.inY)
			assert.Equal(t, tcase.wantOut, b.String())
		})
	}
}

func TestBufferReplaceAll(t *testing.T) {
	tsuite := []struct {
		desc string
		in   string
		out  string
	}{
		{"replace small content with large content no newline", "a\nb\nc\n", "aaaa\nbbbb\nccccc\ndddd\n"},
		{"replace small content with large content newline", "a\nb\nc\n", "aaaa\nbbbb\nccccc\ndddd\n"},
		{"replace large content with small content no newline", "aaaa\nbbbb\nccccc\ndddd\n", "a\nb\nc\n"},
		{"replace large content with small content newline", "aaaa\nbbbb\nccccc\ndddd\n", "a\nb\nc\n"},
		{"replace empty content with non-empty", "", "a"},
		{"replace empty content", "", ""},
		{"replace non-empty content with empty", "a", ""},
		{"replace newline content with empty", "\n", ""},
		{"replace newline content with empty", "a\n\tb\n", "\tc\nd\n"},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			b := NewBuffer()
			b.WriteString(tcase.in)
			b.Replace(tcase.out)
			assert.Equal(t, tcase.out, b.String())
		})
	}

}
