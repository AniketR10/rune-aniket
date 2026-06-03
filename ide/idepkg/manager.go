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

package idepkg

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/ernestrc/go-multierror"
	"github.com/ernestrc/logd-go/logging"
	log "github.com/sirupsen/logrus"
	bluedebug "github.com/unstablebuild/blue/debug"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/blue/release/cdnrelease"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/term"
	"gopkg.in/yaml.v3"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide/starlarkconfig"
	"unstable.build/go-tui/workspace/walkdir"
)

// Sentinel errors returned by release-manager wrappers below so callers
// can branch on the kind of failure with errors.Is without parsing
// messages. The underlying cdnrelease error is preserved in the wrap
// chain for diagnostics.
var (
	// ErrPackageNotFound is returned when the server reports that a
	// requested package does not exist.
	ErrPackageNotFound = errors.New("package not found")
	// ErrVersionNotFound is returned when the server reports that a
	// requested version of a known package does not exist.
	ErrVersionNotFound = errors.New("package version not found")
	// ErrServerUnavailable is returned for transient (5xx) failures
	// from the package server or the signed-URL download backend.
	ErrServerUnavailable = errors.New("package server unavailable")
	// ErrForbidden is returned when the package server rejects the
	// caller with a 403 (no valid token or insufficient subscription).
	// The wrapped message is rendered directly to the user.
	ErrForbidden = errors.New("login first via `login` command and " +
		"ensure you have a valid subscription to download packages")
)

// translatePackageErr maps a *cdnrelease.StatusError on a
// package-scoped call into a user-facing wrapped sentinel. Errors
// without a recognisable status (network errors, non-StatusError
// wraps) are returned unchanged.
func translatePackageErr(err error, pkgID string) error {
	var se *cdnrelease.StatusError
	if !errors.As(err, &se) {
		return err
	}
	switch {
	case se.Status == http.StatusForbidden:
		return ErrForbidden
	case se.Status == http.StatusNotFound:
		return fmt.Errorf("package %q does not exist: %w", pkgID, ErrPackageNotFound)
	case se.Status >= 500:
		return fmt.Errorf("%w (status %d)", ErrServerUnavailable, se.Status)
	}
	return err
}

func translateVersionErr(err error, pkgID, version string) error {
	var se *cdnrelease.StatusError
	if !errors.As(err, &se) {
		return err
	}
	switch {
	case se.URL == "":
		// Signed-URL download from GCS failed
		return fmt.Errorf("download of %q version %q failed: %w (status %d)",
			pkgID, version, ErrServerUnavailable, se.Status)
	case se.Status == http.StatusForbidden:
		return ErrForbidden
	case se.Status == http.StatusNotFound:
		return fmt.Errorf("version %q of package %q does not exist: %w",
			version, pkgID, ErrVersionNotFound)
	case se.Status >= 500:
		return fmt.Errorf("%w (status %d)", ErrServerUnavailable, se.Status)
	}
	return err
}

func translateListErr(err error, pkgID string) error {
	var se *cdnrelease.StatusError
	if !errors.As(err, &se) {
		return err
	}
	switch {
	case se.Status == http.StatusForbidden:
		return ErrForbidden
	case se.Status == http.StatusNotFound && pkgID != "":
		return fmt.Errorf("package %q does not exist: %w", pkgID, ErrPackageNotFound)
	case se.Status >= 500:
		return fmt.Errorf("%w (status %d)", ErrServerUnavailable, se.Status)
	}
	return err
}

// NewManager allocates storage for a new Manager and initializes it.
// The dataDir argument will be used to store downloaded bundles
// and manage executables.
func NewManager(
	n browserapi.Notifications, m release.Manager,
	storage storageapi.Service, scheme schemeapi.Scheme, dataDir string,
	configPath string, wm browserapi.WindowManager,
	scheduleNextTick func(func()) bool,
	interrupter term.Interrupter, opts ...Option,
) *Manager {
	if dataDir == "" {
		panic("data directory must not be empty")
	}
	if configPath == "" {
		panic("config path must not be empty")
	}
	schemeURI, _ := scheme.URI(".")
	binDir := makeBinDirname(dataDir)
	ret := &Manager{
		dataDir:          dataDir,
		configPath:       configPath,
		scheme:           scheme,
		binDir:           binDir,
		schemeURI:        schemeURI,
		interrupter:      interrupter,
		frameCharSet:     component.FrameCharSetDefault(),
		wm:               wm,
		scheduleNextTick: scheduleNextTick,
		n:                n,
		m:                m,
		storage:          storage,
	}
	ret.iterators.m = make(map[string]*sync.Mutex)
	for _, opt := range opts {
		opt(ret)
	}
	return ret
}

// Manager implements ManagerInterface and adds SetPathEnv, which can be used
// to make available downloaded executables via PATH setting.
//
// Note that OS/system is managed by having a separate Manager that points
// to a different underlying release.Manager.
type Manager struct {
	n                browserapi.Notifications
	m                release.Manager
	interrupter      term.Interrupter
	wm               browserapi.WindowManager
	parser           syntaxapi.Parser
	frameCharSet     component.FrameCharSet
	scheduleNextTick func(func()) bool
	storage          storageapi.Service
	dataDir          string
	configPath       string
	scheme           schemeapi.Scheme
	schemeURI        workspaceapi.URI
	binDir           string

	crashReportPkg     string
	crashReportVersion string

	editorMode string

	iterators struct {
		sync.Mutex
		m map[string]*sync.Mutex
	}
}

func (m *Manager) capturePanicReport(f func()) {
	bluedebug.CapturePanic(log.StandardLogger(), m.crashReportPkg, m.crashReportVersion, f)
}

