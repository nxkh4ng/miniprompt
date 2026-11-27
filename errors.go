package miniprompt

import "errors"

// ErrCanceled is returned when the user cancels a prompt with Ctrl+C.
var ErrCanceled = errors.New("miniprompt: canceled")

// ErrNotTerminal is returned by prompts that cannot be shown without a
// terminal, such as Select.
var ErrNotTerminal = errors.New("miniprompt: not a terminal")

// ErrNoSelectOptions is returned by Select when it has no options to show.
var ErrNoSelectOptions = errors.New("miniprompt: select: no options")
