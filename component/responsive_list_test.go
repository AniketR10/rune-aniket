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

package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

type responsiveTestList struct {
	elementHeight int
	*ResponsiveList
}

type testListResponsive struct {
	tui.Component
	height *int
}

func (t *testListResponsive) Height(width int) int {
	return *t.height
}

func (l *responsiveTestList) newTestResponsive(c tui.Component) Responsive {
	// emulate List behaviour, and test max width default
	v := &testListResponsive{Component: c, height: &l.elementHeight}
	return v
}

func (l *responsiveTestList) PushBackList(other testList) {
	l.ResponsiveList.PushBackList(other.(*responsiveTestList).ResponsiveList)
}
func (l *responsiveTestList) PushFrontList(other testList) {
	l.ResponsiveList.PushFrontList(other.(*responsiveTestList).ResponsiveList)
}

func (l *responsiveTestList) PushBack(c tui.Component) ListNode {
	return l.ResponsiveList.PushBack(l.newTestResponsive(c))
}

func (l *responsiveTestList) PushFront(c tui.Component) ListNode {
	return l.ResponsiveList.PushFront(l.newTestResponsive(c))
}

func (l *responsiveTestList) Remove(e *ListNode) tui.Component {
	return l.ResponsiveList.Remove(e).(*testListResponsive).Component
}

func (l *responsiveTestList) Sort(less func(a, b tui.Component) bool) {
	l.ResponsiveList.Sort(func(a, b Responsive) bool {
		return less(a.(*testListResponsive).Component, b.(*testListResponsive).Component)
	})
}

func (l *responsiveTestList) SetElementHeight(i int) {
	// setting element height from here bypassing ResponsiveList
	// allows for all test Responsive components to be resized
	// and emulate SetElementHeight behaviour.
	l.elementHeight = i
	l.Resize(l.list.width, l.list.height)
}

func (l *responsiveTestList) ElementHeight() int {
	return l.elementHeight
}

func newResponsiveTestList(i int) testList {
	ret := &responsiveTestList{ResponsiveList: NewResponsiveList()}
	ret.elementHeight = i
	return ret
}

func TestResponsiveListSort(t *testing.T) {
	testListSort(t, newResponsiveTestList)
}

func TestResponsiveListFrontBack(t *testing.T) {
	testFrontBack(t, newResponsiveTestList)
}

func TestResponsiveListListRemove(t *testing.T) {
	testListRemove(t, newResponsiveTestList)
}

func TestResponsiveListEmptyDraw(t *testing.T) {
	testEmptyListDraw(t, newResponsiveTestList)
}

func TestResponsiveListResponsiveness(t *testing.T) {
	l := NewResponsiveList()
	l.Resize(8, 4)

	w := term.NewStringWriter(8, 4)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
        
        
        
        `,
		}, {
			func() {
				l.PushBack(&TestResponsive{
					WantHeight: 2,
					TestComponent: TestComponent{
						Ch: 'X',
					},
				},
				)
			}, `
XXXXXXXX
XXXXXXXX
        
        `,
		}, {
			func() {
				l.PushBack(&TestResponsive{
					WantHeight: 2,
					TestComponent: TestComponent{
						Ch: 'Y',
					},
				},
				)
			}, `
XXXXXXXX
XXXXXXXX
YYYYYYYY
YYYYYYYY`,
		}, {
			func() {
				l.PushBack(&TestResponsive{
					WantHeight: 3,
					TestComponent: TestComponent{
						Ch: 'Z',
					},
				},
				)
				l.SeekEnd()
			}, `
YYYYYYYY
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ`,
		}, {
			func() {
				el, ok := l.ElementAt(term.Coordinates{})
				require.True(t, ok)
				v := el.Value().(*TestResponsive)
				assert.Equal(t, 'Y', v.Ch)

				el, ok = l.ElementAt(term.Coordinates{Y: 1})
				require.True(t, ok)
				v = el.Value().(*TestResponsive)
				assert.Equal(t, 'Z', v.Ch)

				el, ok = l.ElementAt(term.Coordinates{Y: 3})
				require.True(t, ok)
				v = el.Value().(*TestResponsive)
				assert.Equal(t, 'Z', v.Ch)
			}, `
YYYYYYYY
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ`,
		},
	}

	testutil.TestComponent(t, l, w, tests)
}

func TestResponsiveListResizeLarger(t *testing.T) {
	l := NewResponsiveList()

	w := term.NewStringWriter(8, 4)

	tests := []testutil.ComponentTestCase{
		{
			func() {
				l.PushBack(&TestResponsive{
					WantHeight: 2,
					TestComponent: TestComponent{
						Ch: 'X',
					},
				},
				)
				l.PushBack(&TestResponsive{
					WantHeight: 2,
					TestComponent: TestComponent{
						Ch: 'Y',
					},
				},
				)
				l.PushBack(&TestResponsive{
					WantHeight: 3,
					TestComponent: TestComponent{
						Ch: 'Z',
					},
				},
				)
				assert.True(t, l.SeekEnd())
				l.Resize(8, 4)
				assert.True(t, l.SeekEnd())
			}, `
YYYYYYYY
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ`,
		}, {
			func() {
				el, ok := l.ElementAt(term.Coordinates{})
				require.True(t, ok)
				v := el.Value().(*TestResponsive)
				assert.Equal(t, 'Y', v.Ch)

				el, ok = l.ElementAt(term.Coordinates{Y: 1})
				require.True(t, ok)
				v = el.Value().(*TestResponsive)
				assert.Equal(t, 'Z', v.Ch)

				el, ok = l.ElementAt(term.Coordinates{Y: 2})
				require.True(t, ok)
				v = el.Value().(*TestResponsive)
				assert.Equal(t, 'Z', v.Ch)

				el, ok = l.ElementAt(term.Coordinates{Y: 3})
				require.True(t, ok)
				v = el.Value().(*TestResponsive)
				assert.Equal(t, 'Z', v.Ch)
			}, `
YYYYYYYY
ZZZZZZZZ
ZZZZZZZZ
ZZZZZZZZ`,
		},
	}

	testutil.TestComponent(t, l, w, tests)
}

func TestResponsiveListScroll(t *testing.T) {
	l := NewResponsiveList()

	w := term.NewStringWriter(8, 4)
	l.PushBack(testResponsive('a', 2))
	l.PushBack(testResponsive('b', 1))
	l.PushBack(testResponsive('c', 3))
	l.Resize(7, 3)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
aaaaaaa 
aaaaaaa 
bbbbbbb 
        `,
		}, {
			func() {
				require.True(t, l.SeekDown())
			}, `
aaaaaaa 
bbbbbbb 
ccccccc 
        `,
		}, {
			func() {
				require.True(t, l.SeekDown())
			}, `
bbbbbbb 
ccccccc 
ccccccc 
        `,
		}, {
			func() {
				require.True(t, l.SeekDown())
			}, `
ccccccc 
ccccccc 
ccccccc 
        `,
		}, {
			func() {
				assert.False(t, l.SeekDown())
				require.True(t, l.SeekUp())
			}, `
bbbbbbb 
ccccccc 
ccccccc 
        `,
		}, {
			func() {
				require.True(t, l.SeekUp())
			}, `
aaaaaaa 
bbbbbbb 
ccccccc 
        `,
		}, {
			func() {
				require.True(t, l.SeekUp())
				assert.False(t, l.SeekUp())
			}, `
aaaaaaa 
aaaaaaa 
bbbbbbb 
        `,
		},
	}

	testutil.TestComponent(t, l, w, tests)
}

