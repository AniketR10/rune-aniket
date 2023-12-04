package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"

	configapi "unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/cmd/extension_ai/backend"
	"unstable.build/go-tui/cmd/extension_ai/backend/openai"
	aiExtension "unstable.build/go-tui/cmd/extension_ai/extension"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/process"
	extutil "unstable.build/go-tui/extension/util"
	"unstable.build/go-tui/proto"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:8886", nil))
	}()

	openaiSvc := func(
		ed textapi.Editor, grants []extension.Grant,
		broker proto.MuxBroker, pconfig configapi.Config,
	) (hret extutil.CommandEventHandler, err error) {
		apiKey, err := pconfig.GetString("api_key")
		if err != nil {
			err = fmt.Errorf("failed to get 'api_key' from config: %w", err)
			return nil, err
		}
		return aiExtension.CommandEventHandler(ed, grants, broker, pconfig,
			func(model string) backend.Service {
				return openai.NewClient(apiKey, openai.Config{
					Model: model,
				})
			})
	}

	grantee, perms := extutil.NewEditorEventHandler(aiExtension.AIHandlerCommands,
		openaiSvc, aiExtension.AIHandlerEvents,
		aiExtension.AIHandlerPermissions...)
	process.Serve(grantee, perms...)
}
