package fractal

import (
	"termbox"
	"testing"
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
}
`

type batchTestCase struct {
	batch  string
	input  string
	output string
}

func testBatchWorkload(t *testing.T, width, height int, cases []batchTestCase) {

	writer := NewStringWriter(width, height)
	vi := NewVi(NewViConfig(2, false, nil))
	vi.Resize(width, height)

	for _, tcase := range cases {
		if err := writer.Clear(0, 0); err != nil {
			t.Fatal(err)
		}

		vi.SetContent(tcase.input)

		for _, r := range tcase.batch {
			if _, err := vi.Handle(termbox.Event{Ch: r, Type: termbox.EventKey}); err != nil {
				t.Fatal(err)
			}
		}

		if err := vi.Draw(writer); err != nil {
			t.Fatal(err)
		}

		writer.SetCursor(vi.GetCursor())

		if err := writer.Flush(); err != nil {
			t.Fatal(err)
		}
		out := writer.String()
		if out != tcase.output {
			t.Errorf(">>>>>>>>>>> expected:\n%s\n------------ found:\n%s\n",
				tcase.output, out)
		}
	}
}

func TestViCursor(t *testing.T) {
	cases := []batchTestCase{
		{"", snippet,
			`                    
▐*                  
 * Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"jjjj", snippet,
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
▐ void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
	}
	testBatchWorkload(t, 20, 10, cases)
}
