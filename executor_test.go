package graphql_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/location"
	"github.com/sprucehealth/graphql/language/parser"
	"github.com/sprucehealth/graphql/language/source"
	"github.com/sprucehealth/graphql/testutil"
)

func TestExecutesArbitraryCode(t *testing.T) {
	deepData := map[string]any{}
	data := map[string]any{
		"a": func() any { return "Apple" },
		"b": func() any { return "Banana" },
		"c": func() any { return "Cookie" },
		"d": func() any { return "Donut" },
		"e": func() any { return "Egg" },
		"f": "Fish",
		"pic": func(size int) string {
			return fmt.Sprintf("Pic of size: %v", size)
		},
		"deep": func() any { return deepData },
	}
	data["promise"] = func() any {
		return data
	}
	deepData = map[string]any{
		"a":      func() any { return "Already Been Done" },
		"b":      func() any { return "Boring" },
		"c":      func() any { return []string{"Contrived", "", "Confusing"} },
		"deeper": func() any { return []any{data, nil, data} },
	}

	query := `
      query Example($size: Int) {
        a,
        b,
        x: c
        ...c
        f
        ...on DataType {
          pic(size: $size)
          promise {
            a
          }
        }
        deep {
          a
          b
          c
          deeper {
            a
            b
          }
        }
      }

      fragment c on DataType {
        d
        e
      }
    `

	expected := &graphql.Result{
		Data: map[string]any{
			"b": "Banana",
			"x": "Cookie",
			"d": "Donut",
			"e": "Egg",
			"promise": map[string]any{
				"a": "Apple",
			},
			"a": "Apple",
			"deep": map[string]any{
				"a": "Already Been Done",
				"b": "Boring",
				"c": []any{
					"Contrived",
					"",
					"Confusing",
				},
				"deeper": []any{
					map[string]any{
						"a": "Apple",
						"b": "Banana",
					},
					nil,
					map[string]any{
						"a": "Apple",
						"b": "Banana",
					},
				},
			},
			"f":   "Fish",
			"pic": "Pic of size: 100",
		},
	}

	// Schema Definitions
	picResolverFn := func(ctx context.Context, p graphql.ResolveParams) (any, error) {
		// get and type assert ResolveFn for this field
		picResolver, ok := p.Source.(map[string]any)["pic"].(func(size int) string)
		if !ok {
			return nil, nil
		}
		// get and type assert argument
		sizeArg, ok := p.Args["size"].(int)
		if !ok {
			return nil, nil
		}
		return picResolver(sizeArg), nil
	}
	dataType := graphql.NewObject(graphql.ObjectConfig{
		Name: "DataType",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Type: graphql.String,
			},
			"b": &graphql.Field{
				Type: graphql.String,
			},
			"c": &graphql.Field{
				Type: graphql.String,
			},
			"d": &graphql.Field{
				Type: graphql.String,
			},
			"e": &graphql.Field{
				Type: graphql.String,
			},
			"f": &graphql.Field{
				Type: graphql.String,
			},
			"pic": &graphql.Field{
				Args: graphql.FieldConfigArgument{
					"size": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Type:    graphql.String,
				Resolve: picResolverFn,
			},
		},
	})
	deepDataType := graphql.NewObject(graphql.ObjectConfig{
		Name: "DeepDataType",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Type: graphql.String,
			},
			"b": &graphql.Field{
				Type: graphql.String,
			},
			"c": &graphql.Field{
				Type: graphql.NewList(graphql.String),
			},
			"deeper": &graphql.Field{
				Type: graphql.NewList(dataType),
			},
		},
	})

	// Exploring a way to have a Object within itself
	// in this case DataType has DeepDataType has DataType
	dataType.AddFieldConfig("deep", &graphql.Field{
		Type: deepDataType,
	})
	// in this case DataType has DataType
	dataType.AddFieldConfig("promise", &graphql.Field{
		Type: dataType,
	})

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: dataType,
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	astDoc := testutil.TestParse(t, query)

	// execute
	args := map[string]any{
		"size": 100,
	}
	operationName := "Example"
	ep := graphql.ExecuteParams{
		Schema:        schema,
		Root:          data,
		AST:           astDoc,
		OperationName: operationName,
		Args:          args,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestAliasedString(t *testing.T) {
	query := `
		query _ {
			foo
			bar
		}
    `

	expected := &graphql.Result{
		Data: map[string]any{
			"foo": "bar",
			"bar": "foo",
		},
	}

	type LikeAString string

	const fooEnumValue LikeAString = "foo"

	enumType := graphql.NewEnum(graphql.EnumConfig{
		Name: "Bar",
		Values: graphql.EnumValueConfigMap{
			string(fooEnumValue): &graphql.EnumValueConfig{
				Value: fooEnumValue,
			},
		},
	})

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"foo": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return LikeAString("bar"), nil
					},
				},
				"bar": &graphql.Field{
					Type: graphql.NewNonNull(enumType),
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return fooEnumValue, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %s", err)
	}

	// parse query
	astDoc := testutil.TestParse(t, query)

	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    astDoc,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestMergesParallelFragments(t *testing.T) {
	query := `
      { a, ...FragOne, ...FragTwo }

      fragment FragOne on Type {
        b
        deep { b, deeper: deep { b } }
      }

      fragment FragTwo on Type {
        c
        deep { c, deeper: deep { c } }
      }
    `

	expected := &graphql.Result{
		Data: map[string]any{
			"a": "Apple",
			"b": "Banana",
			"deep": map[string]any{
				"c": "Cherry",
				"b": "Banana",
				"deeper": map[string]any{
					"b": "Banana",
					"c": "Cherry",
				},
			},
			"c": "Cherry",
		},
	}

	typeObjectType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Type",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Type: graphql.String,
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return "Apple", nil
				},
			},
			"b": &graphql.Field{
				Type: graphql.String,
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return "Banana", nil
				},
			},
			"c": &graphql.Field{
				Type: graphql.String,
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return "Cherry", nil
				},
			},
		},
	})
	deepTypeFieldConfig := &graphql.Field{
		Type: typeObjectType,
		Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
			return p.Source, nil
		},
	}
	typeObjectType.AddFieldConfig("deep", deepTypeFieldConfig)

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: typeObjectType,
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, query)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestThreadsSourceCorrectly(t *testing.T) {
	query := `
      query Example { a }
    `

	data := map[string]any{
		"key": "value",
	}

	var resolvedSource map[string]any

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						resolvedSource = p.Source.(map[string]any)
						return resolvedSource, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, query)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		Root:   data,
		AST:    ast,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}

	expected := "value"
	if resolvedSource["key"] != expected {
		t.Fatalf("Expected context.key to equal %v, got %v", expected, resolvedSource["key"])
	}
}

