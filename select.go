package miniprompt

import (
	"io"
	"os"
)

// Select lets the user pick one option from a list.
type Select struct {
	// Marker customises the pending, done and fail glyphs.
	Marker Marker

	// Label is shown above the options.
	Label string

	// Options are the values the user can pick from.
	Options []string

	// Height limits how many options are visible at once.
	// Zero or a value >= len(Options) shows the whole list.
	Height int
}

type selectWidget struct {
	sel    Select
	cursor int
	top    int
}

func newSelectWidget(s Select) *selectWidget {
	return &selectWidget{sel: s}
}

func (w *selectWidget) view(cols int) frame {
	s := markerText(w.sel.Marker.Pending, "?") + w.sel.Label
	lines := []string{cut(s, cols)}

	n := len(w.sel.Options)
	if n == 0 {
		return frame{lines: lines}
	}

	height := w.height()

	if w.top > w.cursor {
		w.top = w.cursor
	}

	if w.top+height <= w.cursor {
		w.top = w.cursor - height + 1
	}

	for i := w.top; i < w.top+height && i < n; i++ {
		prefix := "  "
		if i == w.cursor {
			prefix = "> "
		}

		lines = append(lines, cut(prefix+w.sel.Options[i], cols))
	}

	return frame{
		lines:     lines,
		cursorRow: 1 + (w.cursor - w.top),
		cursorCol: 0,
	}
}

func (w *selectWidget) handle(ev event) action {
	switch ev.kind {
	case evUp:
		if w.cursor > 0 {
			w.cursor--
		}
	case evDown:
		if w.cursor < len(w.sel.Options) {
			w.cursor++
		}
	case evHome:
		w.cursor = 0
	case evEnd:
		if n := len(w.sel.Options); n > 0 {
			w.cursor = n - 1
		}
	case evEnter:
		return actSubmit
	case evInterrupt:
		return actCancel
	}

	w.ensureVisible()

	return actRedraw
}

func (w *selectWidget) submit() (string, error) {
	if len(w.sel.Options) == 0 {
		return "", ErrNoSelectOptions
	}

	return w.sel.Options[w.cursor], nil
}

func (w *selectWidget) fail(err error, cols int) string {
	return failText(w.sel.Marker, err, cols)
}

func (w *selectWidget) final(v string) []string {
	return []string{doneText(w.sel.Marker, w.sel.Label, v)}
}

func (w *selectWidget) height() int {
	n := len(w.sel.Options)
	if n == 0 {
		return 1
	}

	if w.sel.Height > 0 && w.sel.Height < n {
		return w.sel.Height
	}

	return n
}

func (w *selectWidget) ensureVisible() {
	h := w.height()

	if w.cursor < w.top {
		w.top = w.cursor
	}

	if w.cursor >= w.top+h {
		w.top = w.cursor - h + 1
	}
}

// Run prompts on os.Stdin and os.Stdout.
func (s Select) Run() (string, error) {
	return s.RunWith(os.Stdin, os.Stdout)
}

// RunWith reads from r and writes to w. Select needs a terminal.
func (s Select) RunWith(r io.Reader, w io.Writer) (string, error) {
	if len(s.Options) == 0 {
		return "", ErrNoSelectOptions
	}

	rf, wf, ok := terminalFiles(r, w)
	if !ok {
		return "", ErrNotTerminal
	}

	return liveSession[string](rf, wf, newSelectWidget(s))
}
