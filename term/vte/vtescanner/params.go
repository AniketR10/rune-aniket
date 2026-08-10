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

type params struct {
	subparams [MaxParams]uint16
	params    [MaxParams]uint16
	// slices backs slice() so a CSI dispatch costs no allocation in
	// steady state. Its contents are only valid until the next dispatch.
	slices           [MaxParams][]uint16
	currentSubparams uint16
	len              uint16
}

func (p *params) isFull() bool {
	return p.len == MaxParams
}

func (p *params) push(item uint16) {
	p.subparams[p.len-p.currentSubparams] = p.currentSubparams + 1
	p.params[p.len] = item
	p.currentSubparams = 0
	p.len += 1
}

func (p *params) extend(item uint16) {
	p.subparams[p.len-p.currentSubparams] = p.currentSubparams + 1
	p.params[p.len] = item
	p.currentSubparams += 1
	p.len += 1
}

// slice groups the parsed parameters by subparameter run. The returned
// slice, and the slices it holds, alias storage the scanner reuses: a
// driver that needs them past the dispatch call must copy.
func (p *params) slice() [][]uint16 {
	n := 0
	index := uint16(0)
	for index < p.len {
		numSubparams := p.subparams[index]
		p.slices[n] = p.params[index : index+numSubparams]
		n++
		index += numSubparams
	}
	return p.slices[:n]
}

func (p *params) reset() {
	p.currentSubparams = 0
	p.len = 0
}
