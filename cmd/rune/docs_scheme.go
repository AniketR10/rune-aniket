// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/workspace"
)

// docsScheme is the URI scheme that exposes the embedded Rune
// documentation as a read-write in-memory workspace. Workspaces are
// opened as "docs:///".
const docsScheme = "docs"

//go:embed all:docs/docs
var docsFS embed.FS

// docsSchemeRoot is the path inside docsFS that is treated as the
// workspace root. Markdown files below this directory are flattened to
// paths relative to it (e.g. "docs/docs/develop/sdk.md" becomes
// "/develop/sdk.md" inside the workspace).
const docsSchemeRoot = "docs/docs"

// newDocsSchemeFunc returns a schemeapi.SchemeFunc for the "docs"
// URI scheme. The returned scheme is backed by an in-memory file
// system that is pre-populated with the embedded markdown files
// rooted at docsSchemeRoot.
func newDocsSchemeFunc() schemeapi.SchemeFunc {
	inner := workspace.NewInMemorySchemeFunc(docsScheme)
	return func(
		ctx context.Context, cfg config.Config, uri workspaceapi.URI,
	) (schemeapi.Scheme, error) {
		s, err := inner(ctx, cfg, uri)
		if err != nil {
			return nil, err
		}
		if err := prefillDocsScheme(s); err != nil {
			_ = s.Close()
			return nil, fmt.Errorf("prefill docs scheme: %w", err)
		}
		return s, nil
	}
}

// prefillDocsScheme writes every markdown file embedded under
// docsSchemeRoot into s, preserving the relative directory structure.
// Paths inside docsFS use forward slashes regardless of host OS.
func prefillDocsScheme(s schemeapi.Scheme) error {
	return fs.WalkDir(docsFS, docsSchemeRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		data, err := docsFS.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read embedded %q: %w", p, err)
		}
		rel := strings.TrimPrefix(p, docsSchemeRoot+"/")
		dst := "/" + rel
		f, err := s.Create(dst)
		if err != nil {
			return fmt.Errorf("create %q: %w", dst, err)
		}
		if _, werr := f.Write(data); werr != nil {
			_ = f.Close()
			return fmt.Errorf("write %q: %w", dst, werr)
		}
		if cerr := f.Close(); cerr != nil {
			return fmt.Errorf("close %q: %w", dst, cerr)
		}
		return nil
	})
}