func TestResponsiveListScrollAlignmentBottom(t *testing.T) {
	l := NewResponsiveList()
	l.Alignment = SpanAlignmentBottom

	w := term.NewStringWriter(8, 4)
	l.PushBack(testResponsive('a', 2))
	l.PushBack(testResponsive('b', 1))
	l.PushBack(testResponsive('c', 3))
	l.Resize(7, 3)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
ccccccc 
ccccccc 
ccccccc 
        `,
		}, {
			func() {
				for y := 0; y < 3; y++ {
					el, ok := l.ElementAt(term.Coordinates{Y: y})
					require.True(t, ok, y)
					v := el.Value().(*TestResponsive)
					assert.Equal(t, 'c', v.Ch)
				}
				require.True(t, l.SeekUp())
			}, `
bbbbbbb 
ccccccc 
ccccccc 
        `,
		}, {
			func() {
				el, ok := l.ElementAt(term.Coordinates{Y: 0})
				require.True(t, ok)
				v := el.Value().(*TestResponsive)
				assert.Equal(t, 'b', v.Ch)
				el, ok = l.ElementAt(term.Coordinates{Y: 1})
				require.True(t, ok)
				v = el.Value().(*TestResponsive)
				assert.Equal(t, 'c', v.Ch)
				require.True(t, l.SeekUp())
			}, `
aaaaaaa 
bbbbbbb 
ccccccc 
        `,
		}, {
			func() {
				el, ok := l.ElementAt(term.Coordinates{Y: 0})
				require.True(t, ok)
				v := el.Value().(*TestResponsive)
				assert.Equal(t, 'a', v.Ch)
				require.True(t, l.SeekUp())
			}, `
aaaaaaa 
aaaaaaa 
bbbbbbb 
        `,
		}, {
			func() {
				assert.False(t, l.SeekUp())
				require.True(t, l.SeekDown())
				el, ok := l.ElementAt(term.Coordinates{Y: 0})
				require.True(t, ok)
				v := el.Value().(*TestResponsive)
				assert.Equal(t, 'a', v.Ch)
				el, ok = l.ElementAt(term.Coordinates{Y: 2})
				require.True(t, ok)
				v = el.Value().(*TestResponsive)
				assert.Equal(t, 'c', v.Ch)
			}, `
aaaaaaa 
bbbbbbb 
ccccccc 
        `,
		}, {
			func() {
				require.True(t, l.SeekDown())
			}, `
bbbbbbb 
ccccccc 
ccccccc 
        `,
		}, {
			func() {
				require.True(t, l.SeekDown())
				assert.False(t, l.SeekDown())
			}, `
ccccccc 
ccccccc 
ccccccc 
        `,
		},
	}

	testutil.TestComponent(t, l, w, tests)
}
