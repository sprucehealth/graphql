package graphql

import (
	"context"

	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
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
	RootObject map[string]any

	// VariableValues is a mapping of variable name to runtime value to use for all variables
	// defined in the requestString.
	VariableValues map[string]any

	// OperationName is the name of the operation to use if requestString contains multiple
	// possible operations. Can be omitted if requestString contains only
	// one operation.
	OperationName string
}

func Do(ctx context.Context, p Params) *Result {
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

	return Execute(ctx, ExecuteParams{
		Schema:        p.Schema,
		Root:          p.RootObject,
		AST:           ast,
		OperationName: p.OperationName,
		Args:          p.VariableValues,
	})
}

// RequestTypeNames rewrites an ast document to include __typename
// in all selection sets.
func RequestTypeNames(doc *ast.Document) {
	for _, node := range doc.Definitions {
		switch od := node.(type) {
		case *ast.OperationDefinition:
			requestTypeNamesInSelectionSet(od.SelectionSet)
		}
	}
}

func requestTypeNamesInSelectionSet(ss *ast.SelectionSet) {
	if ss == nil {
		return
	}
	ss.Selections = append(ss.Selections, &ast.Field{
		Name: &ast.Name{
			Value: "__typename",
		},
	})
	for _, selections := range ss.Selections {
		requestTypeNamesInSelectionSet(selections.GetSelectionSet())
	}
}
