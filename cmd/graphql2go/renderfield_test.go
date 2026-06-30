package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/parser"
)

// renderFieldSchema exercises every branch of renderFieldDefinition:
//   - Node.id: named form (noName=false) with a leading comment and no deprecation
//   - User.name: a description
//   - User.legacy: a deprecation reason (which suppresses the comment)
//   - User.flagged: a runtime directive (rendered into the Directives slice)
//   - User.avatar: arguments without a custom resolver
//   - User.bio: a custom resolver with no arguments on a regular object
//   - User.fullProfile: a custom resolver with arguments on a regular object
//   - Query.me: a custom resolver with no arguments on a top-level object (map source)
//   - Query.search: a custom resolver with arguments on a top-level object
const renderFieldSchema = `
directive @goField(forceResolver: Boolean, name: String, omittable: Boolean, type: String) on FIELD_DEFINITION
directive @feature(enabled: Boolean) on FIELD_DEFINITION

interface Node {
  # the unique id
  id: ID!
}

type Query {
  me: User @goField(forceResolver: true)
  search(query: String!, limit: Int): User @goField(forceResolver: true)
}

type User {
  id: ID!
  "The display name"
  name: String
  legacy: String @deprecated(reason: "use name")
  flagged: Boolean @feature(enabled: true)
  avatar(size: Int): String
  bio: String @goField(forceResolver: true)
  fullProfile(detailed: Boolean): String @goField(forceResolver: true)
}
`

func newRenderFieldGenerator(t *testing.T) *generator {
	t.Helper()
	root, err := parser.Parse(parser.ParseParams{
		Source:  renderFieldSchema,
		Options: parser.ParseOptions{KeepComments: true},
	})
	if err != nil {
		t.Fatalf("failed to parse schema: %v", err)
	}
	var buf bytes.Buffer
	return newGenerator(&buf, root)
}

func fieldByName(t *testing.T, g *generator, typeName, fieldName string) *ast.FieldDefinition {
	t.Helper()
	node, ok := g.types[typeName]
	if !ok {
		t.Fatalf("type %q not found", typeName)
	}
	var fields []*ast.FieldDefinition
	switch d := node.(type) {
	case *ast.ObjectDefinition:
		fields = d.Fields
	case *ast.InterfaceDefinition:
		fields = d.Fields
	default:
		t.Fatalf("type %q is not an object or interface (%T)", typeName, node)
	}
	for _, f := range fields {
		if f.Name.Value == fieldName {
			return f
		}
	}
	t.Fatalf("field %q not found on type %q", fieldName, typeName)
	return nil
}

// TestRenderFieldDefinition is a characterization test pinning the exact output of
// renderFieldDefinition for every branch. Run with -update to regenerate the golden file.
func TestRenderFieldDefinition(t *testing.T) {
	g := newRenderFieldGenerator(t)
	cases := []struct {
		objName   string
		fieldName string
		noName    bool
	}{
		{"Node", "id", false},
		{"User", "name", true},
		{"User", "legacy", true},
		{"User", "flagged", true},
		{"User", "avatar", true},
		{"User", "bio", true},
		{"User", "fullProfile", true},
		{"Query", "me", true},
		{"Query", "search", true},
	}
	var buf bytes.Buffer
	for _, c := range cases {
		f := fieldByName(t, g, c.objName, c.fieldName)
		out := g.renderFieldDefinition(c.objName, f, c.noName)
		fmt.Fprintf(&buf, "=== %s.%s (noName=%t) ===\n%s\n\n", c.objName, c.fieldName, c.noName, out)
	}

	goldenPath := filepath.Join("testdata", "renderfield.golden")
	if *update {
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath) //nolint:gosec // fixed testdata path
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != string(want) {
		t.Errorf("renderFieldDefinition output does not match %s; rerun with -update to regenerate\n--- got ---\n%s", goldenPath, buf.String())
	}
}