// LibDir returns an iterator to the lib directory of the given package.
// The paths returned by the iterator are always absolute.
func (m *Manager) LibDir(ctx context.Context, pkgID string) (iterator.Iterator[string], error) {
	m.iterators.Lock()
	defer m.iterators.Unlock()

	libDir := makePackageLibDirname(m.dataDir, pkgID)
	ready, ok := m.iterators.m[pkgID]
	if ok {
		return newPendingIterator(ready, m.scheme, m.schemeURI, libDir), nil
	}

	_, err := os.Stat(libDir)
	if err != nil {
		return nil, ErrNotInstalled
	}

	return newReadyIterator(ctx, m.scheme, m.schemeURI, libDir), nil
}

// DescribePackage fetches a Package manifest.
func (m *Manager) DescribePackage(ctx context.Context, pkgID string) (release.Package, error) {
	pkgID = escapeString(pkgID)
	if pkgID == "" {
		return release.Package{}, errors.New("package id must not be empty")
	}
	pkg, err := m.m.GetPackage(ctx, pkgID)
	if err != nil {
		return release.Package{}, translatePackageErr(err, pkgID)
	}
	return pkg, nil
}

// DescribeRelease fetches a release bundle manifest.
func (m *Manager) DescribeRelease(ctx context.Context, pkgID string, version string) (
	release.Bundle, error,
) {
	pkgID = escapeString(pkgID)
	if pkgID == "" {
		return release.Bundle{}, errors.New("package id must not be empty")
	}
	if version == "" {
		return release.Bundle{}, errors.New("release version must not be empty")
	}
	b, err := m.m.Get(ctx, pkgID, release.Version(version),
		release.NopProgressWriter(io.Discard))
	if err != nil {
		return release.Bundle{}, translateVersionErr(err, pkgID, version)
	}
	return b, nil
}

// ListPackages lists all packages.
func (m *Manager) ListPackages(ctx context.Context, filters map[string]string) (
	iterator.Iterator[release.Package], error,
) {
	it, err := m.m.ListPackages(ctx, filters)
	if err != nil {
		return nil, translateListErr(err, "")
	}
	return it, nil
}

// ListPackageVersions lists all bundles of a package.
func (m *Manager) ListPackageVersions(ctx context.Context, pkgID string, filters map[string]string) (
	iterator.Iterator[release.Bundle], error,
) {
	pkgID = escapeString(pkgID)
	it, err := m.m.List(ctx, pkgID, filters)
	if err != nil {
		return nil, translateListErr(err, pkgID)
	}
	return it, nil
}

// InstallPackageVersion downloads a Package bundle by package name and version, reports
// progress via ProgressWriter and returns a release.Bundle and a dirname
// that contains the extracted bundle.
func (m *Manager) InstallPackageVersion(
	ctx context.Context, pkgID string, version release.Version,
	pw repl.ProgressWriter,
) error {
	if pkgID == "" || version == "" {
		return errors.New("package and version must not be empty")
	}
	if pw == nil {
		pw = repl.NopProgressWriter()
	}

	// ensure no one is being naughty
	pkgID = escapeString(pkgID)
	version = release.Version(escapeString(string(version)))

	m.iterators.Lock()
	defer m.iterators.Unlock()

	_, ok := m.iterators.m[pkgID]
	if ok {
		m.log(log.InfoLevel, "there's already an ongoing install of package: %s", pkgID)
		return nil
	}

	tarfile, err := os.CreateTemp("", "")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}

	key := m.makeDownloadKey(pkgID, version)
	err = m.storage.Create(ctx, key, newPkgVersionValue(pkgID, version))
	if err != nil {
		if errors.Is(err, storageapi.ErrAlreadyExists) {
			var existing pkgVersionValue
			if getErr := m.storage.Get(ctx, key, &existing); getErr == nil && !existing.Complete {
				_ = m.storage.Delete(ctx, key)
				_ = os.RemoveAll(makePackageVersionDirname(m.dataDir, pkgID, version))
				_ = os.RemoveAll(makeStagingDirname(m.dataDir, pkgID, version))
				err = m.storage.Create(ctx, key, newPkgVersionValue(pkgID, version))
			}
			if err != nil {
				m.cleanupFile(tarfile)
				return fmt.Errorf("version %s of package %s has "+
					"already been installed", version, pkgID)
			}
		} else {
			m.cleanupFile(tarfile)
			return fmt.Errorf("store package version: %w", err)
		}
	}

	mu := new(sync.Mutex)
	m.iterators.m[pkgID] = mu

	mu.Lock() // block calls to iterator
	go debug.CapturePanicReport(func() {
		m.capturePanicReport(func() {
			m.download(pkgID, version, tarfile, pw, key)
		})
	})

	return err
}

// DeletePackageVersion deletes a package version from local storage. This method is idempotent.
func (m *Manager) DeletePackageVersion(
	ctx context.Context, pkgID string, version release.Version, force bool,
) (ret error) {
	if pkgID == "" || version == "" {
		return errors.New("package and version must not be empty")
	}
	pkgID = escapeString(pkgID)
	version = release.Version(escapeString(string(version)))

	key := m.makeDownloadKey(pkgID, version)
	var val pkgVersionValue
	if err := m.storage.Get(ctx, key, &val); err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			return ErrNotInstalled
		}
		return err
	}

	dirname, libdirname, isInUse, err := m.isPackageVersionInUse(pkgID, version)
	if err != nil {
		ret = multierror.Append(ret, err)
	}
	if isInUse && force {
		// remove lib + executables if package version was being used
		err = os.RemoveAll(libdirname)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
		err = removeExecutables(val.Executables, m.binDir)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	} else if isInUse {
		return ErrVersionInUse
	}

	if err := m.storage.Delete(ctx, key); err != nil {
		ret = multierror.Append(ret, err)
	}

	err = os.RemoveAll(dirname)
	if err != nil {
		ret = multierror.Append(ret, err)
	}
	if ret != nil {
		// best effort to try to keep delete retryable
		_ = m.storage.Set(ctx, key, val)
	}
	return
}

