package plugin

import (
	"context"
	"errors"

	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/vte"
	testutil "unstable.build/go-tui/util/test"
	"unstable.build/go-tui/workspace"
)

func TestPluginHandlerCursor(t *testing.T) {
	makeHandler := func(mock *handler.TestHandler) *Handler {
		h := new(Handler)
		h.initState(nopBrowser{}, vte.DefaultConfig(),
			"cmd", 100 /* width */, true, /* frame */
			component.FrameCharSetDefault(), term.Attributes{})
		h.liveHandler = mock
		h.Resize(100, 100)
		return h
	}
	t.Run("corrects coordinates past width-1 and height-1", func(t *testing.T) {
		h := makeHandler(&handler.TestHandler{CursorPos: term.Coordinates{X: 1000, Y: 1000}})
		pos, _, show := h.Cursor()
		require.True(t, show)
		assert.Equal(t, term.Coordinates{X: 99, Y: 99}, pos)
	})
	t.Run("corrects negative coordinates", func(t *testing.T) {
		h := makeHandler(&handler.TestHandler{CursorPos: term.Coordinates{X: -99, Y: -99}})
		pos, _, show := h.Cursor()
		require.True(t, show)
		assert.Equal(t, term.Coordinates{X: 0, Y: 0}, pos)
	})
}

func TestPluginHandler(t *testing.T) {
	suite := []struct {
		description          string
		cmdAndArgs           string
		maxWidth             int
		frame                bool
		expectConstructorErr error
		drawnComponent       string
	}{
		{
			description: "no cmd and args runs a shell by default",
			cmdAndArgs:  "",
			maxWidth:    4,
			drawnComponent: `
 ◦     sh   0s
$             
              
              
              
              `,
		},
		{
			description: "command with arg",
			cmdAndArgs:  "sleep 2",
			maxWidth:    4,
			drawnComponent: `
 ◦  sleep 2 0s
              
              
              
              
              `,
		},
		{
			description: "max width 0 doesn't panic",
			cmdAndArgs:  "sleep 2",
			maxWidth:    0,
			drawnComponent: `
 ◦  sleep 2 0s
              
              
              
              
              `,
		},
	}

	// important so test correctness doesn't depend on host
	shell := os.Getenv("SHELL")
	defer os.Setenv("SHELL", shell)
	os.Setenv("SHELL", "sh")

	ps1 := os.Getenv("PS1")
	os.Setenv("PS1", "$ ")
	defer os.Setenv("PS1", ps1)

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			ctx := context.Background()
			tempDir, err := os.MkdirTemp("", "")
			require.NoError(t, err)
			uri, err := workspaceapi.ParseURI(filepath.Join("file://", tempDir))
			require.NoError(t, err)
			fileScheme, err := workspace.NewFileScheme(ctx, config.NopConfig(), uri)
			require.NoError(t, err)

			ch := make(chan struct{})
			waitInterrupt := term.FuncInterrupter(func(context.Context) error {
				select {
				case ch <- struct{}{}:
				default:
				}
				return nil
			})
			h, err := New(nopBrowser{interrupt: waitInterrupt}, nopBrowser{}, fileScheme,
				fileScheme, nopBrowser{}, test.cmdAndArgs, test.maxWidth,
				WithFrame(test.frame))
			require.Equal(t, test.expectConstructorErr, err)
			h.Resize(14, 6)

			w := term.NewStringWriter(14, 6)

			tests := []testutil.ComponentTestCase{
				{Action: nil, Expected: test.drawnComponent},
			}

			<-ch
			testutil.TestComponent(t, h, w, tests)
		})
	}
}

type nopBrowser struct {
	interrupt term.Interrupter
}

func (n nopBrowser) PublishEvent(ev term.Event) error {
	if ev.Type != term.EventInterrupt {
		return errors.New("unexpected event type")
	}
	if n.interrupt != nil {
		n.interrupt.Interrupt(context.Background())
	}
	return nil
}

func (n nopBrowser) Notify(notifications.Level, string, ...interface{}) error {
	return nil
}

func (n nopBrowser) NotifyOnce(notifications.Level, string, ...interface{}) error {
	return nil
}

func (n nopBrowser) Tab(uri workspaceapi.URI, name string, h browserapi.Handler) (
	browserapi.Handler, error,
) {
	panic("should not be called")
}

func (n nopBrowser) SetTabName(workspaceapi.URI, string, term.Attributes) error {
	panic("should not be called")
}
