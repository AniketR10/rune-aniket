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

package idecursor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestShouldRecord(t *testing.T) {
	base := location{URI: "file:///a.go", Cursor: term.Coordinates{X: 1, Y: 10}}
	assert.False(t, shouldRecord(base, base))
	assert.False(t, shouldRecord(base, location{URI: "file:///a.go", Cursor: term.Coordinates{X: 20, Y: 15}}))
	assert.True(t, shouldRecord(base, location{URI: "file:///a.go", Cursor: term.Coordinates{X: 1, Y: 25}}))
	assert.True(t, shouldRecord(base, location{URI: "file:///b.go", Cursor: term.Coordinates{X: 1, Y: 11}}))
}

func TestHistoryDocumentRecordPrevNextAndBranch(t *testing.T) {
	ws, err := workspaceapi.ParseURI("file:///workspace")
	require.NoError(t, err)
	doc := newHistoryDocument(ws)
	a := location{URI: "file:///a.go", Cursor: term.Coordinates{Y: 1}}
	b := location{URI: "file:///b.go", Cursor: term.Coordinates{Y: 2}}
	c := location{URI: "file:///c.go", Cursor: term.Coordinates{Y: 3}}
	d := location{URI: "file:///d.go", Cursor: term.Coordinates{Y: 4}}

	assert.True(t, doc.record(a))
	assert.True(t, doc.record(b))
	assert.True(t, doc.record(c))
	require.Len(t, doc.Entries, 3)
	assert.Equal(t, 2, doc.Index)

	prev, err := doc.prev()
	require.NoError(t, err)
	assert.Equal(t, b, prev)
	assert.Equal(t, 1, doc.Index)

	next, err := doc.next()
	require.NoError(t, err)
	assert.Equal(t, c, next)

	_, err = doc.prev()
	require.NoError(t, err)
	assert.True(t, doc.record(d))
	assert.Equal(t, []location{a, b, d}, doc.Entries)
	assert.Equal(t, 2, doc.Index)
}
