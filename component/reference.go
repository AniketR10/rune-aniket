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

var _ tui.Component = (*Reference)(nil)

// Reference can be used to dynamically swap a tui.Component.
// If the underlying tui.Component is nil, then Resize and Draw do nothing.
type Reference struct {
	component tui.Component
	dirty     bool
	height    int
	width     int
}

// NewReference allocates storage for a new Reference and initializes it with ref.
func NewReference(ref tui.Component) *Reference {
	ret := new(Reference)
	ret.Init(ref)
	return ret
}

// Init initializes this reference with ref.
// It can be used subsequently to override the underlying tui.Component reference.
func (r *Reference) Init(ref tui.Component) {
	r.component = ref
	r.dirty = true
}

func (r *Reference) Resize(width, height int) {
	r.height, r.width = height, width
	if r.component == nil {
		return
	}
	r.dirty = false
	r.component.Resize(width, height)
}

func (r *Reference) Draw(w term.Writer) {
	if r.component == nil {
		return
	}
	if r.dirty {
		r.Resize(r.width, r.height)
	}
	r.component.Draw(w)
}
