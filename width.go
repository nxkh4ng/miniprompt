package miniprompt

import (
	"slices"
	"strings"
	"unicode"
)

// displayWidth returns how many terminal cells s occupies.
// Pass plain text, not text that contains ANSI escapes.
//
// Width is summed per rune, so two limits follow: a ZWJ emoji sequence such
// as "👨‍👩‍👧" reports 6 cells where a terminal draws 2, and a tab reports 0.
// Callers never pass tabs, but a tab in a label will misplace the cursor.
func displayWidth(s string) int {
	n := 0
	for _, r := range s {
		n += runeWidth(r)
	}

	return n
}

// runeWidth returns how many terminal cells r occupies.
//
// Two-cell runes come from wideRanges, built with EastAsianWidth=false, so
// ambiguous runes count as one cell. Zero width covers control, format and
// non-spacing marks; spacing combining marks (Mc) count as one cell because
// a terminal advances the cursor for them.
func runeWidth(r rune) int {
	switch {
	case isZeroWidth(r):
		return 0
	case isWide(r):
		return 2
	}

	return 1
}

// isZeroWidth reports control chars, format chars and non-spacing marks
// (Mn, Me), which take no cell of their own.
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

// isWide reports whether r falls in one of the two-cell ranges.
func isWide(r rune) bool {
	_, found := slices.BinarySearchFunc(
		wideRanges, r,
		func(rg runeRange, r rune) int {
			switch {
			case rg.hi < r:
				return -1
			case rg.lo > r:
				return 1
			}

			return 0
		},
	)

	return found
}

// cut returns the leading part of s that fits in cols cells.
// s must not contain ANSI escapes.
func cut(s string, cols int) string {
	if cols <= 0 {
		return ""
	}

	if displayWidth(s) <= cols {
		return s
	}

	var sb strings.Builder
	n := 0

	for _, r := range s {
		w := runeWidth(r)
		if n+w > cols {
			break
		}

		sb.WriteRune(r)
		n += w
	}

	return sb.String()
}
