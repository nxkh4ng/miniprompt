package miniprompt

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// liveResult is what 1 key asks the live loop
// to do next.
type liveResult uint8

const (
	liveRedraw liveResult = iota
	liveSubmit
	liveCancel
)

// runLive prompts on a terminal using raw mode.
func (in Input) runLive(r, w *os.File) (string, error) {
	state, err := term.MakeRaw(int(r.Fd()))
	if err != nil {
		return "", err
	}

	// Raw mode also disables output processing on the same TTY,
	// so newlines must be written as "\r\n"
	defer func() {
		_ = term.Restore(int(r.Fd()), state)
	}()

	restoreVT, err := enableVT(w)
	if err != nil {
		return "", err
	}
	defer restoreVT()

	// Bracketed paste keeps pasted newlines from submitting the prompt
	if _, werr := io.WriteString(w, "\x1b[?2004h"); werr != nil {
		return "", werr
	}
	defer func() {
		_, _ = io.WriteString(w, "\x1b[?2004l")
	}()

	return in.liveLoop(r, w)
}

// liveLoop reads keys and redraws the prompt until the user submits.
// It only needs io.Reader and io.Writer, so it is testable without a TTY.
func (in Input) liveLoop(r io.Reader, w io.Writer) (string, error) {
	e := &editor{}
	dec := &keyDecoder{}
	buf := make([]byte, 256)

	if err := in.drawPrompt(w, e); err != nil {
		return "", err
	}

	for {
		n, readErr := r.Read(buf)
		for _, ev := range dec.decode(buf[:n]) {
			switch applyEvent(ev, e) {
			case liveCancel:
				if _, err := io.WriteString(w, "\r\n"); err != nil {
					return "", err
				}
				return "", ErrCanceled

			case liveSubmit:
				value := e.text()

				if verr := in.check(value); verr != nil {
					if err := in.drawReject(w, e, verr); err != nil {
						return "", err
					}
					continue
				}

				if err := in.drawDone(w, e); err != nil {
					return "", err
				}

				return value, nil
			}

			if err := in.drawPrompt(w, e); err != nil {
				return "", err
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return "", fmt.Errorf("miniprompt: read input: %w", readErr)
			}

			return "", readErr
		}
	}
}

// drawPrompt redraws the typing line and puts the cursor back in place.
func (in Input) drawPrompt(w io.Writer, e *editor) error {
	return writeFrame(w, in.pendingFrame(e))
}

// drawDone replaces the typing line with the final one
// and starts a new line.
func (in Input) drawDone(w io.Writer, e *editor) error {
	_, err := io.WriteString(w, "\r"+in.doneLine(e.text())+"\x1b[K\r\n")
	return err
}

// drawReject prints the validation error and
// reopens the prompt line.
// The typed text is kept so the user can fix it.
func (in Input) drawReject(w io.Writer, e *editor, err error) error {
	if _, werr := io.WriteString(w, "\r\n"+in.failLine(err)+"\r\n"); werr != nil {
		return werr
	}

	return in.drawPrompt(w, e)
}

// writeFrame overwrites the current line with f
// and restores the cursor column.
func writeFrame(w io.Writer, f frame) error {
	var sb strings.Builder

	sb.WriteString("\r")
	sb.WriteString(f.line)
	sb.WriteString("\x1b[K")

	if n := f.back(); n > 0 {
		sb.WriteString("\x1b[")
		sb.WriteString(strconv.Itoa(n))
		sb.WriteString("D")
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// applyEvent applies one decoded key to the editor.
func applyEvent(ev event, e *editor) liveResult {
	switch ev.kind {
	case evRune:
		e.insertRune(ev.r)
	case evPaste:
		e.insertText(ev.text)
	case evBackspace:
		e.backspace()
	case evDelete:
		e.deleteForward()
	case evLeft:
		e.left()
	case evRight:
		e.right()
	case evWordLeft:
		e.wordLeft()
	case evWordRight:
		e.wordRight()
	case evHome:
		e.home()
	case evEnd:
		e.end()
	case evKillWord:
		e.killWord()
	case evSelectAll:
		e.selectAll()
	case evEnter:
		return liveSubmit
	case evInterrupt:
		return liveCancel
	}

	return liveRedraw
}
