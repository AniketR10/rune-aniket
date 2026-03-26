// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package extension

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"syscall"

	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/browser"
	colorPalette "unstable.build/go-tui/cmd/extension_color_palette/extension"
	fuzzyFile "unstable.build/go-tui/cmd/extension_fuzzy_file/extension"
	fuzzyLine "unstable.build/go-tui/cmd/extension_fuzzy_line/extension"
	fuzzySyntax "unstable.build/go-tui/cmd/extension_fuzzy_syntax/extension"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/extensionv2"
	"unstable.build/go-tui/ide"
)

// NewRunner returns an instance of ide.Extensions that knows
// about Rune's default extensions, so it's able to map an empty
// path to the extension's correct path.
//
// It exposes a extension.Grantee for each of the built-in extensions.
// See New for more details.
func NewRunner(
	ctx context.Context, locker sync.Locker, dataDir, arg0 string,
) (*Extensions, error) {
	legacyBuiltinExtensions := map[string]func() (
		extension.Grantee, []extensionapi.Permission,
	){
		"color_palette": colorPalette.Grantee,
		"fuzzy_file":    fuzzyFile.Grantee,
		"fuzzy_line":    fuzzyLine.Grantee,
		"fuzzy_syntax":  fuzzySyntax.Grantee,
	}

	builtinExtensions := map[string]func() (
		extensionapi.WorkspaceExtension, extensionapi.Metadata){}

	grantor := extension.GrantAll()
	extensionOpts := []extensionv2.Option{
		extensionv2.WithPackageName(debug.Package),
		extensionv2.WithPackageVersion(debug.Tag),
		extensionv2.WithSocketEnv("RUNE_SOCKET"),
		extensionv2.WithDataDirEnv("RUNE_DATADIR"),
		extensionv2.WithAuthCertEnv("RUNE_CERT"),
		extensionv2.WithAuthTokenEnv("RUNE_TOKEN"),
	}
	runner, err := extensionv2.NewRunner(ctx, locker,
		grantor, dataDir, extensionOpts...)
	if err != nil {
		return nil, fmt.Errorf("new extension runner: %v", err)
	}

	return &Extensions{
		runner:                    runner,
		arg0:                      arg0,
		extensionIDToGrantee:      legacyBuiltinExtensions,
		extensionIDToWorkspaceExt: builtinExtensions,
	}, nil
}

// Extensions is a ide.Extensions implementation that also provides
// a extension.Grantee for rune's default extensions to run in a
// separete process.
type Extensions struct {
	runner               ide.ExtensionsRunner
	arg0                 string
	extensionIDToGrantee map[string]func() (
		extension.Grantee, []extensionapi.Permission)
	extensionIDToWorkspaceExt map[string]func() (
		extensionapi.WorkspaceExtension, extensionapi.Metadata)
}

// WorkspaceExtensionsRunner satisfies ide.ExtensionsRunner.
func (p *Extensions) WorkspaceExtensionsRunner(
	uri workspaceapi.URI,
	res map[extensionapi.Permission]extension.ResourceRegistrar,
	dataDir string, notifications browser.Notifications,
	exec schemeapi.Executor,
) (extension.Runner, error) {
	other, err := p.runner.WorkspaceExtensionsRunner(uri, res, dataDir, notifications, exec)
	if err != nil {
		return nil, err
	}
	return extensionsRunner{
		dataDir:                   dataDir,
		arg0:                      p.arg0,
		other:                     other,
		extensionIDToGrantee:      p.extensionIDToGrantee,
		extensionIDToWorkspaceExt: p.extensionIDToWorkspaceExt,
	}, nil
}

// Extension returns a workspace extension that is capable of serving any of Rune's
// default extensions.
func (p *Extensions) Extension(extensionID string) (
	extensionapi.WorkspaceExtension, extensionapi.Metadata, error,
) {
	extensionFn, ok := p.extensionIDToWorkspaceExt[extensionID]
	if ok {
		extension, meta := extensionFn()
		return extension, meta, nil
	}

	granteeFn, ok := p.extensionIDToGrantee[extensionID]
	if ok {
		grantee, perms := granteeFn()
		extension, metadata := extensionv2.NewGranteeShim(extensionID, extensionID,
			debug.Tag, grantee, perms...)
		return extension, metadata, nil
	}

	err := errors.New("unknown built-in extension")
	return nil, extensionapi.Metadata{}, err
}

type extensionsRunner struct {
	arg0                 string
	dataDir              string
	other                extension.Runner
	extensionIDToGrantee map[string]func() (
		extension.Grantee, []extensionapi.Permission)
	extensionIDToWorkspaceExt map[string]func() (
		extensionapi.WorkspaceExtension, extensionapi.Metadata)
}

func (p extensionsRunner) Run(extensionID, path string, config config.Config) (ret error) {
	const runCallType = "runExtension"
	traceID := trace.New()

	// override path if this is one of the built-in extensions
	if _, ok := p.extensionIDToGrantee[extensionID]; ok {
		path = fmt.Sprintf("%s --datadir=%s --rune-extension=%s", p.arg0, p.dataDir, extensionID)
	}
	fields := []logging.Field{
		{Key: "extensionID", Value: extensionID},
		{Key: "path", Value: path},
	}

	attemptAt := logging.LogAttempt(traceID, runCallType, fields...)
	defer func() {
		logging.LogResult(ret, attemptAt, traceID, runCallType, fields...)
	}()

	return p.other.Run(extensionID, path, config)
}

func (p extensionsRunner) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	return p.other.(schemeapi.Executor).StartCommand(ctx, cmd)
}

func (p extensionsRunner) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	return p.other.(schemeapi.Executor).Signal(pid, sig)
}

func (p extensionsRunner) Close() (ret error) {
	return p.other.Close()
}
