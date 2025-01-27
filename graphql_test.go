package graphql_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"context"

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

func TestTracing(t *testing.T) {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "RootQueryType",
			Fields: graphql.Fields{
				"object": &graphql.Field{
					Type: graphql.NewNonNull(graphql.NewObject(graphql.ObjectConfig{
						Name: "First",
						Fields: graphql.Fields{
							"first": &graphql.Field{
								Type: graphql.NewNonNull(graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
									Name: "Seconds",
									Fields: graphql.Fields{
										"second": &graphql.Field{
											Type: graphql.Boolean,
											Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
												return true, nil
											},
										},
									},
								}))),
								Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
									return []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil
								},
							},
						},
					})),
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return true, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("wrong result, unexpected errors: %v", err.Error())
	}
	query := "{ object { first { second }} }"

	for range 10 {
		tr := graphql.NewCountingTracer()
		result := graphql.Do(context.Background(), graphql.Params{
			Schema:        schema,
			RequestString: query,
			Tracer:        tr,
		})
		if len(result.Errors) > 0 {
			t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
		}
		var traces []*graphql.TracePathCount
		for _, tr := range tr.IterTraces() {
			t.Logf("%s %d executions, %s total duration, %s max duration, %s average duration\n",
				strings.Join(tr.Path, "."), tr.Count, tr.TotalDuration, tr.MaxDuration, tr.TotalDuration/time.Duration(tr.Count))
			traces = append(traces, tr)
		}
		if len(traces) != 3 {
			t.Logf("Expected 3 traces, got %d", len(traces))
		}
		// Assume the order of execution which should always be consistent unless the executor changes.
		for i, expCount := range []int{1, 1, 10} {
			trace := traces[i]
			if trace.Count != expCount {
				t.Logf("Expected count of %d for %v, got %d", expCount, trace.Path, trace.Count)
			}
		}
		tr.Recycle()
	}
}
