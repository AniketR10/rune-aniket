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

package idetask

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/ide/plugin"
)

func TestManager(t *testing.T) {
	t.Run("StopTask closes windows with no leaks", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		const N = 25
		for i := range N {
			require.NoError(t, m.RunTask(Task{
				Name: fmt.Sprintf("N%02d", i),
				Cmd:  "run",
				Args: []string{fmt.Sprint(i)},
			}))
		}
		for i := range N {
			require.NoError(t, m.StopTask(fmt.Sprintf("N%02d", i)))
		}
		for i, w := range wm.Created() {
			assert.True(t, w.closed, fmt.Sprintf("window %d", i))
		}
	})

	t.Run("RunTask starts and creates window", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		task := Task{
			Name:   "build",
			Cmd:    "echo",
			Args:   []string{"hello"},
			Filter: "",
		}
		err := m.RunTask(task)
		require.NoError(t, err, "RunTask should succeed")

		cmds := exec.StartedCmds()
		require.Len(t, cmds, 1)
		assert.Equal(t, "echo", cmds[0].Path)
		assert.Equal(t, []string{"hello"}, cmds[0].Args)

		ws := wm.Created()
		require.Len(t, ws, 1)
	})

	t.Run("OnFocus task unminimizes it", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		task := Task{Name: "build", Cmd: "echo"}
		err := m.RunTask(task)
		require.NoError(t, err, "RunTask should succeed")

		ws := wm.Created()
		require.Len(t, ws, 1)

		m.onFocus(&fakeWindow{}, ws[0])
		_, minimized := ws[0].IsMinimized()
		assert.False(t, minimized)
	})

	t.Run("OnFocus changes to other window, minimizes it", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		task := Task{Name: "build", Cmd: "echo"}
		err := m.RunTask(task)
		require.NoError(t, err, "RunTask should succeed")

		ws := wm.Created()
		require.Len(t, ws, 1)

		m.onFocus(&fakeWindow{}, ws[0])
		_, minimized := ws[0].IsMinimized()
		assert.False(t, minimized)

		m.onFocus(ws[0], &fakeWindow{})
		_, minimized = ws[0].IsMinimized()
		assert.True(t, minimized)
	})

	t.Run("status color paints frame Bg and preserves configured Fg/Attrs",
		func(t *testing.T) {
			wm := newFakeBrowser()
			exec := newFakeScheme()
			m := newTestManager(wm, exec)

			// Configure the default non-focus frame attrs from config. The
			// newly created floating window may transiently have focus attrs;
			// task minimized frames must still use this configured baseline.
			configured := term.Attributes{
				Fg:    term.ColorSilver,
				Bg:    term.ColorNavy,
				Attrs: term.AttrBold,
			}
			m.SetFrameAttr(configured)
			focusConfigured := term.Attributes{
				Fg:    term.ColorWhite,
				Bg:    term.ColorTeal,
				Attrs: term.AttrUnderline,
			}
			m.SetFocusFrameAttr(focusConfigured)
			wm.nextFrameAttr = term.Attributes{Fg: term.ColorRed}

			require.NoError(t, m.RunTask(Task{Name: "build", Cmd: "echo"}))
			ws := wm.Created()
			require.Len(t, ws, 1)

			got := ws[0].FrameAttr()
			assert.Equal(t, configured.Fg, got.Fg,
				"configured frame Fg must be preserved")
			assert.Equal(t, term.ColorGray, got.Bg,
				"running status should paint frame Bg")
			assert.Equal(t, configured.Attrs, got.Attrs,
				"configured frame Attrs must be preserved")

			m.onFocus(&fakeWindow{}, ws[0])
			assert.Equal(t, focusConfigured, ws[0].FrameAttr(),
				"unminimize should apply the configured focus frame attrs")
		})

	t.Run("SetMaxWidth changes to max width passed to new task's plugin handler", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		var actualMaxWidth int
		var mu sync.Mutex
		m.newPlugin = func(
			publisher browser.EventPublisher, notifications browser.Notifications,
			e schemeapi.Executor, t schemeapi.Terminal, tm browser.TabManager,
			cmdAndArgs []string, maxWidth int, opts ...plugin.Option,
		) (browser.ScrollableFloating, error) {
			actualMaxWidth = maxWidth
			mu.Unlock()
			return browser.NopScrollableFloatingHandler(
				handler.NopScrollableFloating(),
			), nil
		}

		m.SetMaxWidthHeight(999, 111)
		task := Task{Name: "build", Cmd: "echo"}

		mu.Lock()
		err := m.RunTask(task)
		require.NoError(t, err, "RunTask should succeed")

		mu.Lock()
		assert.Equal(t, 999, actualMaxWidth)
	})

	t.Run("executor failure is surfaced as a failed task", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		exec.startErr = errors.New("executor exploded")
		m := newTestManager(wm, exec)

		err := m.RunTask(Task{Name: "X", Cmd: "noop"})
		require.NoError(t, err)
		assert.Len(t, wm.Created(), 1)

		tasks := m.ListTasks()
		require.Len(t, tasks, 1)
		info := tasks[0]
		assert.False(t, info.LastSuccess)
	})

	t.Run("window creation is surfaced as an error, no tasks are leaked", func(t *testing.T) {
		wm := newFakeBrowser()
		wm.createErr = errors.New("wm failed")
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		err := m.RunTask(Task{Name: "x", Cmd: "noop"})
		require.Error(t, err)

		assert.Len(t, exec.PIDs(), 0)
		tasks := m.ListTasks()
		require.Len(t, tasks, 0)
	})

	t.Run("StopTask kills process and closes window", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "svc", Cmd: "run"}))
		require.NoError(t, m.StopTask("svc"))
		ws := wm.Created()
		require.Len(t, ws, 1)
		assert.True(t, ws[0].closed, "window must be closed on StopTask")

		// handler.Close is what actually kills the process
		require.Len(t, wm.createdHandlers, 1)
		assert.True(t, ws[0].closed, "handler must be closed on StopTask")
	})

	t.Run("StopTask on unkown task is an error", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		err := m.StopTask("nope")
		require.Error(t, err)
	})

	t.Run("StopTask a second time is an error", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "job", Cmd: "run"}))
		require.NoError(t, m.StopTask("job"))

		err := m.StopTask("job")
		require.Error(t, err)
	})

	t.Run("RunTask allows concurrent tasks", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		err := m.RunTask(Task{Name: "A", Cmd: "run", Args: []string{"A"}})
		require.NoErrorf(t, err, "start A")
		err = m.RunTask(Task{Name: "B", Cmd: "run", Args: []string{"B"}})
		require.NoErrorf(t, err, "start B")

		require.Len(t, exec.StartedCmds(), 2)
		require.Len(t, wm.Created(), 2)

		require.NoError(t, m.StopTask("A"))
		time.Sleep(5 * time.Millisecond)

		require.Len(t, exec.StartedCmds(), 2)
		ws := wm.Created()
		require.Len(t, ws, 2)
		closed := 0
		for _, w := range ws {
			if w.closed {
				closed++
			}
		}
		assert.Equal(t, 1, closed, "only one window should be closed")
	})

	t.Run("RunTask with duplicate name is an error", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "dup", Cmd: "run"}))
		require.Error(t, m.RunTask(Task{Name: "dup", Cmd: "run"}))
	})

	t.Run("RunTask sets passed alignment", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		expectedAlignment := component.AlignmentBottom
		wm.createHook = func(_ string, cfg browserapi.FloatingConfig) {
			assert.Equal(t, expectedAlignment, cfg.Alignment&expectedAlignment)
		}

		err := m.RunTask(Task{
			Name:              "aligned",
			Cmd:               "run",
			MinimizeAlignment: expectedAlignment,
		})
		require.NoError(t, err)
	})

	t.Run("many tasks start/stop leave no leaks", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		for i := range 10000 {
			name := fmt.Sprintf("task-%d", i)
			require.NoError(t, m.RunTask(Task{Name: name, Cmd: "run"}))
			require.NoError(t, m.StopTask(name))
		}

		for _, w := range wm.Created() {
			assert.True(t, w.closed)
		}
	})

	t.Run("process exit with error sets the task to errored", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "RainingBlood", Cmd: "Slayer"}))

		assertTaskRunsWithin(t, m, 1*time.Second, "RainingBlood", errors.New("boom"),
			func(info TaskInfo) bool {
				if info.Running {
					return false
				}
				assert.False(t, info.LastSuccess)
				assert.NotZero(t, info.LastDuration)
				return true
			})
	})

	t.Run("process exit with no error sets the task to success", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "Yo Marco el Minuto", Cmd: "La Mala"}))

		assertTaskRunsWithin(t, m, 1*time.Second, "Yo Marco el Minuto", nil,
			func(info TaskInfo) bool {
				if info.Running {
					return false
				}
				assert.True(t, info.LastSuccess)
				assert.NotZero(t, info.LastDuration)
				return true
			})
	})

	t.Run("process exit failure preserves captured task output", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		const output = "captured task output"
		m.newPlugin = func(
			publisher browser.EventPublisher, notifications browser.Notifications,
			e schemeapi.Executor, t schemeapi.Terminal, tm browser.TabManager,
			cmdAndArgs []string, maxWidth int, opts ...plugin.Option,
		) (browser.ScrollableFloating, error) {
			return browser.NopScrollableFloatingHandler(
				handler.NopScrollableFloatingFromComponent(
					component.NewResponsiveString(output, component.StringResponsiveConfig{}),
				)), nil
		}

		require.NoError(t, m.RunTask(Task{Name: "build", Cmd: "make"}))

		taskIfc, ok := m.tasks.Load("build")
		require.True(t, ok)
		task := taskIfc.(*Task)
		task.Resize(80, 24)

		assertTaskRunsWithin(t, m, 1*time.Second, "build", errors.New("exit status 1"),
			func(info TaskInfo) bool {
				return !info.Running
			})
		task.WaitInflight()

		w := term.NewStringWriter(80, 24)
		require.NoError(t, w.Clear(term.Attributes{}))
		task.currentHandler().Draw(w)
		require.NoError(t, w.Flush())
		rendered := w.String()

		assert.Contains(t, rendered, output,
			"failed task should keep showing captured stdout/stderr")
		assert.NotContains(t, rendered, "exit status 1",
			"failed task must not replace output with the error string")
	})

	t.Run("if files change on workspace, task is re-run when idle", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "runtask"}))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool {
				if info.Running {
					return false
				}
				assert.True(t, info.LastSuccess)
				assert.NotZero(t, info.LastDuration)
				return true
			})

		sendEvent(t, m, exec, "task", "temp")
		// wait until it's running
		assertTaskWithin(t, m, 1*time.Second, "task",
			func(info TaskInfo) bool {
				return info.Running
			})
		// wait until it's done running
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool {
				if info.Running {
					return false
				}
				assert.True(t, info.LastSuccess)
				assert.Equal(t, 2, info.Runs)
				assert.NotZero(t, info.LastDuration)
				return true
			})

	})

	t.Run("if files change on workspace and it matches filter, task is re-run", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		task := Task{Name: "task", Cmd: "runtask", Filter: "*.go,*.md"}
		require.NoError(t, m.RunTask(task))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool {
				if info.Running {
					return false
				}
				assert.True(t, info.LastSuccess)
				assert.NotZero(t, info.LastDuration)
				return true
			})

		sendEvent(t, m, exec, "task", "dir/someotherfile.md")
		assertTaskWithin(t, m, 1*time.Second, "task",
			func(info TaskInfo) bool {
				return info.Runs == 2
			})
	})

	t.Run("ReplaceTask runs task with new command and arguments", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		task := Task{Name: "task", Cmd: "runtask", Filter: "*.go,*.md"}
		require.NoError(t, m.RunTask(task))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool {
				if info.Running {
					return false
				}
				assert.True(t, info.LastSuccess)
				assert.NotZero(t, info.LastDuration)
				return true
			})

		require.NoError(t, m.ReplaceTask(Task{Name: task.Name, Cmd: "rumtask", Args: []string{"arg1"}}))
		assertTaskWithin(t, m, 1*time.Second, "task",
			func(info TaskInfo) bool {
				assert.Equal(t, []string{"rumtask", "arg1"}, info.CmdAndArgs)
				return info.Runs == 2
			})
	})

	t.Run("if ignored files change on workspace, task is not run", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		task := Task{Name: "task", Cmd: "runtask", Filter: "*.go,dir/*.md"}
		require.NoError(t, m.RunTask(task))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool {
				if info.Running {
					return false
				}
				assert.True(t, info.LastSuccess)
				assert.NotZero(t, info.LastDuration)
				return true
			})

		sendEvent(t, m, exec, "task", "someotherfile.md")
		time.Sleep(1 * time.Second)
		assertTaskWithin(t, m, 1*time.Second, "task",
			func(info TaskInfo) bool {
				assert.Equal(t, 1, info.Runs)
				return true
			})
	})

	t.Run("if commonly ignored files change on workspace, task is not run", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		task := Task{Name: "task", Cmd: "runtask"}
		require.NoError(t, m.RunTask(task))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool {
				if info.Running {
					return false
				}
				return true
			})

		sendEvent(t, m, exec, "task", ".file.swp")
		time.Sleep(1 * time.Second)
		assertTaskWithin(t, m, 1*time.Second, "task",
			func(info TaskInfo) bool {
				assert.Equal(t, 1, info.Runs)
				return true
			})
	})

	t.Run("ctrl-r restarts a running task", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "job", Cmd: "run", Args: []string{"serve"}}))

		taskIfc, ok := m.tasks.Load("job")
		require.True(t, ok)
		task := taskIfc.(*Task)

		// Press ctrl-r while running; should set restartPending.
		exit, handled := task.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'r'})
		assert.False(t, exit)
		assert.True(t, handled)

		// Complete the old run; the donech goroutine should auto-restart.
		assertTaskRunsWithin(t, m, 1*time.Second, "job", nil,
			func(info TaskInfo) bool {
				return info.Running && info.Runs == 2
			})

		cmds := exec.StartedCmds()
		require.GreaterOrEqual(t, len(cmds), 2)
		assert.Equal(t, "run", cmds[0].Path)
		assert.Equal(t, "run", cmds[1].Path)
		assert.Equal(t, []string{"serve"}, cmds[0].Args)
		assert.Equal(t, []string{"serve"}, cmds[1].Args)
	})

	t.Run("ctrl-r on idle task starts it immediately", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "job", Cmd: "build"}))

		// Complete the initial run so task becomes idle.
		assertTaskRunsWithin(t, m, 1*time.Second, "job", nil,
			func(info TaskInfo) bool {
				return !info.Running && info.Runs == 1
			})

		taskIfc, ok := m.tasks.Load("job")
		require.True(t, ok)
		task := taskIfc.(*Task)

		// Press ctrl-r while idle; should start immediately.
		exit, handled := task.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'r'})
		assert.False(t, exit)
		assert.True(t, handled)

		assertTaskWithin(t, m, 1*time.Second, "job",
			func(info TaskInfo) bool {
				return info.Running && info.Runs == 2
			})
	})

	t.Run("ctrl-r uppercase R also restarts", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "job", Cmd: "build"}))

		// Complete the initial run.
		assertTaskRunsWithin(t, m, 1*time.Second, "job", nil,
			func(info TaskInfo) bool {
				return !info.Running && info.Runs == 1
			})

		taskIfc, ok := m.tasks.Load("job")
		require.True(t, ok)
		task := taskIfc.(*Task)

		// Press ctrl-R (uppercase).
		exit, handled := task.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'R'})
		assert.False(t, exit)
		assert.True(t, handled)

		assertTaskWithin(t, m, 1*time.Second, "job",
			func(info TaskInfo) bool {
				return info.Running && info.Runs == 2
			})
	})

	t.Run("ctrl-c still pauses without scheduling a restart", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "job", Cmd: "run"}))

		taskIfc, ok := m.tasks.Load("job")
		require.True(t, ok)
		task := taskIfc.(*Task)

		// Press ctrl-c while running.
		exit, _ := task.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'c'})
		assert.False(t, exit)

		// Complete the run; should NOT restart (paused, not restarting).
		assertTaskRunsWithin(t, m, 1*time.Second, "job", nil,
			func(info TaskInfo) bool {
				if info.Running {
					return false
				}
				assert.Equal(t, 1, info.Runs)
				return true
			})
	})

	t.Run("same file retriggering the task is detected as an infinite loop", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "build"}))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool { return !info.Running })

		taskIfc, ok := m.tasks.Load("task")
		require.True(t, ok)
		task := taskIfc.(*Task)
		task.Resize(80, 24)

		for range loopThreshold {
			triggerRerun(t, m, exec, "task", "out.bin")
		}

		assertTaskWithin(t, m, 1*time.Second, "task",
			func(info TaskInfo) bool {
				if !info.LoopHalted {
					return false
				}
				assert.False(t, info.Running)
				assert.False(t, info.LastSuccess)
				return true
			})

		assert.Eventually(t, func() bool {
			return watchStopped(m, exec, "task")
		}, 1*time.Second, 10*time.Millisecond,
			"watch should be stopped once the loop is detected")

		w := term.NewStringWriter(80, 24)
		require.NoError(t, w.Clear(term.Attributes{}))
		task.currentHandler().Draw(w)
		require.NoError(t, w.Flush())
		assert.Contains(t, w.String(), "out.bin",
			"loop error should name the offending file")
	})

	t.Run("recreating a loop-halted task recovers and runs normally", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "build"}))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool { return !info.Running })

		for range loopThreshold {
			triggerRerun(t, m, exec, "task", "out.bin")
		}
		assertTaskWithin(t, m, 1*time.Second, "task",
			func(info TaskInfo) bool { return info.LoopHalted })

		// The user recreates the task, e.g. with a filter that excludes
		// the offending file. Recovery requires removing the halted task
		// and starting a fresh one under the same name.
		require.NoError(t, m.StopTask("task"))
		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "build", Filter: "*.go"}))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool { return !info.Running })

		info := mustTaskInfo(t, m, "task")
		assert.False(t, info.LoopHalted, "recreated task must not stay halted")
		assert.True(t, info.LastSuccess)
		assert.False(t, watchStopped(m, exec, "task"),
			"recreated task should hold a live watch")

		// A subsequent matching change reruns the recovered task normally.
		triggerRerun(t, m, exec, "task", "main.go")
		info = mustTaskInfo(t, m, "task")
		assert.False(t, info.LoopHalted)
		assert.True(t, info.LastSuccess)
		assert.Equal(t, 2, info.Runs)
	})

	t.Run("replacing a loop-halted task with a new filter recovers and retriggers", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "build"}))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool { return !info.Running })

		for range loopThreshold {
			triggerRerun(t, m, exec, "task", "out.bin")
		}
		assertTaskWithin(t, m, 1*time.Second, "task",
			func(info TaskInfo) bool { return info.LoopHalted })

		// Replace the halted task with one that filters to source files,
		// matching the tasknew -> "replace?" -> yes recovery flow.
		require.NoError(t, m.ReplaceTask(Task{Name: "task", Cmd: "build", Filter: "*.go"}))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool { return !info.Running })

		info := mustTaskInfo(t, m, "task")
		assert.False(t, info.LoopHalted, "replaced task must not stay halted")
		assert.Equal(t, "*.go", info.Filter, "replace must apply the new filter")
		assert.False(t, watchStopped(m, exec, "task"),
			"replaced task should hold a live watch")

		// A subsequent matching change retriggers the replaced task.
		runsBefore := info.Runs
		triggerRerun(t, m, exec, "task", "main.go")
		info = mustTaskInfo(t, m, "task")
		assert.False(t, info.LoopHalted)
		assert.True(t, info.LastSuccess)
		assert.Equal(t, runsBefore+1, info.Runs,
			"matching change should rerun the recovered task")

		// A change that does not match the new filter must not retrigger it.
		runsBefore = info.Runs
		sendEvent(t, m, exec, "task", "out.bin")
		time.Sleep(200 * time.Millisecond)
		info = mustTaskInfo(t, m, "task")
		assert.Equal(t, runsBefore, info.Runs,
			"non-matching change must not rerun the recovered task")
	})

	t.Run("reruns below threshold do not halt the task", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "build"}))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool { return !info.Running })

		for range loopThreshold - 1 {
			triggerRerun(t, m, exec, "task", "out.bin")
		}

		info := mustTaskInfo(t, m, "task")
		assert.False(t, info.LoopHalted)
		assert.False(t, watchStopped(m, exec, "task"))
	})

	t.Run("different files below threshold do not halt the task", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "build"}))
		assertTaskRunsWithin(t, m, 1*time.Second, "task", nil,
			func(info TaskInfo) bool { return !info.Running })

		for i := range loopThreshold + 2 {
			triggerRerun(t, m, exec, "task", fmt.Sprintf("out-%d.bin", i))
		}

		info := mustTaskInfo(t, m, "task")
		assert.False(t, info.LoopHalted)
		assert.False(t, watchStopped(m, exec, "task"))
	})
}

