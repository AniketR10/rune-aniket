package main

import (
	"context"
	"sync"

	"github.com/ernestrc/golang-internal-tools/lsp/protocol"
)

type ioUnlockServer struct {
	mu     *sync.Mutex
	server protocol.Server
}

func (s ioUnlockServer) DidChangeWorkspaceFolders(
	ctx context.Context, p *protocol.DidChangeWorkspaceFoldersParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DidChangeWorkspaceFolders(ctx, p)
}
func (s ioUnlockServer) WorkDoneProgressCancel(
	ctx context.Context, p *protocol.WorkDoneProgressCancelParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.WorkDoneProgressCancel(ctx, p)
}
func (s ioUnlockServer) Initialized(
	ctx context.Context, p *protocol.InitializedParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Initialized(ctx, p)
}
func (s ioUnlockServer) Exit(
	ctx context.Context,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Exit(ctx)
}
func (s ioUnlockServer) DidChangeConfiguration(
	ctx context.Context, p *protocol.DidChangeConfigurationParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DidChangeConfiguration(ctx, p)
}
func (s ioUnlockServer) DidOpen(
	ctx context.Context, p *protocol.DidOpenTextDocumentParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DidOpen(ctx, p)
}
func (s ioUnlockServer) DidChange(
	ctx context.Context, p *protocol.DidChangeTextDocumentParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DidChange(ctx, p)
}
func (s ioUnlockServer) DidClose(
	ctx context.Context, p *protocol.DidCloseTextDocumentParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DidClose(ctx, p)
}
func (s ioUnlockServer) DidSave(
	ctx context.Context, p *protocol.DidSaveTextDocumentParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DidSave(ctx, p)
}
func (s ioUnlockServer) WillSave(
	ctx context.Context, p *protocol.WillSaveTextDocumentParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.WillSave(ctx, p)
}
func (s ioUnlockServer) DidChangeWatchedFiles(
	ctx context.Context, p *protocol.DidChangeWatchedFilesParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DidChangeWatchedFiles(ctx, p)
}
func (s ioUnlockServer) SetTrace(
	ctx context.Context, p *protocol.SetTraceParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.SetTrace(ctx, p)
}
func (s ioUnlockServer) LogTrace(
	ctx context.Context, p *protocol.LogTraceParams,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.LogTrace(ctx, p)
}
func (s ioUnlockServer) Implementation(
	ctx context.Context, p *protocol.ImplementationParams,
) (protocol.Definition /*Definition | DefinitionLink[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Implementation(ctx, p)
}
func (s ioUnlockServer) TypeDefinition(
	ctx context.Context, p *protocol.TypeDefinitionParams,
) (protocol.Definition /*Definition | DefinitionLink[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.TypeDefinition(ctx, p)
}
func (s ioUnlockServer) DocumentColor(
	ctx context.Context, p *protocol.DocumentColorParams,
) ([]protocol.ColorInformation, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DocumentColor(ctx, p)
}
func (s ioUnlockServer) ColorPresentation(
	ctx context.Context, p *protocol.ColorPresentationParams,
) ([]protocol.ColorPresentation, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.ColorPresentation(ctx, p)
}
func (s ioUnlockServer) FoldingRange(
	ctx context.Context, p *protocol.FoldingRangeParams,
) ([]protocol.FoldingRange /*FoldingRange[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.FoldingRange(ctx, p)
}
func (s ioUnlockServer) Declaration(
	ctx context.Context, p *protocol.DeclarationParams,
) (protocol.Declaration /*Declaration | DeclarationLink[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Declaration(ctx, p)
}
func (s ioUnlockServer) SelectionRange(
	ctx context.Context, p *protocol.SelectionRangeParams,
) ([]protocol.SelectionRange /*SelectionRange[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.SelectionRange(ctx, p)
}
func (s ioUnlockServer) PrepareCallHierarchy(
	ctx context.Context, p *protocol.CallHierarchyPrepareParams,
) ([]protocol.CallHierarchyItem /*CallHierarchyItem[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.PrepareCallHierarchy(ctx, p)
}
func (s ioUnlockServer) IncomingCalls(
	ctx context.Context, p *protocol.CallHierarchyIncomingCallsParams,
) ([]protocol.CallHierarchyIncomingCall /*CallHierarchyIncomingCall[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.IncomingCalls(ctx, p)
}
func (s ioUnlockServer) OutgoingCalls(
	ctx context.Context, p *protocol.CallHierarchyOutgoingCallsParams,
) ([]protocol.CallHierarchyOutgoingCall /*CallHierarchyOutgoingCall[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.OutgoingCalls(ctx, p)
}
func (s ioUnlockServer) SemanticTokensFull(
	ctx context.Context, p *protocol.SemanticTokensParams,
) (*protocol.SemanticTokens /*SemanticTokens | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.SemanticTokensFull(ctx, p)
}
func (s ioUnlockServer) SemanticTokensFullDelta(
	ctx context.Context, p *protocol.SemanticTokensDeltaParams,
) (interface{} /* SemanticTokens | SemanticTokensDelta | nil*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.SemanticTokensFullDelta(ctx, p)
}
func (s ioUnlockServer) SemanticTokensRange(
	ctx context.Context, p *protocol.SemanticTokensRangeParams,
) (*protocol.SemanticTokens /*SemanticTokens | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.SemanticTokensRange(ctx, p)
}
func (s ioUnlockServer) SemanticTokensRefresh(
	ctx context.Context,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.SemanticTokensRefresh(ctx)
}
func (s ioUnlockServer) Initialize(
	ctx context.Context, p *protocol.ParamInitialize,
) (*protocol.InitializeResult, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Initialize(ctx, p)
}
func (s ioUnlockServer) Shutdown(
	ctx context.Context,
) error {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Shutdown(ctx)
}
func (s ioUnlockServer) WillSaveWaitUntil(
	ctx context.Context, p *protocol.WillSaveTextDocumentParams,
) ([]protocol.TextEdit /*TextEdit[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.WillSaveWaitUntil(ctx, p)
}
func (s ioUnlockServer) Completion(
	ctx context.Context, p *protocol.CompletionParams,
) (*protocol.CompletionList /*CompletionItem[] | CompletionList | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Completion(ctx, p)
}
func (s ioUnlockServer) Resolve(
	ctx context.Context, p *protocol.CompletionItem,
) (*protocol.CompletionItem, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Resolve(ctx, p)
}
func (s ioUnlockServer) Hover(
	ctx context.Context, p *protocol.HoverParams,
) (*protocol.Hover /*Hover | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Hover(ctx, p)
}
func (s ioUnlockServer) SignatureHelp(
	ctx context.Context, p *protocol.SignatureHelpParams,
) (*protocol.SignatureHelp /*SignatureHelp | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.SignatureHelp(ctx, p)
}
func (s ioUnlockServer) Definition(
	ctx context.Context, p *protocol.DefinitionParams,
) (protocol.Definition /*Definition | DefinitionLink[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Definition(ctx, p)
}
func (s ioUnlockServer) References(
	ctx context.Context, p *protocol.ReferenceParams,
) ([]protocol.Location /*Location[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.References(ctx, p)
}
func (s ioUnlockServer) DocumentHighlight(
	ctx context.Context, p *protocol.DocumentHighlightParams,
) ([]protocol.DocumentHighlight /*DocumentHighlight[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DocumentHighlight(ctx, p)
}
func (s ioUnlockServer) DocumentSymbol(
	ctx context.Context, p *protocol.DocumentSymbolParams,
) ([]interface{} /*SymbolInformation[] | DocumentSymbol[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DocumentSymbol(ctx, p)
}
func (s ioUnlockServer) CodeAction(
	ctx context.Context, p *protocol.CodeActionParams,
) ([]protocol.CodeAction /*(Command | CodeAction)[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.CodeAction(ctx, p)
}
func (s ioUnlockServer) ResolveCodeAction(
	ctx context.Context, p *protocol.CodeAction,
) (*protocol.CodeAction, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.ResolveCodeAction(ctx, p)
}
func (s ioUnlockServer) Symbol(
	ctx context.Context, p *protocol.WorkspaceSymbolParams,
) ([]protocol.SymbolInformation /*SymbolInformation[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Symbol(ctx, p)
}
func (s ioUnlockServer) CodeLens(
	ctx context.Context, p *protocol.CodeLensParams,
) ([]protocol.CodeLens /*CodeLens[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.CodeLens(ctx, p)
}
func (s ioUnlockServer) ResolveCodeLens(
	ctx context.Context, p *protocol.CodeLens,
) (*protocol.CodeLens, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.ResolveCodeLens(ctx, p)
}
func (s ioUnlockServer) DocumentLink(
	ctx context.Context, p *protocol.DocumentLinkParams,
) ([]protocol.DocumentLink /*DocumentLink[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.DocumentLink(ctx, p)
}
func (s ioUnlockServer) ResolveDocumentLink(
	ctx context.Context, p *protocol.DocumentLink,
) (*protocol.DocumentLink, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.ResolveDocumentLink(ctx, p)
}
func (s ioUnlockServer) Formatting(
	ctx context.Context, p *protocol.DocumentFormattingParams,
) ([]protocol.TextEdit /*TextEdit[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Formatting(ctx, p)
}
func (s ioUnlockServer) RangeFormatting(
	ctx context.Context, p *protocol.DocumentRangeFormattingParams,
) ([]protocol.TextEdit /*TextEdit[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.RangeFormatting(ctx, p)
}
func (s ioUnlockServer) OnTypeFormatting(
	ctx context.Context, p *protocol.DocumentOnTypeFormattingParams,
) ([]protocol.TextEdit /*TextEdit[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.OnTypeFormatting(ctx, p)
}
func (s ioUnlockServer) Rename(
	ctx context.Context, p *protocol.RenameParams,
) (*protocol.WorkspaceEdit /*WorkspaceEdit | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Rename(ctx, p)
}
func (s ioUnlockServer) PrepareRename(
	ctx context.Context, p *protocol.PrepareRenameParams,
) (
	/*Range | { range: Range, placeholder: string } | { defaultBehavior: boolean } | null*/
	*protocol.Range,
	error,
) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.PrepareRename(ctx, p)
}
func (s ioUnlockServer) ExecuteCommand(
	ctx context.Context, p *protocol.ExecuteCommandParams,
) (interface{} /*any | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.ExecuteCommand(ctx, p)
}
func (s ioUnlockServer) Moniker(
	ctx context.Context, p *protocol.MonikerParams,
) ([]protocol.Moniker /*Moniker[] | null*/, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.Moniker(ctx, p)
}
func (s ioUnlockServer) NonstandardRequest(
	ctx context.Context, method string, params interface{},
) (interface{}, error) {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.server.NonstandardRequest(ctx, method, params)
}
