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
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"runtime"
	"sync"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/process"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/gui"
	"unstable.build/go-tui/term/gui/font"
)

const (
	pprofAddr            = ":2260"
	configFilename       = ".gsixrc"
	gsixDefaultWallpaper = "G6"
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

	flagPprof         = flag.Bool("p", false, fmt.Sprintf("start pprof server at %s", pprofAddr))
	flagVersion       = flag.Bool("v", false, "print version information")
	flagWorkspace     = flag.String("w", cwdURI().String(), "workspaceapi.URI")
	flagDPI           = flag.Float64("D", font.DefaultDPI(), "DPI")
	flagFontSize      = flag.Float64("f", 13, "font size")
	flagFontFamily    = flag.String("F", "", "font family, default is builtin font")
	flagFontLigatures = flag.Bool("l", true, "font ligatures")
	flagOpacity       = flag.Float64("o", 1, "opacity")
	flagDeviceScale   = flag.Float64("s", 0, "device scale factor of the monitor. "+
		"The default value detected automatically")
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Error(err)
		home = "."
	}
	defaultConfigPath = path.Join(home, configFilename)
	flagConfigPath = flag.String("c", defaultConfigPath, "config file path")
	defaultDataPath = path.Join(home, ".six")
	flagDataPath = flag.String("d", defaultDataPath, "data directory path")

	Version = fmt.Sprintf("%s (HEAD is %s)", Tag, Commit)
}

func main() {
	var code int
	ok, path, err := debug.CapturePanicReportDir(".", "gsix", Tag, func() {
		code = run()
	})
	if ok {
		os.Exit(code)
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Saved crash report %q\n", path)
	os.Exit(4)
}

func extensionRunner(
	locker sync.Locker,
	uri workspaceapi.URI,
	res map[extension.Permission]extension.ResourceRegistrar,
	dataDir string,
	notifications browser.Notifications,
) (extension.Runner, error) {
	notifications = &protectedNotifications{notifications: notifications, locker: locker}
	extensionOpts := []process.Option{
		process.WithLocker(locker),
		process.WithWorkspace(uri),
		process.WithDataDir(dataDir),
		process.WithPackageName("gsix"),
		process.WithPackageVersion(Tag),
		process.WithNotifications(notifications),
	}
	return process.NewManager(extension.GrantAll(res), extensionOpts...)
}

func run() int {
	var err error
	var filenames []string

	flag.Parse()

	if *flagVersion {
		fmt.Printf("GSix %s\n", Version)
		return 0
	}

	filenames = append(filenames, flag.Args()...)

	if *flagPprof {
		runtime.SetBlockProfileRate(1)
		runtime.SetMutexProfileFraction(1)
		go func() {
			log.Println(http.ListenAndServe(pprofAddr, nil))
		}()
	}

	rpc.DisableGRPCLogging()

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

	strcfg := component.StringConfig{
		Alignment: component.SpanAlignmentCentered,
	}
	wallpaper := browser.Wallpaper{
		NewComponent: func() tui.Component {
			return component.NewStringWithConfig(gsixDefaultWallpaper, strcfg)
		},
	}

	publishChan := make(chan term.Event, 50)
	publishEvent := func(ev term.Event) bool {
		select {
		case publishChan <- ev:
			return true
		default:
			return false
		}
	}

	var mu sync.Mutex
	opts := []ide.Option{
		ide.WithExtensionsRunner(ide.FuncExtensionsRunner(extensionRunner)),
		ide.WithDefaultConfigYAML(defaultConfig),
		ide.WithLocker(&mu),
		ide.WithConfigFilename(configFilename),
		ide.WithDefaultWallpaper(wallpaper),
		ide.WithDefaultConfigYAML(defaultConfig),
		ide.WithBell(func() { /* TODO; nop bell for now */ }),
		ide.WithPublishEvent(publishEvent),
		ide.WithScheduleNextTick(func(fn func()) bool {
			return publishEvent(term.Event{Type: term.EventInterrupt, UserFunc: fn})
		}),
	}

	i, err := ide.New(*flagWorkspace, *flagConfigPath,
		*flagDataPath, filenames, opts...)
	if err != nil {
		fmt.Printf("ide: %s", err)
		log.Errorf("ide: %v", err)
		return 1
	}

	options := []gui.Option{
		gui.WithFontDPI(*flagDPI),
		gui.WithFontSize(*flagFontSize),
		gui.WithDeviceScale(*flagDeviceScale),
		gui.WithFontFamily(*flagFontFamily),
		gui.WithOpacity(float32(*flagOpacity)),
		gui.WithLigatures(*flagFontLigatures),
		gui.WithPublishChannel(publishChan),
		gui.WithLocker(&mu),
		gui.WithDefaultAttributes(i.DefaultAttributes()),
	}

	//for _, hinter := range hinters.All() {
	//	options = append(options, gui.WithHinter(hinter))
	//}
	handler, cleanup := i.Handler()
	defer cleanup()

	g, err := gui.New(handler, options...)
	if err != nil {
		fmt.Printf("gui: %s", err)
		log.Errorf("gui: %v", err)
		return 1
	}

	err = g.Run("gsix")
	if err != nil && !errors.Is(err, gui.ErrHandlerExited) {
		fmt.Printf("%s", err)
		log.Errorf("run: %v", err)
		return 1
	}
	return 0
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

type protectedNotifications struct {
	locker        sync.Locker
	notifications browser.Notifications
}

func (n *protectedNotifications) Notify(
	level notifications.Level, msg string, args ...interface{},
) error {
	n.locker.Lock()
	defer n.locker.Unlock()
	return n.notifications.Notify(level, msg, args...)
}

func (n *protectedNotifications) NotifyOnce(
	level notifications.Level, msg string, args ...interface{},
) error {
	n.locker.Lock()
	defer n.locker.Unlock()
	return n.notifications.NotifyOnce(level, msg, args...)
}
