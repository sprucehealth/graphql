package graphql_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/location"
	"github.com/sprucehealth/graphql/testutil"
)

var syncError = "sync"
var nonNullSyncError = "nonNullSync"
var promiseError = "promise"
var nonNullPromiseError = "nonNullPromise"

var throwingData = map[string]any{
	"sync": func() any {
		panic(syncError)
	},
	"nonNullSync": func() any {
		panic(nonNullSyncError)
	},
	"promise": func() any {
		panic(promiseError)
	},
	"nonNullPromise": func() any {
		panic(nonNullPromiseError)
	},
}

var nullingData = map[string]any{
	"sync": func() any {
		return nil
	},
	"nonNullSync": func() any {
		return nil
	},
	"promise": func() any {
		return nil
	},
	"nonNullPromise": func() any {
		return nil
	},
}

var dataType = graphql.NewObject(graphql.ObjectConfig{
	Name: "DataType",
	Fields: graphql.Fields{
		"sync": &graphql.Field{
			Type: graphql.String,
		},
		"nonNullSync": &graphql.Field{
			Type: graphql.NewNonNull(graphql.String),
		},
		"promise": &graphql.Field{
			Type: graphql.String,
		},
		"nonNullPromise": &graphql.Field{
			Type: graphql.NewNonNull(graphql.String),
		},
	},
})

var nonNullTestSchema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: dataType,
})

func init() {
	throwingData["nest"] = func() any {
		return throwingData
	}
	throwingData["nonNullNest"] = func() any {
		return throwingData
	}
	throwingData["promiseNest"] = func() any {
		return throwingData
	}
	throwingData["nonNullPromiseNest"] = func() any {
		return throwingData
	}

	nullingData["nest"] = func() any {
		return nullingData
	}
	nullingData["nonNullNest"] = func() any {
		return nullingData
	}
	nullingData["promiseNest"] = func() any {
		return nullingData
	}
	nullingData["nonNullPromiseNest"] = func() any {
		return nullingData
	}

	dataType.AddFieldConfig("nest", &graphql.Field{
		Type: dataType,
	})
	dataType.AddFieldConfig("nonNullNest", &graphql.Field{
		Type: graphql.NewNonNull(dataType),
	})
	dataType.AddFieldConfig("promiseNest", &graphql.Field{
		Type: dataType,
	})
	dataType.AddFieldConfig("nonNullPromiseNest", &graphql.Field{
		Type: graphql.NewNonNull(dataType),
	})
}

