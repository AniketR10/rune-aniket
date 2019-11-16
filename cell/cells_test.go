package cell

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ernestrc/fractal/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCellsUninitialized(t *testing.T) {
	t.Run("Columns()", func(t *testing.T) {
		var c Cells
		assert.Equal(t, 0, c.Columns(0))
	})

	t.Run("Insert()", func(t *testing.T) {
		var c Cells
		from, until := c.Insert(term.Coordinates{X: 0, Y: 0}, "r")
		assert.Equal(t, term.Coordinates{}, from)
		assert.Equal(t, term.Coordinates{X: 1}, until)
	})

	t.Run("NextWrite()", func(t *testing.T) {
		var c Cells
		assert.Equal(t, term.Coordinates{}, c.NextWrite())
	})

	t.Run("RawCells()", func(t *testing.T) {
		var c Cells
		cs := c.RawCells()
		assert.Equal(t, cs, [][]term.Cell{[]term.Cell{}})
	})

	t.Run("ReadFrom()", func(t *testing.T) {
		var c Cells
		n, err := c.ReadFrom(strings.NewReader("r"))
		assert.NoError(t, err)
		assert.Equal(t, int64(1), n)
	})

	t.Run("Reset()", func(t *testing.T) {
		var c Cells
		c.Reset()
	})

	t.Run("Rows()", func(t *testing.T) {
		var c Cells
		assert.Equal(t, 0, c.Rows())
	})

	t.Run("String()", func(t *testing.T) {
		var c Cells
		assert.Equal(t, "", c.String())
	})

	t.Run("Tabspaces()", func(t *testing.T) {
		var c Cells
		assert.Equal(t, 0, c.Tabspaces())
	})
}

func TestCellsPanicsNegativeCoordinates(t *testing.T) {

	var c Cells
	negativeCoords := []term.Coordinates{
		term.Coordinates{X: -1, Y: 0},
		term.Coordinates{X: 0, Y: -1},
	}

	for _, pos := range negativeCoords {
		t.Run("Cell()", func(t *testing.T) {
			assert.Panics(t, func() {
				c.Cell(pos)
			})
		})

		t.Run("Insert()", func(t *testing.T) {
			assert.Panics(t, func() {
				c.Insert(pos, "r")
			})
		})
		t.Run("Delete(from)", func(t *testing.T) {
			assert.Panics(t, func() {
				c.Delete(pos, term.Coordinates{X: 0, Y: 2})
			})
		})
		t.Run("Delete(until)", func(t *testing.T) {
			assert.Panics(t, func() {
				c.Delete(term.Coordinates{X: 0, Y: 2}, pos)
			})
		})
	}
}

type readFromTestCase struct {
	reads       []string
	errors      []error
	expectedN   int64
	expectedErr error
}

func (r *readFromTestCase) Read(p []byte) (n int, err error) {
	if len(r.reads) == 0 {
		err = io.EOF
		return
	}

	defer func() {
		r.reads = r.reads[1:]
		r.errors = r.errors[1:]
	}()

	read := r.reads[0]
	err = r.errors[0]
	if err != nil {
		return
	}
	n = copy(p, read)
	return
}

func TestCellsReadFrom(t *testing.T) {
	myError := errors.New("oopsie daisy")

	tsuite := []readFromTestCase{
		{[]string{"a"}, []error{nil}, 1, nil},
		{[]string{""}, []error{nil}, 0, nil},
		{[]string{"a", "b"}, []error{nil, nil}, 2, nil},
		{[]string{"a\nb", "c"}, []error{nil, nil}, 4, nil},
		// FIXME {[]string{"\n"}, []error{nil}, 1, nil},
		// FIXME {[]string{"a\r\nb", "c"}, []error{nil, nil}, 5, nil},
		{[]string{""}, []error{myError}, 0, myError},
	}

	for i, tcase := range tsuite {
		var c Cells
		reads := tcase.reads
		n, err := c.ReadFrom(&tcase)
		assert.Equal(t, tcase.expectedErr, err, "tcase %d", i)
		assert.Equal(t, tcase.expectedN, n, "tcase %d", i)
		if tcase.expectedErr == nil {
			assert.Equal(t, strings.Join(reads, ""), c.String())
		}
	}
}

func TestCellsInsert(t *testing.T) {
	const baseCells = `
syntax = "proto2";
package rpc;`

	tsuite := []struct {
		inputStr                    string
		inputAt                     term.Coordinates
		expectedCells               string
		expectedFrom, expectedUntil term.Coordinates
	}{
		{
			inputStr:      ">>>\n",
			inputAt:       term.Coordinates{},
			expectedCells: ">>>\n" + baseCells,
			expectedFrom:  term.Coordinates{},
			expectedUntil: term.Coordinates{Y: 1},
		},
		{
			inputStr: "-",
			inputAt:  term.Coordinates{X: 7, Y: 1},
			expectedCells: `
syntax -= "proto2";
package rpc;`,
			expectedFrom:  term.Coordinates{X: 7, Y: 1},
			expectedUntil: term.Coordinates{X: 8, Y: 1},
		},
		{
			inputStr: "// what's up",
			inputAt:  term.Coordinates{X: 2, Y: 5},
			expectedCells: `
syntax = "proto2";
package rpc;


  // what's up`,
			expectedFrom:  term.Coordinates{X: 12, Y: 2},
			expectedUntil: term.Coordinates{X: 2 + len("// what's up"), Y: 5},
		},
	}

	for i, tcase := range tsuite {
		var c Cells
		_, err := c.ReadFrom(strings.NewReader(baseCells))
		require.NoError(t, err)

		actualFrom, actualUntil := c.Insert(tcase.inputAt, tcase.inputStr)
		assert.Equal(t, tcase.expectedFrom, actualFrom, "test case %d", i)
		assert.Equal(t, tcase.expectedUntil, actualUntil, "test case %d", i)
		assert.Equal(t, tcase.expectedCells, c.String())
	}
}

