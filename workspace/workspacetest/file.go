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

//revive:disable:exported
package workspacetest

import (
	"io"
	"os"
	"time"
)

// File satisfies workspaceapi.File
type File struct {
	Reads  [][]byte
	Writes [][]byte
}

func (t *File) Name() string {
	return ""
}

func (t *File) Fd() uintptr {
	return 0
}

func (t *File) Stat() (os.FileInfo, error) {
	return FileInfo{}, nil
}

func (t *File) Sync() error {
	return nil
}
func (t *File) Truncate(size int64) error {
	return nil
}

func (t *File) Seek(x int64, y int) (int64, error) {
	return 0, nil
}

func (t *File) Read(b []byte) (int, error) {
	if len(t.Reads) == 0 {
		return 0, io.EOF
	}
	copy(b, t.Reads[len(t.Reads)-1])
	t.Reads = t.Reads[:len(t.Reads)-1]
	return len(b), nil
}

func (t *File) ReadAt(b []byte, offset int64) (int, error) {
	return 0, io.EOF
}

func (t *File) Write(b []byte) (int, error) {
	n := make([]byte, len(b))
	copy(n, b)
	t.Writes = append(t.Writes, n)
	return 0, nil
}

func (t *File) Close() error {
	return nil
}

// FileInfo satisfies os.FileInfo.
type FileInfo struct {
	Filename    string
	FileIsDir   bool
	FileModTime time.Time
	FileSize    int64
	FileMode    os.FileMode
}

func (t FileInfo) Name() string {
	return t.Filename
}
func (t FileInfo) Size() int64 {
	return t.FileSize
}

func (t FileInfo) Mode() os.FileMode {
	return t.FileMode
}

func (t FileInfo) ModTime() time.Time {
	return t.FileModTime
}

func (t FileInfo) IsDir() bool {
	return t.FileIsDir
}

func (t FileInfo) Sys() interface{} {
	return nil
}
