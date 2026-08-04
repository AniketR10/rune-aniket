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

package ide

// funcCommandObserver adapts a plain callback to commandObserver for
// subscribers that only care that a command ran, not which one.
type funcCommandObserver func()

func (f funcCommandObserver) observeCommand(_, _ string, _ []string, _ error) {
	f()
}

type commandObserverRegistry struct {
	subscribers []commandObserver
}

func newCommandObserverRegistry() *commandObserverRegistry {
	return &commandObserverRegistry{}
}

func (r *commandObserverRegistry) subscribe(obs commandObserver) {
	if obs == nil {
		return
	}
	r.subscribers = append(r.subscribers, obs)
}

func (r *commandObserverRegistry) observeCommand(
	typed, resolved string, args []string, err error,
) {
	for _, s := range r.subscribers {
		s.observeCommand(typed, resolved, args, err)
	}
}
