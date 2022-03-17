package main

import (
	"context"

	"github.com/ernestrc/golang-internal-tools/lsp/protocol"
	log "github.com/sirupsen/logrus"
)

type lspClientHandler struct {
	h *lspEditorHandler
}

// lsp protocol.Client
func (h *lspClientHandler) ShowMessage(
	ctx context.Context, p *protocol.ShowMessageParams,
) error {
	log.Tracef("lspClientHandler.ShowMessage: %#v", p)
	return nil
}

func (h *lspClientHandler) LogMessage(
	ctx context.Context, p *protocol.LogMessageParams,
) error {
	switch p.Type {
	case protocol.Error:
		log.Error("protocol.Client:", p.Message)
	case protocol.Warning:
		log.Warn("protocol.Client: ", p.Message)
	case protocol.Info:
		log.Info("protocol.Client: ", p.Message)
	case protocol.Log:
		log.Trace("protocol.Client: ", p.Message)
	default:
		log.Trace("protocol.Client: ", p.Message)
	}
	return nil
}

func (h *lspClientHandler) Event(
	ctx context.Context, ev *interface{},
) error {
	log.Tracef("lspClientHandler.Event: %#v", ev)
	return nil
}

func (h *lspClientHandler) PublishDiagnostics(
	ctx context.Context, p *protocol.PublishDiagnosticsParams,
) error {
	log.Tracef("lspClientHandler.PublishDiagnostics: %#v", p)
	h.h.HandleDiagnostics(ctx, p)
	return nil
}

func (h *lspClientHandler) Progress(
	ctx context.Context, p *protocol.ProgressParams,
) error {
	log.Tracef("lspClientHandler.Progress: %#v", p)
	return nil
}

func (h *lspClientHandler) WorkspaceFolders(
	context.Context,
) ([]protocol.WorkspaceFolder /*WorkspaceFolder[] | null*/, error) {
	log.Trace("lspClientHandler.WorkspaceFolders")
	return nil, nil
}

func (h *lspClientHandler) Configuration(
	ctx context.Context, p *protocol.ParamConfiguration,
) ([]interface{}, error) {
	log.Tracef("lspClientHandler.Configuration: %#v", p)

	results := make([]interface{}, len(p.Items))
	for i, item := range p.Items {
		if item.Section != "gopls" {
			continue
		}
		env := map[string]interface{}{}
		// for _, value := range c.app.env {
		// 	l := strings.SplitN(value, "=", 2)
		// 	if len(l) != 2 {
		// 		continue
		// 	}
		// 	env[l[0]] = l[1]
		// }
		m := map[string]interface{}{
			"env": env,
			"analyses": map[string]bool{
				"fillreturns":    true,
				"nonewvars":      true,
				"noresultvalues": true,
				"undeclaredname": true,
			},
		}
		// TODO configure
		m["verboseOutput"] = true
		results[i] = m
	}
	return results, nil
}

func (h *lspClientHandler) WorkDoneProgressCreate(
	ctx context.Context, params *protocol.WorkDoneProgressCreateParams,
) error {
	log.Tracef("lspClientHandler.WorkDoneProgressCreate: %#v", params)
	return nil
}

func (h *lspClientHandler) RegisterCapability(
	ctx context.Context, params *protocol.RegistrationParams,
) error {
	log.Tracef("lspClientHandler.RegisterCapability: %#v", params)
	return nil
}

func (h *lspClientHandler) UnregisterCapability(
	ctx context.Context, params *protocol.UnregistrationParams,
) error {
	log.Tracef("lspClientHandler.UnregisterCapability: %#v", params)
	return nil
}

func (h *lspClientHandler) ShowMessageRequest(
	ctx context.Context, params *protocol.ShowMessageRequestParams,
) (*protocol.MessageActionItem /*MessageActionItem | null*/, error) {
	log.Tracef("lspClientHandler.ShowMessageRequest: %#v", params)
	return nil, nil
}

func (h *lspClientHandler) ApplyEdit(
	ctx context.Context, params *protocol.ApplyWorkspaceEditParams,
) (*protocol.ApplyWorkspaceEditResult, error) {
	log.Tracef("lspClientHandler.ApplyEdit: %#v", params)
	return &protocol.ApplyWorkspaceEditResult{Applied: false, FailureReason: "not implemented"}, nil
}

func (h *lspClientHandler) ShowDocument(
	ctx context.Context, p *protocol.ShowDocumentParams,
) (*protocol.ShowDocumentResult, error) {
	log.Tracef("lspClientHandler.ShowDocument: %#v", p)
	return &protocol.ShowDocumentResult{Success: false}, nil
}
