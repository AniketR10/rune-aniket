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

package schemedoc

import "sync"

// closeGroup is a sync.WaitGroup in which
// wg.Add and wg.Wait can be called concurrently from
// different goroutines.
type closeGroup struct {
	mu         sync.Mutex
	wg         sync.WaitGroup
	waitClosed bool
}

func (wg *closeGroup) AddOne() (func(), bool) {
	wg.mu.Lock()
	if wg.waitClosed {
		wg.mu.Unlock()
		return nil, false
	}
	wg.wg.Add(1)
	wg.mu.Unlock()
	return wg.wg.Done, true
}

func (wg *closeGroup) Close() bool {
	wg.mu.Lock()
	if wg.waitClosed {
		wg.mu.Unlock()
		return false
	}
	wg.waitClosed = true
	wg.mu.Unlock()
	wg.wg.Wait()
	return true
}
