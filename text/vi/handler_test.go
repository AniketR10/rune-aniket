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

package vi

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/handlertest"
	"unstable.build/go-tui/term"
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
	int				i;

	if (!win->w_p_diff)
	{
	/* When there is no window showing a diff for this buffer, remove
	 * it from the diffs... */
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
	vi := setupVi(t, snippet, 2)
	vi.Resize(width, height)

	for _, tcase := range cases {
		for _, r := range tcase.input {
			_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: r})
			require.True(t, handled)

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

func TestMatchingRuneHighlight(t *testing.T) {
	width, height := 20, 10

	vi := setupVi(t, snippet, 2)
	vi.Resize(width, height)

	for _, r := range "jjjjjjj" {
		_, handled := vi.Handle(term.Event{Type: term.EventKey, Ch: r})
		require.True(t, handled)
	}

	c, ok := vi.cursor.Cell()
	assert.True(t, ok)
	require.Equal(t, '{', c.Ch)

	list, ok := vi.cursor.LocationList(matchingLocID)
	require.True(t, ok)
	loc, ok := list.Current()
	require.True(t, ok)
	assert.Equal(t, tcell.AttrReverse, loc.Attr.Attrs)
	assert.Equal(t, term.Coordinates{Y: 31}, loc.From)
	assert.Equal(t, term.Coordinates{Y: 31, X: 1}, loc.To)
}

func TestViIntegrationSequence(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
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
              NORMAL`},
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
              NORMAL`},
		{"/NULL>jjjjjjjjkkkkkkkk",
			`  if (wp == ▐ULL)   
  {                 
    i = diff_buf_idx
    if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
    searching 'NULL'`},
		{"Ahello",
			`f (wp == NULL)hello▐
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              INSERT`},
		{"<hhhhC<",
			`f (wp == NULL▐      
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"p",
			`f (wp == NULL)▐ello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"F(",
			`f ▐wp == NULL)hello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"f)",
			`f (wp == NULL▐hello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"F=",
			`f (wp =▐ NULL)hello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{",",
			`f (wp ▐= NULL)hello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{";",
			`f (wp =▐ NULL)hello 
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"D",
			`f (wp ▐             
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"sbrillo",
			`f (wp brillo▐       
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              INSERT`},
		{"<hhhhhhR == NULL)",
			`f (wp == NULL)▐     
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
             REPLACE`},
		{"<r]h",
			`f (wp == NUL▐]      
                    
 i = diff_buf_idx(wi
 if (i != DB_COUNT) 
 {                  
 curtab->tp_diffbuf[
 curtab->tp_diff_inv
 diff_redraw(TRUE); 
 }                  
              NORMAL`},
		{"Vyp",
			`  if (wp == NULL]   
 ▐if (wp == NULL]   
  {                 
    i = diff_buf_idx
    if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
              NORMAL`},
		{"/i =>",
			`  if (wp == NULL]   
  if (wp == NULL]   
  {                 
    ▐ = diff_buf_idx
    if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
     searching 'i ='`},
		{"dd",
			`  if (wp == NULL]   
  if (wp == NULL]   
  {                 
▐   if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
              NORMAL`},
		{"h",
			`  if (wp == NULL]   
  if (wp == NULL]   
  {                 
▐   if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
              NORMAL`},
		{"h",
			`  if (wp == NULL]   
  if (wp == NULL]   
  {                 
▐   if (i != DB_COUN
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
              NORMAL`},
		{"df=",
			`  if (wp == NULL]   
  if (wp == NULL]   
  {                 
▐DB_COUNT)          
    {               
    curtab->tp_diffb
    curtab->tp_diff_
    diff_redraw(TRUE
    }               
              NORMAL`},
		{"/i>kkFDcndi<ldw",
			`  if (wp == NULL]   
  if (wp == NULL]   
  {                 
di▐urtab->tp_diff_in
    diff_redraw(TRUE
    }               
  }                 
  }                 
  else              
              NORMAL`},
		{"gg",
			`▐                   
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"12gg",
			` * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
  int        i;     
                    
 ▐if (!win->w_p_diff
              NORMAL`},
		{"gg",
			`▐                   
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"jjyyp",
			`                    
/*                  
 * Check if the curr
▐* Check if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"lllcc *",
			`                    
/*                  
 * Check if the curr
 *▐                 
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
              INSERT`},
		{"<jllipotato<",
			`                    
/*                  
 * Check if the curr
 *                  
 * potat▐diff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"7h1j1l1k1h7l",
			`                    
/*                  
 * Check if the curr
 *                  
 * potat▐diff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"1g1g1g1gg",
			`▐                   
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"gg5gg",
			`                    
/*                  
 * Check if the curr
 *                  
▐* potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"8l3h",
			`                    
/*                  
 * Check if the curr
 *                  
 * po▐atodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"10000000000000000000000000000000000000000h",
			`                    
/*                  
 * Check if the curr
 *                  
▐* potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"3j",
			`                    
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
▐iff_buf_adjust(win_
{                   
              NORMAL`},
		{"8l",
			`                    
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
diff_buf▐adjust(win_
{                   
              NORMAL`},
		// 10j means move 10 rows down; not go to start of line (0) and move 1 row down
		{"10j",
			`  win_T  *wp;       
  int        i;     
                    
  if (!win->w_p_diff
  {                 
  /* When there is n
   * it from the dif
  FOR_ALL_WINDOWS(wp
    if (▐p->w_buffer
              NORMAL`},
		{"2k",
			`  win_T  *wp;       
  int        i;     
                    
  if (!win->w_p_diff
  {                 
  /* When there is n
   * it ▐rom the dif
  FOR_ALL_WINDOWS(wp
    if (wp->w_buffer
              NORMAL`},
		{"10k",
			` *▐                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
  int        i;     
                    
  if (!win->w_p_diff
  {                 
              NORMAL`},
		{"922337203685477580719973197k",
			`▐                   
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"jj",
			`                    
/*                  
 *▐Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"4\\$h", // '4' will be forgotten because of $ (go to last line char)
			`                    
                    
ved from the list ▐f
                    
                    
                    
                    
                    
                    
              NORMAL`},
		{"98765432123456789098765432100000000000000013414l",
			`                    
                    
ved from the list o▐
                    
                    
                    
                    
                    
                    
              NORMAL`},
		{"33<h",
			`                    
                    
ved from the list ▐f
                    
                    
                    
                    
                    
                    
              NORMAL`},
		{"6gg3h",
			`                    
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
▐*/                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"111<0gg",
			`▐                   
/*                  
 * Check if the curr
 *                  
 * potatodiff buffer
 */                 
  void              
diff_buf_adjust(win_
{                   
              NORMAL`},
		{"99999ggkk",
			`  {                 
dicurtab->tp_diff_in
    diff_redraw(TRUE
    }               
  }                 
  }                 
▐ else              
  diff_buf_add(win->
}                   
              NORMAL`},
		{"kkk3dd",
			`  {                 
dicurtab->tp_diff_in
    diff_redraw(TRUE
▐ diff_buf_add(win->
}                   
                    
                    
                    
                    
              NORMAL`},
		{"\\$",
			`                    
tp_diff_invalid = TR
edraw(TRUE);        
_add(win->w_buffer)▐
                    
                    
                    
                    
                    
              NORMAL`},
		{"\\$44\\^", // 44 should be ignored
			`{                   
curtab->tp_diff_inva
  diff_redraw(TRUE);
▐iff_buf_add(win->w_
                    
                    
                    
                    
                    
              NORMAL`},
		{"rpl",
			`{                   
curtab->tp_diff_inva
  diff_redraw(TRUE);
p▐ff_buf_add(win->w_
                    
                    
                    
                    
                    
              NORMAL`},
		{"Rabcdef<",
			`{                   
curtab->tp_diff_inva
  diff_redraw(TRUE);
pabcde▐f_add(win->w_
                    
                    
                    
                    
                    
              NORMAL`},
		{"?diff>",
			`{                   
curtab->tp_diff_inva
  ▐iff_redraw(TRUE);
pabcdeff_add(win->w_
                    
                    
                    
                    
                    
    searching 'diff'`},
		{"n",
			`{                   
curtab->tp_▐iff_inva
  diff_redraw(TRUE);
pabcdeff_add(win->w_
                    
                    
                    
                    
                    
    searching 'diff'`},
		{"N",
			`{                   
curtab->tp_diff_inva
  ▐iff_redraw(TRUE);
pabcdeff_add(win->w_
                    
                    
                    
                    
                    
    searching 'diff'`},
	}

	vi := setupViIntegration(t, snippet, 2)
	handlertest.TestHandlerSequence(t, vi, 20, 10, cases)
}

func TestViCount(t *testing.T) {
	motions := []rune{'h', 'j', 'k', 'l'}
	for _, motion := range motions {
		t.Run(fmt.Sprintf(
			"20%c multiplies motion by 20", // doesn't go necessarily to start of line
			motion),
			func(t *testing.T) {
				vi := setupVi(t, snippet, 2)
				vi.Resize(100, 100)
				vi.cursor.MoveToScroll(term.Coordinates{X: 3, Y: 3})
				vi.Handle(term.Event{Type: term.EventKey, Ch: '2'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: '0'})
				vi.Handle(term.Event{Type: term.EventKey, Ch: motion})
				switch motion {
				case 'h':
					assert.Equal(t, vi.cursor.Coordinates(), term.Coordinates{X: 0, Y: 3})
				case 'j':
					assert.Equal(t, vi.cursor.Coordinates(), term.Coordinates{X: 3, Y: 23})
				case 'k':
					assert.Equal(t, vi.cursor.Coordinates(), term.Coordinates{X: 0, Y: 0})
				case 'l':
					assert.Equal(t, vi.cursor.Coordinates(), term.Coordinates{X: 15, Y: 3})
				}
			})
	}
}

func TestVidd(t *testing.T) {
	suite := []struct {
		name             string
		moveCursorFn     func(*viHandlerImpl)
		content          string
		events           string
		expectContent    string
		expectCoords     term.Coordinates
		expectCoordsWrap term.Coordinates
	}{
		{
			name: "2dd",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveToScroll(term.Coordinates{Y: 2})
			},
			content:          "0000\n1111\n2222\n3333\n4444\n5555\n",
			events:           "2dd",
			expectContent:    "0000\n1111\n5555\n",
			expectCoords:     term.Coordinates{Y: 2},
			expectCoordsWrap: term.Coordinates{Y: 2},
		},
		{
			name: "3dd from last line",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()
			},
			content:          "0000\n1111\n2222\n3333\n4444\n5555\n",
			events:           "3dd",
			expectContent:    "0000\n1111\n2222\n3333\n4444\n5555\n",
			expectCoords:     term.Coordinates{Y: 6},
			expectCoordsWrap: term.Coordinates{Y: 6},
		},
		{
			name: "4dd from second to last line",
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()
				vi.cursor.MoveLineUp()
			},
			content:          "0000\n1111\n2222\n3333\n4444\n5555\n",
			events:           "3dd",
			expectContent:    "0000\n1111\n2222\n3333\n4444\n",
			expectCoords:     term.Coordinates{Y: 5},
			expectCoordsWrap: term.Coordinates{Y: 4},
		},
		{
			name:             "10dd from first line wipes all content",
			content:          "0000\n1111\n2222\n3333\n4444\n5555\n",
			events:           "10dd",
			expectContent:    "",
			expectCoords:     term.Coordinates{Y: 0},
			expectCoordsWrap: term.Coordinates{Y: 0},
		},
		{
			name:             "999999999999999999999999999999999999dd",
			content:          "0000\n1111\n2222\n3333\n4444",
			events:           "999999999999999999999999999999999999dd",
			expectContent:    "",
			expectCoords:     term.Coordinates{Y: 0},
			expectCoordsWrap: term.Coordinates{Y: 0},
		},
	}

	for _, tcase := range suite {
		for _, wrap := range []bool{false, true} {
			name := tcase.name
			if wrap {
				name += " (wrap)"
			}
			t.Run(name, func(t *testing.T) {
				vi := setupVi(t, tcase.content, 2, WithWrap(wrap))
				if wrap {
					vi.Resize(2, 10)
				} else {
					vi.Resize(10, 10)
				}
				vi.Draw(term.NoopWriter{})
				if tcase.moveCursorFn != nil {
					tcase.moveCursorFn(vi)
				}
				for _, eventChar := range tcase.events {
					vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
				}
				if wrap {
					assert.Equal(t, tcase.expectCoordsWrap, vi.cursor.Coordinates())
				} else {
					assert.Equal(t, tcase.expectCoords, vi.cursor.Coordinates())
				}
				vi.cursor.MoveFirstLine()
				vi.cursor.Select()
				vi.cursor.MoveLastLine()
				assert.Equal(t, tcase.expectContent, vi.cursor.Selection())
			})
		}
	}
}

