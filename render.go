package miniprompt

import "strings"

// ANSI SGR sequences used by the renderer.
const (
	sgrReset   = "\x1b[0m"
	sgrReverse = "\x1b[7m"
	sgrDim     = "\x1b[2m"
)

// frame is one rendered prompt line.
//
// line may contain ANSI styling.
// cursor and width are display columns,
// counted in terminal cells, never in bytes or runes.
type frame struct {
	line   string
	cursor int
	width  int
}

// back reports how many columns to move left
// to put the cursor in place.
func (f frame) back() int {
	return f.width - f.cursor
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
			width:  prefix + displayWidth(in.Placeholder),
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
		width:  prefix + displayWidth(string(e.buf)),
	}
}