func TestOmitEmpty(t *testing.T) {
	query := `query Example { a {
		b
		c
		d
	} }`

	aType := graphql.NewObject(graphql.ObjectConfig{
		Name: "A",
		Fields: graphql.Fields{
			"b": &graphql.Field{Type: graphql.String},
			"c": &graphql.Field{Type: graphql.String},
			"d": &graphql.Field{Type: graphql.String},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: aType,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return &struct {
							B string `json:"b"`
							C string `json:"c,omitempty"`
						}{}, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	ast := testutil.TestParse(t, query)
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"a": map[string]any{
				"b": "",
				"c": nil,
				"d": nil,
			},
		},
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestCorrectlyThreadsArguments(t *testing.T) {
	query := `
      query Example {
        b(numArg: 123, stringArg: "foo")
      }
    `

	var resolvedArgs map[string]any

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"b": &graphql.Field{
					Args: graphql.FieldConfigArgument{
						"numArg": &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
						"stringArg": &graphql.ArgumentConfig{
							Type: graphql.String,
						},
					},
					Type: graphql.String,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						resolvedArgs = p.Args
						return resolvedArgs, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, query)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}

	expectedNum := 123
	expectedString := "foo"
	if resolvedArgs["numArg"] != expectedNum {
		t.Fatalf("Expected args.numArg to equal `%v`, got `%v`", expectedNum, resolvedArgs["numArg"])
	}
	if resolvedArgs["stringArg"] != expectedString {
		t.Fatalf("Expected args.stringArg to equal `%v`, got `%v`", expectedNum, resolvedArgs["stringArg"])
	}
}

func TestThreadsRootValueContextCorrectly(t *testing.T) {
	query := `
      query Example { a }
    `

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						val, _ := p.Info.RootValue.(map[string]any)["stringKey"].(string)
						return val, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, query)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
		Root: map[string]any{
			"stringKey": "stringValue",
		},
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"a": "stringValue",
		},
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestThreadsContextCorrectly(t *testing.T) {
	query := `
      query Example { a }
    `

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return ctx.Value("foo"), nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, query)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
	}
	//nolint:staticcheck
	result := testutil.TestExecute(t, context.WithValue(context.Background(), "foo", "bar"), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"a": "bar",
		},
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestNullsOutErrorSubtrees(t *testing.T) {
	// TODO: TestNullsOutErrorSubtrees test for go-routines if implemented
	query := `{
      sync,
      syncError,
    }`

	expectedData := map[string]any{
		"sync":      "sync",
		"syncError": nil,
	}
	expectedErrors := []gqlerrors.FormattedError{
		{
			Type:    gqlerrors.ErrorTypeInternal,
			Message: "Error getting syncError",
			Locations: []location.SourceLocation{
				{
					Line: 3, Column: 7,
				},
			},
		},
	}

	data := map[string]any{
		"sync": func() any {
			return "sync"
		},
		"syncError": func() any {
			panic("Error getting syncError")
		},
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"sync": &graphql.Field{
					Type: graphql.String,
				},
				"syncError": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, query)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
		Root:   data,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) == 0 {
		t.Fatalf("wrong result, expected errors, got %v", len(result.Errors))
	}
	if !reflect.DeepEqual(expectedData, result.Data) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expectedData, result.Data))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expectedErrors, result.Errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expectedErrors, result.Errors))
	}
}

