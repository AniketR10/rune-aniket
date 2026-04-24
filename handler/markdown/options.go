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


package markdown

import (
	"net/url"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// Option configures a Handler.
type Option func(*Handler)

// WithOnLinkClick sets a callback for when a link is clicked.
// The callback returns true if it handled the link, false otherwise.
// If the callback returns false (or is not set), clicking local anchors
// (URLs starting with #) will scroll to the corresponding header.
func WithOnLinkClick(fn func(*url.URL) bool) Option {
	return func(h *Handler) {
		h.onLinkClick = fn
	}
}

// WithSelectionAttrs sets the attributes used to highlight selected text.
// The attributes are unioned with existing cell attributes.
// Defaults to term.Attributes with AttrReverse.
func WithSelectionAttrs(attrs term.Attributes) Option {
	return func(h *Handler) {
		h.selectionAttrs = attrs
	}
}
