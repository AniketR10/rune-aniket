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

package gitpkg

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

func verifyRepoConfig(dir string) error {
	data, err := os.ReadFile(filepath.Join(dir, "config.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("the repository must contain a config.yaml " +
				"at its root declaring the extensions to install")
		}
		return fmt.Errorf("read config.yaml: %w", err)
	}
	var cfg map[string]any
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config.yaml: %w", err)
	}
	if err := verifyExtensions(cfg["extensions"], dir); err != nil {
		return err
	}
	return verifyRequirements(cfg["requirements"])
}

func verifyExtensions(raw any, dir string) error {
	if raw == nil {
		return fmt.Errorf("config.yaml must declare a top-level " +
			"extensions map with at least one extension")
	}
	extensions, ok := raw.(map[string]any)
	if !ok || len(extensions) == 0 {
		return fmt.Errorf("config.yaml extensions must be a non-empty "+
			"map of extension IDs, got %T", raw)
	}
	ids := make([]string, 0, len(extensions))
	for id := range extensions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		ext, ok := extensions[id].(map[string]any)
		if !ok {
			return fmt.Errorf("extension %q must be a map, got %T",
				id, extensions[id])
		}
		path, _ := ext["path"].(string)
		if err := verifyExtensionPath(path, dir); err != nil {
			return fmt.Errorf("extension %q: %w", id, err)
		}
	}
	return nil
}

func verifyExtensionPath(path, dir string) error {
	entrypoint := strings.Fields(path)
	if len(entrypoint) == 0 {
		return fmt.Errorf("path must not be empty")
	}
	target := entrypoint[0]
	switch filepath.Ext(target) {
	case ".py", ".rs":
		return nil
	case ".go":
		if filepath.Base(target) == "main.go" {
			return nil
		}
		return fmt.Errorf("go entrypoint must be a main.go, got %s", target)
	}
	if resolved, ok := resolvePackageDir(target, dir); ok {
		if err := verifyGoPackageDir(resolved); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return fmt.Errorf("path must point at a source entrypoint "+
		"(a .py or .rs file, a main.go, or a Go package directory "+
		"with a go.mod), got %s", target)
}

func verifyGoPackageDir(dir string) error {
	modPath := filepath.Join(dir, "go.mod")
	data, err := os.ReadFile(modPath)
	if err != nil {
		return err
	}
	mod, err := modfile.Parse(modPath, data, nil)
	if err != nil {
		return fmt.Errorf("parse %s: %w", modPath, err)
	}
	if len(mod.Require) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join(dir, "go.sum")); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("go package directory %s declares module "+
				"requirements but has no go.sum; run `go mod tidy` and "+
				"commit it so the extension can run offline", dir)
		}
		return err
	}
	return nil
}

func resolvePackageDir(target, dir string) (string, bool) {
	for _, prefix := range []string{
		"$RUNE_DATADIR/lib/$RUNE_PKG_ID",
		"${RUNE_DATADIR}/lib/${RUNE_PKG_ID}",
	} {
		if target == prefix {
			return dir, true
		}
		if rest, ok := strings.CutPrefix(target, prefix+"/"); ok {
			return filepath.Join(dir, filepath.FromSlash(rest)), true
		}
	}
	if !strings.Contains(target, "$") {
		return target, true
	}
	return "", false
}

func verifyRequirements(raw any) error {
	if raw == nil {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("config.yaml requirements must be a list "+
			"of package IDs, got %T", raw)
	}
	for _, v := range list {
		s, ok := v.(string)
		if !ok || s == "" {
			return fmt.Errorf("config.yaml requirements entries must be "+
				"non-empty strings, got %v (%T)", v, v)
		}
	}
	return nil
}
