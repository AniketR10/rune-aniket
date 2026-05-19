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


package shop

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	fileexplorercomp "unstable.build/go-tui/component/fileexplorer"
	"unstable.build/go-tui/text"
)

func (r *Root) toggleFileExplorer() error {
	if r.fileExplorerWin != nil && !r.fileExplorerWin.Closed() {
		return r.fileExplorerWin.Close()
	}
	if r.fileExplorer == nil {
		if err := r.initFileExplorer(); err != nil {
			return err
		}
	}
	target := r.b.Focus()
	win, ok := r.b.SplitRoot(component.AlignmentLeft, r.fileExplorer)
	if !ok {
		return fmt.Errorf("could not create file explorer split")
	}
	r.fileExplorerWin = win
	if h, ok := r.fileExplorer.(*pageExplorerHandler); ok {
		h.target = target
		h.win = win
		h.syncWidth()
	}
	_ = r.b.SetFocus(win)
	return nil
}

func (r *Root) initFileExplorer() error {
	rootURI := mustWorkspaceURI("pages:///")
	buf := cell.NewBuffer()
	comp, err := fileexplorercomp.New(buf, pageFileSystem{pages: r.pages}, rootURI,
		fileexplorercomp.Config{
			IndentWidth: 2,
			IndentRune:  '|',
			Icons: text.IconSet{
				Directory: 'd',
				Default:   'f',
			},
		})
	if err != nil {
		return err
	}
	r.fileExplorer = &pageExplorerHandler{root: r, comp: comp, selected: 0}
	return nil
}

type pageExplorerHandler struct {
	root     *Root
	comp     *fileexplorercomp.Component
	selected int
	width    int
	height   int
	target   browser.Window
	win      browser.Window
}

var _ browserapi.Handler = (*pageExplorerHandler)(nil)

func (h *pageExplorerHandler) Resize(width, height int) {
	h.width, h.height = width, height
	h.comp.Resize(width, height)
	maxIdx := max(0, len(h.root.pages)-1)
	if h.selected > maxIdx {
		h.selected = maxIdx
	}
	h.syncWidth()
}

func (h *pageExplorerHandler) Draw(w term.Writer) {
	h.comp.Draw(w)
	if h.height == 0 || h.width == 0 {
		return
	}
	y := min(h.selected, h.height-1)
	for x := 0; x < h.width; x++ {
		w.UnionAttributes(term.Coordinates{X: x, Y: y},
			term.Attributes{Attrs: term.AttrReverse})
	}
}

func (h *pageExplorerHandler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return false, false
	}
	switch {
	case ev.Key == term.KeyArrowUp || (ev.Ch == 'k' && ev.Mod == 0):
		if h.selected > 0 {
			h.selected--
		}
		h.syncWidth()
		return false, true
	case ev.Key == term.KeyArrowDown || (ev.Ch == 'j' && ev.Mod == 0):
		if h.selected < len(h.root.pages)-1 {
			h.selected++
		}
		h.syncWidth()
		return false, true
	case ev.Key == term.KeyEnter:
		uri, ok := h.comp.NodeAt(term.Coordinates{Y: h.selected})
		if !ok {
			return false, true
		}
		name := strings.TrimSuffix(filepath.Base(uri.Path()), ".md")
		if name == "" {
			return false, true
		}
		if h.target != nil && !h.target.Closed() {
			_ = h.root.b.SetFocus(h.target)
		}
		if err := h.root.showPageByName(name); err != nil {
			h.root.log.Warn("file explorer open page", "page", name, "err", err)
		}
		return false, true
	default:
		return false, false
	}
}

func (h *pageExplorerHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{X: 0, Y: min(h.selected, max(0, h.height-1))},
		term.CursorStyleDefault, true
}

func (h *pageExplorerHandler) Selection() (string, bool) { return "", false }

func (h *pageExplorerHandler) Close() error { return nil }

func (h *pageExplorerHandler) syncWidth() {
	if h.win == nil || h.win.Closed() {
		return
	}
	width, _ := h.comp.Dimensions()
	// The shop browser always uses framed windows, so account for the frame.
	width += 2
	_ = h.root.b.SetWindowWidth(h.win, width)
}

type pageFileSystem struct {
	pages []page
}

func (fsys pageFileSystem) URI(path string) (workspaceapi.URI, error) {
	path = strings.TrimPrefix(path, "/")
	if path == "" || path == "." {
		return workspaceapi.ParseURI("pages:///")
	}
	return workspaceapi.ParseURI("pages:///" + path)
}

func (fsys pageFileSystem) OpenFile(string, int, os.FileMode) (workspaceapi.File, error) {
	return nil, fs.ErrPermission
}

func (fsys pageFileSystem) Remove(string) error { return fs.ErrPermission }

func (fsys pageFileSystem) Stat(path string) (os.FileInfo, error) {
	path = strings.Trim(path, "/")
	if path == "" || path == "." {
		return staticFileInfo{name: "/", dir: true}, nil
	}
	for _, p := range fsys.pages {
		if path == p.name+".md" {
			return staticFileInfo{name: p.name + ".md"}, nil
		}
	}
	return nil, fs.ErrNotExist
}

func (fsys pageFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	name = strings.Trim(name, "/")
	if name != "" && name != "." {
		return nil, fs.ErrNotExist
	}
	ret := make([]os.DirEntry, 0, len(fsys.pages))
	for _, p := range fsys.pages {
		ret = append(ret, staticDirEntry{name: p.name + ".md"})
	}
	return ret, nil
}

func (fsys pageFileSystem) MkdirAll(string, os.FileMode) error { return fs.ErrPermission }

type staticDirEntry struct{ name string }

func (e staticDirEntry) Name() string               { return e.name }
func (e staticDirEntry) IsDir() bool                { return false }
func (e staticDirEntry) Type() fs.FileMode          { return 0 }
func (e staticDirEntry) Info() (fs.FileInfo, error) { return staticFileInfo{name: e.name}, nil }

type staticFileInfo struct {
	name string
	dir  bool
}

func (i staticFileInfo) Name() string { return i.name }
func (i staticFileInfo) Size() int64  { return 0 }
func (i staticFileInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}
func (i staticFileInfo) ModTime() time.Time { return time.Time{} }
func (i staticFileInfo) IsDir() bool        { return i.dir }
func (i staticFileInfo) Sys() any           { return nil }
