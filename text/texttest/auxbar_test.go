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

package texttest

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/comptest"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

func TestAuxBarDraw(t *testing.T) {
	buf := cell.NewBuffer()
	buf.WriteString(copy)
	fs := &testFoldsService{}
	fs.view = buf.WithView(fs)
	scroll := component.NewScroll(buf)
	h := newtestHandler(scroll)
	var wg sync.WaitGroup
	cb := func(fn func()) bool {
		fn()
		wg.Done()
		return true
	}

	wg.Add(1)
	bar := text.WithAuxBar(buf, scroll, h, true /*folds enabled*/, cb)
	bar.Resize(20, 10)
	w := term.NewStringWriter(20, 10)

	tests := []comptest.TestCase{
		{Expected: `
  package main      
                    
 import (          
      "fmt"         
                    
      "github.com/un
  )                 
                    
 func main() {     
      fmt.Println("%`,
		},
	}
	wg.Wait()
	comptest.TestComponent(t, bar, w, tests)

	require.True(t, scroll.SeekDown())

	tests = []comptest.TestCase{
		{Expected: `
                    
 import (          
      "fmt"         
                    
      "github.com/un
  )                 
                    
 func main() {     
      fmt.Println("%
      for i := 0; i `,
		},
	}
	comptest.TestComponent(t, bar, w, tests)

	wg.Add(1)
	_, handled := bar.Handle(
		term.Event{Type: term.EventMouse, Key: term.MouseLeft, MouseX: 0, MouseY: 1})
	require.True(t, handled)

	tests = []comptest.TestCase{
		{Expected: `
                    
 import ( [5 lines]
                    
 func main() {     
      fmt.Println("%
      for i := 0; i 
          fmt.Printl
      }             
  }                 
                    `,
		},
	}

	wg.Wait()
	comptest.TestComponent(t, bar, w, tests)

	wg.Add(1)
	_, handled = bar.Handle(
		term.Event{Type: term.EventMouse, Key: term.MouseLeft, MouseX: 0, MouseY: 3})
	require.True(t, handled)

	tests = []comptest.TestCase{
		{Expected: `
                    
 import ( [5 lines]
                    
 func main() { [6 l
                    
  const fileContent 
      "import (\n"+ 
      "\"fmt\"\n"+  
      "\n"+         
      "\"github.com/`,
		},
	}

	wg.Wait()
	comptest.TestComponent(t, bar, w, tests)
	
	wg.Add(1)
	_, handled = bar.Handle(
		term.Event{Type: term.EventMouse, Key: term.MouseLeft, MouseX: 0, MouseY: 1})
	require.True(t, handled)

	tests = []comptest.TestCase{
		{Expected: `
                    
 import (          
      "fmt"         
                    
      "github.com/un
  )                 
                    
 func main() { [6 l
                    
  const fileContent `,
		},
	}

	wg.Wait()
	comptest.TestComponent(t, bar, w, tests)
	
	// nothing happens if we click outside of line
	_, handled = bar.Handle(
		term.Event{Type: term.EventMouse, Key: term.MouseLeft, MouseX: 1, MouseY: 1})
	require.True(t, handled)

	tests = []comptest.TestCase{
		{Expected: `
                    
 import (          
      "fmt"         
                    
      "github.com/un
  )                 
                    
 func main() { [6 l
                    
  const fileContent `,
		},
	}

	wg.Wait()
	comptest.TestComponent(t, bar, w, tests)
}

var _ = (foldsService)(testFoldsService{})

type foldsService interface {
	Folds() (iterator.Iterator[term.Range], bool)
}

type testFoldsService struct {
	view cell.View
}

func (f testFoldsService) Rows() int {
	return f.view.Rows()
}

func (f testFoldsService) Columns(row int) int {
	return f.view.Columns(row)
}

func (f testFoldsService) Cell(at term.Coordinates) (term.Cell, bool) {
	return f.view.Cell(at)
}

func (f testFoldsService) RawCells() [][]term.Cell {
	return f.view.RawCells()
}

func (f testFoldsService) String() string {
	return f.view.String()
}

func (f testFoldsService) Folds() (iterator.Iterator[term.Range], bool) {
	return iterator.FromSlice([]term.Range{
		{Start: term.Coordinates{Y: 2, X: 0}, End: term.Coordinates{Y: 6}},
		{Start: term.Coordinates{Y: 8}, End: term.Coordinates{Y: 13}},
		{Start: term.Coordinates{Y: 8, X: 12}, End: term.Coordinates{Y: 13}},
	}), true
}

type testHandler struct {
	*component.Scroll
	URI workspaceapi.URI
}

func newtestHandler(scroll *component.Scroll) (t *testHandler) {
	t = new(testHandler)
	t.Scroll = scroll
	return t
}

func (t *testHandler) Resource() workspaceapi.URI {
	return t.URI
}

func (t *testHandler) SetWrap(wrap bool) {
}

func (t *testHandler) ShowCommandBar(show bool) {
}

func (t *testHandler) SetCursorAtScroll(term.Coordinates) bool {
	return false
}

func (t *testHandler) Close() error {
	return nil
}

func (t *testHandler) SeekUp() bool {
	return false
}

func (t *testHandler) SeekDown() bool {
	return false
}

func (t *testHandler) SeekOffset() int {
	return 0
}

func (t *testHandler) MaxSeekOffset() int {
	return 0
}

func (t *testHandler) Handle(ev term.Event) (bool, bool) {
	return false, true
}

func (t *testHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, 0, false
}

func (t *testHandler) Selection() (string, bool) {
	return "", false
}

func (t *testHandler) Man() tui.Manual {
	return tui.Manual{}
}

const copy = `package main

import (
	"fmt"

	"github.com/unstablebuild/blue/cli"
)

func main() {
	fmt.Println("%+v", cli.NewCLI)
	for i := 0; i < 10; i++ {
		fmt.Println("%d", i)
	}
}

const fileContent = "package main\n" +
	"import (\n"+
	"\"fmt\"\n"+
	"\n"+
	"\"github.com/unstablebuild/blue/cli\"\n"
	")"
`
