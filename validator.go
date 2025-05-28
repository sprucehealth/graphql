package graphql

import (
	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/visitor"
)

type ValidationResult struct {
	IsValid bool
	Errors  []gqlerrors.FormattedError
}

// ValidateDocument implements the "Validation" section of the spec.
//
// Validation runs synchronously, returning an array of encountered errors, or
// an empty array if no errors were encountered and the document is valid.
//
// A list of specific validation rules may be provided. If not provided, the
// default list of rules defined by the GraphQL specification will be used.
//
// Each validation rules is a function which returns a visitor
// (see the language/visitor API). Visitor methods are expected to return
// GraphQLErrors, or Arrays of GraphQLErrors when invalid.
func ValidateDocument(schema *Schema, astDoc *ast.Document, rules []ValidationRuleFn) (vr ValidationResult) {
	if len(rules) == 0 {
		rules = SpecifiedRules
	}

	vr.IsValid = false
	if schema == nil {
		vr.Errors = append(vr.Errors, gqlerrors.NewFormattedError("Must provide schema"))
		return vr
	}
	if astDoc == nil {
		vr.Errors = append(vr.Errors, gqlerrors.NewFormattedError("Must provide document"))
		return vr
	}
	typeInfo := NewTypeInfo(&TypeInfoConfig{
		Schema: schema,
	})
	vr.Errors = VisitUsingRules(schema, typeInfo, astDoc, rules)
	vr.IsValid = len(vr.Errors) == 0
	return vr
}

// VisitUsingRules This uses a specialized visitor which runs multiple visitors in parallel,
// while maintaining the visitor skip and break API.
//
// @internal
// Had to expose it to unit test experimental customizable validation feature,
// but not meant for public consumption
func VisitUsingRules(schema *Schema, typeInfo *TypeInfo, astDoc *ast.Document, rules []ValidationRuleFn) []gqlerrors.FormattedError {
	context := NewValidationContext(schema, astDoc, typeInfo)

	visitInstance := func(astNode ast.Node, instance *ValidationRuleInstance) {
		err := visitor.Visit(astNode, &visitor.VisitorOptions{
			Enter: func(p visitor.VisitFuncParams) (string, any) {
				node, ok := p.Node.(ast.Node)
				if !ok {
					return visitor.ActionNoChange, nil
				}

				// Collect type information about the current position in the AST.
				typeInfo.Enter(node)

				action := visitor.ActionNoChange
				var result any
				if instance.Enter != nil {
					action, result = instance.Enter(p)
				}

				// If the result is "false" (ie action === Action.Skip), we're not visiting any descendent nodes,
				// but need to update typeInfo.
				if action == visitor.ActionSkip {
					typeInfo.Leave(node)
				}

				return action, result
			},
			Leave: func(p visitor.VisitFuncParams) (string, any) {
				node, ok := p.Node.(ast.Node)
				if !ok {
					return visitor.ActionNoChange, nil
				}

				var action = visitor.ActionNoChange
				var result any
				if instance.Leave != nil {
					action, result = instance.Leave(p)
				}

				typeInfo.Leave(node)

				return action, result
			},
		})
		// TODO: handle error
		_ = err
	}

	for _, rule := range rules {
		visitInstance(astDoc, rule(context))
	}
	return context.Errors()
}

type HasSelectionSet interface {
	GetLoc() ast.Location
	GetSelectionSet() *ast.SelectionSet
}

var _ HasSelectionSet = (*ast.OperationDefinition)(nil)
var _ HasSelectionSet = (*ast.FragmentDefinition)(nil)

type VariableUsage struct {
	Node *ast.Variable
	Type Input
}

type ValidationContext struct {
	schema                         *Schema
	astDoc                         *ast.Document
	typeInfo                       *TypeInfo
	fragments                      map[string]*ast.FragmentDefinition
	variableUsages                 map[HasSelectionSet][]*VariableUsage
	recursiveVariableUsages        map[*ast.OperationDefinition][]*VariableUsage
	recursivelyReferencedFragments map[*ast.OperationDefinition][]*ast.FragmentDefinition
	fragmentSpreads                map[HasSelectionSet][]*ast.FragmentSpread
	errors                         []gqlerrors.FormattedError
}

func NewValidationContext(schema *Schema, astDoc *ast.Document, typeInfo *TypeInfo) *ValidationContext {
	return &ValidationContext{
		schema:                         schema,
		astDoc:                         astDoc,
		typeInfo:                       typeInfo,
		variableUsages:                 make(map[HasSelectionSet][]*VariableUsage),
		recursiveVariableUsages:        make(map[*ast.OperationDefinition][]*VariableUsage),
		recursivelyReferencedFragments: make(map[*ast.OperationDefinition][]*ast.FragmentDefinition),
		fragmentSpreads:                make(map[HasSelectionSet][]*ast.FragmentSpread),
	}
}

func (ctx *ValidationContext) ReportError(err error) {
	formattedErr := gqlerrors.FormatError(err)
	ctx.errors = append(ctx.errors, formattedErr)
}

func (ctx *ValidationContext) Errors() []gqlerrors.FormattedError {
	return ctx.errors
}

func (ctx *ValidationContext) Schema() *Schema {
	return ctx.schema
}
func (ctx *ValidationContext) Document() *ast.Document {
	return ctx.astDoc
}