// DeletePackage deletes all bundles of the given package.
func (m *Manager) DeletePackage(
	ctx context.Context, pkgID string,
) error {
	if pkgID == "" {
		return errors.New("package and version must not be empty")
	}
	pkgID = escapeString(pkgID)
	it, err := m.ListInstalledPackageVersions(ctx, pkgID)
	if err != nil {
		return fmt.Errorf("list bundles: %w", err)
	}

	var versions []release.Version
	for {
		next, ok := it.Next(ctx)
		if !ok {
			break
		}

		versions = append(versions, next)
	}
	if err := it.Err(); err != nil {
		return fmt.Errorf("bundles iterator: %w", err)
	}
	if len(versions) == 0 {
		return ErrNotInstalled
	}

	errs := make([]error, len(versions))
	var wg sync.WaitGroup
	wg.Add(len(versions))
	for i := 0; i < len(versions); i++ {
		i := i
		go debug.CapturePanicReport(func() {
			m.capturePanicReport(func() {
				version := versions[i]
				defer wg.Done()
				errs[i] = m.DeletePackageVersion(ctx, pkgID, version, true)
			})
		})
	}
	wg.Wait()

	var ret error
	for _, err := range errs {
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	pkgDirname := filepath.Join(m.dataDir, "pkg", pkgID)
	if err := os.RemoveAll(pkgDirname); err != nil {
		ret = multierror.Append(ret, err)
	}
	return ret
}

// ListInstalledPackages lists the packages installed.
func (m *Manager) ListInstalledPackages(ctx context.Context) (
	iterator.Iterator[string], error,
) {
	dit, err := m.storage.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	it := iterator.FromDocumentIterator[pkgVersionValue](dit)
	complete := iterator.Filter(it, func(p pkgVersionValue) bool {
		return p.Complete
	})
	mapped := iterator.Map(complete, func(p pkgVersionValue) string {
		return p.Package
	})
	seen := make(map[string]struct{})
	return iterator.Filter(mapped, func(pkg string) bool {
		if _, ok := seen[pkg]; ok {
			return false
		}
		seen[pkg] = struct{}{}
		return true
	}), nil
}

// ProcessInstalledSettings processes settings by packages.
func (m *Manager) ProcessInstalledSettings(ctx context.Context) (ret error) {
	dit, err := m.storage.List(ctx, nil)
	if err != nil {
		return err
	}
	it := iterator.FromDocumentIterator[pkgVersionValue](dit)
	slice, err := iterator.ToSlice(ctx, it)
	if err != nil {
		return err
	}
	for _, pkv := range slice {
		if !pkv.Complete {
			continue
		}
		_, _, isInUse, err := m.isPackageVersionInUse(pkv.Package, pkv.Version)
		if err != nil {
			err = fmt.Errorf("could not check if package %s version %s is in use: %v",
				pkv.Package, pkv.Version, err)
			ret = errors.Join(ret, err)
			continue
		}
		if !isInUse {
			continue
		}
		dir := makePackageVersionDirname(m.dataDir, pkv.Package, pkv.Version)
		configFile := pkgConfigFile(dir)
		err = m.processConfig(pkv.Package, pkv.Version, configFile)
		if err != nil {
			ret = errors.Join(ret, fmt.Errorf("process %s: %w", configFile, err))
		}
	}
	return ret
}

// ListInstalledPackageVersions lists the bundles installed for the given package.
func (m *Manager) ListInstalledPackageVersions(ctx context.Context, pkgID string) (
	iterator.Iterator[release.Version], error,
) {
	if pkgID == "" {
		return nil, errors.New("package must not be empty")
	}
	pkgID = escapeString(pkgID)
	dit, err := m.storage.List(ctx, []storageapi.Filter{{
		Field: storageapi.Field{
			FieldPath: []string{"Package"},
			Value:     pkgID,
		},
		Op: storageapi.OpEqual,
	}})
	if err != nil {
		return nil, err
	}
	it := iterator.FromDocumentIterator[pkgVersionValue](dit)
	complete := iterator.Filter(it, func(p pkgVersionValue) bool {
		return p.Complete
	})
	return iterator.Map(complete, func(p pkgVersionValue) release.Version {
		return p.Version
	}), nil
}

// UsePackageVersion updates the lib and bin directories to point to the given
// package version.
func (m *Manager) UsePackageVersion(
	ctx context.Context, pkgID string, version release.Version,
) error {
	if pkgID == "" {
		return errors.New("package must not be empty")
	}
	key := m.makeDownloadKey(pkgID, version)
	var val pkgVersionValue
	if err := m.storage.Get(ctx, key, &val); err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			return ErrNotInstalled
		}
		return err
	}
	if !val.Complete {
		return ErrNotInstalled
	}
	_, _, isInUse, err := m.isPackageVersionInUse(pkgID, version)
	if err != nil {
		return err
	}
	if isInUse {
		return ErrVersionInUse
	}
	pkgID = escapeString(pkgID)
	pkgVersionDirname := makePackageVersionDirname(m.dataDir, pkgID, version)

	configFile := pkgConfigFile(pkgVersionDirname)
	err = m.processConfig(pkgID, version, configFile)
	if err != nil {
		return err
	}
	return m.linkLibCopyBin(pkgID, version, val.Executables, pkgVersionDirname)
}

