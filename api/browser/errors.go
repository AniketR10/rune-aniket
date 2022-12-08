package api

import "errors"

var (
	// ErrTabNotFree is returned when a tab is being used in call to
	// SetContent but it's already owned by another Window.
	ErrTabNotFree = errors.New("Tab already rendered in Window")
)
