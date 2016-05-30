package printer

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/sprucehealth/graphql/language/ast"
)

func join(str []string, sep string) string {
	ss := make([]string, 0, len(str))
	// filter out empty strings
	for _, s := range str {
		if s == "" {
			continue
		}
		ss = append(ss, s)
	}
	return strings.Join(ss, sep)
}

func wrap(start, maybeString, end string) string {
	if maybeString == "" {
		return maybeString
	}
	return start + maybeString + end
}
func block(sl []string) string {
	if len(sl) == 0 {
		return ""
	}
	return indent("{\n"+join(sl, "\n")) + "\n}"
}

func indent(s string) string {
	return strings.Replace(s, "\n", "\n  ", -1)
}

type walker struct {
}

func (w *walker) walkASTSlice(sl interface{}) []string {
	v := reflect.ValueOf(sl)
	n := v.Len()
	strs := make([]string, 0, n)
	for i := 0; i < n; i++ {
		s := w.walkAST(v.Index(i).Interface().(ast.Node))
		if s != "" {
			strs = append(strs, s)
		}
	}
	return strs
}

func (w *walker) walkASTSliceAndJoin(sl interface{}, sep string) string {
	strs := w.walkASTSlice(sl)
	return strings.Join(strs, sep)
}

func (w *walker) walkASTSliceAndBlock(sl interface{}) string {
	strs := w.walkASTSlice(sl)
	return block(strs)
}

