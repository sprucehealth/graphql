package graphql

import (
	"github.com/sprucehealth/graphql/gqlerrors"
)

// type Schema any

type Result struct {
	Data   any                        `json:"data"`
	Errors []gqlerrors.FormattedError `json:"errors,omitempty"`
}

func (r *Result) HasErrors() bool {
	return (len(r.Errors) > 0)
}
