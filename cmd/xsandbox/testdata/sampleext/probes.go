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

package main

// This file exhaustively exercises every method of every interface a
// Rune workspace extension can reach through extensionapi.Workspace.
// Each probe drives ALL methods of one interface so the sandbox
// records the full per-method RPC surface, not just one representative
// call per service. The sandbox stubs answer every method with an
// empty default, so the calls succeed headlessly; a probe only returns
// an error when a mandatory setup step (needed to reach the other
// methods) fails.

import (
	"context"
	"os"
	"syscall"

	"github.com/google/go-dap"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/debugapi"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// drain consumes an iterator so streaming RPCs run to completion.
func drain[T any](ctx context.Context, it iterator.Iterator[T]) {
	for {
		if _, ok := it.Next(ctx); !ok {
			return
		}
	}
}

func (h apiHandler) probeFilesystemAll(ctx context.Context) error {
	fs := h.w.FileSystem(ctx)
	_, _ = fs.URI("VERSION")
	if f, err := fs.OpenFile("VERSION", os.O_RDONLY, 0); err == nil {
		_ = f.Close()
	}
	_ = fs.MkdirAll("probe-dir", 0o755)
	_, _ = fs.Stat("VERSION")
	_, _ = fs.ReadDir(".")
	_ = fs.Remove("probe-dir")
	return nil
}

func (h apiHandler) probeExecutorAll(ctx context.Context) error {
	ex := h.w.Executor(ctx)
	done := make(chan error, 1)
	pid, err := ex.Start(ctx, workspaceapi.Cmd{
		Path:    "/bin/echo",
		Args:    []string{"echo", "probe"},
		Watcher: workspaceapi.ChanProcessWatcher(done),
	})
	if err == nil {
		_ = ex.Signal(pid, syscall.SIGTERM)
	}
	_ = ex.Close()
	select {
	case <-done:
	case <-ctx.Done():
	}
	return nil
}

func (h apiHandler) probeTerminalAll(ctx context.Context) error {
	t := h.w.Terminal(ctx)
	pty, err := t.StartPty()
	if err != nil {
		return err
	}
	_ = t.SetPtySize(pty, 80, 24)
	if pty.Master != nil {
		_ = pty.Master.Close()
	}
	if pty.Slave != nil {
		_ = pty.Slave.Close()
	}
	return nil
}

func (h apiHandler) probeStorageAll(ctx context.Context) error {
	store := h.w.Storage(ctx)
	_ = store.Create(ctx, "probe-create", map[string]any{"n": 1})
	_ = store.Set(ctx, "probe", map[string]any{"n": 7})
	_ = store.Update(ctx, "probe", []storageapi.Update{{
		FieldPath: []string{"n"},
		Value:     8,
	}})
	var out map[string]any
	_ = store.Get(ctx, "probe", &out)
	if it, err := store.List(ctx, nil); err == nil {
		for it.HasNext() {
			var doc map[string]any
			if err := it.NextTo(&doc); err != nil {
				break
			}
		}
		_ = it.Close()
	}
	_ = store.Delete(ctx, "probe")
	if part, err := store.Partition("probe-part"); err == nil {
		_ = part.Close()
	}
	_ = store.Close()
	return nil
}

func (h apiHandler) probeWindowManagerAll(ctx context.Context) error {
	wm := h.w.WindowManager(ctx)
	focused, err := wm.Focus()
	if err != nil {
		return err
	}
	win, err := wm.Split(browserapi.OrientationLeft, focused, newPanel())
	if err == nil {
		_ = wm.SetWindowContent(win, newPanel())
		_ = wm.CloseWindow(win)
	}
	_ = wm.Bar(browserapi.BarConfig{
		Orientation: browserapi.OrientationBottom,
		Size:        1,
	}, newPanel())
	uri, _ := workspaceapi.ParseURI("file://" + h.w.DataDir(ctx) + "/wm-panel")
	_, _ = wm.Tab(uri, '*', "panel", newPanel())
	return nil
}

func (h apiHandler) probeNotificationsAll(ctx context.Context) error {
	n := h.w.Notifications(ctx)
	id, _ := n.Notify(browserapi.LevelInfo, "%s: probe notify", h.prefix)
	_, _ = n.NotifyOnce(browserapi.LevelInfo, "%s: probe once", h.prefix)
	if id != "" {
		_ = n.UpdateNotificationProgress(id, "", 1, 2)
	}
	return nil
}

func (h apiHandler) probeResourceOpenerAll(ctx context.Context) error {
	uri, err := workspaceapi.ParseURI("file://" + h.w.DataDir(ctx) + "/VERSION")
	if err != nil {
		return err
	}
	_, err = h.w.ResourceOpener(ctx).Open(uri)
	return err
}

func (h apiHandler) probeInterrupterAll(ctx context.Context) error {
	return h.w.Interrupter(ctx).Interrupt(ctx)
}

func (h apiHandler) probeConfigAll(ctx context.Context) error {
	cfg := h.w.Config(ctx)
	_, _ = cfg.GetInt("n")
	_, _ = cfg.GetFloat("f")
	_, _ = cfg.GetString("prefix")
	_, _ = cfg.GetBool("b")
	_, _ = cfg.GetConfig("section")
	_, _ = cfg.GetMap("m")
	_, _ = cfg.GetAttribute("attr")
	_, _ = cfg.GetColor("color")
	_, _ = cfg.GetRune("r")
	_, _ = cfg.GetSlice("s")
	cfg.Iterate(func(string, any) {})
	return nil
}

func (h apiHandler) probeParserAll(ctx context.Context) error {
	p := h.w.Parser(ctx)
	uri, _ := workspaceapi.ParseURI("file://" + h.w.DataDir(ctx) + "/VERSION")
	if it, err := p.Search("(x) @y", []string{"y"}); err == nil {
		drain[syntaxapi.Result](ctx, it)
	}
	if it, err := p.SearchNode(syntaxapi.NodeCaptureScope); err == nil {
		drain[syntaxapi.Result](ctx, it)
	}
	if it, err := p.Query(uri, "(x) @y", []string{"y"}); err == nil {
		drain[syntaxapi.Result](ctx, it)
	}
	if it, err := p.QueryNode(uri, syntaxapi.NodeCaptureScope); err == nil {
		drain[syntaxapi.Result](ctx, it)
	}
	if it, err := p.Highlight(uri, "package main"); err == nil {
		drain[textapi.Location](ctx, it)
	}
	if it, err := p.ResolveSymbol(ctx, "pkg.Symbol", nil); err == nil {
		drain[syntaxapi.Match](ctx, it)
	}
	if it, err := p.ListReferencedSymbols(ctx); err == nil {
		drain[string](ctx, it)
	}
	return nil
}

func (h apiHandler) probeLLMAll(ctx context.Context) error {
	l := h.w.LLM(ctx)
	model := llmapi.ModelEntry{Name: "probe-model"}
	if it, err := l.CreateCompletion(ctx, model, llmapi.Request{}); err == nil {
		drain[llmapi.Event](ctx, it)
	}
	_, _ = l.CountTokens(model, nil)
	drain[llmapi.ModelEntry](ctx, l.Models())
	_, _ = l.GetModel(ctx, model)
	return nil
}

func (h apiHandler) probeEditorAll(ctx context.Context) error {
	ed := h.w.Editor(ctx)
	_ = ed.SubscribeEvents([]textapi.EventType{textapi.EventTypeChange},
		editorNopHandler{})
	uri, _ := workspaceapi.ParseURI("file://" + h.w.DataDir(ctx) + "/VERSION")
	handler, err := ed.Editor(uri)
	if err != nil {
		// No resource is open headlessly; the Editor RPC still ran.
		return nil
	}
	_ = ed.SetLocationList(handler, textapi.LocationPriorityInfo, "probe",
		textapi.LocationSlice(nil))
	_ = ed.MoveToNextLocation(handler, "probe")
	_ = ed.MoveToPrevLocation(handler, "probe")
	if _, err := ed.Cursor(handler); err == nil {
		_ = ed.SetCursor(handler, term.Coordinates{})
	}
	if cv := ed.CellView(handler); cv != nil {
		_, _ = cv.RawCells()
	}
	if ce := ed.CellEditor(handler); ce != nil {
		_, _, _, _ = ce.Edit(ctx, term.Coordinates{}, term.Coordinates{}, "")
	}
	_ = ed.SetDefaultAttributes(handler, term.Attributes{})
	return nil
}

// probeLSPAll calls every LSP method with zero-value params. The
// sandbox's stubLSP answers each with an empty default.
func (h apiHandler) probeLSPAll(ctx context.Context) error {
	l := h.w.LSP(ctx)
	_, _ = l.Initialize(ctx, semanticapi.InitializeParams{})
	_ = l.Initialized(ctx)
	_, _ = l.Completion(ctx, semanticapi.CompletionParams{})
	_, _ = l.Hover(ctx, semanticapi.HoverParams{})
	_, _ = l.SignatureHelp(ctx, semanticapi.SignatureHelpParams{})
	_, _ = l.Definition(ctx, semanticapi.DefinitionParams{})
	_, _ = l.Declaration(ctx, semanticapi.DeclarationParams{})
	_, _ = l.TypeDefinition(ctx, semanticapi.TypeDefinitionParams{})
	_, _ = l.Implementation(ctx, semanticapi.ImplementationParams{})
	_, _ = l.References(ctx, semanticapi.ReferenceParams{})
	_, _ = l.DocumentHighlight(ctx, semanticapi.DocumentHighlightParams{})
	_, _ = l.DocumentSymbol(ctx, semanticapi.DocumentSymbolParams{})
	_, _ = l.CodeAction(ctx, semanticapi.CodeActionParams{})
	_, _ = l.CodeLens(ctx, semanticapi.CodeLensParams{})
	_, _ = l.Formatting(ctx, semanticapi.DocumentFormattingParams{})
	_, _ = l.RangeFormatting(ctx, semanticapi.DocumentRangeFormattingParams{})
	_, _ = l.Rename(ctx, semanticapi.RenameParams{})
	_, _ = l.PrepareRename(ctx, semanticapi.PrepareRenameParams{})
	_, _ = l.FoldingRange(ctx, semanticapi.FoldingRangeParams{})
	_, _ = l.SelectionRange(ctx, semanticapi.SelectionRangeParams{})
	_, _ = l.SemanticTokensFull(ctx, semanticapi.SemanticTokensParams{})
	_, _ = l.SemanticTokensRange(ctx, semanticapi.SemanticTokensRangeParams{})
	_, _ = l.Diagnostic(ctx, semanticapi.DocumentDiagnosticParams{})
	_, _ = l.WorkspaceDiagnostic(ctx, semanticapi.WorkspaceDiagnosticParams{})
	_, _ = l.WorkspaceSymbol(ctx, semanticapi.WorkspaceSymbolParams{Query: "probe"})
	_, _ = l.ExecuteCommand(ctx, semanticapi.ExecuteCommandParams{})
	_, _ = l.PrepareCallHierarchy(ctx, semanticapi.CallHierarchyPrepareParams{})
	_, _ = l.CallHierarchyIncomingCalls(ctx, semanticapi.CallHierarchyIncomingCallsParams{})
	_, _ = l.CallHierarchyOutgoingCalls(ctx, semanticapi.CallHierarchyOutgoingCallsParams{})
	_, _ = l.CompletionResolve(ctx, semanticapi.CompletionItem{})
	_, _ = l.CodeLensResolve(ctx, semanticapi.CodeLens{})
	_, _ = l.DocumentColor(ctx, semanticapi.DocumentColorParams{})
	_, _ = l.ColorPresentation(ctx, semanticapi.ColorPresentationParams{})
	_, _ = l.DocumentLink(ctx, semanticapi.DocumentLinkParams{})
	_, _ = l.DocumentLinkResolve(ctx, semanticapi.DocumentLink{})
	_, _ = l.OnTypeFormatting(ctx, semanticapi.DocumentOnTypeFormattingParams{})
	_, _ = l.LinkedEditingRange(ctx, semanticapi.LinkedEditingRangeParams{})
	_, _ = l.Moniker(ctx, semanticapi.MonikerParams{})
	_, _ = l.WillSaveWaitUntil(ctx, semanticapi.WillSaveTextDocumentParams{})
	_, _ = l.SemanticTokensFullDelta(ctx, semanticapi.SemanticTokensDeltaParams{})
	_, _ = l.PrepareTypeHierarchy(ctx, semanticapi.TypeHierarchyPrepareParams{})
	_, _ = l.TypeHierarchySupertypes(ctx, semanticapi.TypeHierarchySupertypesParams{})
	_, _ = l.TypeHierarchySubtypes(ctx, semanticapi.TypeHierarchySubtypesParams{})
	_, _ = l.InlayHint(ctx, semanticapi.InlayHintParams{})
	_, _ = l.InlayHintResolve(ctx, semanticapi.InlayHint{})
	_, _ = l.InlineValue(ctx, semanticapi.InlineValueParams{})
	_, _ = l.WillCreateFiles(ctx, semanticapi.CreateFilesParams{})
	_, _ = l.WillRenameFiles(ctx, semanticapi.RenameFilesParams{})
	_, _ = l.WillDeleteFiles(ctx, semanticapi.DeleteFilesParams{})
	_ = l.WillSave(ctx, semanticapi.WillSaveTextDocumentParams{})
	_ = l.DidChangeConfiguration(ctx, semanticapi.DidChangeConfigurationParams{})
	_ = l.DidChangeWatchedFiles(ctx, semanticapi.DidChangeWatchedFilesParams{})
	_ = l.DidChangeWorkspaceFolders(ctx, semanticapi.DidChangeWorkspaceFoldersParams{})
	_ = l.WorkDoneProgressCancel(ctx, semanticapi.WorkDoneProgressCancelParams{})
	_ = l.SetTrace(ctx, semanticapi.SetTraceParams{})
	_ = l.DidCreateFiles(ctx, semanticapi.CreateFilesParams{})
	_ = l.DidRenameFiles(ctx, semanticapi.RenameFilesParams{})
	_ = l.DidDeleteFiles(ctx, semanticapi.DeleteFilesParams{})
	_ = l.DidOpen(ctx, semanticapi.DidOpenTextDocumentParams{})
	_ = l.DidChange(ctx, semanticapi.DidChangeTextDocumentParams{})
	_ = l.DidSave(ctx, semanticapi.DidSaveTextDocumentParams{})
	_ = l.DidClose(ctx, semanticapi.DidCloseTextDocumentParams{})
	_ = l.Shutdown(ctx)
	_ = l.Exit(ctx)
	return nil
}

// debugNopSubscriber satisfies debugapi.EventSubscriber.
type debugNopSubscriber struct{}

func (debugNopSubscriber) OnEvent(dap.EventMessage) {}
func (debugNopSubscriber) OnClose(string)           {}

// probeDebuggerAll starts a session and drives every DAP method. The
// sandbox's stubDebugger returns a valid session ID and empty results
// for every method, so all of them run headlessly.
func (h apiHandler) probeDebuggerAll(ctx context.Context) error {
	d := h.w.Debugger(ctx)
	sid, _, err := d.CreateSession(ctx, "go",
		debugapi.ClientCapabilities{ClientID: "xsandbox"}, debugNopSubscriber{})
	if err != nil {
		return err
	}
	_ = d.Launch(ctx, sid, debugapi.LaunchRequestArguments{})
	_ = d.Attach(ctx, sid, debugapi.AttachRequestArguments{})
	_ = d.ConfigurationDone(ctx, sid)
	_, _ = d.SetBreakpoints(ctx, sid, &dap.SetBreakpointsArguments{})
	_, _ = d.SetFunctionBreakpoints(ctx, sid, &dap.SetFunctionBreakpointsArguments{})
	_, _ = d.SetExceptionBreakpoints(ctx, sid, &dap.SetExceptionBreakpointsArguments{})
	_, _ = d.Continue(ctx, sid, &dap.ContinueArguments{})
	_ = d.Next(ctx, sid, &dap.NextArguments{})
	_ = d.StepIn(ctx, sid, &dap.StepInArguments{})
	_ = d.StepOut(ctx, sid, &dap.StepOutArguments{})
	_ = d.StepBack(ctx, sid, &dap.StepBackArguments{})
	_ = d.ReverseContinue(ctx, sid, &dap.ReverseContinueArguments{})
	_ = d.Pause(ctx, sid, &dap.PauseArguments{})
	_, _ = d.Threads(ctx, sid)
	_, _ = d.StackTrace(ctx, sid, &dap.StackTraceArguments{})
	_, _ = d.Scopes(ctx, sid, &dap.ScopesArguments{})
	_, _ = d.Variables(ctx, sid, &dap.VariablesArguments{})
	_, _ = d.SetVariable(ctx, sid, &dap.SetVariableArguments{})
	_, _ = d.Source(ctx, sid, &dap.SourceArguments{})
	_, _ = d.Evaluate(ctx, sid, &dap.EvaluateArguments{})
	_, _ = d.SetExpression(ctx, sid, &dap.SetExpressionArguments{})
	_, _ = d.Completions(ctx, sid, &dap.CompletionsArguments{})
	_, _ = d.ExceptionInfo(ctx, sid, &dap.ExceptionInfoArguments{})
	_, _ = d.Modules(ctx, sid, &dap.ModulesArguments{})
	_, _ = d.LoadedSources(ctx, sid)
	_, _ = d.ReadMemory(ctx, sid, &dap.ReadMemoryArguments{})
	_, _ = d.WriteMemory(ctx, sid, &dap.WriteMemoryArguments{})
	_, _ = d.Disassemble(ctx, sid, &dap.DisassembleArguments{})
	_, _ = d.GotoTargets(ctx, sid, &dap.GotoTargetsArguments{})
	_ = d.Goto(ctx, sid, &dap.GotoArguments{})
	_ = d.Restart(ctx, sid)
	_ = d.Terminate(ctx, sid, &dap.TerminateArguments{})
	_ = d.Disconnect(ctx, sid, &dap.DisconnectArguments{})
	return nil
}

// probeAccessors calls the remaining Workspace accessors that don't map
// to a distinct service RPC (Commands returns the command registry,
// RawConn the low-level gRPC connection) so no accessor is left
// untouched.
func (h apiHandler) probeAccessors(ctx context.Context) error {
	_ = h.w.Commands(ctx)
	_ = h.w.RawConn()
	return nil
}

// editorNopHandler satisfies textapi.EventHandler.
type editorNopHandler struct{}

func (editorNopHandler) Handle(context.Context, textapi.Event) bool { return false }
