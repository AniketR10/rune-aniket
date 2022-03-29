package workspace

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestManagerInit(t *testing.T) {
	tsuite := []struct {
		desc        string
		inWorkspace URI
		expectGetwd string
		wantChdir   string
	}{
		{"does not change working directory if already current dir",
			URI{uri: "file:///tmp"}, "/tmp", ""},
		{"changes working directory if not current dir",
			URI{uri: "file:///tmp"}, "/tmp/hello", "/tmp"},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.desc, func(t *testing.T) {
			m := new(Manager)
			m.osGetwd = func() (string, error) {
				return tcase.expectGetwd, nil
			}
			var actualChdir string
			m.osChdir = func(chdir string) error {
				actualChdir = chdir
				return nil
			}
			m.init(tcase.inWorkspace)
			assert.Equal(t, tcase.wantChdir, actualChdir)
		})
	}
}
