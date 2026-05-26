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

package byoe

import "github.com/unstablebuild/rune-go-sdk/term"

// ignoreAttrWriter wraps a term.Writer so that the embedded TUI
// editor's draw pass writes cell content but never paints colors or
// styles. Rune then layers its own location-list attributes on top
// via UnionAttributes on the underlying writer, ensuring Rune-managed
// highlights win over whatever the embedded editor would have drawn.
//
// The zero-value Attributes is never propagated up to the underlying
// writer because callers reach it through the wrapper only — they hold
// the unwrapped writer for the overlay pass themselves.
type ignoreAttrWriter struct {
	term.Writer
}

func (w ignoreAttrWriter) SetCell(pos term.Coordinates, c term.Cell) {
	c.Attributes = term.Attributes{}
	w.Writer.SetCell(pos, c)
}

func (w ignoreAttrWriter) UnionAttributes(term.Coordinates, term.Attributes) {}
