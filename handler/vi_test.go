package handler

import (
	"strings"
	"testing"

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

func setupVi(t *testing.T, text string, width, height int, config ViConfig) *Vi {
	vi := NewVi().WithConfig(config)
	vi.Resize(width, height)

	_, err := vi.ReadFrom(strings.NewReader(text))
	require.NoError(t, err)
	return vi
}

func testBatchWorkload(t *testing.T, width, height int, cases []batchTestCase) {
	writer := term.NewStringWriter(width, height)

	vi := setupVi(t, snippet, width, height, ViConfig{Tabspaces: 2})

	for _, tcase := range cases {
		err := writer.Clear(term.Attributes{Fg: 0, Bg: 0})
		require.NoError(t, err)

		for _, r := range tcase.batch {
			if r == '>' {
				vi.Handle(term.Event{Key: term.KeyEnter, Type: term.EventKey})
			} else {
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
	vi := setupVi(t, snippet, width, height, ViConfig{Tabspaces: 2})

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
		// FIXME: > is ENTER key
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
	}

	testBatchWorkload(t, 20, 10, cases)
}
