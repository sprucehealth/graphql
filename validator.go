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
// @internal
func VisitUsingRules(schema *Schema, typeInfo *TypeInfo, astDoc *ast.Document, rules []ValidationRuleFn) (errors []gqlerrors.FormattedError) {
	context := NewValidationContext(schema, astDoc, typeInfo)

	var visitInstance func(astNode ast.Node, instance *ValidationRuleInstance)

	visitInstance = func(astNode ast.Node, instance *ValidationRuleInstance) {
		visitor.Visit(astNode, &visitor.VisitorOptions{
			Enter: func(p visitor.VisitFuncParams) (string, interface{}) {
				var action = visitor.ActionNoChange
				var result interface{}
				switch node := p.Node.(type) {
				case ast.Node:
					// Collect type information about the current position in the AST.
					typeInfo.Enter(node)

					// Do not visit top level fragment definitions if this instance will
					// visit those fragments inline because it
					// provided `visitSpreadFragments`.

					if _, ok := node.(*ast.FragmentDefinition); ok && p.Parent != nil && instance.VisitSpreadFragments {
						return visitor.ActionSkip, nil
					}

					// Get the visitor function from the validation instance, and if it
					// exists, call it with the visitor arguments.
					if instance.Enter != nil {
						action, result = instance.Enter(p)
					}

					// If any validation instances provide the flag `visitSpreadFragments`
					// and this node is a fragment spread, visit the fragment definition
					// from this point.
					if _, ok := node.(*ast.FragmentSpread); ok && action == visitor.ActionNoChange && result == nil && instance.VisitSpreadFragments {
						node, _ := node.(*ast.FragmentSpread)
						name := node.Name
						nameVal := ""
						if name != nil {
							nameVal = name.Value
						}
						fragment := context.Fragment(nameVal)
						if fragment != nil {
							visitInstance(fragment, instance)
						}
					}

					// If the result is "false" (ie action === Action.Skip), we're not visiting any descendent nodes,
					// but need to update typeInfo.
					if action == visitor.ActionSkip {
						typeInfo.Leave(node)
					}
				}

				return action, result
			},
			Leave: func(p visitor.VisitFuncParams) (string, interface{}) {
				var action = visitor.ActionNoChange
				var result interface{}
				switch node := p.Node.(type) {
				case ast.Node:
					// Get the visitor function from the validation instance, and if it
					// exists, call it with the visitor arguments.
					if instance.Leave != nil {
						action, result = instance.Leave(p)
					}

					// Update typeInfo.
					typeInfo.Leave(node)
				}
				return action, result
			},
		})
	}

	for _, rule := range rules {
		visitInstance(astDoc, rule(context))
	}
	return context.Errors()
}

type ValidationContext struct {
	schema    *Schema
	astDoc    *ast.Document
	typeInfo  *TypeInfo
	fragments map[string]*ast.FragmentDefinition
	errors    []gqlerrors.FormattedError
}

func NewValidationContext(schema *Schema, astDoc *ast.Document, typeInfo *TypeInfo) *ValidationContext {
	return &ValidationContext{
		schema:   schema,
		astDoc:   astDoc,
		typeInfo: typeInfo,
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
