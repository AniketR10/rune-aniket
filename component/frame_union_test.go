package component

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	testutil "github.com/ernestrc/go-tui/util/test"
)

func TestDrawFrameUnionNoFrame(t *testing.T) {
	one := Virtual{C: &TestComponent{Ch: 'X'}}
	two := Virtual{C: &TestComponent{Ch: 'B'}}
	three := Virtual{C: &TestComponent{Ch: 'b'}}
	four := Virtual{C: &TestComponent{Ch: 'x'}}
	five := Virtual{C: &TestComponent{Ch: '\''}}
	main := Virtual{C: &TestComponent{Ch: 'A'}}
	f := NewFrameUnion(&main, false)
	f.UnionTop(&one)
	one.Resize(20, 3)
	two.Resize(20, 1)
	three.Resize(20, 1)
	four.Resize(20, 1)
	five.Resize(20, 1)
	f.Resize(20, 16)

	w := term.NewStringWriter(20, 20)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
XXXXXXXXXXXXXXXXXXXX
XXXXXXXXXXXXXXXXXXXX
XXXXXXXXXXXXXXXXXXXX
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
                    
                    
                    
                    `,
		}, {
			func() {
				one.Resize(2, 4)
				f.Resize(2, 3)
			}, `
AA                  
AA                  
AA                  
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				one.Resize(3, 1)
				f.Resize(3, 3)
			}, `
XXX                 
AAA                 
AAA                 
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				one.Resize(2, 1)
				f.Resize(2, 2)
			}, `
XX                  
AA                  
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				f.UnionBottom(&two)
				f.Resize(2, 2)
			}, `
AA                  
AA                  
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				one.Resize(20, 1)
				f.Resize(20, 16)
			}, `
XXXXXXXXXXXXXXXXXXXX
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
BBBBBBBBBBBBBBBBBBBB
                    
                    
                    
                    `,
		}, {
			func() {
				f.UnionBottom(&three)
				f.UnionTop(&four)
				f.UnionBottom(&five)
				f.Resize(20, 20)
			}, `
XXXXXXXXXXXXXXXXXXXX
xxxxxxxxxxxxxxxxxxxx
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAA
''''''''''''''''''''
bbbbbbbbbbbbbbbbbbbb
BBBBBBBBBBBBBBBBBBBB`,
		},
	}

	testutil.TestComponent(t, f, w, tests)
}

func TestDrawFrameUnionWithFrame(t *testing.T) {
	one := Virtual{C: NewFrame(&TestComponent{Ch: 'X'})}
	two := Virtual{C: NewFrame(&TestComponent{Ch: 'B'})}
	three := Virtual{C: NewFrame(&TestComponent{Ch: 'b'})}
	four := Virtual{C: NewFrame(&TestComponent{Ch: 'x'})}
	five := Virtual{C: NewFrame(&TestComponent{Ch: '\''})}
	six := Virtual{C: NewFrame(&TestComponent{Ch: '6'})}
	seven := Virtual{C: NewFrame(&TestComponent{Ch: '7'})}
	eight := Virtual{C: NewFrame(&TestComponent{Ch: '8'})}
	nine := Virtual{C: NewFrame(&TestComponent{Ch: '9'})}
	main := Virtual{C: NewFrame(&TestComponent{Ch: 'A'})}
	f := NewFrameUnion(&main, true)
	f.UnionTop(&one)
	for _, v := range []*Virtual{&one, &two, &three, &four, &five} {
		v.Resize(20, 3)
	}
	for _, v := range []*Virtual{&six, &seven, &eight, &nine} {
		v.Resize(3, 16)
	}
	f.Resize(20, 16)

	w := term.NewStringWriter(20, 20)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
┌──────────────────┐
│XXXXXXXXXXXXXXXXXX│
├──────────────────┤
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘
                    
                    
                    
                    `,
		}, {
			func() {
				f.Left = '┊'
				f.Right = '┊'
			}, `
┌──────────────────┐
│XXXXXXXXXXXXXXXXXX│
┊──────────────────┊
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘
                    
                    
                    
                    `,
		}, {
			func() {
				one.Resize(2, 4)
				f.Resize(2, 3)
			}, `
AA                  
AA                  
AA                  
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				one.Resize(3, 4)
				f.Resize(3, 3)
			}, `
┌─┐                 
│A│                 
└─┘                 
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				one.Resize(2, 2)
				f.Resize(2, 2)
			}, `
XX                  
AA                  
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				f.UnionBottom(&two)
				f.Resize(2, 2)
			}, `
AA                  
AA                  
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		}, {
			func() {
				one.Resize(20, 3)
				f.Resize(20, 16)
			}, `
┌──────────────────┐
│XXXXXXXXXXXXXXXXXX│
┊──────────────────┊
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
┊──────────────────┊
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘
                    
                    
                    
                    `,
		}, {
			func() {
				f.UnionBottom(&three)
				f.UnionTop(&four)
				f.UnionBottom(&five)
				f.UnionLeft(&six)
				f.UnionLeft(&seven)
				f.UnionRight(&eight)
				f.UnionRight(&nine)
				f.Resize(20, 20)
			}, `
┌──────────────────┐
│XXXXXXXXXXXXXXXXXX│
┊──────────────────┊
│xxxxxxxxxxxxxxxxxx│
┊─┬─┬──────────┬─┬─┊
│6│7│AAAAAAAAAA│9│8│
│6│7│AAAAAAAAAA│9│8│
│6│7│AAAAAAAAAA│9│8│
│6│7│AAAAAAAAAA│9│8│
│6│7│AAAAAAAAAA│9│8│
│6│7│AAAAAAAAAA│9│8│
│6│7│AAAAAAAAAA│9│8│
│6│7│AAAAAAAAAA│9│8│
┊─┴─┴──────────┴─┴─┊
│''''''''''''''''''│
┊──────────────────┊
│bbbbbbbbbbbbbbbbbb│
┊──────────────────┊
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`,
		},
	}

	testutil.TestComponent(t, f, w, tests)
}