func TestUsesTheInlineOperationIfNoOperationNameIsProvided(t *testing.T) {
	doc := `{ a }`
	data := map[string]any{
		"a": "b",
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"a": "b",
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
		Root:   data,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestUsesTheOnlyOperationIfNoOperationNameIsProvided(t *testing.T) {
	doc := `query Example { a }`
	data := map[string]any{
		"a": "b",
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"a": "b",
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
		Root:   data,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestUsesTheNamedOperationIfOperationNameIsProvided(t *testing.T) {
	doc := `query Example { first: a } query OtherExample { second: a }`
	data := map[string]any{
		"a": "b",
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"second": "b",
		},
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema:        schema,
		AST:           ast,
		Root:          data,
		OperationName: "OtherExample",
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestThrowsIfNoOperationIsProvided(t *testing.T) {
	doc := `fragment Example on Type { a }`
	data := map[string]any{
		"a": "b",
	}

	expectedErrors := []gqlerrors.FormattedError{
		{
			Message:   "Must provide an operation.",
			Locations: []location.SourceLocation{},
			Type:      "INTERNAL",
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
		Root:   data,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != 1 {
		t.Fatalf("wrong result, expected len(1) unexpected len: %v", len(result.Errors))
	}
	if result.Data != nil {
		t.Fatalf("wrong result, expected nil result.Data, got %v", result.Data)
	}
	result.Errors[0].OriginalError = nil
	result.Errors[0].StackTrace = ""
	if !reflect.DeepEqual(expectedErrors, result.Errors) {
		t.Fatalf("unexpected result, Diff: %v", testutil.Diff(expectedErrors, result.Errors))
	}
}

func TestThrowsIfNoOperationNameIsProvidedWithMultipleOperations(t *testing.T) {
	doc := `query Example { a } query OtherExample { a }`
	data := map[string]any{
		"a": "b",
	}

	expectedErrors := []gqlerrors.FormattedError{
		{
			Type:      "INTERNAL",
			Message:   "Must provide operation name if query contains multiple operations.",
			Locations: []location.SourceLocation{},
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
		Root:   data,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != 1 {
		t.Fatalf("wrong result, expected len(1) unexpected len: %v", len(result.Errors))
	}
	if result.Data != nil {
		t.Fatalf("wrong result, expected nil result.Data, got %v", result.Data)
	}
	result.Errors[0].OriginalError = nil
	result.Errors[0].StackTrace = ""
	if !reflect.DeepEqual(expectedErrors, result.Errors) {
		t.Fatalf("unexpected result, Diff: %v", testutil.Diff(expectedErrors, result.Errors))
	}
}

func TestThrowsIfUnknownOperationNameIsProvided(t *testing.T) {
	doc := `query Example { a } query OtherExample { a }`
	data := map[string]any{
		"a": "b",
	}

	expectedErrors := []gqlerrors.FormattedError{
		{
			Message:   `Unknown operation named "UnknownExample".`,
			Locations: []location.SourceLocation{},
			Type:      "INTERNAL",
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema:        schema,
		AST:           ast,
		Root:          data,
		OperationName: "UnknownExample",
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if result.Data != nil {
		t.Fatalf("wrong result, expected nil result.Data, got %v", result.Data)
	}
	result.Errors[0].OriginalError = nil
	result.Errors[0].StackTrace = ""
	if !reflect.DeepEqual(expectedErrors, result.Errors) {
		t.Fatalf("unexpected result, Diff: %v", testutil.Diff(expectedErrors, result.Errors))
	}
}
func TestUsesTheQuerySchemaForQueries(t *testing.T) {
	doc := `query Q { a } mutation M { c } subscription S { a }`
	data := map[string]any{
		"a": "b",
		"c": "d",
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"a": "b",
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Q",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name: "M",
			Fields: graphql.Fields{
				"c": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "S",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema:        schema,
		AST:           ast,
		Root:          data,
		OperationName: "Q",
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestUsesTheMutationSchemaForMutations(t *testing.T) {
	doc := `query Q { a } mutation M { c }`
	data := map[string]any{
		"a": "b",
		"c": "d",
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"c": "d",
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Q",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name: "M",
			Fields: graphql.Fields{
				"c": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema:        schema,
		AST:           ast,
		Root:          data,
		OperationName: "M",
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestUsesTheSubscriptionSchemaForSubscriptions(t *testing.T) {
	doc := `query Q { a } subscription S { a }`
	data := map[string]any{
		"a": "b",
		"c": "d",
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"a": "b",
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Q",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "S",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema:        schema,
		AST:           ast,
		Root:          data,
		OperationName: "S",
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestCorrectFieldOrderingDespiteExecutionOrder(t *testing.T) {
	doc := `
	{
      b,
      a,
      c,
      d,
      e
    }
	`
	data := map[string]any{
		"a": func() any { return "a" },
		"b": func() any { return "b" },
		"c": func() any { return "c" },
		"d": func() any { return "d" },
		"e": func() any { return "e" },
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"a": "a",
			"b": "b",
			"c": "c",
			"d": "d",
			"e": "e",
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
				"b": &graphql.Field{
					Type: graphql.String,
				},
				"c": &graphql.Field{
					Type: graphql.String,
				},
				"d": &graphql.Field{
					Type: graphql.String,
				},
				"e": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
		Root:   data,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}

	// TODO: test to ensure key ordering
	// The following does not work
	// - iterating over result.Data map
	//   Note that golang's map iteration order is randomized
	//   So, iterating over result.Data won't do it for a test
	// - Marshal the result.Data to json string and assert it
	//   json.Marshal seems to re-sort the keys automatically
	//
	t.Skipf("TODO: Ensure key ordering")
}

func TestAvoidsRecursion(t *testing.T) {
	doc := `
      query Q {
        a
        ...Frag
        ...Frag
      }

      fragment Frag on Type {
        a,
        ...Frag
      }
    `
	data := map[string]any{
		"a": "b",
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"a": "b",
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema:        schema,
		AST:           ast,
		Root:          data,
		OperationName: "Q",
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDoesNotIncludeIllegalFieldsInOutput(t *testing.T) {
	doc := `mutation M {
      thisIsIllegalDontIncludeMe
    }`

	expected := &graphql.Result{
		Data: map[string]any{},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Q",
			Fields: graphql.Fields{
				"a": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name: "M",
			Fields: graphql.Fields{
				"c": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, expected len(%v) errors, got len(%v)", len(expected.Errors), len(result.Errors))
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestDoesNotIncludeArgumentsThatWereNotSet(t *testing.T) {
	doc := `{ field(a: true, c: false, e: 0) }`

	expected := &graphql.Result{
		Data: map[string]any{
			"field": `{"a":true,"c":false,"e":0}`,
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Type",
			Fields: graphql.Fields{
				"field": &graphql.Field{
					Type: graphql.String,
					Args: graphql.FieldConfigArgument{
						"a": &graphql.ArgumentConfig{
							Type: graphql.Boolean,
						},
						"b": &graphql.ArgumentConfig{
							Type: graphql.Boolean,
						},
						"c": &graphql.ArgumentConfig{
							Type: graphql.Boolean,
						},
						"d": &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
						"e": &graphql.ArgumentConfig{
							Type: graphql.Int,
						},
					},
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						args, _ := json.Marshal(p.Args)
						return string(args), nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

type testSpecialType struct {
	Value string
}
type testNotSpecialType struct {
	Value string
}

func TestFailsWhenAnIsTypeOfCheckIsNotMet(t *testing.T) {
	query := `{ specials { value } }`

	data := map[string]any{
		"specials": []any{
			testSpecialType{"foo"},
			testNotSpecialType{"bar"},
		},
	}

	expected := &graphql.Result{
		Data: map[string]any{
			"specials": []any{
				map[string]any{
					"value": "foo",
				},
				nil,
			},
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:      "INTERNAL",
				Message:   `Expected value of type "SpecialType" but got: graphql_test.testNotSpecialType.`,
				Locations: []location.SourceLocation{},
			},
		},
	}

	specialType := graphql.NewObject(graphql.ObjectConfig{
		Name: "SpecialType",
		IsTypeOf: func(p graphql.IsTypeOfParams) bool {
			if _, ok := p.Value.(testSpecialType); ok {
				return true
			}
			return false
		},
		Fields: graphql.Fields{
			"value": &graphql.Field{
				Type: graphql.String,
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return p.Source.(testSpecialType).Value, nil
				},
			},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"specials": &graphql.Field{
					Type: graphql.NewList(specialType),
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return p.Source.(map[string]any)["specials"], nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, query)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
		Root:   data,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) == 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	result.Errors[0].OriginalError = nil
	result.Errors[0].StackTrace = ""
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestFailsToExecuteQueryContainingATypeDefinition(t *testing.T) {
	query := `
      { foo }

      type Query { foo: String }
	`
	expected := &graphql.Result{
		Data: nil,
		Errors: []gqlerrors.FormattedError{
			{
				Type:      "INTERNAL",
				Message:   "GraphQL cannot execute a request containing a ObjectDefinition",
				Locations: []location.SourceLocation{},
			},
		},
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"foo": &graphql.Field{
					Type: graphql.String,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, query)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != 1 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	result.Errors[0].OriginalError = nil
	result.Errors[0].StackTrace = ""
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestQuery_ExecutionAddsErrorsFromFieldResolveFn(t *testing.T) {
	qError := errors.New("queryError")
	q := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Type: graphql.String,
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return nil, qError
				},
			},
			"b": &graphql.Field{
				Type: graphql.String,
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return "ok", nil
				},
			},
		},
	})
	blogSchema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: q,
	})
	if err != nil {
		t.Fatalf("unexpected error, got: %v", err)
	}
	query := "{ a }"
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        blogSchema,
		RequestString: query,
	})
	if len(result.Errors) == 0 {
		t.Fatal("wrong result, expected errors, got no errors")
	}
	if result.Errors[0].Error() != qError.Error() {
		t.Fatalf("wrong result, unexpected error, got: %v, expected: %v", result.Errors[0], qError)
	}
}

func TestQuery_ExecutionDoesNotAddErrorsFromFieldResolveFn(t *testing.T) {
	qError := errors.New("queryError")
	q := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Type: graphql.String,
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return nil, qError
				},
			},
			"b": &graphql.Field{
				Type: graphql.String,
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return "ok", nil
				},
			},
		},
	})
	blogSchema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: q,
	})
	if err != nil {
		t.Fatalf("unexpected error, got: %v", err)
	}
	query := "{ b }"
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        blogSchema,
		RequestString: query,
	})
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %+v", result.Errors)
	}
}

func TestMutation_ExecutionAddsErrorsFromFieldResolveFn(t *testing.T) {
	mError := errors.New("mutationError")
	q := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Type: graphql.String,
			},
		},
	})
	m := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"foo": &graphql.Field{
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					"f": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return nil, mError
				},
			},
			"bar": &graphql.Field{
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					"b": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return "ok", nil
				},
			},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    q,
		Mutation: m,
	})
	if err != nil {
		t.Fatalf("unexpected error, got: %v", err)
	}
	query := "mutation _ { newFoo: foo(f:\"title\") }"
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) == 0 {
		t.Fatal("wrong result, expected errors, got no errors")
	}
	if result.Errors[0].Error() != mError.Error() {
		t.Fatalf("wrong result, unexpected error, got: %v, expected: %v", result.Errors[0], mError)
	}
}

func TestMutation_ExecutionDoesNotAddErrorsFromFieldResolveFn(t *testing.T) {
	mError := errors.New("mutationError")
	q := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Type: graphql.String,
			},
		},
	})
	m := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"foo": &graphql.Field{
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					"f": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return nil, mError
				},
			},
			"bar": &graphql.Field{
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					"b": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return "ok", nil
				},
			},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    q,
		Mutation: m,
	})
	if err != nil {
		t.Fatalf("unexpected error, got: %v", err)
	}
	query := "mutation _ { newBar: bar(b:\"title\") }"
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %+v", result.Errors)
	}
}

