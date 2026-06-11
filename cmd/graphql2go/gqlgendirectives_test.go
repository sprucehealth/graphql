package main

import (
	"bytes"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/language/parser"
)

// newDirectiveTestGenerator parses a schema and returns a generator with the directive
// maps initialised, ready for applySchemaDirectives. It bypasses the full newGenerator
// pipeline (cycle detection, top-level resolver enforcement) to keep the test focused.
func newDirectiveTestGenerator(t *testing.T, schema string) *generator {
	t.Helper()
	root, err := parser.Parse(parser.ParseParams{Source: schema})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	g := &generator{doc: root, types: make(map[string]ast.Node)}
	g.cfg.Resolvers = make(map[string][]string)
	g.cfg.CustomFieldTypes = make(map[string]string)
	g.cfg.CustomScalarTypes = make(map[string]string)
	g.goFieldNames = make(map[string]string)
	g.goTags = make(map[string][]structTag)
	g.omittableFields = make(map[string]bool)
	g.modelNullOmittable = make(map[string]bool)
	g.boundEnums = make(map[string]string)
	g.extraFields = make(map[string]map[string]extraFieldSpec)
	return g
}

func TestApplySchemaDirectives(t *testing.T) {
	schema := `
scalar DateTime @goModel(model: "time.Time")

enum Status @goModel(model: "statuspkg.Status") {
  ACTIVE
}

type User {
  displayName: String @goField(name: "FullName") @goTag(key: "json", value: "full_name") @goTag(key: "db", value: "full_name")
  computed: String @goField(forceResolver: true)
  externalRef: String @goField(type: "refpkg.Ref")
  lastSeen: Int @goField(omittable: true)
}

input UserInput @goExtraField(name: "TraceID", type: "string", description: "trace id") {
  nickname: String @goField(name: "Nick", omittable: true)
}`

	g := newDirectiveTestGenerator(t, schema)
	g.applySchemaDirectives()

	if got := g.cfg.CustomScalarTypes["DateTime"]; got != "time.Time" {
		t.Errorf(`CustomScalarTypes["DateTime"] = %q, want "time.Time"`, got)
	}
	if got := g.boundEnums["Status"]; got != "statuspkg.Status" {
		t.Errorf(`boundEnums["Status"] = %q, want "statuspkg.Status"`, got)
	}
	if got := g.goFieldNames["User.displayName"]; got != "FullName" {
		t.Errorf(`goFieldNames["User.displayName"] = %q, want "FullName"`, got)
	}
	if got := g.cfg.CustomFieldTypes["User.externalRef"]; got != "refpkg.Ref" {
		t.Errorf(`CustomFieldTypes["User.externalRef"] = %q, want "refpkg.Ref"`, got)
	}
	if !slices.Contains(g.cfg.Resolvers["User"], "computed") {
		t.Errorf(`Resolvers["User"] = %v, want to contain "computed"`, g.cfg.Resolvers["User"])
	}
	wantTags := []structTag{{key: "json", value: "full_name"}, {key: "db", value: "full_name"}}
	if got := g.goTags["User.displayName"]; !reflect.DeepEqual(got, wantTags) {
		t.Errorf(`goTags["User.displayName"] = %v, want %v`, got, wantTags)
	}
	if !g.omittableFields["UserInput.nickname"] {
		t.Errorf("UserInput.nickname should be omittable")
	}
	if !g.omittableFields["User.lastSeen"] {
		t.Errorf("User.lastSeen (output field) should be omittable")
	}
	if got := g.goFieldNames["UserInput.nickname"]; got != "Nick" {
		t.Errorf(`goFieldNames["UserInput.nickname"] = %q, want "Nick"`, got)
	}
	wantExtra := extraFieldSpec{name: "TraceID", goType: "string", description: "trace id"}
	if got := g.extraFields["UserInput"]["TraceID"]; got != wantExtra {
		t.Errorf(`extraFields["UserInput"]["TraceID"] = %+v, want %+v`, got, wantExtra)
	}
}

// TestApplySchemaDirectives_DirectivesWin verifies a directive overrides a value seeded
// from the JSON config.
func TestApplySchemaDirectives_DirectivesWin(t *testing.T) {
	g := newDirectiveTestGenerator(t, `type User { externalRef: String @goField(type: "refpkg.Ref") }`)
	g.cfg.CustomFieldTypes["User.externalRef"] = "fromconfig"
	g.applySchemaDirectives()
	if got := g.cfg.CustomFieldTypes["User.externalRef"]; got != "refpkg.Ref" {
		t.Errorf("directive should win: got %q, want %q", got, "refpkg.Ref")
	}
}

func TestApplySchemaDirectives_GoModelOnObjectErrors(t *testing.T) {
	g := newDirectiveTestGenerator(t, `type Foo @goModel(model: "x.Y") { id: ID }`)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected @goModel on OBJECT to fail")
		}
	}()
	g.applySchemaDirectives()
}

func TestApplySchemaDirectives_GoModelMultipleModelsErrors(t *testing.T) {
	g := newDirectiveTestGenerator(t, `scalar Foo @goModel(models: ["a.B", "c.D"])`)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected @goModel with multiple models to fail")
		}
	}()
	g.applySchemaDirectives()
}