// TestTaskHandlerAccessRace exercises the Task render/handler accessors
// concurrently with the donech goroutine that swaps t.handler on run
// completion. Under -race it reproduces the data race between
// Task.Dimensions/Draw reads and doSetError writes of t.handler.
func TestTaskHandlerAccessRace(t *testing.T) {
	wm := newFakeBrowser()
	exec := newFakeScheme()
	m := newTestManager(wm, exec)

	require.NoError(t, m.RunTask(Task{Name: "job", Cmd: "run"}))

	taskIfc, ok := m.tasks.Load("job")
	require.True(t, ok)
	task := taskIfc.(*Task)

	var stop atomic.Bool
	var readers sync.WaitGroup
	readers.Go(func() {
		for !stop.Load() {
			_, _ = task.Dimensions()
			_, _ = task.Selection()
			_, _, _ = task.Cursor()
			_ = task.MaxSeekOffset()
			_ = task.SeekOffset()
			_ = task.SeekDown()
			_ = task.SeekUp()
		}
	})

	for range 50 {
		task.donech <- errors.New("boom")
		task.WaitInflight()
	}

	stop.Store(true)
	readers.Wait()
}

func sendEvent(t *testing.T, m *Manager, exec *fakeScheme, taskname, filename string) {
	sendEventInfo(t, m, exec, taskname, testEventInfo{uri: mustURI(t, filename)})
}

