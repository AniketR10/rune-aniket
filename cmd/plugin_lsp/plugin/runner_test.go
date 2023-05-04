package plugin

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"
	"testing"

	"github.com/ernestrc/golang-internal-tools/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text"
)

//go:embed test/*.in
var testin embed.FS

//go:embed test/*.want
var testout embed.FS

func TestLSPFormatting(t *testing.T) {
	baseDir := "test"
	entries, err := testin.ReadDir(baseDir)
	require.NoError(t, err)

	for _, entry := range entries {
		name := entry.Name()
		extension := filepath.Ext(name)
		name = name[0 : len(name)-len(extension)]

		input := entry.Name()
		output := fmt.Sprintf("%s.want", name)
		t.Run(fmt.Sprintf("%s", name), func(t *testing.T) {
			in, err := testin.Open(path.Join(baseDir, input))
			require.NoError(t, err)

			out, err := testout.Open(path.Join(baseDir, output))
			require.NoError(t, err)

			want, err := ioutil.ReadAll(out)
			require.NoError(t, err)

			buffer := cell.NewBuffer()
			_, err = buffer.ReadFrom(in)
			require.NoError(t, err)
			editsRaw := cell.CellsToString(buffer.RawCells()[buffer.Rows()-2:])
			var edits []protocol.TextEdit
			err = json.Unmarshal([]byte(editsRaw), &edits)
			require.NoError(t, err)

			var b editBuilder
			b.init(4, makeFile(), text.NewCellEditor(buffer.Editor()), cell.StringToCells(buffer.String(), 4))
			b.applyEdits(edits)
			assert.Equal(t, string(want), b.buf.String())
		})
	}
}
