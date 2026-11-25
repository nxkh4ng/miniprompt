package miniprompt

import "strings"

// ANSI SGR sequences used by the renderer.
const (
	sgrReset   = "\x1b[0m"
	sgrReverse = "\x1b[7m"
	sgrDim     = "\x1b[2m"
)

// frame is one rendered prompt block.
//
// line may contain ANSI styling;
// msg, when not empty, is drawn on a second row and must be plain text.
// cursor is the cursor cell on line, counted in terminal cells.
type frame struct {
	line   string
	msg    string
	cursor int
}

// pendingFrame for Input renders the line while the user is typing.
//
// The placeholder is drawn only when the buffer is empty.
// The selection is drawn in reverse video.
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

		return frame{
			line:   sb.String(),
			cursor: cursor,
		}
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

	return frame{
		line:   sb.String(),
		cursor: cursor,
	}
}

// doneLine is the final line shown once the value is accepted.
func (in Input) doneLine(value string) string {
	return markerText(in.Marker.Done, "✓") + in.Label + " " + value
}

// failLine is the line shown under the prompt when validation fails.
func (in Input) failLine(err error, cols int) string {
	s := markerText(in.Marker.Fail, "✗") + oneLine(err.Error())
	return cut(s, cols)
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
