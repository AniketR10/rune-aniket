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


package html

import (
	"net/http"

	"github.com/unstablebuild/rune-go-sdk/term"
	htmlcomp "unstable.build/go-tui/component/html"
	"unstable.build/go-tui/component/markdown"
)

// Option configures a [Handler].
type Option func(*Handler)

// WithHTTPClient sets the HTTP client used for fetching HTML content.
// Defaults to [http.DefaultClient].
func WithHTTPClient(client *http.Client) Option {
	return func(h *Handler) {
		h.httpClient = client
	}
}

// WithMarkdownConfig sets the markdown rendering configuration used
// when converting fetched HTML content. Defaults to
// [markdown.DefaultConfig].
func WithMarkdownConfig(cfg markdown.Config) Option {
	return func(h *Handler) {
		h.mdCfg = cfg
	}
}

// WithSelectionAttrs sets the attributes used to highlight selected
// text. Defaults to [term.AttrReverse].
func WithSelectionAttrs(attrs term.Attributes) Option {
	return func(h *Handler) {
		h.selectionAttrs = attrs
	}
}

// WithComponentOptions appends additional options that are forwarded
// to every [htmlcomp.Component] created by the handler.
func WithComponentOptions(opts ...htmlcomp.Option) Option {
	return func(h *Handler) {
		h.compOpts = append(h.compOpts, opts...)
	}
}

// BarPosition specifies where the navigation bar appears.
type BarPosition int

const (
	// BarTop places the navigation bar at the top of the handler.
	BarTop BarPosition = iota
	// BarBottom places the navigation bar at the bottom of the handler.
	BarBottom
)

// WithNavigationBar adds a navigation bar with back/forward buttons
// and a URL input box at the specified position.
func WithNavigationBar(pos BarPosition) Option {
	return func(h *Handler) {
		h.barPos = pos
		h.bar = newNavigationBar("")
	}
}
