package visitor_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/parser"
	"github.com/sprucehealth/graphql/language/visitor"
	"github.com/sprucehealth/graphql/testutil"
)

func parse(t *testing.T, query string) *ast.Document {
	astDoc, err := parser.Parse(parser.ParseParams{
		Source:  query,
		Options: parser.ParseOptions{},
	})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return astDoc
}

func TestVisitor_AllowsSkippingASubTree(t *testing.T) {
	query := `{ a, b { x }, c }`
	astDoc := parse(t, query)

	visited := []any{}
	expectedVisited := []any{
		[]any{"enter", "Document", nil},
		[]any{"enter", "OperationDefinition", nil},
		[]any{"enter", "SelectionSet", nil},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "a"},
		[]any{"leave", "Name", "a"},
		[]any{"leave", "Field", nil},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "c"},
		[]any{"leave", "Name", "c"},
		[]any{"leave", "Field", nil},
		[]any{"leave", "SelectionSet", nil},
		[]any{"leave", "OperationDefinition", nil},
		[]any{"leave", "Document", nil},
	}

	v := &visitor.VisitorOptions{
		Enter: func(p visitor.VisitFuncParams) (string, any) {
			switch node := p.Node.(type) {
			case *ast.Name:
				visited = append(visited, []any{"enter", kind(node), node.Value})
			case *ast.Field:
				visited = append(visited, []any{"enter", kind(node), nil})
				if node.Name != nil && node.Name.Value == "b" {
					return visitor.ActionSkip, nil
				}
			case ast.Node:
				visited = append(visited, []any{"enter", kind(node), nil})
			default:
				visited = append(visited, []any{"enter", nil, nil})
			}
			return visitor.ActionNoChange, nil
		},
		Leave: func(p visitor.VisitFuncParams) (string, any) {
			switch node := p.Node.(type) {
			case *ast.Name:
				visited = append(visited, []any{"leave", kind(node), node.Value})
			case ast.Node:
				visited = append(visited, []any{"leave", kind(node), nil})
			default:
				visited = append(visited, []any{"leave", nil, nil})
			}
			return visitor.ActionNoChange, nil
		},
	}

	_ = visitor.Visit(astDoc, v)

	if !reflect.DeepEqual(visited, expectedVisited) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expectedVisited, visited))
	}
}

func TestVisitor_AllowsEarlyExitWhileVisiting(t *testing.T) {
	visited := []any{}

	query := `{ a, b { x }, c }`
	astDoc := parse(t, query)

	expectedVisited := []any{
		[]any{"enter", "Document", nil},
		[]any{"enter", "OperationDefinition", nil},
		[]any{"enter", "SelectionSet", nil},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "a"},
		[]any{"leave", "Name", "a"},
		[]any{"leave", "Field", nil},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "b"},
		[]any{"leave", "Name", "b"},
		[]any{"enter", "SelectionSet", nil},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "x"},
	}

	v := &visitor.VisitorOptions{
		Enter: func(p visitor.VisitFuncParams) (string, any) {
			switch node := p.Node.(type) {
			case *ast.Name:
				visited = append(visited, []any{"enter", kind(node), node.Value})
				if node.Value == "x" {
					return visitor.ActionBreak, nil
				}
			case ast.Node:
				visited = append(visited, []any{"enter", kind(node), nil})
			default:
				visited = append(visited, []any{"enter", nil, nil})
			}
			return visitor.ActionNoChange, nil
		},
		Leave: func(p visitor.VisitFuncParams) (string, any) {
			switch node := p.Node.(type) {
			case *ast.Name:
				visited = append(visited, []any{"leave", kind(node), node.Value})
			case ast.Node:
				visited = append(visited, []any{"leave", kind(node), nil})
			default:
				visited = append(visited, []any{"leave", nil, nil})
			}
			return visitor.ActionNoChange, nil
		},
	}

	_ = visitor.Visit(astDoc, v)

	if !reflect.DeepEqual(visited, expectedVisited) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expectedVisited, visited))
	}
}

