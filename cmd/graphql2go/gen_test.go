package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sprucehealth/graphql/language/parser"
)

var update = flag.Bool("update", false, "update golden files")

// TestGenerator runs the server generator against testdata/directives.graphql, which
// exercises every supported gqlgen-style directive, and compares the gofmt'd output to a
// golden file. Run with -update to regenerate the golden file.
func TestGenerator(t *testing.T) {
	schemaPath := filepath.Join("testdata", "directives.graphql")
	b, err := os.ReadFile(schemaPath) //nolint:gosec // fixed testdata path
	if err != nil {
		t.Fatal(err)
	}
	root, err := parser.Parse(parser.ParseParams{
		Source:  string(b),
		Options: parser.ParseOptions{KeepComments: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	g := newGenerator(&buf, root)
	generateServer(g)
	raw := buf.String()

	// Codegen directives must never leak into the generated runtime schema, neither as
	// directive definitions nor as applied directives on fields.
	for name := range codegenDirectives {
		if defName := goDirectiveDefName(name); strings.Contains(raw, defName) {
			t.Errorf("generated output references codegen directive definition %q", defName)
		}
		if quoted := fmt.Sprintf("%q", name); strings.Contains(raw, quoted) {
			t.Errorf("generated output contains codegen directive name %s", quoted)
		}
	}
	if !strings.Contains(raw, "var Directives = []*graphql.Directive{\n}") {
		t.Errorf("expected an empty runtime Directives list, got:\n%s", raw)
	}

	// An omittable custom-resolver field's interface method returns a pointer (here a
	// custom scalar bound to time.Time)...
	if want := "LastActive(ctx context.Context, parent *User, p graphql.ResolveParams) (*time.Time, error)"; !strings.Contains(raw, want) {
		t.Errorf("expected omittable resolver to return a pointer:\n%s\nnot found in:\n%s", want, raw)
	}
	// ...while a non-omittable custom-resolver field is unchanged (backwards compat).
	if want := "Computed(ctx context.Context, parent *User, p graphql.ResolveParams) (string, error)"; !strings.Contains(raw, want) {
		t.Errorf("expected non-omittable resolver to keep value return:\n%s\nnot found in:\n%s", want, raw)
	}

	formatted, err := format.Source([]byte(raw))
	if err != nil {
		t.Fatalf("generated output is not valid Go: %v\n%s", err, raw)
	}

	goldenPath := filepath.Join("testdata", "directives.server.go.golden")
	if *update {
		if err := os.WriteFile(goldenPath, formatted, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath) //nolint:gosec // fixed testdata path
	if err != nil {
		t.Fatal(err)
	}
	if string(formatted) != string(want) {
		t.Errorf("generated output does not match %s; rerun with -update to regenerate", goldenPath)
	}
}
