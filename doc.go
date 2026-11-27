// Package miniprompt builds small interactive prompts for the terminal.
//
// # How a prompt runs
//
// A prompt has two paths:
//
//   - Live mode, used when stdin and stdout are terminals. The input is put
//     in raw mode so every key press arrives immediately, and the prompt is
//     redrawn in place. The terminal is restored on submit, cancel and error.
//   - Plain mode, used by text prompts such as Input when either side is not
//     a terminal. The prompt is written once and one line is read.
//
// The live path is the interesting one:
//
//	bytes -> keyDecoder -> event -> widget.handle -> action
//	                      ^                        |
//	                      +---- screen.draw <------+
//
// keyDecoder keeps unfinished bytes, so an escape sequence or a UTF-8 rune
// split across two reads is decoded correctly. A widget owns the prompt state
// (the line being typed, the highlighted option). It returns a frame for the
// driver to draw, and an action saying whether the prompt should redraw,
// submit, or cancel. screen remembers how many rows the block reserves and
// where the cursor is inside it, so the block can be repainted without
// growing the transcript.
//
// # Writing a widget
//
// Implement widget[T], where T is the value the prompt returns. Mutate state
// only in handle and submit; view should just describe the current state.
// Return actRedraw after a state change, actSubmit when the value is ready,
// and actCancel for Ctrl+C. Reuse the shared helpers (doneText, failText,
// cut) so every prompt looks the same.
//
// # Plain mode
//
// Text prompts fall back to plain mode when either side is not a terminal,
// which keeps tests and piped input working. Choice prompts return
// ErrNotTerminal instead, because a list cannot be shown without a terminal.
package miniprompt
