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

func checkList(t *testing.T, testType graphql.Type, testData any, expected *graphql.Result) {
	data := map[string]any{
		"test": testData,
	}

	dataType := graphql.NewObject(graphql.ObjectConfig{
		Name: "DataType",
		Fields: graphql.Fields{
			"test": &graphql.Field{
				Type: testType,
			},
		},
	})
	dataType.AddFieldConfig("nest", &graphql.Field{
		Type: dataType,
		Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
			return data, nil
		},
	})

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: dataType,
	})
	if err != nil {
		t.Fatalf("Error in schema %v", err.Error())
	}

	// parse query
	ast := testutil.TestParse(t, `{ nest { test } }`)

	// execute
	ep := graphql.ExecuteParams{
		Schema: schema,
		AST:    ast,
		Root:   data,
	}
	result := testutil.TestExecute(t, context.Background(), ep)
	if len(expected.Errors) != len(result.Errors) {
		t.Fatalf("wrong result, Diff: %v", testutil.Diff(expected.Errors, result.Errors))
	}
	for i := range expected.Errors {
		expected.Errors[i].Type = gqlerrors.ErrorTypeInternal
	}
	for i := range result.Errors {
		result.Errors[i].OriginalError = nil
		result.Errors[i].StackTrace = ""
	}
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, result))
	}
}

// Describe [T] Array<T>
func TestLists_ListOfNullableObjects_ContainsValues(t *testing.T) {
	ttype := graphql.NewList(graphql.Int)
	data := []any{
		1, 2,
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_ListOfNullableObjects_ContainsNull(t *testing.T) {
	ttype := graphql.NewList(graphql.Int)
	data := []any{
		1, nil, 2,
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, nil, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_ListOfNullableObjects_ReturnsNull(t *testing.T) {
	ttype := graphql.NewList(graphql.Int)
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": nil,
			},
		},
	}
	checkList(t, ttype, nil, expected)
}

// Describe [T] Func()Array<T> // equivalent to Promise<Array<T>>
func TestLists_ListOfNullableFunc_ContainsValues(t *testing.T) {
	ttype := graphql.NewList(graphql.Int)

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return []any{
			1, 2,
		}
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_ListOfNullableFunc_ContainsNull(t *testing.T) {
	ttype := graphql.NewList(graphql.Int)

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return []any{
			1, nil, 2,
		}
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, nil, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_ListOfNullableFunc_ReturnsNull(t *testing.T) {
	ttype := graphql.NewList(graphql.Int)

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return nil
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": nil,
			},
		},
	}
	checkList(t, ttype, data, expected)
}

// Describe [T] Array<Func()<T>> // equivalent to Array<Promise<T>>
func TestLists_ListOfNullableArrayOfFuncContainsValues(t *testing.T) {
	ttype := graphql.NewList(graphql.Int)

	// `data` is a slice of functions that return values
	// Note that its uses the expected signature `func() any {...}`
	data := []any{
		func() any {
			return 1
		},
		func() any {
			return 2
		},
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_ListOfNullableArrayOfFuncContainsNulls(t *testing.T) {
	ttype := graphql.NewList(graphql.Int)

	// `data` is a slice of functions that return values
	// Note that its uses the expected signature `func() any {...}`
	data := []any{
		func() any {
			return 1
		},
		func() any {
			return nil
		},
		func() any {
			return 2
		},
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, nil, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}

// Describe [T]! Array<T>
func TestLists_NonNullListOfNullableObjectsContainsValues(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.Int))
	data := []any{
		1, 2,
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NonNullListOfNullableObjectsContainsNull(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.Int))
	data := []any{
		1, nil, 2,
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, nil, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NonNullListOfNullableObjectsReturnsNull(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.Int))
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Message: "Cannot return null for non-nullable field DataType.test.",
				Locations: []location.SourceLocation{
					{
						Line:   1,
						Column: 10,
					},
				},
			},
		},
	}
	checkList(t, ttype, nil, expected)
}

// Describe [T]! Func()Array<T> // equivalent to Promise<Array<T>>
func TestLists_NonNullListOfNullableFunc_ContainsValues(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.Int))

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return []any{
			1, 2,
		}
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NonNullListOfNullableFunc_ContainsNull(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.Int))

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return []any{
			1, nil, 2,
		}
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, nil, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NonNullListOfNullableFunc_ReturnsNull(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.Int))

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return nil
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Message: "Cannot return null for non-nullable field DataType.test.",
				Locations: []location.SourceLocation{
					{
						Line:   1,
						Column: 10,
					},
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}

// Describe [T]! Array<Func()<T>> // equivalent to Array<Promise<T>>
func TestLists_NonNullListOfNullableArrayOfFunc_ContainsValues(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.Int))

	// `data` is a slice of functions that return values
	// Note that its uses the expected signature `func() any {...}`
	data := []any{
		func() any {
			return 1
		},
		func() any {
			return 2
		},
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NonNullListOfNullableArrayOfFunc_ContainsNulls(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.Int))

	// `data` is a slice of functions that return values
	// Note that its uses the expected signature `func() any {...}`
	data := []any{
		func() any {
			return 1
		},
		func() any {
			return nil
		},
		func() any {
			return 2
		},
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, nil, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}