func TestMutation_NonNullSubField(t *testing.T) {
	queryType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"a": &graphql.Field{
				Type: graphql.String,
			},
		},
	})
	accountType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Account",
		Fields: graphql.Fields{
			"id": &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
		},
	})
	authenticatePayloadType := graphql.NewObject(graphql.ObjectConfig{
		Name: "AuthenticatePayload",
		Fields: graphql.Fields{
			"account": &graphql.Field{Type: accountType},
		},
	})
	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"authenticate": &graphql.Field{
				Type: graphql.NewNonNull(authenticatePayloadType),
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return struct {
						Account *struct{} `json:"account"`
					}{
						Account: nil,
					}, nil
				},
			},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	})
	if err != nil {
		t.Fatalf("unexpected error, got: %v", err)
	}
	query := "mutation _ { authenticate { account { id } } }"
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %+v", result.Errors)
	}
}

func TestGraphqlTag(t *testing.T) {
	typeObjectType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Type",
		Fields: graphql.Fields{
			"fooBar": &graphql.Field{Type: graphql.String},
		},
	})
	var baz = &graphql.Field{
		Type:        typeObjectType,
		Description: "typeObjectType",
		Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
			t := struct {
				FooBar string `graphql:"fooBar"`
			}{"foo bar value"}
			return t, nil
		},
	}
	q := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"baz": baz,
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: q,
	})
	if err != nil {
		t.Fatalf("unexpected error, got: %v", err)
	}
	query := "{ baz { fooBar } }"
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) != 0 {
		t.Fatalf("wrong result, unexpected errors: %+v", result.Errors)
	}
	expectedData := map[string]any{
		"baz": map[string]any{
			"fooBar": "foo bar value",
		},
	}
	if !reflect.DeepEqual(result.Data, expectedData) {
		t.Fatalf("unexpected result, got: %+v, expected: %+v", expectedData, result.Data)
	}
}

