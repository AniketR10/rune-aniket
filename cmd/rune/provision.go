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
	"context"
	"errors"
	"io"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/blue/release/cdnrelease"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/ide/idepkg"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/workspacessh"
)

// provisionRemote runs on the remote `rune -x` server before it starts
// serving. It mirrors the local toolchain by installing the packages named in
// the --install manifest into the remote ~/.rune, then loads the remote config
// (now including any gui.env block merged by those installs, plus the
// workspace-root .rune/config.yaml overlay) and applies gui.env and the
// ~/.rune/bin PATH entry to this process so LSP/tool/debug children inherit a
// working toolchain. It returns the loaded config for the file scheme to reuse.
//
// Failure policy: provisioning failures must never abort the ssh connection.
// Every fallible step warns and continues, and the returned config is always
// usable (falling back to the default config on load errors).
func provisionRemote(
	scheme schemeapi.Scheme, uri workspaceapi.URI,
) config.Config {
	installRemotePackages(scheme)
	return loadRemoteConfigAndApplyEnv(scheme, uri)
}

// installRemotePackages installs and activates every package in the --install
// manifest. Ordering matters: installs must complete before the config is
// loaded, because a package's gui.env block is merged into the remote config
// during UsePackageVersion.
//
// Progress is streamed as JSON Lines to os.Stderr: provisioning runs before
// StartSchemeServer, so stderr is still a plain pipe here and the local side
// (workspacessh) forwards each line to a browser notification.
func installRemotePackages(scheme schemeapi.Scheme) {
	installRemotePackagesTo(scheme, os.Stderr)
}

func installRemotePackagesTo(scheme schemeapi.Scheme, progress io.Writer) {
	entries := idepkg.ParseProvisionManifest(*flagWorkspaceServerInstall)
	if len(entries) == 0 {
		return
	}

	// Release downloads are unauthenticated, matching newAPIClient: the
	// oauth transport fails client-side for logged-out users, which would
	// break installs under the usage-based paywall.
	releaseManager := cdnrelease.NewManager(
		&http.Client{},
		idepkg.ReleasesURL(*flagHTTPAddress, idepkg.HostArch()))

	configBase := func() map[string]any {
		tree, err := ide.DefaultConfigTree(runeDefaultConfig())
		if err != nil {
			log.Warnf("provision: decode default config tree: %v", err)
			return nil
		}
		return tree
	}

	mgr, storage := idepkg.NewProvisioningManager(
		newRuneStorage(*flagDataPath), releaseManager, scheme,
		*flagDataPath, *flagConfigPath, "", configBase)
	defer func() { _ = storage.Close() }()

	ctx := context.Background()
	if err := mgr.Reconcile(ctx); err != nil {
		log.Warnf("provision: reconcile remote packages: %v", err)
	}
	total := len(entries)
	// Installs run sequentially: each package's gui.env is merged into the
	// same user config during activation, and concurrent read-modify-write
	// of that shared file loses env keys. Serializing keeps the merge
	// correct without a lock.
	for i, e := range entries {
		installOnePackage(ctx, mgr, progress, e, i+1, total)
	}
	emitProvisionProgress(progress, workspacessh.ProvisionProgress{
		Index: total, Total: total, Phase: workspacessh.ProvisionPhaseDone,
	})
}

// remoteInstaller is the slice of the provisioning manager installOnePackage
// needs, kept small so the version-fallback logic can be tested without
// network or storage.
type remoteInstaller interface {
	InstallPackageVersion(ctx context.Context, id string, version release.Version, pw repl.ProgressWriter) error
	UsePackageVersion(ctx context.Context, id string, version release.Version) error
	LatestVersion(ctx context.Context, id string) (release.Version, error)
}

