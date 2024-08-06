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

package term

import (
	"github.com/unstablebuild/tcell/v3"
)

// AttributesDifference computes the set difference between a and b,
// that is it returns a set of attributes that contain
// all the bit flags set in a but not set in b, and returns
// ColorDefault if a's color is equal to b's color or returns
// the color set in a.
func AttributesDifference(a, b Attributes) Attributes {
	retFgColor := a.Fg
	retBgColor := a.Bg
	if a.Fg == b.Fg {
		retFgColor = tcell.ColorDefault
	}
	if a.Bg == b.Bg {
		retBgColor = tcell.ColorDefault
	}
	return Attributes{Fg: retFgColor, Bg: retBgColor, Attrs: (a.Attrs &^ b.Attrs)}
}

// AttributesUnion computes the set union between a and b,
// that is it returns a set of attributes that contain
// all the bit flags set in a, b or both, and uses the color
// defined in b or if not set, uses the color in a.
func AttributesUnion(a, b Attributes) Attributes {
	retFgColor := b.Fg
	retBgColor := b.Bg
	if b.Fg == tcell.ColorDefault {
		retFgColor = a.Fg
	}
	if b.Bg == tcell.ColorDefault {
		retBgColor = a.Bg
	}
	return Attributes{Fg: retFgColor, Bg: retBgColor, Attrs: (a.Attrs | b.Attrs)}
}
