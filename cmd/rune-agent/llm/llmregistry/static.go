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

package llmregistry

import (
	"context"
	"slices"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// StaticRegistry is a concurrent-safe, in-memory implementation
// of MutableRegistry backed by sync.Map.
type StaticRegistry struct {
	models sync.Map // string → ModelEntry
}

// NewStatic creates a new empty StaticRegistry.
func NewStatic() *StaticRegistry {
	return &StaticRegistry{}
}

// Register adds entries to the registry.
// If a model name already exists, it is overwritten.
func (r *StaticRegistry) Register(entries ...ModelEntry) {
	for _, e := range entries {
		r.models.Store(e.Name, e)
	}
}

// Models returns an iterator over all registered model entries.
func (r *StaticRegistry) Models() iterator.Iterator[ModelEntry] {
	var entries []ModelEntry
	r.models.Range(func(_, v any) bool {
		entries = append(entries, v.(ModelEntry))
		return true
	})
	slices.SortFunc(entries, func(a, b ModelEntry) int {
		return strings.Compare(a.Name, b.Name)
	})
	return iterator.FromSlice(entries)
}

// Get returns the entry for the given model name.
func (r *StaticRegistry) Get(_ context.Context, model string) (ModelEntry, bool) {
	v, ok := r.models.Load(model)
	if !ok {
		return ModelEntry{}, false
	}
	return v.(ModelEntry), true
}