// Describe [T!] Array<T>
func TestLists_NullableListOfNonNullObjects_ContainsValues(t *testing.T) {
	ttype := graphql.NewList(graphql.NewNonNull(graphql.Int))
	data := []any{
		1, 2,
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NullableListOfNonNullObjects_ContainsNull(t *testing.T) {
	ttype := graphql.NewList(graphql.NewNonNull(graphql.Int))
	data := []any{
		1, nil, 2,
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": nil,
			},
		},
		Errors: []gqlerrors.FormattedError{
			{
				Message: "Cannot return null for non-nullable field DataType.test.",
				Locations: []location.SourceLocation{
					{
						Line:   1,
						Column: 10,
					},
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NullableListOfNonNullObjects_ReturnsNull(t *testing.T) {
	ttype := graphql.NewList(graphql.NewNonNull(graphql.Int))

	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": nil,
			},
		},
	}
	checkList(t, ttype, nil, expected)
}

// Describe [T!] Func()Array<T> // equivalent to Promise<Array<T>>
func TestLists_NullableListOfNonNullFunc_ContainsValues(t *testing.T) {
	ttype := graphql.NewList(graphql.NewNonNull(graphql.Int))

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return []any{
			1, 2,
		}
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NullableListOfNonNullFunc_ContainsNull(t *testing.T) {
	ttype := graphql.NewList(graphql.NewNonNull(graphql.Int))

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return []any{
			1, nil, 2,
		}
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": nil,
			},
		},
		Errors: []gqlerrors.FormattedError{
			{
				Message: "Cannot return null for non-nullable field DataType.test.",
				Locations: []location.SourceLocation{
					{
						Line:   1,
						Column: 10,
					},
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NullableListOfNonNullFunc_ReturnsNull(t *testing.T) {
	ttype := graphql.NewList(graphql.NewNonNull(graphql.Int))

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return nil
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": nil,
			},
		},
	}
	checkList(t, ttype, data, expected)
}

// Describe [T!] Array<Func()<T>> // equivalent to Array<Promise<T>>
func TestLists_NullableListOfNonNullArrayOfFunc_ContainsValues(t *testing.T) {
	ttype := graphql.NewList(graphql.NewNonNull(graphql.Int))

	// `data` is a slice of functions that return values
	// Note that its uses the expected signature `func() any {...}`
	data := []any{
		func() any {
			return 1
		},
		func() any {
			return 2
		},
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NullableListOfNonNullArrayOfFunc_ContainsNulls(t *testing.T) {
	ttype := graphql.NewList(graphql.NewNonNull(graphql.Int))

	// `data` is a slice of functions that return values
	// Note that its uses the expected signature `func() any {...}`
	data := []any{
		func() any {
			return 1
		},
		func() any {
			return nil
		},
		func() any {
			return 2
		},
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, nil, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}

// Describe [T!]! Array<T>
func TestLists_NonNullListOfNonNullObjects_ContainsValues(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))
	data := []any{
		1, 2,
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NonNullListOfNonNullObjects_ContainsNull(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))
	data := []any{
		1, nil, 2,
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Message: "Cannot return null for non-nullable field DataType.test.",
				Locations: []location.SourceLocation{
					{
						Line:   1,
						Column: 10,
					},
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NonNullListOfNonNullObjects_ReturnsNull(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))

	expected := &graphql.Result{
		Data: map[string]any{
			"nest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Message: "Cannot return null for non-nullable field DataType.test.",
				Locations: []location.SourceLocation{
					{
						Line:   1,
						Column: 10,
					},
				},
			},
		},
	}
	checkList(t, ttype, nil, expected)
}

// Describe [T!]! Func()Array<T> // equivalent to Promise<Array<T>>
func TestLists_NonNullListOfNonNullFunc_ContainsValues(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return []any{
			1, 2,
		}
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NonNullListOfNonNullFunc_ContainsNull(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return []any{
			1, nil, 2,
		}
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Message: "Cannot return null for non-nullable field DataType.test.",
				Locations: []location.SourceLocation{
					{
						Line:   1,
						Column: 10,
					},
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NonNullListOfNonNullFunc_ReturnsNull(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))

	// `data` is a function that return values
	// Note that its uses the expected signature `func() any {...}`
	data := func() any {
		return nil
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": nil,
		},
		Errors: []gqlerrors.FormattedError{
			{
				Message: "Cannot return null for non-nullable field DataType.test.",
				Locations: []location.SourceLocation{
					{
						Line:   1,
						Column: 10,
					},
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}

// Describe [T!]! Array<Func()<T>> // equivalent to Array<Promise<T>>
func TestLists_NonNullListOfNonNullArrayOfFunc_ContainsValues(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))

	// `data` is a slice of functions that return values
	// Note that its uses the expected signature `func() any {...}`
	data := []any{
		func() any {
			return 1
		},
		func() any {
			return 2
		},
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
func TestLists_NonNullListOfNonNullArrayOfFunc_ContainsNulls(t *testing.T) {
	ttype := graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))

	// `data` is a slice of functions that return values
	// Note that its uses the expected signature `func() any {...}`
	data := []any{
		func() any {
			return 1
		},
		func() any {
			return nil
		},
		func() any {
			return 2
		},
	}
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": []any{
					1, nil, 2,
				},
			},
		},
	}
	checkList(t, ttype, data, expected)
}

func TestLists_UserErrorExpectIterableButDidNotGetOne(t *testing.T) {
	ttype := graphql.NewList(graphql.Int)
	data := "Not an iterable"
	expected := &graphql.Result{
		Data: map[string]any{
			"nest": map[string]any{
				"test": nil,
			},
		},
		Errors: []gqlerrors.FormattedError{
			{
				Type:      "INTERNAL",
				Message:   "User Error: expected iterable, but did not find one for field DataType.test.",
				Locations: []location.SourceLocation{},
			},
		},
	}
	checkList(t, ttype, data, expected)
}
