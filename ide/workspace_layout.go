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

package ide

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/browser"
	tcomponent "unstable.build/go-tui/component"
	"unstable.build/go-tui/workspace"
)

const (
	workspaceLayoutDocumentKind   = "workspace-layout"
	workspaceLayoutDocumentPrefix = "workspace-layouts:"
)

type workspaceLayoutDocument struct {
	Kind         string
	WorkspaceURI string
	Layout       tcomponent.TileLayout
}

func workspaceLayoutDocumentID(workspaceURI workspaceapi.URI) string {
	return workspaceLayoutDocumentPrefix + url.QueryEscape(workspaceURI.String())
}

func saveWorkspaceLayout(
	ctx context.Context,
	store storageapi.Service,
	workspaceURI workspaceapi.URI,
	layout tcomponent.TileLayout,
) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	err := store.Set(ctx, workspaceLayoutDocumentID(workspaceURI), workspaceLayoutDocument{
		Kind:         workspaceLayoutDocumentKind,
		WorkspaceURI: workspaceURI.String(),
		Layout:       layout,
	})
	if err != nil {
		return fmt.Errorf("save workspace layout %q: %w", workspaceURI.String(), err)
	}
	return nil
}

func loadWorkspaceLayout(
	ctx context.Context,
	store storageapi.Service,
	workspaceURI workspaceapi.URI,
) (tcomponent.TileLayout, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	var doc workspaceLayoutDocument
	err := store.Get(ctx, workspaceLayoutDocumentID(workspaceURI), &doc)
	if errors.Is(err, storageapi.ErrNotFound) {
		return tcomponent.TileLayout{}, false, nil
	}
	if err != nil {
		return tcomponent.TileLayout{}, false, fmt.Errorf(
			"load workspace layout %q: %w", workspaceURI.String(), err)
	}
	return normalizeWorkspaceLayout(doc.Layout), true, nil
}

func normalizeWorkspaceLayout(layout tcomponent.TileLayout) tcomponent.TileLayout {
	if len(layout.Floating) == 0 {
		layout.Floating = nil
	}
	for i := range layout.Children {
		layout.Children[i] = normalizeWorkspaceLayout(layout.Children[i])
	}
	return layout
}

func clearWorkspaceLayout(
	ctx context.Context,
	store storageapi.Service,
	workspaceURI workspaceapi.URI,
) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	if err := store.Delete(ctx, workspaceLayoutDocumentID(workspaceURI)); err != nil &&
		!errors.Is(err, storageapi.ErrNotFound) {
		return fmt.Errorf("clear workspace layout %q: %w", workspaceURI.String(), err)
	}
	return nil
}

func (e *ex) saveWorkspaceLayout(ctx context.Context) error {
	return saveWorkspaceLayout(ctx, e.storage, e.workspaceURI,
		e.comp.Browser().TileLayout())
}

func (e *ex) fileWindowIDs() map[string]uint64 {
	ret := make(map[string]uint64)
	e.comp.Browser().IterateWindows(func(win browser.Window) {
		content, err := win.Content()
		if err != nil {
			return
		}
		tab, ok := content.(*browser.Tab)
		if !ok {
			return
		}
		if _, ok := tab.Closer().(workspace.FlusherCloser); !ok {
			return
		}
		ret[tab.URI().String()] = win.WindowID()
	})
	return ret
}

func (e *ex) loadWorkspaceLayout(ctx context.Context) (tcomponent.TileLayout, bool, error) {
	return loadWorkspaceLayout(ctx, e.storage, e.workspaceURI)
}

func (e *ex) clearWorkspaceLayout(ctx context.Context) error {
	return clearWorkspaceLayout(ctx, e.storage, e.workspaceURI)
}