func TestVidfd(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"jjdfd",
			`                    
/*                  
▐be added to or remo
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"jjcfc",
			`                    
/*                  
▐ if the current buf
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
	}

	newVi := func(t *testing.T) tui.Handler {
		return setupVi(t, snippet, 2)
	}
	handlertest.TestHandlerIsolated(t, newVi, 20, 10, cases)
}

func TestWrapMoveDownLastLogicalLine(t *testing.T) {
	sample := `abcde
fghih
ijklm
opkrs
tuvxy
11111
22222
33333
44444
55555
66666
`
	vi := setupVi(t, sample, 2, WithWrap(true))
	vi.Resize(3, 20)
	vi.Draw(term.NoopWriter{})
	for _, eventChar := range "jjjjjjjjjjjjjjjjjjjjj" {
		vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
	}
	assert.Equal(t, term.Coordinates{X: 3, Y: 10}, vi.cursor.ScrollCoordinates(vi.cursor.Coordinates()))
	vi.cursor.SelectLine()
	assert.Equal(t, "66666\n", vi.cursor.Selection())
}

func TestViDeleteAWord(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"jjjjjwdw",
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
 ▐                  
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"jjjjjwcw",
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  ▐                 
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
		{"jjjjjwce",
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  ▐                 
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
		{"jjjjjwecb",
			`                    
/*                  
 * Check if the curr
 * diff buffers.    
 */                 
  ▐                 
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
		{"jjjwwcw",
			`                    
/*                  
 * Check if the curr
 * ▐uffers.         
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
		{"jjjwwce",
			`                    
/*                  
 * Check if the curr
 * ▐buffers.        
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              INSERT`},
	}

	newVi := func(t *testing.T) tui.Handler {
		return setupVi(t, snippet, 2)
	}
	handlertest.TestHandlerIsolated(t, newVi, 20, 10, cases)
}

func TestViCursorIsolated(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"jjddp",
			`                    
/*                  
 * diff buffers.    
▐* Check if the curr
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		// insert block one rune (for now until repeater captures all insert)
		{"`jjjIh<",
			`h                   
h/*                 
h * Check if the cur
h▐* diff buffers.   
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		{"/C>/>",
			`                    
/*                  
 * ▐heck if the curr
 * diff buffers.    
 */                 
  void              
diff_buf_adjust(win_
{                   
  win_T  *wp;       
              NORMAL`},
		// TODO check yank paste after last line
		// the only thing from integration tests is that there's no
		// unix View that trims last EOL, this must in turn translatre in
		// some internal difference which renders this test failure
		/*{"Gyyp",
					`    curtab->tp_diff_
		    diff_redraw(TRUE
		    }
		  }
		  }
		  else
		  diff_buf_add(win->
		}
		▐
		              NORMAL`}, */
	}

	newVi := func(t *testing.T) tui.Handler {
		return setupViIntegration(t, snippet, 2)
	}
	handlertest.TestHandlerIsolated(t, newVi, 20, 10, cases)
}

func TestIntegrationScrollEvent(t *testing.T) {
	tsuite := []struct {
		desc      string
		cursorPos term.Coordinates
		ev        term.Event
	}{
		{"move to matching rune", term.Coordinates{Y: 7}, term.Event{Type: term.EventKey, Ch: '%'}},
		{"MoveEndLine", term.Coordinates{Y: 2}, term.Event{Type: term.EventKey, Ch: '$'}},
		{"MoveRightStartWord", term.Coordinates{X: 4, Y: 9}, term.Event{Type: term.EventKey, Ch: 'w'}},
		{"MoveLeftStartWord", term.Coordinates{X: 21, Y: 9}, term.Event{Type: term.EventKey, Ch: 'b'}},
		{"MoveLeftStartWordGroup", term.Coordinates{X: 21, Y: 9}, term.Event{Type: term.EventKey, Ch: 'B'}},
	}

	for _, tcase := range tsuite {
		tcase := tcase
		t.Run(tcase.desc, func(t *testing.T) {
			vi := setupVi(t, snippet, 2)
			vi.cursor.Insert('a')
			vi.setCursorAtScroll(tcase.cursorPos)
			vi.Resize(4, 4)

			var called int
			vi.less.Scroll().Subscribe(component.FuncScrollSubscriber(func(from, to term.Coordinates) {
				called++
			}))

			_, ok := vi.Handle(tcase.ev)
			assert.True(t, ok)
			assert.Equal(t, 1, called)
		})
	}
}

func TestIntegrationMoveWordSpecialChars(t *testing.T) {
	t.Run("navigating special chars MoveRightEndWord and MoveLeftStartWord", func(t *testing.T) {
		const snippetSpecialChars = `aaa.aaa,aaa:aaa;aaa aaa)aaa"aaa'aaa(aaa{aaa}aaa[aaa` +
			`]aaa	aaa\aaa/aaa+aaa_aaa@aaa#aaa=aaa<aaa>aaa!aaa?aaa|` +
			`aaa^aaa&aaa*aaa%aaa.aaa-`

		vi := setupVi(t, snippetSpecialChars, 2)
		prevCoords := term.Coordinates{X: 0, Y: 0}
		vi.setCursorAtScroll(prevCoords)
		vi.Resize(200, 30)
		jumps := []rune{
			'a', '.', 'a', ',', 'a', ':', 'a', ';', 'a', 'a', ')', 'a', '"',
			'a', '\'', 'a', '(', 'a', '{', 'a', '}', 'a', '[', 'a',
			']', 'a', 'a', '\\', 'a', '/', 'a', '+', 'a', 'a', '@', 'a', '#', 'a',
			'=', 'a', '<', 'a', '>', 'a', '!', 'a', '?', 'a', '|',
			'a', '^', 'a', '&', 'a', '*', 'a', '%', 'a', '.', 'a', '-',
		}

		// forward
		for i := 0; i < len(jumps); i++ {
			_, ok := vi.Handle(term.Event{Type: term.EventKey, Ch: 'e'})
			require.True(t, ok)
			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, jumps[i], cell.Ch, "(forward) expected '%c', got '%c'", jumps[i], cell.Ch)
		}

		// backwards
		for i := len(jumps) - 1; i >= 1; i-- {
			_, ok := vi.Handle(term.Event{Type: term.EventKey, Ch: 'b'})
			require.True(t, ok)
			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, jumps[i-1], cell.Ch, "(backwards) expected '%c', got '%c'", jumps[i-1], cell.Ch)
		}
	})
	t.Run("navigating new lines MoveRightEndWord and MoveLeftStartWord", func(t *testing.T) {
		t.Skip("Broken and to be fixed by OX-365")

		const snippetNewLines = `ab#cde
fghi.j
$klmno
pqr@st`

		vi := setupVi(t, snippetNewLines, 2)
		prevCoords := term.Coordinates{X: 0, Y: 0}
		vi.setCursorAtScroll(prevCoords)
		vi.Resize(10, 10)

		jumps := []rune{
			'b', '#', 'e',
			'f', 'i', '.', 'j',
			'$', 'o',
			'p', 'r', '@', 't', // FIXME: Instead of t gets p, to be fixed in OX-365
		}

		// forward
		for i := 0; i < len(jumps); i++ {
			_, ok := vi.Handle(term.Event{Type: term.EventKey, Ch: 'e'})
			require.True(t, ok)
			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, jumps[i], cell.Ch, "(forward) expected '%c', got '%c'", jumps[i], cell.Ch)
		}

		revJumps := []rune{
			's', '@', 'p',
			'o', '$',
			'j', '.', 'f',
			'e', 'c', '#', 'a',
		}

		// backwards
		for i := 0; i < len(revJumps); i++ {
			_, ok := vi.Handle(term.Event{Type: term.EventKey, Ch: 'b'})
			require.True(t, ok)
			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, revJumps[i], cell.Ch, "(backwards) [index %v] expected '%c', got '%c'", i, revJumps[i], cell.Ch)
		}
	})
}

