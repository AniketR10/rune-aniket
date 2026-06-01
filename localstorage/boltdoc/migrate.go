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

package boltdoc

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
)

// LegacyPending reports whether storageDir contains any schemedoc-style
// records that have not yet been migrated to the bolt file boltFileName
// inside the same directory. Returns false when the bolt file already
// exists or when the directory is empty/absent.
func LegacyPending(storageDir, boltFileName string) (bool, error) {
	boltPath := filepath.Join(storageDir, boltFileName)
	if _, err := os.Stat(boltPath); err == nil {
		return false, nil
	}
	entries, err := os.ReadDir(storageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("read legacy storage dir: %w", err)
	}
	files, dirs := classifyLegacyEntries(entries, boltFileName)
	return len(files) > 0 || len(dirs) > 0, nil
}

// MigrateLegacy walks the legacy schemedoc directory layout at storageDir
// (one URL-escaped file per record, one URL-escaped subdirectory per
// partition) and inserts every record into the destination storage. On
// success, the legacy entries are moved into "<storageDir>/../.db.legacy"
// so a recovery path remains for one release.
//
// The migration runs synchronously and aborts on the first error; on
// abort, the legacy directory is left untouched. The caller is responsible
// for treating an abort as fatal (e.g. discarding the partially-populated
// destination and falling back to an in-memory stub) so users do not see
// partial data.
func MigrateLegacy(
	ctx context.Context, dst storageapi.Service,
	storageDir, boltFileName string, marshaler docmarshal.Marshaler,
) error {
	entries, err := os.ReadDir(storageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read legacy storage dir: %w", err)
	}
	rootFiles, partitionDirs := classifyLegacyEntries(entries, boltFileName)
	if len(rootFiles) == 0 && len(partitionDirs) == 0 {
		return nil
	}
	for _, name := range rootFiles {
		if err := migrateFile(ctx, dst, marshaler, storageDir, name); err != nil {
			return err
		}
	}
	for _, partition := range partitionDirs {
		if err := migratePartition(ctx, dst, marshaler, storageDir, partition); err != nil {
			return err
		}
	}
	legacyDir := filepath.Join(filepath.Dir(storageDir), ".db.legacy")
	if err := stashLegacy(storageDir, legacyDir, rootFiles, partitionDirs); err != nil {
		return fmt.Errorf("stash legacy entries: %w", err)
	}
	return nil
}

func classifyLegacyEntries(entries []os.DirEntry, skip string) (files, dirs []string) {
	for _, e := range entries {
		name := e.Name()
		if name == skip {
			continue
		}
		if e.IsDir() {
			dirs = append(dirs, name)
			continue
		}
		files = append(files, name)
	}
	return files, dirs
}

func migrateFile(
	ctx context.Context, svc storageapi.Service, marshaler docmarshal.Marshaler,
	dir, name string,
) error {
	id, err := url.PathUnescape(name)
	if err != nil {
		return fmt.Errorf("decode legacy id %q: %w", name, err)
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return fmt.Errorf("read legacy record %q: %w", name, err)
	}
	var doc map[string]any
	if err := marshaler.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("unmarshal legacy record %q: %w", name, err)
	}
	if err := svc.Create(ctx, id, &doc); err != nil {
		return fmt.Errorf("write migrated record %q: %w", id, err)
	}
	return nil
}

func migratePartition(
	ctx context.Context, svc storageapi.Service, marshaler docmarshal.Marshaler,
	dir, partitionDir string,
) error {
	name, err := url.PathUnescape(partitionDir)
	if err != nil {
		return fmt.Errorf("decode legacy partition %q: %w", partitionDir, err)
	}
	partitionSvc, err := svc.Partition(name)
	if err != nil {
		return fmt.Errorf("create partition %q: %w", name, err)
	}
	subPath := filepath.Join(dir, partitionDir)
	entries, err := os.ReadDir(subPath)
	if err != nil {
		return fmt.Errorf("read legacy partition %q: %w", name, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := migrateFile(ctx, partitionSvc, marshaler, subPath, e.Name()); err != nil {
			return err
		}
	}
	return nil
}

func stashLegacy(srcDir, legacyDir string, files, dirs []string) error {
	if err := os.MkdirAll(legacyDir, 0o777); err != nil {
		return err
	}
	for _, name := range files {
		from := filepath.Join(srcDir, name)
		to := filepath.Join(legacyDir, name)
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	for _, name := range dirs {
		from := filepath.Join(srcDir, name)
		to := filepath.Join(legacyDir, name)
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	return nil
}
