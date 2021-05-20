package parser

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kr/pretty"
	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/location"
	"github.com/sprucehealth/graphql/language/printer"
	"github.com/sprucehealth/graphql/language/source"
)

func TestBadToken(t *testing.T) {
	_, err := Parse(ParseParams{
		Source: source.New("GraphQL", "query _ {\n  me {\n    id`\n  }\n}"),
	})
	if err == nil {
		t.Fatal("expected a parse error")
	}
}

func TestAcceptsOptionToNotIncludeSource(t *testing.T) {
	opts := ParseOptions{
		NoSource: true,
	}
	params := ParseParams{
		Source:  "{ field }",
		Options: opts,
	}
	document, err := Parse(params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	oDef := ast.OperationDefinition{
		Loc: ast.Location{
			Start: 0, End: 9,
		},
		Operation: "query",
		SelectionSet: &ast.SelectionSet{
			Loc: ast.Location{
				Start: 0, End: 9,
			},
			Selections: []ast.Selection{
				&ast.Field{
					Loc: ast.Location{
						Start: 2, End: 7,
					},
					Name: &ast.Name{
						Loc: ast.Location{
							Start: 2, End: 7,
						},
						Value: "field",
					},
				},
			},
		},
	}
	expectedDocument := &ast.Document{
		Loc: ast.Location{
			Start: 0, End: 9,
		},
		Definitions: []ast.Node{&oDef},
	}
	if !reflect.DeepEqual(document, expectedDocument) {
		t.Fatalf("unexpected document, expected: %v, got: %v", expectedDocument, document)
	}
}

func TestParseProvidesUsefulErrors(t *testing.T) {
	opts := ParseOptions{
		NoSource: true,
	}
	params := ParseParams{
		Source:  "{",
		Options: opts,
	}
	_, err := Parse(params)

	expectedError := &gqlerrors.Error{
		Message: `Syntax Error GraphQL (1:2) Expected Name, found EOF

1: {
    ^
`,
		Positions: []int{1},
		Locations: []location.SourceLocation{{Line: 1, Column: 2}},
	}
	checkError(t, err, expectedError)

	testErrorMessagesTable := []errorMessageTest{
		{
			`{ ...MissingOn }
fragment MissingOn Type
`,
			`Syntax Error GraphQL (2:20) Expected "on", found Name "Type"`,
			false,
		},
		{
			`{ field: {} }`,
			`Syntax Error GraphQL (1:10) Expected Name, found {`,
			false,
		},
		{
			`notanoperation Foo { field }`,
			`Syntax Error GraphQL (1:1) Unexpected Name "notanoperation"`,
			false,
		},
		{
			"...",
			`Syntax Error GraphQL (1:1) Unexpected ...`,
			false,
		},
	}
	for _, test := range testErrorMessagesTable {
		if test.skipped {
			t.Skipf("Skipped test: %v", test.source)
		}
		_, err := Parse(ParseParams{Source: test.source})
		checkErrorMessage(t, err, test.expectedMessage)
	}

}

func TestParseProvidesUsefulErrorsWhenUsingSource(t *testing.T) {
	test := errorMessageTest{
		source.New("MyQuery.graphql", "query"),
		`Syntax Error MyQuery.graphql (1:6) Expected {, found EOF`,
		false,
	}
	testErrorMessage(t, test)
}

func TestParsesTypeSpecDirective(t *testing.T) {
	source := `
		type ExampleType {
			oldField1: String @deprecated
			oldField2: String @deprecated(reason: "Use newField")
		}
	`
	// should not return error
	astDoc, err := Parse(ParseParams{Source: source})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedASTDoc := &ast.Document{
		Loc: ast.Location{
			Start: 3,
			End:   117,
		},
		Definitions: []ast.Node{
			&ast.ObjectDefinition{
				Loc: ast.Location{
					Start: 3,
					End:   115,
				},
				Name: &ast.Name{
					Loc: ast.Location{
						Start: 8,
						End:   19,
					},
					Value: "ExampleType",
				},
				Fields: []*ast.FieldDefinition{
					{
						Loc: ast.Location{
							Start: 25,
							End:   54,
						},
						Name: &ast.Name{
							Loc: ast.Location{
								Start: 25,
								End:   34,
							},
							Value: "oldField1",
						},
						Arguments: nil,
						Type: &ast.Named{
							Loc: ast.Location{
								Start: 36,
								End:   42,
							},
							Name: &ast.Name{
								Loc: ast.Location{
									Start: 36,
									End:   42,
								},
								Value: "String",
							},
						},
						Directives: []*ast.Directive{
							{
								Loc: ast.Location{
									Start: 43,
									End:   54,
								},
								Name: &ast.Name{
									Loc: ast.Location{
										Start: 44,
										End:   54,
									},
									Value: "deprecated",
								},
							},
						},
					},
					{
						Loc: ast.Location{
							Start: 58,
							End:   111,
						},
						Name: &ast.Name{
							Loc: ast.Location{
								Start: 58,
								End:   67,
							},
							Value: "oldField2",
						},
						Type: &ast.Named{
							Loc: ast.Location{
								Start: 69,
								End:   75,
							},
							Name: &ast.Name{
								Loc: ast.Location{
									Start: 69,
									End:   75,
								},
								Value: "String",
							},
						},
						Directives: []*ast.Directive{
							{
								Loc: ast.Location{
									Start: 76,
									End:   111,
								},
								Name: &ast.Name{
									Loc: ast.Location{
										Start: 77,
										End:   87,
									},
									Value: "deprecated",
								},
								Arguments: []*ast.Argument{
									{
										Loc: ast.Location{
											Start: 88,
											End:   110,
										},
										Name: &ast.Name{
											Loc: ast.Location{
												Start: 88,
												End:   94,
											},
											Value: "reason",
										},
										Value: &ast.StringValue{
											Loc: ast.Location{
												Start: 96,
												End:   110,
											},
											Value: "Use newField",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	astDocQuery := printer.Print(astDoc)
	expectedASTDocQuery := printer.Print(expectedASTDoc)

	if !reflect.DeepEqual(astDocQuery, expectedASTDocQuery) {
		t.Fatalf("unexpected document: %s\n%s", pretty.Sprint(astDoc), pretty.Diff(expectedASTDoc, astDoc))
	}
}

func TestParsesVariableInlineValues(t *testing.T) {
	source := `{ field(complex: { a: { b: [ $var ] } }) }`
	// should not return error
	_, err := Parse(ParseParams{Source: source})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsesConstantDefaultValues(t *testing.T) {
	test := errorMessageTest{
		`query Foo($x: Complex = { a: { b: [ $var ] } }) { field }`,
		`Syntax Error GraphQL (1:37) Unexpected $`,
		false,
	}
	testErrorMessage(t, test)
}

func TestDoesNotAcceptFragmentsNameOn(t *testing.T) {
	test := errorMessageTest{
		`fragment on on on { on }`,
		`Syntax Error GraphQL (1:10) Unexpected Name "on"`,
		false,
	}
	testErrorMessage(t, test)
}

func TestDoesNotAcceptFragmentsSpreadOfOn(t *testing.T) {
	test := errorMessageTest{
		`{ ...on }'`,
		`Syntax Error GraphQL (1:9) Expected Name, found }`,
		false,
	}
	testErrorMessage(t, test)
}

func TestDoesNotAllowNullAsValue(t *testing.T) {
	test := errorMessageTest{
		`{ fieldWithNullableStringInput(input: null) }'`,
		`Syntax Error GraphQL (1:39) Unexpected Name "null"`,
		false,
	}
	testErrorMessage(t, test)
}

func TestParsesMultiByteCharacters_Unicode(t *testing.T) {

	doc := `
        # This comment has a \u0A0A multi-byte character.
        { field(arg: "Has a \u0A0A multi-byte character.") }
	`
	astDoc := parse(t, doc)

	expectedASTDoc := &ast.Document{
		Loc: ast.Location{Start: 67, End: 121},
		Definitions: []ast.Node{
			&ast.OperationDefinition{
				Loc:       ast.Location{Start: 67, End: 119},
				Operation: "query",
				SelectionSet: &ast.SelectionSet{
					Loc: ast.Location{Start: 67, End: 119},
					Selections: []ast.Selection{
						&ast.Field{
							Loc: ast.Location{Start: 67, End: 117},
							Name: &ast.Name{
								Loc:   ast.Location{Start: 69, End: 74},
								Value: "field",
							},
							Arguments: []*ast.Argument{
								{
									Loc: ast.Location{Start: 75, End: 116},
									Name: &ast.Name{

										Loc: ast.Location{
											Start: 75,
											End:   78,
										},
										Value: "arg",
									},
									Value: &ast.StringValue{
										Loc:   ast.Location{Start: 80, End: 116},
										Value: "Has a \u0A0A multi-byte character.",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	astDocQuery := printer.Print(astDoc)
	expectedASTDocQuery := printer.Print(expectedASTDoc)

	if !reflect.DeepEqual(astDocQuery, expectedASTDocQuery) {
		t.Fatalf("unexpected document, expected: %v, got: %v", astDocQuery, expectedASTDocQuery)
	}
}

func TestParsesMultiByteCharacters_UnicodeText(t *testing.T) {

	doc := `
        # This comment has a фы世界 multi-byte character.
        { field(arg: "Has a фы世界 multi-byte character.") }
	`
	astDoc := parse(t, doc)

	expectedASTDoc := &ast.Document{
		Loc: ast.Location{
			Start: 67,
			End:   121,
		},
		Definitions: []ast.Node{
			&ast.OperationDefinition{
				Loc: ast.Location{
					Start: 67,
					End:   119,
				},
				Operation: "query",
				SelectionSet: &ast.SelectionSet{
					Loc: ast.Location{
						Start: 67,
						End:   119,
					},
					Selections: []ast.Selection{
						&ast.Field{
							Loc: ast.Location{
								Start: 67,
								End:   117,
							},
							Name: &ast.Name{
								Loc: ast.Location{
									Start: 69,
									End:   74,
								},
								Value: "field",
							},
							Arguments: []*ast.Argument{
								{
									Loc: ast.Location{
										Start: 75,
										End:   116,
									},
									Name: &ast.Name{
										Loc: ast.Location{
											Start: 75,
											End:   78,
										},
										Value: "arg",
									},
									Value: &ast.StringValue{
										Loc: ast.Location{
											Start: 80,
											End:   116,
										},
										Value: "Has a фы世界 multi-byte character.",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	astDocQuery := printer.Print(astDoc)
	expectedASTDocQuery := printer.Print(expectedASTDoc)

	if !reflect.DeepEqual(astDocQuery, expectedASTDocQuery) {
		t.Fatalf("unexpected document, expected: %v, got: %v", astDocQuery, expectedASTDocQuery)
	}
}

func TestParsesKitchenSink(t *testing.T) {
	b, err := os.ReadFile("../../kitchen-sink.graphql")
	if err != nil {
		t.Fatalf("unable to load kitchen-sink.graphql")
	}
	source := string(b)
	_, err = Parse(ParseParams{Source: source})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAllowsNonKeywordsAnywhereNameIsAllowed(t *testing.T) {
	nonKeywords := []string{
		"on",
		"fragment",
		"query",
		"mutation",
		"subscription",
		"true",
		"false",
	}
	for _, keyword := range nonKeywords {
		fragmentName := keyword
		// You can't define or reference a fragment named `on`.
		if keyword == "on" {
			fragmentName = "a"
		}
		source := fmt.Sprintf(`query %v {
			... %v
			... on %v { field }
		}
		fragment %v on Type {
		%v(%v: $%v) @%v(%v: $%v)
		}
		`, keyword, fragmentName, keyword, fragmentName, keyword, keyword, keyword, keyword, keyword, keyword)
		_, err := Parse(ParseParams{Source: source})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestParsesExperimentalSubscriptionFeature(t *testing.T) {
	source := `
      subscription Foo {
        subscriptionField
      }
    `
	_, err := Parse(ParseParams{Source: source})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsesAnonymousMutationOperations(t *testing.T) {
	source := `
		mutation {
			mutationField
		}
	`
	_, err := Parse(ParseParams{Source: source})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsesAnonymousSubscriptionOperations(t *testing.T) {
	source := `
      subscription {
        subscriptionField
      }
    `
	_, err := Parse(ParseParams{Source: source})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsesNamedMutationOperations(t *testing.T) {
	source := `
      mutation Foo {
        mutationField
      }
    `
	_, err := Parse(ParseParams{Source: source})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsesNamedSubscriptionOperations(t *testing.T) {
	source := `
      subscription Foo {
        subscriptionField
      }
    `
	_, err := Parse(ParseParams{Source: source})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComments(t *testing.T) {
	source := `
		# Unconnected comment
		# part of the same group

		# Type doc comment
		# two lines
		type Foo {
			# Field comment
			bar: String! # Line comment
		}

		# enum doc
		enum SomeType {
			RED # yep
			# color 2
			BLUE
		}

		# Implemented by types: StringListSettingValue BooleanSettingValue TextSettingValue SelectableSettingValue
		interface SettingValue {
			# key doc
			key: Boolean # key line
		}

		# input doc
		input SomeInput {
			# bar doc
			bar: String # bar comment
		}

		type Query {
			someQuery(
				foo: String,
				blah: ID # blah comment
			): String
		}
	`
	document, err := Parse(ParseParams{Source: source, Options: ParseOptions{NoSource: true, KeepComments: true}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedDocument := &ast.Document{
		Loc: ast.Location{Start: 90, End: 580},
		Definitions: []ast.Node{
			&ast.ObjectDefinition{
				Loc:  ast.Location{Start: 90, End: 154},
				Name: &ast.Name{Loc: ast.Location{Start: 95, End: 98}, Value: "Foo"},
				Doc: &ast.CommentGroup{
					Loc: ast.Location{Start: 55, End: 87},
					List: []*ast.Comment{
						{Loc: ast.Location{Start: 55, End: 73}, Text: "# Type doc comment"},
						{Loc: ast.Location{Start: 76, End: 87}, Text: "# two lines"},
					},
				},
				Fields: []*ast.FieldDefinition{
					{
						Loc:  ast.Location{Start: 123, End: 135},
						Name: &ast.Name{Loc: ast.Location{Start: 123, End: 126}, Value: "bar"},
						Type: &ast.NonNull{
							Loc: ast.Location{Start: 128, End: 135},
							Type: &ast.Named{
								Loc:  ast.Location{Start: 128, End: 134},
								Name: &ast.Name{Loc: ast.Location{Start: 128, End: 134}, Value: "String"},
							},
						},
						Doc: &ast.CommentGroup{
							Loc: ast.Location{Start: 104, End: 119},
							List: []*ast.Comment{
								{Loc: ast.Location{Start: 104, End: 119}, Text: "# Field comment"},
							},
						},
						Comment: &ast.CommentGroup{
							Loc: ast.Location{Start: 136, End: 150},
							List: []*ast.Comment{
								{Loc: ast.Location{Start: 136, End: 150}, Text: "# Line comment"},
							},
						},
					},
				},
			},
			&ast.EnumDefinition{
				Loc:  ast.Location{Start: 171, End: 224},
				Name: &ast.Name{Loc: ast.Location{Start: 176, End: 184}, Value: "SomeType"},
				Values: []*ast.EnumValueDefinition{
					{
						Loc:  ast.Location{Start: 190, End: 193},
						Name: &ast.Name{Loc: ast.Location{Start: 190, End: 193}, Value: "RED"},
						Comment: &ast.CommentGroup{
							Loc:  ast.Location{Start: 194, End: 199},
							List: []*ast.Comment{{Loc: ast.Location{Start: 194, End: 199}, Text: "# yep"}},
						},
					},
					{
						Loc:  ast.Location{Start: 216, End: 220},
						Name: &ast.Name{Loc: ast.Location{Start: 216, End: 220}, Value: "BLUE"},
						Doc: &ast.CommentGroup{
							Loc:  ast.Location{Start: 203, End: 212},
							List: []*ast.Comment{{Loc: ast.Location{Start: 203, End: 212}, Text: "# color 2"}},
						},
						Comment: (*ast.CommentGroup)(nil),
					},
				},
				Doc: &ast.CommentGroup{
					Loc:  ast.Location{Start: 158, End: 168},
					List: []*ast.Comment{{Loc: ast.Location{Start: 158, End: 168}, Text: "# enum doc"}},
				},
			},
			&ast.InterfaceDefinition{
				Loc:  ast.Location{Start: 337, End: 405},
				Name: &ast.Name{Loc: ast.Location{Start: 347, End: 359}, Value: "SettingValue"},
				Fields: []*ast.FieldDefinition{
					{
						Loc:  ast.Location{Start: 378, End: 390},
						Name: &ast.Name{Loc: ast.Location{Start: 378, End: 381}, Value: "key"},
						Type: &ast.Named{
							Loc:  ast.Location{Start: 383, End: 390},
							Name: &ast.Name{Loc: ast.Location{Start: 383, End: 390}, Value: "Boolean"},
						},
						Doc: &ast.CommentGroup{
							Loc: ast.Location{Start: 365, End: 374},
							List: []*ast.Comment{
								{Loc: ast.Location{Start: 365, End: 374}, Text: "# key doc"},
							},
						},
						Comment: &ast.CommentGroup{
							Loc: ast.Location{Start: 391, End: 401},
							List: []*ast.Comment{
								{Loc: ast.Location{Start: 391, End: 401}, Text: "# key line"},
							},
						},
					},
				},
				Doc: &ast.CommentGroup{
					Loc: ast.Location{Start: 228, End: 334},
					List: []*ast.Comment{
						{Loc: ast.Location{Start: 228, End: 334}, Text: "# Implemented by types: StringListSettingValue BooleanSettingValue TextSettingValue SelectableSettingValue"},
					},
				},
			},
			&ast.InputObjectDefinition{
				Loc:  ast.Location{Start: 423, End: 486},
				Name: &ast.Name{Loc: ast.Location{Start: 429, End: 438}, Value: "SomeInput"},
				Fields: []*ast.InputValueDefinition{
					{
						Loc:  ast.Location{Start: 457, End: 468},
						Name: &ast.Name{Loc: ast.Location{Start: 457, End: 460}, Value: "bar"},
						Type: &ast.Named{
							Loc:  ast.Location{Start: 462, End: 468},
							Name: &ast.Name{Loc: ast.Location{Start: 462, End: 468}, Value: "String"},
						},
						DefaultValue: nil,
						Doc: &ast.CommentGroup{
							Loc:  ast.Location{Start: 444, End: 453},
							List: []*ast.Comment{{Loc: ast.Location{Start: 444, End: 453}, Text: "# bar doc"}},
						},
						Comment: &ast.CommentGroup{
							Loc:  ast.Location{Start: 469, End: 482},
							List: []*ast.Comment{{Loc: ast.Location{Start: 469, End: 482}, Text: "# bar comment"}},
						},
					},
				},
				Doc: &ast.CommentGroup{
					Loc:  ast.Location{Start: 409, End: 420},
					List: []*ast.Comment{{Loc: ast.Location{Start: 409, End: 420}, Text: "# input doc"}},
				},
			},
			&ast.ObjectDefinition{
				Loc:  ast.Location{Start: 490, End: 578},
				Name: &ast.Name{Loc: ast.Location{Start: 495, End: 500}, Value: "Query"},
				Fields: []*ast.FieldDefinition{
					{
						Loc:  ast.Location{Start: 506, End: 574},
						Name: &ast.Name{Loc: ast.Location{Start: 506, End: 515}, Value: "someQuery"},
						Arguments: []*ast.InputValueDefinition{
							{
								Loc:  ast.Location{Start: 521, End: 532},
								Name: &ast.Name{Loc: ast.Location{Start: 521, End: 524}, Value: "foo"},
								Type: &ast.Named{
									Loc:  ast.Location{Start: 526, End: 532},
									Name: &ast.Name{Loc: ast.Location{Start: 526, End: 532}, Value: "String"},
								},
							},
							{
								Loc:  ast.Location{Start: 538, End: 546},
								Name: &ast.Name{Loc: ast.Location{Start: 538, End: 542}, Value: "blah"},
								Type: &ast.Named{
									Loc:  ast.Location{Start: 544, End: 546},
									Name: &ast.Name{Loc: ast.Location{Start: 544, End: 546}, Value: "ID"},
								},
								Comment: &ast.CommentGroup{
									Loc:  ast.Location{Start: 547, End: 561},
									List: []*ast.Comment{{Loc: ast.Location{Start: 547, End: 561}, Text: "# blah comment"}},
								},
							},
						},
						Type: &ast.Named{
							Loc:  ast.Location{Start: 568, End: 574},
							Name: &ast.Name{Loc: ast.Location{Start: 568, End: 574}, Value: "String"},
						},
					},
				},
			},
		},
		Comments: []*ast.CommentGroup{
			{
				Loc: ast.Location{Start: 3, End: 51},
				List: []*ast.Comment{
					{Loc: ast.Location{Start: 3, End: 24}, Text: "# Unconnected comment"},
					{Loc: ast.Location{Start: 27, End: 51}, Text: "# part of the same group"},
				},
			},
			{
				Loc: ast.Location{Start: 55, End: 87},
				List: []*ast.Comment{
					{Loc: ast.Location{Start: 55, End: 73}, Text: "# Type doc comment"},
					{Loc: ast.Location{Start: 76, End: 87}, Text: "# two lines"},
				},
			},
			{Loc: ast.Location{Start: 104, End: 119}, List: []*ast.Comment{{Loc: ast.Location{Start: 104, End: 119}, Text: "# Field comment"}}},
			{Loc: ast.Location{Start: 136, End: 150}, List: []*ast.Comment{{Loc: ast.Location{Start: 136, End: 150}, Text: "# Line comment"}}},
			{Loc: ast.Location{Start: 158, End: 168}, List: []*ast.Comment{{Loc: ast.Location{Start: 158, End: 168}, Text: "# enum doc"}}},
			{Loc: ast.Location{Start: 194, End: 199}, List: []*ast.Comment{{Loc: ast.Location{Start: 194, End: 199}, Text: "# yep"}}},
			{Loc: ast.Location{Start: 203, End: 212}, List: []*ast.Comment{{Loc: ast.Location{Start: 203, End: 212}, Text: "# color 2"}}},
			{
				Loc: ast.Location{Start: 228, End: 334},
				List: []*ast.Comment{
					{Loc: ast.Location{Start: 228, End: 334}, Text: "# Implemented by types: StringListSettingValue BooleanSettingValue TextSettingValue SelectableSettingValue"},
				},
			},
			{Loc: ast.Location{Start: 365, End: 374}, List: []*ast.Comment{{Loc: ast.Location{Start: 365, End: 374}, Text: "# key doc"}}},
			{Loc: ast.Location{Start: 391, End: 401}, List: []*ast.Comment{{Loc: ast.Location{Start: 391, End: 401}, Text: "# key line"}}},
			{Loc: ast.Location{Start: 409, End: 420}, List: []*ast.Comment{{Loc: ast.Location{Start: 409, End: 420}, Text: "# input doc"}}},
			{Loc: ast.Location{Start: 444, End: 453}, List: []*ast.Comment{{Loc: ast.Location{Start: 444, End: 453}, Text: "# bar doc"}}},
			{Loc: ast.Location{Start: 469, End: 482}, List: []*ast.Comment{{Loc: ast.Location{Start: 469, End: 482}, Text: "# bar comment"}}},
			{Loc: ast.Location{Start: 547, End: 561}, List: []*ast.Comment{{Loc: ast.Location{Start: 547, End: 561}, Text: "# blah comment"}}},
		},
	}
	if !reflect.DeepEqual(document, expectedDocument) {
		t.Fatalf("document doesn't match:\n%s\n\ndifferences:\n\n%s", pretty.Sprint(document), pretty.Diff(expectedDocument, document))
	}
}

func TestImplementsInterface(t *testing.T) {
	source := `
		interface A {}
		interface B {}
		type Foo implements A, B {}
		type Bar implements A & B {}
	`
	document, err := Parse(ParseParams{Source: source, Options: ParseOptions{NoSource: true, KeepComments: true}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedDocument := &ast.Document{
		Loc: ast.Location{Start: 3, End: 97},
		Definitions: []ast.Node{
			&ast.InterfaceDefinition{
				Loc:    ast.Location{Start: 3, End: 17},
				Name:   &ast.Name{Loc: ast.Location{Start: 13, End: 14}, Value: "A"},
				Fields: []*ast.FieldDefinition{},
			},
			&ast.InterfaceDefinition{
				Loc:    ast.Location{Start: 20, End: 34},
				Name:   &ast.Name{Loc: ast.Location{Start: 30, End: 31}, Value: "B"},
				Fields: []*ast.FieldDefinition{},
			},
			&ast.ObjectDefinition{
				Loc:    ast.Location{Start: 37, End: 64},
				Name:   &ast.Name{Loc: ast.Location{Start: 42, End: 45}, Value: "Foo"},
				Fields: []*ast.FieldDefinition{},
				Interfaces: []*ast.Named{
					{
						Loc:  ast.Location{Start: 57, End: 58},
						Name: &ast.Name{Loc: ast.Location{Start: 57, End: 58}, Value: "A"},
					},
					{
						Loc:  ast.Location{Start: 60, End: 61},
						Name: &ast.Name{Loc: ast.Location{Start: 60, End: 61}, Value: "B"},
					},
				},
			},
			&ast.ObjectDefinition{
				Loc:    ast.Location{Start: 67, End: 95},
				Name:   &ast.Name{Loc: ast.Location{Start: 72, End: 75}, Value: "Bar"},
				Fields: []*ast.FieldDefinition{},
				Interfaces: []*ast.Named{
					{
						Loc:  ast.Location{Start: 87, End: 88},
						Name: &ast.Name{Loc: ast.Location{Start: 87, End: 88}, Value: "A"},
					},
					{
						Loc:  ast.Location{Start: 91, End: 92},
						Name: &ast.Name{Loc: ast.Location{Start: 91, End: 92}, Value: "B"},
					},
				},
			},
		},
	}
	if !reflect.DeepEqual(document, expectedDocument) {
		t.Fatalf("document doesn't match:\n%s\n\ndifferences:\n\n%s", pretty.Sprint(document), pretty.Diff(expectedDocument, document))
	}
}

func TestParseCreatesAst(t *testing.T) {
	body := `{
  node(id: 4) {
    id,
    name
  }
}
`
	source := source.New("", body)
	document, err := Parse(
		ParseParams{
			Source: source,
			Options: ParseOptions{
				NoSource: true,
			},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	oDef := ast.OperationDefinition{
		Loc: ast.Location{
			Start: 0, End: 40,
		},
		Operation: "query",
		SelectionSet: &ast.SelectionSet{
			Loc: ast.Location{
				Start: 0, End: 40,
			},
			Selections: []ast.Selection{
				&ast.Field{
					Loc: ast.Location{
						Start: 4, End: 38,
					},
					Name: &ast.Name{
						Loc: ast.Location{
							Start: 4, End: 8,
						},
						Value: "node",
					},
					Arguments: []*ast.Argument{
						{
							Name: &ast.Name{
								Loc: ast.Location{
									Start: 9, End: 11,
								},
								Value: "id",
							},
							Value: &ast.IntValue{
								Loc: ast.Location{
									Start: 13, End: 14,
								},
								Value: "4",
							},
							Loc: ast.Location{
								Start: 9, End: 14,
							},
						},
					},
					SelectionSet: &ast.SelectionSet{
						Loc: ast.Location{
							Start: 16, End: 38,
						},
						Selections: []ast.Selection{
							&ast.Field{
								Loc: ast.Location{
									Start: 22, End: 24,
								},
								Name: &ast.Name{
									Loc: ast.Location{
										Start: 22, End: 24,
									},
									Value: "id",
								},
							},
							&ast.Field{
								Loc: ast.Location{
									Start: 30, End: 34,
								},
								Name: &ast.Name{
									Loc: ast.Location{
										Start: 30, End: 34,
									},
									Value: "name",
								},
							},
						},
					},
				},
			},
		},
	}
	expectedDocument := &ast.Document{
		Loc: ast.Location{
			Start: 0, End: 41,
		},
		Definitions: []ast.Node{&oDef},
	}
	if !reflect.DeepEqual(document, expectedDocument) {
		t.Fatalf("document differs\n%s", pretty.Diff(expectedDocument, document))
	}

}

type errorMessageTest struct {
	source          interface{}
	expectedMessage string
	skipped         bool
}

func testErrorMessage(t *testing.T, test errorMessageTest) {
	if test.skipped {
		t.Skipf("Skipped test: %v", test.source)
	}
	_, err := Parse(ParseParams{Source: test.source})
	checkErrorMessage(t, err, test.expectedMessage)
}

func checkError(t *testing.T, err error, expectedError *gqlerrors.Error) {
	if expectedError == nil {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return // ok
	}
	// else expectedError != nil
	if err == nil {
		t.Fatalf("unexpected nil error\nexpected:\n%v\n\ngot:\n%v", expectedError, err)
	}
	if err.Error() != expectedError.Message {
		t.Fatalf("unexpected error.\nexpected:\n%v\n\ngot:\n%v", expectedError, err.Error())
	}
	gErr := toError(err)
	if gErr == nil {
		t.Fatalf("unexpected nil Error")
	}
	if len(expectedError.Positions) > 0 && !reflect.DeepEqual(gErr.Positions, expectedError.Positions) {
		t.Fatalf("unexpected Error.Positions.\nexpected:\n%v\n\ngot:\n%v", expectedError.Positions, gErr.Positions)
	}
	if len(expectedError.Locations) > 0 && !reflect.DeepEqual(gErr.Locations, expectedError.Locations) {
		t.Fatalf("unexpected Error.Locations.\nexpected:\n%v\n\ngot:\n%v", expectedError.Locations, gErr.Locations)
	}
}

func checkErrorMessage(t *testing.T, err error, expectedMessage string) {
	if err == nil {
		t.Fatalf("unexpected nil error\nexpected:\n%v\n\ngot:\n%v", expectedMessage, err)
	}
	if err.Error() != expectedMessage {
		// only check first line of error message
		lines := strings.Split(err.Error(), "\n")
		if lines[0] != expectedMessage {
			t.Fatalf("unexpected error.\nexpected:\n%v\n\ngot:\n%v", expectedMessage, lines[0])
		}
	}
}

func toError(err error) *gqlerrors.Error {
	if err == nil {
		return nil
	}
	switch err := err.(type) {
	case *gqlerrors.Error:
		return err
	default:
		return nil
	}
}

func TestBadQueryHang(t *testing.T) {
	if _, err := Parse(ParseParams{
		Source: source.New("GraphQL", "{g(d:[d[\xb9\x19 rp\\�{\xef\xbf\xbd2~� c"),
	}); err == nil {
		t.Fatal("Expected an error")
	}
}

func BenchmarkParser(b *testing.B) {
	body := `
mutation _ {
	doSomeCoolStuff(input: {
    objectID: "someKindOfID",
  }) {
    success
    errorCode
    errorMessage
    object {
      id
    }
  }
}

mutation _{
  doAnotherThing(input: {
    objectID: "toThisObject",
    msg: {
      text: "Testing",
      internal: false,
    }
  }) {
    success
    errorCode
    errorMessage
  }
}

query _ {
  queryThatThing(id: "fromThisID", other: 123123123.123123) {
    title
    subtitle
    stuff {
      id
      name
    }
    items {
      edges {
        node {
          id
          data {
            __typename
            ...on Message {
			  summaryMarkup
              textMarkup
            }
          }
        }
      }
    }
  }
}

query _ {
  me {
    account {
      organizations {
        id
      }
    }
  }
}
`
	source := source.New("", body)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Parse(
			ParseParams{
				Source: source,
				Options: ParseOptions{
					NoSource: true,
				},
			},
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}
