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

package vte

import (
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
)

//go:generate go run ./runewidthgen

const (
	// widthTableLimit covers the Basic Multilingual Plane plus the
	// Supplementary Multilingual Plane, which together hold every script
	// and every emoji a terminal realistically renders.
	widthTableLimit = 0x20000
	widthBlockShift = 6
	widthBlockSize  = 1 << widthBlockShift
)

// runeWidth returns the number of cells c occupies. The generated table
// shortcuts graphemecluster.StringWidth, which costs two Unicode table
// binary searches on every call.
func runeWidth(c rune) int {
	if c < 0x7F {
		return 1
	}
	if c < widthTableLimit {
		block := int(runeWidthBlockIndex[c>>widthBlockShift])
		return int(runeWidthBlocks[block<<widthBlockShift|int(c)&(widthBlockSize-1)] - '0')
	}
	return graphemecluster.StringWidth(string(c))
}