func TestContextDeadline(t *testing.T) {
	timeout := time.Millisecond * time.Duration(100)
	acceptableDelay := time.Millisecond * time.Duration(10)
	expectedErrors := []gqlerrors.FormattedError{
		{
			Message:   context.DeadlineExceeded.Error(),
			Locations: []location.SourceLocation{},
			Type:      "INTERNAL",
		},
	}

	// Query type includes a field that won't resolve within the deadline
	var queryType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"hello": &graphql.Field{
					Type: graphql.String,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						time.Sleep(2 * time.Second)
						return "world", nil
					},
				},
			},
		})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		t.Fatalf("unexpected error, got: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()
	result := graphql.Do(ctx, graphql.Params{
		Schema:        schema,
		RequestString: "{hello}",
	})
	duration := time.Since(startTime)

	if duration > timeout+acceptableDelay {
		t.Fatalf("graphql.Do completed in %s, should have completed in %s", duration, timeout)
	}
	if !result.HasErrors() || len(result.Errors) == 0 {
		t.Fatalf("Result should include errors when deadline is exceeded")
	}
	result.Errors[0].OriginalError = nil
	result.Errors[0].StackTrace = ""
	if !reflect.DeepEqual(expectedErrors, result.Errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expectedErrors, result.Errors))
	}
}