// PackageVersionInUse returns the package version in use for the given package.
func (m *Manager) PackageVersionInUse(
	ctx context.Context, pkgID string,
) (release.Version, error) {
	if pkgID == "" {
		return "", errors.New("package must not be empty")
	}
	pkgID = escapeString(pkgID)
	it, err := m.m.List(ctx, pkgID, nil)
	if err != nil {
		return "", translateListErr(err, pkgID)
	}

	var versions []release.Version
	for {
		next, ok := it.Next(ctx)
		if !ok {
			break
		}

		versions = append(versions, next.Version)
	}
	if err := it.Err(); err != nil {
		return "", fmt.Errorf("bundles iterator: %w", err)
	}

	latest := release.Version(release.Latest)
	var inUse atomic.Value
	inUse.Store(latest)

	errs := make([]error, len(versions))
	var wg sync.WaitGroup
	wg.Add(len(versions))
	for i := 0; i < len(versions); i++ {
		version := versions[i]
		go debug.CapturePanicReport(func() {
			m.capturePanicReport(func() {
				defer wg.Done()
				var isInUse bool
				_, _, isInUse, errs[i] = m.isPackageVersionInUse(pkgID, version)
				if isInUse { // only one will be in use
					inUse.Store(version)
				}
			})
		})
	}
	wg.Wait()

	var ret error
	for _, err := range errs {
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	if ret != nil {
		return "", ret
	}

	versionInUse := inUse.Load().(release.Version)
	if versionInUse == latest {
		return "", errors.New("no versions of this package are currently in use")
	}
	return versionInUse, nil
}

func (m *Manager) isPackageVersionInUse(
	pkgID string, version release.Version,
) (dirname string, libdirname string, isInUse bool, err error) {
	dirname = makePackageVersionDirname(m.dataDir, pkgID, version)
	libdirname = makePackageLibDirname(m.dataDir, pkgID)
	infodirname, serr := os.Stat(dirname)
	infolibdirname, lerr := os.Stat(libdirname)
	if serr != nil || lerr != nil {
		return
	}
	isInUse = os.SameFile(infodirname, infolibdirname)
	return
}

func (m *Manager) cleanupFile(file *os.File) {
	if err := file.Close(); err != nil {
		m.log(log.WarnLevel, "close temp tar file: %v", err)
	}
	if err := os.Remove(file.Name()); err != nil {
		m.log(log.WarnLevel, "remove temp tar file: %v", err)
	}
}

func (m *Manager) makeDownloadKey(pkgID string, version release.Version) string {
	return fmt.Sprintf("%s:%s", pkgID, version)
}

func newPkgVersionValue(pkgID string, version release.Version) pkgVersionValue {
	return pkgVersionValue{Package: pkgID, Version: version}
}

func (m *Manager) download(
	pkgID string, version release.Version, tarfile *os.File,
	pw repl.ProgressWriter, key string,
) {
	ctx := context.Background()
	defer m.cleanupFile(tarfile)

	if err := makePkgDirs(m.dataDir); err != nil {
		m.abortDownload(err, pkgID, version)
		return
	}

	writer := &progressTarWriter{
		Writer: tarfile,
		pw:     pw,
	}
	m.log(log.TraceLevel, "fetching package %s version %s", pkgID, version)
	_, err := m.m.Get(ctx, pkgID, version, writer)
	if err != nil {
		err = translateVersionErr(err, pkgID, string(version))
		m.abortDownload(err, pkgID, version)
		return
	}

	m.log(log.TraceLevel, "extracting package %s version %s", pkgID, version)
	// extract to staging dir then atomically rename to final dir
	stagingDir := makeStagingDirname(m.dataDir, pkgID, version)
	_ = os.RemoveAll(stagingDir)
	pkgVersionDirname := makePackageVersionDirname(m.dataDir, pkgID, version)
	_, executables, err := m.untar(tarfile, stagingDir, pw)
	if err != nil {
		_ = os.RemoveAll(stagingDir)
		m.abortDownload(err, pkgID, version)
		return
	}

	_ = os.RemoveAll(pkgVersionDirname)
	if err := os.Rename(stagingDir, pkgVersionDirname); err != nil {
		_ = os.RemoveAll(stagingDir)
		err = fmt.Errorf("rename staging dir: %w", err)
		m.abortDownload(err, pkgID, version)
		return
	}

	configFile := pkgConfigFile(pkgVersionDirname)

	err = m.linkLibCopyBin(pkgID, version, executables, pkgVersionDirname)
	if err != nil {
		_ = os.RemoveAll(pkgVersionDirname)
		m.abortDownload(err, pkgID, version)
		return
	}

	updates := []storageapi.Update{
		{FieldPath: []string{"Executables"}, Value: executables},
		{FieldPath: []string{"Complete"}, Value: true},
	}
	if err := m.storage.Update(ctx, key, updates); err != nil {
		err = fmt.Errorf("update storage field: %w", err)
		_ = os.RemoveAll(pkgVersionDirname)
		_ = removeExecutables(executables, m.binDir)
		m.abortDownload(err, pkgID, version)
		return
	}

	if err := m.processConfig(pkgID, version, configFile); err != nil {
		m.log(log.WarnLevel, "process config for package %s version %s: %v",
			pkgID, version, err)
		m.scheduleNextTick(func() {
			_, _ = m.n.Notify(browserapi.LevelError,
				"process configuration for %s version %s: %s",
				pkgID, version, err)
		})
	}

	// Signal completion so notification-backed writers can
	// dismiss the in-progress notification before we post the
	// terminal success notification.
	pw.Progress(1, 1, "done")
	m.scheduleNextTick(func() {
		_, _ = m.n.Notify(browserapi.LevelSuccess,
			"downloaded version %s of package %s", version, pkgID)
	})

	m.iterators.Lock()
	defer m.iterators.Unlock()

	ready, ok := m.iterators.m[pkgID]
	if !ok {
		panic("iterator for package not found")
	}
	delete(m.iterators.m, pkgID)

	ready.Unlock()

	if err := m.interrupter.Interrupt(ctx); err != nil {
		m.log(log.WarnLevel, "interrupt: %v", err)
	}
}

func (m *Manager) abortDownload(
	err error, pkgID string, version release.Version,
) {
	m.log(log.WarnLevel, "aborting installation of package %s version %s: %v",
		pkgID, version, err)
	key := m.makeDownloadKey(pkgID, version)
	if err := m.storage.Delete(context.Background(), key); err != nil {
		m.log(log.ErrorLevel, "delete pkg %s version %s "+
			"lock key (%s): %v", pkgID, version, key, err)
	}
	m.notifyError(err, pkgID, version)

	m.iterators.Lock()
	defer m.iterators.Unlock()

	ready, ok := m.iterators.m[pkgID]
	if !ok {
		panic("iterator for package not found")
	}
	delete(m.iterators.m, pkgID)

	ready.Unlock()
}

func (m *Manager) notifyError(
	err error, pkgID string, version release.Version,
) {
	m.scheduleNextTick(func() {
		_, _ = m.n.Notify(browserapi.LevelError,
			"downloading version %s of package %s failed: %v",
			version, pkgID, err)
	})
	if err := m.interrupter.Interrupt(context.Background()); err != nil {
		m.log(log.WarnLevel, "interrupt: %v", err)
	}
}

func (m *Manager) linkLibVersion(pkgID string, version release.Version) error {
	dirname := makePackageVersionDirname(m.dataDir, pkgID, version)
	libdirname := makePackageLibDirname(m.dataDir, pkgID)
	tmpLink := libdirname + ".tmp"
	_ = os.Remove(tmpLink)
	if err := os.Symlink(dirname, tmpLink); err != nil {
		return fmt.Errorf("symlink lib dir: %w", err)
	}
	if err := os.Rename(tmpLink, libdirname); err != nil {
		_ = os.Remove(tmpLink)
		return fmt.Errorf("rename lib symlink: %w", err)
	}
	return nil
}

func (m *Manager) linkLibCopyBin(
	pkgID string, version release.Version,
	executables []executableEntry, pkgVersionDirname string,
) error {
	m.log(log.TraceLevel, "linking package %s version %s library", pkgID, version)
	err := m.linkLibVersion(pkgID, version)
	if err != nil {
		return err
	}

	m.log(log.TraceLevel, "copying package %s version %s executables", pkgID, version)
	if err := copyExecutables(executables, pkgVersionDirname, m.binDir); err != nil {
		return err
	}
	return nil
}

func (m *Manager) promptConfigChange(
	pkgID string, pkgVersion release.Version, configYAML []byte,
	userDoc, pkgDoc *yaml.Node,
) error {
	message := fmt.Sprintf(
		"Extension %s (version %s) wants to **update** your configuration "+
			"with the following settings:\n\n```yaml\n%s\n```\n\nDo you want to allow this?",
		pkgID, pkgVersion, string(configYAML))

	apply := func() error {
		mergeYAMLNodes(userDoc.Content[0], pkgDoc.Content[0])

		backup, err := backupUserConfig(m.configPath)
		if err != nil {
			return fmt.Errorf("backup user config: %w", err)
		}

		m.log(log.InfoLevel, "created config backup "+
			"before applying package updates: %s", backup)

		if strings.HasSuffix(strings.ToLower(m.configPath), ".star") {
			mergedCfg, err := loadIdePkgConfigFromYAMLDoc(userDoc)
			if err != nil {
				return fmt.Errorf("decode merged yaml doc: %w", err)
			}
			if err := starlarkconfig.WriteConfigFileAtomic(m.configPath, mergedCfg); err != nil {
				return fmt.Errorf("write starlark config: %w", err)
			}
		} else {
			if err := writeYAMLAtomic(m.configPath, userDoc, pkgDoc.Content[0]); err != nil {
				return fmt.Errorf("write config: %w", err)
			}
		}

		return nil
	}

	prompt := handler.NewPrompt(handler.PromptConfig{
		HighlightAttr: term.Attributes{
			Attrs: term.AttrBold,
			Bg:    term.ColorRed,
		},
		OptionAttr: term.Attributes{
			Attrs: term.AttrBold,
			Bg:    term.ColorGray,
		},
		OptionBindings: []term.KeyComb{{Ch: 'a'}, {Ch: 'd'}},
		PromptConfig: component.PromptConfig{
			Message:    message,
			Options:    []string{"    Allow    ", "    Deny    "},
			NewMessage: markdownOrFallback(m.parser, m.scheduleNextTick),
		},
		PromptHandler: handler.FuncPromptHandler(func(idx int, _ string) {
			allowed := idx == 0
			if !allowed {
				return
			}
			if err := apply(); err != nil {
				_, _ = m.n.Notify(browserapi.LevelError, "apply configuration: %s", err)
			} else {
				_, _ = m.n.Notify(browserapi.LevelSuccess, "applied %s "+
					"configuration updates. Restart the program to load the changes.",
					pkgID)
			}
		}, func() error { return nil }),
	})

	ok := m.scheduleNextTick(func() {
		_, err := m.wm.Floating(prompt, browserapi.FloatingConfig{
			Alignment: component.AlignmentCentered,
		})
		if err != nil {
			_, _ = m.n.Notify(browserapi.LevelError, "show config prompt: %s", err)
		}
	})
	if !ok {
		m.log(log.ErrorLevel, "idepkg config prompt: could not schedule")
		return nil
	}

	return nil
}

func (m *Manager) processConfig(
	pkgID string, pkgVersion release.Version, pkgConfigFile string,
) error {
	_, err := os.Stat(pkgConfigFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat config: %v", err)
	}
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(pkgConfigFile)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	if _, statErr := os.Stat(m.configPath); os.IsNotExist(statErr) {
		return nil
	}

	userCfg, err := loadIdePkgConfigFile(m.configPath)
	if err != nil {
		return fmt.Errorf("load user config: %w", err)
	}

	runeVarMapping := func(key string) string {
		switch key {
		case "RUNE_DATADIR":
			return m.dataDir
		case "RUNE_PKG_ID":
			return pkgID
		case "RUNE_PKG_VERSION":
			return string(pkgVersion)
		}
		return ""
	}

	pkgOverlayCfg, err := loadIdePkgConfigOverlay(
		pkgConfigFile, data, map[string]any{},
		pkgID, pkgVersion, m.dataDir, m.editorMode,
	)
	if err != nil {
		return fmt.Errorf("decode package config: %w", err)
	}
	if len(pkgOverlayCfg) == 0 {
		return nil
	}
	expandMapValues(pkgOverlayCfg, runeVarMapping)

	missingCfg := idePkgMissingKeys(userCfg, pkgOverlayCfg)
	if missingCfg == nil {
		return nil
	}

	pkgDoc, err := mapToYAMLDocument(missingCfg)
	if err != nil {
		return fmt.Errorf("package config to yaml: %w", err)
	}
	userDoc, err := mapToYAMLDocument(userCfg)
	if err != nil {
		return fmt.Errorf("user config to yaml: %w", err)
	}

	missingYAML, err := yaml.Marshal(missingCfg)
	if err != nil {
		return fmt.Errorf("marshal missing keys: %w", err)
	}

	err = m.promptConfigChange(pkgID, pkgVersion, missingYAML, userDoc, pkgDoc)
	if err != nil {
		return fmt.Errorf("prompt config change: %w", err)
	}
	return nil
}

func (m *Manager) untar(
	tarfile *os.File, dirname string, pw repl.ProgressWriter,
) (string, []executableEntry, error) {
	if err := os.MkdirAll(dirname, 0777); err != nil {
		err = fmt.Errorf("mkdir: %w", err)
		return "", nil, err
	}
	stat, err := tarfile.Stat()
	if err != nil {
		return "", nil, fmt.Errorf("stat tarball file: %w", err)
	}
	totalBytes := stat.Size()
	_, err = tarfile.Seek(0, 0)
	if err != nil {
		err = fmt.Errorf("seek tarball file: %w", err)
		return "", nil, err
	}

	// Count compressed bytes read from disk so progress tracks
	// against tarfile size (the only total we know up front).
	counter := &countingReader{r: tarfile}
	gzr, err := gzip.NewReader(counter)
	if err != nil {
		err = fmt.Errorf("new gzip reader: %w", err)
		return "", nil, err
	}
	defer func() { _ = gzr.Close() }()

	executables, err := untar(dirname, gzr, func() {
		// Hold back the terminal extract sample so notification
		// writers that auto-dismiss on progress==total stay alive
		// for the "installing" steps that follow.
		n := counter.n
		if n >= totalBytes {
			n = totalBytes - 1
		}
		pw.Progress(n, totalBytes, "extracting")
	})
	if err != nil {
		err = fmt.Errorf("untar into %s: %w", dirname, err)
		return "", nil, err
	}

	configFile := pkgConfigFile(dirname)
	return configFile, executables, nil
}

func loadIdePkgConfigFile(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return loadIdePkgConfigFromBytes(path, data, "", "", "", "")
}

// pkgConfigFile returns the path to the package's settings file inside dir.
// Packages may ship either config.yaml (legacy) or config.star (mode-aware);
// when both exist, config.yaml wins. The returned path may not exist on disk —
// callers must stat it themselves.
func pkgConfigFile(dir string) string {
	yamlPath := filepath.Join(dir, "config.yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return yamlPath
	}
	starPath := filepath.Join(dir, "config.star")
	if _, err := os.Stat(starPath); err == nil {
		return starPath
	}
	return yamlPath
}

func idePkgStarlarkParams(
	pkgID string, pkgVersion release.Version, dataDir string,
	editorMode string,
) map[string]any {
	params := map[string]any{}
	if dataDir != "" {
		params["RUNE_DATADIR"] = dataDir
	}
	if pkgID != "" {
		params["RUNE_PKG_ID"] = pkgID
	}
	if pkgVersion != "" {
		params["RUNE_PKG_VERSION"] = string(pkgVersion)
	}
	if editorMode != "" {
		params["RUNE_EDITOR_MODE"] = editorMode
	}
	return params
}

func loadIdePkgConfigFromBytes(
	filename string, data []byte,
	pkgID string, pkgVersion release.Version, dataDir string,
	editorMode string,
) (map[string]any, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".star") {
		cfg, err := starlarkconfig.Decode(starlarkconfig.Source{
			Src:      data,
			Filename: filename,
			Params:   idePkgStarlarkParams(pkgID, pkgVersion, dataDir, editorMode),
		})
		// A user config file is allowed to be empty or contain only
		// comments. Treat a missing top-level `config` binding as an
		// empty configuration so the package overlay can still create
		// one.
		if errors.Is(err, starlarkconfig.ErrMissingConfig) {
			return map[string]any{}, nil
		}
		return cfg, err
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return normalizeIdePkgConfig(cfg).(map[string]any), nil
}

func loadIdePkgConfigOverlay(
	filename string, data []byte, base map[string]any,
	pkgID string, pkgVersion release.Version, dataDir string,
	editorMode string,
) (map[string]any, error) {
	if strings.HasSuffix(strings.ToLower(filename), ".star") {
		return starlarkconfig.Decode(starlarkconfig.Source{
			Src:      data,
			Filename: filename,
			Params:   idePkgStarlarkParams(pkgID, pkgVersion, dataDir, editorMode),
			Base:     base,
		})
	}
	return loadIdePkgConfigFromBytes(filename, data, pkgID, pkgVersion, dataDir,
		editorMode)
}

func normalizeIdePkgConfig(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeIdePkgConfig(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k.(string)] = normalizeIdePkgConfig(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeIdePkgConfig(val)
		}
		return out
	default:
		return t
	}
}

