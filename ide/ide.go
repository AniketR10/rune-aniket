// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package ide

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"sync/atomic"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/ssh"
)

// IDE encapsulates the ability to run an IDE within a TUI session.
type IDE struct {
	ideConfig
	options
	locker           sync.Locker
	workspaceManager *workspace.Manager
	root             *workspaceManagerHandler
	running          int32
	publishEventFn   EventPublisher
}

// EventPublisher is a function that publishes the given event back
// into the event loop.
type EventPublisher func(term.Event) bool

// New allocates storage for a new IDE and initializes it with config
// at cfgfilename and filename. Note that if filename is empty, a default inmutable
// buffer will be loaded.
func New(
	cwd, cfgfilename, sixDir string,
	filenames []string,
	opts ...Option,
) (i *IDE, err error) {
	i = new(IDE)
	err = i.init(cwd, cfgfilename, "", sixDir, filenames, opts...)
	return
}

// NewRecovery allocates storage for a new IDE and initializes in recovery mode.
// The underlying editor will use recfilename to try to recover file at filename.
// Note that this function panics if either filename or recfilename are empty.
func NewRecovery(
	cwd, cfgfilename, filename, recfilename, sixDir string,
	opts ...Option,
) (i *IDE, err error) {
	if filename == "" || recfilename == "" {
		panic(fmt.Sprintf("invalid input: filename='%s', recfilename='%s'",
			filename, recfilename))
	}
	i = new(IDE)
	err = i.init(cwd, cfgfilename, recfilename, sixDir,
		[]string{filename}, opts...)
	return
}

