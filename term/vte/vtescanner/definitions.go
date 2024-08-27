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

package vtescanner

const (
	MaxIntermediates = 2
	MaxOSCParams     = 16
	MaxOSCRaw        = 1024
	// MaxParams represents the max number of parameters
	// passed to the Perform ifc.
	MaxParams = 32
)

// State represents the scanner state.
type State uint8

const (
	Anywhere State = iota
	CsiEntry
	CsiIgnore
	CsiIntermediate
	CsiParam
	DcsEntry
	DcsIgnore
	DcsIntermediate
	DcsParam
	DcsPassthrough
	Escape
	EscapeIntermediate
	Ground
	OSCString
	SosPmApcString
	Utf8
)

// Action represents the scanner action.
type Action int

const (
	None Action = iota
	Clear
	Collect
	CSIDispatch
	ESCDispatch
	Execute
	Hook
	Ignore
	OSCEnd
	OSCPut
	OSCStart
	Param
	Print
	Put
	Unhook
	BeginUtf8
)

// unpack unpacks a uint8 into a State and Action.
func unpack(delta uint8) (State, Action) {
	return State(delta & 0x0f), Action(delta >> 4)
}

// nolint:unused
// pack packs a State and Action into a uint8.
func pack(state State, action Action) uint8 {
	return uint8(action)<<4 | uint8(state)
}
