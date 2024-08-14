package graphql

import (
	"errors"

	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
)

func NewLocatedError(err any, nodes []ast.Node) *gqlerrors.Error {
	message := "An unknown error occurred."
	var origError error
	switch err := err.(type) {
	case error:
		message = err.Error()
		origError = err
	case string:
		message = err
		origError = errors.New(err)
	}
	stack := message
	return gqlerrors.NewError(
		gqlerrors.ErrorTypeInternal, // TODO
		message,
		nodes,
		stack,
		nil,
		[]int{},
		origError,
	)
}

func FieldASTsToNodeASTs(fieldASTs []*ast.Field) []ast.Node {
	nodes := make([]ast.Node, len(fieldASTs))
	for i, fieldAST := range fieldASTs {
		nodes[i] = fieldAST
	}
	return nodes
}
