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

	"gopkg.in/yaml.v3"
)

// verifyRepoConfig validates the package config of a cloned git
// repository before anything is written to the datadir. The repo must
// ship a config.yaml at its root declaring at least one extension whose
// entrypoint is a source file the extension runner can start (a .py or
// .rs file, or a main.go), and requirements, when present, must be a
// list of non-empty strings.
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
	if err := verifyExtensions(cfg["extensions"]); err != nil {
		return err
	}
	return verifyRequirements(cfg["requirements"])
}

func verifyExtensions(raw any) error {
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
		if err := verifyExtensionPath(path); err != nil {
			return fmt.Errorf("extension %q: %w", id, err)
		}
	}
	return nil
}

// verifyExtensionPath checks that an extension entrypoint is a source
// file the extension runner knows how to start. The rules mirror the
// runner's source-entrypoint rewriting: a .py file, a .rs file, or a
// main.go.
func verifyExtensionPath(path string) error {
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
	return fmt.Errorf("path must point at a source entrypoint "+
		"(a .py or .rs file, or a main.go), got %s", target)
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
