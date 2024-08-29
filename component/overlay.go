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

package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

// Overlay overlays one component over another one, with potentially padding
// and/or alignment, specified by SpanConfig.
type Overlay struct {
	background tui.Component
	Span
}

// NewOverlay allocates storage for a new overlay and initializes it.
func NewOverlay(
	background, cover tui.Component, backAttr term.Attributes, config SpanConfig,
) *Overlay {
	ret := new(Overlay)
	ret.Init(background, cover, backAttr, config)
	return ret
}

// Init initializes this overlay with the given background, cover and configuration.
func (o *Overlay) Init(
	background, cover tui.Component, backAttr term.Attributes, config SpanConfig,
) {
	// clean cells before drawing on top
	cover = WithBackground(cover, term.Cell{Attributes: backAttr})

	o.Span.Init(cover, config)
	o.background = background
}

// Resize satisfies tui.Component.
func (o *Overlay) Resize(width, height int) {
	o.background.Resize(width, height)
	o.Span.Resize(width, height)
}

// Draw satisfies tui.Component.
func (o *Overlay) Draw(w term.Writer) {
	o.background.Draw(w)
	o.Span.Draw(w)
}
