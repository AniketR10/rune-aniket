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


package idedebug

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// PkgManager abstracts the ability to resolve package
// directories for a given package identifier.
type PkgManager interface {
	// LibDir returns an iterator of directory paths where
	// the package's binaries may be found.
	LibDir(ctx context.Context, pkgID string) (iterator.Iterator[string], error)
}

type debugConfig struct {
	id      string
	command string
	// args are passed to the debug adapter binary.
	// The placeholder {addr} is replaced at runtime
	// with the TCP address the adapter should listen
	// on (e.g. "127.0.0.1:56789").
	args []string
}

var _debugAdapters = map[string]debugConfig{
	"go": {
		id:      "go",
		command: "dlv",
		args:    []string{"dap", "--listen={addr}"},
	},
}

func debugAdapterForFile(filename workspaceapi.URI) (debugConfig, error) {
	return debugAdapterForFilename(filename.Path())
}

func doDebugAdapterForFile(filename string) (string, error) {
	ext := filepath.Ext(filename)
	switch ext {
	case ".go":
		return "go", nil
	default:
		return "", errors.New("unsupported language")
	}
}

func debugAdapterForFilename(filename string) (debugConfig, error) {
	id, err := doDebugAdapterForFile(filepath.Base(filename))
	if err != nil {
		return debugConfig{}, err
	}
	cfg, ok := _debugAdapters[id]
	if !ok {
		return debugConfig{}, fmt.Errorf("%s language debug adapter is not supported yet", id)
	}
	return cfg, nil
}
