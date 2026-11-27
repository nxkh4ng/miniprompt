package miniprompt

import (
	"fmt"
	"io"
)

// action is what one event asks the driver to do next.
type action uint8

const (
	// actRedraw is the zero value, so an event that changes state only has
	// to return from handle for the block to be repainted.
	actRedraw action = iota
	// actSubmit asks the driver to call submit.
	actSubmit
	// actCancel asks the driver to clear the block and return ErrCanceled.
	actCancel
)

// widget is the per-prompt state machine that run drives.
//
// A widget owns the prompt state: the line being typed, the highlighted
// option, and so on. Everything else (reading keys, moving the cursor,
// restoring the terminal) is shared by the driver.
type widget[T any] interface {
	// view returns the rows to draw. cols is the terminal width in cells,
	// so a widget can fit long content.
	view(cols int) frame

	// handle applies one event and reports what run should do next.
	handle(ev event) action

	// submit validates the current state and returns the value.
	// A non-nil error is drawn as a footer and the prompt continues.
	submit() (T, error)

	// fail renders the footer for a rejected submit.
	fail(err error, cols int) string

	// final returns the lines left in the transcript after a submit.
	final(v T) []string
}

// run reads keys, applies them to wd, and redraws until submit or cancel.
// It only needs io.Reader and io.Writer, so it is testable without a TTY.
//
// run is the live path: the caller (liveSession) has already put the terminal
// in raw mode, which makes Read return as soon as a key is pressed.
func run[T any](wd widget[T], r io.Reader, w io.Writer) (T, error) {
	var zero T

	scr := &screen{w: w}

	cols := terminalWidth(w)
	if err := scr.draw(wd.view(cols)); err != nil {
		return zero, err
	}

	dec := &keyDecoder{}
	buf := make([]byte, 256)

	for {
		n, readErr := r.Read(buf)

		events := dec.decode(buf[:n])
		if readErr != nil {
			// Release a paste left open by EOF.
			events = append(events, dec.flush()...)
		}

		for _, ev := range events {
			switch wd.handle(ev) {
			case actCancel:
				if err := scr.clear(); err != nil {
					return zero, err
				}

				return zero, ErrCanceled

			case actSubmit:
				v, verr := wd.submit()
				if verr != nil {
					// Leave the last cell free so the message
					// cannot trigger a deferred wrap.
					cols = terminalWidth(w)
					f := wd.view(cols)
					f.footer = wd.fail(verr, cols-1)

					if err := scr.draw(f); err != nil {
						return zero, err
					}

					// The prompt stays open after a rejected submit.
					continue
				}

				return v, scr.finish(wd.final(v))
			}

			// Re-read the width on every key so a resize takes effect.
			cols = terminalWidth(w)
			if err := scr.draw(wd.view(cols)); err != nil {
				return zero, err
			}
		}

		if readErr != nil {
			return zero, fmt.Errorf("miniprompt: read input: %w", readErr)
		}
	}
}
