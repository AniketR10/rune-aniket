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

	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// CompositeRegistry merges multiple registries. Earlier registries
// take precedence when model names overlap.
type CompositeRegistry struct {
	registries []Registry
}

// NewComposite creates a CompositeRegistry from the given registries.
// Earlier registries take precedence on name collisions.
func NewComposite(registries ...Registry) *CompositeRegistry {
	return &CompositeRegistry{registries: registries}
}

// Models returns a lazy iterator over all model entries from all
// sub-registries, concatenated in registration order.
func (c *CompositeRegistry) Models() iterator.Iterator[ModelEntry] {
	its := make([]iterator.Iterator[ModelEntry], len(c.registries))
	for i, r := range c.registries {
		its[i] = r.Models()
	}
	return iterator.Aggregate(its...)
}

// Get looks up a model across all sub-registries.
// Earlier registries take precedence; later registries are only
// consulted when the model is not found in a preceding one.
func (c *CompositeRegistry) Get(ctx context.Context, model string) (ModelEntry, bool) {
	for _, r := range c.registries {
		if e, ok := r.Get(ctx, model); ok {
			return e, true
		}
	}
	return ModelEntry{}, false
}
