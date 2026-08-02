// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/benchdraw"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/ide/pkgtrust"
	"unstable.build/go-tui/term/gui"
)

// guiBenchConfig parametrizes a production GUI benchmark session.
type guiBenchConfig struct {
	// pixelsW/pixelsH size the screen in pixels (device scale is
	// pinned to 1, so logical and device pixels coincide).
	pixelsW, pixelsH int
	// transparent selects the transparent-window configuration with
	// fractional window opacity, which switches the renderer blend
	// paths.
	transparent bool
	// files are workspace-relative paths opened through the
	// production startup open path (bootstrapHandler filenames).
	files []string
	// workspaceFiles maps workspace-relative paths to fixture
	// contents staged before the IDE starts.
	workspaceFiles map[string]string
	// syntaxLangs stages the committed tree-sitter grammar fixtures
	// for the given language IDs into the data dir's package lib
	// layout so syntax highlighting engages hermetically.
	syntaxLangs []string
	// forceFullRepaint builds the reference full-repaint GUI (row
	// damage tracking disabled). The differential correctness harness
	// runs each scenario once with it false and once true and asserts
	// identical frame hashes.
	forceFullRepaint bool
}

// guiBenchSession wires the full production GUI stack: a real
// bootstrapHandler-built ide.IDE dispatched through gui.GUI
// Update/Draw, with GPU work flushed by benchdraw.
type guiBenchSession struct {
	tb          testing.TB
	root        *bootstrapHandler
	g           *gui.GUI
	publishChan chan term.Event
	screen      *ebiten.Image
	mu          *sync.Mutex
	workDir     string
	dataDir     string
	cleanups    []func()
	pixelBuf    []byte
}

func (s *guiBenchSession) close() {
	_ = s.root.Close()
	for i := len(s.cleanups) - 1; i >= 0; i-- {
		s.cleanups[i]()
	}
}

// guiBenchConfigYAML renders the deterministic config fixture. The
// builtin font is selected by omitting gui.font_family, the font size
// is pinned so grids are identical across machines, and the upgrade
// auto-check is disabled to keep the session off the network.
func guiBenchConfigYAML(transparent bool) string {
	cfg := `editor:
  mode: modal
upgrade:
  auto_check_enabled: false
gui:
  font_size: 15
`
	if transparent {
		cfg += `  enable_transparent_window: true
  window_opacity:
    fg: 0.9
    bg: 0.7
`
	} else {
		// The default rune.star enables the transparent window, so the
		// opaque variant must switch it back off explicitly.
		cfg += "  enable_transparent_window: false\n"
	}
	return cfg
}

// stageSyntaxFixture copies the committed tree-sitter grammar fixture
// for langID from ide/syntax/syntaxtest into the data dir's installed
// package layout (<dataDir>/lib/<langID>).
func stageSyntaxFixture(tb testing.TB, dataDir, langID string) {
	tb.Helper()
	src := filepath.Join("..", "..", "ide", "syntax", "syntaxtest", langID)
	entries, err := os.ReadDir(src)
	if err != nil {
		tb.Fatalf("read syntax fixture %s: %v", src, err)
	}
	dst := filepath.Join(dataDir, "lib", langID)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		tb.Fatalf("mkdir %s: %v", dst, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			tb.Fatalf("read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o755); err != nil {
			tb.Fatalf("write %s: %v", e.Name(), err)
		}
	}
}

