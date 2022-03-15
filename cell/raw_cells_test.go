package cell

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	rawCellsFortune = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`
	emptyString = ""
)

func TestRawCellsInsertMiddlePadding(t *testing.T) {
	fixture := `{
	b
	c
}
`
	fixtureCells := [][]term.Cell{
		{{Ch: '{'}},
		{{}, {}, {}, {Ch: '\t'}, {Ch: 'b'}},
		{{}, {}, {}, {Ch: '\t'}, {Ch: 'c'}},
		{{Ch: '}'}},
		{},
	}
	var c rawCells
	c.init(4)
	c.ReadFrom(strings.NewReader(fixture))

	from := term.Coordinates{X: 0, Y: 1}
	to := term.Coordinates{Y: 2}
	start, end, str := c.Delete(from, to)
	assert.Equal(t, from, start)
	assert.Equal(t, to, end)
	assert.Equal(t, "\tb\n", str)
	require.Equal(t, "{\n\tc\n}\n", c.String())

	from, to = c.Insert(term.Coordinates{X: 1, Y: 1}, "\tb\n")
	assert.Equal(t, term.Coordinates{X: 0, Y: 1}, from)
	assert.Equal(t, term.Coordinates{X: 0, Y: 2}, to)

	start, end, str = c.Delete(from, to)
	assert.Equal(t, term.Coordinates{X: 0, Y: 1}, start)
	assert.Equal(t, term.Coordinates{Y: 2}, end)
	assert.Equal(t, "\tb\n", str)

	from, to = c.Insert(term.Coordinates{X: 0, Y: 1}, "\tb\n")
	assert.Equal(t, term.Coordinates{X: 0, Y: 1}, from)
	assert.Equal(t, term.Coordinates{Y: 2}, to)
	assert.Equal(t, fixtureCells, c.RawCells())
}

func TestRawCellsPanicsNegativeCoordinates(t *testing.T) {

	var c rawCells
	c.init(4)
	negativeCoords := []term.Coordinates{
		term.Coordinates{X: -1, Y: 0},
		term.Coordinates{X: 0, Y: -1},
	}

	for _, pos := range negativeCoords {
		t.Run("cell()", func(t *testing.T) {
			assert.Panics(t, func() {
				c.Cell(pos)
			})
		})

		t.Run("insert()", func(t *testing.T) {
			assert.Panics(t, func() {
				c.Insert(pos, "r")
			})
		})
		t.Run("delete(from)", func(t *testing.T) {
			assert.Panics(t, func() {
				c.Delete(pos, term.Coordinates{X: 0, Y: 2})
			})
		})
		t.Run("delete(until)", func(t *testing.T) {
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

func TestRawCellsReadFrom(t *testing.T) {
	myError := errors.New("oopsie daisy")

	tsuite := []readFromTestCase{
		{[]string{"a"}, []error{nil}, 1, nil},
		{[]string{""}, []error{nil}, 0, nil},
		{[]string{"a", "b"}, []error{nil, nil}, 2, nil},
		{[]string{"ab", "c"}, []error{nil, nil}, 3, nil},
		{[]string{"a", ""}, []error{nil, io.EOF}, 1, nil},
		{[]string{""}, []error{myError}, 0, myError},
	}

	for i, tcase := range tsuite {
		var c rawCells
		c.init(defTabSpaces)
		reads := tcase.reads
		n, err := c.ReadFrom(&tcase)
		assert.Equal(t, tcase.expectedErr, err, "tcase %d", i)
		assert.Equal(t, tcase.expectedN, n, "tcase %d", i)
		if tcase.expectedErr == nil {
			assert.Equal(t, strings.Join(reads, ""), c.String())
		}
	}
}

func TestRawCellsStringReadFrom(t *testing.T) {
	tsuite := []struct {
		in   string
		want [][]term.Cell
	}{
		{"", [][]term.Cell{[]term.Cell{}}},
		{"\n", [][]term.Cell{[]term.Cell{}, []term.Cell{}}},
		{"\t\n", [][]term.Cell{[]term.Cell{{}, {}, {}, {Ch: '\t'}}, []term.Cell{}}},
		{"\t", [][]term.Cell{[]term.Cell{{}, {}, {}, {Ch: '\t'}}}},
		{"a", [][]term.Cell{[]term.Cell{{Ch: 'a'}}}},
		{"\nb", [][]term.Cell{[]term.Cell{}, []term.Cell{{Ch: 'b'}}}},
		{"c\n", [][]term.Cell{[]term.Cell{{Ch: 'c'}}, []term.Cell{}}},
		{"\n\n\n", [][]term.Cell{[]term.Cell{}, []term.Cell{}, []term.Cell{}, []term.Cell{}}},
		{"\n\n\na", [][]term.Cell{[]term.Cell{}, []term.Cell{}, []term.Cell{}, []term.Cell{{Ch: 'a'}}}},
	}

	for i, _tcase := range tsuite {
		tcase := _tcase
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			var c rawCells
			c.init(defTabSpaces)
			n, err := c.ReadFrom(strings.NewReader(tcase.in))
			assert.NoError(t, err)
			assert.Equal(t, int64(len(tcase.in)), n)
			assert.Equal(t, tcase.in, c.String())
			assert.Equal(t, tcase.want, c.RawCells())
		})
	}
}

func TestRawCellsInsert(t *testing.T) {
	const baseRawCells = `
