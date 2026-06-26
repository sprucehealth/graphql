package graphql_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/testutil"
)

type T struct {
	Query    string
	Schema   graphql.Schema
	Expected any
}

var Tests = []T{}

func init() {
	Tests = []T{
		{
			Query: `
				query HeroNameQuery {
					hero {
						name
					}
				}
			`,
			Schema: testutil.StarWarsSchema,
			Expected: &graphql.Result{
				Data: map[string]any{
					"hero": map[string]any{
						"name": "R2-D2",
					},
				},
			},
		},
		{
			Query: `
				query HeroNameAndFriendsQuery {
					hero {
						id
						name
						friends {
							name
						}
					}
				}
			`,
			Schema: testutil.StarWarsSchema,
			Expected: &graphql.Result{
				Data: map[string]any{
					"hero": map[string]any{
						"id":   "2001",
						"name": "R2-D2",
						"friends": []any{
							map[string]any{
								"name": "Luke Skywalker",
							},
							map[string]any{
								"name": "Han Solo",
							},
							map[string]any{
								"name": "Leia Organa",
							},
						},
					},
				},
			},
		},
	}
}

func TestQuery(t *testing.T) {
	for _, test := range Tests {
		params := graphql.Params{
			Schema:        test.Schema,
			RequestString: test.Query,
		}
		testGraphql(test, params, t)
	}
}

func testGraphql(test T, p graphql.Params, t *testing.T) {
	result := graphql.Do(context.Background(), p)
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(result, test.Expected) {
		t.Fatalf("wrong result, query: %v, graphql result diff: %v", test.Query, testutil.Diff(test.Expected, result))
	}
}

func TestBasicGraphQLExample(t *testing.T) {
	// taken from `graphql-js` README

	helloFieldResolved := func(ctx context.Context, p graphql.ResolveParams) (any, error) {
		return "world", nil
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "RootQueryType",
			Fields: graphql.Fields{
				"hello": &graphql.Field{
					Description: "Returns `world`",
					Type:        graphql.String,
					Resolve:     helloFieldResolved,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("wrong result, unexpected errors: %v", err.Error())
	}
	query := "{ hello }"
	expected := map[string]any{
		"hello": "world",
	}

	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	if !reflect.DeepEqual(result.Data, expected) {
		t.Fatalf("wrong result, query: %v, graphql result diff: %v", query, testutil.Diff(expected, result))
	}
}

func TestThreadsContextFromParamsThrough(t *testing.T) {
	extractFieldFromContextFn := func(ctx context.Context, p graphql.ResolveParams) (any, error) {
		return ctx.Value(p.Args["key"]), nil
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"value": &graphql.Field{
					Type: graphql.String,
					Args: graphql.FieldConfigArgument{
						"key": &graphql.ArgumentConfig{Type: graphql.String},
					},
					Resolve: extractFieldFromContextFn,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("wrong result, unexpected errors: %v", err.Error())
	}
	query := `{ value(key:"a") }`

	//nolint:staticcheck
	result := graphql.Do(context.WithValue(context.Background(), "a", "xyz"), graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	expected := map[string]any{"value": "xyz"}
	if !reflect.DeepEqual(result.Data, expected) {
		t.Fatalf("wrong result, query: %v, graphql result diff: %v", query, testutil.Diff(expected, result))
	}
}

func TestEmptyStringIsNotNull(t *testing.T) {
	checkForEmptyString := func(ctx context.Context, p graphql.ResolveParams) (any, error) {
		arg := p.Args["arg"]
		if arg == nil || arg.(string) != "" {
			t.Errorf("Expected empty string for input arg, got %#v", arg)
		}
		return "yay", nil
	}
	returnEmptyString := func(ctx context.Context, p graphql.ResolveParams) (any, error) {
		return "", nil
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"checkEmptyArg": &graphql.Field{
					Type: graphql.String,
					Args: graphql.FieldConfigArgument{
						"arg": &graphql.ArgumentConfig{Type: graphql.String},
					},
					Resolve: checkForEmptyString,
				},
				"checkEmptyResult": &graphql.Field{
					Type:    graphql.String,
					Resolve: returnEmptyString,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("wrong result, unexpected errors: %v", err.Error())
	}
	query := `{ checkEmptyArg(arg:"") checkEmptyResult }`

	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
	}
	expected := map[string]any{"checkEmptyArg": "yay", "checkEmptyResult": ""}
	if !reflect.DeepEqual(result.Data, expected) {
		t.Errorf("wrong result, query: %v, graphql result diff: %v", query, testutil.Diff(expected, result))
	}
}

func TestBoolPointer(t *testing.T) {
	tr := true
	fa := false
	for _, exp := range []*bool{nil, &tr, &fa} {
		trueField := func(ctx context.Context, p graphql.ResolveParams) (any, error) {
			return exp, nil
		}

		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{
				Name: "RootQueryType",
				Fields: graphql.Fields{
					"allowed": &graphql.Field{
						Description: "Returns true",
						Type:        graphql.Boolean,
						Resolve:     trueField,
					},
				},
			}),
		})
		if err != nil {
			t.Fatalf("wrong result, unexpected errors: %v", err.Error())
		}
		query := "{ allowed }"
		expected := map[string]any{
			"allowed": nil,
		}
		if exp != nil {
			expected["allowed"] = *exp
		}

		result := graphql.Do(context.Background(), graphql.Params{
			Schema:        schema,
			RequestString: query,
		})
		if len(result.Errors) > 0 {
			t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
		}
		if !reflect.DeepEqual(result.Data, expected) {
			t.Fatalf("wrong result, query: %v, graphql result diff: %v", query, testutil.Diff(expected, result.Data))
		}
	}
}

func TestNullScalarsMap(t *testing.T) {
	objType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ReturnObj",
		Fields: graphql.Fields{
			"string": &graphql.Field{Type: graphql.String},
			"int":    &graphql.Field{Type: graphql.Int},
			"bool":   &graphql.Field{Type: graphql.Boolean},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"returnNull": &graphql.Field{
					Type: objType,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return map[string]any{
							"string": nil,
							"int":    nil,
							"bool":   nil,
						}, nil
					},
				},
				"returnZero": &graphql.Field{
					Type: objType,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return map[string]any{
							"string": "",
							"int":    0,
							"bool":   false,
						}, nil
					},
				},
				"returnSomething": &graphql.Field{
					Type: objType,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return map[string]any{
							"string": "hello",
							"int":    1,
							"bool":   true,
						}, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("wrong result, unexpected errors: %v", err.Error())
	}

	cases := map[string]any{
		`{ returnNull { string int bool } }`:      map[string]any{"returnNull": map[string]any{"string": nil, "int": nil, "bool": nil}},
		`{ returnZero { string int bool } }`:      map[string]any{"returnZero": map[string]any{"string": "", "int": 0, "bool": false}},
		`{ returnSomething { string int bool } }`: map[string]any{"returnSomething": map[string]any{"string": "hello", "int": 1, "bool": true}},
	}

	for query, expected := range cases {
		result := graphql.Do(context.Background(), graphql.Params{
			Schema:        schema,
			RequestString: query,
		})
		if len(result.Errors) > 0 {
			t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
		}
		if !reflect.DeepEqual(result.Data, expected) {
			t.Errorf("wrong result, query: %v, graphql result diff: %v", query, testutil.Diff(expected, result))
		}
	}
}