// nulls a nullable field that panics
func TestNonNull_NullsANullableFieldThatThrowsSynchronously(t *testing.T) {
	doc := `
      query Q {
        sync
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"sync": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: syncError,
				Locations: []location.SourceLocation{
					{
						Line: 3, Column: 9,
					},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   throwingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsANullableFieldThatThrowsInAPromise(t *testing.T) {
	doc := `
      query Q {
        promise
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"promise": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: promiseError,
				Locations: []location.SourceLocation{
					{
						Line: 3, Column: 9,
					},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   throwingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsASynchronouslyReturnedObjectThatContainsANullableFieldThatThrowsSynchronously(t *testing.T) {
	doc := `
      query Q {
        nest {
          nonNullSync,
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: nonNullSyncError,
				Locations: []location.SourceLocation{
					{
						Line: 4, Column: 11,
					},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   throwingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsASynchronouslyReturnedObjectThatContainsANonNullableFieldThatThrowsInAPromise(t *testing.T) {
	doc := `
      query Q {
        nest {
          nonNullPromise,
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: nonNullPromiseError,
				Locations: []location.SourceLocation{
					{
						Line: 4, Column: 11,
					},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   throwingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsAnObjectReturnedInAPromiseThatContainsANonNullableFieldThatThrowsSynchronously(t *testing.T) {
	doc := `
      query Q {
        promiseNest {
          nonNullSync,
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"promiseNest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: nonNullSyncError,
				Locations: []location.SourceLocation{
					{
						Line: 4, Column: 11,
					},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   throwingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsAnObjectReturnedInAPromiseThatContainsANonNullableFieldThatThrowsInAPromise(t *testing.T) {
	doc := `
      query Q {
        promiseNest {
          nonNullPromise,
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"promiseNest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: nonNullPromiseError,
				Locations: []location.SourceLocation{
					{
						Line: 4, Column: 11,
					},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   throwingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestNonNull_NullsAComplexTreeOfNullableFieldsThatThrow(t *testing.T) {
	doc := `
      query Q {
        nest {
          sync
          promise
          nest {
            sync
            promise
          }
          promiseNest {
            sync
            promise
          }
        }
        promiseNest {
          sync
          promise
          nest {
            sync
            promise
          }
          promiseNest {
            sync
            promise
          }
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"sync":    nil,
				"promise": nil,
				"nest": map[string]any{
					"sync":    nil,
					"promise": nil,
				},
				"promiseNest": map[string]any{
					"sync":    nil,
					"promise": nil,
				},
			},
			"promiseNest": map[string]any{
				"sync":    nil,
				"promise": nil,
				"nest": map[string]any{
					"sync":    nil,
					"promise": nil,
				},
				"promiseNest": map[string]any{
					"sync":    nil,
					"promise": nil,
				},
			},
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: syncError,
				Locations: []location.SourceLocation{
					{Line: 4, Column: 11},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: syncError,
				Locations: []location.SourceLocation{
					{Line: 7, Column: 13},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: syncError,
				Locations: []location.SourceLocation{
					{Line: 11, Column: 13},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: syncError,
				Locations: []location.SourceLocation{
					{Line: 16, Column: 11},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: syncError,
				Locations: []location.SourceLocation{
					{Line: 19, Column: 13},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: syncError,
				Locations: []location.SourceLocation{
					{Line: 23, Column: 13},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: promiseError,
				Locations: []location.SourceLocation{
					{Line: 5, Column: 11},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: promiseError,
				Locations: []location.SourceLocation{
					{Line: 8, Column: 13},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: promiseError,
				Locations: []location.SourceLocation{
					{Line: 12, Column: 13},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: promiseError,
				Locations: []location.SourceLocation{
					{Line: 17, Column: 11},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: promiseError,
				Locations: []location.SourceLocation{
					{Line: 20, Column: 13},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: promiseError,
				Locations: []location.SourceLocation{
					{Line: 24, Column: 13},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   throwingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	if !reflect.DeepEqual(expected.Data, result.Data) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Data, result.Data))
	}
	for i := range result.Errors {
		result.Errors[i].OriginalError = nil
	}
	sort.Sort(gqlerrors.FormattedErrors(expected.Errors))
	sort.Sort(gqlerrors.FormattedErrors(result.Errors))
	if !reflect.DeepEqual(expected.Errors, result.Errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
}
func TestNonNull_NullsTheFirstNullableObjectAfterAFieldThrowsInALongChainOfFieldsThatAreNonNull(t *testing.T) {
	doc := `
      query Q {
        nest {
          nonNullNest {
            nonNullPromiseNest {
              nonNullNest {
                nonNullPromiseNest {
                  nonNullSync
                }
              }
            }
          }
        }
        promiseNest {
          nonNullNest {
            nonNullPromiseNest {
              nonNullNest {
                nonNullPromiseNest {
                  nonNullSync
                }
              }
            }
          }
        }
        anotherNest: nest {
          nonNullNest {
            nonNullPromiseNest {
              nonNullNest {
                nonNullPromiseNest {
                  nonNullPromise
                }
              }
            }
          }
        }
        anotherPromiseNest: promiseNest {
          nonNullNest {
            nonNullPromiseNest {
              nonNullNest {
                nonNullPromiseNest {
                  nonNullPromise
                }
              }
            }
          }
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"nest":               nil,
			"promiseNest":        nil,
			"anotherNest":        nil,
			"anotherPromiseNest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: nonNullSyncError,
				Locations: []location.SourceLocation{
					{Line: 8, Column: 19},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: nonNullSyncError,
				Locations: []location.SourceLocation{
					{Line: 19, Column: 19},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: nonNullPromiseError,
				Locations: []location.SourceLocation{
					{Line: 30, Column: 19},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: nonNullPromiseError,
				Locations: []location.SourceLocation{
					{Line: 41, Column: 19},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   throwingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	if !reflect.DeepEqual(expected.Data, result.Data) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Data, result.Data))
	}
	for i := range result.Errors {
		result.Errors[i].OriginalError = nil
	}
	sort.Sort(gqlerrors.FormattedErrors(expected.Errors))
	sort.Sort(gqlerrors.FormattedErrors(result.Errors))
	if !reflect.DeepEqual(expected.Errors, result.Errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}

}
func TestNonNull_NullsANullableFieldThatSynchronouslyReturnsNull(t *testing.T) {
	doc := `
      query Q {
        sync
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"sync": nil,
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   nullingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	if !reflect.DeepEqual(expected.Data, result.Data) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Data, result.Data))
	}
	if !reflect.DeepEqual(expected.Errors, result.Errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
}
func TestNonNull_NullsANullableFieldThatSynchronouslyReturnsNullInAPromise(t *testing.T) {
	doc := `
      query Q {
        promise
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"promise": nil,
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   nullingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	if !reflect.DeepEqual(expected.Data, result.Data) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Data, result.Data))
	}
	if !reflect.DeepEqual(expected.Errors, result.Errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
}
func TestNonNull_NullsASynchronouslyReturnedObjectThatContainsANonNullableFieldThatReturnsNullSynchronously(t *testing.T) {
	doc := `
      query Q {
        nest {
          nonNullSync,
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: `Cannot return null for non-nullable field DataType.nonNullSync.`,
				Locations: []location.SourceLocation{
					{Line: 4, Column: 11},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   nullingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsASynchronouslyReturnedObjectThatContainsANonNullableFieldThatReturnsNullInAPromise(t *testing.T) {
	doc := `
      query Q {
        nest {
          nonNullPromise,
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: `Cannot return null for non-nullable field DataType.nonNullPromise.`,
				Locations: []location.SourceLocation{
					{Line: 4, Column: 11},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   nullingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

func TestNonNull_NullsAnObjectReturnedInAPromiseThatContainsANonNullableFieldThatReturnsNullSynchronously(t *testing.T) {
	doc := `
      query Q {
        promiseNest {
          nonNullSync,
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"promiseNest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: `Cannot return null for non-nullable field DataType.nonNullSync.`,
				Locations: []location.SourceLocation{
					{Line: 4, Column: 11},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   nullingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsAnObjectReturnedInAPromiseThatContainsANonNullableFieldThatReturnsNullInAPromise(t *testing.T) {
	doc := `
      query Q {
        promiseNest {
          nonNullPromise,
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"promiseNest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: `Cannot return null for non-nullable field DataType.nonNullPromise.`,
				Locations: []location.SourceLocation{
					{Line: 4, Column: 11},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   nullingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsAComplexTreeOfNullableFieldsThatReturnNull(t *testing.T) {
	doc := `
      query Q {
        nest {
          sync
          promise
          nest {
            sync
            promise
          }
          promiseNest {
            sync
            promise
          }
        }
        promiseNest {
          sync
          promise
          nest {
            sync
            promise
          }
          promiseNest {
            sync
            promise
          }
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"sync":    nil,
				"promise": nil,
				"nest": map[string]any{
					"sync":    nil,
					"promise": nil,
				},
				"promiseNest": map[string]any{
					"sync":    nil,
					"promise": nil,
				},
			},
			"promiseNest": map[string]any{
				"sync":    nil,
				"promise": nil,
				"nest": map[string]any{
					"sync":    nil,
					"promise": nil,
				},
				"promiseNest": map[string]any{
					"sync":    nil,
					"promise": nil,
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   nullingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	if !reflect.DeepEqual(expected.Data, result.Data) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Data, result.Data))
	}
	if !reflect.DeepEqual(expected.Errors, result.Errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
}
func TestNonNull_NullsTheFirstNullableObjectAfterAFieldReturnsNullInALongChainOfFieldsThatAreNonNull(t *testing.T) {
	doc := `
      query Q {
        nest {
          nonNullNest {
            nonNullPromiseNest {
              nonNullNest {
                nonNullPromiseNest {
                  nonNullSync
                }
              }
            }
          }
        }
        promiseNest {
          nonNullNest {
            nonNullPromiseNest {
              nonNullNest {
                nonNullPromiseNest {
                  nonNullSync
                }
              }
            }
          }
        }
        anotherNest: nest {
          nonNullNest {
            nonNullPromiseNest {
              nonNullNest {
                nonNullPromiseNest {
                  nonNullPromise
                }
              }
            }
          }
        }
        anotherPromiseNest: promiseNest {
          nonNullNest {
            nonNullPromiseNest {
              nonNullNest {
                nonNullPromiseNest {
                  nonNullPromise
                }
              }
            }
          }
        }
      }
	`
	expected := &graphql.Result{
		Data: map[string]any{
			"nest":               nil,
			"promiseNest":        nil,
			"anotherNest":        nil,
			"anotherPromiseNest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: `Cannot return null for non-nullable field DataType.nonNullSync.`,
				Locations: []location.SourceLocation{
					{Line: 8, Column: 19},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: `Cannot return null for non-nullable field DataType.nonNullSync.`,
				Locations: []location.SourceLocation{
					{Line: 19, Column: 19},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: `Cannot return null for non-nullable field DataType.nonNullPromise.`,
				Locations: []location.SourceLocation{
					{Line: 30, Column: 19},
				},
			},
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: `Cannot return null for non-nullable field DataType.nonNullPromise.`,
				Locations: []location.SourceLocation{
					{Line: 41, Column: 19},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   nullingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	if !reflect.DeepEqual(expected.Data, result.Data) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Data, result.Data))
	}
	for i := range result.Errors {
		result.Errors[i].OriginalError = nil
	}
	sort.Sort(gqlerrors.FormattedErrors(expected.Errors))
	sort.Sort(gqlerrors.FormattedErrors(result.Errors))
	if !reflect.DeepEqual(expected.Errors, result.Errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
}

func TestNonNull_NullsTheTopLevelIfSyncNonNullableFieldThrows(t *testing.T) {
	doc := `
      query Q { nonNullSync }
	`
	expected := &graphql.Result{
		Data: nil,
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: nonNullSyncError,
				Locations: []location.SourceLocation{
					{Line: 2, Column: 17},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   throwingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsTheTopLevelIfSyncNonNullableFieldErrors(t *testing.T) {
	doc := `
      query Q { nonNullPromise }
	`
	expected := &graphql.Result{
		Data: nil,
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: nonNullPromiseError,
				Locations: []location.SourceLocation{
					{Line: 2, Column: 17},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   throwingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsTheTopLevelIfSyncNonNullableFieldReturnsNull(t *testing.T) {
	doc := `
      query Q { nonNullSync }
	`
	expected := &graphql.Result{
		Data: nil,
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: `Cannot return null for non-nullable field DataType.nonNullSync.`,
				Locations: []location.SourceLocation{
					{Line: 2, Column: 17},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   nullingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestNonNull_NullsTheTopLevelIfSyncNonNullableFieldResolvesNull(t *testing.T) {
	doc := `
      query Q { nonNullPromise }
	`
	expected := &graphql.Result{
		Data: nil,
		Errors: []gqlerrors.FormattedError{
			{
				Type:    gqlerrors.ErrorTypeInternal,
				Message: `Cannot return null for non-nullable field DataType.nonNullPromise.`,
				Locations: []location.SourceLocation{
					{Line: 2, Column: 17},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: nonNullTestSchema,
		AST:    ast,
		Root:   nullingData,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	result.Errors[0].OriginalError = nil
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