// idePkgMissingKeys returns the subset of overlay keys that are not
// present in user, preserving overlay values for the keys it returns.
// For keys that are mappings on both sides, it recurses and includes
// only the new sub-keys. The returned map is nil when every overlay
// key (at every nesting level) is already present in user; this signals
// that no prompt and no write is needed. Existing user values are never
// included or compared — they are always preserved.
func idePkgMissingKeys(user, overlay map[string]any) map[string]any {
	var missing map[string]any
	for key, overlayVal := range overlay {
		userVal, ok := user[key]
		if !ok {
			if missing == nil {
				missing = map[string]any{}
			}
			missing[key] = overlayVal
			continue
		}
		overlayMap, overlayIsMap := overlayVal.(map[string]any)
		userMap, userIsMap := userVal.(map[string]any)
		if !overlayIsMap || !userIsMap {
			continue
		}
		nestedMissing := idePkgMissingKeys(userMap, overlayMap)
		if nestedMissing == nil {
			continue
		}
		if missing == nil {
			missing = map[string]any{}
		}
		missing[key] = nestedMissing
	}
	return missing
}

func mapToYAMLDocument(cfg map[string]any) (*yaml.Node, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func loadIdePkgConfigFromYAMLDoc(doc *yaml.Node) (map[string]any, error) {
	data, err := yaml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return normalizeIdePkgConfig(cfg).(map[string]any), nil
}

func newReadyIterator(
	ctx context.Context, scheme schemeapi.Scheme,
	schemeURI workspaceapi.URI, libDir string,
) iterator.Iterator[string] {
	it, err := walkdir.ListFiles(ctx, scheme, libDir)
	if err != nil {
		return iterator.Error[string](fmt.Errorf("list files: %v", err))
	}
	// make paths absolute
	return iterator.Map(it, func(filename string) string {
		path, _ := workspaceapi.ExpandPathWithURI(filename, schemeURI)
		return path
	})
}

func (m *Manager) log(level log.Level, msg string, args ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "idepkg.Manager").Logf(level, msg, args...)
}