// sendEventInfo pushes a fully-specified event onto the task's watch
// channel, locking the fake scheme so the lookup does not race the
// watcher's StopWatch.
func sendEventInfo(t *testing.T, m *Manager, exec *fakeScheme, taskname string, ev testEventInfo) {
	t.Helper()
	taskIfc, ok := m.tasks.Load(taskname)
	require.True(t, ok)
	task := taskIfc.(*Task)
	task.mu.Lock()
	id := task.watchID
	task.mu.Unlock()
	exec.mu.Lock()
	ch := exec.tasks[id]
	exec.mu.Unlock()
	require.NotNil(t, ch)
	ch <- ev
}

func mustURI(t *testing.T, filename string) workspaceapi.URI {
	t.Helper()
	uri, err := workspaceapi.ParseURI("memory:///" + strings.TrimPrefix(filename, "/"))
	require.NoError(t, err)
	return uri
}

// triggerRerun emits a file event, settles the resulting run successfully,
// and waits for the task to return to idle. It models one
// build -> file-change -> rebuild cycle.
//
// It waits on the monotonic Runs counter rather than the transient Running
// edge: when this rerun crosses the loop-detection threshold, the watcher
// halts the task (clearing running) in the same iteration that started it,
// so polling for Running races the halt and can miss it. Runs is only
// incremented and is observable whether the run settles or halts.
func triggerRerun(t *testing.T, m *Manager, exec *fakeScheme, taskname, filename string) {
	t.Helper()
	before := mustTaskInfo(t, m, taskname).Runs
	sendEvent(t, m, exec, taskname, filename)
	assertTaskWithin(t, m, 1*time.Second, taskname,
		func(info TaskInfo) bool { return info.Runs > before })
	taskIfc, ok := m.tasks.Load(taskname)
	require.True(t, ok)
	taskIfc.(*Task).donech <- nil
	assertTaskWithin(t, m, 1*time.Second, taskname,
		func(info TaskInfo) bool { return !info.Running })
}

