package graphql

import (
	"context"

	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/parser"
	"github.com/sprucehealth/graphql/language/source"
)

type Params struct {
	// Schema is the GraphQL type system to use when validating and executing a query.
	Schema Schema

	// RequestString is a GraphQL language formatted string representing the requested operation.
	RequestString string

	// RootObject is the value provided as the first argument to resolver functions on the top
	// level type (e.g. the query object type).
	RootObject map[string]interface{}

	// VariableValues is a mapping of variable name to runtime value to use for all variables
	// defined in the requestString.
	VariableValues map[string]interface{}

	// OperationName is the name of the operation to use if requestString contains multiple
	// possible operations. Can be omitted if requestString contains only
	// one operation.
	OperationName string

	// Context may be provided to pass application-specific per-request
	// information to resolve functions.
	Context context.Context
}

func Do(p Params) *Result {
	source := source.New("GraphQL request", p.RequestString)
	ast, err := parser.Parse(parser.ParseParams{Source: source})
	if err != nil {
		return &Result{
			Errors: gqlerrors.FormatErrors(err),
		}
	}
	validationResult := ValidateDocument(&p.Schema, ast, nil)

	if !validationResult.IsValid {
		return &Result{
			Errors: validationResult.Errors,
		}
	}

	return Execute(ExecuteParams{
		Schema:        p.Schema,
		Root:          p.RootObject,
		AST:           ast,
		OperationName: p.OperationName,
		Args:          p.VariableValues,
		Context:       p.Context,
	})
}
