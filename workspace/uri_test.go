package workspace

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseURI(t *testing.T) {
	tsuite := []struct {
		in      string
		wantOut URI
		wantErr bool
	}{
		{"file:///", URI{
			uri: "file:///", name: "/", parsed: url.URL{Scheme: "file", Path: "/"},
		}, false},
		{"file:///tmp/a", URI{
			uri: "file:///tmp/a", name: "a", parsed: url.URL{Scheme: "file", Path: "/tmp/a"},
		}, false},
		{"/tmp/a", URI{}, true},
	}

	for _, tcase := range tsuite {
		t.Run(fmt.Sprintf("ParseURI(%s)", tcase.in), func(t *testing.T) {
			actualOut, actualErr := ParseURI(tcase.in)
			if tcase.wantErr {
				require.Error(t, actualErr)
			} else {
				require.NoError(t, actualErr)
			}
			assert.Equal(t, tcase.wantOut, actualOut)
		})
	}
}