// watchStopped reports whether the task's workspace watch channel has
// been removed from the fake scheme (i.e. StopWatch ran).
func watchStopped(m *Manager, exec *fakeScheme, taskname string) bool {
	taskIfc, ok := m.tasks.Load(taskname)
	if !ok {
		return true
	}
	task := taskIfc.(*Task)
	exec.mu.Lock()
	defer exec.mu.Unlock()
	_, ok = exec.tasks[task.watchID]
	return !ok
}

func mustTaskInfo(t *testing.T, m *Manager, taskname string) TaskInfo {
	t.Helper()
	taskIfc, ok := m.tasks.Load(taskname)
	require.True(t, ok)
	return taskIfc.(*Task).Info()
}

func newTestManager(b *fakeBrowser, scheme schemeapi.Scheme) *Manager {
	m := NewManager(b, b, scheme, func(fn func()) bool {
		fn()
		return true
	})
	m.newPlugin = func(
		publisher browser.EventPublisher, notifications browser.Notifications,
		e schemeapi.Executor, t schemeapi.Terminal, tm browser.TabManager,
		cmdAndArgs []string, maxWidth int, opts ...plugin.Option,
	) (browser.ScrollableFloating, error) {
		_, err := e.StartCommand(context.Background(), workspaceapi.Cmd{
			Path: cmdAndArgs[0],
			Args: cmdAndArgs[1:],
			SysProcAttr: &syscall.SysProcAttr{
				Setsid:  true,
				Setctty: true,
			},
		})
		if err != nil {
			return nil, err
		}
		var closed bool
		handler := browser.FuncScrollableFloatingHandler(
			handler.NopScrollableFloating(),
			func() error {
				if closed {
					return errors.New("already closed")
				}
				closed = true
				return nil
			})
		b.createdHandlers = append(b.createdHandlers, handler)
		return handler, nil
	}
	return m
}

func assertTaskRunsWithin(
	t *testing.T, m *Manager, within time.Duration,
	name string, doneErr error, fn func(info TaskInfo) bool,
) {
	t.Helper()
	taskIfc, ok := m.tasks.Load(name)
	require.True(t, ok)
	task := taskIfc.(*Task)
	task.donech <- doneErr

	assertTaskWithin(t, m, within, name, fn)
}

func assertTaskWithin(
	t *testing.T, m *Manager, within time.Duration,
	name string, fn func(info TaskInfo) bool,
) {
	timeout := time.After(within)
	for {
		taskIfc, ok := m.tasks.Load(name)
		require.True(t, ok)
		task := taskIfc.(*Task)

		select {
		case <-timeout:
			t.Logf("didn't set the task to done within the expected time")
			t.Fail()
			return
		default:
			info := task.Info()
			if !fn(info) {
				continue
			}
			return
		}
	}
}