syntax = "proto2";
package rpc;`

	const expectedRawCellsCase3 = `
syntax = "proto2";
package rpc;


  // what's up`

	const expectedRawCellsCase2 = `
syntax -= "proto2";
package rpc;`

	const expectedRawCellsCase4 = `Love in your heart wasn't put there to stay.

Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`

	const expectedRawCellsCase5 = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
			-- Oscar Hammerstein 中国`

	tsuite := []struct {
		overrideBaseRawCells     *string
		inputStr                 string
		inputAt                  term.Coordinates
		expectedRawCells         string
		expectedFrom, expectedTo term.Coordinates
	}{
		{
			inputStr:         ">>>\n",
			inputAt:          term.Coordinates{},
			expectedRawCells: ">>>\n" + baseRawCells,
			expectedFrom:     term.Coordinates{},
			expectedTo:       term.Coordinates{Y: 1},
		},
		{
			inputStr:         "-",
			inputAt:          term.Coordinates{X: 7, Y: 1},
			expectedRawCells: expectedRawCellsCase2,
			expectedFrom:     term.Coordinates{X: 7, Y: 1},
			expectedTo:       term.Coordinates{X: 8, Y: 1},
		},
		{
			inputStr:         "// what's up",
			inputAt:          term.Coordinates{X: 2, Y: 5},
			expectedRawCells: expectedRawCellsCase3,
			expectedFrom:     term.Coordinates{X: 12, Y: 2},
			expectedTo:       term.Coordinates{X: 14, Y: 5},
		},
		{
			overrideBaseRawCells: &rawCellsFortune,
			expectedRawCells:     expectedRawCellsCase4,
			inputAt:              term.Coordinates{Y: 1},
			inputStr:             "\n",
			expectedFrom:         term.Coordinates{Y: 1},
			expectedTo:           term.Coordinates{Y: 2},
		},
		{
			overrideBaseRawCells: &rawCellsFortune,
			expectedRawCells:     expectedRawCellsCase5,
			inputAt:              term.Coordinates{Y: 2},
			inputStr:             "\t",
			expectedFrom:         term.Coordinates{Y: 2},
			expectedTo:           term.Coordinates{X: 4, Y: 2},
		},
		{
			overrideBaseRawCells: &emptyString,
			expectedRawCells:     "\t\n",
			inputAt:              term.Coordinates{},
			inputStr:             "\t\n",
			expectedFrom:         term.Coordinates{},
			expectedTo:           term.Coordinates{Y: 1},
		},
		{
			overrideBaseRawCells: &emptyString,
			expectedRawCells:     "\n",
			inputAt:              term.Coordinates{},
			inputStr:             "\n",
			expectedFrom:         term.Coordinates{},
			expectedTo:           term.Coordinates{Y: 1},
		},
	}

	for i, tcase := range tsuite {
		var c rawCells
		c.init(defTabSpaces)
		var input string
		if tcase.overrideBaseRawCells != nil {
			input = *tcase.overrideBaseRawCells
		} else {
			input = baseRawCells
		}
		if input != "" {
			_, err := c.ReadFrom(strings.NewReader(input))
			require.NoError(t, err)
		}

		actualFrom, actualTo := c.Insert(tcase.inputAt, tcase.inputStr)
		assert.Equal(t, tcase.expectedFrom, actualFrom, "test case %d", i)
		assert.Equal(t, tcase.expectedTo, actualTo, "test case %d", i)
		assert.Equal(t, tcase.expectedRawCells, c.String())
	}
}

func TestRawCellsDelete(t *testing.T) {
	const baseRawCells = `
