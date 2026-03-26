// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

package html

import (
	"net/http"

	"unstable.build/go-tui/component/markdown"
)

type config struct {
	mdCfg      markdown.Config
	httpClient *http.Client
}

// Option configures a [Component].
type Option func(*config)

// WithMarkdownConfig sets the markdown rendering configuration.
func WithMarkdownConfig(cfg markdown.Config) Option {
	return func(c *config) {
		c.mdCfg = cfg
	}
}

// WithHTTPClient sets the HTTP client used for fetching content.
func WithHTTPClient(client *http.Client) Option {
	return func(c *config) {
		c.httpClient = client
	}
}
