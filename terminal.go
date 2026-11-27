package miniprompt

import (
	"io"
	"os"

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
	// so newlines must be written as "\r\n".
	defer func() {
		_ = term.Restore(int(r.Fd()), state)
	}()

	restoreVT, err := enableVT(w)
	if err != nil {
		return zero, err
	}
	defer restoreVT()

	// Bracketed paste keeps pasted newlines from submitting the prompt.
	if _, werr := io.WriteString(w, "\x1b[?2004h"); werr != nil {
		return zero, werr
	}
	defer func() {
		_, _ = io.WriteString(w, "\x1b[?2004l")
	}()

	return run(wd, r, w)
}

// terminalWidth reports the output width in cells, or defaultCols when w is
// not a terminal.
func terminalWidth(w io.Writer) int {
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

// terminalFiles reports whether r and w are both terminals.
func terminalFiles(r io.Reader, w io.Writer) (*os.File, *os.File, bool) {
	rf, rok := r.(*os.File)
	wf, wok := w.(*os.File)
	if !rok || !wok {
		return nil, nil, false
	}

	if !term.IsTerminal(int(rf.Fd())) || !term.IsTerminal(int(wf.Fd())) {
		return nil, nil, false
	}

	return rf, wf, true
}