func TestIntegrationNewFile(t *testing.T) {
	vi := setupVi(t, "", 2)
	vi.Resize(4, 4)
	for _, ch := range "ihello\nworld" {
		vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	assert.Equal(t, "hello\nworld", vi.less.Buffer().String())
}

func TestExitInsertMode(t *testing.T) {
	t.Run("escape and control-c exit insert mode into normal", func(t *testing.T) {
		vi := setupVi(t, "aaaa\nbbbb\ncccc\ndddd", 2)
		vi.Resize(4, 4)

		vi.setInsertMode()
		assert.Equal(t, vi.mode(), insertMode)

		vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
		assert.Equal(t, vi.mode(), normalMode)

		vi.setInsertMode()
		assert.Equal(t, vi.mode(), insertMode)

		vi.Handle(term.Event{Type: term.EventKey, Ch: 'c', Mod: term.ModCtrl})
		assert.Equal(t, vi.mode(), normalMode)
	})
}

func TestExitVisualMode(t *testing.T) {
	t.Run("escape and control-c exit visual mode into normal", func(t *testing.T) {
		vi := setupVi(t, "aaaa\nbbbb\ncccc\ndddd", 2)
		vi.Resize(4, 4)

		vi.setVisualMode()
		assert.Equal(t, vi.mode(), visualMode)

		vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
		assert.Equal(t, vi.mode(), normalMode)

		vi.setVisualMode()
		assert.Equal(t, vi.mode(), visualMode)

		vi.Handle(term.Event{Type: term.EventKey, Ch: 'c', Mod: term.ModCtrl})
		assert.Equal(t, vi.mode(), normalMode)
	})
}

func TestViCountChangeToVisualMode(t *testing.T) {
	codeSnippet := "abcdefghij\n1234567"
	// In a window width of 4 without wrapping::
	//
	//   abcd (efghij hidden right)
	//   1234 (567 hidden right)
	//
	// In a window width of 4 with wrapping::
	//
	//   abcd
	//   efgh
	//   ij
	//   1234
	//   567

	suite := []struct {
		name                   string
		scrollWidth            int
		cursorAt               term.Coordinates
		countDigits            []rune
		selection              string
		expectScrollCoords     term.Coordinates
		expectScrollCoordsWrap term.Coordinates
		expectWindowCoords     term.Coordinates
		expectWindowCoordsWrap term.Coordinates
	}{
		{
			name:                   "1v unitary selects only current char",
			scrollWidth:            4,
			cursorAt:               term.Coordinates{X: 1, Y: 0}, // 'b' in snippet
			countDigits:            []rune{'1'},
			selection:              "b",
			expectScrollCoords:     term.Coordinates{X: 1, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 1, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 1, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 1, Y: 0},
		},
		{
			name:                   "2v count N and change from normal to visual mode moves cursor N cells right",
			scrollWidth:            4,
			cursorAt:               term.Coordinates{X: 1, Y: 0}, // 'b' in snippet
			countDigits:            []rune{'2'},
			selection:              "bc",
			expectScrollCoords:     term.Coordinates{X: 2, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 2, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 2, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 2, Y: 0},
		},
		{
			name:        "11v narrow scroll count N and change from normal to visual exceeds line end but not entire content",
			scrollWidth: 4,
			cursorAt:    term.Coordinates{X: 2, Y: 0}, // 'c' in snippet
			countDigits: []rune{'1', '5'},
			selection:   "cdefghij", // does not beyond code line
			// cursor will be at last char if it doesn't have room rightwards (last char index: 9)
			expectScrollCoords:     term.Coordinates{X: 9, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 10, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 3, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 2, Y: 2},
		},
		{
			name:        "11v wide scroll count N and change from normal to visual exceeds line end but not entire content",
			scrollWidth: 100,
			cursorAt:    term.Coordinates{X: 2, Y: 0}, // 'c' in snippet
			countDigits: []rune{'1', '5'},
			selection:   "cdefghij", // does not beyond code line
			// cursor will be at last char if it doesn't have room rightwards (last char index: 9)
			expectScrollCoords:     term.Coordinates{X: 10, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 10, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 10, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 10, Y: 0},
		},
		{
			name:                   "3v count N and change from normal to visual in last column wraps row below",
			scrollWidth:            4,
			cursorAt:               term.Coordinates{X: 3, Y: 0}, // 'd' in snippet
			countDigits:            []rune{'3'},
			selection:              "def",
			expectScrollCoords:     term.Coordinates{X: 5, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 5, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 3, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 1, Y: 1},
		},
		{
			name:        "99999999999999999999v narrow scroll count N and change from normal to visual",
			scrollWidth: 4,
			cursorAt:    term.Coordinates{X: 3, Y: 0}, // 'd' in snippet
			countDigits: []rune{'9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9'},
			selection:   "defghij",
			// cursor will be at last char if it doesn't have room rightwards (last char index: 9)
			expectScrollCoords:     term.Coordinates{X: 9, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 10, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 3, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 2, Y: 2},
		},
		{
			name:        "99999999999999999999v wide scroll count N and change from normal to visual",
			scrollWidth: 100,
			cursorAt:    term.Coordinates{X: 3, Y: 0}, // 'd' in snippet
			countDigits: []rune{'9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9', '9'},
			selection:   "defghij",
			// cursor will be at last char if it doesn't have room rightwards (last char index: 9)
			expectScrollCoords:     term.Coordinates{X: 10, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 10, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 10, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 10, Y: 0},
		},
		{
			name:                   "stay within code line",
			scrollWidth:            4,
			cursorAt:               term.Coordinates{X: 0, Y: 0}, // 'a' in snippet
			countDigits:            []rune{'1', '0'},
			selection:              "abcdefghij",
			expectScrollCoords:     term.Coordinates{X: 9, Y: 0},
			expectScrollCoordsWrap: term.Coordinates{X: 9, Y: 0},
			expectWindowCoords:     term.Coordinates{X: 3, Y: 0},
			expectWindowCoordsWrap: term.Coordinates{X: 1, Y: 2},
		},
	}

	for _, tcase := range suite {
		for _, wrap := range []bool{false, true} {
			name := tcase.name
			if wrap {
				name += " (wrap)"
			}

			t.Run(name, func(t *testing.T) {
				vi := setupVi(t, codeSnippet, 2, WithWrap(wrap))
				vi.Resize(tcase.scrollWidth, 20)
				// In order to create the wraps a Draw  must be issued so [scroll.Draw]
				// can create them, otherwise it's the same as passing WithWrap(false).
				vi.Draw(term.NoopWriter{})

				vi.cursor.MoveToScroll(tcase.cursorAt)

				for _, countDigit := range tcase.countDigits {
					vi.Handle(term.Event{Type: term.EventKey, Ch: countDigit})
				}
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'v'})

				windowCoords := vi.cursor.Coordinates()
				scrollCoords := vi.cursor.ScrollCoordinates(windowCoords)

				if wrap {
					assert.Equal(t, tcase.expectScrollCoordsWrap,
						scrollCoords, "wrong scroll coords (wrap)")
					assert.Equal(t, tcase.expectWindowCoordsWrap,
						windowCoords, "wrong window coords (wrap)")
				} else {
					assert.Equal(t, tcase.expectScrollCoords,
						scrollCoords, "wrong scroll coords")
					assert.Equal(t, tcase.expectWindowCoords,
						windowCoords, "wrong window coords")
				}
			})
		}
	}
}

