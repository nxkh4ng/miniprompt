package miniprompt

import (
	"unicode/utf8"
)

// This file decodes raw terminal bytes into semantic events.
// It performs no I/O and knows nothing about rendering.
//
// One Read may end in the middle of a UTF-8 rune or an escape sequence, so
// the decoder keeps unfinished bytes in pending and retries them after the
// next chunk.
//
// A sequence is complete when it reaches a "final byte" (0x40-0x7e),
// SS3 sequences are always 3 bytes; until then it is reported as incomplete.

type eventKind uint8

const (
	evUnknown eventKind = iota
	evRune
	evEnter
	evBackspace
	evDelete
	evHome
	evEnd
	evLeft
	evRight
	evWordLeft
	evWordRight
	evKillWordLeft
	evKillWordRight
	evSelectAll
	evInterrupt
)

// event is a decoded input event.
// r only set when kind is evRune.
type event struct {
	kind eventKind
	r    rune
}

// keySeqs maps known ANSI sequences to event kinds.
// Unknown CSI and SS3 sequences are consumed as evUnknown.
var keySeqs = map[string]eventKind{
	ansiCSI + "C":    evRight,
	ansiCSI + "D":    evLeft,
	ansiCSI + "H":    evHome,
	ansiCSI + "F":    evEnd,
	ansiCSI + "3~":   evDelete,
	ansiCSI + "3;5~": evKillWordRight,
	ansiCSI + "1;5C": evWordRight,
	ansiCSI + "1;5D": evWordLeft,

	ansiSS3 + "C": evRight,
	ansiSS3 + "D": evLeft,
	ansiSS3 + "H": evHome,
	ansiSS3 + "F": evEnd,
}

// decoder turns raw terminal bytes into events,
// keeping unfinished bytes between reads.
type decoder struct {
	// pending holds bytes that do not form a complete key yet.
	pending []byte
}

// decode appends chunk to pending and
// returns every complete event it can decode.
func (d *decoder) decode(chunk []byte) []event {
	d.pending = append(d.pending, chunk...)

	var events []event
	for len(d.pending) > 0 {
		ev, used, needMore := decodeKey(d.pending)
		if needMore {
			break
		}

		d.pending = d.pending[used:]
		events = append(events, ev)
	}

	return events
}

// decodeKey decodes the first key in b, which must not be empty.
// When needMore is false, used is always at least 1.
func decodeKey(b []byte) (ev event, used int, needMore bool) {
	// 0x00            0x20             0x7F  0x80                0xFF
	// |--- control ----||--- printable ---|  |------ not ASCII -----|

	switch b[0] {
	// CR, LF
	case '\r', '\n':
		return event{kind: evEnter}, 1, false

	// Ctrl+H, Del
	case 0x08, 0x7f:
		return event{kind: evBackspace}, 1, false

	// Ctrl+A
	case 0x01:
		return event{kind: evSelectAll}, 1, false

	// Ctrl+W
	case 0x17:
		return event{kind: evKillWordLeft}, 1, false

	// Ctrl+C
	case 0x03:
		return event{kind: evInterrupt}, 1, false

	// ESC
	case 0x1b:
		return decodeEscape(b)
	}

	if b[0] < 0x20 {
		return event{kind: evUnknown}, 1, false
	}
	if b[0] < 0x7f {
		return event{kind: evRune, r: rune(b[0])}, 1, false
	}

	return decodeUTF8(b)
}

// decodeUTF8 decodes 1 UTF-8 rune from b, which must not be empty.
// A truncated rune is reported as incomplete and retries with more bytes.
func decodeUTF8(b []byte) (ev event, used int, needMore bool) {
	r, size := utf8.DecodeRune(b)
	if r != utf8.RuneError || size != 1 {
		return event{kind: evRune, r: r}, size, false
	}

	if !utf8.FullRune(b) && len(b) < utf8.UTFMax {
		return event{}, 0, true
	}

	return event{kind: evUnknown}, 1, false
}

// decodeEscape dispatches sequences that start with ESC.
// An ESC followed by an ordinary byte is dropped; that byte is decoded next.
func decodeEscape(b []byte) (ev event, used int, needMore bool) {
	if len(b) == 1 {
		return event{}, 0, true
	}

	switch b[1] {
	case '[':
		return decodeCSI(b)
	case 'O':
		return decodeSS3(b)
	}

	return event{kind: evUnknown}, 1, false
}

// decodeCSI decodes `ESC + [ + number; + final byte`
func decodeCSI(b []byte) (ev event, used int, needMore bool) {
	for i := 2; i < len(b); i++ {
		if !isCSIFinal(b[i]) {
			continue
		}

		seq := b[:i+1]
		if kind, ok := keySeqs[string(seq)]; ok {
			return event{kind: kind}, i + 1, false
		}

		return event{kind: evUnknown}, i + 1, false
	}

	// Incomplete sequence, wait for more bytes
	return event{}, 0, true
}

// decodeSS3 decodes `ESC + O + final byte`
func decodeSS3(b []byte) (ev event, used int, needMore bool) {
	if len(b) < 3 {
		return event{}, 0, true
	}

	seq := b[:3]
	if kind, ok := keySeqs[string(seq)]; ok {
		return event{kind: kind}, 3, false
	}

	return event{kind: evUnknown}, 3, false
}
