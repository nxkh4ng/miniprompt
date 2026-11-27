package miniprompt

import "strings"

// ANSI SGR sequences used by the renderer.
const (
	sgrReset   = "\x1b[0m"
	sgrReverse = "\x1b[7m"
	sgrDim     = "\x1b[2m"
)

// Marker customises the three glyphs a prompt shows.
//
// The zero value draws "?" while waiting, "✓" on submit, and "✗" on a
// validation error. A single space (" ") hides that marker; any other value
// is shown followed by one space.
type Marker struct {
	Pending string
	Done    string
	Fail    string
}

// frame is one rendered prompt block.
//
// lines holds one entry per row and may contain ANSI styling. cursorRow and
// cursorCol are 0-based: cursorRow indexes lines, cursorCol is a cell in that
// row. footer, when not empty, is drawn on a row below lines and must be plain
// text.
type frame struct {
	lines     []string
	cursorRow int
	cursorCol int
	footer    string
}

// rows reports how many screen rows the frame occupies, at least one.
func (f frame) rows() int {
	n := len(f.lines)
	if f.footer != "" {
		n++
	}

	return max(n, 1)
}

// pendingFrame renders the line while the user is typing.
//
// The placeholder is drawn only when the buffer is empty.
// The selection is drawn in reverse video.
//
// Input currently draws a single, unbounded line; clipping it to cols is the
// job of a future viewport.
func (in Input) pendingFrame(e *editor) frame {
	var sb strings.Builder

	sb.WriteString(markerText(in.Marker.Pending, "?"))
	sb.WriteString(in.Label)
	sb.WriteString(" ")

	prefix := displayWidth(sb.String())
	cursor := prefix + displayWidth(string(e.buf[:e.cursor]))

	if len(e.buf) == 0 && in.Placeholder != "" {
		sb.WriteString(sgrDim)
		sb.WriteString(in.Placeholder)
		sb.WriteString(sgrReset)

		return frame{lines: []string{sb.String()}, cursorCol: cursor}
	}

	start, end, ok := e.selection()
	if ok {
		sb.WriteString(string(e.buf[:start]))
		sb.WriteString(sgrReverse)
		sb.WriteString(string(e.buf[start:end]))
		sb.WriteString(sgrReset)
		sb.WriteString(string(e.buf[end:]))
	} else {
		sb.WriteString(string(e.buf))
	}

	return frame{lines: []string{sb.String()}, cursorCol: cursor}
}

// doneLine is the final line shown once the value is accepted.
func (in Input) doneLine(value string) string {
	return doneText(in.Marker, in.Label, value)
}

// failLine is the line shown under the prompt when validation fails.
func (in Input) failLine(err error, cols int) string {
	return failText(in.Marker, err, cols)
}

// oneLine replaces control chars with spaces
// so a validator message containing a newline cannot
// break the block layout.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r < byteSpace || r == byteDel {
			return ' '
		}
		return r
	}, s)
}

// markerText returns a marker followed by a space.
// An empty marker uses def; a single space hides the marker.
func markerText(m, def string) string {
	switch m {
	case "":
		return def + " "
	case " ":
		return ""
	default:
		return m + " "
	}
}

// doneText renders the final summary line shared by all widgets.
func doneText(m Marker, label, value string) string {
	return markerText(m.Done, "✓") + label + " " + value
}

// failText renders the error row shared by all widgets.
func failText(m Marker, err error, cols int) string {
	s := markerText(m.Fail, "✗") + oneLine(err.Error())
	return cut(s, cols)
}
