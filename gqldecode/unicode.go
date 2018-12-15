package gqldecode

import (
	"strings"
	"unicode/utf8"
)

// IsValidPlane0Unicode returns true iff the provided string only has valid plane 0 unicode (no emoji)
func IsValidPlane0Unicode(s string) bool {
	for _, r := range s {
		if !utf8.ValidRune(r) {
			return false
		}
		if utf8.RuneLen(r) > 3 {
			return false
		}
	}
	return true
}

var unicodeSanitizeReplacer = strings.NewReplacer(
	string('\u200B'), "", // zero-width space
	string('\uFEFF'), "", // zero-width no-break space
	string('\u200D'), "", // zero-width joiner
	string('\u200C'), "", // zero-width non-joiner
)

func sanitizeUnicode(s string) string {
	return unicodeSanitizeReplacer.Replace(s)
}
