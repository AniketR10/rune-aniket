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

package ideupgrade

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/component/markdown"
	"unstable.build/go-tui/debug"
)

const (
	upgradeNowOpt  = "   Upgrade Now   "
	remindLaterOpt = "   Remind Me Later   "
	skipVersionOpt = "   Skip This Version   "
)

// Choice is the user's answer to the upgrade prompt.
type Choice int

// The possible answers to the upgrade prompt. ChoiceDismissed covers
// both an explicit dismissal and the case where no window manager is
// available to render the prompt.
const (
	ChoiceDismissed Choice = iota
	ChoiceUpgradeNow
	ChoiceRemindLater
	ChoiceSkipVersion
)

// PromptChoice renders the floating upgrade prompt and blocks until
// the user answers it or ctx is cancelled. Remind/skip answers are
// persisted before returning. It must not be called from the IDE
// event loop: the prompt is scheduled onto that loop and the answer
// arrives from it.
func (m *Manager) PromptChoice(
	ctx context.Context, manifest Manifest,
) (Choice, error) {
	if m.cfg.WindowManager == nil {
		return ChoiceDismissed, nil
	}
	// Buffered so the first answer never blocks the event loop and a
	// dismissal following a selection is simply dropped.
	answer := make(chan Choice, 1)
	reply := func(c Choice) {
		select {
		case answer <- c:
		default:
		}
	}
	prompt := m.newPrompt(manifest, reply)
	scheduled := m.cfg.ScheduleNextTick(func() {
		if _, err := m.cfg.WindowManager.Floating(prompt, browserapi.FloatingConfig{
			Alignment: component.AlignmentCentered,
		}); err != nil {
			log.WithError(err).Warn("ideupgrade: show upgrade prompt")
			reply(ChoiceDismissed)
		}
	})
	if !scheduled {
		return ChoiceDismissed, errors.New("could not schedule upgrade prompt")
	}

	select {
	case <-ctx.Done():
		return ChoiceDismissed, ctx.Err()
	case c := <-answer:
		m.persistChoice(ctx, c, manifest)
		return c, nil
	}
}

// showPrompt renders the floating upgrade prompt for manifest without
// blocking. The user's choice is persisted (remind / skip) and, when
// "Upgrade Now" is chosen, the upgrade runs on a background goroutine
// reporting through notifications.
func (m *Manager) showPrompt(ctx context.Context, manifest Manifest) {
	if m.cfg.WindowManager == nil {
		_, _ = m.cfg.Notifications.Notify(browserapi.LevelInfo,
			"Rune %s is available", manifest.Version)
		return
	}

	prompt := m.newPrompt(manifest, func(c Choice) {
		if c != ChoiceUpgradeNow {
			m.persistChoice(ctx, c, manifest)
			return
		}
		go debug.CapturePanicReport(func() {
			if err := m.upgradeWithNotifications(ctx, manifest); err != nil {
				log.WithError(err).Warn("ideupgrade: run upgrade")
			}
		})
	})

	m.cfg.ScheduleNextTick(func() {
		if _, err := m.cfg.WindowManager.Floating(prompt, browserapi.FloatingConfig{
			Alignment: component.AlignmentCentered,
		}); err != nil {
			log.WithError(err).Warn("ideupgrade: show upgrade prompt")
		}
	})
}

// persistChoice records a remind/skip answer. Upgrade and dismissal
// leave the stored state untouched so the next check prompts again.
func (m *Manager) persistChoice(ctx context.Context, c Choice, manifest Manifest) {
	switch c {
	case ChoiceRemindLater:
		m.remindLater(ctx)
	case ChoiceSkipVersion:
		m.skipVersion(ctx, manifest.Version)
	}
}

// newPrompt builds the floating upgrade prompt for manifest. onChoice
// is invoked with the user's answer, or with ChoiceDismissed when the
// prompt is closed without a selection.
func (m *Manager) newPrompt(
	manifest Manifest, onChoice func(Choice),
) *handler.Prompt {
	message := fmt.Sprintf("A new version of Rune is available: **%s** (current: %s).\n\nUpgrade now?",
		manifest.Version, m.cfg.CurrentVersion)
	if manifest.Changelog != "" {
		message = manifest.Changelog + "\n\n---\n\n" + message
	}

	return handler.NewPrompt(handler.PromptConfig{
		HighlightAttr: term.Attributes{
			Attrs: term.AttrBold,
			Bg:    term.ColorBlue,
		},
		OptionAttr: term.Attributes{
			Attrs: term.AttrBold,
			Bg:    term.ColorGray,
		},
		OptionBindings: []term.KeyComb{{Ch: 'y'}, {Ch: 'l'}, {Ch: 's'}},
		PromptConfig: component.PromptConfig{
			Message:    message,
			Options:    []string{upgradeNowOpt, remindLaterOpt, skipVersionOpt},
			NewMessage: markdownOrFallback(m.cfg.Parser, m.cfg.ScheduleNextTick),
		},
		PromptHandler: handler.FuncPromptHandler(func(idx int, _ string) {
			switch idx {
			case 0:
				onChoice(ChoiceUpgradeNow)
			case 1:
				onChoice(ChoiceRemindLater)
			case 2:
				onChoice(ChoiceSkipVersion)
			}
		}, func() error {
			onChoice(ChoiceDismissed)
			return nil
		}),
	})
}

// markdownOrFallback returns a NewMessage callback that renders the
// prompt body as markdown when possible, falling back to a plain
// responsive string when the parser is unavailable. It mirrors the
// helper in idepkg/update_checker.go but is duplicated here to avoid
// pulling idepkg into ideupgrade.
func markdownOrFallback(
	parser syntaxapi.Parser, scheduleNextTick func(func()) bool,
) func(string) component.Floating {
	return func(str string) component.Floating {
		mcfg := markdown.DefaultConfig()
		mcfg.HeaderPrefix = false
		mcfg.ScheduleNextTick = scheduleNextTick
		mcfg.Parser = parser
		mkd, err := markdown.NewWithConfig(str, mcfg)
		if err == nil {
			return component.NewSpan(mkd, component.SpanConfig{
				PadHorizontal:    4,
				PadVertical:      2,
				ContentAlignment: component.AlignmentCentered,
			})
		}
		cfg := component.StringResponsiveConfig{
			NoSplitWords: true,
			StringConfig: component.StringConfig{
				PaddingVertical:   4,
				PaddingHorizontal: 4,
				Alignment:         component.AlignmentCentered,
			},
		}
		messageResponsive := component.NewResponsiveString(str, cfg)

		return component.NewAspectRatioFloatingResponsive(
			messageResponsive, component.DefaultAspectRatio)
	}
}