// installOnePackage installs and activates a single manifest entry, emitting
// progress. When the pinned version is not published for the remote's platform
// (ErrVersionNotFound), it degrades to the latest available version so the
// remote toolchain still comes up rather than failing on an arch-specific
// version gap. Returns whether the package was installed.
func installOnePackage(
	ctx context.Context, inst remoteInstaller, progress io.Writer,
	e idepkg.ProvisionEntry, index, total int,
) bool {
	emitProvisionProgress(progress, workspacessh.ProvisionProgress{
		Index: index, Total: total, Package: e.ID, Version: e.Version,
		Phase: workspacessh.ProvisionPhaseInstalling,
	})

	version := release.Version(e.Version)
	err := inst.InstallPackageVersion(ctx, e.ID, version, repl.NopProgressWriter())
	if err != nil && errors.Is(err, idepkg.ErrVersionNotFound) {
		// The exact local version may not be published for the remote's
		// platform; fall back to the latest available version.
		latest, lerr := inst.LatestVersion(ctx, e.ID)
		if lerr != nil {
			log.Warnf("provision: resolve latest %s (pinned %s missing): %v", e.ID, e.Version, lerr)
		} else {
			log.Warnf("provision: %s@%s not available for this platform; installing latest %s",
				e.ID, e.Version, latest)
			version = latest
			err = inst.InstallPackageVersion(ctx, e.ID, version, repl.NopProgressWriter())
		}
	}
	// A fully-installed package is an idempotent no-op: fall through to
	// activation instead of reporting a failure, so re-provisioning never
	// surfaces a spurious "already installed" warning.
	if err != nil && errors.Is(err, idepkg.ErrAlreadyInstalled) {
		err = nil
	}
	if err != nil {
		log.Warnf("provision: install %s@%s: %v", e.ID, version, err)
		emitProvisionProgress(progress, workspacessh.ProvisionProgress{
			Index: index, Total: total, Package: e.ID, Version: string(version),
			Phase: workspacessh.ProvisionPhaseFailed, Detail: err.Error(),
		})
		return false
	}
	if err := inst.UsePackageVersion(ctx, e.ID, version); err != nil {
		log.Warnf("provision: use %s@%s: %v", e.ID, version, err)
	}
	emitProvisionProgress(progress, workspacessh.ProvisionProgress{
		Index: index, Total: total, Package: e.ID, Version: string(version),
		Phase: workspacessh.ProvisionPhaseActivating,
	})
	return true
}

// emitProvisionProgress writes one JSON-Lines progress record. Emission is
// best-effort: it never changes the warn-and-continue provisioning policy, so
// marshal and write errors are ignored.
func emitProvisionProgress(w io.Writer, p workspacessh.ProvisionProgress) {
	line, err := workspacessh.EncodeProvisionProgress(p)
	if err != nil {
		return
	}
	_, _ = io.WriteString(w, line)
}

// loadRemoteConfigAndApplyEnv loads the remote ~/.rune config overlaid with
// the workspace-root .rune/config.yaml, applies its gui.env block to this
// process, and prepends ~/.rune/bin to PATH. It returns the loaded config so
// the file scheme can reuse it (e.g. for zdotdir). On any error it warns and
// returns a usable config so serving is never blocked.
func loadRemoteConfigAndApplyEnv(
	scheme schemeapi.Scheme, uri workspaceapi.URI,
) config.Config {
	cwd := workspace.NewSchemeWorkspace(uri, scheme,
		func(fn func()) bool { fn(); return true })
	rootCfg, err := ide.ConfigWithOverlays(
		*flagConfigPath, runeDefaultConfig(), cwd,
		[]string{workspaceConfigFilename},
	)
	if err != nil {
		log.Warnf("provision: load remote config: %v", err)
		if rootCfg == nil {
			rootCfg = config.NopConfig()
		}
	}

	guiCfg, ok, err := getGUIConfig(rootCfg)
	if err != nil {
		log.Warnf("provision: read gui config: %v", err)
	} else if ok {
		if env, err := getGUIEnvVars(guiCfg); err != nil {
			log.Warnf("provision: read gui.env: %v", err)
		} else if err := applyGUIEnvVars(env); err != nil {
			log.Warnf("provision: apply gui.env: %v", err)
		}
	}

	if err := setupRuneBinPATH(*flagDataPath); err != nil {
		log.Warnf("provision: set ~/.rune/bin on PATH: %v", err)
	}
	return rootCfg
}
