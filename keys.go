package miniprompt

// This file decodes raw terminal bytes into semantic events.
// It performs no I/O and knows nothing about rendering.
//
// One Read may end in the middle of a UTF-8 rune or an escape sequence, so
// the decoder keeps unfinished bytes in pending and retries them after the
// next chunk. A sequence is complete when it reaches a "final byte"
// (0x40-0x7e); until then it is reported as incomplete.

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
	evTab
	evBackspace
	evDelete
	evLeft
	evRight
	evUp
	evDown
	evWordLeft  // Ctrl+left
	evWordRight // Ctrl+right
	evHome
	evEnd
	evKillWordLeft  // Ctrl+w
	evKillWordRight // Ctrl+del
	evSelectAll     // Ctrl+a
	evInterrupt     // Ctrl+c
	evPaste         // Bracketed paste
)

// event is a decoded input event.
// Only the field relevant to kind is populated.
type event struct {
	kind eventKind
	r    rune
	text string
}

// Bracketed paste markers. The terminal wraps pasted text between them.
const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

var (
	pasteStartBytes = []byte(pasteStart)
	pasteEndBytes   = []byte(pasteEnd)
)

// escapeSeqs maps known ANSI sequences to events.
// Unknown CSI and SS3 sequences are consumed as evUnknown so their bytes do
// not become text.
var escapeSeqs = map[string]eventKind{
	// CSI sequences
	"\x1b[A":    evUp,
	"\x1b[B":    evDown,
	"\x1b[C":    evRight,
	"\x1b[D":    evLeft,
	"\x1b[H":    evHome,
	"\x1b[F":    evEnd,
	"\x1b[1~":   evHome,
	"\x1b[4~":   evEnd,
	"\x1b[7~":   evHome,
	"\x1b[8~":   evEnd,
	"\x1b[3~":   evDelete,
	"\x1b[3;5~": evKillWordRight,
	"\x1b[1;5C": evWordRight,
	"\x1b[1;5D": evWordLeft,
	"\x1b[5C":   evWordRight,
	"\x1b[5D":   evWordLeft,

	// SS3 sequences, used by terminals in application cursor mode
	"\x1bOA": evUp,
	"\x1bOB": evDown,
	"\x1bOC": evRight,
	"\x1bOD": evLeft,
	"\x1bOH": evHome,
	"\x1bOF": evEnd,
}

// ASCII and control byte values shared by the decoder and the renderer.
const (
	byteCtrlA = 0x01
	byteCtrlC = 0x03
	byteCtrlD = 0x04
	byteCtrlH = 0x08
	byteCtrlW = 0x17
	byteTab   = 0x09
	byteEsc   = 0x1b
	byteDel   = 0x7f
	byteSpace = 0x20

	// A CSI sequence ends with a byte in this range.
	csiFinalMin = 0x40
	csiFinalMax = 0x7e
)

// keyDecoder turns raw terminal bytes into events.
//
// One Read can contain a partial UTF-8 rune, a partial escape sequence,
// or several keys at once, so the decoder keeps unfinished bytes.
type keyDecoder struct {
	// pending holds bytes that do not form a complete key yet.
	pending []byte

	// pasting is true between pasteStart and pasteEnd.
	pasting bool
}

// decode appends chunk to pending and returns every complete event it can
// decode.
func (d *keyDecoder) decode(chunk []byte) []event {
	d.pending = append(d.pending, chunk...)

	var events []event

	for len(d.pending) > 0 {
		if d.pasting {
			ev, ok := d.readPaste()
			if !ok {
				break
			}

			events = append(events, ev)

			continue
		}

		// The paste marker is decoder control, not a key event.
		if bytes.HasPrefix(d.pending, pasteStartBytes) {
			d.pending = d.pending[len(pasteStart):]
			d.pasting = true

			continue
		}

		ev, n, incomplete := decodeKey(d.pending)
		if incomplete {
			break
		}

		d.pending = d.pending[n:]
		events = append(events, ev)
	}

	// Drop the backing slice when idle.
	if len(d.pending) == 0 {
		d.pending = nil
	}

	return events
}

// readPaste returns one evPaste if the paste end marker is present.
//
// Each call scans the whole pending buffer, so a very large paste is
// quadratic; pastes are small in practice.
func (d *keyDecoder) readPaste() (event, bool) {
	idx := bytes.Index(d.pending, pasteEndBytes)
	if idx < 0 {
		return event{}, false
	}

	// Keep bytes after pasteEnd for normal decoding.
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

// decodeKey decodes the first key in b, which must not be empty.
// n is the number of bytes consumed; incomplete means b holds only part of a
// key and should be retried with more bytes.
func decodeKey(b []byte) (event, int, bool) {
	switch b[0] {
	case '\r', '\n':
		return event{kind: evEnter}, 1, false
	case byteTab:
		return event{kind: evTab}, 1, false
	case byteCtrlH, byteDel:
		return event{kind: evBackspace}, 1, false

	// Control keys with dedicated events.
	case byteCtrlC:
		return event{kind: evInterrupt}, 1, false
	case byteCtrlA:
		return event{kind: evSelectAll}, 1, false
	case byteCtrlW:
		return event{kind: evKillWordLeft}, 1, false

	// Ctrl+d is ignored; only Ctrl+c cancels.
	case byteCtrlD:
		return event{kind: evUnknown}, 1, false

	case byteEsc:
		return decodeEscape(b)
	}

	// Ignore other control bytes.
	if b[0] < byteSpace {
		return event{kind: evUnknown}, 1, false
	}

	// Printable ASCII.
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

	// RuneError with size 1 means either a truncated rune or invalid bytes.
	// It is truncated only when the bytes cannot yet be a full rune and there
	// is still room to add more.
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
	}

	// ESC followed by an ordinary byte: drop the ESC and let the next
	// iteration decode the byte normally.
	return event{kind: evUnknown}, 1, false
}

// decodeCSI decodes ESC [ ... final.
func decodeCSI(b []byte) (event, int, bool) {
	for i := 2; i < len(b); i++ {
		if !isCSIFinal(b[i]) {
			continue
		}

		seq := b[:i+1]
		if kind, ok := escapeSeqs[string(seq)]; ok {
			return event{kind: kind}, i + 1, false
		}

		return event{kind: evUnknown}, i + 1, false
	}

	// Incomplete sequence: wait for more bytes.
	return event{}, 0, true
}

// decodeSS3 decodes fixed three-byte ESC O X sequences.
func decodeSS3(b []byte) (event, int, bool) {
	if len(b) < 3 {
		return event{}, 0, true
	}

	if kind, ok := escapeSeqs[string(b[:3])]; ok {
		return event{kind: kind}, 3, false
	}

	return event{kind: evUnknown}, 3, false
}

func isCSIFinal(b byte) bool {
	return b >= csiFinalMin && b <= csiFinalMax
}

// normalizePaste flattens pasted text to one line
// and removes control bytes.
func normalizePaste(b []byte) string {
	var sb strings.Builder
	sb.Grow(len(b))

	for i := 0; i < len(b); i++ {
		switch {
		// CRLF becomes one space.
		case b[i] == '\r':
			if i+1 < len(b) && b[i+1] == '\n' {
				i++
			}

			sb.WriteByte(' ')

		// Other line breaks and tabs become spaces.
		case b[i] == '\n', b[i] == '\t':
			sb.WriteByte(' ')

		// Drop other control bytes.
		case b[i] < byteSpace, b[i] == byteDel:

		default:
			sb.WriteByte(b[i])
		}
	}

	// Remove invalid UTF-8.
	return strings.ToValidUTF8(sb.String(), "")
}
