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

// fixtureext is a tiny workspace extension used only by xsandbox
// tests. On startup it reads its config, fetches the workspace
// config over RPC, reads README.md from the workspace, and registers
// a "hello" command that sends a greeting notification when invoked.
// It also watches the workspace root and sends a "saw <file>"
// notification for every created or written file, so the sandbox can
// exercise the workspace.Scheme/Watch stream, and subscribes to
// editor open events, notifying "opened <file>" for each one.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/debug"
)

func main() {
	meta := extensionapi.Metadata{
		DeveloperID:      "Unstable Build",
		DeveloperEmail:   "it@unstable.build",
		DeveloperKey:     "xsandbox-fixture",
		ExtensionID:      "xsandbox_fixture",
		ExtensionName:    "xsandbox fixture",
		ExtensionVersion: "test",
		Permissions: extensionapi.NewPermissions(
			extensionapi.PermissionCommands,
			extensionapi.PermissionNotifications,
			extensionapi.PermissionFileSystem,
			extensionapi.PermissionConfig,
			extensionapi.PermissionEditor,
		),
	}
	ext := extensionapi.FuncWorkspaceExtension(extend)
	if err := extensionapi.ServeWorkspaceExtension(ext, meta); err != nil {
		log.Fatal(err)
	}
}

func extend(
	ctx context.Context, w *extensionapi.Workspace, cfg config.Config,
) error {
	greeting, err := cfg.GetString("greeting")
	if err != nil || greeting == "" {
		greeting = "hello"
	}

	// Exercise the config service RPC in addition to the handshake
	// config already provided in cfg.
	w.Config(ctx).Iterate(func(string, any) {})

	fs := w.FileSystem(ctx)
	f, err := fs.OpenFile("README.md", os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open README.md: %w", err)
	}
	data, err := io.ReadAll(f)
	if cerr := f.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("read README.md: %w", err)
	}
	slog.Info("fixture read workspace file", "file", "README.md", "bytes", len(data))

	notifications := w.Notifications(ctx)
	if err := watchWorkspace(fs, notifications); err != nil {
		return fmt.Errorf("watch workspace: %w", err)
	}

	if err := w.Editor(ctx).SubscribeEvents(
		[]textapi.EventType{textapi.EventTypeOpen},
		openNotifier{notifications: notifications},
	); err != nil {
		return fmt.Errorf("subscribe editor events: %w", err)
	}

	return w.RegisterCommand(textapi.CommandManual{
		Name:     "hello",
		Summary:  "Send a greeting notification",
		Synopsis: "[<name>...]",
	}, helloHandler{
		notifications: notifications,
		greeting:      greeting,
	})
}

// watcher is the subset of the workspace client used to observe
// out-of-band file changes; workspaceapi.FileSystem does not expose
// Watch, but the concrete RPC client does.
type watcher interface {
	Watch(path string, c chan<- schemeapi.EventInfo, events ...schemeapi.Event) (int, error)
}

func watchWorkspace(fs any, notifications browserapi.Notifications) error {
	wt, ok := fs.(watcher)
	if !ok {
		return fmt.Errorf("filesystem client %T does not support watching", fs)
	}
	events := make(chan schemeapi.EventInfo)
	if _, err := wt.Watch(".", events, schemeapi.Create, schemeapi.Write); err != nil {
		return err
	}
	go debug.CapturePanicReport(func() {
		for ev := range events {
			_, err := notifications.Notify(browserapi.LevelInfo,
				"saw %s", path.Base(ev.URI().Path()))
			if err != nil {
				slog.Warn("fixture notify watch event", "error", err)
			}
		}
	})
	return nil
}

// openNotifier reports editor open events back through notifications.
type openNotifier struct {
	notifications browserapi.Notifications
}

func (n openNotifier) Handle(_ context.Context, ev textapi.Event) bool {
	_, err := n.notifications.Notify(browserapi.LevelInfo,
		"opened %s", path.Base(ev.URI.Path()))
	if err != nil {
		slog.Warn("fixture notify open event", "error", err)
	}
	return false
}

type helloHandler struct {
	notifications browserapi.Notifications
	greeting      string
}

func (h helloHandler) HandleCommand(_ context.Context, cmd textapi.Command) error {
	msg := strings.TrimSpace(h.greeting + " " + strings.Join(cmd.Args, " "))
	_, err := h.notifications.Notify(browserapi.LevelInfo, "%s", msg)
	return err
}

func (h helloHandler) Complete(
	context.Context, string, []string,
) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}