// progressTarWriter composes a tarfile io.Writer with a
// caller-supplied repl.ProgressWriter to satisfy
// release.ProgressWriter. The terminal download sample
// (progress==total) is held back so notification-backed writers
// don't auto-dismiss before the extract/install phases run.
type progressTarWriter struct {
	io.Writer
	pw repl.ProgressWriter
}

func (w *progressTarWriter) Progress(progress, total int64, units string) {
	if total == 0 || progress > total {
		return
	}
	if progress == total {
		return
	}
	w.pw.Progress(progress, total, units)
}

// countingReader wraps an io.Reader to track the total number of
// bytes consumed. It is used to report extraction progress against
// the on-disk tarfile size — the only total known up front, since
// tar headers don't expose entry counts.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func isExecutable(info fs.FileInfo) bool {
	// check owner/group/other exec bits, if any
	// match then the file is an executable.
	return info.Mode()&os.ModeType == 0 && info.Mode()&0111 != 0
}

func untar(dst string, r io.Reader, onProgress func()) ([]executableEntry, error) {
	tr := tar.NewReader(r)

	var executables []executableEntry
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar next: %w", err)
		}
		if onProgress != nil {
			onProgress()
		}

		target := filepath.Join(dst, filepath.Clean(hdr.Name))
		if isExecutable(hdr.FileInfo()) && !isHidden(hdr.Name) {
			executables = append(executables, executableEntry{
				Name: hdr.Name,
				Mode: hdr.Mode,
			})
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, hdr.FileInfo().Mode()); err != nil {
				return nil, fmt.Errorf("make dir %s: %w", target, err)
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return nil, fmt.Errorf("make parent dirs for symlink %s: %w", target, err)
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return nil, fmt.Errorf("symlink %s -> %s: %w", target, hdr.Linkname, err)
			}
		case tar.TypeLink:
			/* hard links are ignored */
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return nil, fmt.Errorf("make parent dirs: %w", err)
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC,
				hdr.FileInfo().Mode())
			if err != nil {
				return nil, fmt.Errorf("create file %s: %w", target, err)
			}
			_, err = io.Copy(f, tr)
			_ = f.Close()
			if err != nil {
				return nil, fmt.Errorf("write file %s: %w", target, err)
			}
		}
	}
	return executables, nil
}

