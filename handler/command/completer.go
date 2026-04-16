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

package command

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"mvdan.cc/sh/v3/shell"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/workspace/walkdir"
)

// Completer abstracts the ability to complete command arguments.
type Completer interface {
	// Complete takes the given command and arguments and returns an iterator
	// over an expanded list of options for the last argument. It also returns
	// an expanded version of the last argument, if there is one, or an empty
	// string if the last argument could/should not be automatically expanded.
	Complete(ctx context.Context, args []string) (
		iterator.Iterator[string], string, error,
	)
}

// FuncCompleter returns a Completer that calls fn every time Complete is called.
func FuncCompleter(
	fn func(context.Context, []string) (iterator.Iterator[string], string, error),
) Completer {
	return fnCompleter{fn: fn}
}

// NopCompleter returns a Completer that does nothing.
func NopCompleter() Completer {
	return fnCompleter{}
}

type fnCompleter struct {
	fn func(context.Context, []string) (iterator.Iterator[string], string, error)
}

func (d fnCompleter) Complete(
	ctx context.Context, args []string,
) (iterator.Iterator[string], string, error) {
	if d.fn != nil {
		return d.fn(ctx, args)
	}
	return iterator.Empty[string](), "", nil
}

// FilePathCompleter returns a files path completer with the given directory reader.
func FilePathCompleter(reader walkdir.Reader) Completer {
	return walkDirCompleter(reader, false)
}

// DirsCompleter returns a files path completer with the given directory reader.
func DirsCompleter(reader walkdir.Reader) Completer {
	return walkDirCompleter(reader, true)
}

func walkDirCompleter(reader walkdir.Reader, dirOnly bool) Completer {
	traverse := func(
		ctx context.Context, w walkdir.Reader, root string,
	) (iterator.Iterator[string], error) {
		fn := walkdir.ListFiles
		if dirOnly {
			fn = walkdir.ListDirs
		}
		it, err := fn(ctx, w, root)
		if err != nil {
			return nil, err
		}
		it = iterator.Filter(it, func(val string) bool {
			return !strings.HasSuffix(val, ".swp")
		})
		if dirOnly {
			it = iterator.Filter(it, func(val string) bool {
				return !strings.HasPrefix(val, ".")
			})
		}
		return it, nil
	}
	return FuncCompleter(func(
		ctx context.Context, args []string,
	) (iterator.Iterator[string], string, error) {
		if len(args) == 0 || args[len(args)-1] == "" {
			it, err := traverse(ctx, reader, ".")
			if err != nil {
				return nil, "", err
			}
			return it, "", nil
		}

		var modifiedLast string
		last := args[len(args)-1]

		// take ~ as the home of the user using the editor.
		// rather than the home directory of the user at the workspace.
		// do not always expand without making sure that we are not
		// erasing trailing /, which prevents user from editing files
		// in folders.
		var err error
		if strings.Contains(last, "~") {
			last, err = workspaceapi.ExpandPath(last, user.Current,
				func() (string, error) {
					// do not really expand to cwd,
					// let parseURIOrWorkspaceURI take care of that
					return ".", nil
				})
			if err != nil {
				return nil, "", fmt.Errorf("expand path: %v", err)
			}
			modifiedLast = last
		}

		uri, err := parseURIOrWorkspaceURI(reader, last)
		if err != nil {
			return nil, "", err
		}

		cwd, err := reader.URI(".")
		if err != nil {
			return nil, "", err
		}

		// if filter is absolute path and it happens to be the current working
		// directory of the given reader, the iterator returned by walkdir.ListFiles
		// will return paths relative to it, but filter will be absolute, machting
		// no results.
		needsExpand := workspaceapi.HasPrefix(uri, cwd) && filepath.IsAbs(last)

		it, err := traverse(ctx, reader, uri.Path())
		if err != nil {
			return nil, "", err
		}
		if needsExpand {
			it = iterator.Map(it, func(val string) string {
				return workspaceapi.Join(cwd, val).Path()
			})
		}
		return it, modifiedLast, nil
	})
}

// OutputLinesCompleter returns a files path completer with the given directory reader.
func OutputLinesCompleter(w schemeapi.Executor, cmdAndArgs []string) Completer {
	return FuncCompleter(func(
		ctx context.Context, args []string,
	) (iterator.Iterator[string], string, error) {
		if len(cmdAndArgs) == 0 {
			return nil, "", errors.New("expected at least one argument with the name " +
				"of the executable to run")
		}
		ch := make(chan error)
		cmdAndArgsStr := strings.Join(cmdAndArgs, " ")
		// NOTE: this uses os.Getenv, but it should use the workspace's
		// Getenv mechanism, which should be implemented at some point.
		cmdAndArgs, err := shell.Fields(cmdAndArgsStr, os.Getenv)
		if err != nil {
			return nil, "", fmt.Errorf("expand shell arguments: %w", err)
		}
		cmd := workspaceapi.Cmd{
			Path:    cmdAndArgs[0],
			Watcher: workspaceapi.ChanProcessWatcher(ch),
		}
		if len(cmdAndArgs) > 1 {
			cmd.Args = cmdAndArgs[1:]
		}

		pr, pw := io.Pipe()
		cmd.Stdout = pw
		scanner := bufio.NewScanner(pr)
		pid, err := w.StartCommand(ctx, cmd)
		if err != nil {
			return nil, "", err
		}

		go debug.CapturePanicReport(func() {

			select {
			case <-ctx.Done():
			case <-ch:
			}
			_ = pr.Close()

		})
		return iterator.FromFunc(func(ctx context.Context) (string, bool, error) {
			if scanner.Scan() {
				return scanner.Text(), true, nil
			}
			if err := scanner.Err(); err != nil {
				return "", false, err
			}
			return "", false, nil // EOF
		}, func() (ret error) {
			if err := w.Signal(pid, syscall.SIGINT); err != nil {
				ret = errors.Join(ret, err)
			}
			if err := pr.Close(); err != nil {
				ret = errors.Join(ret, err)
			}
			return ret
		}), "", nil
	})
}

func parseURIOrWorkspaceURI(reader walkdir.Reader, path string) (workspaceapi.URI, error) {
	uri, err := workspaceapi.ParseURI(path)
	if err != nil {
		uri, err = reader.URI(path)
	}
	return uri, err
}