func TestCellsDelete(t *testing.T) {
	const baseCells = `
syntax = "proto2";
package rpc;


  // what's up`

	const expectedCellsCase1 = `
syntax = "proto2";
package rpc;


  `

	const expectedCellsCase2 = `
syntax  "proto2";
package rpc;


  // what's up`
	const expectedCellsCase3 = `
syntax = "proto2";
package rpc;

/ what's up`
	const expectedCellsCase4 = `;
package rpc;


  // what's up`

	tsuite := []struct {
		expectedStr        string
		expectedCells      string
		inputFrom, inputTo term.Coordinates
		overrideBaseCells  string //optional; otherwise baseCells is used
	}{
		{
			overrideBaseCells: "a\nb",
			expectedStr:       "a",
			expectedCells:     "\nb",
			inputFrom:         term.Coordinates{},
			inputTo:           term.Coordinates{},
		},
		{
			overrideBaseCells: "a\nb",
			expectedStr:       "a\n",
			expectedCells:     "b",
			inputFrom:         term.Coordinates{},
			inputTo:           term.Coordinates{X: 1},
		},
		{
			expectedStr:   "// what's up",
			expectedCells: expectedCellsCase1,
			inputFrom:     term.Coordinates{X: 2, Y: 5},
			inputTo:       term.Coordinates{X: 2 + len("// what's up"), Y: 5},
		},
		{
			expectedStr:   "=",
			expectedCells: expectedCellsCase2,
			inputFrom:     term.Coordinates{X: 7, Y: 1},
			inputTo:       term.Coordinates{X: 7, Y: 1},
		},
		{
			overrideBaseCells: "a\nbc",
			expectedStr:       "a\nb",
			expectedCells:     "c",
			inputFrom:         term.Coordinates{},
			inputTo:           term.Coordinates{X: 0, Y: 1},
		},
		{
			overrideBaseCells: "aa\nbb",
			expectedStr:       "a\nb",
			expectedCells:     "ab",
			inputFrom:         term.Coordinates{X: 1},
			inputTo:           term.Coordinates{X: 0, Y: 1},
		},
		{
			overrideBaseCells: "a\nbc",
			expectedStr:       "a\nbc",
			expectedCells:     "",
			inputFrom:         term.Coordinates{},
			inputTo:           term.Coordinates{X: 1, Y: 1},
		},
		{
			overrideBaseCells: "a\nb\nc",
			expectedStr:       "a\nb\n",
			expectedCells:     "c",
			inputFrom:         term.Coordinates{},
			inputTo:           term.Coordinates{X: 1, Y: 1},
		},
		{
			overrideBaseCells: "a",
			expectedStr:       "a",
			expectedCells:     "",
			inputFrom:         term.Coordinates{},
			inputTo:           term.Coordinates{X: 1},
		},
		{
			// inverted from/until
			expectedStr:   baseCells,
			expectedCells: "",
			inputTo:       term.Coordinates{},
			inputFrom:     term.Coordinates{X: 14, Y: 5},
		},
		{
			expectedStr:   baseCells,
			expectedCells: "",
			inputFrom:     term.Coordinates{},
			inputTo:       term.Coordinates{X: 14, Y: 5},
		},
		{
			expectedStr:   "\nsyntax = \"proto2\"",
			expectedCells: expectedCellsCase4,
			inputFrom:     term.Coordinates{},
			inputTo:       term.Coordinates{X: 16, Y: 1},
		},
		{
			expectedStr:   "\n  /",
			expectedCells: expectedCellsCase3,
			inputFrom:     term.Coordinates{Y: 4},
			inputTo:       term.Coordinates{X: 2, Y: 5},
		},
		{
			overrideBaseCells: "a\tb",
			expectedStr:       "a\t",
			expectedCells:     "b",
			inputFrom:         term.Coordinates{},
			inputTo:           term.Coordinates{X: 4},
		},
		{
			overrideBaseCells: "a\tb",
			expectedStr:       "a\t",
			expectedCells:     "b",
			inputFrom:         term.Coordinates{},
			inputTo:           term.Coordinates{X: 2},
		},
		{
			overrideBaseCells: "a\tb",
			expectedStr:       "a\t",
			expectedCells:     "b",
			inputFrom:         term.Coordinates{X: 2},
			inputTo:           term.Coordinates{X: 4},
		},
		{
			overrideBaseCells: "a\n\tb",
			expectedStr:       "a\n\t",
			expectedCells:     "b",
			inputFrom:         term.Coordinates{},
			inputTo:           term.Coordinates{Y: 1, X: 2},
		},
		{
			overrideBaseCells: "a\n\tb\n\tc",
			expectedStr:       "\tb\n\t",
			expectedCells:     "a\nc",
			inputFrom:         term.Coordinates{Y: 1, X: 2},
			inputTo:           term.Coordinates{Y: 2, X: 2},
		},
	}

	for i, tcase := range tsuite {
		var c Cells
		base := baseCells
		if tcase.overrideBaseCells != "" {
			base = tcase.overrideBaseCells
		}
		_, err := c.ReadFrom(strings.NewReader(base))
		require.NoError(t, err)

		actualStr := c.Delete(tcase.inputFrom, tcase.inputTo)
		assert.Equal(t, tcase.expectedStr, actualStr, "expected return string in test case %d", i)
		assert.Equal(t, tcase.expectedCells, c.String(), "expected cells in test case %d", i)
	}
}
