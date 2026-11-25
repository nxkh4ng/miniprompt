package miniprompt

// This file decodes raw terminal bytes into semantic events.
// It performs no I/O and knows nothing about rendering.

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

type eventKind uint8

const (
	evUnknown eventKind = iota
	evRune
	evEnter
	evBackspace
	evDelete
	evLeft
	evRight
	evWordLeft  // Ctrl+left
	evWordRight // Ctrl+right
	evHome
	evEnd
	evKillWord   // Ctrl+w
	evSelectAll  // Ctrl+a
	evInterrupt  // Ctrl+c
	evPaste      // Bracketed paste
	evPasteStart // Internal: switches the decoder into paste mode
)

// event is a decoded input event.
// Only the field relevant to kind is populated.
type event struct {
	kind eventKind
	r    rune
	text string
}

// Bracketed paste markers.
// The terminal wraps pasted text between these 2 sequences.
const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

// escapeSeqs maps common ANSI sequences to events.
// Unknown CSI or SS3 sequences are consumed as evUnknown.
var escapeSeqs = [...]struct {
	seq  string
	kind eventKind
}{
	// CSI sequences
	{"\x1b[A", evUnknown},
	{"\x1b[B", evUnknown},
	{"\x1b[C", evRight},
	{"\x1b[D", evLeft},
	{"\x1b[H", evHome},
	{"\x1b[F", evEnd},
	{"\x1b[1~", evHome},
	{"\x1b[4~", evEnd},
	{"\x1b[7~", evHome},
	{"\x1b[8~", evEnd},
	{"\x1b[3~", evDelete},
	{"\x1b[1;5C", evWordRight},
	{"\x1b[1;5D", evWordLeft},
	{"\x1b[5C", evWordRight},
	{"\x1b[5D", evWordLeft},

	// SS3 sequences, used by terminal in application cursor mode
	{"\x1bOA", evUnknown},
	{"\x1bOB", evUnknown},
	{"\x1bOC", evRight},
	{"\x1bOD", evLeft},
	{"\x1bOH", evHome},
	{"\x1bOF", evEnd},
}

// Control bytes and ASCII boundaries.
const (
	byteCtrlA = 0x01
	byteCtrlC = 0x03
	byteCtrlD = 0x04
	byteCtrlH = 0x08
	byteCtrlW = 0x17
	byteEsc   = 0x1b
	byteDel   = 0x7f
	byteSpace = 0x20

	// CSI sequences end with a byte in this range
	byteCsiFinalMin = 0x40
	byteCsiFinalMax = 0x7e
)

// keyDecoder turns raw terminal bytes into events.
//
// One Read can contain a partial UTF-8 rune, a partial escape sequence,
// or several keys at once, so the decoder keeps unfinished bytes.
type keyDecoder struct {
	// pending holds bytes that do not form a complete key set.
	pending []byte

	// pasting is true between pasteStart and pasteEnd.
	pasting bool
}

// decode appends chunk to pending and
// returns every complete event it can decode.
func (d *keyDecoder) decode(chunk []byte) []event {
	d.pending = append(d.pending, chunk...)

	var events []event
	for len(d.pending) > 0 {
		// Paste mode: consume everything up to pasteEnd
		if d.pasting {
			ev, ok := d.readPaste()
			if !ok {
				break
			}
			events = append(events, ev)
			continue
		}

		// Decode one key
		// Keep the rest for the next iteration
		ev, n, incomplete := decodeOne(d.pending)
		if incomplete {
			break
		}

		d.pending = d.pending[n:]

		// Paste start is decoder control, not an editor event
		if ev.kind == evPasteStart {
			d.pasting = true
			continue
		}

		events = append(events, ev)
	}

	// Drop the backing slice when idle
	if len(d.pending) == 0 {
		d.pending = nil
	}

	return events
}

// readPaste returns 1 evPaste if pasteEnd is present.
func (d *keyDecoder) readPaste() (event, bool) {
	idx := bytes.Index(d.pending, []byte(pasteEnd))
	if idx < 0 {
		return event{}, false
	}

	// Keep bytes after pasteEnd for normal decoding
	text := normalizePaste(d.pending[:idx])
	d.pending = d.pending[idx+len(pasteEnd):]
	d.pasting = false

	return event{kind: evPaste, text: text}, true
}

