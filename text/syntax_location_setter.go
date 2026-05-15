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

package text

import (
	"sync/atomic"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/go-tui/ide/syntax"
)

// newSyntaxLocationSetter returns a syntax.LocationSetter whose
// dispatch is gated by a Handler reference that the caller publishes
// later via the returned publish function. This is used by
// Component.newFileBuffer because syntax.WithTree schedules an
// asynchronous parse that may invoke SetLocationList after Edit has
// failed and newFileBuffer has returned; capturing the named-return
// handler directly would dispatch against a nil interface value.
//
// Until publishHandler is called, dispatch is a no-op. After publish,
// dispatch forwards to handler.SetLocationList. Concurrency-safe via
// atomic.Value so the gate read in the dispatch path never races with
// the event-loop write.
func newSyntaxLocationSetter() (
	setter syntax.LocationSetter,
	publishHandler func(Handler),
) {
	var ref atomic.Value
	setter = syntax.FuncLocationSetter(func(ll textapi.LocationList) {
		v := ref.Load()
		if v == nil {
			return
		}
		h, ok := v.(handlerHolder)
		if !ok || h.h == nil {
			return
		}
		h.h.SetLocationList(textapi.LocationPriorityInfo, "syntax", ll)
	})
	publishHandler = func(h Handler) {
		if h == nil {
			return
		}
		ref.Store(handlerHolder{h: h})
	}
	return
}

// handlerHolder wraps a Handler so the atomic.Value stays type-stable
// across stores. atomic.Value rejects subsequent stores of a different
// concrete type, so we always store this wrapper.
type handlerHolder struct{ h Handler }
