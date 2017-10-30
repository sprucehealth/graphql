package gqldecode

import (
	"testing"

	"github.com/sprucehealth/backend/libs/test"
)

func TestIsValidPlane0Unicode(t *testing.T) {
	test.Equals(t, true, IsValidPlane0Unicode(`This is a välid string`))
	test.Equals(t, false, IsValidPlane0Unicode(`This is not 😡`))
}
