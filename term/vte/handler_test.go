package vte

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/term"
	vtetest "unstable.build/go-tui/term/vte/test"
	"unstable.build/go-tui/workspace"
)

func TestHandlerIntegration(t *testing.T) {
	cases := []vtetest.Case{
		{"",
			`$ ▐                 
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"ls",
			`$ ls▐               
                    
                    
                    
                    
                    
                    
                    
                    
                    `},
		{"^^echo bla>",
			`$ echo bla          
bla                 
$ ▐                 
                    
                    
                    
                    
                    
                    
                    `},
		{"vi>ihello",
			`hello▐              
~                   
~                   
~                   
~                   
~                   
~                   
~                   
~                   
-- INSERT --        `},
		{"<:quit!>",
			`$ echo bla          
bla                 
$ vi                
$ ▐                 
                    
                    
                    
                    
                    
                    `},
	}

	cfg := DefaultConfig()
	testSequence(t, cfg, 150*time.Millisecond, cases)
}

func testSequence(t *testing.T, cfg Config, timeout time.Duration, cases []vtetest.Case) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(context.Background())
	temp := os.TempDir()

	uri, err := workspaceapi.CurrentUserHostURI(temp)
	require.NoError(t, err)

	scheme, err := workspace.NewFileScheme(ctx, config.NopConfig(), uri)
	require.NoError(t, err)

	ps1 := os.Getenv("PS1")
	os.Setenv("PS1", "$ ")

	ch := make(chan struct{}, 50 /* big enough for the max length sequence of events */)
	cfg.WidthHint = 20
	cfg.HeightHint = 10
	cfg.Shell = "sh" // all systems were this runs should have sh
	handler, err := NewHandler(chanEventPublisher{ch}, nopNotifications{}, scheme, scheme, nopTabManager{}, cfg, "")
	require.NoError(t, err)

	var cbs []func()
	cfg.ScheduleNextTick = func(cb func()) bool {
		cbs = append(cbs, cb)
		return true
	}

	t.Cleanup(func() {
		handler.Close()
		scheme.Close()
		cancel()
		os.Setenv("PS1", ps1)
	})

	vtetest.TestSequence(t, handler, cfg.WidthHint, cfg.HeightHint,
		timeout, ch, cases, func(int) {
			for _, cb := range cbs {
				cb()
			}
			cbs = cbs[:0]
		})
}

type chanEventPublisher struct {
	ch chan struct{}
}

func (p chanEventPublisher) PublishEvent(term.Event) error {
	p.ch <- struct{}{}
	return nil
}

type nopNotifications struct {
}

func (nopNotifications) Notify(level notifications.Level, msg string, args ...any) error {
	return nil
}