func (ctx *ValidationContext) Fragment(name string) *ast.FragmentDefinition {
	if ctx.fragments == nil {
		if ctx.Document() == nil {
			return nil
		}
		defs := ctx.Document().Definitions
		fragments := make(map[string]*ast.FragmentDefinition)
		for _, def := range defs {
			if def, ok := def.(*ast.FragmentDefinition); ok {
				defName := ""
				if def.Name != nil {
					defName = def.Name.Value
				}
				fragments[defName] = def
			}
		}
		ctx.fragments = fragments
	}
	return ctx.fragments[name]
}

func (ctx *ValidationContext) FragmentSpreads(node HasSelectionSet) []*ast.FragmentSpread {
	if spreads, ok := ctx.fragmentSpreads[node]; ok && spreads != nil {
		return spreads
	}

	spreads := []*ast.FragmentSpread{}
	setsToVisit := []*ast.SelectionSet{node.GetSelectionSet()}

	for len(setsToVisit) != 0 {
		var set *ast.SelectionSet
		// pop
		set, setsToVisit = setsToVisit[len(setsToVisit)-1], setsToVisit[:len(setsToVisit)-1]
		if set.Selections != nil {
			for _, selection := range set.Selections {
				switch selection := selection.(type) {
				case *ast.FragmentSpread:
					spreads = append(spreads, selection)
				case *ast.Field:
					if selection.SelectionSet != nil {
						setsToVisit = append(setsToVisit, selection.SelectionSet)
					}
				case *ast.InlineFragment:
					if selection.SelectionSet != nil {
						setsToVisit = append(setsToVisit, selection.SelectionSet)
					}
				}
			}
		}
		ctx.fragmentSpreads[node] = spreads
	}
	return spreads
}

func (ctx *ValidationContext) RecursivelyReferencedFragments(operation *ast.OperationDefinition) []*ast.FragmentDefinition {
	if fragments, ok := ctx.recursivelyReferencedFragments[operation]; ok && fragments != nil {
		return fragments
	}

	fragments := []*ast.FragmentDefinition{}
	collectedNames := map[string]bool{}
	nodesToVisit := []HasSelectionSet{operation}

	for len(nodesToVisit) != 0 {
		var node HasSelectionSet

		node, nodesToVisit = nodesToVisit[len(nodesToVisit)-1], nodesToVisit[:len(nodesToVisit)-1]
		spreads := ctx.FragmentSpreads(node)
		for _, spread := range spreads {
			fragName := ""
			if spread.Name != nil {
				fragName = spread.Name.Value
			}
			if res, ok := collectedNames[fragName]; !ok || !res {
				collectedNames[fragName] = true
				fragment := ctx.Fragment(fragName)
				if fragment != nil {
					fragments = append(fragments, fragment)
					nodesToVisit = append(nodesToVisit, fragment)
				}
			}
		}
	}

	ctx.recursivelyReferencedFragments[operation] = fragments
	return fragments
}

func (ctx *ValidationContext) VariableUsages(node HasSelectionSet) []*VariableUsage {
	if usages, ok := ctx.variableUsages[node]; ok && usages != nil {
		return usages
	}
	typeInfo := NewTypeInfo(&TypeInfoConfig{
		Schema: ctx.schema,
	})

	var usages []*VariableUsage
	err := visitor.Visit(node, &visitor.VisitorOptions{
		Enter: func(p visitor.VisitFuncParams) (string, any) {
			if node, ok := p.Node.(ast.Node); ok {
				typeInfo.Enter(node)
				switch node := node.(type) {
				case *ast.VariableDefinition:
					typeInfo.Leave(node)
					return visitor.ActionSkip, nil
				case *ast.Variable:
					usages = append(usages, &VariableUsage{
						Node: node,
						Type: typeInfo.InputType(),
					})
				}
			}
			return visitor.ActionNoChange, nil
		},
		Leave: func(p visitor.VisitFuncParams) (string, any) {
			if node, ok := p.Node.(ast.Node); ok {
				typeInfo.Leave(node)
			}
			return visitor.ActionNoChange, nil
		},
	})
	// TODO: handle error
	_ = err

	ctx.variableUsages[node] = usages
	return usages
}

func (ctx *ValidationContext) RecursiveVariableUsages(operation *ast.OperationDefinition) []*VariableUsage {
	if usages, ok := ctx.recursiveVariableUsages[operation]; ok && usages != nil {
		return usages
	}
	usages := ctx.VariableUsages(operation)

	fragments := ctx.RecursivelyReferencedFragments(operation)
	for _, fragment := range fragments {
		fragmentUsages := ctx.VariableUsages(fragment)
		usages = append(usages, fragmentUsages...)
	}

	ctx.recursiveVariableUsages[operation] = usages
	return usages
}

func (ctx *ValidationContext) Type() Output {
	return ctx.typeInfo.Type()
}
func (ctx *ValidationContext) ParentType() Composite {
	return ctx.typeInfo.ParentType()
}
func (ctx *ValidationContext) InputType() Input {
	return ctx.typeInfo.InputType()
}
func (ctx *ValidationContext) FieldDef() *FieldDefinition {
	return ctx.typeInfo.FieldDef()
}
func (ctx *ValidationContext) Directive() *Directive {
	return ctx.typeInfo.Directive()
}
func (ctx *ValidationContext) Argument() *Argument {
	return ctx.typeInfo.Argument()
}
