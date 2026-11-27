package miniprompt

import (
	"slices"
	"unicode"
)

// editor holds the line being typed and the cursor position.
//
// The selection is the range between anchor and cursor.
// anchor == cursor means there is no selection.
type editor struct {
	buf    []rune
	cursor int
	anchor int
}

// setCursor moves the cursor and drops any selection.
func (e *editor) setCursor(pos int) {
	e.cursor = pos
	e.anchor = pos
}

// selection returns the selected range and whether anything is selected.
func (e *editor) selection() (start, end int, ok bool) {
	if e.anchor == e.cursor {
		return 0, 0, false
	}

	if e.anchor < e.cursor {
		return e.anchor, e.cursor, true
	}

	return e.cursor, e.anchor, true
}

// deleteSelection removes the selection and reports whether it removed anything.
func (e *editor) deleteSelection() bool {
	start, end, ok := e.selection()
	if !ok {
		return false
	}

	e.buf = slices.Delete(e.buf, start, end)
	e.setCursor(start)

	return true
}

// insert inserts one rune, replacing the selection.
func (e *editor) insert(r rune) {
	e.deleteSelection()

	e.buf = slices.Insert(e.buf, e.cursor, r)
	e.setCursor(e.cursor + 1)
}

// insertText inserts a whole string, used for paste.
func (e *editor) insertText(s string) {
	e.deleteSelection()

	rs := []rune(s)
	e.buf = slices.Insert(e.buf, e.cursor, rs...)
	e.setCursor(e.cursor + len(rs))
}

// backspace removes the rune before the cursor, or the selection.
func (e *editor) backspace() {
	if e.deleteSelection() || e.cursor == 0 {
		return
	}

	e.buf = slices.Delete(e.buf, e.cursor-1, e.cursor)
	e.setCursor(e.cursor - 1)
}

// delete removes the rune after the cursor, or the selection.
func (e *editor) delete() {
	if e.deleteSelection() || e.cursor >= len(e.buf) {
		return
	}

	e.buf = slices.Delete(e.buf, e.cursor, e.cursor+1)
}

// left moves one rune left, or collapses the selection to its start.
func (e *editor) left() {
	if start, _, ok := e.selection(); ok {
		e.setCursor(start)
		return
	}

	if e.cursor > 0 {
		e.setCursor(e.cursor - 1)
	}
}

// right moves one rune right, or collapses the selection to its end.
func (e *editor) right() {
	if _, end, ok := e.selection(); ok {
		e.setCursor(end)
		return
	}

	if e.cursor < len(e.buf) {
		e.setCursor(e.cursor + 1)
	}
}

// home moves the cursor to the start of the line.
func (e *editor) home() {
	e.setCursor(0)
}

// end moves the cursor to the end of the line.
func (e *editor) end() {
	e.setCursor(len(e.buf))
}

// wordLeft jumps to the start of the word before the cursor.
func (e *editor) wordLeft() {
	if start, _, ok := e.selection(); ok {
		e.setCursor(start)
		return
	}

	e.setCursor(e.wordStart(e.cursor))
}

// wordRight jumps to the end of the word after the cursor.
func (e *editor) wordRight() {
	if _, end, ok := e.selection(); ok {
		e.setCursor(end)
		return
	}

	e.setCursor(e.wordEnd(e.cursor))
}

// killWordLeft removes the selection, or the word before the cursor.
func (e *editor) killWordLeft() {
	if e.deleteSelection() {
		return
	}

	start := e.wordStart(e.cursor)
	e.buf = slices.Delete(e.buf, start, e.cursor)
	e.setCursor(start)
}

// killWordRight removes the selection, or the word after the cursor.
func (e *editor) killWordRight() {
	if e.deleteSelection() {
		return
	}

	end := e.wordEnd(e.cursor)
	e.buf = slices.Delete(e.buf, e.cursor, end)
}

// selectAll selects the whole line.
func (e *editor) selectAll() {
	e.anchor = 0
	e.cursor = len(e.buf)
}

// wordStart skips separators then word runes backwards from pos.
func (e *editor) wordStart(pos int) int {
	for pos > 0 && !isWord(e.buf[pos-1]) {
		pos--
	}

	for pos > 0 && isWord(e.buf[pos-1]) {
		pos--
	}

	return pos
}

// wordEnd skips separators then word runes forwards from pos.
func (e *editor) wordEnd(pos int) int {
	for pos < len(e.buf) && !isWord(e.buf[pos]) {
		pos++
	}

	for pos < len(e.buf) && isWord(e.buf[pos]) {
		pos++
	}

	return pos
}

// isWord defines what Ctrl+left, Ctrl+right and Ctrl+w treat as a word.
func isWord(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
