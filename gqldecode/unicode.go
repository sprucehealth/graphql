package gqldecode

import (
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
