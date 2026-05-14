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

package ideupgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// newDarwinFixture returns a fake DMG payload (a single Rune.app
// containing Contents/MacOS/rune) plus the upgradeOpts that drive a
// darwin-flavoured upgrade against a temp install root.
func newDarwinFixture(t *testing.T, fake *fakePlatformOps) (upgradeOpts, string) {
	t.Helper()
	root := t.TempDir()
	cache := t.TempDir()

	const newBinary = "#!/bin/sh\necho new\n"
	fake.dmgPayload = map[string]string{
		"Contents/MacOS/rune": newBinary,
	}
	// Anything works for the artifact bytes; sha256 is verified against this.
	fake.archiveContent = []byte("dmg-bytes")
	sum := sha256.Sum256(fake.archiveContent)

	manifest := Manifest{
		Version:  "v9.9.9",
		Filename: "Rune-v9.9.9.dmg",
		URL:      "https://example.invalid/Rune-v9.9.9.dmg",
		SHA256:   hex.EncodeToString(sum[:]),
		Size:     int64(len(fake.archiveContent)),
	}
	opts := upgradeOpts{
		manifest:         manifest,
		currentVersion:   "v9.9.0",
		cacheDir:         cache,
		installRoot:      root,
		appName:          "Rune.app",
		cliSymlinkPath:   filepath.Join(root, "bin", "rune"),
		cliBinaryRelPath: filepath.Join("Contents", "MacOS", "rune"),
		backupRetention:  1,
		ops:              fake,
	}
	return opts, root
}

// dittoIntoTmp runs runUpgradeDarwin against an existing app bundle
// at installRoot/Rune.app populated with the given content.
func writeExistingApp(t *testing.T, installRoot, content string) {
	t.Helper()
	app := filepath.Join(installRoot, "Rune.app", "Contents", "MacOS", "rune")
	require.NoError(t, os.MkdirAll(filepath.Dir(app), 0o755))
	require.NoError(t, os.WriteFile(app, []byte(content), 0o755))
}

func TestRunUpgradeDarwin_HappyPath(t *testing.T) {
	fake := &fakePlatformOps{}
	opts, root := newDarwinFixture(t, fake)
	writeExistingApp(t, root, "#!/bin/sh\necho old\n")

	err := runUpgrade(context.Background(), opts)
	require.NoError(t, err)

	// New binary is in place.
	got, err := os.ReadFile(filepath.Join(root, "Rune.app", "Contents", "MacOS", "rune"))
	require.NoError(t, err)
	require.Equal(t, "#!/bin/sh\necho new\n", string(got))

	// Backup of the old version was created.
	backup := filepath.Join(root, "Rune.app.bak-v9.9.0")
	_, err = os.Stat(backup)
	require.NoError(t, err)

	// Symlink resolves to the new CLI binary.
	target, err := os.Readlink(opts.cliSymlinkPath)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "Rune.app", "Contents", "MacOS", "rune"), target)

	// Call sequence includes the expected steps.
	seq := fake.callSeq()
	wantSeq := []string{
		"Download:" + opts.manifest.URL,
		"VerifySHA256",
		"MountDMG",
		"AssessGatekeeper",
		"Rename:Rune.app->Rune.app.bak-v9.9.0",
		"Ditto",
		"Symlink",
	}
	for _, want := range wantSeq {
		require.Contains(t, seq, want, "missing call %q in sequence %v", want, seq)
	}
	// Detach is called via deferred runUpgradeDarwin cleanup.
	require.Contains(t, seq, "Detach")
}

func TestRunUpgradeDarwin_SHA256Mismatch(t *testing.T) {
	fake := &fakePlatformOps{}
	opts, root := newDarwinFixture(t, fake)
	writeExistingApp(t, root, "old")
	opts.manifest.SHA256 = strings.Repeat("0", 64)

	err := runUpgrade(context.Background(), opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "verify sha256")

	seq := fake.callSeq()
	require.NotContains(t, seq, "MountDMG")
	require.NotContains(t, seq, "Ditto")

	// Existing bundle is intact.
	got, err := os.ReadFile(filepath.Join(root, "Rune.app", "Contents", "MacOS", "rune"))
	require.NoError(t, err)
	require.Equal(t, "old", string(got))
}

