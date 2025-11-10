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
	"strings"

	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

// Divider returns a divider compatible with Responsive collections
// that will simply draw a line that will occupy perc of its width.
func Divider(perc float64, cfg StringConfig) Responsive {
	return &divider{perc: perc, cfg: cfg}
}

type divider struct {
	perc float64
	cfg  StringConfig
	comp Virtual[tui.Component]
}

func (d *divider) Resize(width, height int) {
	totalWidth := int(float64(width) * d.perc)

	var builder strings.Builder
	for i := 0; i < totalWidth; i++ {
		builder.WriteRune(d.cfg.FrameCharSet.HorizontalTop)
	}

	d.comp.C = NewStringWithConfig(builder.String(), d.cfg)

	offsetX := int((float64(width) - float64(totalWidth)) / 2)
	offsetY := int(float64(height) / 2)
	d.comp.Resize(totalWidth, 1)
	d.comp.Move(term.Coordinates{X: offsetX, Y: offsetY})
}

func (d *divider) Draw(w term.Writer) {
	d.comp.Draw(w)
}

func (d *divider) Height(width int) int {
	return 1
}
