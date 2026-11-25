package miniprompt

import "unicode"

// displayWidth returns how many terminal cells s occupies.
// Pass plain text, not text that contains ANSI escapes.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		n += runeWidth(r)
	}

	return n
}

// runeWidth returns how many terminal cells r occupies.
//
// This is an approximation of the Unicode East Asian Width table:
// it covers control and combining marks (0 cell) and
// the common wide block (2 cells), not every code point.
func runeWidth(r rune) int {
	switch {
	case isZeroWidth(r):
		return 0
	case isWide(r):
		return 2
	}

	return 1
}

// isZeroWidth reports control chars, format chars and
// combining marks, which take no cell of their own.
func isZeroWidth(r rune) bool {
	switch {
	// C0, DEL, C1
	case r < 0x20, r >= 0x7f && r < 0xa0:
		return true

	// Hangul Jamo vowels and trailing consonants
	case r >= 0x1160 && r <= 0x11ff:
		return true
	}

	return unicode.Is(unicode.Mn, r) ||
		unicode.Is(unicode.Me, r) ||
		unicode.Is(unicode.Cf, r)
}

// isWide reports characters drawn in two cells:
// CJK, kana, Hangul syllables, fullwidth forms and emoji.
func isWide(r rune) bool {
	switch {
	// Hangul Jamo leading consonants
	case r >= 0x1100 && r <= 0x115f:
		return true

	// CJK, kana, Yi
	case r >= 0x2e80 && r <= 0xa4cf:
		return true

	// Hangul Jamo Extended-A
	case r >= 0xa960 && r <= 0xa97f:
		return true

	// Hangul syllables
	case r >= 0xac00 && r <= 0xd7a3:
		return true

	// CJK compatibility ideographs
	case r >= 0xf900 && r <= 0xfaff:
		return true

	// vertical forms
	case r >= 0xfe10 && r <= 0xfe19:
		return true

	// CJK compatibility forms
	case r >= 0xfe30 && r <= 0xfe6f:
		return true

	// fullwidth forms
	case r >= 0xff00 && r <= 0xff60:
		return true

	// fullwidth signs
	case r >= 0xffe0 && r <= 0xffe6:
		return true

	// regional indicators, flags
	case r >= 0x1f1e6 && r <= 0x1f1ff:
		return true

	// emoji
	case r >= 0x1f300 && r <= 0x1f9ff:
		return true

	// CJK extensions
	case r >= 0x20000 && r <= 0x3fffd:
		return true
	}

	return false
}
