// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestAppLaunchArgs(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		execPath string
		wantArgs []string
		wantOK   bool
	}{
		{
			name:     "darwin app",
			goos:     "darwin",
			execPath: filepath.Join("Applications", "Rune.app", "Contents", "MacOS", "rune"),
			wantArgs: []string{
				"--rune-extension-runner=" + filepath.Join("Applications", "Rune.app", "Contents", "Resources", "rune-extension"),
				"--rune-zdotdir=" + filepath.Join("Applications", "Rune.app", "Contents", "Resources", "zdot"),
				"-G", "-w", "",
			},
			wantOK: true,
		},
		{
			name:     "darwin extension",
			goos:     "darwin",
			execPath: filepath.Join("Applications", "Rune.app", "Contents", "Resources", "rune-extension"),
			wantOK:   false,
		},
		{
			name:     "linux freedesktop app",
			goos:     "linux",
			execPath: filepath.Join("opt", "Rune", "rune.app", "bin", "rune"),
			wantArgs: []string{
				"--rune-extension-runner=" + filepath.Join("opt", "Rune", "rune.app", "bin", "rune"),
				"--rune-zdotdir=" + filepath.Join("opt", "Rune", "rune.app", "share", "zdot"),
				"-G", "-w", "",
			},
			wantOK: true,
		},
		{
			name:     "linux non app",
			goos:     "linux",
			execPath: filepath.Join("usr", "local", "bin", "rune"),
			wantArgs: []string{
				"--rune-extension-runner=" + filepath.Join("usr", "local", "bin", "rune"),
				"-G", "-w", "",
			},
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotArgs, gotOK := appLaunchArgs(tt.goos, tt.execPath)
			if gotOK != tt.wantOK {
				t.Fatalf("appLaunchArgs() ok = %v, want %v", gotOK, tt.wantOK)
			}
			if !reflect.DeepEqual(gotArgs, tt.wantArgs) {
				t.Fatalf("appLaunchArgs() = %#v, want %#v", gotArgs, tt.wantArgs)
			}
		})
	}
}