func TestNullScalarsStruct(t *testing.T) {
	objType := graphql.NewObject(graphql.ObjectConfig{
		Name: "ReturnObj",
		Fields: graphql.Fields{
			"string": &graphql.Field{Type: graphql.String},
			"int":    &graphql.Field{Type: graphql.Int},
			"bool":   &graphql.Field{Type: graphql.Boolean},
		},
	})
	type resObject struct {
		String *string `json:"string,omitempty"`
		Int    *int    `json:"int,omitempty"`
		Bool   *bool   `json:"bool,omitempty"`
	}
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"returnNull": &graphql.Field{
					Type: objType,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return &resObject{
							String: nil,
							Int:    nil,
							Bool:   nil,
						}, nil
					},
				},
				"returnZero": &graphql.Field{
					Type: objType,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return &resObject{
							String: new(""),
							Int:    new(0),
							Bool:   new(false),
						}, nil
					},
				},
				"returnSomething": &graphql.Field{
					Type: objType,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return &resObject{
							String: new("hello"),
							Int:    new(1),
							Bool:   new(true),
						}, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("wrong result, unexpected errors: %v", err.Error())
	}

	cases := map[string]any{
		`{ returnNull { string int bool } }`:      map[string]any{"returnNull": map[string]any{"string": nil, "int": nil, "bool": nil}},
		`{ returnZero { string int bool } }`:      map[string]any{"returnZero": map[string]any{"string": "", "int": 0, "bool": false}},
		`{ returnSomething { string int bool } }`: map[string]any{"returnSomething": map[string]any{"string": "hello", "int": 1, "bool": true}},
	}

	for query, expected := range cases {
		result := graphql.Do(context.Background(), graphql.Params{
			Schema:        schema,
			RequestString: query,
		})
		if len(result.Errors) > 0 {
			t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
		}
		if !reflect.DeepEqual(result.Data, expected) {
			t.Errorf("wrong result, query: %v, graphql result diff: %v", query, testutil.Diff(expected, result))
		}
	}
}
