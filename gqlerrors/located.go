package gqlerrors

import (
	"github.com/sprucehealth/graphql/language/ast"
)

func FieldASTsToNodeASTs(fieldASTs []*ast.Field) []ast.Node {
	nodes := []ast.Node{}
	for _, fieldAST := range fieldASTs {
		nodes = append(nodes, fieldAST)
	}
	return nodes
}