func copyExecutables(files []executableEntry, dirname, targetdirname string) error {
	var ret error
	for _, executable := range files {
		name := filepath.Clean(executable.Name)
		orig := filepath.Join(dirname, name)
		origfile, err := os.OpenFile(orig, os.O_RDONLY, 0)
		if err != nil {
			ret = multierror.Append(ret, fmt.Errorf("open executable: %w", err))
			continue
		}
		if err := swapExecutable(origfile, targetdirname, name, executable.Mode); err != nil {
			ret = multierror.Append(ret, err)
		}
		_ = origfile.Close()
	}
	return ret
}

// swapExecutable installs orig into targetdirname by writing a temp
// file and atomically renaming it over the destination. Overwriting in
// place (O_TRUNC) mutates the inode of an already-running binary, which
// macOS Gatekeeper/AMFI detects and kills for unsigned extensions;
// renaming installs a fresh inode and leaves the running process
// untouched.
func swapExecutable(orig *os.File, targetdirname, name string, mode int64) error {
	target := filepath.Join(targetdirname, filepath.Base(name))
	tmp, err := os.CreateTemp(targetdirname, filepath.Base(name)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp executable for %s: %w", target, err)
	}
	tmpName := tmp.Name()
	if _, err := io.Copy(tmp, orig); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("copy executable %s: %w", target, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp executable for %s: %w", target, err)
	}
	if err := os.Chmod(tmpName, os.FileMode(mode)); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod executable %s: %w", target, err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename executable %s: %w", target, err)
	}
	if err := clearQuarantine(target); err != nil {
		return fmt.Errorf("clear quarantine on %s: %w", target, err)
	}
	return nil
}

