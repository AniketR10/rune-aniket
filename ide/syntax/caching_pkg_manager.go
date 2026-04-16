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

package syntax

import (
	"context"
	"slices"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/workspace/walkdir"
)

type cachingPkgManager struct {
	pkg   PkgManager
	files sync.Map
}

type cacheablePkgFilesIterator struct {
	iterator.Iterator[string]
	pkg   *cachingPkgManager
	pkgID string
}

func newCachingPkgManager(pkg PkgManager) *cachingPkgManager {
	return &cachingPkgManager{pkg: pkg}
}

func (m *cachingPkgManager) LibDir(ctx context.Context, pkgID string) (iterator.Iterator[string], error) {
	if files, ok := m.files.Load(pkgID); ok {
		return iterator.FromSlice(files.([]string)), nil
	}

	ctx = walkdir.ContextWithWorkerCount(ctx, 1)
	it, err := m.pkg.LibDir(ctx, pkgID)
	if err != nil {
		return nil, err
	}
	return cacheablePkgFilesIterator{Iterator: it, pkg: m, pkgID: pkgID}, nil
}

func (it cacheablePkgFilesIterator) cache(files []string) {
	it.pkg.cache(it.pkgID, files)
}

func (m *cachingPkgManager) cache(pkgID string, files []string) {
	m.files.Store(pkgID, slices.Clone(files))
}
