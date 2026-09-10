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

package idenotice

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

const (
	noticeDocumentKind   = "workspace-notice"
	noticeDocumentPrefix = "workspace-notice:"
)

type store struct {
	storage storageapi.Service
}

type noticeDocument struct {
	Kind         string
	WorkspaceURI string
	Fingerprint  string
}

func newStore(s storageapi.Service) *store {
	return &store{storage: s}
}

func noticeDocumentID(uri string) string {
	return noticeDocumentPrefix + url.QueryEscape(uri)
}

func (s *store) Shown(ctx context.Context, uri, fingerprint string) (bool, error) {
	var doc noticeDocument
	err := s.storage.Get(ctx, noticeDocumentID(uri), &doc)
	if errors.Is(err, storageapi.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("idenotice: load %q: %w", uri, err)
	}
	return doc.Fingerprint == fingerprint, nil
}

func (s *store) MarkShown(ctx context.Context, uri, fingerprint string) error {
	doc := noticeDocument{
		Kind:         noticeDocumentKind,
		WorkspaceURI: uri,
		Fingerprint:  fingerprint,
	}
	if err := s.storage.Set(ctx, noticeDocumentID(uri), doc); err != nil {
		return fmt.Errorf("idenotice: store %q: %w", uri, err)
	}
	return nil
}
