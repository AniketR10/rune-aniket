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

package ide

import (
	"net/url"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/rune/internal/ide/idenotice"
)

func newNoticeConfig(
	cfg ideConfig, storage storageapi.Service, uri workspaceapi.URI,
) (idenotice.Config, bool) {
	path, literal, show := cfg.workspaceNotice()
	if path == "" && literal == "" {
		return idenotice.Config{}, false
	}
	return idenotice.Config{
		Path:         path,
		Literal:      literal,
		Show:         show,
		Storage:      storageapi.WithPartition(storage, "notice"),
		WorkspaceURI: uri,
	}, true
}

// noticeLinkCopier mirrors the markdown link handling used by
// text.Component and the Hover handler: http(s) links are copied to
// the system clipboard and surface as a notification, anything else
// falls back to the markdown handler's default (in-page anchor
// scrolling).
func noticeLinkCopier(
	clip clipboard.Register, n browserapi.Notifications,
) func(*url.URL) bool {
	return func(link *url.URL) bool {
		if link.Scheme != "http" && link.Scheme != "https" {
			return false
		}
		linkstr := link.String()
		if err := clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: linkstr}); err != nil {
			_, _ = n.Notify(browserapi.LevelError,
				"copy URL to clipboard: %v", err)
			return true
		}
		_, _ = n.Notify(browserapi.LevelSuccess,
			"copied URL %s to clipboard", linkstr)
		return true
	}
}
