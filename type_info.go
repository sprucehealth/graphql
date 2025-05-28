package graphql

import (
	"github.com/sprucehealth/graphql/language/ast"
)

type fieldDefFn func(schema *Schema, parentType Type, fieldAST *ast.Field) *FieldDefinition

// TODO: can move TypeInfo to a utils package if there ever is one
/**
 * TypeInfo is a utility class which, given a GraphQL schema, can keep track
 * of the current field and type definitions at any point in a GraphQL document
 * AST during a recursive descent by calling `enter(node)` and `leave(node)`.
 */

type TypeInfo struct {
	schema          *Schema
	typeStack       []Output
	parentTypeStack []Composite
	inputTypeStack  []Input
	fieldDefStack   []*FieldDefinition
	directive       *Directive
	argument        *Argument
	getFieldDef     fieldDefFn
}

type TypeInfoConfig struct {
	Schema *Schema

	// NOTE: this experimental optional second parameter is only needed in order
	// to support non-spec-compliant codebases. You should never need to use it.
	// It may disappear in the future.
	FieldDefFn fieldDefFn
}

func NewTypeInfo(opts *TypeInfoConfig) *TypeInfo {
	getFieldDef := opts.FieldDefFn
	if getFieldDef == nil {
		getFieldDef = DefaultTypeInfoFieldDef
	}
	return &TypeInfo{
		schema:      opts.Schema,
		getFieldDef: getFieldDef,
	}
}

func (ti *TypeInfo) Type() Output {
	if len(ti.typeStack) > 0 {
		return ti.typeStack[len(ti.typeStack)-1]
	}
	return nil
}

func (ti *TypeInfo) ParentType() Composite {
	if len(ti.parentTypeStack) > 0 {
		return ti.parentTypeStack[len(ti.parentTypeStack)-1]
	}
	return nil
}

func (ti *TypeInfo) InputType() Input {
	if len(ti.inputTypeStack) > 0 {
		return ti.inputTypeStack[len(ti.inputTypeStack)-1]
	}
	return nil
}
func (ti *TypeInfo) FieldDef() *FieldDefinition {
	if len(ti.fieldDefStack) > 0 {
		return ti.fieldDefStack[len(ti.fieldDefStack)-1]
	}
	return nil
}

func (ti *TypeInfo) Directive() *Directive {
	return ti.directive
}

func (ti *TypeInfo) Argument() *Argument {
	return ti.argument
}