func TestViCountChangeToLineVisualMode(t *testing.T) {
	codeSnippet := "abc\n123\ndef\n456\nghij\n7891"
	suite := []struct {
		name        string
		scrollWidth int
		cursorAt    term.Coordinates
		countDigits []rune
		selection   string
	}{
		{
			name:        "1V unitary selects current line",
			scrollWidth: 2,
			cursorAt:    term.Coordinates{X: 1, Y: 0}, // 'b' in snippet
			countDigits: []rune{'1'},
			selection:   "abc\n",
		},
		{
			name:        "2V unitary selects current line and line below",
			scrollWidth: 2,
			cursorAt:    term.Coordinates{X: 1, Y: 0}, // 'b' in snippet
			countDigits: []rune{'2'},
			selection:   "abc\n123\n",
		},
		{
			name:        "999V content overflow",
			scrollWidth: 2,
			cursorAt:    term.Coordinates{X: 1, Y: 3}, // '5' in snippet
			countDigits: []rune{'9', '9', '9'},
			selection:   "456\nghij\n7891\n",
		},
	}

	for _, tcase := range suite {
		for _, wrap := range []bool{true, false} {
			name := tcase.name
			if wrap {
				name += " (wrap)"
			}

			t.Run(name, func(t *testing.T) {
				vi := setupVi(t, codeSnippet, 2, WithWrap(wrap))
				vi.Resize(tcase.scrollWidth, 4)
				vi.cursor.MoveToScroll(tcase.cursorAt)

				for _, countDigit := range tcase.countDigits {
					vi.Handle(term.Event{Type: term.EventKey, Ch: countDigit})
				}
				vi.Handle(term.Event{Type: term.EventKey, Ch: 'V'})
				assert.Equal(t, tcase.selection, vi.cursor.Selection())
			})
		}
	}
}

