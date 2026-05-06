// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

package workspacessh

import "context"

// NotificationLevel mirrors the levels supported by the IDE notifications
// surface. Defined here so the workspacessh package does not depend on the
// IDE notifications package directly.
type NotificationLevel int

const (
	// NotificationInfo is an informational notification.
	NotificationInfo NotificationLevel = iota
	// NotificationWarning is a warning notification.
	NotificationWarning
	// NotificationError is an error notification.
	NotificationError
)

// UI is the surface workspacessh uses to interact with the user when
// authenticating against a remote host. The IDE provides a Browser-backed
// implementation; tests provide their own (typically recording) impl.
//
// All methods may be invoked from goroutines that are not the IDE's event
// loop: implementations are responsible for marshalling work onto the
// appropriate goroutine if required.
type UI interface {
	// PromptSecret displays a redacted single-line input with the given label
	// and returns the typed value. Returns context.Canceled if the user
	// dismisses the prompt.
	PromptSecret(ctx context.Context, label string) (string, error)

	// PromptText displays an unredacted single-line input. defaultValue may be
	// empty.
	PromptText(ctx context.Context, label, defaultValue string) (string, error)

	// PromptChoice displays a multi-option prompt and returns the selected
	// index, or context.Canceled if dismissed.
	PromptChoice(ctx context.Context, message string, options []string) (int, error)

	// Notify shows a non-blocking notification.
	Notify(level NotificationLevel, msg string)
}