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

package searchbox

import "github.com/unstablebuild/rune-go-sdk/term"

// Floating is the floating search component owned by a Box.
type Floating = floating

// Button identifies one of the search box buttons.
type Button = button

const (
	// ButtonNone is the absence of a button.
	ButtonNone = buttonNone
	// ButtonUpgrade is the find-to-replace button.
	ButtonUpgrade = buttonUpgrade
	// ButtonNext is the replace-next button.
	ButtonNext = buttonNext
	// ButtonAll is the replace-all button.
	ButtonAll = buttonAll
	// PaddingX is the horizontal padding of the search box.
	PaddingX = paddingX
)

// Rect is the position and size of a search box element.
type Rect struct {
	X, Y, Width, Height int
}

func toRect(r rect) Rect { return Rect{X: r.x, Y: r.y, Width: r.width, Height: r.height} }

// NewFloating builds the floating component without a window manager.
func NewFloating(owner *Box, mode Mode, query string) *Floating {
	return newFloating(owner, mode, query)
}

// Floating returns the currently open floating component, if any.
func (b *Box) Floating() *Floating { return b.floating }

// QueryRect returns the query input geometry.
func (f *Floating) QueryRect() Rect { return toRect(f.layout.query) }

// ReplacementRect returns the replacement input geometry.
func (f *Floating) ReplacementRect() Rect { return toRect(f.layout.replacement) }

// UpgradeRect returns the find-to-replace button geometry.
func (f *Floating) UpgradeRect() Rect { return toRect(f.layout.upgrade) }

// NextRect returns the replace-next button geometry.
func (f *Floating) NextRect() Rect { return toRect(f.layout.next) }

// AllRect returns the replace-all button geometry.
func (f *Floating) AllRect() Rect { return toRect(f.layout.all) }

// ContentHeight returns the height the content wants.
func (f *Floating) ContentHeight() int { return f.layout.contentHeight }

// ViewportHeight returns the height the content was last resized to.
func (f *Floating) ViewportHeight() int { return f.layout.height }

// Hover returns the button currently under the pointer.
func (f *Floating) Hover() Button { return f.hover }

// SetHover marks button as hovered.
func (f *Floating) SetHover(b Button) { f.hover = b }

// ButtonAt returns the button at the given viewport coordinates.
func (f *Floating) ButtonAt(x, y int) Button { return f.buttonAt(x, y) }

// Mode returns whether the replacement input is shown.
func (f *Floating) Mode() Mode { return f.mode }

// ReplacementFocused reports whether the replacement input has focus.
func (f *Floating) ReplacementFocused() bool { return f.focus == focusReplacement }

// FocusReplacement moves focus to the replacement input.
func (f *Floating) FocusReplacement() { f.setFocus(focusReplacement) }

// Upgrade turns a find box into a find and replace box.
func (f *Floating) Upgrade() { f.upgrade() }

// QueryText returns the current query.
func (f *Floating) QueryText() string { return f.query.text() }

// HasReplacement reports whether the replacement input exists.
func (f *Floating) HasReplacement() bool { return f.replacement != nil }

// ReplacementText returns the current replacement.
func (f *Floating) ReplacementText() string {
	if f.replacement == nil {
		return ""
	}
	return f.replacement.text()
}

// HandleReplacement sends ev straight to the replacement input.
func (f *Floating) HandleReplacement(ev term.Event) {
	f.replacement.Handle(ev)
}