func TestVigg(t *testing.T) {
	code := `11111111111
222222
333333333333

 5
6
7777777777777777777777777777777777777777777777777777777777777777777777777777777777777777777
   88888888
9999
11111111
  222222222222222
3333333
   4444444444444444
`
	codeLong := code
	for i := 0; i < 200; i++ {
		codeLong += code
	}

	suite := []struct {
		name         string
		fileText     string
		narrowWrap   bool // scroll width of 4 with wrapping enaled
		moveCursorFn func(*viHandlerImpl)
		events       string
		expectCoord  term.Coordinates
	}{
		{
			name:        "gg from 0,0",
			fileText:    code,
			narrowWrap:  true,
			events:      "gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:         "gg from 1,0",
			fileText:     code,
			narrowWrap:   true,
			moveCursorFn: func(vi *viHandlerImpl) { vi.cursor.MoveRight() },
			events:       "gg",
			expectCoord:  term.Coordinates{X: 0, Y: 0},
		},
		{
			name:       "6gg from start of wrapped line",
			fileText:   code,
			narrowWrap: true,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveDown()
				vi.cursor.MoveRightColumns(2)
			},
			events:      "6gg",
			expectCoord: term.Coordinates{X: 0, Y: 7}, // lonely 6 in `code`
		},
		{
			name:       "6gg from middle of wrapped line",
			fileText:   code,
			narrowWrap: true,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveDown()
				vi.cursor.MoveRightColumns(2)
			},
			events:      "6gg",
			expectCoord: term.Coordinates{X: 0, Y: 7}, // lonely 6 in `code`
		},
		{
			name:       "6gg from end of wrapped line",
			fileText:   code,
			narrowWrap: true,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveDown()
				vi.cursor.MoveEndLine()
			},
			events:      "6gg",
			expectCoord: term.Coordinates{X: 0, Y: 7}, // lonely 6 in `code`
		},
		{
			name:       "gg to same line called from wrapped lines below brings cursor to start of line",
			fileText:   code,
			narrowWrap: true,
			events:     "3gg",
			moveCursorFn: func(vi *viHandlerImpl) {
				// Move to wrapped line of 333s in `code`.
				vi.cursor.MoveDownLines(7)
				vi.cursor.MoveRightColumns(2)
			},
			expectCoord: term.Coordinates{X: 0, Y: 5}, // beginning of 333s
		},
		{
			name:     "gg from end of long file",
			fileText: codeLong,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()

			},
			events:      "gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:     "gg from middle of long file",
			fileText: codeLong,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()
				scrollCoords := vi.cursor.ScrollCoordinates(vi.cursor.Coordinates())
				vi.cursor.MoveFirstLine()
				vi.cursor.MoveDownLines(scrollCoords.Y / 2)

			},
			events:      "gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:        "999gg beyond limits of file",
			fileText:    code,
			events:      "999gg",
			expectCoord: term.Coordinates{X: 0, Y: 8},
		},
		{
			name:     "0g",
			fileText: code,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()
			},
			events:      "0gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:     "00g",
			fileText: code,
			moveCursorFn: func(vi *viHandlerImpl) {
				vi.cursor.MoveLastLine()
			},
			events:      "00gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:        "3gg in single char file",
			fileText:    "a",
			events:      "3gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:        "3gg in empty file",
			fileText:    "",
			events:      "3gg",
			expectCoord: term.Coordinates{X: 0, Y: 0},
		},
		{
			name:        "3gg in file that's only new lines",
			fileText:    "\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n",
			events:      "3gg",
			expectCoord: term.Coordinates{X: 0, Y: 2},
		},
		{
			name:        "9999999999999999999999999999999999999999999999999999gg",
			fileText:    code,
			events:      "9999999999999999999999999999999999999999999999999999gg",
			expectCoord: term.Coordinates{X: 0, Y: 8},
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, tcase.fileText, 2, WithWrap(tcase.narrowWrap))
			scrollWidth := 100
			if tcase.narrowWrap {
				scrollWidth = 4
			}
			vi.Resize(scrollWidth, 10)
			vi.Draw(term.NoopWriter{})
			if tcase.moveCursorFn != nil {
				tcase.moveCursorFn(vi)
			}
			for _, eventChar := range tcase.events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
			}
			assert.Equal(t, tcase.expectCoord, vi.cursor.Coordinates())
		})
	}
}

