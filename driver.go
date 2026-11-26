package miniprompt

import (
	"errors"
	"fmt"
	"io"
)

// action is what one event asks the driver to do next.
type action uint8

const (
	actRedraw action = iota
	actSubmit
	actCancel
)

// widget is the per-prompt state machine the driver runs.
// T is the value returned on submit.
type widget[T any] interface {
	view(cols int) frame
	handle(ev event) action
	submit() (T, error)
	fail(err error, cols int) string
	final(v T) []string
}

// run reads keys, applies them to wd, and redraws until submit or cancel.
// It only needs io.Reader and io.Writer, so it is testable without a TTY.
func run[T any](wd widget[T], r io.Reader, w io.Writer) (T, error) {
	var zero T

	cols := size(w)
	prev, err := writeFrame(w, wd.view(cols), 0)
	if err != nil {
		return zero, err
	}

	dec := &keyDecoder{}
	buf := make([]byte, 256)

	for {
		n, readErr := r.Read(buf)
		events := dec.decode(buf[:n])

		if readErr != nil {
			// Release a paste left open by EOF
			events = append(events, dec.flush()...)
		}

		for _, ev := range events {
			switch wd.handle(ev) {
			case actCancel:
				if err := clearBlock(w, wd.view(cols), prev); err != nil {
					return zero, err
				}
				return zero, ErrCanceled

			case actSubmit:
				v, verr := wd.submit()
				if verr != nil {
					// Leave the last cell free so the message
					// cannot trigger a deferred wrap
					f := wd.view(cols)
					f.footer = wd.fail(verr, cols-1)

					if prev, err = writeFrame(w, f, prev); err != nil {
						return zero, err
					}
					continue
				}
				return v, finishBlock(w, wd.final(v), prev)
			}

			// Re-read on every key so resize takes effect
			cols = size(w)
			if prev, err = writeFrame(w, wd.view(cols), prev); err != nil {
				return zero, err
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return zero, fmt.Errorf("miniprompt: read input: %w", readErr)
			}
			return zero, readErr
		}
	}
}