syntax = "proto2";
package rpc;


  // what's up`

	const expectedRawCellsCase1 = `
syntax = "proto2";
package rpc;


  `

	const expectedRawCellsCase2 = `
syntax  "proto2";
package rpc;


  // what's up`
	const expectedRawCellsCase3 = `
syntax = "proto2";
package rpc;

/ what's up`
	const expectedRawCellsCase4 = `;
package rpc;


  // what's up`

	const inputRawCellsCase5 = `Love in your heart wasn't put there to stay.

Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`

	tsuite := []struct {
		expectedStr          string
		expectedRawCells     string
		inputFrom, inputTo   term.Coordinates
		overrideBaseRawCells string //optional; otherwise baseRawCells is used
		//optional; otherwise inputFrom and inputTo is assumed to be returned
		expectedStart, expectedEnd *term.Coordinates
	}{
		{
			overrideBaseRawCells: "a",
			expectedStr:          "a",
			expectedRawCells:     "",
			inputFrom:            term.Coordinates{X: 0},
			inputTo:              term.Coordinates{X: 1},
		},
		{
			overrideBaseRawCells: "a",
			expectedStr:          "a",
			expectedRawCells:     "",
			inputFrom:            term.Coordinates{X: 0},
			inputTo:              term.Coordinates{X: 1},
		},
		{
			overrideBaseRawCells: "a\nb",
			expectedStr:          "a",
			expectedRawCells:     "\nb",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{X: 1},
		},
		{
			overrideBaseRawCells: "a\nb",
			expectedStr:          "a\n",
			expectedRawCells:     "b",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{Y: 1},
		},
		{
			expectedStr:      "// what's up",
			expectedRawCells: expectedRawCellsCase1,
			inputFrom:        term.Coordinates{X: 2, Y: 5},
			inputTo:          term.Coordinates{X: 2 + len("// what's up"), Y: 5},
		},
		{
			expectedStr:      "=",
			expectedRawCells: expectedRawCellsCase2,
			inputFrom:        term.Coordinates{X: 7, Y: 1},
			inputTo:          term.Coordinates{X: 8, Y: 1},
		},
		{
			overrideBaseRawCells: "a\nbc",
			expectedStr:          "a\nb",
			expectedRawCells:     "c",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{X: 1, Y: 1},
		},
		{
			overrideBaseRawCells: "aa\nbb",
			expectedStr:          "a\nb",
			expectedRawCells:     "ab",
			inputFrom:            term.Coordinates{X: 1},
			inputTo:              term.Coordinates{X: 1, Y: 1},
		},
		{ // 8
			overrideBaseRawCells: "a\nbc",
			expectedStr:          "a\nbc",
			expectedRawCells:     "",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{Y: 2},
		},
		{
			overrideBaseRawCells: "a\nb\nc",
			expectedStr:          "a\nb\n",
			expectedRawCells:     "c",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{Y: 2},
		},
		{
			overrideBaseRawCells: "a",
			expectedStr:          "a",
			expectedRawCells:     "",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{X: 1},
		},
		{
			// inverted from/until
			expectedStr:      baseRawCells,
			expectedRawCells: "",
			inputFrom:        term.Coordinates{Y: 6},
			inputTo:          term.Coordinates{},
			// returns inverted from/to
			expectedStart: &term.Coordinates{},
			expectedEnd:   &term.Coordinates{Y: 6},
		},
		{
			expectedStr:      baseRawCells,
			expectedRawCells: "",
			inputFrom:        term.Coordinates{},
			inputTo:          term.Coordinates{X: 14, Y: 5},
		},
		{
			expectedStr:      "\nsyntax = \"proto2\"",
			expectedRawCells: expectedRawCellsCase4,
			inputFrom:        term.Coordinates{},
			inputTo:          term.Coordinates{X: 17, Y: 1},
		},
		{
			expectedStr:      "\n  /",
			expectedRawCells: expectedRawCellsCase3,
			inputFrom:        term.Coordinates{Y: 4},
			inputTo:          term.Coordinates{X: 3, Y: 5},
		},
		{
			overrideBaseRawCells: "a\tb",
			expectedStr:          "a",
			expectedRawCells:     "\tb",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{X: 1},
		},
		{
			overrideBaseRawCells: "a\tb",
			expectedStr:          "a\t",
			expectedRawCells:     "b",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{X: 5},
		},
		{
			overrideBaseRawCells: "a\tb",
			expectedStr:          "a\t",
			expectedRawCells:     "b",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{X: 2},
			expectedEnd:          &term.Coordinates{X: 5},
		},
		{
			overrideBaseRawCells: "aa\tb",
			expectedStr:          "\t",
			expectedRawCells:     "aab",
			inputFrom:            term.Coordinates{X: 3},
			inputTo:              term.Coordinates{X: 4},
			expectedStart:        &term.Coordinates{X: 2},
			expectedEnd:          &term.Coordinates{X: 6},
		},
		{
			overrideBaseRawCells: "a\n\tb",
			expectedStr:          "a\n\t",
			expectedRawCells:     "b",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{Y: 1, X: 2},
			expectedEnd:          &term.Coordinates{Y: 1, X: 4},
		},
		{
			overrideBaseRawCells: "a\n\tb\n\tc",
			expectedStr:          "\tb\n\t",
			expectedRawCells:     "a\nc",
			inputFrom:            term.Coordinates{Y: 1, X: 2},
			inputTo:              term.Coordinates{Y: 2, X: 3},
			expectedStart:        &term.Coordinates{Y: 1, X: 0},
			expectedEnd:          &term.Coordinates{Y: 2, X: 4},
		},
		{
			overrideBaseRawCells: "\t\t\ta",
			expectedStr:          "\t",
			expectedRawCells:     "\t\ta",
			inputFrom:            term.Coordinates{X: 5},
			inputTo:              term.Coordinates{X: 6},
			expectedStart:        &term.Coordinates{X: 4},
			expectedEnd:          &term.Coordinates{X: 8},
		},
		{
			overrideBaseRawCells: "\t\t\ta",
			expectedStr:          "\t",
			expectedRawCells:     "\t\ta",
			inputFrom:            term.Coordinates{X: 3},
			inputTo:              term.Coordinates{X: 4},
			expectedStart:        &term.Coordinates{X: 0},
			expectedEnd:          &term.Coordinates{X: 4},
		},
		{
			overrideBaseRawCells: "\t\t\ta",
			expectedStr:          "\t\t",
			expectedRawCells:     "\ta",
			inputFrom:            term.Coordinates{X: 0},
			inputTo:              term.Coordinates{X: 8},
		},
		{ // 24
			overrideBaseRawCells: "a\nb\n\nc\n\n\nd",
			expectedStr:          "a\nb\n\nc\n",
			expectedRawCells:     "\n\nd",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{Y: 4},
		},
		{
			overrideBaseRawCells: "a\nbb\nccc",
			expectedStr:          "\nbb",
			expectedRawCells:     "a\nccc",
			inputFrom:            term.Coordinates{X: 2, Y: 1},
			inputTo:              term.Coordinates{X: 1},
			expectedStart:        &term.Coordinates{X: 1},
			expectedEnd:          &term.Coordinates{X: 2, Y: 1},
		},
		{
			overrideBaseRawCells: "\t\n",
			expectedStr:          "\t\n",
			expectedRawCells:     "",
			inputFrom:            term.Coordinates{},
			inputTo:              term.Coordinates{Y: 1},
		},
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			var c rawCells
			c.init(defTabSpaces)
			base := baseRawCells
			if tcase.overrideBaseRawCells != "" {
				base = tcase.overrideBaseRawCells
			}

			_, err := c.ReadFrom(strings.NewReader(base))
			require.NoError(t, err)

			actualStart, actualEnd, actualStr := c.Delete(tcase.inputFrom, tcase.inputTo)
			assert.Equal(t, tcase.expectedStr, actualStr,
				"expected return string")
			assert.Equal(t, tcase.expectedRawCells, c.String(),
				"expected resulting cells")

			if tcase.expectedStart == nil {
				tcase.expectedStart = &tcase.inputFrom
			}
			if tcase.expectedEnd == nil {
				tcase.expectedEnd = &tcase.inputTo
			}
			assert.Equal(t, *tcase.expectedStart, actualStart)
			assert.Equal(t, *tcase.expectedEnd, actualEnd)

			// test symmetry
			from, to := c.Insert(actualStart, actualStr)
			assert.Equal(t, actualStart, from)
			assertEquivalentEnd(t, &c, actualEnd, to)
			assert.Equal(t, base, c.String(),
				"insert was not able to reverse delete")
		})
	}
}