func isHidden(file string) bool {
	return strings.HasPrefix(filepath.Base(file), ".")
}

func removeExecutables(files []executableEntry, targetdirname string) error {
	var ret error
	for _, executable := range files {
		name := filepath.Clean(executable.Name)
		target := filepath.Join(targetdirname, filepath.Base(name))
		err := os.Remove(target)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			ret = multierror.Append(ret, fmt.Errorf("remove executable %s: %w", target, err))
			continue
		}
	}
	return ret
}

func makeBinDirname(dataDir string) string {
	return filepath.Join(dataDir, "bin")
}

func makeLibDirname(dataDir string) string {
	return filepath.Join(dataDir, "lib")
}

func makePackageVersionDirname(
	dataDir, pkgID string, version release.Version,
) string {
	return filepath.Join(dataDir, "pkg", pkgID, string(version))
}

func makePkgDirs(dataDir string) error {
	binDir := makeBinDirname(dataDir)
	libDir := makeLibDirname(dataDir)
	targets := []string{binDir, libDir}
	for _, target := range targets {
		err := os.MkdirAll(target, 0777)
		if err != nil {
			return fmt.Errorf("mkdir dir %s: %w", target, err)
		}
	}
	return nil
}

func makePackageLibDirname(
	dataDir, pkgID string,
) string {
	return filepath.Join(dataDir, "lib", pkgID)
}

func makeStagingDirname(dataDir, pkgID string, version release.Version) string {
	return filepath.Join(dataDir, "pkg", pkgID, ".staging-"+string(version))
}

type pkgVersionValue struct {
	Package     string
	Version     release.Version
	Executables []executableEntry
	Complete    bool
}

// executableEntry is the UTF-8-safe representation of an executable file
// extracted from a package tarball. The raw [tar.Header] cannot be persisted
// directly because PAX records (for example macOS's
// "com.apple.provenance" xattr) may contain non-UTF-8 bytes
type executableEntry struct {
	Name string
	Mode int64
}

func escapeString(val string) string {
	val = url.PathEscape(val)
	val = strings.ReplaceAll(val, ":", "_")
	return val
}

// Reconcile cleans up incomplete installs left by a previous crash.
// It should be called once at startup, before any new installs.
func (m *Manager) Reconcile(ctx context.Context) error {
	// Phase 1: Clean leftover staging directories.
	pkgRoot := filepath.Join(m.dataDir, "pkg")
	if entries, err := os.ReadDir(pkgRoot); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			pkgDir := filepath.Join(pkgRoot, entry.Name())
			subEntries, err := os.ReadDir(pkgDir)
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				if strings.HasPrefix(sub.Name(), ".staging-") {
					_ = os.RemoveAll(filepath.Join(pkgDir, sub.Name()))
				}
			}
		}
	}

	// Phase 2: Clean stale storage entries (Complete == false).
	dit, err := m.storage.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("list storage entries: %w", err)
	}
	it := iterator.FromDocumentIterator[pkgVersionValue](dit)
	entries, err := iterator.ToSlice(ctx, it)
	if err != nil {
		return fmt.Errorf("read storage entries: %w", err)
	}
	for _, pkv := range entries {
		if pkv.Package == "" || pkv.Version == "" {
			continue
		}
		key := m.makeDownloadKey(pkv.Package, pkv.Version)
		dirname := makePackageVersionDirname(m.dataDir, pkv.Package, pkv.Version)
		if !pkv.Complete {
			_ = os.RemoveAll(dirname)
			// Remove lib symlink if it points to the stale version.
			libdirname := makePackageLibDirname(m.dataDir, pkv.Package)
			if target, lerr := os.Readlink(libdirname); lerr == nil {
				if target == dirname {
					_ = os.Remove(libdirname)
					_ = removeExecutables(pkv.Executables, m.binDir)
				}
			}
			_ = m.storage.Delete(ctx, key)
			continue
		}
		// Phase 4: Verify complete entries — if dir is missing, delete storage.
		if _, serr := os.Stat(dirname); os.IsNotExist(serr) {
			_ = m.storage.Delete(ctx, key)
		}
	}

	// Phase 3: Clean stale .tmp symlinks in lib/.
	libRoot := makeLibDirname(m.dataDir)
	if libEntries, err := os.ReadDir(libRoot); err == nil {
		for _, entry := range libEntries {
			if strings.HasSuffix(entry.Name(), ".tmp") {
				_ = os.Remove(filepath.Join(libRoot, entry.Name()))
			}
		}
	}

	return nil
}

type libDirIterator struct {
	ready     *sync.Mutex
	it        iterator.Iterator[string]
	scheme    schemeapi.Scheme
	schemeURI workspaceapi.URI
	libDir    string
}

func newPendingIterator(
	mu *sync.Mutex, scheme schemeapi.Scheme,
	schemeURI workspaceapi.URI, libDir string,
) *libDirIterator {
	return &libDirIterator{
		ready:     mu,
		scheme:    scheme,
		schemeURI: schemeURI,
		libDir:    libDir,
	}
}

func (l *libDirIterator) Next(ctx context.Context) (string, bool) {
	l.ready.Lock()
	defer l.ready.Unlock()
	if l.it == nil {
		l.it = newReadyIterator(context.Background(), l.scheme, l.schemeURI, l.libDir)
	}
	return l.it.Next(ctx)
}

func (l *libDirIterator) Err() error {
	l.ready.Lock()
	defer l.ready.Unlock()
	if l.it == nil {
		l.it = newReadyIterator(context.Background(), l.scheme, l.schemeURI, l.libDir)
	}
	return l.it.Err()
}

func (l *libDirIterator) Close() error {
	l.ready.Lock()
	defer l.ready.Unlock()
	if l.it == nil {
		return nil
	}
	return l.it.Close()
}
