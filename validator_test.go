package graphql_test

import (
	"reflect"
	"testing"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/location"
	"github.com/sprucehealth/graphql/language/parser"
	"github.com/sprucehealth/graphql/language/source"
	"github.com/sprucehealth/graphql/testutil"
)

func expectValid(t *testing.T, schema *graphql.Schema, queryString string) {
	source := source.New("GraphQL Request", queryString)
	AST, err := parser.Parse(parser.ParseParams{Source: source})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	validationResult := graphql.ValidateDocument(schema, AST, nil)

	if !validationResult.IsValid || len(validationResult.Errors) > 0 {
		t.Fatalf("Unexpected error: %v", validationResult.Errors)
	}

}

func TestValidator_SupportsFullValidation_ValidatesQueries(t *testing.T) {
	expectValid(t, testutil.TestSchema, `
      query {
        catOrDog {
          ... on Cat {
            furColor
          }
          ... on Dog {
            isHousetrained
          }
        }
      }
    `)
}

func TestConcurrentValidateDocument(t *testing.T) {
	validate := func() {
		query := `
		query HeroNameAndFriendsQuery {
			hero {
				id
				name
				friends {
					name
				}
			}
		}
	`
		ast, err := parser.Parse(parser.ParseParams{Source: source.New("", query)})
		if err != nil {
			t.Fatal(err)
		}
		r := graphql.ValidateDocument(&testutil.StarWarsSchema, ast, nil)
		if !r.IsValid {
			t.Fatal("Not valid")
		}
	}
	go validate()
	validate()
}

func BenchmarkValidateDocument(b *testing.B) {
	query := `
		query HeroNameAndFriendsQuery {
			hero {
				id
				name
				friends {
					name
				}
			}
		}
	`
	ast, err := parser.Parse(parser.ParseParams{Source: source.New("", query)})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := graphql.ValidateDocument(&testutil.StarWarsSchema, ast, nil)
		if !r.IsValid {
			b.Fatal("Not valid")
		}
	}
}

// BenchmarkValidateDocumentRepeatedField stresses OverlappingFieldsCanBeMergedRule
func BenchmarkValidateDocumentRepeatedField(b *testing.B) {
	query := `
		query HeroNameAndFriendsQuery {
			hero {
				id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id
				id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id id
			}
		}
	`
	ast, err := parser.Parse(parser.ParseParams{Source: source.New("", query)})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := graphql.ValidateDocument(&testutil.StarWarsSchema, ast, nil)
		if !r.IsValid {
			b.Fatal("Not valid")
		}
	}
}

// NOTE: experimental
func TestValidator_SupportsFullValidation_ValidatesUsingACustomTypeInfo(t *testing.T) {

	// This TypeInfo will never return a valid field.
	typeInfo := graphql.NewTypeInfo(&graphql.TypeInfoConfig{
		Schema: testutil.TestSchema,
		FieldDefFn: func(schema *graphql.Schema, parentType graphql.Type, fieldAST *ast.Field) *graphql.FieldDefinition {
			return nil
		},
	})

	ast := testutil.TestParse(t, `
	  query {
        catOrDog {
          ... on Cat {
            furColor
          }
          ... on Dog {
            isHousetrained
          }
        }
      }
	`)

	errors := graphql.VisitUsingRules(testutil.TestSchema, typeInfo, ast, graphql.SpecifiedRules)

	expectedErrors := []gqlerrors.FormattedError{
		{
			Type:    gqlerrors.ErrorTypeBadQuery,
			Message: `Cannot query field "catOrDog" on type "QueryRoot". Did you mean "catOrDog"?`,
			Locations: []location.SourceLocation{
				{Line: 3, Column: 9},
			},
		},
		{
			Type:    gqlerrors.ErrorTypeBadQuery,
			Message: `Cannot query field "furColor" on type "Cat". Did you mean "furColor"?`,
			Locations: []location.SourceLocation{
				{Line: 5, Column: 13},
			},
		},
		{
			Type:    gqlerrors.ErrorTypeBadQuery,
			Message: `Cannot query field "isHousetrained" on type "Dog". Did you mean "isHousetrained"?`,
			Locations: []location.SourceLocation{
				{Line: 8, Column: 13},
			},
		},
	}
	if !reflect.DeepEqual(expectedErrors, errors) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expectedErrors, errors))
	}
}