func (ti *TypeInfo) Enter(node ast.Node) {
	schema := ti.schema
	var ttype Type
	switch node := node.(type) {
	case *ast.SelectionSet:
		namedType := GetNamed(ti.Type())
		var compositeType Composite
		if IsCompositeType(namedType) {
			compositeType, _ = namedType.(Composite)
		}
		ti.parentTypeStack = append(ti.parentTypeStack, compositeType)
	case *ast.Field:
		parentType := ti.ParentType()
		var fieldDef *FieldDefinition
		if parentType != nil {
			fieldDef = ti.getFieldDef(schema, parentType.(Type), node)
		}
		ti.fieldDefStack = append(ti.fieldDefStack, fieldDef)
		if fieldDef != nil {
			ti.typeStack = append(ti.typeStack, fieldDef.Type)
		} else {
			ti.typeStack = append(ti.typeStack, nil)
		}
	case *ast.Directive:
		nameVal := ""
		if node.Name != nil {
			nameVal = node.Name.Value
		}
		ti.directive = schema.Directive(nameVal)
	case *ast.OperationDefinition:
		switch node.Operation {
		case ast.OperationTypeQuery:
			ttype = schema.QueryType()
		case ast.OperationTypeMutation:
			ttype = schema.MutationType()
		case ast.OperationTypeSubscription:
			ttype = schema.SubscriptionType()
		}
		ti.typeStack = append(ti.typeStack, ttype)
	case *ast.InlineFragment:
		if node.TypeCondition != nil {
			ttype, _ = typeFromAST(*schema, node.TypeCondition)
			ti.typeStack = append(ti.typeStack, ttype)
		} else {
			ti.typeStack = append(ti.typeStack, ti.Type())
		}
	case *ast.FragmentDefinition:
		if node.TypeCondition != nil {
			ttype, _ = typeFromAST(*schema, node.TypeCondition)
			ti.typeStack = append(ti.typeStack, ttype)
		} else {
			ti.typeStack = append(ti.typeStack, ti.Type())
		}
	case *ast.VariableDefinition:
		ttype, _ = typeFromAST(*schema, node.Type)
		ti.inputTypeStack = append(ti.inputTypeStack, ttype)
	case *ast.Argument:
		nameVal := ""
		if node.Name != nil {
			nameVal = node.Name.Value
		}
		var argType Input
		var argDef *Argument
		directive := ti.Directive()
		fieldDef := ti.FieldDef()
		if directive != nil {
			for _, arg := range directive.Args {
				if arg.Name() == nameVal {
					argDef = arg
				}
			}
		} else if fieldDef != nil {
			for _, arg := range fieldDef.Args {
				if arg.Name() == nameVal {
					argDef = arg
				}
			}
		}
		if argDef != nil {
			argType = argDef.Type
		}
		ti.argument = argDef
		ti.inputTypeStack = append(ti.inputTypeStack, argType)
	case *ast.ListValue:
		listType := GetNullable(ti.InputType())
		if list, ok := listType.(*List); ok {
			ti.inputTypeStack = append(ti.inputTypeStack, list.OfType)
		} else {
			ti.inputTypeStack = append(ti.inputTypeStack, nil)
		}
	case *ast.ObjectField:
		var fieldType Input
		objectType := GetNamed(ti.InputType())

		if objectType, ok := objectType.(*InputObject); ok {
			nameVal := ""
			if node.Name != nil {
				nameVal = node.Name.Value
			}
			if inputField, ok := objectType.Fields()[nameVal]; ok {
				fieldType = inputField.Type
			}
		}
		ti.inputTypeStack = append(ti.inputTypeStack, fieldType)
	}
}
func (ti *TypeInfo) Leave(node ast.Node) {
	switch node.(type) {
	case *ast.SelectionSet:
		// pop ti.parentTypeStack
		_, ti.parentTypeStack = ti.parentTypeStack[len(ti.parentTypeStack)-1], ti.parentTypeStack[:len(ti.parentTypeStack)-1]
	case *ast.Field:
		// pop ti.fieldDefStack
		_, ti.fieldDefStack = ti.fieldDefStack[len(ti.fieldDefStack)-1], ti.fieldDefStack[:len(ti.fieldDefStack)-1]
		// pop ti.typeStack
		_, ti.typeStack = ti.typeStack[len(ti.typeStack)-1], ti.typeStack[:len(ti.typeStack)-1]
	case *ast.Directive:
		ti.directive = nil
	case *ast.FragmentDefinition, *ast.OperationDefinition, *ast.InlineFragment:
		// pop ti.typeStack
		_, ti.typeStack = ti.typeStack[len(ti.typeStack)-1], ti.typeStack[:len(ti.typeStack)-1]
	case *ast.VariableDefinition:
		// pop ti.inputTypeStack
		_, ti.inputTypeStack = ti.inputTypeStack[len(ti.inputTypeStack)-1], ti.inputTypeStack[:len(ti.inputTypeStack)-1]
	case *ast.Argument:
		ti.argument = nil
		// pop ti.inputTypeStack
		_, ti.inputTypeStack = ti.inputTypeStack[len(ti.inputTypeStack)-1], ti.inputTypeStack[:len(ti.inputTypeStack)-1]
	case *ast.ObjectField, *ast.ListValue:
		// pop ti.inputTypeStack
		_, ti.inputTypeStack = ti.inputTypeStack[len(ti.inputTypeStack)-1], ti.inputTypeStack[:len(ti.inputTypeStack)-1]
	}
}

// DefaultTypeInfoFieldDef Not exactly the same as the executor's definition of FieldDef, in this
// statically evaluated environment we do not always have an Object type,
// and need to handle Interface and Union types.
func DefaultTypeInfoFieldDef(schema *Schema, parentType Type, fieldAST *ast.Field) *FieldDefinition {
	name := ""
	if fieldAST.Name != nil {
		name = fieldAST.Name.Value
	}
	if name == SchemaMetaFieldDef.Name && schema.QueryType() == parentType {
		return SchemaMetaFieldDef
	}
	if name == TypeMetaFieldDef.Name && schema.QueryType() == parentType {
		return TypeMetaFieldDef
	}
	if name == TypeNameMetaFieldDef.Name {
		switch v := parentType.(type) {
		case *Object:
			if v != nil {
				return TypeNameMetaFieldDef
			}
		case *Interface:
			if v != nil {
				return TypeNameMetaFieldDef
			}
		case *Union:
			if v != nil {
				return TypeNameMetaFieldDef
			}
		}
	}

	if parentType, ok := parentType.(*Object); ok && parentType != nil {
		return parentType.Fields()[name]
	}
	if parentType, ok := parentType.(*Interface); ok && parentType != nil {
		return parentType.Fields()[name]
	}
	return nil
}
