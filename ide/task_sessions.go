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

package ide

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/ide/idetask"
)

const (
	taskSessionDocumentKind   = "task-session"
	taskSessionDocumentPrefix = "task-sessions:"
	taskSessionAutoNamePrefix = "__open-task-session-"
)

type taskSessionDocument struct {
	Kind                     string
	WorkspaceURI             string
	Name                     string
	TaskName                 string
	Filter                   string
	Cmd                      string
	Args                     []string
	MinimizeAlignment        component.Alignment
	WindowID                 uint64
	WindowMinimized          bool
	WindowMinimizedAlignment component.Alignment
}

func taskSessionDocumentID(workspaceURI string, name string) string {
	return taskSessionDocumentPrefix + url.QueryEscape(workspaceURI) + ":" + url.QueryEscape(name)
}

func taskSessionAutoName(idx int) string {
	return fmt.Sprintf("%s%06d", taskSessionAutoNamePrefix, idx)
}

func (e *ex) saveOpenTaskSessions(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := e.clearOpenTaskSessions(ctx); err != nil {
		return err
	}
	ret := new(multierror.Error)
	var idx int
	for _, info := range e.tasks.ListTasks() {
		if len(info.CmdAndArgs) == 0 {
			continue
		}
		name := taskSessionAutoName(idx)
		idx++
		doc := taskSessionDocument{
			Kind:                     taskSessionDocumentKind,
			WorkspaceURI:             e.workspaceURI.String(),
			Name:                     name,
			TaskName:                 info.Name,
			Filter:                   info.Filter,
			Cmd:                      info.CmdAndArgs[0],
			Args:                     append([]string(nil), info.CmdAndArgs[1:]...),
			MinimizeAlignment:        info.MinimizeAlignment,
			WindowID:                 info.WindowID,
			WindowMinimized:          info.WindowMinimized,
			WindowMinimizedAlignment: info.WindowMinimizedAlignment,
		}
		err := e.storage.Set(ctx, taskSessionDocumentID(e.workspaceURI.String(), doc.Name), doc)
		ret = multierror.Append(ret, err)
	}
	return ret.ErrorOrNil()
}

func (e *ex) hasOpenTaskSessions(ctx context.Context) (bool, error) {
	docs, err := e.openTaskSessionDocuments(ctx)
	return len(docs) != 0, err
}

func (e *ex) restoreOpenTaskSessions(ctx context.Context) error {
	docs, err := e.openTaskSessionDocuments(ctx)
	if err != nil {
		return err
	}
	ret := new(multierror.Error)
	for _, doc := range docs {
		if doc.TaskName == "" {
			doc.TaskName = doc.Name
		}
		if doc.TaskName == "" || doc.Cmd == "" {
			continue
		}
		task := idetask.Task{
			Name:              doc.TaskName,
			Filter:            doc.Filter,
			Cmd:               doc.Cmd,
			Args:              append([]string(nil), doc.Args...),
			MinimizeAlignment: doc.MinimizeAlignment,
		}
		if err := e.tasks.RunTask(task); err != nil {
			ret = multierror.Append(ret, fmt.Errorf("restore task %q: %w", doc.TaskName, err))
			continue
		}
		if win := e.taskWindow(doc.TaskName); win != nil {
			if doc.WindowMinimized {
				minimizeWindow(win, doc.WindowMinimizedAlignment)
			} else {
				win.Unminimize()
			}
		}
	}
	return ret.ErrorOrNil()
}

func (e *ex) taskWindow(name string) browser.Window {
	var ret browser.Window
	e.comp.Browser().IterateWindows(func(win browser.Window) {
		if ret != nil {
			return
		}
		content, err := win.Content()
		if err != nil {
			return
		}
		task, ok := content.(*idetask.Task)
		if !ok {
			return
		}
		if task.Info().Name == name {
			ret = win
		}
	})
	return ret
}

func minimizeWindow(win browser.Window, alignment component.Alignment) {
	switch alignment {
	case component.AlignmentTop:
		win.MinimizeUp(0)
	case component.AlignmentBottom:
		win.MinimizeDown(0)
	case component.AlignmentLeft:
		win.MinimizeLeft(0)
	default:
		win.MinimizeRight(0)
	}
}

func (e *ex) clearOpenTaskSessions(ctx context.Context) error {
	docs, err := e.openTaskSessionDocuments(ctx)
	if err != nil {
		return err
	}
	ret := new(multierror.Error)
	for _, doc := range docs {
		ret = multierror.Append(ret,
			e.storage.Delete(ctx, taskSessionDocumentID(e.workspaceURI.String(), doc.Name)))
	}
	return ret.ErrorOrNil()
}

func (e *ex) openTaskSessionDocuments(ctx context.Context) ([]taskSessionDocument, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var docs []taskSessionDocument
	for i := 0; ; i++ {
		name := taskSessionAutoName(i)
		var doc taskSessionDocument
		err := e.storage.Get(ctx, taskSessionDocumentID(e.workspaceURI.String(), name), &doc)
		if errors.Is(err, storageapi.ErrNotFound) {
			return docs, nil
		}
		if err != nil {
			return nil, err
		}
		if doc.Name == "" {
			doc.Name = name
		}
		if doc.WorkspaceURI != e.workspaceURI.String() || doc.Name == "" {
			continue
		}
		docs = append(docs, doc)
	}
}
