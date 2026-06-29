# graphql2go

`graphql2go` parses a `.graphql` schema and emits Go.

```bash
graphql2go -schema schema.graphql -artifact server -out schema.go   # resolver interfaces + type registration
graphql2go -schema schema.graphql -artifact client -out client.go   # client methods
```

Run `graphql2go -h` for the full flag list (`-config`, `-schema`, `-out`,
`-client_types`, `-nullable_inputs`, `-assert_identity`, `-v`).

## Customization

Generation can be customized two ways, which coexist:

1. A **JSON config file** (`-config`) — see the `config` struct in `main.go`.
2. **Schema directives** embedded inline, matching the
   [99designs/gqlgen](https://github.com/99designs/gqlgen) directive set.

When both specify a value for the same type or field, **the directive wins**.

The directive *definitions* may be included verbatim in the schema (so the schema
stays compatible with gqlgen); graphql2go recognizes these names intrinsically and
never emits them into the generated runtime schema.

An **unknown argument** on any of these directives is a hard error (e.g.
`@goModel(type: "...")` fails — `@goModel` takes `model:`, the `type:` argument belongs to
`@goField`). A **scalar used as a field type** may be given a Go type mapping via
`@goModel(model: "...")` or the `CustomScalarTypes` config; without one it defaults to using
the scalar's own name as the Go type (valid when the generated code lives in that type's
package). Either way the scalar is a value type, so pointer-wrapping follows
nullability/omittable — a non-null field is unwrapped, a nullable (omittable) field is a
pointer.

### Supported directives

```graphql
directive @goModel(model: String, models: [String!], forceGenerate: Boolean)
  on OBJECT | INPUT_OBJECT | SCALAR | ENUM | INTERFACE | UNION
directive @goField(forceResolver: Boolean, name: String, omittable: Boolean, type: String,
                   autoBindGetterHaser: Boolean, forceGenerate: Boolean, batch: Boolean)
  on INPUT_FIELD_DEFINITION | FIELD_DEFINITION
directive @goTag(key: String!, value: String) repeatable
  on INPUT_FIELD_DEFINITION | FIELD_DEFINITION
directive @goExtraField(name: String, type: String!, overrideTags: String, description: String) repeatable
  on OBJECT | INPUT_OBJECT
directive @goModelCompatibility(nullOmittable: Boolean) on OBJECT | INPUT_OBJECT
directive @inlineArguments on ARGUMENT_DEFINITION
```

| Directive | Behavior in graphql2go |
| --- | --- |
| `@goModel` on **SCALAR** | Binds the scalar's Go type (e.g. `@goModel(model: "time.Time")`). Equivalent to the config's `CustomScalarTypes`. |
| `@goModel` on **ENUM** | Binds the enum to an external Go type: no `type X string` or constants are generated, field references use the bound type, and the runtime enum emits values as `BoundType("VALUE")`. `forceGenerate: true` generates the enum normally instead. |
| `@goModel` on OBJECT / INPUT_OBJECT / INTERFACE / UNION | **Not supported** — errors. |
| `@goField(forceResolver: true)` | Generates a custom resolver for the field (config's `Resolvers`). |
| `@goField(type: "pkg.T")` | Overrides the Go type of the field (config's `CustomFieldTypes`). |
| `@goField(name: "GoName")` | Overrides the generated Go struct field name. |
| `@goField(omittable: true)` | Generates the field as a pointer so its absence is distinguishable. On input fields it always points; on output fields only value types (scalars and enums) gain a pointer — non-null fields, objects, interfaces, unions, lists, and fields with an explicit `type:` override are left unchanged. |
| `@goField(autoBindGetterHaser / batch / forceGenerate)` | Recognized but ignored (no graphql2go equivalent). |
| `@goTag(key, value)` | Adds/overrides a struct tag on the generated field. A key matching a default tag (`json`, `gql`) replaces it; other keys are appended. |
| `@goExtraField(name, type, overrideTags, description)` | Adds a synthetic Go field to the model (config's `ExtraFields`). `overrideTags` replaces the default ``json:"-"``; `description` becomes a doc comment. Works on objects and input objects. |
| `@goModelCompatibility(nullOmittable: Boolean)` | Sets, for every field on the model, whether nullable fields are omittable (pointer-wrapped). Overrides the global `NullOmittable` config; overridden per field by `@goField(omittable:)`. |
| `@inlineArguments` | Recognized but a no-op (unimplemented). |

Go types named in `model`/`type`/`overrideTags` are emitted verbatim — graphql2go
does not manage imports, so the caller must ensure the referenced packages are
importable, exactly as with the JSON config's `CustomScalarTypes`/`CustomFieldTypes`.

### Omittable / nullable pointers

By default a nullable field is **not** pointer-wrapped (output scalars/enums are value
types; inputs follow the legacy `-nullable_inputs` flag and `NullableInputTypes` config).
Setting `"NullOmittable": true` in the JSON config makes every nullable field on inputs
and output models *omittable* — pointer-wrapped so absence is distinguishable from the
zero value. The decision resolves most-specific-first:

1. `@goField(omittable: Boolean)` on the field.
2. `@goModelCompatibility(nullOmittable: Boolean)` on the model.
3. (inputs only) `NullableInputTypes[type]`, then the `-nullable_inputs` flag.
4. The global `NullOmittable` config (default `false`).

A field is pointer-wrapped only when omittable **and** nullable: non-null fields are never
wrapped, and reference types already nilable in Go — objects (already `*T`), interfaces,
unions and lists — are left unchanged (only value types gain a pointer). The default
(`NullOmittable` off, no directives) reproduces the original output exactly.

See `testdata/directives.graphql` for an example schema exercising every directive and
`testdata/directives.server.go.golden` for the generated output (regenerate with
`go test ./cmd/graphql2go -run TestGenerator -update`).
