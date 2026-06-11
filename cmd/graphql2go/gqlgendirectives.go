package main

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/sprucehealth/graphql/language/ast"
)

// codegenDirectives are the gqlgen-style directives that graphql2go interprets at
// code-generation time. They are recognised whether or not the schema also defines them,
// and they are never emitted into the generated runtime schema.
var codegenDirectives = map[string]bool{
	"goModel":              true,
	"goField":              true,
	"goTag":                true,
	"goExtraField":         true,
	"goModelCompatibility": true,
	"inlineArguments":      true,
}

// structTag is a single key/value entry rendered into a Go struct field tag.
type structTag struct {
	key   string
	value string
}

// extraFieldSpec is a synthetic Go struct field added to a generated model, sourced from
// either the JSON config's ExtraFields or a @goExtraField directive.
type extraFieldSpec struct {
	name        string
	goType      string
	tags        string // raw struct-tag body (without backticks); empty means default json:"-"
	description string
}

// applySchemaDirectives reads gqlgen-style directives off the parsed schema and folds them
// into the generator configuration. It runs after the JSON config file is loaded so that
// directive values take precedence on conflict.
func (g *generator) applySchemaDirectives() {
	for _, def := range g.doc.Definitions {
		switch def := def.(type) {
		case *ast.ObjectDefinition:
			g.applyGoModel(def.Name.Value, def.Directives, "OBJECT")
			g.applyGoModelCompatibility(def.Name.Value, def.Directives)
			g.applyGoExtraFields(def.Name.Value, def.Directives)
			for _, f := range def.Fields {
				g.applyGoFieldAndTags(def.Name.Value, f.Name.Value, f.Directives, false)
			}
		case *ast.InputObjectDefinition:
			g.applyGoModel(def.Name.Value, def.Directives, "INPUT_OBJECT")
			g.applyGoModelCompatibility(def.Name.Value, def.Directives)
			g.applyGoExtraFields(def.Name.Value, def.Directives)
			for _, f := range def.Fields {
				g.applyGoFieldAndTags(def.Name.Value, f.Name.Value, f.Directives, true)
			}
		case *ast.InterfaceDefinition:
			g.applyGoModel(def.Name.Value, def.Directives, "INTERFACE")
		case *ast.UnionDefinition:
			g.applyGoModel(def.Name.Value, def.Directives, "UNION")
		case *ast.EnumDefinition:
			g.applyGoModel(def.Name.Value, def.Directives, "ENUM")
		case *ast.ScalarDefinition:
			g.applyGoModel(def.Name.Value, def.Directives, "SCALAR")
		}
	}
}

// applyGoModel binds a GraphQL type to an external Go type. Only SCALAR and ENUM are
// supported; the other locations are rejected.
func (g *generator) applyGoModel(typeName string, dirs []*ast.Directive, location string) {
	for _, d := range dirs {
		if d.Name.Value != "goModel" {
			continue
		}
		var model string
		var forceGenerate bool
		for _, a := range d.Arguments {
			switch a.Name.Value {
			case "model":
				model = stringArg(a.Value)
			case "models":
				models := listStringArg(a.Value)
				if len(models) > 1 {
					g.failf("@goModel on %q: 'models' with more than one entry is not supported", typeName)
				}
				if len(models) == 1 {
					model = models[0]
				}
			case "forceGenerate":
				forceGenerate = boolArg(a.Value)
			default:
				g.failf("@goModel on %q: unknown argument %q (valid: model, models, forceGenerate)", typeName, a.Name.Value)
			}
		}
		if model == "" {
			continue
		}
		switch location {
		case "SCALAR":
			g.cfg.CustomScalarTypes[typeName] = model
		case "ENUM":
			// forceGenerate keeps the generated enum type and constants, ignoring the binding.
			if !forceGenerate {
				g.boundEnums[typeName] = model
			}
		default:
			g.failf("@goModel on %s %q is not supported (only SCALAR and ENUM)", location, typeName)
		}
	}
}