func TestRunUpgradeDarwin_GatekeeperRejection(t *testing.T) {
	fake := &fakePlatformOps{gatekeeperErr: errors.New("denied")}
	opts, root := newDarwinFixture(t, fake)
	writeExistingApp(t, root, "old")

	err := runUpgrade(context.Background(), opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "gatekeeper")

	got, err := os.ReadFile(filepath.Join(root, "Rune.app", "Contents", "MacOS", "rune"))
	require.NoError(t, err)
	require.Equal(t, "old", string(got))

	seq := fake.callSeq()
	require.NotContains(t, seq, "Ditto")
}

func TestRunUpgradeDarwin_DittoFailureRollsBack(t *testing.T) {
	fake := &fakePlatformOps{dittoErr: errors.New("ditto failed")}
	opts, root := newDarwinFixture(t, fake)
	writeExistingApp(t, root, "old")

	err := runUpgrade(context.Background(), opts)
	require.Error(t, err)

	// Old bundle restored.
	got, err := os.ReadFile(filepath.Join(root, "Rune.app", "Contents", "MacOS", "rune"))
	require.NoError(t, err)
	require.Equal(t, "old", string(got))

	// Backup directory removed by rollback.
	_, err = os.Stat(filepath.Join(root, "Rune.app.bak-v9.9.0"))
	require.True(t, os.IsNotExist(err))
}

func TestRunUpgradeDarwin_SymlinkFailureDoesNotRollBack(t *testing.T) {
	fake := &fakePlatformOps{symlinkErr: errors.New("perm denied")}
	opts, root := newDarwinFixture(t, fake)
	writeExistingApp(t, root, "old")

	// Symlink failures are non-fatal now: the bundle is already in
	// place, so runUpgrade surfaces the symlink error via notify and
	// returns nil rather than rolling the whole upgrade back.
	var notified []string
	opts.notify = func(format string, args ...any) {
		notified = append(notified, fmt.Sprintf(format, args...))
	}
	err := runUpgrade(context.Background(), opts)
	require.NoError(t, err)
	require.NotEmpty(t, notified)
	var sawSymlinkWarning bool
	for _, msg := range notified {
		if strings.Contains(msg, "CLI symlink") {
			sawSymlinkWarning = true
			break
		}
	}
	require.True(t, sawSymlinkWarning,
		"expected a notify message about the failed CLI symlink, got %v", notified)

	// New bundle is still in place; backup snapshot is preserved.
	got, err := os.ReadFile(filepath.Join(root, "Rune.app", "Contents", "MacOS", "rune"))
	require.NoError(t, err)
	require.Equal(t, "#!/bin/sh\necho new\n", string(got))
	_, err = os.Stat(filepath.Join(root, "Rune.app.bak-v9.9.0"))
	require.NoError(t, err)
}

func TestRunUpgradeDarwin_PrunesOldBackups(t *testing.T) {
	fake := &fakePlatformOps{}
	opts, root := newDarwinFixture(t, fake)
	writeExistingApp(t, root, "old")

	// Pre-seed two stale backups; only one most recent backup should
	// remain after the upgrade because BackupRetention=1.
	old1 := filepath.Join(root, "Rune.app.bak-v9.0.0")
	old2 := filepath.Join(root, "Rune.app.bak-v9.5.0")
	require.NoError(t, os.MkdirAll(old1, 0o755))
	require.NoError(t, os.MkdirAll(old2, 0o755))
	old := time.Now().Add(-72 * time.Hour)
	require.NoError(t, os.Chtimes(old1, old, old))
	require.NoError(t, os.Chtimes(old2, old.Add(time.Hour), old.Add(time.Hour)))

	err := runUpgrade(context.Background(), opts)
	require.NoError(t, err)

	// The freshly-created backup is the most recent and must remain.
	_, err = os.Stat(filepath.Join(root, "Rune.app.bak-v9.9.0"))
	require.NoError(t, err)
	// One of the older backups should be gone.
	_, errOld1 := os.Stat(old1)
	require.True(t, os.IsNotExist(errOld1), "old backup %s should be pruned", old1)
}

