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

package workspacetest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/api/schemeapi"
	"unstable.build/go-tui/api/workspaceapi"
)

// NewNopScheme returns a scheme that does nothing and workspace.Executor API panics.
func NewNopScheme(scheme string) schemeapi.SchemeFunc {
	return func(ctx context.Context, cfg config.Config, uri workspaceapi.URI) (schemeapi.Scheme, error) {
		scheme := &testScheme{scheme: scheme}
		scheme.openFunc = func(name string, flag int, perm os.FileMode) (
			workspaceapi.File, *workspaceapi.Error,
		) {
			return &File{}, nil
		}
		scheme.removeFunc = func(name string) error {
			return nil
		}
		scheme.renameFunc = func(oldName, newName string) error {
			return nil
		}
		scheme.statFunc = func(name string) (os.FileInfo, error) {
			// best effort
			return FileInfo{FileIsDir: !strings.Contains(name, ".")}, nil
		}
		scheme.lstatFunc = func(name string) (os.FileInfo, error) {
			return FileInfo{}, nil
		}
		return scheme, nil
	}
}

type testScheme struct {
	scheme     string
	openFunc   func(name string, flag int, perm os.FileMode) (workspaceapi.File, *workspaceapi.Error)
	removeFunc func(name string) error
	renameFunc func(oldName, newName string) error
	statFunc   func(name string) (os.FileInfo, error)
	lstatFunc  func(name string) (os.FileInfo, error)
}

func (t *testScheme) Command(ctx context.Context, name string, arg ...string) (workspaceapi.Pid, error) {
	panic("unimplemented")
}
func (t *testScheme) StartCommand(context.Context, workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	panic("unimplemented")
}
func (t *testScheme) Signal(workspaceapi.Pid, syscall.Signal) error {
	panic("unimplemented")
}

func (t *testScheme) NewFile(fd uintptr, name string) workspaceapi.File {
	panic("unimplemented")
}

func (t *testScheme) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI(fmt.Sprintf("%s://%s", t.scheme, filepath.Join("/", path)))
}

func (t *testScheme) Open(path string, flag int, perm os.FileMode) (workspaceapi.File, *workspaceapi.Error) {
	return t.openFunc(path, flag, perm)
}

func (t *testScheme) Remove(path string) error {
	return t.removeFunc(path)
}

func (t *testScheme) Rename(old, new string) error {
	return t.renameFunc(old, new)
}

func (t *testScheme) Stat(path string) (os.FileInfo, error) {
	return t.statFunc(path)
}

func (t *testScheme) NewPty(context.Context) (ret workspaceapi.Pty, err error) {
	panic("unimplemented")
}

func (t *testScheme) SetPtySize(p workspaceapi.Pty, width, height int) (err error) {
	panic("unimplemented")
}

func (t *testScheme) Lstat(path string) (os.FileInfo, error) {
	return t.lstatFunc(path)
}

func (t *testScheme) ReadLink(path string) (string, error) {
	return path, nil
}

func (t *testScheme) ReadDir(string) (
	[]os.DirEntry, error,
) {
	panic("unimplemented")
}

func (t *testScheme) MkdirAll(path string, perm os.FileMode) error {
	panic("unimplemented")
}

func (t *testScheme) Watch(
	path string, c chan<- workspaceapi.EventInfo, events ...workspaceapi.Event,
) (int, error) {
	panic("unimplemented")
}

func (t *testScheme) StopWatch(ID int) error {
	panic("unimplemented")
}

func (t *testScheme) Close() error {
	return nil
}
