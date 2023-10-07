package termutil

import (
	"os"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

type Option func(t *Terminal)

func WithLogFile(path string) Option {
	return func(t *Terminal) {
		if path == "-" {
			t.logFile = os.Stdout
			return
		}
		t.logFile, _ = os.Create(path)
	}
}

func WithTheme(theme *Theme) Option {
	return func(t *Terminal) {
		t.theme = theme
	}
}

func WithShell(shell string) Option {
	return func(t *Terminal) {
		t.shell = shell
	}
}

func WithWatcher(watcher workspaceapi.Watcher) Option {
	return func(t *Terminal) {
		t.watcher = watcher
	}
}

func WithWindowManipulator(m WindowManipulator) Option {
	return func(t *Terminal) {
		t.windowManipulator = m
	}
}
