// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package dream

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

//go:embed template
var templateFS embed.FS

const templateVersion = 4

// bootstrapAction describes the action taken by bootstrap.
type bootstrapAction int

const (
	actionNoop     bootstrapAction = iota // already up-to-date
	actionCreated                         // freshly created
	actionUpgraded                        // templates upgraded
)

// bootstrapResult carries the action taken and the pre-upgrade version.
type bootstrapResult struct {
	Action      bootstrapAction
	FromVersion int // workspace version before upgrade (0 = fresh/noop)
}

// bootstrap initializes or upgrades the memory workspace at dataPath.
// Returns the action taken so the caller can run post-bootstrap steps
// (go mod tidy, fix agent for upgrades).
func bootstrap(fsys workspaceapi.FileSystem, dataPath string) (bootstrapResult, error) {
	_, err := fsys.Stat(filepath.Join(dataPath, "go.mod"))
	if errors.Is(err, os.ErrNotExist) {
		// Fresh bootstrap.
		if mkErr := fsys.MkdirAll(dataPath, 0o755); mkErr != nil {
			return bootstrapResult{}, fmt.Errorf("mkdir %s: %w", dataPath, mkErr)
		}
		if wErr := writeTemplates(fsys, dataPath); wErr != nil {
			return bootstrapResult{}, fmt.Errorf("bootstrap: %w", wErr)
		}
		if vErr := writeVersion(fsys, dataPath); vErr != nil {
			return bootstrapResult{}, fmt.Errorf("write version: %w", vErr)
		}
		return bootstrapResult{Action: actionCreated}, nil
	}
	if err != nil {
		return bootstrapResult{}, fmt.Errorf("stat go.mod: %w", err)
	}

	// go.mod exists — check template version.
	current := readVersion(fsys, dataPath)
	if current >= templateVersion {
		return bootstrapResult{}, nil
	}

	// Upgrade: rewrite template files, preserving LLM-generated memory files.
	if wErr := writeTemplates(fsys, dataPath); wErr != nil {
		return bootstrapResult{}, fmt.Errorf("upgrade: %w", wErr)
	}
	if vErr := writeVersion(fsys, dataPath); vErr != nil {
		return bootstrapResult{}, fmt.Errorf("write version: %w", vErr)
	}
	return bootstrapResult{Action: actionUpgraded, FromVersion: current}, nil
}

// writeTemplates writes all embedded template files to dataPath.
func writeTemplates(fsys workspaceapi.FileSystem, dataPath string) error {
	return fs.WalkDir(templateFS, "template", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		data, readErr := templateFS.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read embedded %s: %w", path, readErr)
		}

		// Strip "template/" prefix and ".tmpl" suffix to get the target filename.
		rel := strings.TrimPrefix(path, "template/")
		rel = strings.TrimSuffix(rel, ".tmpl")
		target := filepath.Join(dataPath, rel)

		f, createErr := fsys.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if createErr != nil {
			return fmt.Errorf("create %s: %w", target, createErr)
		}
		defer func() { _ = f.Close() }()

		if _, writeErr := f.Write(data); writeErr != nil {
			return fmt.Errorf("write %s: %w", target, writeErr)
		}
		return nil
	})
}

// readVersion reads the template version from the version
// file. Returns 0 if the file is missing or unparseable.
func readVersion(fsys workspaceapi.FileSystem, dataPath string) int {
	f, err := fsys.OpenFile(filepath.Join(dataPath, "version"), os.O_RDONLY, 0)
	if err != nil {
		return 0
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(f)
	if err != nil {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return v
}

// writeVersion writes the current template version to the
// version file.
func writeVersion(fsys workspaceapi.FileSystem, dataPath string) error {
	f, err := fsys.OpenFile(
		filepath.Join(dataPath, "version"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644,
	)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = fmt.Fprintf(f, "%d\n", templateVersion)
	return err
}
