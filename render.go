package miniprompt

import (
	"strings"
	"unicode"
)

// Glyphs are the leading symbols for the three states every prompt shares:
// waiting for input, accepted, and rejected.
//
// The zero value draws "? ", "✓ " and "✗ ". A single space (" ") hides the
// glyph; any other value is drawn followed by one space.
//
// Glyphs does not cover a widget's own symbols, such as Select's pointer.
type Glyphs struct {
	Pending string
	Done    string
	Fail    string
}

// frame is one rendered prompt block.
//
// lines holds one entry per row and may contain ANSI styling. cursorRow and
// cursorCol are 0-based: cursorRow indexes lines, cursorCol is a cell in that
// row. footer, when not empty, is drawn on one plain-text row below lines.
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

// headerText renders the first line shared by every prompt:
// the pending glyph followed by the label.
//
// It has no trailing space, so a widget decides whether its content needs one.
func headerText(g Glyphs, label string) string {
	return glyphText(g.Pending, "?") + label
}

// doneText renders the final line left in the transcript after a submit.
func doneText(g Glyphs, label, value string) string {
	return glyphText(g.Done, "✓") + label + " " + value
}

// failText renders the footer for a rejected submit.
//
// The message is flattened to one line and cut to cols cells, so a validator
// returning a multi-line message cannot break the block layout.
func failText(g Glyphs, err error, cols int) string {
	s := glyphText(g.Fail, "✗") + oneLine(err.Error())
	return cut(s, cols)
}

// oneLine replaces control runes with spaces so a message can containing
// a newline or tab cannot break the block layout.
func oneLine(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

// glyphText returns one glyph followed by a space.
// A empty glyph uses def; a single space hides the glyph.
func glyphText(g, def string) string {
	switch g {
	case "":
		return def + " "
	case " ":
		return ""
	default:
		return g + " "
	}
}
