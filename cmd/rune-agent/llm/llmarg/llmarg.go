// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

// Package llmarg resolves user-supplied model arguments into
// fully-qualified llmapi.ModelEntry values. The host llmrouter is
// strict on empty Provider (see llm/llmrouter); this package is the
// lenient client-side counterpart shared between the rune-agent
// extension and the agentshell REPL. It accepts either `provider/name` or
// a bare `name`; bare names resolve
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
