// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agentshell

import (
	"context"
	"fmt"

	"unstable.build/go-tui/cmd/rune-agent/memory/dream"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func (s *shell) handleDream(
	ctx context.Context, args []string,
) (iterator.Iterator[component.Responsive], error) {
	model := s.defaultModel
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--model":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("--model requires a value")
			}
			i++
			model = args[i]
		default:
			return nil, fmt.Errorf(
				"unknown argument: %s\nusage: dream [--model MODEL]", args[i])
		}
	}

	entry, ok := s.modelRegistry.Get(ctx, model)
	if !ok {
		return nil, fmt.Errorf("model %q not found in registry", model)
	}

	deps := dream.Deps{
		LLM:           s.svc,
		Store:         s.store,
		Storage:       s.storage,
		FS:            s.fs,
		Exec:          s.exec,
		LSP:           s.lsp,
		Parser:        s.parser,
		Notifications: s.notifications,
		DataPath:      s.dataPath,
		Model:         model,
		Provider:      entry.Provider,
	}

	it, err := dream.Dream(ctx, deps)
	if err != nil {
		return nil, err
	}

	notifID, err := s.notifications.Notify(
		browserapi.LevelInfo, "Dream: starting...")
	if err != nil {
		_ = it.Close()
		return nil, fmt.Errorf("notify: %v", err)
	}
	var lastProgress, lastTotal int64 = 0, 1
	_ = s.notifications.UpdateNotificationProgress(notifID, "Dream: starting...", lastProgress, lastTotal)

	return iterator.FromFunc(
		func(ctx context.Context) (component.Responsive, bool, error) {
			for {
				p, ok := it.Next(ctx)
				if !ok {
					if err := it.Err(); err != nil {
						return nil, false, err
					}
					return nil, false, nil
				}

				if p.Total > 0 {
					lastTotal = int64(p.Total)
					lastProgress = int64(p.Progress)
				}

				switch p.Type {
				case dream.ProgressError:
					if p.Message != "" {
						_ = s.notifications.UpdateNotificationProgress(
							notifID, p.Message, min(lastProgress, lastTotal-1), lastTotal)
					}
					return responsiveString(fmt.Sprintf("Error [%s]: %s", p.DialogueID, p.Message)), true, nil

				case dream.ProgressDone:
					_ = s.notifications.UpdateNotificationProgress(
						notifID, p.Message, lastTotal, lastTotal)
					return responsiveString(fmt.Sprintf("Done: %s", p.Message)), true, nil

				default:
					if p.Message != "" {
						_ = s.notifications.UpdateNotificationProgress(
							notifID, p.Message, min(lastProgress, lastTotal-1), lastTotal)
					}
				}
			}
		},
		func() error { return it.Close() },
	), nil
}

func responsiveString(s string) component.Responsive {
	return component.NewResponsiveString(s, component.StringResponsiveConfig{})
}
