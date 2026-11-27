package miniprompt

import (
	"fmt"
	"io"
	"strings"
)

// ansiEraseLine clears a row from the cursor to the end of the line.
const ansiEraseLine = "\x1b[K"

// screen draws the prompt block in place.
//
// The block is a group of rows owned by the prompt. The hardware cursor is
// always somewhere inside it, on frame.cursorRow. To repaint, screen first
// moves back to the first row of the block, draws the new frame, clears rows
// left over from a taller previous frame, and finally puts the cursor back on
// the requested row and column.
//
// rows and row are the only state: rows is how many rows are reserved on
// screen, row is the 0-based row holding the cursor. The zero value is ready
// to use.
type screen struct {
	w    io.Writer
	rows int
	row  int
}

// draw repaints the reserved block with f.
func (s *screen) draw(f frame) error {
	rows := f.rows()
	total := max(rows, s.rows)

	var sb strings.Builder

	// The cursor sits on block top + s.row; go back to the top first.
	moveUp(&sb, s.row)
	sb.WriteString("\r")

	for i := range rows {
		if i > 0 {
			sb.WriteString("\r\n")
		}

		line := f.footer
		if i < len(f.lines) {
			line = f.lines[i]
		}

		sb.WriteString(line)
		sb.WriteString(ansiEraseLine)
	}

	// Clear rows left over from a taller previous frame.
	for i := rows; i < total; i++ {
		sb.WriteString("\r\n")
		sb.WriteString(ansiEraseLine)
	}

	// Move from the last row up to the requested cursor row, then set the
	// column absolutely so it does not depend on the previous row.
	moveUp(&sb, total-1-f.cursorRow)
	moveColumn(&sb, f.cursorCol)

	s.rows = total
	s.row = f.cursorRow

	return s.write(sb.String())
}

// finish replaces the block with lines and leaves the cursor on the row just
// below them. It resets the screen so the next prompt starts clean.
func (s *screen) finish(lines []string) error {
	var sb strings.Builder

	moveUp(&sb, s.row)
	sb.WriteString("\r")

	for i, line := range lines {
		if i > 0 {
			sb.WriteString("\r\n")
		}

		sb.WriteString(line)
		sb.WriteString(ansiEraseLine)
	}

	if s.rows > len(lines) {
		// Clear the rows the previous frame used, then step back to the
		// line just below the final output.
		for i := len(lines); i < s.rows; i++ {
			sb.WriteString("\r\n")
			sb.WriteString(ansiEraseLine)
		}

		moveUp(&sb, s.rows-len(lines)-1)
	} else {
		sb.WriteString("\r\n")
	}

	s.rows, s.row = 0, 0

	return s.write(sb.String())
}

// clear moves below the block and clears the line there.
// The block itself is left visible, matching the cancel behavior.
func (s *screen) clear() error {
	var sb strings.Builder

	moveUp(&sb, s.row)

	if s.rows == 0 {
		sb.WriteString("\r")
	} else {
		for i := 0; i < s.rows; i++ {
			sb.WriteString("\r\n")
		}
	}

	sb.WriteString(ansiEraseLine)

	s.rows, s.row = 0, 0

	return s.write(sb.String())
}

func (s *screen) write(text string) error {
	_, err := io.WriteString(s.w, text)
	return err
}

// moveUp emits CSI n A. n <= 0 is a no-op.
func moveUp(sb *strings.Builder, n int) {
	if n > 0 {
		fmt.Fprintf(sb, "\x1b[%dA", n)
	}
}

// moveColumn emits CHA, which sets an absolute 1-based column.
func moveColumn(sb *strings.Builder, col int) {
	fmt.Fprintf(sb, "\x1b[%dG", col+1)
}