func TestViggBeyondContent(t *testing.T) {
	t.Run("discrepancy between visual cursor and actual cursor", func(t *testing.T) {
		fileText := "a\nb\nc\nd"
		vi := setupVi(t, fileText, 2)
		vi.Resize(100, 30)
		vi.Draw(term.NoopWriter{})
		for _, eventChar := range "999gg" {
			vi.Handle(term.Event{Type: term.EventKey, Ch: eventChar})
		}
		assert.Equal(t, term.Coordinates{X: 0, Y: 3}, vi.cursor.Coordinates())
		vi.cursor.MoveUp()
		assert.Equal(t, term.Coordinates{X: 0, Y: 2}, vi.cursor.Coordinates())

	})
}

func TestSetCursorAtScrollBounds(t *testing.T) {
	fileText := "123\n456\n789\nd"
	vi := setupVi(t, fileText, 2)
	vi.Resize(8, 8)

	t.Run("vertical bounds", func(t *testing.T) {
		vi.setCursorAtScroll(term.Coordinates{X: 0, Y: 999})
		assert.Equal(t, term.Coordinates{X: 0, Y: 4}, vi.cursor.Coordinates())
	})
}

func TestResetCount(t *testing.T) {
	t.Run("numbers are accumulated into vi counter", func(t *testing.T) {
		code := ""
		for i := 0; i < 45; i++ {
			code += fmt.Sprintf("%v\n", i)
		}

		vi := setupVi(t, code, 2)
		vi.Resize(5, 50)

		vi.Handle(term.Event{Type: term.EventKey, Ch: '1'})
		vi.Handle(term.Event{Type: term.EventKey, Ch: '1'})
		vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})

		assert.Equal(t, vi.cursor.Coordinates().Y, 11)
	})
	t.Run("when parsing numbers and the parsed int exceeds MaxInt count should become MaxInt and not be reset", func(t *testing.T) {
		vi := setupVi(t, "aaa\nbbb\nccc\nddd", 2)
		vi.Resize(10, 10)

		maxIntStr := strconv.Itoa(math.MaxInt)

		for _, maxIntChar := range maxIntStr {
			vi.Handle(term.Event{Type: term.EventKey, Ch: maxIntChar})
		}
		vi.Handle(term.Event{Type: term.EventKey, Ch: '9'})
		vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})

		assert.Equal(t, vi.cursor.Coordinates().Y, 3)
	})

	t.Run("single motion on vi initialized with scroll", func(t *testing.T) {
		vi := setupViWithScroll(t, "aaa\nbbb\nccc\nddd", 2)
		vi.Resize(10, 10)
		vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
		assert.Equal(t, 1, vi.cursor.Coordinates().Y)
	})
}

