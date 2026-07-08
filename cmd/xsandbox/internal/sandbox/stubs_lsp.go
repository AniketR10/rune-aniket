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

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

// stubLSP answers every LSP request with an empty result so an
// extension exercising the semantic service gets well-formed replies
// by default; specs can override individual methods with respond.
type stubLSP struct{}

var _ semanticapi.LSP = stubLSP{}

func (stubLSP) Initialize(context.Context, semanticapi.InitializeParams) (semanticapi.InitializeResult, error) {
	return semanticapi.InitializeResult{}, nil
}
func (stubLSP) Initialized(context.Context) error { return nil }
func (stubLSP) Shutdown(context.Context) error    { return nil }
func (stubLSP) Exit(context.Context) error        { return nil }

func (stubLSP) DidOpen(context.Context, semanticapi.DidOpenTextDocumentParams) error     { return nil }
func (stubLSP) DidChange(context.Context, semanticapi.DidChangeTextDocumentParams) error { return nil }
func (stubLSP) DidClose(context.Context, semanticapi.DidCloseTextDocumentParams) error   { return nil }
func (stubLSP) DidSave(context.Context, semanticapi.DidSaveTextDocumentParams) error     { return nil }

func (stubLSP) Completion(context.Context, semanticapi.CompletionParams) (semanticapi.CompletionResult, error) {
	return semanticapi.CompletionResult{}, nil
}
func (stubLSP) Hover(context.Context, semanticapi.HoverParams) (*semanticapi.Hover, error) {
	return nil, nil
}
func (stubLSP) SignatureHelp(context.Context, semanticapi.SignatureHelpParams) (*semanticapi.SignatureHelp, error) {
	return nil, nil
}
func (stubLSP) Definition(context.Context, semanticapi.DefinitionParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (stubLSP) Declaration(context.Context, semanticapi.DeclarationParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (stubLSP) TypeDefinition(context.Context, semanticapi.TypeDefinitionParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (stubLSP) Implementation(context.Context, semanticapi.ImplementationParams) (semanticapi.LocationResult, error) {
	return semanticapi.LocationResult{}, nil
}
func (stubLSP) References(context.Context, semanticapi.ReferenceParams) ([]semanticapi.Location, error) {
	return nil, nil
}
func (stubLSP) DocumentHighlight(context.Context, semanticapi.DocumentHighlightParams) ([]semanticapi.DocumentHighlight, error) {
	return nil, nil
}
func (stubLSP) DocumentSymbol(context.Context, semanticapi.DocumentSymbolParams) (semanticapi.DocumentSymbolResult, error) {
	return semanticapi.DocumentSymbolResult{}, nil
}
func (stubLSP) CodeAction(context.Context, semanticapi.CodeActionParams) ([]semanticapi.CodeActionResult, error) {
	return nil, nil
}
func (stubLSP) CodeLens(context.Context, semanticapi.CodeLensParams) ([]semanticapi.CodeLens, error) {
	return nil, nil
}
func (stubLSP) Formatting(context.Context, semanticapi.DocumentFormattingParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (stubLSP) RangeFormatting(context.Context, semanticapi.DocumentRangeFormattingParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (stubLSP) Rename(context.Context, semanticapi.RenameParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (stubLSP) PrepareRename(context.Context, semanticapi.PrepareRenameParams) (*semanticapi.PrepareRenameResult, error) {
	return nil, nil
}
func (stubLSP) FoldingRange(context.Context, semanticapi.FoldingRangeParams) ([]semanticapi.FoldingRange, error) {
	return nil, nil
}
func (stubLSP) SelectionRange(context.Context, semanticapi.SelectionRangeParams) ([]semanticapi.SelectionRange, error) {
	return nil, nil
}

func (stubLSP) SemanticTokensFull(context.Context, semanticapi.SemanticTokensParams) (*semanticapi.SemanticTokens, error) {
	return nil, nil
}
func (stubLSP) SemanticTokensRange(context.Context, semanticapi.SemanticTokensRangeParams) (*semanticapi.SemanticTokens, error) {
	return nil, nil
}

func (stubLSP) Diagnostic(context.Context, semanticapi.DocumentDiagnosticParams) (semanticapi.DocumentDiagnosticReport, error) {
	return semanticapi.DocumentDiagnosticReport{}, nil
}
func (stubLSP) WorkspaceDiagnostic(context.Context, semanticapi.WorkspaceDiagnosticParams) (semanticapi.WorkspaceDiagnosticReport, error) {
	return semanticapi.WorkspaceDiagnosticReport{}, nil
}

func (stubLSP) WorkspaceSymbol(context.Context, semanticapi.WorkspaceSymbolParams) ([]semanticapi.SymbolInformation, error) {
	return nil, nil
}
func (stubLSP) ExecuteCommand(context.Context, semanticapi.ExecuteCommandParams) (string, error) {
	return "", nil
}

func (stubLSP) PrepareCallHierarchy(context.Context, semanticapi.CallHierarchyPrepareParams) ([]semanticapi.CallHierarchyItem, error) {
	return nil, nil
}
func (stubLSP) CallHierarchyIncomingCalls(context.Context, semanticapi.CallHierarchyIncomingCallsParams) ([]semanticapi.CallHierarchyIncomingCall, error) {
	return nil, nil
}
func (stubLSP) CallHierarchyOutgoingCalls(context.Context, semanticapi.CallHierarchyOutgoingCallsParams) ([]semanticapi.CallHierarchyOutgoingCall, error) {
	return nil, nil
}

func (stubLSP) CompletionResolve(_ context.Context, item semanticapi.CompletionItem) (semanticapi.CompletionItem, error) {
	return item, nil
}
func (stubLSP) CodeLensResolve(_ context.Context, lens semanticapi.CodeLens) (semanticapi.CodeLens, error) {
	return lens, nil
}
func (stubLSP) DocumentColor(context.Context, semanticapi.DocumentColorParams) ([]semanticapi.ColorInformation, error) {
	return nil, nil
}
func (stubLSP) ColorPresentation(context.Context, semanticapi.ColorPresentationParams) ([]semanticapi.ColorPresentation, error) {
	return nil, nil
}
func (stubLSP) DocumentLink(context.Context, semanticapi.DocumentLinkParams) ([]semanticapi.DocumentLink, error) {
	return nil, nil
}
func (stubLSP) DocumentLinkResolve(_ context.Context, link semanticapi.DocumentLink) (semanticapi.DocumentLink, error) {
	return link, nil
}
func (stubLSP) OnTypeFormatting(context.Context, semanticapi.DocumentOnTypeFormattingParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (stubLSP) LinkedEditingRange(context.Context, semanticapi.LinkedEditingRangeParams) (*semanticapi.LinkedEditingRanges, error) {
	return nil, nil
}
func (stubLSP) Moniker(context.Context, semanticapi.MonikerParams) ([]semanticapi.Moniker, error) {
	return nil, nil
}
func (stubLSP) WillSaveWaitUntil(context.Context, semanticapi.WillSaveTextDocumentParams) ([]semanticapi.TextEdit, error) {
	return nil, nil
}
func (stubLSP) SemanticTokensFullDelta(context.Context, semanticapi.SemanticTokensDeltaParams) (*semanticapi.SemanticTokensDelta, error) {
	return nil, nil
}

func (stubLSP) PrepareTypeHierarchy(context.Context, semanticapi.TypeHierarchyPrepareParams) ([]semanticapi.TypeHierarchyItem, error) {
	return nil, nil
}
func (stubLSP) TypeHierarchySupertypes(context.Context, semanticapi.TypeHierarchySupertypesParams) ([]semanticapi.TypeHierarchyItem, error) {
	return nil, nil
}
func (stubLSP) TypeHierarchySubtypes(context.Context, semanticapi.TypeHierarchySubtypesParams) ([]semanticapi.TypeHierarchyItem, error) {
	return nil, nil
}

func (stubLSP) InlayHint(context.Context, semanticapi.InlayHintParams) ([]semanticapi.InlayHint, error) {
	return nil, nil
}
func (stubLSP) InlayHintResolve(_ context.Context, hint semanticapi.InlayHint) (semanticapi.InlayHint, error) {
	return hint, nil
}
func (stubLSP) InlineValue(context.Context, semanticapi.InlineValueParams) ([]semanticapi.InlineValue, error) {
	return nil, nil
}

func (stubLSP) WillCreateFiles(context.Context, semanticapi.CreateFilesParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (stubLSP) WillRenameFiles(context.Context, semanticapi.RenameFilesParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}
func (stubLSP) WillDeleteFiles(context.Context, semanticapi.DeleteFilesParams) (*semanticapi.WorkspaceEdit, error) {
	return nil, nil
}

func (stubLSP) WillSave(context.Context, semanticapi.WillSaveTextDocumentParams) error { return nil }
func (stubLSP) DidChangeConfiguration(context.Context, semanticapi.DidChangeConfigurationParams) error {
	return nil
}
func (stubLSP) DidChangeWatchedFiles(context.Context, semanticapi.DidChangeWatchedFilesParams) error {
	return nil
}
func (stubLSP) DidChangeWorkspaceFolders(context.Context, semanticapi.DidChangeWorkspaceFoldersParams) error {
	return nil
}
func (stubLSP) WorkDoneProgressCancel(context.Context, semanticapi.WorkDoneProgressCancelParams) error {
	return nil
}
func (stubLSP) SetTrace(context.Context, semanticapi.SetTraceParams) error          { return nil }
func (stubLSP) DidCreateFiles(context.Context, semanticapi.CreateFilesParams) error { return nil }
func (stubLSP) DidRenameFiles(context.Context, semanticapi.RenameFilesParams) error { return nil }
func (stubLSP) DidDeleteFiles(context.Context, semanticapi.DeleteFilesParams) error { return nil }