func TestVisitor_VisitsKitchenSink(t *testing.T) {
	t.Skip("This test seems bad")

	b, err := os.ReadFile("../../kitchen-sink.graphql")
	if err != nil {
		t.Fatalf("unable to load kitchen-sink.graphql")
	}

	query := string(b)
	astDoc := parse(t, query)

	visited := []any{}
	expectedVisited := []any{
		[]any{"enter", "Document", nil},
		[]any{"enter", "OperationDefinition", nil},
		[]any{"enter", "Name", "OperationDefinition"},
		[]any{"leave", "Name", "OperationDefinition"},
		[]any{"enter", "VariableDefinition", nil},
		[]any{"enter", "Variable", "VariableDefinition"},
		[]any{"enter", "Name", "Variable"},
		[]any{"leave", "Name", "Variable"},
		[]any{"leave", "Variable", "VariableDefinition"},
		[]any{"enter", "Named", "VariableDefinition"},
		[]any{"enter", "Name", "Named"},
		[]any{"leave", "Name", "Named"},
		[]any{"leave", "Named", "VariableDefinition"},
		[]any{"leave", "VariableDefinition", nil},
		[]any{"enter", "VariableDefinition", nil},
		[]any{"enter", "Variable", "VariableDefinition"},
		[]any{"enter", "Name", "Variable"},
		[]any{"leave", "Name", "Variable"},
		[]any{"leave", "Variable", "VariableDefinition"},
		[]any{"enter", "Named", "VariableDefinition"},
		[]any{"enter", "Name", "Named"},
		[]any{"leave", "Name", "Named"},
		[]any{"leave", "Named", "VariableDefinition"},
		[]any{"enter", "EnumValue", "VariableDefinition"},
		[]any{"leave", "EnumValue", "VariableDefinition"},
		[]any{"leave", "VariableDefinition", nil},
		[]any{"enter", "SelectionSet", "OperationDefinition"},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"enter", "Argument", nil},
		[]any{"enter", "Name", "Argument"},
		[]any{"leave", "Name", "Argument"},
		[]any{"enter", "ListValue", "Argument"},
		[]any{"enter", "IntValue", nil},
		[]any{"leave", "IntValue", nil},
		[]any{"enter", "IntValue", nil},
		[]any{"leave", "IntValue", nil},
		[]any{"leave", "ListValue", "Argument"},
		[]any{"leave", "Argument", nil},
		[]any{"enter", "SelectionSet", "Field"},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"leave", "Field", nil},
		[]any{"enter", "InlineFragment", nil},
		[]any{"enter", "Named", "InlineFragment"},
		[]any{"enter", "Name", "Named"},
		[]any{"leave", "Name", "Named"},
		[]any{"leave", "Named", "InlineFragment"},
		[]any{"enter", "Directive", nil},
		[]any{"enter", "Name", "Directive"},
		[]any{"leave", "Name", "Directive"},
		[]any{"leave", "Directive", nil},
		[]any{"enter", "SelectionSet", "InlineFragment"},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"enter", "SelectionSet", "Field"},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"leave", "Field", nil},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"enter", "Argument", nil},
		[]any{"enter", "Name", "Argument"},
		[]any{"leave", "Name", "Argument"},
		[]any{"enter", "IntValue", "Argument"},
		[]any{"leave", "IntValue", "Argument"},
		[]any{"leave", "Argument", nil},
		[]any{"enter", "Argument", nil},
		[]any{"enter", "Name", "Argument"},
		[]any{"leave", "Name", "Argument"},
		[]any{"enter", "Variable", "Argument"},
		[]any{"enter", "Name", "Variable"},
		[]any{"leave", "Name", "Variable"},
		[]any{"leave", "Variable", "Argument"},
		[]any{"leave", "Argument", nil},
		[]any{"enter", "Directive", nil},
		[]any{"enter", "Name", "Directive"},
		[]any{"leave", "Name", "Directive"},
		[]any{"enter", "Argument", nil},
		[]any{"enter", "Name", "Argument"},
		[]any{"leave", "Name", "Argument"},
		[]any{"enter", "Variable", "Argument"},
		[]any{"enter", "Name", "Variable"},
		[]any{"leave", "Name", "Variable"},
		[]any{"leave", "Variable", "Argument"},
		[]any{"leave", "Argument", nil},
		[]any{"leave", "Directive", nil},
		[]any{"enter", "SelectionSet", "Field"},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"leave", "Field", nil},
		[]any{"enter", "FragmentSpread", nil},
		[]any{"enter", "Name", "FragmentSpread"},
		[]any{"leave", "Name", "FragmentSpread"},
		[]any{"leave", "FragmentSpread", nil},
		[]any{"leave", "SelectionSet", "Field"},
		[]any{"leave", "Field", nil},
		[]any{"leave", "SelectionSet", "Field"},
		[]any{"leave", "Field", nil},
		[]any{"leave", "SelectionSet", "InlineFragment"},
		[]any{"leave", "InlineFragment", nil},
		[]any{"leave", "SelectionSet", "Field"},
		[]any{"leave", "Field", nil},
		[]any{"leave", "SelectionSet", "OperationDefinition"},
		[]any{"leave", "OperationDefinition", nil},
		[]any{"enter", "OperationDefinition", nil},
		[]any{"enter", "Name", "OperationDefinition"},
		[]any{"leave", "Name", "OperationDefinition"},
		[]any{"enter", "SelectionSet", "OperationDefinition"},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"enter", "Argument", nil},
		[]any{"enter", "Name", "Argument"},
		[]any{"leave", "Name", "Argument"},
		[]any{"enter", "IntValue", "Argument"},
		[]any{"leave", "IntValue", "Argument"},
		[]any{"leave", "Argument", nil},
		[]any{"enter", "Directive", nil},
		[]any{"enter", "Name", "Directive"},
		[]any{"leave", "Name", "Directive"},
		[]any{"leave", "Directive", nil},
		[]any{"enter", "SelectionSet", "Field"},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"enter", "SelectionSet", "Field"},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"leave", "Field", nil},
		[]any{"leave", "SelectionSet", "Field"},
		[]any{"leave", "Field", nil},
		[]any{"leave", "SelectionSet", "Field"},
		[]any{"leave", "Field", nil},
		[]any{"leave", "SelectionSet", "OperationDefinition"},
		[]any{"leave", "OperationDefinition", nil},
		[]any{"enter", "FragmentDefinition", nil},
		[]any{"enter", "Name", "FragmentDefinition"},
		[]any{"leave", "Name", "FragmentDefinition"},
		[]any{"enter", "Named", "FragmentDefinition"},
		[]any{"enter", "Name", "Named"},
		[]any{"leave", "Name", "Named"},
		[]any{"leave", "Named", "FragmentDefinition"},
		[]any{"enter", "SelectionSet", "FragmentDefinition"},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"enter", "Argument", nil},
		[]any{"enter", "Name", "Argument"},
		[]any{"leave", "Name", "Argument"},
		[]any{"enter", "Variable", "Argument"},
		[]any{"enter", "Name", "Variable"},
		[]any{"leave", "Name", "Variable"},
		[]any{"leave", "Variable", "Argument"},
		[]any{"leave", "Argument", nil},
		[]any{"enter", "Argument", nil},
		[]any{"enter", "Name", "Argument"},
		[]any{"leave", "Name", "Argument"},
		[]any{"enter", "Variable", "Argument"},
		[]any{"enter", "Name", "Variable"},
		[]any{"leave", "Name", "Variable"},
		[]any{"leave", "Variable", "Argument"},
		[]any{"leave", "Argument", nil},
		[]any{"enter", "Argument", nil},
		[]any{"enter", "Name", "Argument"},
		[]any{"leave", "Name", "Argument"},
		[]any{"enter", "ObjectValue", "Argument"},
		[]any{"enter", "ObjectField", nil},
		[]any{"enter", "Name", "ObjectField"},
		[]any{"leave", "Name", "ObjectField"},
		[]any{"enter", "StringValue", "ObjectField"},
		[]any{"leave", "StringValue", "ObjectField"},
		[]any{"leave", "ObjectField", nil},
		[]any{"leave", "ObjectValue", "Argument"},
		[]any{"leave", "Argument", nil},
		[]any{"leave", "Field", nil},
		[]any{"leave", "SelectionSet", "FragmentDefinition"},
		[]any{"leave", "FragmentDefinition", nil},
		[]any{"enter", "OperationDefinition", nil},
		[]any{"enter", "SelectionSet", "OperationDefinition"},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"enter", "Argument", nil},
		[]any{"enter", "Name", "Argument"},
		[]any{"leave", "Name", "Argument"},
		[]any{"enter", "BooleanValue", "Argument"},
		[]any{"leave", "BooleanValue", "Argument"},
		[]any{"leave", "Argument", nil},
		[]any{"enter", "Argument", nil},
		[]any{"enter", "Name", "Argument"},
		[]any{"leave", "Name", "Argument"},
		[]any{"enter", "BooleanValue", "Argument"},
		[]any{"leave", "BooleanValue", "Argument"},
		[]any{"leave", "Argument", nil},
		[]any{"leave", "Field", nil},
		[]any{"enter", "Field", nil},
		[]any{"enter", "Name", "Field"},
		[]any{"leave", "Name", "Field"},
		[]any{"leave", "Field", nil},
		[]any{"leave", "SelectionSet", "OperationDefinition"},
		[]any{"leave", "OperationDefinition", nil},
		[]any{"leave", "Document", nil},
	}

	v := &visitor.VisitorOptions{
		Enter: func(p visitor.VisitFuncParams) (string, any) {
			switch node := p.Node.(type) {
			case ast.Node:
				if p.Parent != nil {
					visited = append(visited, []any{"enter", kind(node), kind(p.Parent)})
				} else {
					visited = append(visited, []any{"enter", kind(node), nil})
				}
			}
			return visitor.ActionNoChange, nil
		},
		Leave: func(p visitor.VisitFuncParams) (string, any) {
			switch node := p.Node.(type) {
			case ast.Node:
				if p.Parent != nil {
					visited = append(visited, []any{"leave", kind(node), kind(p.Parent)})
				} else {
					visited = append(visited, []any{"leave", kind(node), nil})
				}
			}
			return visitor.ActionNoChange, nil
		},
	}

	_ = visitor.Visit(astDoc, v)

	if !reflect.DeepEqual(visited, expectedVisited) {
		for i, v := range visited {
			if !reflect.DeepEqual(v, expectedVisited[i]) {
				t.Logf("%d    %v != %v", i, v, expectedVisited[i])
				break
			}
		}
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expectedVisited, visited))
	}
}

func kind(v any) string {
	return reflect.TypeOf(v).String()[5:]
}
