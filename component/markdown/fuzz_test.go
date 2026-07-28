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

package markdown

import (
	"testing"

	"github.com/unstablebuild/rune-go-sdk/term"
)

func FuzzParse(f *testing.F) {
	f.Add("# Hello")
	f.Add("Hello **world**")
	f.Add("- item\n- item2")
	f.Add("```go\ncode\n```")
	f.Add("> quote")
	f.Add("---")
	f.Add("| a | b |\n|---|---|\n| 1 | 2 |")
	f.Add("")
	f.Add("# H1\n\n## H2\n\n### H3")
	f.Add("*italic* **bold** `code` ~~strike~~")
	f.Add("[link](http://example.com)")

	cfg := DefaultConfig()
	f.Fuzz(func(t *testing.T, input string) {
		_, err := parse(input, &cfg)
		if err != nil {
			t.Skip()
		}
	})
}

func FuzzRender(f *testing.F) {
	f.Add("# Hello", 80, 24)
	f.Add("Hello **world**", 40, 10)
	f.Add("- item\n- item2", 20, 5)
	f.Add("", 10, 10)
	f.Add("# Title\n\nParagraph", 1, 1)
	f.Add("Long text that needs to wrap around", 5, 3)

	f.Fuzz(func(t *testing.T, input string, width, height int) {
		if width < 0 || height < 0 || width > 1000 || height > 1000 {
			t.Skip()
		}

		md, err := New(input)
		if err != nil {
			t.Skip()
		}
		md.Resize(width, height)

		if width > 0 && height > 0 {
			w := term.NewStringWriter(width, height)
			md.Draw(w)
		}
	})
}

func FuzzScroll(f *testing.F) {
	f.Add("# H1\n\n# H2\n\n# H3", 10)
	f.Add("Line\nLine\nLine\nLine\nLine\nLine\nLine\nLine\nLine\nLine\n", 5)
	f.Add("Short", 100)

	f.Fuzz(func(t *testing.T, input string, scrollOps int) {
		if scrollOps < 0 || scrollOps > 1000 {
			t.Skip()
		}

		md, err := New(input)
		if err != nil {
			t.Skip()
		}
		md.Resize(40, 5)

		for range scrollOps {
			if scrollOps%2 == 0 {
				md.SeekDown()
			} else {
				md.SeekUp()
			}
		}

		if md.SeekOffset() < 0 {
			t.Errorf("offset became negative: %d", md.SeekOffset())
		}
		if md.SeekOffset() > md.MaxSeekOffset() {
			t.Errorf("offset exceeded max: %d > %d", md.SeekOffset(), md.MaxSeekOffset())
		}
	})
}
