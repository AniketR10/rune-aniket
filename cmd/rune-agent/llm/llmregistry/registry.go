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

// ModelEntry describes a model available in the registry.
type ModelEntry struct {
	// Name is the model identifier (e.g. "gpt-4o", "claude-opus-4-6").
	Name string
	// Provider identifies which LLM provider serves this model
	// (e.g. "openai", "anthropic", "gemini", "ollama").
	Provider string
	// ContextWindow is the nominal maximum context window in tokens.
	ContextWindow int
	// BaseURL is the provider-specific API base URL.
	// Empty string means use the provider's default.
	BaseURL string
}

// Registry provides access to available models.
type Registry interface {
	// Models returns an iterator over all registered model entries.
	Models() iterator.Iterator[ModelEntry]
	// Get returns the entry for the given model name.
	Get(ctx context.Context, model string) (ModelEntry, bool)
}

// MutableRegistry is a Registry that supports registration.
type MutableRegistry interface {
	Registry
	// Register adds model entries to the registry.
	Register(entries ...ModelEntry)
}
