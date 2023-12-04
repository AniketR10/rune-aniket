package extension

import (
	configapi "unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/cmd/extension_ai/backend"
	"unstable.build/go-tui/extension"
	extutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/proto"
)

// GranteeWithService returns this extension's grantee, with the given
// backend.Service constructor as the backend servicing the LLM.
func GranteeWithService(
	svcFunc func(config configapi.Config, model string) (backend.Service, error),
) (extension.Grantee, []extension.Permission) {
	commandEventHandler := func(
		ed textapi.Editor, grants []extension.Grant,
		broker proto.MuxBroker, pconfig configapi.Config,
	) (hret extutil.CommandEventHandler, err error) {
		return CommandEventHandler(ed, grants, broker, pconfig, svcFunc)
	}
	grantee, perms := extutil.NewEditorEventHandler(AIHandlerCommands,
		commandEventHandler, AIHandlerEvents,
		AIHandlerPermissions...)
	return grantee, perms
}
