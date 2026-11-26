package miniprompt

import (
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// defaultCols is the width assumed when the output is not a terminal.
const defaultCols = 80

// liveSession runs wd on a terminal in raw mode.
// It restores the terminal on every exit path.
func liveSession[T any](r, w *os.File, wd widget[T]) (T, error) {
	var zero T

	state, err := term.MakeRaw(int(r.Fd()))
	if err != nil {
		return zero, err
	}

	// Raw mode also disables output processing on the same TTY,
	// so newlines must be written as "\r\n"
	defer func() {
		_ = term.Restore(int(r.Fd()), state)
	}()

	restoreVT, err := enableVT(w)
	if err != nil {
		return zero, err
	}
	defer restoreVT()

	// Bracketed paste keeps pasted newlines from submitting the prompt
	if _, werr := io.WriteString(w, "\x1b[?2004h"); werr != nil {
		return zero, werr
	}
	defer func() {
		_, _ = io.WriteString(w, "\x1b[?2004l")
	}()

	return run(wd, r, w)
}

// liveLoop is the live path without raw-mode setup, for tests.
func (in Input) liveLoop(r io.Reader, w io.Writer) (string, error) {
	return run(newInputWidget(in), r, w)
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