func TestContextDeadlineWait(t *testing.T) {
	timeout := time.Millisecond * time.Duration(100)
	acceptableDelay := time.Millisecond * time.Duration(10)
	expectedErrors := []gqlerrors.FormattedError{
		{
			Message:   context.DeadlineExceeded.Error(),
			Locations: []location.SourceLocation{},
			Type:      "INTERNAL",
		},
	}

	// Query type includes a field that won't resolve within the deadline
	var queryType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"hello": &graphql.Field{
					Type: graphql.String,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						<-ctx.Done()
						return nil, fmt.Errorf("Resolvers: %w", ctx.Err())
					},
				},
			},
		})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		t.Fatalf("unexpected error, got: %v", err)
	}

	// 0 timeout wait

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime := time.Now()
	result := graphql.Do(ctx, graphql.Params{
		Schema:        schema,
		RequestString: "{hello}",
	})
	duration := time.Since(startTime)

	if duration > timeout+acceptableDelay {
		t.Fatalf("graphql.Do completed in %s, should have completed in %s", duration, timeout)
	}
	if !result.HasErrors() || len(result.Errors) == 0 {
		t.Fatalf("Result should include errors when deadline is exceeded")
	}
	result.Errors[0].OriginalError = nil
	result.Errors[0].StackTrace = ""
	if !reflect.DeepEqual(expectedErrors, result.Errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expectedErrors, result.Errors))
	}

	// >0 second timeout wait

	ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startTime = time.Now()
	ast, err := parser.Parse(parser.ParseParams{Source: source.New("GraphQL request", "{hello}")})
	if err != nil {
		t.Fatal(err)
	}
	result = graphql.Execute(ctx, graphql.ExecuteParams{
		Schema:      schema,
		AST:         ast,
		TimeoutWait: time.Second,
	})
	duration = time.Since(startTime)

	if duration > timeout+acceptableDelay {
		t.Fatalf("graphql.Do completed in %s, should have completed in %s", duration, timeout)
	}
	if !result.HasErrors() || len(result.Errors) == 0 {
		t.Fatalf("Result should include errors when deadline is exceeded")
	}
	if result.Errors[0].Error() != "Resolvers: context deadline exceeded" {
		t.Fatalf("Unexpected 'Resolvers: context deadline exceeded' got '%s'", result.Errors[0].Error())
	}
}