// applyGoFieldAndTags reads @goField and @goTag off a field or input field.
func (g *generator) applyGoFieldAndTags(typeName, fieldName string, dirs []*ast.Directive, isInput bool) {
	fieldKey := typeName + "." + fieldName
	for _, d := range dirs {
		switch d.Name.Value {
		case "goField":
			for _, a := range d.Arguments {
				switch a.Name.Value {
				case "forceResolver":
					if boolArg(a.Value) && !isInput {
						if !slices.Contains(g.cfg.Resolvers[typeName], fieldName) {
							g.cfg.Resolvers[typeName] = append(g.cfg.Resolvers[typeName], fieldName)
						}
					}
				case "type":
					if t := stringArg(a.Value); t != "" {
						g.cfg.CustomFieldTypes[fieldKey] = t
					}
				case "name":
					if n := stringArg(a.Value); n != "" {
						g.goFieldNames[fieldKey] = n
					}
				case "omittable":
					// Store the explicit value so a field can opt out of a model/global
					// nullOmittable setting with @goField(omittable: false).
					g.omittableFields[fieldKey] = boolArg(a.Value)
				case "autoBindGetterHaser", "batch", "forceGenerate":
					// No graphql2go equivalent; recognised and ignored.
					if *flagVerbose {
						log.Printf("graphql2go: @goField(%s) on %s ignored (no equivalent)", a.Name.Value, fieldKey)
					}
				default:
					g.failf("@goField on %q: unknown argument %q (valid: forceResolver, name, omittable, type, autoBindGetterHaser, forceGenerate, batch)", fieldKey, a.Name.Value)
				}
			}
		case "goTag":
			var key, value string
			for _, a := range d.Arguments {
				switch a.Name.Value {
				case "key":
					key = stringArg(a.Value)
				case "value":
					value = stringArg(a.Value)
				default:
					g.failf("@goTag on %q: unknown argument %q (valid: key, value)", fieldKey, a.Name.Value)
				}
			}
			if key != "" {
				g.goTags[fieldKey] = append(g.goTags[fieldKey], structTag{key: key, value: value})
			}
		}
	}
}

// applyGoExtraFields reads @goExtraField directives off an object or input object.
func (g *generator) applyGoExtraFields(typeName string, dirs []*ast.Directive) {
	for _, d := range dirs {
		if d.Name.Value != "goExtraField" {
			continue
		}
		var spec extraFieldSpec
		for _, a := range d.Arguments {
			switch a.Name.Value {
			case "name":
				spec.name = stringArg(a.Value)
			case "type":
				spec.goType = stringArg(a.Value)
			case "overrideTags":
				spec.tags = stringArg(a.Value)
			case "description":
				spec.description = stringArg(a.Value)
			default:
				g.failf("@goExtraField on %q: unknown argument %q (valid: name, type, overrideTags, description)", typeName, a.Name.Value)
			}
		}
		if spec.name == "" || spec.goType == "" {
			g.failf("@goExtraField on %q requires both 'name' and 'type'", typeName)
		}
		if g.extraFields[typeName] == nil {
			g.extraFields[typeName] = make(map[string]extraFieldSpec)
		}
		g.extraFields[typeName][spec.name] = spec
	}
}

// applyGoModelCompatibility reads @goModelCompatibility off an object or input object and
// records the model-level nullOmittable override (only when the argument is present).
func (g *generator) applyGoModelCompatibility(typeName string, dirs []*ast.Directive) {
	for _, d := range dirs {
		if d.Name.Value != "goModelCompatibility" {
			continue
		}
		for _, a := range d.Arguments {
			switch a.Name.Value {
			case "nullOmittable":
				g.modelNullOmittable[typeName] = boolArg(a.Value)
			default:
				g.failf("@goModelCompatibility on %q: unknown argument %q (valid: nullOmittable)", typeName, a.Name.Value)
			}
		}
	}
}

// mergeTags returns the default tags with the overrides applied: an override whose key
// matches a default replaces that default's value in place, otherwise it is appended in
// directive order.
func mergeTags(defaults, overrides []structTag) []structTag {
	out := append([]structTag(nil), defaults...)
	for _, o := range overrides {
		var replaced bool
		for i := range out {
			if out[i].key == o.key {
				out[i].value = o.value
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, o)
		}
	}
	return out
}

// renderStructTag renders tags as the body of a Go struct field tag (without backticks),
// e.g. `json:"name,omitempty" db:"name"`.
func renderStructTag(tags []structTag) string {
	parts := make([]string, len(tags))
	for i, t := range tags {
		parts[i] = fmt.Sprintf("%s:%q", t.key, t.value)
	}
	return strings.Join(parts, " ")
}

func stringArg(v ast.Value) string {
	if s, ok := v.(*ast.StringValue); ok {
		return s.Value
	}
	return ""
}

func boolArg(v ast.Value) bool {
	if b, ok := v.(*ast.BooleanValue); ok {
		return b.Value
	}
	return false
}

func listStringArg(v ast.Value) []string {
	lv, ok := v.(*ast.ListValue)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(lv.Values))
	for _, item := range lv.Values {
		if s, ok := item.(*ast.StringValue); ok {
			out = append(out, s.Value)
		}
	}
	return out
}
