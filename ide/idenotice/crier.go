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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"unstable.build/go-tui/component/markdown"
	mdhandler "unstable.build/go-tui/handler/markdown"
)

// fileSystem is the subset of workspace.Workspace that the Crier
// needs. Narrowed for testability.
type fileSystem interface {
	OpenFile(name string, flag int, perm os.FileMode) (workspaceapi.File, error)
}

// Crier resolves the configured workspace notice and opens it in a
// floating window.
type Crier struct {
	fs    fileSystem
	wm    browserapi.WindowManager
	cfg   Config
	store *store
}

// New returns a Crier that loads the notice via fs and displays it
// through wm.
func New(fs fileSystem, wm browserapi.WindowManager, cfg Config) *Crier {
	return &Crier{
		fs:    fs,
		wm:    wm,
		cfg:   cfg,
		store: newStore(cfg.Storage),
	}
}

// Show resolves the configured notice and opens a floating window.
// It is a no-op when no notice is configured, and under ShowOnce
// it is also a no-op when the same fingerprint was already shown.
func (c *Crier) Show(ctx context.Context) error {
	content, isMarkdown, err := c.resolveContent()
	if err != nil {
		return err
	}
	if content == "" {
		return nil
	}
	fp := fingerprint(content, isMarkdown)
	if c.cfg.effectiveShow() == ShowOnce {
		shown, err := c.store.Shown(ctx, c.cfg.WorkspaceURI, fp)
		if err != nil {
			return err
		}
		if shown {
			return nil
		}
	}
	if err := c.openFloating(content, isMarkdown); err != nil {
		return err
	}
	if c.cfg.effectiveShow() == ShowOnce {
		if err := c.store.MarkShown(ctx, c.cfg.WorkspaceURI, fp); err != nil {
			return err
		}
	}
	return nil
}

func (c *Crier) resolveContent() (string, bool, error) {
	if c.cfg.Literal != "" {
		return c.cfg.Literal, true, nil
	}
	if c.cfg.Path == "" {
		return "", false, nil
	}
	if c.fs == nil {
		return "", false, errors.New("idenotice: nil filesystem")
	}
	f, err := c.fs.OpenFile(c.cfg.Path, os.O_RDONLY, 0)
	if err != nil {
		return "", false, fmt.Errorf("idenotice: open %q: %w", c.cfg.Path, err)
	}
	defer f.Close() //nolint:errcheck
	b, err := io.ReadAll(f)
	if err != nil {
		return "", false, fmt.Errorf("idenotice: read %q: %w", c.cfg.Path, err)
	}
	isMarkdown := strings.HasSuffix(strings.ToLower(c.cfg.Path), ".md")
	return string(b), isMarkdown, nil
}

// fingerprint mixes the isMarkdown bit into the hash so a path
// renamed from .md to .txt (or vice versa) reshows under ShowOnce.
func fingerprint(content string, isMarkdown bool) string {
	h := sha256.New()
	h.Write([]byte(content))
	if isMarkdown {
		h.Write([]byte{1})
	} else {
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func (c *Crier) openFloating(content string, isMarkdown bool) error {
	if c.wm == nil {
		return errors.New("idenotice: nil window manager")
	}
	if isMarkdown {
		md, err := markdown.New(content)
		if err != nil {
			return fmt.Errorf("idenotice: parse markdown: %w", err)
		}
		mdh := mdhandler.New(md)
		var win browserapi.Window
		bh := browserapi.FuncHandler(mdh, func() error {
			return c.wm.CloseWindow(win)
		})
		floating := browserapi.FuncFloating(bh, mdh.Dimensions)
		win, err = c.wm.Floating(floating, browserapi.FloatingConfig{
			Alignment: component.AlignmentCentered,
		})
		if err != nil {
			return fmt.Errorf("idenotice: open floating: %w", err)
		}
		return nil
	}
	str := plaincomponent(content)
	var win browserapi.Window
	bh := browserapi.FuncHandler(handler.NopFromComponent(str), func() error {
		return c.wm.CloseWindow(win)
	})
	floating := browserapi.FuncFloating(bh, str.Dimensions)
	win, err := c.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		return fmt.Errorf("idenotice: open floating: %w", err)
	}
	return nil
}

func plaincomponent(text string) *component.ResponsiveString {
	return component.NewResponsiveString(text, component.StringResponsiveConfig{
		NoSplitWords: true,
	})
}
