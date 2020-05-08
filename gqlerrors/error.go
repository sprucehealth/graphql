package gqlerrors

import (
	"fmt"

	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/location"
	"github.com/sprucehealth/graphql/language/source"
)

// ErrorType is a category type for an error.
type ErrorType string

// Well defined error types
const (
	ErrorTypeBadQuery     ErrorType = "BAD_QUERY"
	ErrorTypeInternal     ErrorType = "INTERNAL"
	ErrorTypeInvalidInput ErrorType = "INVALID_INPUT"
	ErrorTypeSyntax       ErrorType = "SYNTAX"
)

// Error is a structured error.
type Error struct {
	Type          ErrorType
	Message       string
	Stack         string
	Nodes         []ast.Node
	Source        *source.Source
	Positions     []int
	Locations     []location.SourceLocation
	OriginalError error
}

// Error implements Golang's built-in `error` interface
func (g Error) Error() string {
	return fmt.Sprintf("%v", g.Message)
}

// NewError returns a new structured error.
func NewError(typ ErrorType, message string, nodes []ast.Node, stack string, source *source.Source, positions []int, origError error) *Error {
	if stack == "" && message != "" {
		stack = message
	}
	if source == nil {
		if len(nodes) != 0 {
			// get source from first node
			node := nodes[0]
			if node.GetLoc().Source != nil {
				source = node.GetLoc().Source
			}
		}
	}
	if len(positions) == 0 && len(nodes) > 0 {
		for _, node := range nodes {
			positions = append(positions, node.GetLoc().Start)
		}
	}
	locations := []location.SourceLocation{}
	for _, pos := range positions {
		loc := location.GetLocation(source, pos)
		locations = append(locations, loc)
	}
	return &Error{
		Type:          typ,
		Message:       message,
		Stack:         stack,
		Nodes:         nodes,
		Source:        source,
		Positions:     positions,
		Locations:     locations,
		OriginalError: origError,
	}
}