func (w *walker) walkAST(root ast.Node) string {
	if root == nil {
		return ""
	}

	switch node := root.(type) {
	case *ast.Name:
		if node == nil {
			return ""
		}
		return node.Value
	case *ast.Variable:
		return "$" + node.Name.Value
	case *ast.Document:
		return w.walkASTSliceAndJoin(node.Definitions, "\n\n") + "\n"
	case *ast.OperationDefinition:
		name := w.walkAST(node.Name)
		selectionSet := w.walkAST(node.SelectionSet)
		varDefs := wrap("(", w.walkASTSliceAndJoin(node.VariableDefinitions, ", "), ")")
		directives := w.walkASTSliceAndJoin(node.Directives, " ")
		if name == "" && directives == "" && varDefs == "" && node.Operation == "query" {
			return selectionSet
		}
		return join([]string{
			node.Operation,
			join([]string{name, varDefs}, ""),
			directives,
			selectionSet,
		}, " ")
	case *ast.VariableDefinition:
		variable := w.walkAST(node.Variable)
		ttype := w.walkAST(node.Type)
		defaultValue := w.walkAST(node.DefaultValue)
		return variable + ": " + ttype + wrap(" = ", defaultValue, "")
	case *ast.SelectionSet:
		if node == nil {
			return ""
		}
		return w.walkASTSliceAndBlock(node.Selections)
	case *ast.Field:
		alias := w.walkAST(node.Alias)
		name := w.walkAST(node.Name)
		args := w.walkASTSliceAndJoin(node.Arguments, ", ")
		directives := w.walkASTSliceAndJoin(node.Directives, " ")
		selectionSet := w.walkAST(node.SelectionSet)
		return join(
			[]string{
				wrap("", alias, ": ") + name + wrap("(", args, ")"),
				directives,
				selectionSet,
			},
			" ")
	case *ast.Argument:
		name := w.walkAST(node.Name)
		value := w.walkAST(node.Value)
		return name + ": " + value
	case *ast.FragmentSpread:
		name := w.walkAST(node.Name)
		directives := w.walkASTSliceAndJoin(node.Directives, " ")
		return "..." + name + wrap(" ", directives, "")
	case *ast.InlineFragment:
		typeCondition := w.walkAST(node.TypeCondition)
		directives := w.walkASTSliceAndJoin(node.Directives, " ")
		selectionSet := w.walkAST(node.SelectionSet)
		if typeCondition == "" {
			return "... " + wrap("", directives, " ") + selectionSet
		} else {
			return "... on " + typeCondition + " " + wrap("", directives, " ") + selectionSet
		}
	case *ast.FragmentDefinition:
		name := w.walkAST(node.Name)
		typeCondition := w.walkAST(node.TypeCondition)
		directives := w.walkASTSliceAndJoin(node.Directives, " ")
		selectionSet := w.walkAST(node.SelectionSet)
		return "fragment " + name + " on " + typeCondition + " " + wrap("", directives, " ") + selectionSet
	case *ast.IntValue:
		return node.Value
	case *ast.FloatValue:
		return node.Value
	case *ast.StringValue:
		return strconv.Quote(node.Value)
	case *ast.BooleanValue:
		return strconv.FormatBool(node.Value)
	case *ast.EnumValue:
		return node.Value
	case *ast.ListValue:
		return "[" + w.walkASTSliceAndJoin(node.Values, ", ") + "]"
	case *ast.ObjectValue:
		return "{" + w.walkASTSliceAndJoin(node.Fields, ", ") + "}"
	case *ast.ObjectField:
		name := w.walkAST(node.Name)
		value := w.walkAST(node.Value)
		return name + ": " + value
	case *ast.Directive:
		name := w.walkAST(node.Name)
		args := w.walkASTSliceAndJoin(node.Arguments, ", ")
		return "@" + name + wrap("(", args, ")")
	case *ast.Named:
		if node == nil {
			return ""
		}
		return w.walkAST(node.Name)
	case *ast.List:
		return "[" + w.walkAST(node.Type) + "]"
	case *ast.NonNull:
		return w.walkAST(node.Type) + "!"
	case *ast.ScalarDefinition:
		name := w.walkAST(node.Name)
		return "scalar " + name
	case *ast.ObjectDefinition:
		name := w.walkAST(node.Name)
		interfaces := w.walkASTSliceAndJoin(node.Interfaces, ", ")
		fields := w.walkASTSliceAndBlock(node.Fields)
		return joinComments(node.Doc, "", "\n") + "type " + name + " " + wrap("implements ", interfaces, " ") + fields
	case *ast.FieldDefinition:
		name := w.walkAST(node.Name)
		ttype := w.walkAST(node.Type)
		args := w.walkASTSliceAndJoin(node.Arguments, ", ")
		return joinComments(node.Doc, "", "\n") + name + wrap("(", args, ")") + ": " + ttype + joinComments(node.Comment, " ", "")
	case *ast.InputValueDefinition:
		name := w.walkAST(node.Name)
		ttype := w.walkAST(node.Type)
		defaultValue := w.walkAST(node.DefaultValue)
		return joinComments(node.Doc, "", "\n") + name + ": " + ttype + wrap(" = ", defaultValue, "") + joinComments(node.Comment, " ", "")
	case *ast.InterfaceDefinition:
		name := w.walkAST(node.Name)
		fields := w.walkASTSliceAndBlock(node.Fields)
		return joinComments(node.Doc, "", "\n") + "interface " + name + " " + fields
	case *ast.UnionDefinition:
		name := w.walkAST(node.Name)
		types := w.walkASTSliceAndJoin(node.Types, " | ")
		return joinComments(node.Doc, "", "\n") + "union " + name + " = " + types + joinComments(node.Comment, " ", "")
	case *ast.EnumDefinition:
		name := w.walkAST(node.Name)
		values := w.walkASTSliceAndBlock(node.Values)
		return joinComments(node.Doc, "", "\n") + "enum " + name + " " + values
	case *ast.EnumValueDefinition:
		return joinComments(node.Doc, "", "\n") + w.walkAST(node.Name) + joinComments(node.Comment, " ", "")
	case *ast.InputObjectDefinition:
		name := w.walkAST(node.Name)
		fields := w.walkASTSliceAndBlock(node.Fields)
		return joinComments(node.Doc, "", "\n") + "input " + name + " " + fields
	case *ast.TypeExtensionDefinition:
		return "extend " + w.walkAST(node.Definition)
	case *ast.CommentGroup:
		lines := make([]string, len(node.List))
		for i, c := range node.List {
			lines[i] = c.Text
		}
		fmt.Printf("%v\n", lines)
		return strings.Join(lines, "\n")
	case *ast.DirectiveDefinition:
		name := w.walkAST(node.Name)
		args := wrap("(", w.walkASTSliceAndJoin(node.Arguments, ", "), ")")
		return fmt.Sprintf("directive @%v%v on %v", name, args, w.walkASTSliceAndJoin(node.Locations, " | "))
	case ast.Type:
		return node.String()
	case ast.Value:
		return fmt.Sprintf("%v", node.GetValue())
	}
	return fmt.Sprintf("[Unknown node type %T]", root)
}

func joinComments(cg *ast.CommentGroup, prefix, suffix string) string {
	if cg == nil {
		return ""
	}
	lines := make([]string, len(cg.List))
	for i, c := range cg.List {
		lines[i] = c.Text
	}
	return prefix + strings.Join(lines, "\n") + suffix
}

func Print(node ast.Node) string {
	return (&walker{}).walkAST(node)
}
