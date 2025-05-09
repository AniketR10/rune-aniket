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

package extension

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

func prepareTestResources() (*fileBarEditorHandler, workspaceapi.URI, [][]term.Cell) {
	h := new(fileBarEditorHandler)
	h.symbols = make(map[string][]symbInfo, 0)

	uri, _ := workspaceapi.ParseURI("file:///testdata/hello.ts")

	buf := cell.NewBuffer()
	buf.Init()
	f, _ := os.Open("testdata/hello.ts")
	buf.ReadFrom(f)
	cells := buf.RawCells()

	return h, uri, cells
}

func TestFuncNameSymbol(t *testing.T) {
	h, uri, cells := prepareTestResources()
	resourceName := uri.String()

	h.buildSymbolData(uri, cells)
	assert.Equal(t, len(h.symbols[resourceName]), 69)

	h.resolveCursorSymbol(uri, 978)
	assert.Equal(t, "getRelativePathToDirectoryOrUrl", h.currSymbolName)
	h.cleanSymbolData(uri)
	assert.Equal(t, "", h.currSymbolName)
	h.resolveCursorSymbol(uri, 793)
	assert.Equal(t, "", h.currSymbolName)
	h.buildSymbolData(uri, cells)
	h.resolveCursorSymbol(uri, 793)
	assert.Equal(t, "comparePathsWorker", h.currSymbolName)
}

func BenchmarkFuncNameSymbolScrolling(b *testing.B) {
	h, uri, cells := prepareTestResources()
	for i := range cells {
		if i%5 == 0 { // simulate a text change every 5 lines
			h.buildSymbolData(uri, cells)
		}
		h.resolveCursorSymbol(uri, i)
	}
}
