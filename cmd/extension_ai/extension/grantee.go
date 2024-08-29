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

package extension

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	configapi "unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/cmd/extension_ai/backend"
	"unstable.build/go-tui/cmd/extension_ai/backend/openai"
	"unstable.build/go-tui/cmd/extension_ai/dialogue"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/extension/extutil"
	"unstable.build/go-tui/rpc"
)

// GranteeWithService returns this extension's grantee, with the given
// backend.Service constructor as the backend servicing the LLM.
func GranteeWithService(
	svcFunc func(configapi.Config, map[string]int, string) (backend.Service, error),
	defaultAvailableModels map[string]int,
	defaultModel string,
	options ...dialogue.Option,
) (extension.Grantee, []extension.Permission) {
	commandEventHandler := func(
		ctx context.Context, ed textapi.Editor, grants []extension.Grant,
		broker rpc.MuxBroker, pconfig configapi.Config,
	) (hret extutil.CommandEventHandler, err error) {
		return CommandEventHandler(ctx, ed, grants, broker, pconfig,
			svcFunc, defaultAvailableModels, defaultModel, options...)
	}
	grantee, perms := extutil.NewEditorEventHandler(
		aIHandlerCommands, commandEventHandler, aIHandlerEvents,
		aIHandlerPermissions...)
	return grantee, perms
}

const (
	defaultDefaultModel = openai.GPT3Dot5Turbo
	defaultBaseURL      = "" // uses openai's default base URL
)

// DefaultOpenAIGrantee returns GranteeWithService satisfied by an OpenAI-like
// backend service. Configuration can set a 'base_url' to override openai's
// service URL with a custom one.
func DefaultOpenAIGrantee() (extension.Grantee, []extension.Permission) {
	openaiSvc := func(
		config configapi.Config, availableModels map[string]int, model string,
	) (backend.Service, error) {
		apiKey, err := config.GetString("api_key")
		if err != nil {
			err = fmt.Errorf("failed to get 'api_key' from config: %w", err)
			return nil, err
		}
		openaiConfig := openai.Config{
			Model: model,
		}
		openaiConfig.BaseURL, err = config.GetString("base_url")
		if err != nil {
			if !errors.Is(err, configapi.ErrNotFound) {
				err = fmt.Errorf("failed to get 'base_url' from config: %w", err)
				return nil, err
			}
			openaiConfig.BaseURL = defaultBaseURL
		}
		openaiConfig.FrequencyPenalty, err = config.GetFloat("frequency_penalty")
		if err != nil {
			if !errors.Is(err, configapi.ErrNotFound) {
				err = fmt.Errorf("failed to get 'frequency_penalty' from config: %w", err)
				return nil, err
			}
		}
		openaiConfig.MaxTokens, err = config.GetInt("max_tokens")
		if err != nil {
			if !errors.Is(err, configapi.ErrNotFound) {
				err = fmt.Errorf("failed to get 'max_tokens' from config: %w", err)
				return nil, err
			}
		}
		openaiConfig.PresencePenalty, err = config.GetFloat("presence_penalty")
		if err != nil {
			if !errors.Is(err, configapi.ErrNotFound) {
				err = fmt.Errorf("failed to get 'presence_penalty' from config: %w", err)
				return nil, err
			}
		}
		openaiConfig.Temperature, err = config.GetFloat("temperature")
		if err != nil {
			if !errors.Is(err, configapi.ErrNotFound) {
				err = fmt.Errorf("failed to get 'temperature' from config: %w", err)
				return nil, err
			}
		}
		openaiConfig.TopP, err = config.GetFloat("top_p")
		if err != nil {
			if !errors.Is(err, configapi.ErrNotFound) {
				err = fmt.Errorf("failed to get 'top_p' from config: %w", err)
				return nil, err
			}
		}
		return openai.NewClient(apiKey, openaiConfig, availableModels), nil
	}
	return GranteeWithService(openaiSvc,
		openai.AvailableModels(), defaultDefaultModel,
		dialogue.WithCompleter(dialogue.FuncCompleter(logCompletion)),
	)
}

func logCompletion(
	ctx context.Context, dialogueID, completionID string,
	finishReason backend.FinishReason,
	msg backend.ChatCompletionMessage,
) {
	log.WithFields(log.Fields{
		"reason":            finishReason,
		"dialogueID":        dialogueID,
		"completionID":      completionID,
		logging.KeyCallType: "logCompletion",
		logging.KeyFile:     "main.go",
	}).Debugf("%+v", msg)
}
