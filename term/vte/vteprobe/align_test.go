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

package vteprobe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlignByGutter(t *testing.T) {
	t.Parallel()

	lines := []string{
		"package main",
		"",
		"import \"fmt\"",
		"",
		"func main() {",
		"\tfmt.Println(\"hello\")",
		"}",
	}
	rows := []extractedRow{
		makeRow(" 1 package main"),
		makeRow(" 2 "),
		makeRow(" 3 import \"fmt\""),
		makeRow(" 4 "),
		makeRow(" 5 func main() {"),
		makeRow(" 6     fmt.Println(\"hello\")"),
		makeRow(" 7 }"),
	}
	g := detectGutter(rows, 0, len(rows)-1)
	assert.True(t, g.present)

	a := alignByGutter(rows, 0, len(rows)-1, g, lines, []int{4, 2, 8})
	assert.True(t, a.ok, "expected aligned, coverage=%v", a.coverage)
	assert.Equal(t, 1, a.topFileLine)
	assert.Equal(t, 4, a.tabstop, "should infer tabstop 4 because line 6 renders 4 spaces before fmt")
}

func TestAlignByContent(t *testing.T) {
	t.Parallel()

	lines := []string{
		"package main",
		"",
		"import \"fmt\"",
		"",
		"func main() {",
		"\tfmt.Println(\"hello\")",
		"}",
	}
	rows := []extractedRow{
		makeRow("import \"fmt\""),
		makeRow(""),
		makeRow("func main() {"),
		makeRow("    fmt.Println(\"hello\")"),
		makeRow("}"),
	}

	a := alignByContentWithGutter(rows, 0, len(rows)-1, 0, lines, []int{4, 2, 8})
	assert.True(t, a.ok, "expected aligned, coverage=%v", a.coverage)
	assert.Equal(t, 3, a.topFileLine)
	assert.Equal(t, 4, a.tabstop)
}