func TestNoModeHandlesNonCtrlModifiers(t *testing.T) {
	vi := setupVi(t, "a", 2)
	vi.Resize(4, 4)

	modes := []viMode{
		normalMode,
		insertMode,
		deleteMode,
		gMode,
		foldMode,
		yankMode,
		visualMode,
		visualLineMode,
		visualBlockMode,
		replaceMode,
		replaceOneMode,
		searchMode,
	}
	modifiers := []term.Modifier{
		term.ModAlt, term.ModShift, term.ModMeta,
		term.ModCtrlShift, term.ModCtrlAlt, term.ModCtrlMeta,
		term.ModCtrlShiftAlt, term.ModCtrlShiftMeta, term.ModCtrlAltMeta,
		term.ModShiftMeta, term.ModAltMeta, term.ModAltShiftMeta,
		term.ModAltShift,
	}

	for _, mod := range modifiers {
		for _, mode := range modes {
			t.Run(fmt.Sprintf("handle %v in %v", mod, mode), func(t *testing.T) {
				vi.currMode = mode
				exit, handled := vi.Handle(term.Event{Type: term.EventKey, Mod: mod, Ch: 'a'})
				assert.False(t, exit)
				assert.False(t, handled)
			})
		}
	}
}

func TestCursorOutOfBounds(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		for _, contentWindowOverflow := range []bool{false, true} {
			name := "cursor go beyond rows and move one up"
			if wrap {
				name += " (wrap)"
			}
			if contentWindowOverflow {
				name += " (content overflow)"
			}

			t.Run(name, func(t *testing.T) {
				vi := setupVi(t, "aaaa\nbbbb\ncccc\ndddd\neeee", 2, WithWrap(wrap))

				windowWidth := 2
				if contentWindowOverflow {
					vi.Resize(windowWidth, 3)
				} else {
					vi.Resize(windowWidth, 10)
				}

				beyondRowsCoords := term.Coordinates{Y: 99}
				ok := vi.setCursorAtScroll(beyondRowsCoords)
				require.True(t, ok)

				scrollCoords := vi.cursor.ScrollCoordinates(vi.cursor.Coordinates())
				assert.Equal(t, term.Coordinates{X: 0, Y: 5}, scrollCoords)

				vi.Handle(term.Event{Type: term.EventKey, Ch: 'k'})
				scrollCoords = vi.cursor.ScrollCoordinates(vi.cursor.Coordinates())
				assert.Equal(t, term.Coordinates{X: 0, Y: 4}, scrollCoords,
					"the cursor is trapped at the last line")
			})
		}
	}
}

func TestMoveCursorArrowKeys(t *testing.T) {
	t.Run("arrow keys can be used to move cursor", func(t *testing.T) {
		vi := setupVi(t, "aaaa\nbbbb\ncccc\ndddd", 2)
		vi.Resize(4, 4)

		modes := []viMode{
			normalMode,
			insertMode,
			visualMode,
		}

		for _, mod := range modes {
			vi.currMode = mod

			coords, _, _ := vi.Cursor()
			require.Equal(t, 0, coords.X)
			require.Equal(t, 0, coords.Y)

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
			coords, _, _ = vi.Cursor()
			require.Equal(t, 1, coords.X)
			require.Equal(t, 0, coords.Y)

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
			coords, _, _ = vi.Cursor()
			require.Equal(t, 1, coords.X)
			require.Equal(t, 1, coords.Y)

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowLeft})
			coords, _, _ = vi.Cursor()
			require.Equal(t, 0, coords.X)
			require.Equal(t, 1, coords.Y)

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowUp})
			coords, _, _ = vi.Cursor()
			require.Equal(t, 0, coords.X)
			require.Equal(t, 0, coords.Y)
		}

	})
}

func TestSetNormalModeClearing(t *testing.T) {
	vi := setupVi(t, "aaaa\nbbbb\ncccc\ndddd", 2)
	vi.Resize(4, 4)

	vi.Handle(term.Event{Type: term.EventKey, Ch: '/'})
	assert.Equal(t, searchMode, vi.mode())

	vi.setNormalMode()
	assert.Equal(t, normalMode, vi.mode())

	assert.Equal(t, handler.LessNormalMode, vi.less.Mode())
}

func setupVi(
	t *testing.T, text string, tabspaces int, opts ...Option,
) *viHandlerImpl {
	buf := cell.NewBuffer()
	buf.InitWithTabspaces(tabspaces)
	_, err := buf.ReadFrom(strings.NewReader(text))
	require.NoError(t, err)

	vi := new(viHandlerImpl)
	vi.init(buf, opts...)

	return vi
}

func setupViWithScroll(
	t *testing.T, text string, tabspaces int, opts ...Option,
) *viHandlerImpl {
	buf := cell.NewBuffer()
	buf.InitWithTabspaces(tabspaces)
	_, err := buf.ReadFrom(strings.NewReader(text))
	require.NoError(t, err)

	scroll := component.NewScroll(buf)

	vi := new(viHandlerImpl)
	vi.initWithScroll(scroll, opts...)

	return vi
}

func setupViIntegration(
	t *testing.T, text string, tabspaces int, opts ...Option,
) tui.Handler {
	buf := cell.NewBuffer()
	buf.InitWithTabspaces(tabspaces)
	_, err := buf.ReadFrom(strings.NewReader(text))
	require.NoError(t, err)

	vi := New(buf, uri, opts...)

	return vi
}