func (i *IDE) init(
	cwd, cfgfilename, recfilename string, sixDir string,
	filenames []string,
	opts ...Option,
) error {
	op := defaultOptions()
	for _, o := range opts {
		o(&op)
	}
	i.options = op

	configErr := loadConfig(&i.ideConfig, cfgfilename,
		op.defaultWallpaper, op.defaultConfig, op.bell, op.scheduleFn)

	if logPath := i.ideConfig.logOutputPath(); logPath != "" {
		f, err := workspace.OpenFile(logPath,
			os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			expanded, expandErr := workspaceapi.CurrentUserHostURI(logPath)
			if expandErr != nil {
				return multierr.Append(
					fmt.Errorf("open log file %q: %w", logPath, err),
					fmt.Errorf("make uri %q: %w", logPath, expandErr),
				)
			}
			logDir := path.Dir(expanded.Path())
			dirErr := os.MkdirAll(logDir, 0755)
			if dirErr != nil {
				return multierr.Append(
					fmt.Errorf("open log file %q: %w", logPath, err),
					fmt.Errorf("make dir %q: %w", logDir, dirErr),
				)
			}
			f, err = workspace.OpenFile(logPath,
				os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				return fmt.Errorf("open log file %q: %w", logPath, err)
			}
		}

		level := i.ideConfig.logLevel()

		log.SetOutput(f)
		log.SetLevel(level)
		log.SetFormatter(logging.LogrusLogdFormatter{})
	} else {
		log.SetOutput(io.Discard)
		log.SetLevel(log.PanicLevel)
	}

	i.publishEventFn = op.publishEvent
	i.locker = op.locker

	// register default schemes
	workspaceManager := workspace.NewManager(i.ideConfig.workspace())
	err := workspaceManager.RegisterScheme(ssh.Scheme, ssh.New)
	if err != nil {
		return fmt.Errorf("register ssh scheme: %w", err)
	}
	err = workspaceManager.RegisterScheme(workspace.FileScheme, workspace.NewFileScheme)
	if err != nil {
		return fmt.Errorf("register file scheme: %w", err)
	}
	err = workspaceManager.RegisterScheme(workspace.MemoryScheme, workspace.NewMemoryScheme)
	if err != nil {
		return fmt.Errorf("register memory scheme: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("user home dir: %v", err)
	}

	homeDirURI, err := workspaceapi.CurrentUserHostURI(homeDir)
	if err != nil {
		return fmt.Errorf("make home dir uri: %v", err)
	}

	var cwdURI *workspaceapi.URI
	if cwd != "" {
		uri, parseErr := workspaceapi.ParseURI(cwd)
		if parseErr != nil {
			parseErr = fmt.Errorf("could not parse workspace uri: %w", parseErr)
			var pathErr error
			uri, pathErr = workspaceapi.CurrentUserHostURI(cwd)
			if pathErr != nil {
				return multierr.Append(
					parseErr, fmt.Errorf("make current host URI: %w", pathErr))
			}
		}
		cwdURI = &uri
	}

	root, err := newWorkspaceManagerHandler(cwdURI, homeDirURI,
		workspaceManager, i.ideConfig, recfilename, filenames,
		sixDir, i.publishEvent, op.extensionRunner, i.locker, op.extensions,
		func() (ideConfig, error) {
			return reloadConfig(cfgfilename,
				op.defaultWallpaper, op.defaultConfig, op.bell, op.scheduleFn)
		}, op.workspaceConfig, op.tabBarOffset,
		op.tabBarHeight, op.workspacesBarHeight,
		op.workspacesBarOffset, op.workspacesBarFrame)
	if err != nil {
		return fmt.Errorf("new workspace manager: %w", err)
	}
	i.workspaceManager = workspaceManager
	i.root = root
	root.logNonFatalErrs(root.focusBrowser(), configErr, i.ideConfig.errors)

	return nil
}

func (i *IDE) publishEvent(ev term.Event) bool {
	// avoid termbox' screen panicking because
	// some component wants to publish interrupt
	// before we are fully initialized
	running := atomic.LoadInt32(&i.running)
	if running != 1 {
		return false
	}

	return i.publishEventFn(ev)
}

// Interrupt satisfies term.Interrupter
func (i *IDE) Interrupt(ctx context.Context) error {
	payload, _ := term.PayloadFromContext(ctx)
	if !i.publishEvent(term.Event{Type: term.EventInterrupt, Raw: payload}) {
		return errEventStreamNotReady
	}
	return nil
}

// Run initialzes the underlying terminal environment and runs
// it with a workspace handler
func (i *IDE) Run() error {
	err := tui.Init()
	if err != nil {
		return fmt.Errorf("tui init: %w", err)
	}
	atomic.StoreInt32(&i.running, 1)

	term.SetAttr(i.DefaultAttributes())
	term.SetInputMode(i.ideConfig.inputMode())

	var root tui.Handler = i.root
	if i.options.shader != nil {
		const defaultShaderFPS = 30
		shader := shader.New(i.root, i.options.shader,
			i, defaultShaderFPS, i.options.shaderDuration)
		root = handler.WithComponent(i.root, shader)
	}

	err = tui.RunWithLocker(root, i.locker)
	if err != nil {
		return fmt.Errorf("tui run: %w", err)
	}

	return nil
}

// SubscribeCommand subscribes the given handler in calls to the given cmd,
// or returns an error if there's already a CommandHandler
// installed for this command.
//
// The command will be automatically installed to all active
// and future workspaces.
func (c *IDE) SubscribeCommand(
	cmd textapi.CommandManual, handler text.CommandHandler,
) error {
	return c.root.subscribeCommand(cmd, handler)
}

// DefaultAttributes return the default attributes to be used to fill the screen.
func (i *IDE) DefaultAttributes() term.Attributes {
	return i.ideConfig.defaultAttr()
}

// Config returns the configuration loaded by this IDE.
func (i *IDE) Config() config.Config {
	return config.MapConfig(i.ideConfig.cfg)
}

// Handler returns the root Handler of this IDE, and
// a cleanup function when this IDE is no longer in use.
// This can be used insteaf of Run and Close, which
// install this IDE on a TUI system.
func (i *IDE) Handler() (tui.Handler, func()) {
	atomic.StoreInt32(&i.running, 1)
	return i.root, func() {
		running := atomic.CompareAndSwapInt32(&i.running, 1, 0)
		if !running {
			return
		}
		_ = i.closeResources()
	}
}

// Browser returns the current browser in focus.
func (i *IDE) Browser() browser.Browser {
	return i.root.focusBrowser()
}

func (i *IDE) closeResources() (ret error) {
	i.root.mu.Lock()
	defer i.root.mu.Unlock()

	if err := i.root.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}

	if err := i.workspaceManager.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}

	return
}

// Close satisfies io.Closer by closing this all ide's resources, including
// the terminal state.
func (i *IDE) Close() error {
	running := atomic.CompareAndSwapInt32(&i.running, 1, 0)
	if !running {
		return nil
	}
	err := i.closeResources()
	tui.Close()
	return err
}
