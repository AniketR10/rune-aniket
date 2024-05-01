package document

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/encoding/json"
	"github.com/unstablebuild/blue/encoding/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

type testStruct struct {
	Id        string
	Content   string
	UpdatedAt time.Time
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

func (t testStruct) WithUpdatedTime(now time.Time) testStruct {
	t.UpdatedAt = now
	return t
}

func TestDocumentOpen(t *testing.T) {
	t.Run("file should have a default template after Open", func(t *testing.T) {
		ctx := context.Background()

		workspaceURI, err := workspaceapi.ParseURI("inmemory:///tmp")
		require.NoError(t, err)
		marshaler := yaml.Marshaler()
		svc := document.NewInMemoryServiceWithMarshaler(marshaler)
		scheme, err := WorkspaceScheme[testStruct](workspaceURI, svc,
			marshaler, errors.New("missing id"))(ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		f, werr := scheme.Open("a", os.O_CREATE|os.O_RDWR, 0)
		require.Nil(t, werr)

		data, err := ioutil.ReadAll(f)
		require.NoError(t, err)

		assert.True(t, strings.HasPrefix(string(data), "id: a\ncontent: \"\""))
	})

	t.Run("swap at rest should be populated", func(t *testing.T) {
		ctx := context.Background()
		workspaceURI, err := workspaceapi.ParseURI("inmemory:///tmp")
		require.NoError(t, err)
		marshaler := json.Marshaler()

		svc := document.NewInMemoryServiceWithMarshaler(marshaler)

		// two schemes so there's no usage of the cached file in memory
		scheme1, err := WorkspaceScheme[testStruct](workspaceURI, svc,
			marshaler, errors.New("missing id"))(ctx, config.NopConfig(),
			workspaceURI)
		scheme2, err := WorkspaceScheme[testStruct](workspaceURI, svc,
			marshaler, errors.New("missing id"))(ctx, config.NopConfig(),
			workspaceURI)
		defer scheme1.Close()
		defer scheme2.Close()

		uri, err := scheme1.URI(".")
		require.NoError(t, err)
		wp1 := workspace.NewSchemeWorkspace(uri, scheme1)
		wp2 := workspace.NewSchemeWorkspace(uri, scheme2)
		fileuri := workspaceapi.Join(uri, "dataAtRestTest")
		swapfileuri := workspaceapi.Join(uri, ".dataAtRestTest.swp")
		swapDir := workspaceapi.Join(uri, ".")

		buf := cell.NewBuffer()
		_, err = wp1.Load(fileuri, buf, swapDir, false)
		require.NoError(t, err)
		_, err = write(buf, []byte("short"))

		// write is async so wait for it
		time.Sleep(2000 * time.Millisecond)
		require.NoError(t, err)

		buf = cell.NewBuffer()
		fc2, err := wp2.Recover(fileuri, swapfileuri, buf, true)
		require.NoError(t, err)
		require.NoError(t, fc2.Flush())

		f, werr := scheme2.Open("dataAtRestTest", os.O_RDONLY, 0)
		require.Nil(t, werr)
		data, err := readAll(f)
		require.NoError(t, err)
		require.Equal(t, "short", string(data))

		require.NoError(t, fc2.Close())
		require.NoError(t, f.Close())
	})
}

func readAll(r io.Reader) ([]byte, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var temp testStruct
	err = json.Marshaler().Unmarshal(data, &temp)
	if err != nil {
		return nil, err
	}

	return []byte(temp.Content), nil
}

func write(buf *cell.Buffer, data []byte) (int, error) {
	var doc testStruct

	err := json.Marshaler().Unmarshal([]byte(buf.String()), &doc)
	if err != nil {
		return 0, fmt.Errorf("Unmarshal: %v: %q", err, buf.String())
	}
	var builder strings.Builder
	builder.WriteString(doc.Content)
	builder.Write(data)
	doc.Content = builder.String()

	data, err = json.Marshaler().Marshal(doc)
	if err != nil {
		return 0, fmt.Errorf("Marshal: %v", err)
	}

	end := term.Coordinates{Y: buf.Rows(), X: buf.Columns(buf.Rows()-1) + 1}
	buf.Edit(context.Background(), term.Coordinates{}, end, string(data))
	return len(data), nil
}
