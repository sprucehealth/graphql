package gqldecode

import (
	"testing"
)

func TestIsValidPlane0Unicode(t *testing.T) {
	if !IsValidPlane0Unicode(`This is a välid string`) {
		t.Fail()
	}
	if IsValidPlane0Unicode(`This is not 😡`) {
		t.Fail()
	}
}

func TestSanitizeUnicode(t *testing.T) {
	cases := map[string]string{
		"foo":    "foo",
		"🤦‍♀️":   "🤦‍♀️",
		"\uFEFF": "", // zero-width no-break space
	}
	for in, exp := range cases {
		if out := sanitizeUnicode(in); out != exp {
			t.Errorf("sanitizeUnicode(%q) = %q, expected %q", in, out, exp)
		}
	}
}
