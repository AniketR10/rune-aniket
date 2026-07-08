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
