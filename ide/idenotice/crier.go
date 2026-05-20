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
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
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
	fs               fileSystem
	wm               browserapi.WindowManager
	parser           syntaxapi.Parser
	scheduleNextTick func(func()) bool
	onLinkClick      func(*url.URL) bool
	cfg              Config
	store            *store
}

// New returns a Crier that loads the notice via fs and displays it
// through wm. parser and scheduleNextTick are forwarded to the
// markdown component so fenced code blocks get asynchronous syntax
// highlighting. onLinkClick is invoked when the user clicks a link
// in the rendered notice; returning true suppresses the markdown
// handler's default same-page anchor behavior.
func New(
	fs fileSystem, wm browserapi.WindowManager,
	parser syntaxapi.Parser, scheduleNextTick func(func()) bool,
	onLinkClick func(*url.URL) bool,
	cfg Config,
) *Crier {
	return &Crier{
		fs:               fs,
		wm:               wm,
		parser:           parser,
		scheduleNextTick: scheduleNextTick,
		onLinkClick:      onLinkClick,
		cfg:              cfg,
		store:            newStore(cfg.Storage),
	}
}

// Show resolves the configured notice and opens a floating window.
// It is a no-op when no notice is configured, and under ShowOnce
// it is also a no-op when the same fingerprint was already shown.
func (c *Crier) Show(ctx context.Context) error {
	content, err := c.resolveContent()
	if err != nil {
		return err
	}
	if content == "" {
		return nil
	}
	fp := fingerprint(content)
	uri := c.cfg.WorkspaceURI.String()
	if c.cfg.effectiveShow() == ShowOnce {
		shown, err := c.store.Shown(ctx, uri, fp)
		if err != nil {
			return err
		}
		if shown {
			return nil
		}
	}
	if err := c.openFloating(content); err != nil {
		return err
	}
	if c.cfg.effectiveShow() == ShowOnce {
		if err := c.store.MarkShown(ctx, uri, fp); err != nil {
			return err
		}
	}
	return nil
}

func (c *Crier) resolveContent() (string, error) {
	if c.cfg.Literal != "" {
		return c.cfg.Literal, nil
	}
	if c.cfg.Path == "" {
		return "", nil
	}
	f, err := c.fs.OpenFile(c.cfg.Path, os.O_RDONLY, 0)
	if err != nil {
		return "", fmt.Errorf("idenotice: open %q: %w", c.cfg.Path, err)
	}
	defer f.Close() //nolint:errcheck
	b, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("idenotice: read %q: %w", c.cfg.Path, err)
	}
	return string(b), nil
}

func fingerprint(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func (c *Crier) openFloating(content string) error {
	inner, err := buildNoticeHandler(content,
		c.parser, c.scheduleNextTick, c.onLinkClick)
	if err != nil {
		return err
	}
	var win browserapi.Window
	bh := browserapi.FuncHandler(inner, func() error {
		return c.wm.CloseWindow(win)
	})
	floating := browserapi.FuncFloating(bh, inner.Dimensions)
	win, err = c.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	if err != nil {
		return fmt.Errorf("idenotice: open floating: %w", err)
	}
	return nil
}

func buildNoticeHandler(
	content string,
	parser syntaxapi.Parser, scheduleNextTick func(func()) bool,
	onLinkClick func(*url.URL) bool,
) (*handler.Span, error) {
	mcfg := markdown.DefaultConfig()
	mcfg.HeaderPrefix = false
	mcfg.Parser = parser
	mcfg.ScheduleNextTick = scheduleNextTick
	md, err := markdown.NewWithConfig(content, mcfg)
	if err != nil {
		return nil, fmt.Errorf("idenotice: parse markdown: %w", err)
	}
	mdh := mdhandler.New(md, mdhandler.WithOnLinkClick(onLinkClick))
	return handler.NewSpan(mdh, component.SpanConfig{
		PadHorizontal:    2,
		ContentAlignment: component.AlignmentCentered,
	}), nil
}