// in last line, delete can use Y:y+1, X:0 but insert might return
// the equivalent of that which is Y:y, X: len(insert)
// that's because delete til last character can be expressed both by
// deleting til past last line or til past las character of last line
func assertEquivalentEnd(
	t *testing.T, c *rawCells, expectedEnd, end term.Coordinates,
) {
	if end.Y == c.Rows()-1 && end.X == c.Columns(end.Y) && expectedEnd != end {
		end.Y++
		end.X = 0
	}
	assert.Equal(t, expectedEnd, end)
}

func TestRawCellsCell(t *testing.T) {
	var c rawCells
	c.init(defTabSpaces)
	c.ReadFrom(strings.NewReader(benchmarkFortune))

	cell, ok := c.Cell(term.Coordinates{})
	assert.False(t, ok)

	cell, ok = c.Cell(term.Coordinates{X: 1})
	assert.False(t, ok)

	cell, ok = c.Cell(term.Coordinates{Y: 1, X: 16})
	assert.True(t, ok)
	assert.Equal(t, term.Cell{Ch: 'L'}, cell)

	cell, ok = c.Cell(term.Coordinates{Y: 3, X: 37})
	assert.True(t, ok)
	assert.Equal(t, term.Cell{Ch: '中'}, cell)

	cell, ok = c.Cell(term.Coordinates{Y: 666})
	assert.False(t, ok)
}

