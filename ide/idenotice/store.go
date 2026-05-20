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
