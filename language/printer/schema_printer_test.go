package printer_test

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/printer"
	"github.com/sprucehealth/graphql/testutil"
)

func TestSchemaPrinter_PrintsMinimalAST(t *testing.T) {
	astDoc := &ast.ScalarDefinition{
		Name: &ast.Name{
			Value: "foo",
		},
	}
	results := printer.Print(astDoc)
	expected := "scalar foo"
	if !reflect.DeepEqual(results, expected) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, results))
	}
}

func TestSchemaPrinter_DoesNotAlterAST(t *testing.T) {
	b, err := os.ReadFile("../../schema-kitchen-sink.graphql")
	if err != nil {
		t.Fatalf("unable to load schema-kitchen-sink.graphql")
	}

	query := string(b)
	astDoc := parse(t, query)

	astDocBefore := testutil.ASTToJSON(t, astDoc)

	_ = printer.Print(astDoc)

	astDocAfter := testutil.ASTToJSON(t, astDoc)

	_ = testutil.ASTToJSON(t, astDoc)

	if !reflect.DeepEqual(astDocAfter, astDocBefore) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(astDocAfter, astDocBefore))
	}
}

func TestSchemaPrinter_PrintsKitchenSink(t *testing.T) {
	b, err := os.ReadFile("../../schema-kitchen-sink.graphql")
	if err != nil {
		t.Fatalf("unable to load schema-kitchen-sink.graphql")
	}

	query := string(b)
	astDoc := parse(t, query)
	expected := `schema {
  query: QueryType
  mutation: MutationType
}

type Foo implements Bar {
  one: Type
  two(argument: InputType!): Type
  three(argument: InputType, other: String): Int
  four(argument: String = "string"): String
  five(argument: [String] = ["string", "string"]): String
  six(argument: InputType = {key: "value"}): Type
}

type AnnotatedObject @onObject(arg: "value") {
  annotatedField(arg: Type = "default" @onArg): Type @onField
}

interface Bar {
  one: Type
  four(argument: String = "string"): String
}

interface AnnotatedInterface @onInterface {
  annotatedField(arg: Type @onArg): Type @onField
}

union Feed = Story | Article | Advert

union AnnotatedUnion @onUnion = A | B

scalar CustomScalar

scalar AnnotatedScalar @onScalar

enum Site {
  DESKTOP
  MOBILE
}

enum AnnotatedEnum @onEnum {
  ANNOTATED_VALUE @onEnumValue
  OTHER_VALUE
}

input InputType {
  key: String!
  answer: Int = 42
}

input AnnotatedInput @onInputObjectType {
  annotatedField: Type @onField
}

extend type Foo {
  seven(argument: [String]): Type
}

extend type Foo @onType {}

type NoFields {}

directive @skip(if: Boolean!) on FIELD | FRAGMENT_SPREAD | INLINE_FRAGMENT

directive @include(if: Boolean!) on FIELD | FRAGMENT_SPREAD | INLINE_FRAGMENT
`
	results := printer.Print(astDoc)
	if !reflect.DeepEqual(expected, results) {
		for _, l := range testutil.Diff(results, expected) {
			x := strings.Split(l, " != ")
			if len(x) != 2 {
				t.Logf("%s", l)
			} else {
				x1, err1 := strconv.Unquote(x[0])
				x2, err2 := strconv.Unquote(x[1])
				if err1 != nil || err2 != nil {
					t.Logf("%s", l)
				} else {
					t.Logf("%s\n!=\n%s", x1, x2)
				}
			}
		}
		t.Fatalf("Unexpected result")
	}
}