func TestRawCellsInsertDeleteSymmetry(t *testing.T) {
	// enable if want to brute-test insert/delete symmetry
	t.SkipNow()

	file, err := os.Open("raw_cells_test.go")
	require.NoError(t, err)
	defer file.Close()

	var r rawCells
	r.init(defTabSpaces)
	r.ReadFrom(file)

	cells := r.RawCells()
	for i, row := range cells {
		for j := range row {
			start, end, str := r.Delete(term.Coordinates{}, term.Coordinates{X: j, Y: i})
			from, to := r.Insert(start, str)
			assert.Equal(t, end, to)
			assert.Equal(t, start, from)
		}
	}

	file.Seek(0, 0)
	b, err := ioutil.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, string(b), r.String())
}

func newBenchmarkRawCells(fortunes int) (*rawCells, string) {
	cells := new(rawCells)
	cells.init(defTabSpaces)
	payload := ""
	for i := 0; i < fortunes; i++ {
		payload = payload + benchmarkFortune
	}
	return cells, payload
}

func benchmarkBufferReadFrom(b *testing.B, fortunes int) {
	cells, payload := newBenchmarkRawCells(fortunes)
	reader := strings.NewReader(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cells.reset()
		reader.Reset(payload)
		_, _ = cells.ReadFrom(reader)
	}
}

var benchmarkFortune = `
				Love in your heart wasn't put there to stay.
				Love isn't love 'til you give it away.
				-- Oscar Hammerstein 中国
`

// NOTE: names starting with 'Buffer' are kept so we can
// compare to when ReadFrom was implemented in Buffer.
func BenchmarkBufferReadFrom10(b *testing.B) {
	benchmarkBufferReadFrom(b, 10)
}
func BenchmarkBufferReadFrom100(b *testing.B) {
	benchmarkBufferReadFrom(b, 100)
}
func BenchmarkBufferReadFrom1000(b *testing.B) {
	benchmarkBufferReadFrom(b, 1000)
}
func BenchmarkBufferReadFrom10000(b *testing.B) {
	benchmarkBufferReadFrom(b, 10000)
}

// func BenchmarkBufferReadFrom100MB(b *testing.B) {
// 	benchmarkBufferReadFrom(b, 1000000)
// }