func TestPasteVisualMode(t *testing.T) {
	name := "standard select"
	for _, wrap := range []bool{false, true} {
		if wrap {
			name += " (wrap)"
		}

		t.Run(name, func(t *testing.T) {
			vi := setupVi(t, "ABC0123456789", 2)
			if wrap {
				vi.Resize(4, 10)
			} else {
				vi.Resize(10, 10)
			}

			events := "vllyvlld"
			for _, event := range events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			}

			require.Equal(t, "0123456789", vi.less.Buffer().String())
			paste, err := vi.config.clipboard.Paste(vi.config.defaultRegister)
			require.NoError(t, err)
			require.Equal(t, "ABC", paste.Text)

			events = "lllvll"
			for _, event := range events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			}
			require.Equal(t, "345", vi.cursor.Selection())

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'p'})

			// after pasting in visual mode vim goes to normal mode again
			assert.Equal(t, normalMode, vi.mode())

			assert.Equal(t, "012ABC6789", vi.less.Buffer().String())

			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, 'A', cell.Ch,
				"current cell char is not 'A' but '%c'", cell.Ch)
		})
	}

	name = "line select"
	for _, wrap := range []bool{false, true} {
		if wrap {
			name += " (wrap)"
		}

		t.Run(name, func(t *testing.T) {
			vi := setupVi(t, "ABC0123456\n789\n", 2)
			if wrap {
				vi.Resize(4, 10)
			} else {
				vi.Resize(10, 10)
			}

			events := "vllyvlld"
			for _, event := range events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			}

			require.Equal(t, "0123456\n789\n", vi.less.Buffer().String())
			paste, err := vi.config.clipboard.Paste(vi.config.defaultRegister)
			require.NoError(t, err)
			require.Equal(t, "ABC", paste.Text)

			events = "V"
			for _, event := range events {
				vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			}

			if wrap {
				// FIXME: Is "0123456\n"
				// At the moment Cursor.SelectLine does not honor wrap lines and honors
				// logical lines. This will be changed by PR #127 "Add directional vi
				// (d)elete and (y)ank  (OX-222).
				// require.Equal(t, "0123", vi.cursor.Selection())
			} else {
				require.Equal(t, "0123456\n", vi.cursor.Selection())
			}

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'p'})

			// after pasting in visual mode vim goes to normal mode again
			assert.Equal(t, normalMode, vi.mode())

			if wrap {
				// FIXME: Is "ABC789\n"
				// At the moment Cursor.SelectLine does not honor wrap lines and honors
				// logical lines. This will be changed by PR #127 "Add directional vi
				// (d)elete and (y)ank  (OX-222).
				//assert.Equal(t, "ABC456\n789\n", vi.less.Buffer().String())
			} else {
				assert.Equal(t, "ABC\n789\n", vi.less.Buffer().String())
			}

			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, 'A', cell.Ch,
				"current cell char is not 'A' but '%c'", cell.Ch)
		})
	}

	name = "block select"
	for _, wrap := range []bool{false, true} {
		if wrap {
			name += " (wrap)"
		}

		t.Run(name, func(t *testing.T) {
			vi := setupVi(t, "ABC\nDEF\n0123456\n789\n", 2)
			if wrap {
				vi.Resize(4, 10)
			} else {
				vi.Resize(10, 10)
			}

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'v', Mod: term.ModCtrl})
			for _, event := range "lljyVjd" {
				vi.Handle(term.Event{Type: term.EventKey, Ch: event})
			}

			require.Equal(t, "0123456\n789\n", vi.less.Buffer().String())
			paste, err := vi.config.clipboard.Paste(vi.config.defaultRegister)
			require.NoError(t, err)
			require.Equal(t, "ABC\nDEF", paste.Text)

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
			vi.Handle(term.Event{Type: term.EventKey, Ch: 'v', Mod: term.ModCtrl})
			vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})

			if wrap {
				require.Equal(t, "1\n8", vi.cursor.Selection())
			} else {
				require.Equal(t, "1\n8", vi.cursor.Selection())
			}

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'p'})

			if wrap {
				assert.Equal(t, "0ABC23456\n7DEF9\n", vi.less.Buffer().String())
			} else {
				assert.Equal(t, "0ABC23456\n7DEF9\n", vi.less.Buffer().String())
			}

			cell, ok := vi.cursor.Cell()
			require.True(t, ok)
			require.Equal(t, 'A', cell.Ch,
				"current cell char is not 'A' but '%c'", cell.Ch)
		})
	}
}

func TestVisualBlockInsert(t *testing.T) {
	fileContent := "aaaaaa\nbbbbbb\ncccccc\ndddddd"

	suite := []struct {
		name          string
		moveCursorFn  func(*viHandlerImpl)
		inputSequence string
		expect        string
	}{
		{
			name:          "no backspace",
			inputSequence: "01234",
			expect:        "01234aaaaaa\n01234bbbbbb\n01234cccccc\ndddddd",
		},
		{
			name:          "backspace",
			inputSequence: "01234^xy",
			expect:        "0123xyaaaaaa\n0123xybbbbbb\n0123xycccccc\ndddddd",
		},
		{
			name:          "many backspace",
			inputSequence: "01234^^^xy",
			expect:        "01xyaaaaaa\n01xybbbbbb\n01xycccccc\ndddddd",
		},
		{
			name:          "more backspaces than characters in row",
			inputSequence: "ABC^^^^^^^",
			expect:        "aaaaaa\nbbbbbb\ncccccc\ndddddd",
		},
		{
			name:          "backspace only",
			inputSequence: "^",
			expect:        "aaaaaa\nbbbbbb\ncccccc\ndddddd",
		},
		{
			name:          "many backspace only",
			inputSequence: "^^^",
			expect:        "aaaaaa\nbbbbbb\ncccccc\ndddddd",
		},
	}

	for _, tcase := range suite {
		t.Run(tcase.name, func(t *testing.T) {
			vi := setupVi(t, fileContent, 2)
			vi.Resize(4, 4)

			if tcase.moveCursorFn != nil {
				tcase.moveCursorFn(vi)
			}

			vi.Handle(term.Event{Type: term.EventKey, Ch: 'v', Mod: term.ModCtrl})
			vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
			vi.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
			vi.Handle(term.Event{Type: term.EventKey, Ch: 'I'})

			for _, ch := range tcase.inputSequence {
				// following same convetions as handler.handlertest.SequenceTestCase
				if ch == '^' {
					vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyBackspace})
				} else {
					vi.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}

			}

			vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
			assert.Equal(t, tcase.expect,
				vi.less.Buffer().String())

		})
	}
}
