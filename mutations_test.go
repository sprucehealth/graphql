package graphql_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/location"
	"github.com/sprucehealth/graphql/testutil"
)

// testNumberHolder maps to numberHolderType
type testNumberHolder struct {
	TheNumber int `json:"theNumber"` // map field to `theNumber` so it can be resolve by the default ResolveFn
}
type testRoot struct {
	NumberHolder *testNumberHolder
}

func newTestRoot(originalNumber int) *testRoot {
	return &testRoot{
		NumberHolder: &testNumberHolder{originalNumber},
	}
}
func (r *testRoot) ImmediatelyChangeTheNumber(newNumber int) *testNumberHolder {
	r.NumberHolder.TheNumber = newNumber
	return r.NumberHolder
}
func (r *testRoot) PromiseToChangeTheNumber(newNumber int) *testNumberHolder {
	return r.ImmediatelyChangeTheNumber(newNumber)
}
func (r *testRoot) FailToChangeTheNumber(newNumber int) *testNumberHolder {
	panic("Cannot change the number")
}
func (r *testRoot) PromiseAndFailToChangeTheNumber(newNumber int) *testNumberHolder {
	panic("Cannot change the number")
}

// numberHolderType creates a mapping to testNumberHolder
var numberHolderType = graphql.NewObject(graphql.ObjectConfig{
	Name: "NumberHolder",
	Fields: graphql.Fields{
		"theNumber": &graphql.Field{
			Type: graphql.Int,
		},
	},
})

var mutationsTestSchema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"numberHolder": &graphql.Field{
				Type: numberHolderType,
			},
		},
	}),
	Mutation: graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"immediatelyChangeTheNumber": &graphql.Field{
				Type: numberHolderType,
				Args: graphql.FieldConfigArgument{
					"newNumber": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					newNumber := 0
					obj, _ := p.Source.(*testRoot)
					newNumber, _ = p.Args["newNumber"].(int)
					return obj.ImmediatelyChangeTheNumber(newNumber), nil
				},
			},
			"promiseToChangeTheNumber": &graphql.Field{
				Type: numberHolderType,
				Args: graphql.FieldConfigArgument{
					"newNumber": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					newNumber := 0
					obj, _ := p.Source.(*testRoot)
					newNumber, _ = p.Args["newNumber"].(int)
					return obj.PromiseToChangeTheNumber(newNumber), nil
				},
			},
			"failToChangeTheNumber": &graphql.Field{
				Type: numberHolderType,
				Args: graphql.FieldConfigArgument{
					"newNumber": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					newNumber := 0
					obj, _ := p.Source.(*testRoot)
					newNumber, _ = p.Args["newNumber"].(int)
					return obj.FailToChangeTheNumber(newNumber), nil
				},
			},
			"promiseAndFailToChangeTheNumber": &graphql.Field{
				Type: numberHolderType,
				Args: graphql.FieldConfigArgument{
					"newNumber": &graphql.ArgumentConfig{
						Type: graphql.Int,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					newNumber := 0
					obj, _ := p.Source.(*testRoot)
					newNumber, _ = p.Args["newNumber"].(int)
					return obj.PromiseAndFailToChangeTheNumber(newNumber), nil
				},
			},
		},
	}),
})

func TestMutations_ExecutionOrdering_EvaluatesMutationsSerially(t *testing.T) {
	root := newTestRoot(6)
	doc := `mutation M {
      first: immediatelyChangeTheNumber(newNumber: 1) {
        theNumber
      },
      second: promiseToChangeTheNumber(newNumber: 2) {
        theNumber
      },
      third: immediatelyChangeTheNumber(newNumber: 3) {
        theNumber
      }
      fourth: promiseToChangeTheNumber(newNumber: 4) {
        theNumber
      },
      fifth: immediatelyChangeTheNumber(newNumber: 5) {
        theNumber
      }
    }`

	expected := &graphql.Result{
		Data: map[string]any{
			"first": map[string]any{
				"theNumber": 1,
			},
			"second": map[string]any{
				"theNumber": 2,
			},
			"third": map[string]any{
				"theNumber": 3,
			},
			"fourth": map[string]any{
				"theNumber": 4,
			},
			"fifth": map[string]any{
				"theNumber": 5,
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: mutationsTestSchema,
		AST:    ast,
		Root:   root,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
func TestMutations_EvaluatesMutationsCorrectlyInThePresenceOfAFailedMutation(t *testing.T) {
	root := newTestRoot(6)
	doc := `mutation M {
      first: immediatelyChangeTheNumber(newNumber: 1) {
        theNumber
      },
      second: promiseToChangeTheNumber(newNumber: 2) {
        theNumber
      },
      third: failToChangeTheNumber(newNumber: 3) {
        theNumber
      }
      fourth: promiseToChangeTheNumber(newNumber: 4) {
        theNumber
      },
      fifth: immediatelyChangeTheNumber(newNumber: 5) {
        theNumber
      }
      sixth: promiseAndFailToChangeTheNumber(newNumber: 6) {
        theNumber
      }
    }`

	expected := &graphql.Result{
		Data: map[string]any{
			"first": map[string]any{
				"theNumber": 1,
			},
			"second": map[string]any{
				"theNumber": 2,
			},
			"third": nil,
			"fourth": map[string]any{
				"theNumber": 4,
			},
			"fifth": map[string]any{
				"theNumber": 5,
			},
			"sixth": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Message: `Cannot change the number`,
				Locations: []location.SourceLocation{
					{Line: 8, Column: 7},
				},
			},
			{
				Message: `Cannot change the number`,
				Locations: []location.SourceLocation{
					{Line: 17, Column: 7},
				},
			},
		},
	}
	// parse query
	ast := testutil.TestParse(t, doc)

	// execute
	ep := graphql.ExecuteParams{
		Schema: mutationsTestSchema,
		AST:    ast,
		Root:   root,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(result.Errors) != len(expected.Errors) {
		t.Fatalf("Unexpected errors, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	t.Skipf("Testing equality for slice of errors in results")
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}
