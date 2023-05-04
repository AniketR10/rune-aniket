package ide

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync/atomic"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/ssh"
)

// IDE encapsulates the ability to run an IDE within a TUI session.
type IDE struct {
	ideConfig
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
	publishEvent EventPublisher,
	pluginRunner Plugins,
	filenames ...string,
) (i *IDE, err error) {
	i = new(IDE)
	err = i.init(cwd, cfgfilename, "", sixDir,
		publishEvent, pluginRunner, filenames...)
	return
}

// NewRecovery allocates storage for a new IDE and initializes in recovery mode.
// The underlying editor will use recfilename to try to recover file at filename.
// Note that this function panics if either filename or recfilename are empty.
func NewRecovery(
	cwd, cfgfilename, filename, recfilename, sixDir string,
	publishEvent EventPublisher,
	pluginRunner Plugins,
) (i *IDE, err error) {
	if filename == "" || recfilename == "" {
		panic(fmt.Sprintf("invalid input: filename='%s', recfilename='%s'",
			filename, recfilename))
	}
	i = new(IDE)
	err = i.init(cwd, cfgfilename, recfilename, sixDir,
		publishEvent, pluginRunner, filename)
	return
}

func (i *IDE) init(cwd, cfgfilename, recfilename string,
	sixDir string,
	publishEvent func(term.Event) bool,
	pluginRunner Plugins,
	filenames ...string,
) error {
	isConfigErr, configErr := loadConfig(&i.ideConfig, cfgfilename)
	// return errors that are not decoding errors but
	// let decoding errors be just logged
	if configErr != nil && !isConfigErr {
		return configErr
	}

	cwdURI, parseErr := workspaceapi.ParseURI(cwd)
	if parseErr != nil {
		var pathErr error
		cwdURI, pathErr = workspaceapi.CurrentUserHostURI(cwd)
		if pathErr != nil {
			return multierr.Append(pathErr, parseErr)
		}
	}

	if i.ideConfig.logOutputPath() != "" {
		f, err := workspace.OpenFile(i.ideConfig.logOutputPath(),
			os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}

		level := i.ideConfig.logLevel()

		log.SetOutput(f)
		log.SetLevel(level)
		fmt := &logging.LogrusFormatter{}
		log.SetFormatter(fmt)
	} else {
		log.SetOutput(ioutil.Discard)
		log.SetLevel(log.PanicLevel)
	}

	i.publishEventFn = publishEvent

	// register default schemes
	workspaceManager := workspace.NewManager(i.ideConfig.workspace())
	err := workspaceManager.RegisterScheme(ssh.Scheme, ssh.New)
	if err != nil {
		return err
	}
	err = workspaceManager.RegisterScheme(workspace.FileScheme, workspace.NewFileScheme)
	if err != nil {
		return err
	}
	err = workspaceManager.RegisterScheme(workspace.MemoryScheme, workspace.NewMemoryScheme)
	if err != nil {
		return err
	}

	root, err := newWorkspaceManagerHandler(cwdURI,
		workspaceManager, i.ideConfig, recfilename, filenames,
		sixDir, i.publishEvent, pluginRunner)
	if err != nil {
		return err
	}
	i.workspaceManager = workspaceManager
	i.root = root
	logNonFatalErrs(configErr, i.ideConfig.errors)

	// we probably couldn't load log path
	// so report to user via stdout
	if configErr != nil {
		// this is best effort. log to stdout is a bulletproof fallback
		fmt.Printf("Config Decode error: %s\n", configErr)
	}

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
		return err
	}
	atomic.StoreInt32(&i.running, 1)

	term.SetOutputMode(i.ideConfig.outputMode())
	term.SetInputMode(i.ideConfig.inputMode())

	err = tui.RunWithLocker(i.root, &i.root.mu)
	if err != nil {
		return err
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
