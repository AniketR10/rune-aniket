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

package vctrl

import (
	"context"
	"errors"

	"github.com/sourcegraph/go-diff/diff"
)

// Service is an interface that wraps methods to perform Git operations.
type Service interface {
	// Diff returns the file diff for the given file.
	Diff(ctx context.Context, workPath string) (*diff.FileDiff, error)

	// CurrentCommit returns the current commit hash.
	CurrentCommit(ctx context.Context, workPath string) (string, error)

	// RemoteURL returns the remote URL given a remote name.
	RemoteURL(ctx context.Context, workPath string, remoteName string) (string, error)

	// RelPath extracts the path relative to the repository.
	RelPath(ctx context.Context, workPath string) (string, error)
}

var (
	// ErrDiffNoChanges is returned when diff is run and returns no changes.
	ErrDiffNoChanges = errors.New("diff no changes")
)
