// Copyright (C) 2017-2026 Unstable Build, LLC
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

package shop

import (
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

	"unstable.build/rune/browser"
	mdcomp "unstable.build/rune/component/markdown"
	mdhandler "unstable.build/rune/handler/markdown"
)

const shellTabName = "shell"

func (r *Root) mustInitLayout() {
	r.mustInitPageTabs()
	r.mustInitShellTab()

	left := r.b.Focus()
	home, ok := r.pageTabs["home"]
	if !ok {
		panic("shop: missing home tab")
	}
	if err := left.SetContent(home); err != nil {
		panic(fmt.Errorf("set home content: %w", err))
	}

	right, ok := r.b.Split(browserapi.OrientationRight, left, nil)
	if !ok {
		panic("shop: split right failed")
	}
	if _, ok := r.b.Split(browserapi.OrientationBottom, right, r.shellTab); !ok {
		panic("shop: split bottom failed")
	}
	_ = r.b.SetFocus(left)
}

func (r *Root) mustInitPageTabs() {
	for _, p := range r.pages {
		if _, ok := r.pageTab(p.name); ok {
			continue
		}
		r.pageTabs[p.name] = r.mustNewPageTab(p)
	}
}

func (r *Root) mustNewPageTab(p page) *browser.Tab {
	mdCfg := mdcomp.DefaultConfig()
	mdCfg.HeaderPrefix = false
	if r.scheduleNextTick != nil {
		mdCfg.ScheduleNextTick = r.scheduleNextTick
	}
	md, err := mdcomp.NewWithConfig(p.body, mdCfg)
	if err != nil {
		panic(fmt.Errorf("build markdown page %q: %w", p.name, err))
	}
	mdH := mdhandler.New(md)
	uri := r.pageURI(p.name)
	return r.b.NewTab(uri, pageIcon(p.name), p.name, browser.NopHandler(mdH), nil)
}

func pageIcon(name string) rune {
	switch name {
	case "home":
		return '⌂'
	case "about":
		return 'i'
	case "product":
		return '□'
	case "pricing":
		return '$'
	case "account":
		return '@'
	default:
		return '•'
	}
}

func (r *Root) pageTab(name string) (*browser.Tab, bool) {
	p, ok := r.pageByName(name)
	if !ok {
		return nil, false
	}
	uri := r.pageURI(name)
	if current, ok := r.b.Tab(uri); ok {
		r.pageTabs[name] = current
		return current, true
	}
	// The tab was closed; recreate it on demand.
	t := r.mustNewPageTab(p)
	r.pageTabs[name] = t
	return t, true
}

func (r *Root) showPageByName(name string) error {
	tab, ok := r.pageTab(name)
	if !ok {
		return fmt.Errorf("unknown page %q", name)
	}
	win := r.invokeWindow()
	if err := win.SetContent(tab); err != nil {
		return err
	}
	r.pageWin = win
	return nil
}

func (r *Root) allPageNames() []string {
	ret := make([]string, 0, len(r.pages))
	for _, p := range r.pages {
		ret = append(ret, p.name)
	}
	return ret
}

func (r *Root) pageByName(name string) (page, bool) {
	for _, p := range r.pages {
		if p.name == name {
			return p, true
		}
	}
	return page{}, false
}

func (r *Root) pageURI(name string) workspaceapi.URI {
	return mustParseURI("shop:///page/" + name)
}

func tabIndexOptions(b *browser.Component) []string {
	tabs := b.Tabs()
	ret := make([]string, 0, len(tabs))
	for i, t := range tabs {
		pretty := fmt.Sprintf("%d", i+1)
		if name, _, ok := b.TabName(t.URI()); ok && name != "" {
			pretty += " " + name
		}
		ret = append(ret, pretty)
	}
	if len(ret) == 0 {
		return []string{""}
	}
	return ret
}

func mustWorkspaceURI(raw string) workspaceapi.URI {
	uri, err := workspaceapi.ParseURI(raw)
	if err != nil {
		panic(fmt.Errorf("parse workspace uri %q: %w", raw, err))
	}
	return uri
}
