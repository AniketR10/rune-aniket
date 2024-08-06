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

package scanner

// Driver is an interface that abstracts behavioural actions requested by a Scanner.
type Driver interface {
	// Print draws a character to the screen and updates states.
	Print(rune)

	// Execute executes a C0 or C1 control function.
	Execute(byte)

	// Hook is invoked when a final character arrives in the first part of a device control string.
	// The control function should be determined from the private marker, final character, and
	// execute with a parameter list. A handler should be selected for remaining characters in the
	// string; the handler function should subsequently be called by Put for every character in
	// the control string.
	// The ignore flag indicates that more than two intermediates arrived and
	// subsequent characters were ignored.
	Hook(params [][]uint16, intermediates []byte, ignore bool, action rune)

	// Put passes bytes as part of a device control string to the handler chosen in Hook.
	// C0 controls will also be passed to the handler.
	Put(byte)

	// Unhook is called when a device control string is terminated.
	// The previously selected handler should be notified that the DCS has terminated.
	Unhook()

	// OSCDispatch dispatches an operating system command.
	OSCDispatch(params [][]byte, bellTerminated bool)

	// CSIDispatch is called when a final character has arrived for a CSI sequence.
	// The ignore flag indicates that either more than two intermediates arrived
	// or the number of parameters exceeded the maximum supported length,
	// and subsequent characters were ignored.
	CSIDispatch(params [][]uint16, intermediates []byte, ignore bool, action rune)

	// ESCDispatch is called when the final character of an escape sequence has arrived.
	// The ignore flag indicates that more than two intermediates arrived and
	// subsequent characters were ignored.
	ESCDispatch(intermediates []byte, ignore bool, ch byte)
}
