package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"

	log "github.com/sirupsen/logrus"
	configapi "unstable.build/go-tui/api/config"
	"unstable.build/go-tui/cmd/extension_ai/backend"
	"unstable.build/go-tui/cmd/extension_ai/backend/openai"
	"unstable.build/go-tui/cmd/extension_ai/extension"
	"unstable.build/go-tui/extension/process"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:8886", nil))
	}()
	openaiSvc := func(config configapi.Config, model string) (backend.Service, error) {
		apiKey, err := config.GetString("api_key")
		if err != nil {
			err = fmt.Errorf("failed to get 'api_key' from config: %w", err)
			return nil, err
		}
		return openai.NewClient(apiKey, openai.Config{
			Model:   model,
		}, availableModels()), nil
	}
	grantee, perms := extension.GranteeWithService(openaiSvc,
		availableModels(), defaultModel)
	process.Serve(grantee, perms...)
}

var defaultModel = openai.GPT3Dot5Turbo

func availableModels() map[string]int {
	return openai.AvailableModels()
}
