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

package symbolresolve_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/go-tui/ide/idelsp/symbolresolve"
)

// specIter wraps a single spec in an iterator for engine-level tests that
// exercise one language directly.
func specIter(spec *symbolresolve.Spec) iterator.Iterator[symbolresolve.Spec] {
	return iterator.FromSlice([]symbolresolve.Spec{*spec})
}

func TestSpecFor(t *testing.T) {
	t.Parallel()

	assert.Same(t, symbolresolve.Go, symbolresolve.SpecFor("go"))
	assert.Same(t, symbolresolve.Python, symbolresolve.SpecFor("python"))
	assert.Nil(t, symbolresolve.SpecFor("rust"))
}

func TestAll(t *testing.T) {
	t.Parallel()

	all := symbolresolve.All()
	assert.Contains(t, all, symbolresolve.Go)
	assert.Contains(t, all, symbolresolve.Python)
}