// fakeScheme simulates process start/exit and captures the Cmd passed in.
// It writes deterministic stdout/stderr and terminates when ctx is canceled.
type fakeScheme struct {
	schemeapi.Scheme
	mu        sync.Mutex
	started   []workspaceapi.Cmd
	pidSeq    atomic.Int64
	procs     map[workspaceapi.Pid]*procState
	startErr  error                      // if set, StartCommand will return this error
	watchErr  error                      // if set, Watch will return this error
	startHook func(cmd workspaceapi.Cmd) // optional test hook
	tasks     map[int]chan<- schemeapi.EventInfo
	next      int
}

type procState struct {
	ctx       context.Context
	cancelled atomic.Bool
	doneCh    chan struct{}

	stdout io.Writer
	stderr io.Writer
}

func newFakeScheme() *fakeScheme {
	return &fakeScheme{
		procs: make(map[workspaceapi.Pid]*procState),
		tasks: make(map[int]chan<- schemeapi.EventInfo),
	}
}

func (f *fakeScheme) StopWatch(id int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.tasks, id)
	return nil
}

func (f *fakeScheme) Watch(
	path string, c chan<- schemeapi.EventInfo, events ...schemeapi.Event,
) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.watchErr != nil {
		return 0, f.watchErr
	}
	f.next++
	f.tasks[f.next] = c
	return f.next, nil
}

func (f *fakeScheme) ReadDir(string) ([]os.DirEntry, error) {
	return nil, nil
}

func (f *fakeScheme) URI(path string) (workspaceapi.URI, error) {
	// Root the workspace at "/" so that workspaceapi.RelPath of an event
	// URI (built as memory:///<file>) yields a clean relative path with
	// no leading separator. A "memory://." cwd would relativize to
	// "/<file>", whose empty leading path component breaks nested-path
	// glob filters like "src/*.go". filepath.Join collapses the "///"
	// scheme separator, so build the URI string directly.
	if path == "" || path == "." {
		return workspaceapi.ParseURI("memory:///")
	}
	return workspaceapi.ParseURI("memory:///" + strings.TrimPrefix(path, "/"))
}

func (f *fakeScheme) OpenFile(path string, flag int, perm os.FileMode) (
	workspaceapi.File, error,
) {
	return nil, os.ErrNotExist
}

func (f *fakeScheme) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	if f.startHook != nil {
		f.startHook(cmd)
	}
	if f.startErr != nil {
		return 0, f.startErr
	}

	p := workspaceapi.Pid(f.pidSeq.Add(1))
	ps := &procState{
		ctx:    ctx,
		doneCh: make(chan struct{}),
		stdout: cmd.Stdout,
		stderr: cmd.Stderr,
	}
	f.mu.Lock()
	f.started = append(f.started, cmd)
	f.procs[p] = ps
	f.mu.Unlock()

	go func() {
		defer close(ps.doneCh)
		<-ctx.Done()
		ps.cancelled.Store(true)
	}()

	return p, nil
}

func (f *fakeScheme) PIDs() []workspaceapi.Pid {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]workspaceapi.Pid, 0, len(f.procs))
	for pid := range f.procs {
		out = append(out, pid)
	}
	return out
}

func (f *fakeScheme) StartedCmds() []workspaceapi.Cmd {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]workspaceapi.Cmd, len(f.started))
	copy(cp, f.started)
	return cp
}

func (f *fakeScheme) Wait(pid workspaceapi.Pid, timeout time.Duration) bool {
	f.mu.Lock()
	ps := f.procs[pid]
	f.mu.Unlock()
	if ps == nil {
		return true
	}
	select {
	case <-ps.doneCh:
		return true
	case <-time.After(timeout):
		return false
	}
}

type fakeWindow struct {
	mu        sync.Mutex
	name      string
	minimized bool
	closed    bool
	alignment component.Alignment

	stdoutBuf bytes.Buffer
	stderrBuf bytes.Buffer
	statusBuf bytes.Buffer

	minAlign component.Alignment
	cfg      browserapi.FloatingConfig

	frameAttr term.Attributes
}

func (w *fakeWindow) Content() tui.Handler {
	return nil
}

func (w *fakeWindow) Unminimize() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.minimized = false
	return true
}
func (w *fakeWindow) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}

func (w *fakeWindow) SetContent(browserapi.Handler) error {
	panic("unimplemeted")
}

func (w *fakeWindow) Focus() (bool, error) {
	panic("unimplemeted")
}

func (w *fakeWindow) WindowID() uint64 {
	return uint64(uintptr(unsafe.Pointer(w)))
}

func (w *fakeWindow) ID() uint64 {
	return uint64(uintptr(unsafe.Pointer(w)))
}

func (w *fakeWindow) IsFloating() bool {
	return true
}

func (w *fakeWindow) IsMinimized() (component.Alignment, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.alignment, w.minimized
}

func (w *fakeWindow) MinimizeUp(padding int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.alignment = component.AlignmentTop
	w.minimized = true
	return true
}
func (w *fakeWindow) MinimizeDown(padding int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.alignment = component.AlignmentBottom
	w.minimized = true
	return true
}

func (w *fakeWindow) MinimizeLeft(padding int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.alignment = component.AlignmentLeft
	w.minimized = true
	return true
}
func (w *fakeWindow) MinimizeRight(padding int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.alignment = component.AlignmentRight
	w.minimized = true
	return true
}
func (w *fakeWindow) SetFrameAttr(attr term.Attributes) (term.Attributes, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	prev := w.frameAttr
	w.frameAttr = attr
	return prev, true
}

func (w *fakeWindow) FrameAttr() term.Attributes {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.frameAttr
}

func (w *fakeWindow) Position() term.Coordinates { return term.Coordinates{} }
func (w *fakeWindow) Width() int                 { return 0 }
func (w *fakeWindow) Height() int                { return 0 }

func (w *fakeWindow) Closed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

type fakeBrowser struct {
	browser.Browser
	mu              sync.Mutex
	baseWindow      *fakeWindow
	focus           *fakeWindow
	created         []*fakeWindow
	createErr       error
	createHook      func(taskName string, cfg browserapi.FloatingConfig)
	createdHandlers []browser.ScrollableFloating
	nextFrameAttr   term.Attributes
}

func newFakeBrowser() *fakeBrowser {
	base := &fakeWindow{}
	return &fakeBrowser{
		baseWindow: base,
		focus:      base,
	}
}

type fakeBrowserWindow struct {
	*fakeWindow
}

func (w fakeBrowserWindow) Content() (browserapi.Handler, error) {
	return w.fakeWindow.Content().(browserapi.Handler), nil
}

