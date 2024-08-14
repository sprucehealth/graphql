package graphql_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/location"
	"github.com/sprucehealth/graphql/testutil"
)

var directivesTestSchema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: graphql.NewObject(graphql.ObjectConfig{
		Name: "TestType",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Type: graphql.String,
			},
			"b": &graphql.Field{
				Type: graphql.String,
			},
		},
	}),
})

var directivesTestData map[string]any = map[string]any{
	"a": func() any { return "a" },
	"b": func() any { return "b" },
}

func executeDirectivesTestQuery(t *testing.T, doc string) *graphql.Result {
	ast := testutil.TestParse(t, doc)
	ep := graphql.ExecuteParams{
		Schema: directivesTestSchema,
		AST:    ast,
		Root:   directivesTestData,
	}
	return testutil.TestExecute(t, context.Background(), ep)
}

func TestDirectives_DirectivesMustBeNamed(t *testing.T) {
	invalidDirective := graphql.NewDirective(graphql.DirectiveConfig{
		Locations: []string{
			graphql.DirectiveLocationField,
		},
	})
	_, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "TestType",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
		Directives: []*graphql.Directive{invalidDirective},
	})
	expectedErr := gqlerrors.FormattedError{
		Message:       "Directive must be named.",
		Locations:     []location.SourceLocation{},
		Type:          "INTERNAL",
		OriginalError: errors.New("Directive must be named."),
	}
	e := err.(gqlerrors.FormattedError)
	e.StackTrace = ""
	if !reflect.DeepEqual(expectedErr, e) {
		t.Fatalf("Expected error to be equal, got: %v", testutil.Diff(expectedErr, err))
	}
}

func TestDirectives_DirectiveNameMustBeValid(t *testing.T) {
	invalidDirective := graphql.NewDirective(graphql.DirectiveConfig{
		Name: "123invalid name",
		Locations: []string{
			graphql.DirectiveLocationField,
		},
	})
	_, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "TestType",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
		Directives: []*graphql.Directive{invalidDirective},
	})
	expectedErr := gqlerrors.FormattedError{
		Message:       `Names must match /^[_a-zA-Z][_a-zA-Z0-9]*$/ but "123invalid name" does not.`,
		Locations:     []location.SourceLocation{},
		Type:          "INTERNAL",
		OriginalError: errors.New("Names must match /^[_a-zA-Z][_a-zA-Z0-9]*$/ but \"123invalid name\" does not."),
	}
	e := err.(gqlerrors.FormattedError)
	e.StackTrace = ""
	if !reflect.DeepEqual(expectedErr, e) {
		t.Fatalf("Expected error to be equal, got: %v", testutil.Diff(expectedErr, err))
	}
}

func TestDirectives_DirectiveNameMustProvideLocations(t *testing.T) {
	invalidDirective := graphql.NewDirective(graphql.DirectiveConfig{
		Name: "skip",
	})
	_, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "TestType",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
		Directives: []*graphql.Directive{invalidDirective},
	})
	expectedErr := gqlerrors.FormattedError{
		Message:       `Must provide locations for directive.`,
		Locations:     []location.SourceLocation{},
		Type:          "INTERNAL",
		OriginalError: errors.New("Must provide locations for directive."),
	}
	e := err.(gqlerrors.FormattedError)
	e.StackTrace = ""
	if !reflect.DeepEqual(expectedErr, e) {
		t.Fatalf("Expected error to be equal, got: %v", testutil.Diff(expectedErr, err))
	}
}

func TestDirectives_DirectiveArgNamesMustBeValid(t *testing.T) {
	invalidDirective := graphql.NewDirective(graphql.DirectiveConfig{
		Name: "skip",
		Description: "Directs the executor to skip this field or fragment when the `if` " +
			"argument is true.",
		Args: graphql.FieldConfigArgument{
			"123if": &graphql.ArgumentConfig{
				Type:        graphql.NewNonNull(graphql.Boolean),
				Description: "Skipped when true.",
			},
		},
		Locations: []string{
			graphql.DirectiveLocationField,
			graphql.DirectiveLocationFragmentSpread,
			graphql.DirectiveLocationInlineFragment,
		},
	})
	_, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "TestType",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
		Directives: []*graphql.Directive{invalidDirective},
	})
	expectedErr := gqlerrors.FormattedError{
		Message:       `Names must match /^[_a-zA-Z][_a-zA-Z0-9]*$/ but "123if" does not.`,
		Locations:     []location.SourceLocation{},
		Type:          "INTERNAL",
		OriginalError: errors.New("Names must match /^[_a-zA-Z][_a-zA-Z0-9]*$/ but \"123if\" does not."),
	}
	e := err.(gqlerrors.FormattedError)
	e.StackTrace = ""
	if !reflect.DeepEqual(expectedErr, e) {
		t.Fatalf("Expected error to be equal, got: %v", testutil.Diff(expectedErr, err))
	}
}

