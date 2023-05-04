package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"runtime"
	"sync"
	"syscall"

	"github.com/ernestrc/blue/debug"
	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/plugin/process"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/workspace"
	workspacepb "unstable.build/go-tui/workspace/rpc"
	"unstable.build/go-tui/workspace/ssh"
)

var (
	// compile-time variables
	Tag     = "development"
	Commit  = "HEAD"
	Version string

	defaultConfigPath string
	flagConfigPath    *string
	defaultDataPath   string
	flagDataPath      *string

	flagRecover                = flag.String("r", "", "recover from recovery file")
	flagPprof                  = flag.Bool("p", false, "start pprof server at :6060")
	flagVersion                = flag.Bool("v", false, "print version information")
	flagWorkspace              = flag.String("w", cwdURI().String(), "workspaceapi.URI")
	flagWorkspaceServer        = flag.String("x", "", "runs workspace server from standard input and output")
	flagWorkspaceServerLogFile = flag.String("o", "", "log workspace server TRACE level logs to given file")
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Error(err)
		home = "."
	}
	defaultConfigPath = path.Join(home, ".sixrc")
	flagConfigPath = flag.String("c", defaultConfigPath, "config file path")
	defaultDataPath = path.Join(home, ".six")
	flagDataPath = flag.String("d", defaultDataPath, "data directory path")

	Version = fmt.Sprintf("%s (HEAD is %s)", Tag, Commit)
}

func cwdURI() workspaceapi.URI {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %s", err)
	}
	uri, err := workspaceapi.CurrentUserHostURI(wd)
	if err != nil {
		log.Fatalf("Failed to parse working directory as URI %s: %s", wd, err)
	}
	return uri
}

func startWorkspaceServer() int {
	l := log.New()

	newScheme := workspace.NewFileScheme
	if serverLogs := *flagWorkspaceServerLogFile; serverLogs != "" {
		f, err := workspace.OpenFile(serverLogs,
			os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal(err)
		}
		l.SetOutput(f)
		l.SetLevel(log.TraceLevel)
		l.SetFormatter(&logging.LogrusFormatter{})
		proto.EnableGRPCLogging(f, f, f)

		defer f.Close()
		defer f.Sync()

		newScheme = workspace.LoggingScheme("file", newScheme)

	} else {
		l.SetOutput(ioutil.Discard)
		l.SetLevel(log.PanicLevel)
		proto.DisableGRPCLogging()
	}

	l.Tracef("Initialized debug logger")

	// log unhandled signals for debugging
	ch := make(chan os.Signal, 1)
	quitch := make(chan struct{})
	grpcServer := grpc.NewServer()
	signal.Notify(ch)

	defer close(quitch)
	defer signal.Reset()

	go func() {
		defer grpcServer.Stop()
		for {
			select {
			case sig := <-ch:
				switch sig {
				case syscall.SIGTERM, syscall.SIGINT:
					l.Infof("Received %v signal: cleaning up...", sig)
					return
				case syscall.SIGKILL:
					l.Info("Received SIGKILL signal: exiting")
					os.Exit(1)
				case syscall.SIGURG:
					/* received when socket urgent data is ready to be read */
				default:
					l.Debugf("Received unhandled signal: %#v", sig)
				}
			case <-quitch:
				return
			}
		}
	}()

	uri, err := workspaceapi.CurrentUserHostURI(*flagWorkspaceServer)
	if err != nil {
		l.Error(err)
		return 2
	}
	scheme, err := newScheme(context.Background(), config.NopConfig(), uri)
	if err != nil {
		l.Error(err)
		return 3
	}
	defer scheme.Close()

	server := workspacepb.NewServer(scheme, new(sync.Mutex))
	defer server.Stop()

	err = ssh.StartSchemeServer(l, server, grpcServer)
	if err != nil {
		l.Error(err)
		return 4
	}
	l.Tracef("StartSchemeServer returned with no error")
	return 0
}

func main() {
	var code int
	ok, report := debug.CapturePanic(log.StandardLogger(), "six", Tag, func() {
		code = run()
	})
	if ok {
		os.Exit(code)
	}
	data, err := yaml.Marshal(report)
	if err != nil {
		log.Fatal(err)
	}
	f, err := ioutil.TempFile(".", "six_crash_report_")
	if err != nil {
		log.Fatal(err)
	}
	_, err = f.Write(data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Saved crash report %q\n", f.Name())
	os.Exit(4)
}

func pluginRunner(
	locker sync.Locker,
	uri workspaceapi.URI,
	res map[plugin.Permission]plugin.ResourceRegistrar,
	dataDir string,
) (plugin.Runner, error) {
	pluginOpts := []process.Option{
		process.WithLocker(locker),
		process.WithWorkspace(uri),
		process.WithDataDir(dataDir),
	}
	return process.NewManager(plugin.GrantAll(res), pluginOpts...)
}

func run() int {
	var err error
	var filenames []string

	flag.Parse()

	if *flagVersion {
		fmt.Printf("Six %s\n", Version)
		return 0
	}

	for _, file := range flag.Args() {
		filenames = append(filenames, file)
	}

	if *flagPprof {
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(1)
		go func() {
			log.Println(http.ListenAndServe(":6060", nil))
		}()
	}

	proto.DisableGRPCLogging()

	if *flagWorkspaceServer != "" {
		code := startWorkspaceServer()
		return code
	}

	if *flagConfigPath != defaultConfigPath {
		if _, err := os.Stat(*flagConfigPath); err != nil {
			err = fmt.Errorf("stat %q: %s", *flagConfigPath, err)
			fmt.Printf("%s", err)
			return 1
		}
	}

	// ensure that data path exists
	if _, err := os.Stat(*flagDataPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			err = fmt.Errorf("stat %q: %s", *flagDataPath, err)
			fmt.Printf("%s", err)
			return 1
		}
		if err := os.Mkdir(*flagDataPath, 0777); err != nil {
			err = fmt.Errorf("mkdir %q: %s", *flagDataPath, err)
			fmt.Printf("%s", err)
			return 1
		}
	}

	var i *ide.IDE
	if *flagRecover != "" && len(filenames) != 0 {
		i, err = ide.NewRecovery(*flagWorkspace, *flagConfigPath,
			filenames[0], *flagRecover, *flagDataPath, tui.PublishEvent, ide.FuncPlugins(pluginRunner))
	} else if *flagRecover != "" {
		err = fmt.Errorf("flag -r requires to pass the original filename")
	} else {
		i, err = ide.New(*flagWorkspace, *flagConfigPath,
			*flagDataPath, tui.PublishEvent, ide.FuncPlugins(pluginRunner), filenames...)
	}

	if err != nil {
		fmt.Printf("%s", err)
		return 1
	}

	var ret error
	if err := i.Run(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := i.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if ret != nil {
		fmt.Printf("%s", ret)
		log.Error(ret)
		return 1
	}

	return 0
}