func (m *fakeBrowser) Focus() (browser.Window, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fakeBrowserWindow{m.focus}, nil
}

func (m *fakeBrowser) SetFocus(win browser.Window) (browser.Window, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	prev := fakeBrowserWindow{m.focus}
	m.focus = win.(fakeBrowserWindow).fakeWindow
	return prev, nil
}

func (m *fakeBrowser) PublishEvent(ev term.Event) error {
	return nil
}

func (m *fakeBrowser) RemoveTab(h browserapi.Handler) error {
	return nil
}

func (m *fakeBrowser) Floating(h browser.Floating, cfg browserapi.FloatingConfig) (
	browser.Window, error,
) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	w := &fakeWindow{cfg: cfg, frameAttr: m.nextFrameAttr}
	m.mu.Lock()
	m.created = append(m.created, w)
	m.mu.Unlock()
	if m.createHook != nil {
		m.createHook("", cfg)
	}
	return fakeBrowserWindow{w}, nil
}

func (m *fakeBrowser) Created() []*fakeWindow {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]*fakeWindow, len(m.created))
	copy(cp, m.created)
	return cp
}

type testEventInfo struct {
	uri workspaceapi.URI
	ev  schemeapi.Event
	dir bool
}

func (t testEventInfo) Event() schemeapi.Event {
	return t.ev
}

func (t testEventInfo) URI() workspaceapi.URI {
	return t.uri
}

func (t testEventInfo) IsDir() (bool, error) {
	return t.dir, nil
}

// settleRun pushes a successful donech completion and waits for the task
// to become idle. Unlike triggerRerun it does not send a file event, so
// it settles a run started by some other path (ReplaceTask, ctrl-r).
func settleRun(t *testing.T, m *Manager, name string) {
	t.Helper()
	assertTaskRunsWithin(t, m, time.Second, name, nil,
		func(info TaskInfo) bool { return !info.Running })
}

func runAndSettle(t *testing.T, m *Manager, task Task) {
	t.Helper()
	require.NoError(t, m.RunTask(task))
	settleRun(t, m, task.Name)
}

// TestManagerReplaceTaskEdgeCases probes ReplaceTask, the most fragile
// surface: it tears down and re-arms a watcher in place while preserving
// the window. These cases hunt for lost state, leaked watches, and
// mishandled error/race paths.
func TestManagerReplaceTaskEdgeCases(t *testing.T) {
	t.Run("replace nonexistent task is an error", func(t *testing.T) {
		m := newTestManager(newFakeBrowser(), newFakeScheme())
		err := m.ReplaceTask(Task{Name: "ghost", Cmd: "run"})
		require.Error(t, err)
	})

	t.Run("replace preserves the same window (no new window created)", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})
		require.Len(t, wm.Created(), 1)

		require.NoError(t, m.ReplaceTask(Task{Name: "task", Cmd: "rebuild"}))
		settleRun(t, m, "task")
		assert.Len(t, wm.Created(), 1,
			"replace must reuse the existing window, not create a new one")
	})

	t.Run("replace stops the old watch and leaves exactly one live watch", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})
		require.NoError(t, m.ReplaceTask(Task{Name: "task", Cmd: "build", Filter: "*.go"}))
		settleRun(t, m, "task")

		assert.Eventually(t, func() bool {
			exec.mu.Lock()
			defer exec.mu.Unlock()
			return len(exec.tasks) == 1
		}, time.Second, 10*time.Millisecond,
			"exactly one watch must remain live after replace")
	})

	t.Run("replace clearing a filter reruns on any file again", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		runAndSettle(t, m, Task{Name: "task", Cmd: "build", Filter: "*.go"})

		// With the *.go filter, a .txt change is ignored.
		sendEvent(t, m, exec, "task", "data.txt")
		time.Sleep(150 * time.Millisecond)
		assert.Equal(t, 1, mustTaskInfo(t, m, "task").Runs)

		// Replace with an empty filter; now any file should retrigger.
		require.NoError(t, m.ReplaceTask(Task{Name: "task", Cmd: "build", Filter: ""}))
		settleRun(t, m, "task")
		runsBefore := mustTaskInfo(t, m, "task").Runs
		triggerRerun(t, m, exec, "task", "data.txt")
		assert.Equal(t, runsBefore+1, mustTaskInfo(t, m, "task").Runs,
			"cleared filter should rerun on previously-ignored files")
	})

	t.Run("replace while running does not lose the new command", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		// Start a task and leave it running (do not settle).
		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "build"}))
		assertTaskWithin(t, m, time.Second, "task",
			func(info TaskInfo) bool { return info.Running })

		// Replace while the first run is still in flight.
		require.NoError(t, m.ReplaceTask(Task{Name: "task", Cmd: "rebuild", Args: []string{"x"}}))
		info := mustTaskInfo(t, m, "task")
		assert.Equal(t, []string{"rebuild", "x"}, info.CmdAndArgs,
			"replace must update CmdAndArgs even when a run was in flight")
	})

	t.Run("replace surfaces watch errors without leaving a dead task", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		taskIfc, _ := m.tasks.Load("task")
		oldID := func() int {
			tk := taskIfc.(*Task)
			tk.mu.Lock()
			defer tk.mu.Unlock()
			return tk.watchID
		}()

		exec.watchErr = errors.New("watch exploded")
		err := m.ReplaceTask(Task{Name: "task", Cmd: "build", Filter: "*.go"})
		require.Error(t, err)

		// A failed replace must leave the original watch untouched: it must
		// never tear down the existing watch (which the watcher's deferred
		// StopWatch would do asynchronously after a stray cancel).
		assert.Never(t, func() bool {
			exec.mu.Lock()
			defer exec.mu.Unlock()
			_, live := exec.tasks[oldID]
			return !live
		}, 300*time.Millisecond, 10*time.Millisecond,
			"a failed replace must not strand the task without a watch")

		// And the task must still retrigger via that surviving watch.
		exec.watchErr = nil
		runsBefore := mustTaskInfo(t, m, "task").Runs
		triggerRerun(t, m, exec, "task", "main.go")
		assert.Equal(t, runsBefore+1, mustTaskInfo(t, m, "task").Runs,
			"task must still retrigger after a failed replace")
	})

	t.Run("rapid successive replaces leave exactly one live watch", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		for i := range 8 {
			require.NoError(t, m.ReplaceTask(Task{
				Name: "task", Cmd: "build", Filter: fmt.Sprintf("*.v%d", i),
			}))
			settleRun(t, m, "task")
		}

		assert.Eventually(t, func() bool {
			exec.mu.Lock()
			defer exec.mu.Unlock()
			return len(exec.tasks) == 1
		}, time.Second, 10*time.Millisecond,
			"rapid replaces must not leak watches")
	})

	t.Run("replace then stop leaves no live watch", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})
		require.NoError(t, m.ReplaceTask(Task{Name: "task", Cmd: "build", Filter: "*.go"}))
		settleRun(t, m, "task")
		require.NoError(t, m.StopTask("task"))

		assert.Eventually(t, func() bool {
			exec.mu.Lock()
			defer exec.mu.Unlock()
			return len(exec.tasks) == 0
		}, time.Second, 10*time.Millisecond,
			"stopping a replaced task must close its watch")
	})

	t.Run("replace gives the task a fresh loop window", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		// Almost trip the loop on out.bin.
		for range loopThreshold - 1 {
			triggerRerun(t, m, exec, "task", "out.bin")
		}
		// Replace (new watcher, fresh goroutine-local rerun map).
		require.NoError(t, m.ReplaceTask(Task{Name: "task", Cmd: "build"}))
		settleRun(t, m, "task")

		// One more out.bin event must not immediately halt.
		triggerRerun(t, m, exec, "task", "out.bin")
		assert.False(t, mustTaskInfo(t, m, "task").LoopHalted,
			"replaced task must not inherit the prior loop window")
	})
}

