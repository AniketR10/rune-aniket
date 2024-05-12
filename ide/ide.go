package ide

import (
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
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/ssh"
)

// IDE encapsulates the ability to run an IDE within a TUI session.
type IDE struct {
	ideConfig
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
	configErr := loadConfig(&i.ideConfig, cfgfilename, op.defaultWallpaper)

	cwdURI, parseErr := workspaceapi.ParseURI(cwd)
	if parseErr != nil {
		parseErr = fmt.Errorf("could not parse workspace uri: %w", parseErr)
		var pathErr error
		cwdURI, pathErr = workspaceapi.CurrentUserHostURI(cwd)
		if pathErr != nil {
			return multierr.Append(
				parseErr, fmt.Errorf("make current host URI: %w", pathErr))
		}
	}

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

	root, err := newWorkspaceManagerHandler(cwdURI, homeDirURI,
		workspaceManager, i.ideConfig, recfilename, filenames,
		sixDir, i.publishEvent, op.extensionRunner, i.locker, op.extensions,
		func() (ideConfig, error) {
			return reloadConfig(cfgfilename, op.defaultWallpaper)
		}, op.workspaceConfig)
	if err != nil {
		return fmt.Errorf("new workspace manager: %w", err)
	}
	i.workspaceManager = workspaceManager
	i.root = root
	wh := i.root.focusHandler().(*workspaceHandler)
	i.root.logNonFatalErrs(wh, configErr, i.ideConfig.errors)

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

// Run initialzes the underlying terminal environment and runs
// it with a workspace handler
func (i *IDE) Run() error {
	err := tui.Init()
	if err != nil {
		return fmt.Errorf("tui init: %w", err)
	}
	atomic.StoreInt32(&i.running, 1)

	term.SetInputMode(i.ideConfig.inputMode())

	err = tui.RunWithLocker(i.root, i.locker)
	if err != nil {
		return fmt.Errorf("tui run: %w", err)
	}

	return nil
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
