package workspace

import (
	"context"
	"os"
	"sort"
	"testing"

	"github.com/ernestrc/blue/iterator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
)

type readLinesTestFile struct {
	fullPath string
	content  string
}

func TestReadLines(t *testing.T) {
	tsuite := []struct {
		desc    string
		inFiles []readLinesTestFile
		wantErr string
		wantOut []string
	}{
		{"no files returns no data", nil, "", nil},
		{"empty files returns no data", []readLinesTestFile{{"a", ""}, {"b", ""}}, "", nil},
		{"one file returns data", []readLinesTestFile{{"a", "0\n1"}}, "", []string{"a:1:0", "a:2:1"}},
		{"multiple files returns data", []readLinesTestFile{{"a", "4\n5"}, {"b", "0\n1"}}, "",
			[]string{"a:1:4", "a:2:5", "b:1:0", "b:2:1"}},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			uri, err := ParseURI("memory:///")
			require.NoError(t, err)
			scheme, err := NewMemoryScheme(config.NopConfig(), uri)
			require.NoError(t, err)
			workspace := NewSchemeWorkspace(uri, scheme)

			var paths []string
			for _, file := range tcase.inFiles {
				f, werr := scheme.Open(file.fullPath, os.O_CREATE, 0)
				require.Nil(t, werr)
				_, err = f.Write([]byte(file.content))
				require.NoError(t, err)
				require.NoError(t, f.Sync())
				require.NoError(t, f.Close())
				paths = append(paths, file.fullPath)
			}
			itIn := iterator.FromSlice(paths)

			// sut
			itOut, err := ReadLines(context.Background(), workspace, itIn)
			if tcase.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tcase.wantErr)
				return
			}
			require.NoError(t, err)
			var lines []string
			for {
				line, ok := itOut.Next()
				if !ok {
					require.NoError(t, itOut.Err())
					break
				}
				lines = append(lines, line)
			}
			// order doesn't matter and the iterator is unordered
			sort.Strings(lines)
			assert.Equal(t, tcase.wantOut, lines)
		})
	}
}