// TestManagerLoopDetection probes the build -> file-change -> rebuild
// loop detector at its boundaries and against interacting state
// transitions (pause, restart, replace).
func TestManagerLoopDetection(t *testing.T) {
	t.Run("exactly threshold reruns halts", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		for range loopThreshold {
			triggerRerun(t, m, exec, "task", "out.bin")
		}
		assertTaskWithin(t, m, time.Second, "task",
			func(info TaskInfo) bool { return info.LoopHalted })
	})

	t.Run("threshold minus one reruns does not halt", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		for range loopThreshold - 1 {
			triggerRerun(t, m, exec, "task", "out.bin")
		}
		assert.False(t, mustTaskInfo(t, m, "task").LoopHalted)
		assert.False(t, watchStopped(m, exec, "task"))
	})

	t.Run("two files each below threshold do not halt", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		// Interleave A and B so each accumulates threshold-1 within the
		// window; neither alone crosses the line.
		for range loopThreshold - 1 {
			triggerRerun(t, m, exec, "task", "a.bin")
			triggerRerun(t, m, exec, "task", "b.bin")
		}
		assert.False(t, mustTaskInfo(t, m, "task").LoopHalted)
	})

	t.Run("loop attributed to the offending file even when interleaved", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		taskIfc, _ := m.tasks.Load("task")
		task := taskIfc.(*Task)
		task.Resize(80, 24)

		// "noise.bin" fires a few times but never crosses; "loop.bin"
		// crosses the threshold and must be the named culprit.
		triggerRerun(t, m, exec, "task", "noise.bin")
		for range loopThreshold {
			triggerRerun(t, m, exec, "task", "loop.bin")
		}
		assertTaskWithin(t, m, time.Second, "task",
			func(info TaskInfo) bool { return info.LoopHalted })

		w := term.NewStringWriter(80, 24)
		require.NoError(t, w.Clear(term.Attributes{}))
		task.currentHandler().Draw(w)
		require.NoError(t, w.Flush())
		rendered := w.String()
		assert.Contains(t, rendered, "loop.bin", "must name the culprit file")
		assert.NotContains(t, rendered, "noise.bin", "must not blame the noise file")
	})

	t.Run("events that do not start a run are not counted", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		// Leave the task running so every event hits the t.running guard in
		// tryRunning and returns false: these must NOT count toward the loop.
		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "build"}))
		assertTaskWithin(t, m, time.Second, "task",
			func(info TaskInfo) bool { return info.Running })

		for range loopThreshold * 3 {
			sendEvent(t, m, exec, "task", "out.bin")
		}
		time.Sleep(200 * time.Millisecond)
		assert.False(t, mustTaskInfo(t, m, "task").LoopHalted,
			"events suppressed by the running guard must not trip the loop detector")
	})

	t.Run("halted task ignores further events", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		for range loopThreshold {
			triggerRerun(t, m, exec, "task", "out.bin")
		}
		assertTaskWithin(t, m, time.Second, "task",
			func(info TaskInfo) bool { return info.LoopHalted })

		runs := mustTaskInfo(t, m, "task").Runs
		// The watch is stopped; sending more events must be inert. The
		// channel may already be gone, so do a best-effort non-blocking send.
		assert.Eventually(t, func() bool { return watchStopped(m, exec, "task") },
			time.Second, 10*time.Millisecond)
		time.Sleep(100 * time.Millisecond)
		assert.Equal(t, runs, mustTaskInfo(t, m, "task").Runs,
			"a halted task must not run again")
	})
}

// triggerRerunInfo behaves like triggerRerun but sends a fully-specified
// event (event type, dir flag) instead of a bare Create on a file.
func triggerRerunInfo(t *testing.T, m *Manager, exec *fakeScheme, name string, ev testEventInfo) {
	t.Helper()
	sendEventInfo(t, m, exec, name, ev)
	assertTaskWithin(t, m, time.Second, name,
		func(info TaskInfo) bool { return info.Running })
	assertTaskRunsWithin(t, m, time.Second, name, nil,
		func(info TaskInfo) bool { return !info.Running })
}

