package text

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testFilesLines = 3

var fileWithNoEOL string
var fileWithEOL string

func init() {
	f, err := ioutil.TempFile("", "test_raw_cells_1_")
	if err != nil {
		return
	}

	f.WriteString("LINE")
	for i := 1; i < testFilesLines; i++ {
		_, err := f.WriteString("\nLINE")
		if err != nil {
			return
		}
	}

	fileWithNoEOL = f.Name()
	f.Close()

	f, err = ioutil.TempFile("", "test_raw_cells_2_")
	if err != nil {
		return
	}
	defer f.Close()

	for i := 0; i < testFilesLines; i++ {
		_, err := f.WriteString("LINE\n")
		if err != nil {
			return
		}
	}

	fileWithEOL = f.Name()
}

func TestUnixFile(t *testing.T) {
	require.NotZero(t, fileWithNoEOL)
	require.NotZero(t, fileWithEOL)

	f, err := os.Open(fileWithNoEOL)
	require.NoError(t, err)
	defer f.Close()

	f2, err := os.Open(fileWithEOL)
	require.NoError(t, err)
	defer f2.Close()

	tsuite := []struct {
		input    io.Reader
		expected string
		rows     int
	}{
		{strings.NewReader(""), "", 1},
		{strings.NewReader("fjelkwfjlkew"), "fjelkwfjlkew", 1},
		{strings.NewReader("fjelkwfjlkew\nfewjklfe"), "fjelkwfjlkew\nfewjklfe", 2},
		{f, "LINE\nLINE\nLINE", testFilesLines},
		{f2, "LINE\nLINE\nLINE", testFilesLines},
	}

	for i, tcase := range tsuite {
		reader := tcase.input
		{
			c := cell.NewBuffer()
			_, err := c.ReadFrom(tcase.input)
			require.NoError(t, err)

			reader := newUnixFileReader(c)

			assert.Equal(t, tcase.rows, reader.Rows(), fmt.Sprintf("ReadFrom(%d)", i))
			assert.Equal(t, tcase.expected, reader.String(), i)
		}

		s := reader.(io.Seeker)
		s.Seek(0, 0)

		{
			c := cell.NewBuffer()
			bytes, err := ioutil.ReadAll(tcase.input)
			require.NoError(t, err)
			c.Edit(term.Coordinates{}, term.Coordinates{}, string(bytes))

			reader := newUnixFileReader(c)

			assert.Equal(t, tcase.rows, reader.Rows(), fmt.Sprintf("insert(%d)", i))
			assert.Equal(t, tcase.expected, reader.String(), i)
		}
	}
}
