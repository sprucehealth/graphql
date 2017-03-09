package printer_test

import (
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/parser"
	"github.com/sprucehealth/graphql/language/printer"
	"github.com/sprucehealth/graphql/testutil"
)

func parse(t testing.TB, query string) *ast.Document {
	astDoc, err := parser.Parse(parser.ParseParams{
		Source:  query,
		Options: parser.ParseOptions{},
	})
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	return astDoc
}

func TestDoesNotAlterAST(t *testing.T) {
	b, err := ioutil.ReadFile("../../kitchen-sink.graphql")
	if err != nil {
		t.Fatalf("unable to load kitchen-sink.graphql")
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

func TestPrintsMinimalAST(t *testing.T) {
	astDoc := &ast.Field{
		Name: &ast.Name{
			Value: "foo",
		},
	}
	results := printer.Print(astDoc)
	expected := "foo"
	if !reflect.DeepEqual(results, expected) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(expected, results))
	}
}

func TestPrintsKitchenSink(t *testing.T) {
	b, err := ioutil.ReadFile("../../kitchen-sink.graphql")
	if err != nil {
		t.Fatalf("unable to load kitchen-sink.graphql")
	}

	query := string(b)
	astDoc := parse(t, query)
	expected := `query namedQuery($foo: ComplexFooType, $bar: Bar = DefaultBarValue) {
  customUser: user(id: [987, 654]) {
    id
    ... on User @defer {
      field2 {
        id
        alias: field1(first: 10, after: $foo) @include(if: $foo) {
          id
          ...frag
        }
      }
    }
  }
}

mutation favPost {
  fav(post: 123) @defer {
    post {
      id
    }
  }
}

fragment frag on Follower {
  foo(size: $size, bar: $b, obj: {key: "value"})
}

{
  unnamed(truthyVal: true, falseyVal: false)
  query
}
`

	results := printer.Print(astDoc)
	if !reflect.DeepEqual(expected, results) {
		t.Fatalf("Unexpected result, Diff: %v", testutil.Diff(results, expected))
	}
}

func TestComments(t *testing.T) {
	source := `# Unconnected comment
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

# Blah
interface SettingValue {
	# key doc
	key: Boolean # key line
}

# non doc comment
# booo

# input doc
input SomeInput {
	# bar doc
	bar: String # bar comment
}
`
	document, err := parser.Parse(parser.ParseParams{Source: source, Options: parser.ParseOptions{NoSource: true, KeepComments: true}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// TODO: the printer doesn't yet handle non doc or line comments
	expected := `# Type doc comment
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

# Blah
interface SettingValue {
  # key doc
  key: Boolean # key line
}

# input doc
input SomeInput {
  # bar doc
  bar: String # bar comment
}
`

	res := printer.Print(document)
	if res != expected {
		println(len(res), len(expected))
		t.Fatalf("Expected:\n%s\ngot:\n%s", expected, res)
	}
}

func BenchmarkPrint(b *testing.B) {
	buf, err := ioutil.ReadFile("../../kitchen-sink.graphql")
	if err != nil {
		b.Fatalf("unable to load kitchen-sink.graphql: %s", err)
	}
	astDoc := parse(b, string(buf))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		printer.Print(astDoc)
	}
}