// flush is called when Read reaches EOF.
// A paste left open still returns the bytes received so far.
func (d *keyDecoder) flush() []event {
	var events []event

	if d.pasting && len(d.pending) > 0 {
		events = append(events, event{
			kind: evPaste,
			text: normalizePaste(d.pending),
		})
	}

	d.pending = nil
	d.pasting = false

	return events
}

// decodeOne decodes the first key in b.
// n is the number of bytes consumed.
// incomplete means b holds only part of a key.
func decodeOne(b []byte) (event, int, bool) {
	switch b[0] {
	case '\r', '\n':
		return event{kind: evEnter}, 1, false
	case byteCtrlH, byteDel:
		return event{kind: evBackspace}, 1, false

	// Control keys with dedicated events
	case byteCtrlC:
		return event{kind: evInterrupt}, 1, false
	case byteCtrlA:
		return event{kind: evSelectAll}, 1, false
	case byteCtrlW:
		return event{kind: evKillWord}, 1, false

	// Ctrl+d is ignored; only Ctrl+c cancels
	case byteCtrlD:
		return event{kind: evUnknown}, 1, false

	// Escape sequences need their own parser
	case byteEsc:
		return decodeEscape(b)
	}

	// Ignore other control bytes
	if b[0] < byteSpace {
		return event{kind: evUnknown}, 1, false
	}

	// Printable ASCII
	if b[0] < byteDel {
		return event{kind: evRune, r: rune(b[0])}, 1, false
	}

	return decodeUTF8(b)
}

// decodeUTF8 decodes one UTF-8 rune.
// A truncated rune is reported as incomplete and retried later.
func decodeUTF8(b []byte) (event, int, bool) {
	r, size := utf8.DecodeRune(b)
	if r != utf8.RuneError || size != 1 {
		return event{kind: evRune, r: r}, size, false
	}

	// RuneError + size 1 means either a truncated run or invalid bytes
	if !utf8.FullRune(b) && len(b) < utf8.UTFMax {
		return event{}, 0, true
	}

	return event{kind: evUnknown}, 1, false
}

// decodeEscape dispatches sequences that start with ESC.
func decodeEscape(b []byte) (event, int, bool) {
	if len(b) == 1 {
		return event{}, 0, true
	}

	switch b[1] {
	case '[':
		return decodeCSI(b)
	case 'O':
		return decodeSS3(b)

	// ESC + normal byte
	// Drop ESC and decode the byte normally
	default:
		return event{kind: evUnknown}, 1, false
	}
}

// decodeCSI decodes ESC [ ... final.
// CSI sequences end with a byte in the range 0x40-0x7e.
func decodeCSI(b []byte) (event, int, bool) {
	for i := 2; i < len(b); i++ {
		if b[i] < byteCsiFinalMin || b[i] > byteCsiFinalMax {
			continue
		}

		seq := string(b[:i+1])

		// Paste start is handled by the decoder, not the editor
		if seq == pasteStart {
			return event{kind: evPasteStart}, i + 1, false
		}

		if kind, ok := escapeKind(seq); ok {
			return event{kind: kind}, i + 1, false
		}

		// Consumed unknown sequences so their bytes do not become text
		return event{kind: evUnknown}, i + 1, false
	}

	// Incomplete sequence wait for more bytes
	return event{}, 0, true
}

// decodeSS3 decodes fixed three-byte ESC O X sequences.
func decodeSS3(b []byte) (event, int, bool) {
	if len(b) < 3 {
		return event{}, 0, true
	}

	seq := string(b[:3])
	if kind, ok := escapeKind(seq); ok {
		return event{kind: kind}, 3, false
	}

	return event{kind: evUnknown}, 3, false
}

// escapeKind looks up a known ANSI sequence.
func escapeKind(seq string) (eventKind, bool) {
	for _, known := range escapeSeqs {
		if known.seq == seq {
			return known.kind, true
		}
	}

	return evUnknown, false
}

// normalizePaste flattens pasted text to one line
// and removes control bytes.
func normalizePaste(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))

	for i := 0; i < len(b); i++ {
		switch {
		// CRLF becomes one space
		case b[i] == '\r':
			if i+1 < len(b) && b[i+1] == '\n' {
				i++
			}
			sb.WriteByte(' ')

		// Other line breaks and tabs becomes spaces
		case b[i] == '\n', b[i] == '\t':
			sb.WriteByte(' ')

		// Drop other control bytes
		case b[i] < byteSpace, b[i] == byteDel:
		default:
			sb.WriteByte(b[i])
		}
	}

	// Remove invalid UTF-8
	return strings.ToValidUTF8(sb.String(), "")
}
