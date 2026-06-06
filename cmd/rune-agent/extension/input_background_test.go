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

package extension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestResolveInputBackgroundColor(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]any
		def  term.Color
		want term.Color
	}{
		{
			name: "no input_box_attr keeps the default",
			cfg:  map[string]any{},
			def:  term.ColorDefault,
			want: term.ColorDefault,
		},
		{
			name: "input_box_attr without bg keeps the default",
			cfg:  map[string]any{"input_box_attr": map[string]any{"fg": "default"}},
			def:  term.ColorDefault,
			want: term.ColorDefault,
		},
		{
			name: "input_box_attr bg default keeps the default",
			cfg: map[string]any{
				"input_box_attr": map[string]any{"fg": "default", "bg": "default"},
			},
			def:  term.ColorDefault,
			want: term.ColorDefault,
		},
		{
			name: "explicit bg overrides the default",
			cfg: map[string]any{
				"input_box_attr": map[string]any{"bg": "red"},
			},
			def:  term.ColorDefault,
			want: term.ColorRed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveInputBackgroundColor(tc.def, config.MapConfig(tc.cfg))
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDefaultComponentInputBackgroundIsDefault(t *testing.T) {
	assert.Equal(t, term.ColorDefault, defaultComponentCfg.InputBackgroundColor,
		"compose input must default to the terminal default background")
}
