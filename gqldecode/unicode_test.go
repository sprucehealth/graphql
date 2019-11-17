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