// TestManagerEventHandling exercises event-type and filter-matching
// behavior that the default Create-on-file event could not reach.
func TestManagerEventHandling(t *testing.T) {
	eventTypes := []struct {
		name string
		ev   schemeapi.Event
	}{
		{"create", schemeapi.Create},
		{"write", schemeapi.Write},
		{"rename", schemeapi.Rename},
		{"remove", schemeapi.Remove},
	}
	for _, tc := range eventTypes {
		t.Run("event type "+tc.name+" reruns an idle task", func(t *testing.T) {
			wm := newFakeBrowser()
			exec := newFakeScheme()
			m := newTestManager(wm, exec)
			runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

			triggerRerunInfo(t, m, exec, "task",
				testEventInfo{uri: mustURI(t, "x.txt"), ev: tc.ev})
			assert.Equal(t, 2, mustTaskInfo(t, m, "task").Runs,
				"%s event should rerun the task", tc.name)
		})
	}

	t.Run("directory event matching filter reruns the task", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		triggerRerunInfo(t, m, exec, "task",
			testEventInfo{uri: mustURI(t, "subdir"), ev: schemeapi.Create, dir: true})
		assert.Equal(t, 2, mustTaskInfo(t, m, "task").Runs)
	})

	filterCases := []struct {
		name    string
		filter  string
		file    string
		matches bool
	}{
		{"single glob match", "*.go", "main.go", true},
		{"single glob no match", "*.go", "main.rs", false},
		{"multi glob first", "*.go,*.md", "readme.md", true},
		{"multi glob none", "*.go,*.md", "main.rs", false},
		{"nested path glob", "src/*.go", "src/main.go", true},
		{"nested path glob miss", "src/*.go", "lib/main.go", false},
		{"double-star glob", "src/**", "src/a/b/c.go", true},
		{"leading whitespace segment", " *.go", "main.go", true},
		{"trailing whitespace segment", "*.go ", "main.go", true},
		{"empty middle segment", "*.go,,*.md", "readme.md", true},
		{"trailing comma", "*.md,", "readme.md", true},
		{"leading comma", ",*.md", "readme.md", true},
	}
	for _, tc := range filterCases {
		t.Run("filter "+tc.name, func(t *testing.T) {
			wm := newFakeBrowser()
			exec := newFakeScheme()
			m := newTestManager(wm, exec)
			runAndSettle(t, m, Task{Name: "task", Cmd: "build", Filter: tc.filter})

			runsBefore := mustTaskInfo(t, m, "task").Runs
			if tc.matches {
				triggerRerun(t, m, exec, "task", tc.file)
				assert.Equal(t, runsBefore+1, mustTaskInfo(t, m, "task").Runs,
					"filter %q should match %q", tc.filter, tc.file)
			} else {
				sendEvent(t, m, exec, "task", tc.file)
				time.Sleep(150 * time.Millisecond)
				assert.Equal(t, runsBefore, mustTaskInfo(t, m, "task").Runs,
					"filter %q should not match %q", tc.filter, tc.file)
			}
		})
	}

	t.Run("filename with spaces matching filter reruns", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build", Filter: "*.txt"})

		triggerRerun(t, m, exec, "task", "my notes.txt")
		assert.Equal(t, 2, mustTaskInfo(t, m, "task").Runs)
	})

	t.Run("unicode filename matching filter reruns", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build", Filter: "*.go"})

		triggerRerun(t, m, exec, "task", "café.go")
		assert.Equal(t, 2, mustTaskInfo(t, m, "task").Runs)
	})
}

// TestManagerLifecycleEdgeCases probes construction validation, idempotent
// teardown, and the pause/restart interactions around a watched task.
func TestManagerLifecycleEdgeCases(t *testing.T) {
	t.Run("RunTask with empty command panics", func(t *testing.T) {
		m := newTestManager(newFakeBrowser(), newFakeScheme())
		assert.Panics(t, func() {
			_ = m.RunTask(Task{Name: "x", Cmd: ""})
		})
	})

	t.Run("RunTask with empty name panics", func(t *testing.T) {
		m := newTestManager(newFakeBrowser(), newFakeScheme())
		assert.Panics(t, func() {
			_ = m.RunTask(Task{Name: "", Cmd: "run"})
		})
	})

	t.Run("Resize after close is a no-op and does not panic", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "build"}))
		taskIfc, _ := m.tasks.Load("task")
		task := taskIfc.(*Task)
		require.NoError(t, m.StopTask("task"))

		assert.NotPanics(t, func() { task.Resize(40, 12) })
	})

	t.Run("FocusTask on unknown task returns false", func(t *testing.T) {
		m := newTestManager(newFakeBrowser(), newFakeScheme())
		assert.False(t, m.FocusTask("nope"))
	})

	t.Run("ListTasks reflects running and finished tasks", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		require.NoError(t, m.RunTask(Task{Name: "a", Cmd: "run"}))
		runAndSettle(t, m, Task{Name: "b", Cmd: "run"})

		infos := m.ListTasks()
		require.Len(t, infos, 2)
		byName := map[string]TaskInfo{}
		for _, i := range infos {
			byName[i.Name] = i
		}
		assert.True(t, byName["a"].Running)
		assert.False(t, byName["b"].Running)
	})

	t.Run("ctrl-c pause then file change still reruns the task", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		taskIfc, _ := m.tasks.Load("task")
		task := taskIfc.(*Task)
		task.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'c'})

		runsBefore := mustTaskInfo(t, m, "task").Runs
		triggerRerun(t, m, exec, "task", "main.go")
		assert.Equal(t, runsBefore+1, mustTaskInfo(t, m, "task").Runs,
			"a paused-but-idle task should still rerun on file changes")
	})

	t.Run("loop detector resets after the task is stopped and recreated", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		// Almost trip the loop, then stop+recreate. The new task's loop
		// window must start fresh, not inherit prior timestamps.
		for range loopThreshold - 1 {
			triggerRerun(t, m, exec, "task", "out.bin")
		}
		require.NoError(t, m.StopTask("task"))
		runAndSettle(t, m, Task{Name: "task", Cmd: "build"})

		// One more event on the same file must not immediately halt.
		triggerRerun(t, m, exec, "task", "out.bin")
		assert.False(t, mustTaskInfo(t, m, "task").LoopHalted,
			"recreated task must not inherit the prior loop window")
	})

	t.Run("concurrent StopTask calls do not double-close or panic", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)
		require.NoError(t, m.RunTask(Task{Name: "task", Cmd: "build"}))

		var wg sync.WaitGroup
		var errs [4]error
		for i := range errs {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				errs[i] = m.StopTask("task")
			}(i)
		}
		wg.Wait()

		okCount := 0
		for _, e := range errs {
			if e == nil {
				okCount++
			}
		}
		assert.Equal(t, 1, okCount, "exactly one concurrent StopTask should succeed")
	})

	t.Run("Close stops all tasks and their watches", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		for i := range 5 {
			require.NoError(t, m.RunTask(Task{Name: fmt.Sprintf("t%d", i), Cmd: "run"}))
		}
		require.NoError(t, m.Close())

		assert.Eventually(t, func() bool {
			exec.mu.Lock()
			defer exec.mu.Unlock()
			return len(exec.tasks) == 0
		}, time.Second, 10*time.Millisecond,
			"Close should stop every task watch")
		assert.Empty(t, m.ListTasks())
	})

	t.Run("SetMaxWidthHeight concurrent with task runs does not race or panic", func(t *testing.T) {
		wm := newFakeBrowser()
		exec := newFakeScheme()
		m := newTestManager(wm, exec)

		for i := range 4 {
			require.NoError(t, m.RunTask(Task{Name: fmt.Sprintf("t%d", i), Cmd: "run"}))
		}

		// Models the real production interaction: a single resize caller
		// (the event loop) ranges over tasks calling setMaxWidthHeight while
		// watcher goroutines drive tryRunning on those same tasks.
		var stop atomic.Bool
		var wg sync.WaitGroup
		wg.Go(func() {
			for j := 0; !stop.Load(); j++ {
				m.SetMaxWidthHeight(80+j%40, 24+j%20)
			}
		})
		for i := range 4 {
			name := fmt.Sprintf("t%d", i)
			settleRun(t, m, name)
			for range 5 {
				triggerRerun(t, m, exec, name, fmt.Sprintf("%s.go", name))
			}
		}
		stop.Store(true)
		wg.Wait()

		assert.Len(t, m.ListTasks(), 4)
	})
}
