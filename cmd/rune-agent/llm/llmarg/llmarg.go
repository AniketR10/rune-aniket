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

// Package llmarg resolves user-supplied model arguments into
// fully-qualified llmapi.ModelEntry values. The host llmrouter is
// strict on empty Provider (see llm/llmrouter); this package is the
// lenient client-side counterpart shared between the rune-agent
// extension, the agentshell REPL, and the claudeimport tool. It
// accepts either `provider/name` or a bare `name`; bare names resolve
// when exactly one provider exposes them, otherwise the returned
// error lists the qualified candidates so the user knows how to
// disambiguate.
package llmarg

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// Resolve returns the fully-qualified llmapi.ModelEntry that matches
// arg in svc's catalog. See the package doc for the disambiguation
// rules.
func Resolve(
	ctx context.Context, svc llmapi.Service, arg string,
) (llmapi.ModelEntry, error) {
	if provider, name, ok := Split(arg); ok {
		entry, err := svc.GetModel(ctx, llmapi.ModelEntry{Provider: provider, Name: name})
		if err != nil {
			if errors.Is(err, llmapi.ErrModelNotFound) {
				return llmapi.ModelEntry{}, fmt.Errorf(
					"model %q is not available. Available models: %s",
					arg, available(ctx, svc))
			}
			return llmapi.ModelEntry{}, err
		}
		return entry, nil
	}
	// A bare name may be a router-side alias (e.g. "default", "query")
	// that the host resolves through GetModel with an empty Provider. A
	// normal model name returns ErrModelNotFound and falls through to the
	// single-provider catalog match below.
	if entry, err := svc.GetModel(ctx, llmapi.ModelEntry{Name: arg}); err == nil {
		return entry, nil
	}
	var matches []llmapi.ModelEntry
	it := svc.Models()
	for {
		entry, ok := it.Next(ctx)
		if !ok {
			break
		}
		if entry.Name == arg {
			matches = append(matches, entry)
		}
	}
	_ = it.Close()
	switch len(matches) {
	case 0:
		return llmapi.ModelEntry{}, fmt.Errorf(
			"model %q is not available. Available models: %s",
			arg, available(ctx, svc))
	case 1:
		return matches[0], nil
	default:
		qualified := make([]string, 0, len(matches))
		for _, m := range matches {
			qualified = append(qualified, m.Provider+"/"+m.Name)
		}
		sort.Strings(qualified)
		return llmapi.ModelEntry{}, fmt.Errorf(
			"model %q is ambiguous: prefix with the intended provider "+
				"(also available as %s)",
			arg, strings.Join(qualified, ", "))
	}
}

// Split splits a `provider/name` argument into its two non-empty
// halves.
func Split(arg string) (provider, name string, ok bool) {
	provider, name, found := strings.Cut(arg, "/")
	if !found || provider == "" || name == "" {
		return "", "", false
	}
	return provider, name, true
}

// Qualify renders entry as the `provider/name` argument that Resolve
// routes deterministically to a single provider. It is the inverse of
// Split. When the provider is unset it returns the bare name, which
// remains valid input for Resolve when the name is unique.
func Qualify(entry llmapi.ModelEntry) string {
	if entry.Provider != "" && entry.Name != "" {
		return entry.Provider + "/" + entry.Name
	}
	return entry.Name
}

// Available returns a sorted, comma-separated list of
// `provider/name` strings for every entry in svc's catalog.
func Available(ctx context.Context, svc llmapi.Service) string {
	return available(ctx, svc)
}

func available(ctx context.Context, svc llmapi.Service) string {
	it := svc.Models()
	defer func() { _ = it.Close() }()
	var names []string
	for {
		e, ok := it.Next(ctx)
		if !ok {
			break
		}
		names = append(names, e.Provider+"/"+e.Name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