func TestFieldOmittable(t *testing.T) {
	cases := []struct {
		name    string
		isInput bool
		setup   func(g *generator)
		want    bool
	}{
		{name: "default off", want: false},
		{name: "global on", setup: func(g *generator) { g.cfg.NullOmittable = true }, want: true},
		{name: "model true beats global off", setup: func(g *generator) { g.modelNullOmittable["T"] = true }, want: true},
		{name: "model false beats global on", setup: func(g *generator) {
			g.cfg.NullOmittable = true
			g.modelNullOmittable["T"] = false
		}, want: false},
		{name: "field true beats model false", setup: func(g *generator) {
			g.modelNullOmittable["T"] = false
			g.omittableFields["T.f"] = true
		}, want: true},
		{name: "field false beats model true", setup: func(g *generator) {
			g.modelNullOmittable["T"] = true
			g.omittableFields["T.f"] = false
		}, want: false},
		{name: "input NullableInputTypes true", isInput: true, setup: func(g *generator) {
			g.cfg.NullableInputTypes = map[string]bool{"T": true}
		}, want: true},
		{name: "input NullableInputTypes false beats global on", isInput: true, setup: func(g *generator) {
			g.cfg.NullOmittable = true
			g.cfg.NullableInputTypes = map[string]bool{"T": false}
		}, want: false},
		{name: "output ignores NullableInputTypes", isInput: false, setup: func(g *generator) {
			g.cfg.NullableInputTypes = map[string]bool{"T": true}
		}, want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := &generator{omittableFields: map[string]bool{}, modelNullOmittable: map[string]bool{}}
			if c.setup != nil {
				c.setup(g)
			}
			if got := g.fieldOmittable("T", "f", c.isInput); got != c.want {
				t.Errorf("fieldOmittable = %v, want %v", got, c.want)
			}
		})
	}
}

func TestFieldOmittable_NullableInputsFlag(t *testing.T) {
	old := *flagNullableInputs
	*flagNullableInputs = true
	defer func() { *flagNullableInputs = old }()
	g := &generator{omittableFields: map[string]bool{}, modelNullOmittable: map[string]bool{}}
	if !g.fieldOmittable("T", "f", true) {
		t.Error("input field should be omittable via -nullable_inputs")
	}
	if g.fieldOmittable("T", "f", false) {
		t.Error("output field should ignore -nullable_inputs")
	}
}

func TestApplySchemaDirectives_UnknownArgErrors(t *testing.T) {
	cases := []struct {
		name   string
		schema string
	}{
		{"goModel type instead of model", `scalar Timestamp @goModel(type: "time.Time")`},
		{"goField unknown", `type T { f: String @goField(bogus: true) }`},
		{"goTag unknown", `type T { f: String @goTag(bad: "x") }`},
		{"goExtraField unknown", `type T @goExtraField(name: "X", type: "int", nope: "y") { f: String }`},
		{"goModelCompatibility unknown", `type T @goModelCompatibility(bogus: true) { f: String }`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := newDirectiveTestGenerator(t, c.schema)
			defer func() {
				r := recover()
				if r == nil {
					t.Fatal("expected panic for unknown argument")
				}
				if err, ok := r.(error); !ok || !strings.Contains(err.Error(), "unknown argument") {
					t.Fatalf("expected an 'unknown argument' error, got %v", r)
				}
			}()
			g.applySchemaDirectives()
		})
	}
}

func TestGoType_MappedScalar(t *testing.T) {
	g := newDirectiveTestGenerator(t, `scalar Foo @goModel(model: "foopkg.Foo")`)
	g.types["Foo"] = g.doc.Definitions[0]
	g.applySchemaDirectives()
	fooType := &ast.Named{Name: &ast.Name{Value: "Foo"}}
	if got := g.goType(fooType, "T.f"); got != "foopkg.Foo" {
		t.Errorf("goType = %q, want %q", got, "foopkg.Foo")
	}
}

// TestNullOmittableGlobal verifies the global NullOmittable config pointer-wraps nullable
// fields (but not non-null fields) on both output and input models.
func TestNullOmittableGlobal(t *testing.T) {
	g := newDirectiveTestGenerator(t, `type Thing { id: ID! name: String count: Int }
input ThingInput { id: ID! name: String }`)
	g.cfg.NullOmittable = true
	for _, def := range g.doc.Definitions {
		switch d := def.(type) {
		case *ast.ObjectDefinition:
			g.types[d.Name.Value] = d
		case *ast.InputObjectDefinition:
			g.types[d.Name.Value] = d
		}
	}
	var buf bytes.Buffer
	g.w = &buf
	for _, def := range g.doc.Definitions {
		switch d := def.(type) {
		case *ast.ObjectDefinition:
			g.genObjectModel(d)
		case *ast.InputObjectDefinition:
			g.genInputModel(d)
		}
	}
	out := buf.String()
	for _, want := range []string{"Name *string", "Count *int", "ID string"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in generated output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "ID *string") {
		t.Errorf("non-null ID should not be a pointer:\n%s", out)
	}
}
