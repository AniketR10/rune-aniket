package handler

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const snippet = `
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
}`

type batchTestCase struct {
	batch  string
	output string
}

func setupVi(t *testing.T, text string, width, height int, opts ...ViOption) *Vi {
	buf := cell.NewBuffer()
	_, err := buf.ReadFrom(strings.NewReader(text))
	require.NoError(t, err)

	opts = append(opts, WithViBuffer(buf))

	vi, err := NewVi(opts...)
	require.NoError(t, err)
	vi.Resize(width, height)

	return vi
}

func testBatchWorkload(t *testing.T, width, height int, cases []batchTestCase) {
	writer := term.NewStringWriter(width, height)

	vi := setupVi(t, snippet, width, height, WithViTabspaces(2))

	for _, tcase := range cases {
		err := writer.Clear(term.Attributes{Fg: 0, Bg: 0})
		require.NoError(t, err)

		for _, r := range tcase.batch {
			switch r {
			case '>':
				vi.Handle(term.Event{Key: term.KeyEnter, Type: term.EventKey})
			case '<':
				vi.Handle(term.Event{Key: term.KeyEsc, Type: term.EventKey})
			default:
				vi.Handle(term.Event{Ch: r, Type: term.EventKey})
			}
		}

		vi.Draw(writer)

		cursor, ok := vi.Cursor()
		require.True(t, ok)
		writer.SetCursor(cursor)

		err = writer.Flush()
		require.NoError(t, err)

		out := writer.String()
		assert.Equal(t, tcase.output, out)
	}
}

func TestCellAtCursor(t *testing.T) {
	cases := []struct {
		input string
		cell  rune
	}{
		{"k", '\x00'},
		{"j", '/'},
		{"l", '*'},
		{"$", '*'},
	}

	width, height := 20, 10

	writer := term.NewStringWriter(width, height)
	vi := setupVi(t, snippet, width, height, WithViTabspaces(2))

	for _, tcase := range cases {
		for _, r := range tcase.input {
			vi.Handle(term.Event{Type: term.EventKey, Ch: r})

			vi.Draw(writer)

			err := writer.Flush()
			require.NoError(t, err)
		}
		c, ok := vi.cursor.Cell()
		if tcase.cell == '\x00' {
			assert.False(t, ok)
		} else {
			assert.True(t, ok)
			assert.Equal(t, tcase.cell, c.Ch)
		}
	}
}

func TestViCursor(t *testing.T) {
	cases := []batchTestCase{
		{"",
			`▐                   
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
:             NORMAL`},
		{"jjjj",
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
▐*/                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
:             NORMAL`},
		{"/NULL>",
			`  if (wp == ▐ULL)   
  {                 
    i = diff_buf_idx
    if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
:             NORMAL`},
		{":w",
			`  if (wp == NULL)   
  {                 
    i = diff_buf_idx
    if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
:w▐          COMMAND`},
		{">",
			`  if (wp == ▐ULL)   
  {                 
    i = diff_buf_idx
    if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
Error: Cannot NORMAL`},
		{"Ahello",
			`f (wp == NULL)hello▐
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
:             INSERT`},
		{"<hhhhC<",
			`f (wp == NULL▐      
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
:             NORMAL`},
	}

	testBatchWorkload(t, 20, 10, cases)
}

func TestViCommandMode(t *testing.T) {
	const height, width = 10, 10

	buf := cell.NewBuffer()
	_, err := buf.ReadFrom(strings.NewReader(snippet))
	require.NoError(t, err)

	vi, err := NewVi(WithViBuffer(buf))
	require.NoError(t, err)

	vi.Resize(width, height)

	pos, _ := vi.Cursor()
	assert.Equal(t, term.Coordinates{}, pos)

	assert.False(t, vi.Handle(term.Event{Ch: ':'}))
	pos, _ = vi.Cursor()
	assert.Equal(t, term.Coordinates{Y: height - 1, X: 1}, pos)

	assert.False(t, vi.Handle(term.Event{Ch: 'q'}))
	pos, _ = vi.Cursor()
	assert.Equal(t, term.Coordinates{Y: height - 1, X: 2}, pos)

	assert.True(t, vi.Handle(term.Event{Key: term.KeyEnter}))
}
