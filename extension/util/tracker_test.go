package util

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/clipboard"
	"unstable.build/go-tui/workspace"
)

func TestResourceTrackerIntegration(t *testing.T) {
	clipboard := clipboard.NewInMemory()
	wrap := true
	tabspaces := 4

	simpleEd := text.NewSimpleEditor(clipboard, wrap, true, /* command bar */
		term.Attributes{}, term.Attributes{})

	cfg := text.DefaultConfig()
	cfg.Tabspaces = tabspaces

	cwd := makeURI(t, "memory:///")

	scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), cwd)
	require.NoError(t, err)
	loader := workspace.NewSchemeWorkspace(cwd, scheme)

	ed, err := text.NewComponent(simpleEd, loader, cfg)
	require.NoError(t, err)

	tracker := NewResourceTracker(tabspaces, wrap)
	require.NoError(t, ed.SubscribeEvents(ResourceTrackerEventsComplete(), tracker))

	ed.Resize(8, 8)

	res1 := makeURI(t, "memory:///1")

	// sut
	var bh browserapi.Handler
	var edh text.Handler
	t.Run("Edit on editor is tracked by tracker", func(t *testing.T) {
		bh, err = ed.OpenFileTab(res1, false)
		require.NoError(t, err)

		res, ok := tracker.Resource(res1)
		require.True(t, ok)

		assert.Equal(t, wrap, res.Scroll.Wrap)
		assert.Equal(t, tabspaces, res.Scroll.Buffer().Tabspaces())

		edh, err = ed.Editor(res1)
		require.NoError(t, err)

		assert.Equal(t, "memory:///1", res.URI().String())
		assert.Equal(t, "", res.Scroll.Buffer().String())
		// not in focus yet, so propagated scroll width, height is 0
		assert.Equal(t, 0, res.Scroll.Width())
		assert.Equal(t, 0, res.Scroll.SizeHeight())
	})

	t.Run("switching focus to content propagates width, height", func(t *testing.T) {
		win, err := ed.Focus()
		require.NoError(t, err)
		require.NoError(t, win.SetContent(bh))

		res, ok := tracker.Resource(res1)
		require.True(t, ok)

		assert.Equal(t, 6, res.Scroll.Width())
		assert.Equal(t, 4, res.Scroll.SizeHeight())
	})

	t.Run("updates to buffer are replicated to resource", func(t *testing.T) {
		res, ok := tracker.Resource(res1)
		require.True(t, ok)

		ed.CellEditor(edh).
			Edit(term.Coordinates{}, term.Coordinates{},
				"abcdefghi\n1234\nXXXX\nX\nX\nX\nX\nX\nX")

		assert.Equal(t, "abcdefghi\n1234\nXXXX\nX\nX\nX\nX\nX\nX",
			res.Scroll.Buffer().String())
	})

	t.Run("scroll position is replicated to resource", func(t *testing.T) {
		handled := true
		for handled {
			_, handled = bh.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		}

		res, ok := tracker.Resource(res1)
		require.True(t, ok)

		assert.Equal(t, term.Coordinates{Y: 6}, res.Offset())
	})

	t.Run("cursor position is replicated to resource", func(t *testing.T) {
		res, ok := tracker.Resource(res1)
		require.True(t, ok)

		cur, err := ed.Cursor(edh)
		require.NoError(t, err)
		assert.Equal(t, term.Coordinates{Y: 8}, cur)
		assert.Equal(t, term.Coordinates{Y: 8}, res.Cursor())

		require.NoError(t, ed.SetCursor(edh, term.Coordinates{Y: 2}))
		assert.Equal(t, term.Coordinates{Y: 2}, res.Cursor())
	})
}

func makeURI(t *testing.T, uriStr string) workspaceapi.URI {
	uri, err := workspaceapi.ParseURI(uriStr)
	require.NoError(t, err)
	return uri
}
