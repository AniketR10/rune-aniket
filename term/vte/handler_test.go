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

// this is the timeout to wait for the shell to stop updating the
// internal state of the vte, to call a test case "complete", so
// assertions can run. The slower the host of the tests, the longer
// this timeout should be.
var defaultWaitForIdleVte = 30 * time.Millisecond

func init() {
	if os.Getenv("NOCI") == "" {
		defaultWaitForIdleVte = 200 * time.Millisecond
	}
}

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
	testSequence(t, cfg, defaultWaitForIdleVte, cases)
}

func testSequence(t *testing.T, cfg Config, timeout time.Duration, cases []vtetest.Case) {
	shell := "sh" // all systems were this runs should have sh
	testSequenceShell(t, cfg, timeout, shell, cases)
}

func testSequenceShell(t *testing.T, cfg Config, timeout time.Duration, shell string, cases []vtetest.Case) {
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
	cfg.Shell = shell
	handler, err := NewHandler(chanEventPublisher{ch}, nopNotifications{},
		scheme, scheme, nopTabManager{}, cfg, "")
	require.NoError(t, err)

	t.Cleanup(func() {
		handler.Close()
		scheme.Close()
		cancel()
		os.Setenv("PS1", ps1)
	})

	vtetest.TestSequence(t, handler, cfg.WidthHint, cfg.HeightHint,
		timeout, ch, cases)
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

func (nopNotifications) NotifyOnce(level notifications.Level, msg string, args ...any) error {
	return nil
}