func TestRunUpgradeLinux_HappyPath(t *testing.T) {
	fake := &fakePlatformOps{
		archiveLayout: map[string]string{
			"bin/rune":          "#!/bin/sh\necho new\n",
			"share/rune/README": "hello",
		},
	}
	root := t.TempDir()
	cache := t.TempDir()
	fake.archiveContent = []byte("tarball-bytes")
	sum := sha256.Sum256(fake.archiveContent)

	opts := upgradeOpts{
		manifest: Manifest{
			Version:  "v0.42.1",
			Filename: "rune-v0.42.1.tar.gz",
			URL:      "https://example.invalid/rune-v0.42.1.tar.gz",
			SHA256:   hex.EncodeToString(sum[:]),
		},
		currentVersion:   "v0.42.0",
		cacheDir:         cache,
		installRoot:      root,
		appName:          "rune.app",
		cliSymlinkPath:   filepath.Join(root, "bin-cli", "rune"),
		cliBinaryRelPath: filepath.Join("bin", "rune"),
		backupRetention:  1,
		ops:              fake,
	}

	// Simulate an existing install.
	existing := filepath.Join(root, "rune.app", "bin", "rune")
	require.NoError(t, os.MkdirAll(filepath.Dir(existing), 0o755))
	require.NoError(t, os.WriteFile(existing, []byte("#!/bin/sh\necho old\n"), 0o755))

	err := runUpgradeLinux(context.Background(), opts, filepath.Join(cache, opts.manifest.Filename))
	// runUpgradeLinux is invoked directly because runUpgrade gates on
	// runtime.GOOS. The artifact path is derived the same way runUpgrade
	// would (cache + filename); make sure it exists for the verifier
	// path even though we bypass Download here.
	require.NoError(t, err)

	// Tarball is extracted into place.
	got, err := os.ReadFile(filepath.Join(root, "rune.app", "bin", "rune"))
	require.NoError(t, err)
	require.Equal(t, "#!/bin/sh\necho new\n", string(got))

	// Symlink points at the new binary.
	target, err := os.Readlink(opts.cliSymlinkPath)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "rune.app", "bin", "rune"), target)

	// Backup of the old install was created.
	_, err = os.Stat(filepath.Join(root, "rune.app.bak-v0.42.0"))
	require.NoError(t, err)
}

func TestRunUpgradeLinux_RenameFailureRollsBack(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	fake := &fakePlatformOps{
		archiveLayout: map[string]string{"bin/rune": "new"},
		renameErr: map[string]error{
			// Fail when we try to swap the staging dir into place.
			filepath.Join(root, "rune.app"): errors.New("cross-device"),
		},
		renameOnce: true,
	}
	fake.archiveContent = []byte("t")
	sum := sha256.Sum256(fake.archiveContent)

	opts := upgradeOpts{
		manifest: Manifest{
			Version:  "v2",
			Filename: "rune.tar.gz",
			URL:      "https://example.invalid/rune.tar.gz",
			SHA256:   hex.EncodeToString(sum[:]),
		},
		currentVersion:   "v1",
		cacheDir:         cache,
		installRoot:      root,
		appName:          "rune.app",
		cliSymlinkPath:   filepath.Join(root, "cli", "rune"),
		cliBinaryRelPath: filepath.Join("bin", "rune"),
		backupRetention:  1,
		ops:              fake,
	}

	existing := filepath.Join(root, "rune.app", "bin", "rune")
	require.NoError(t, os.MkdirAll(filepath.Dir(existing), 0o755))
	require.NoError(t, os.WriteFile(existing, []byte("old"), 0o755))

	err := runUpgradeLinux(context.Background(), opts, filepath.Join(cache, opts.manifest.Filename))
	require.Error(t, err)

	// Old install is restored from the backup.
	got, err := os.ReadFile(existing)
	require.NoError(t, err)
	require.Equal(t, "old", string(got))
}
