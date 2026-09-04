// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package sandbox

import (
	"context"

	"github.com/google/go-dap"
	"github.com/unstablebuild/rune-go-sdk/api/debugapi"
)

// stubDebugger answers every DAP request with an empty result so an
// extension exercising the debug service gets well-formed replies by
// default; specs can override individual methods with respond.
type stubDebugger struct{}

var _ debugapi.Debugger = stubDebugger{}

func (stubDebugger) CreateSession(
	_ context.Context, _ string, _ debugapi.ClientCapabilities,
	_ debugapi.EventSubscriber,
) (string, *dap.Capabilities, error) {
	return "xsandbox-session", &dap.Capabilities{}, nil
}

func (stubDebugger) Launch(context.Context, string, debugapi.LaunchRequestArguments) error {
	return nil
}

func (stubDebugger) Attach(context.Context, string, debugapi.AttachRequestArguments) error {
	return nil
}

func (stubDebugger) ConfigurationDone(context.Context, string) error { return nil }

func (stubDebugger) Disconnect(context.Context, string, *dap.DisconnectArguments) error {
	return nil
}

func (stubDebugger) Terminate(context.Context, string, *dap.TerminateArguments) error {
	return nil
}

func (stubDebugger) Restart(context.Context, string) error { return nil }

func (stubDebugger) SetBreakpoints(context.Context, string, *dap.SetBreakpointsArguments) ([]dap.Breakpoint, error) {
	return nil, nil
}

func (stubDebugger) SetFunctionBreakpoints(context.Context, string, *dap.SetFunctionBreakpointsArguments) ([]dap.Breakpoint, error) {
	return nil, nil
}

func (stubDebugger) SetExceptionBreakpoints(context.Context, string, *dap.SetExceptionBreakpointsArguments) ([]dap.Breakpoint, error) {
	return nil, nil
}

func (stubDebugger) Continue(context.Context, string, *dap.ContinueArguments) (*dap.ContinueResponseBody, error) {
	return &dap.ContinueResponseBody{}, nil
}

func (stubDebugger) Next(context.Context, string, *dap.NextArguments) error { return nil }

func (stubDebugger) StepIn(context.Context, string, *dap.StepInArguments) error { return nil }

func (stubDebugger) StepOut(context.Context, string, *dap.StepOutArguments) error { return nil }

func (stubDebugger) StepBack(context.Context, string, *dap.StepBackArguments) error { return nil }

func (stubDebugger) ReverseContinue(context.Context, string, *dap.ReverseContinueArguments) error {
	return nil
}

func (stubDebugger) Pause(context.Context, string, *dap.PauseArguments) error { return nil }

func (stubDebugger) Threads(context.Context, string) ([]dap.Thread, error) { return nil, nil }

func (stubDebugger) StackTrace(context.Context, string, *dap.StackTraceArguments) (*dap.StackTraceResponseBody, error) {
	return &dap.StackTraceResponseBody{}, nil
}

func (stubDebugger) Scopes(context.Context, string, *dap.ScopesArguments) ([]dap.Scope, error) {
	return nil, nil
}

func (stubDebugger) Variables(context.Context, string, *dap.VariablesArguments) ([]dap.Variable, error) {
	return nil, nil
}

func (stubDebugger) SetVariable(context.Context, string, *dap.SetVariableArguments) (*dap.SetVariableResponseBody, error) {
	return &dap.SetVariableResponseBody{}, nil
}

func (stubDebugger) Source(context.Context, string, *dap.SourceArguments) (*dap.SourceResponseBody, error) {
	return &dap.SourceResponseBody{}, nil
}

func (stubDebugger) Evaluate(context.Context, string, *dap.EvaluateArguments) (*dap.EvaluateResponseBody, error) {
	return &dap.EvaluateResponseBody{}, nil
}

func (stubDebugger) SetExpression(context.Context, string, *dap.SetExpressionArguments) (*dap.SetExpressionResponseBody, error) {
	return &dap.SetExpressionResponseBody{}, nil
}

func (stubDebugger) Completions(context.Context, string, *dap.CompletionsArguments) ([]dap.CompletionItem, error) {
	return nil, nil
}

func (stubDebugger) ExceptionInfo(context.Context, string, *dap.ExceptionInfoArguments) (*dap.ExceptionInfoResponseBody, error) {
	return &dap.ExceptionInfoResponseBody{}, nil
}

func (stubDebugger) Modules(context.Context, string, *dap.ModulesArguments) (*dap.ModulesResponseBody, error) {
	return &dap.ModulesResponseBody{}, nil
}

func (stubDebugger) LoadedSources(context.Context, string) ([]dap.Source, error) {
	return nil, nil
}

func (stubDebugger) ReadMemory(context.Context, string, *dap.ReadMemoryArguments) (*dap.ReadMemoryResponseBody, error) {
	return &dap.ReadMemoryResponseBody{}, nil
}

func (stubDebugger) WriteMemory(context.Context, string, *dap.WriteMemoryArguments) (*dap.WriteMemoryResponseBody, error) {
	return &dap.WriteMemoryResponseBody{}, nil
}

func (stubDebugger) Disassemble(context.Context, string, *dap.DisassembleArguments) ([]dap.DisassembledInstruction, error) {
	return nil, nil
}

func (stubDebugger) GotoTargets(context.Context, string, *dap.GotoTargetsArguments) ([]dap.GotoTarget, error) {
	return nil, nil
}

func (stubDebugger) Goto(context.Context, string, *dap.GotoArguments) error { return nil }
