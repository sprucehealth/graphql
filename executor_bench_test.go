package graphql

import (
	"context"
	"testing"

	"github.com/sprucehealth/graphql/language/parser"
)

func TestDefaultResolveFn(t *testing.T) {
	p := ResolveParams{
		Source: &struct {
			A string `json:"a"`
			B string `json:"b"`
			C string `json:"c"`
			D string `json:"d"`
			E string `json:"e"`
			F string `json:"f"`
			G string `json:"g"`
			H string `json:"h"`
		}{
			F: "testing",
		},
		Info: ResolveInfo{
			FieldName: "F",
		},
	}
	v, err := defaultResolveFn(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := v.(string); !ok {
		t.Fatalf("Expected string, got %T", v)
	} else if s != "testing" {
		t.Fatalf("Expected 'testing'")
	}

	p = ResolveParams{
		Source: map[string]any{
			"A": "a",
			"B": "b",
			"C": "c",
			"D": "d",
			"E": "e",
			"F": "testing",
			"G": func() any { return "g" },
			"H": "h",
		},
		Info: ResolveInfo{
			FieldName: "F",
		},
	}
	v, err = defaultResolveFn(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := v.(string); !ok {
		t.Fatalf("Expected string, got %T", v)
	} else if s != "testing" {
		t.Fatalf("Expected 'testing'")
	}

	p.Info.FieldName = "G"
	v, err = defaultResolveFn(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := v.(string); !ok {
		t.Fatalf("Expected string, got %T", v)
	} else if s != "g" {
		t.Fatalf("Expected 'testing'")
	}
}

func BenchmarkDefaultResolveFnStruct(b *testing.B) {
	p := ResolveParams{
		Source: &struct {
			A string `json:"a"`
			B string `json:"b"`
			C string `json:"c"`
			D string `json:"d"`
			E string `json:"e"`
			F string `json:"f"`
			G string `json:"g"`
			H string `json:"h"`
		}{
			F: "testing",
		},
		Info: ResolveInfo{
			FieldName: "F",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := defaultResolveFn(context.Background(), p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDefaultResolveFnMap(b *testing.B) {
	p := ResolveParams{
		Source: map[string]any{
			"A": "a",
			"B": "b",
			"C": "c",
			"D": "d",
			"E": "e",
			"F": "testing",
			"G": "g",
			"H": "h",
		},
		Info: ResolveInfo{
			FieldName: "F",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := defaultResolveFn(context.Background(), p); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkQuery(b *testing.B) {
	type enumValueType string

	const enumValue enumValueType = "foo"

	enumType := NewEnum(EnumConfig{
		Name: "Bar",
		Values: EnumValueConfigMap{
			"foo": &EnumValueConfig{
				Value: enumValue,
			},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query: NewObject(ObjectConfig{
			Name: "Query",
			Fields: Fields{
				"foo": &Field{
					Type: NewNonNull(enumType),
					Resolve: func(ctx context.Context, p ResolveParams) (any, error) {
						return enumValue, nil
					},
				},
				"someID": &Field{
					Type: NewNonNull(ID),
					Resolve: func(ctx context.Context, p ResolveParams) (any, error) {
						return "abc", nil
					},
				},
				"someString": &Field{
					Type: NewNonNull(String),
					Resolve: func(ctx context.Context, p ResolveParams) (any, error) {
						return "bar", nil
					},
				},
				"someInt": &Field{
					Type: NewNonNull(Int),
					Resolve: func(ctx context.Context, p ResolveParams) (any, error) {
						return 123, nil
					},
				},
				"someFloat": &Field{
					Type: NewNonNull(Float),
					Resolve: func(ctx context.Context, p ResolveParams) (any, error) {
						return 1.23, nil
					},
				},
				"someBoolean": &Field{
					Type: NewNonNull(Boolean),
					Resolve: func(ctx context.Context, p ResolveParams) (any, error) {
						return true, nil
					},
				},
			},
		}),
	})
	if err != nil {
		b.Fatalf("Error in schema %s", err)
	}

	astDoc, err := parser.Parse(parser.ParseParams{
		Source: `
			query _ {
				foo
				someID
				someString
				someInt
				someFloat
				someBoolean
			}
		`,
		Options: parser.ParseOptions{NoSource: true},
	})
	if err != nil {
		b.Fatalf("Parse failed: %s", err)
	}

	ep := ExecuteParams{
		Schema: schema,
		AST:    astDoc,
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := Execute(context.Background(), ep)
		if len(result.Errors) > 0 {
			b.Fatalf("wrong result, unexpected errors: %v", result.Errors)
		}
	}
}
