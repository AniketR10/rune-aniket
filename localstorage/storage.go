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

package localstorage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/firstmover"
	"unstable.build/go-tui/api/config"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/localstorage/schemedoc"
	"unstable.build/go-tui/workspace"
)

// New returns a document.Service storage service that
// uses the local directory dir to setup a local filesystem-based
// multi-process safe, goroutine-safe document.Service.
func New(ctx context.Context, dir string, marshaler docmarshal.Marshaler) (
	*firstmover.Service, error,
) {
	storageDir := filepath.Join(dir, ".db")
	err := os.MkdirAll(storageDir, 0777)
	if err != nil {
		return nil, fmt.Errorf("mkdir: %v", err)
	}
	storageDirURI, err := workspaceapi.CurrentUserHostURI(storageDir)
	if err != nil {
		return nil, fmt.Errorf("URI: %v", err)
	}
	scheme, err := workspace.NewFileScheme(ctx, config.NopConfig(), storageDirURI)
	if err != nil {
		return nil, err
	}
	storage, err := schemedoc.NewDocumentService(scheme, marshaler)
	if err != nil {
		return nil, err
	}

	// place lock path at parent dir of .db
	lockPath := filepath.Join(dir, ".dblock")

	cfg := firstmover.DefaultConfig()
	cfg.Marshaler = marshaler
	cfg.CloseError = schemedoc.ErrClosing
	return firstmover.New(storage, lockPath, cfg), nil
}
