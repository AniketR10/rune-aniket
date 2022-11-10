package document

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/encoding/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

type testStruct struct {
	Id        string
	Content   string
	UpdatedAt time.Time `yaml:"-"`
}

func (t testStruct) ID() string {
	return t.Id
}

func (t testStruct) WithID(id string) testStruct {
	t.Id = id
	return t
}

func (t testStruct) UpdatedTime() time.Time {
	return t.UpdatedAt
}

func TestDocumentOpen(t *testing.T) {
	t.Run("file should have a default template after Open", func(t *testing.T) {
		workspaceURI, err := workspace.ParseURI("inmemory:///tmp")
		require.NoError(t, err)
		marshaler := yaml.Marshaler()
		svc := document.NewInMemoryServiceWithMarshaler(marshaler)
		scheme, err := WorkspaceScheme[testStruct](workspaceURI, svc,
			marshaler, errors.New("missing id"))(config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		f, werr := scheme.Open("a", os.O_CREATE|os.O_RDWR, 0)
		require.Nil(t, werr)

		data, err := ioutil.ReadAll(f)
		require.NoError(t, err)

		assert.Equal(t, "id: a\ncontent: \"\"\n", string(data))
	})
}