func TestDirectivesWorksWithoutDirectives(t *testing.T) {
	query := `{ a, b }`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnScalarsIfTrueIncludesScalar(t *testing.T) {
	query := `{ a, b @include(if: true) }`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnScalarsIfFalseOmitsOnScalar(t *testing.T) {
	query := `{ a, b @include(if: false) }`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnScalarsUnlessFalseIncludesScalar(t *testing.T) {
	query := `{ a, b @skip(if: false) }`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnScalarsUnlessTrueOmitsScalar(t *testing.T) {
	query := `{ a, b @skip(if: true) }`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnFragmentSpreadsIfFalseOmitsFragmentSpread(t *testing.T) {
	query := `
        query Q {
          a
          ...Frag @include(if: false)
        }
        fragment Frag on TestType {
          b
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnFragmentSpreadsIfTrueIncludesFragmentSpread(t *testing.T) {
	query := `
        query Q {
          a
          ...Frag @include(if: true)
        }
        fragment Frag on TestType {
          b
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnFragmentSpreadsUnlessFalseIncludesFragmentSpread(t *testing.T) {
	query := `
        query Q {
          a
          ...Frag @skip(if: false)
        }
        fragment Frag on TestType {
          b
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnFragmentSpreadsUnlessTrueOmitsFragmentSpread(t *testing.T) {
	query := `
        query Q {
          a
          ...Frag @skip(if: true)
        }
        fragment Frag on TestType {
          b
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnInlineFragmentIfFalseOmitsInlineFragment(t *testing.T) {
	query := `
        query Q {
          a
          ... on TestType @include(if: false) {
            b
          }
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnInlineFragmentIfTrueIncludesInlineFragment(t *testing.T) {
	query := `
        query Q {
          a
          ... on TestType @include(if: true) {
            b
          }
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnInlineFragmentUnlessFalseIncludesInlineFragment(t *testing.T) {
	query := `
        query Q {
          a
          ... on TestType @skip(if: false) {
            b
          }
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnInlineFragmentUnlessTrueIncludesInlineFragment(t *testing.T) {
	query := `
        query Q {
          a
          ... on TestType @skip(if: true) {
            b
          }
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnAnonymousInlineFragmentIfFalseOmitsAnonymousInlineFragment(t *testing.T) {
	query := `
        query Q {
          a
          ... @include(if: false) {
            b
          }
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnAnonymousInlineFragmentIfTrueIncludesAnonymousInlineFragment(t *testing.T) {
	query := `
        query Q {
          a
          ... @include(if: true) {
            b
          }
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnAnonymousInlineFragmentUnlessFalseIncludesAnonymousInlineFragment(t *testing.T) {
	query := `
        query Q {
          a
          ... @skip(if: false) {
            b
          }
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksOnAnonymousInlineFragmentUnlessTrueIncludesAnonymousInlineFragment(t *testing.T) {
	query := `
        query Q {
          a
          ... @skip(if: true) {
            b
          }
        }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksWithSkipAndIncludeDirectives_IncludeAndNoSkip(t *testing.T) {
	query := `{ a, b @include(if: true) @skip(if: false) }`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksWithSkipAndIncludeDirectives_IncludeAndSkip(t *testing.T) {
	query := `{ a, b @include(if: true) @skip(if: true) }`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksWithSkipAndIncludeDirectives_NoIncludeAndSkip(t *testing.T) {
	query := `{ a, b @include(if: false) @skip(if: true) }`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDirectivesWorksWithSkipAndIncludeDirectives_NoIncludeOrSkip(t *testing.T) {
	query := `{ a, b @include(if: false) @skip(if: false) }`
	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
		},
	}
	result := executeDirectivesTestQuery(t, query)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

var fieldDefinitionDirectivesTestSchema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: graphql.NewObject(graphql.ObjectConfig{
		Name: "TestType",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "InnerType",
					Fields: graphql.Fields{
						"b": &graphql.Field{
							Type: graphql.String,
							Directives: []*ast.Directive{
								{
									Name: &ast.Name{Value: "fieldDefDirective"},
								},
							},
						},
					},
				}),
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return struct {
						b string
					}{
						b: "b",
					}, nil
				},
				Directives: []*ast.Directive{
					{
						Name: &ast.Name{Value: "fieldDefDirective"},
					},
				},
			},
		},
	}),
})

var fieldDefinitionDirectivesTestData map[string]any = map[string]any{
	"a": func() any { return "a" },
	"b": func() any { return "b" },
}

func executeFieldDefinitionDirectivesTestQuery(t *testing.T, doc string, handler func(context.Context, *ast.Directive, *graphql.FieldDefinition) error) *graphql.Result {
	ast := testutil.TestParse(t, doc)
	ep := graphql.ExecuteParams{
		Schema:                          fieldDefinitionDirectivesTestSchema,
		AST:                             ast,
		Root:                            fieldDefinitionDirectivesTestData,
		FieldDefinitionDirectiveHandler: handler,
	}
	return testutil.TestExecute(t, context.Background(), ep)
}

func TestFieldDefinitionDirectiveHandler(t *testing.T) {
	query := `{ a { b } }`
	var checkedA bool
	var checkedB bool
	result := executeFieldDefinitionDirectivesTestQuery(t, query, func(ctx context.Context, d *ast.Directive, fd *graphql.FieldDefinition) error {
		if fd.Name == "a" {
			checkedA = true
		}
		if fd.Name == "b" {
			checkedB = true
		}
		return nil
	})
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !checkedA {
		t.Fatalf("A was never checked by handler")
	}
	if !checkedB {
		t.Fatalf("B was never checked by handler")
	}
}
