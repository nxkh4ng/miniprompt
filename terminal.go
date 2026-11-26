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

// defaultCols is the width assumed when
// the output is not a terminal.
const defaultCols = 80

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
	cols := size(w)

	rows, err := in.draw(w, e, "", 0)
	if err != nil {
		return "", err
	}

	for {
		n, readErr := r.Read(buf)
		for _, ev := range dec.decode(buf[:n]) {
			switch applyEvent(ev, e) {
			case liveCancel:
				if _, werr := io.WriteString(w, "\r\n\x1b[K"); werr != nil {
					return "", werr
				}
				return "", ErrCanceled

			case liveSubmit:
				value := e.text()

				if verr := in.check(value); verr != nil {
					// Leave the last cell free so the message
					// cannot trigger a deferred wrap
					msg := in.failLine(verr, cols-1)

					if rows, err = in.draw(w, e, msg, rows); err != nil {
						return "", err
					}
					continue
				}

				return value, in.finish(w, value, rows)
			}

			if rows, err = in.draw(w, e, "", rows); err != nil {
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

// draw repaints the prompt block and reports how many rows it uses.
// footer is drawn on the last row; "" clears that row.
func (in Input) draw(w io.Writer, e *editor, footer string, prev int) (int, error) {
	f := in.pendingFrame(e)
	f.footer = footer

	return writeFrame(w, f, prev)
}

// finish replaces the block with the accepted line and moves below it,
// erasing the row the error used.
func (in Input) finish(w io.Writer, value string, rows int) error {
	s := "\r" + in.doneLine(value) + "\x1b[K\r\n"
	if rows > 1 {
		s += "\x1b[K"
	}

	_, err := io.WriteString(w, s)
	return err
}

// size reports the output width for the live prompt.
func size(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return defaultCols
	}

	cols, _, err := term.GetSize(int(f.Fd()))
	if err != nil || cols <= 0 {
		return defaultCols
	}

	return cols
}

// writeFrame repaints f over the block the previous frame used.
// It reports how many rows the block now occupies.
//
// The cursor is placed with CHA so the result does not depend on
// the column the previous row left behind.
func writeFrame(w io.Writer, f frame, prev int) (int, error) {
	rows := len(f.lines)
	if f.footer != "" {
		rows++
	}
	if rows == 0 {
		rows = 1
	}
	total := max(rows, prev)

	var sb strings.Builder
	sb.WriteString("\r")

	for i := 0; i < rows; i++ {
		if i > 0 {
			sb.WriteString("\r\n")
		}

		if i < len(f.lines) {
			sb.WriteString(f.lines[i])
		} else {
			sb.WriteString(f.footer)
		}
		sb.WriteString("\x1b[K")
	}

	// Clear rows left over from a taller previous frame
	for i := rows; i < total; i++ {
		sb.WriteString("\r\n\x1b[K")
	}

	// Cursor is on the last row; move it up to f.row
	if up := total - 1 - f.row; up > 0 {
		sb.WriteString("\x1b[")
		sb.WriteString(strconv.Itoa(up))
		sb.WriteString("A")
	}

	// CHA is absolute, so it does not depend on the previous row
	sb.WriteString("\x1b[")
	sb.WriteString(strconv.Itoa(f.col + 1))
	sb.WriteString("G")

	_, err := io.WriteString(w, sb.String())
	return total, err
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
	case evKillWordLeft:
		e.killWordLeft()
	case evKillWordRight:
		e.killWordRight()
	case evSelectAll:
		e.selectAll()
	case evEnter:
		return liveSubmit
	case evInterrupt:
		return liveCancel
	}

	return liveRedraw
}

// finishBlock overwrites the prompt block with the final lines
// and leaves the cursor on the line below.
func finishBlock(w io.Writer, lines []string, prev int) error {
	var sb strings.Builder
	sb.WriteString("\r")

	for i, line := range lines {
		if i > 0 {
			sb.WriteString("\r\n")
		}
		sb.WriteString(line)
		sb.WriteString("\x1b[K")
	}

	for i := len(lines); i < prev; i++ {
		sb.WriteString("\r\n\x1b[K")
	}

	if prev <= len(lines) {
		sb.WriteString("\r\n")
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// clearBlock moves below the block and clears that line.
// The block itself is left visible, matching the current cancel behavior.
func clearBlock(w io.Writer, f frame, rows int) error {
	var sb strings.Builder
	for i := f.row; i < rows; i++ {
		sb.WriteString("\r\n")
	}
	sb.WriteString("\x1b[K")

	_, err := io.WriteString(w, sb.String())
	return err
}