// newGUIBenchSession builds the production GUI stack for a benchmark
// scenario: a bootstrapped bootstrapHandler (the same construction
// path runGUI uses), a gui.GUI built from buildGUIOptions, and a
// screen image sized to cfg. The apiclient points at a local 404
// server and an unreachable gRPC endpoint so nothing leaves the host.
func newGUIBenchSession(tb testing.TB, cfg guiBenchConfig) *guiBenchSession {
	tb.Helper()

	// Keep benchmark result files clean: the IDE logs to logrus at
	// info during construction, which would otherwise interleave with
	// the benchmark output stream.
	prevOut := log.StandardLogger().Out
	prevLevel := log.GetLevel()
	log.SetOutput(io.Discard)
	log.SetLevel(log.PanicLevel)

	s := &guiBenchSession{tb: tb}
	s.cleanups = append(s.cleanups, func() {
		log.SetOutput(prevOut)
		log.SetLevel(prevLevel)
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	s.cleanups = append(s.cleanups, srv.Close)

	s.dataDir = tb.TempDir()
	s.workDir = tb.TempDir()
	for rel, content := range cfg.workspaceFiles {
		path := filepath.Join(s.workDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			tb.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			tb.Fatalf("write %s: %v", path, err)
		}
	}
	for _, lang := range cfg.syntaxLangs {
		stageSyntaxFixture(tb, s.dataDir, lang)
	}

	configPath := filepath.Join(s.dataDir, "config.yaml")
	if err := os.WriteFile(configPath,
		[]byte(guiBenchConfigYAML(cfg.transparent)), 0o644); err != nil {
		tb.Fatalf("write config: %v", err)
	}

	restoreFlags := overrideBootstrapFlags(tb, bootstrapFlagOverrides{
		httpAddress:    srv.URL,
		grpcAddress:    "127.0.0.1:1",
		dataPath:       s.dataDir,
		configPath:     configPath,
		websiteAddress: "https://rune.test",
	})
	s.cleanups = append(s.cleanups, restoreFlags)

	s.mu = new(sync.Mutex)
	s.publishChan = make(chan term.Event, 4096)
	publishEvent := func(ev term.Event) bool {
		select {
		case s.publishChan <- ev:
			return true
		default:
			return false
		}
	}

	var files []string
	for _, rel := range cfg.files {
		files = append(files, filepath.Join(s.workDir, filepath.FromSlash(rel)))
	}

	rootCfg, err := ide.Config(configPath, runeDefaultConfig())
	if err != nil {
		tb.Fatalf("load config: %v", err)
	}

	root, err := newBootstrapHandler(
		s.dataDir, configPath, s.workDir, "" /* zdotDir */, files,
		nil /* launchCmd */, ide.FuncExtensionsRunner(testE2EExtensionsRunner),
		s.mu, publishEvent,
		func(*url.URL) error { return nil }, clipboard.NewInMemory(),
		s.dataDir /* installBackupDir */, rootCfg,
		pkgtrust.NewStore(s.dataDir, nil),
	)
	if err != nil {
		tb.Fatalf("new bootstrap handler: %v", err)
	}
	s.root = root
	if root.realIDE == nil {
		tb.Fatal("bench session must build the configured IDE directly")
	}

	browser := root.browser()
	guiCfg, ok, err := getGUIConfig(root.config())
	if err != nil {
		tb.Fatalf("gui config: %v", err)
	}
	if !ok {
		tb.Fatal("bench config fixture must contain a gui section")
	}
	transparentWindow := getGUITransparentWindow(browser, guiCfg)
	if transparentWindow != cfg.transparent {
		tb.Fatalf("transparent window: config resolved %v, want %v",
			transparentWindow, cfg.transparent)
	}

	// Device scale is pinned before the shared production options run
	// so every font load resolves against scale 1 regardless of the
	// host monitor. Everything after mirrors runGUI exactly.
	options := []gui.Option{gui.WithDeviceScale(1)}
	options = append(options, buildGUIOptions(browser, guiCfg,
		transparentWindow, s.publishChan, s.mu, false)...)
	options = append(options, gui.WithSize(cfg.pixelsW, cfg.pixelsH))
	if cfg.forceFullRepaint {
		options = append(options, gui.WithForceFullRepaint(true))
	}

	g, err := gui.New(root, options...)
	if err != nil {
		tb.Fatalf("gui: %v", err)
	}
	s.g = g
	root.attachGUI(g, transparentWindow)
	if fg, bg := getGUIWindowOpacity(browser, guiCfg); transparentWindow && (fg != 1 || bg != 1) {
		g.SetOpacity(bg, fg)
	}
	if err := root.setupConfiguredIDE(root.realIDE, root.client); err != nil {
		tb.Fatalf("setup ide: %v", err)
	}

	s.screen = ebiten.NewImage(cfg.pixelsW, cfg.pixelsH)
	return s
}

// frame runs one production frame: Update dispatches queued events and
// redraws the handler, Draw renders, and benchdraw flushes the
// enqueued GPU commands synchronously.
func (s *guiBenchSession) frame() {
	if err := s.g.Update(); err != nil {
		s.tb.Fatalf("gui update: %v", err)
	}
	benchdraw.BeginFrame(s.tb)
	s.g.Draw(s.screen)
	benchdraw.EndFrame(s.tb)
}

// frameHash runs one production frame and returns the SHA-256 of the
// resulting screen pixels. ReadPixels must run inside the benchdraw
// frame window (between BeginFrame and EndFrame): the atlas only
// services reads while inFrame, and reading enqueues a flush of the
// pending draw commands through the nop graphics driver benchdraw
// installs. The hash therefore reflects the fully rasterized frame,
// including custom box-drawing and glyph masks, without a window. The
// pixel buffer is reused across calls.
func (s *guiBenchSession) frameHash() [sha256.Size]byte {
	if err := s.g.Update(); err != nil {
		s.tb.Fatalf("gui update: %v", err)
	}
	b := s.screen.Bounds()
	need := 4 * b.Dx() * b.Dy()
	if cap(s.pixelBuf) < need {
		s.pixelBuf = make([]byte, need)
	}
	s.pixelBuf = s.pixelBuf[:need]

	benchdraw.BeginFrame(s.tb)
	s.g.Draw(s.screen)
	s.screen.ReadPixels(s.pixelBuf)
	benchdraw.EndFrame(s.tb)
	return sha256.Sum256(s.pixelBuf)
}

// publish injects an event through the production publish channel; it
// fails the benchmark if the channel is full, so scripted workloads
// notice when they outrun the frame loop.
func (s *guiBenchSession) publish(ev term.Event) {
	select {
	case s.publishChan <- ev:
	default:
		s.tb.Fatalf("publish channel full while injecting %v", ev.Type)
	}
}

// publishKeys turns a plain string into per-rune key events. Newlines
// map to Enter and spaces to the space key.
func (s *guiBenchSession) publishKeys(text string) {
	for _, r := range text {
		switch r {
		case '\n':
			s.publish(term.Event{Type: term.EventKey, Key: term.KeyEnter})
		case ' ':
			s.publish(term.Event{Type: term.EventKey, Key: term.KeySpace, Ch: ' '})
		default:
			s.publish(term.Event{Type: term.EventKey, Ch: r})
		}
	}
}

// settle pumps production frames until the session reaches a steady
// state: all pending workspace installs finished and no frame has
// rendered for a while (init/loading/open shaders expired and all
// scheduled callbacks drained). This mirrors the ebiten game loop
// ticking during startup.
func (s *guiBenchSession) settle(timeout time.Duration) {
	s.tb.Helper()

	installed := make(chan struct{})
	go debug.CapturePanicReport(func() {
		s.root.realIDE.WaitWorkspaces()
		close(installed)
	})

	const quietFrames = 30
	deadline := time.Now().Add(timeout)
	quiet := 0
	workspacesReady := false
	for {
		if time.Now().After(deadline) {
			s.tb.Fatalf("session did not settle within %v", timeout)
		}
		if !workspacesReady {
			select {
			case <-installed:
				workspacesReady = true
			default:
			}
		}
		rendered := s.g.NeedsRender()
		s.frame()
		if workspacesReady && !rendered {
			quiet++
			if quiet >= quietFrames {
				return
			}
		} else {
			quiet = 0
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// guiBenchScenario scripts a workload through the production event
// paths. step runs before every measured frame and publishes whatever
// events the workload calls for.
type guiBenchScenario struct {
	name           string
	files          []string
	workspaceFiles map[string]string
	syntaxLangs    []string
	// openFile is a workspace-relative path opened and focused via the
	// :edit command after the session settles, so scripted keystrokes
	// drive a live editor buffer instead of the home view.
	openFile string
	setup    func(s *guiBenchSession)
	step     func(s *guiBenchSession, frame int)
}

// runGUIBenchScenario is the Tier A entry point: it builds the
// session, runs the scenario's setup, then measures b.N scripted
// frames through the production Update/Draw/flush path.
func runGUIBenchScenario(b *testing.B, sc guiBenchScenario, cfg guiBenchConfig) {
	if testing.Short() {
		b.Skip("gui bench scenarios are not short-mode friendly")
	}
	cfg.files = sc.files
	cfg.workspaceFiles = sc.workspaceFiles
	cfg.syntaxLangs = sc.syntaxLangs
	s := newGUIBenchSession(b, cfg)
	defer s.close()
	s.settle(60 * time.Second)
	if sc.openFile != "" {
		s.openWorkspaceFile(sc.openFile)
	}
	if sc.setup != nil {
		sc.setup(s)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if sc.step != nil {
			sc.step(s, i)
		}
		s.frame()
	}
	b.StopTimer()
}