func TestContextCancel(t *testing.T) {
	expectedErrors := []gqlerrors.FormattedError{
		{
			Message:   context.Canceled.Error(),
			Locations: []location.SourceLocation{},
			Type:      "INTERNAL",
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Query type includes a field that won't resolve within the deadline
	var resolveCount int
	var queryType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"hello1": &graphql.Field{
					Type: graphql.String,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						resolveCount++
						cancel()
						return "hello1", nil
					},
				},
				"hello2": &graphql.Field{
					Type: graphql.String,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						resolveCount++
						cancel()
						return "hello2", nil
					},
				},
			},
		})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		t.Fatalf("unexpected error, got: %v", err)
	}

	result := graphql.Do(ctx, graphql.Params{
		Schema:        schema,
		RequestString: "{hello1 hello2}",
	})

	if !result.HasErrors() || len(result.Errors) == 0 {
		t.Fatalf("Result should include errors when deadline is exceeded")
	}
	result.Errors[0].OriginalError = nil
	result.Errors[0].StackTrace = ""
	if !reflect.DeepEqual(expectedErrors, result.Errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expectedErrors, result.Errors))
	}
	if resolveCount != 1 {
		t.Fatalf("Expected only 1 resolver to be called")
	}
}

func TestDeprecatedField(t *testing.T) {
	query := `
		query _ {
			foo
			bar
		}
    `

	expected := &graphql.Result{
		Data: map[string]any{
			"foo": "bar",
			"bar": "foo",
		},
	}

	type LikeAString string

	const fooEnumValue LikeAString = "foo"

	enumType := graphql.NewEnum(graphql.EnumConfig{
		Name: "Bar",
		Values: graphql.EnumValueConfigMap{
			string(fooEnumValue): &graphql.EnumValueConfig{
				Value: fooEnumValue,
			},
		},
	})

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"foo": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return LikeAString("bar"), nil
					},
				},
				"bar": &graphql.Field{
					Type: graphql.NewNonNull(enumType),
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return fooEnumValue, nil
					},
					DeprecationReason: "Use subtitleMarkup and bodyMarkup",
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %s", err)
	}

	// parse query
	astDoc := testutil.TestParse(t, query)

	var depField string
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    astDoc,
		DeprecatedFieldFn: func(ctx context.Context, parent *graphql.Object, fd *graphql.FieldDefinition) error {
			if fd != nil && parent != nil {
				depField = fmt.Sprintf("%s.%s", parent.Name(), fd.Name)
			}
			return nil
		},
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
	if depField != "Query.bar" {
		t.Fatalf("Expected deprecated field \"Query.bar\" got %q", depField)
	}

	ep = graphql.ExecuteParams{
		Schema: schema,
		AST:    astDoc,
		DeprecatedFieldFn: func(ctx context.Context, parent *graphql.Object, fd *graphql.FieldDefinition) error {
			return errors.New("deprecated field")
		},
	}
	result = testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) == 0 {
		t.Fatal("Expecte an error")
	}
	if result.Errors[0].Message != "deprecated field" {
		t.Fatalf("Expected \"deprecated field\" error got %+#v", result.Errors[0])
	}
}

func TestCoroutines(t *testing.T) {
	query := `
		query _ {
			foo
			bar
		}
    `

	expected := &graphql.Result{
		Data: map[string]any{
			"foo": "bar",
			"bar": "foo",
		},
	}

	const fooEnumValue = "foo"

	enumType := graphql.NewEnum(graphql.EnumConfig{
		Name: "Bar",
		Values: graphql.EnumValueConfigMap{
			string(fooEnumValue): &graphql.EnumValueConfig{
				Value: fooEnumValue,
			},
		},
	})

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"foo": &graphql.Field{
					Type: graphql.NewNonNull(graphql.String),
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						graphql.PauseCoroutine(ctx)
						return "bar", nil
					},
				},
				"bar": &graphql.Field{
					Type: graphql.NewNonNull(enumType),
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return fooEnumValue, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("Error in schema %s", err)
	}

	// parse query
	astDoc := testutil.TestParse(t, query)

	ep := graphql.ExecuteParams{
		Schema:           schema,
		AST:              astDoc,
		EnableCoroutines: true,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
