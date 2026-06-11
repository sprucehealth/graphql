package graphql_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/ast"
)

// emailScalar is a custom scalar whose ParseValue/ParseLiteral reject invalid
// values with a descriptive error. It exercises the error return on the scalar
// decode functions, ensuring the custom message reaches the client on both the
// literal (constant) and variable paths.
var emailScalar = graphql.NewScalar(graphql.ScalarConfig{
	Name: "Email",
	Serialize: func(value any) (any, error) {
		s, ok := value.(string)
		if !ok {
			return nil, nil
		}
		return s, nil
	},
	ParseValue: func(value any) (any, error) {
		s, ok := value.(string)
		if !ok {
			return nil, errors.New("Email must be a string")
		}
		if !strings.Contains(s, "@") {
			return nil, errors.New("Email must contain an @")
		}
		return s, nil
	},
	ParseLiteral: func(valueAST ast.Value) (any, error) {
		sv, ok := valueAST.(*ast.StringValue)
		if !ok {
			return nil, errors.New("Email must be a string")
		}
		if !strings.Contains(sv.Value, "@") {
			return nil, errors.New("Email must contain an @")
		}
		return sv.Value, nil
	},
})

var emailTestSchema, _ = graphql.NewSchema(graphql.SchemaConfig{
	Query: graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"echoEmail": &graphql.Field{
				Type: graphql.String,
				Args: graphql.FieldConfigArgument{
					"email": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(emailScalar),
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					return p.Args["email"], nil
				},
			},
		},
	}),
})

// findError returns the first error whose message contains substr.
func findError(t *testing.T, errs []gqlerrors.FormattedError, substr string) gqlerrors.FormattedError {
	t.Helper()
	for _, err := range errs {
		if strings.Contains(err.Message, substr) {
			return err
		}
	}
	t.Fatalf("no error containing %q, got %v", substr, errs)
	return gqlerrors.FormattedError{}
}

func TestScalarParseError_ValidLiteral(t *testing.T) {
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        emailTestSchema,
		RequestString: `{ echoEmail(email: "a@b.com") }`,
	})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if got := result.Data.(map[string]any)["echoEmail"]; got != "a@b.com" {
		t.Fatalf("expected a@b.com, got %v", got)
	}
}

func TestScalarParseError_InvalidLiteral(t *testing.T) {
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        emailTestSchema,
		RequestString: `{ echoEmail(email: "not-an-email") }`,
	})
	err := findError(t, result.Errors, "Email must contain an @")
	if err.Type != gqlerrors.ErrorTypeBadQuery {
		t.Errorf("expected error type %q, got %q", gqlerrors.ErrorTypeBadQuery, err.Type)
	}
	// The custom message is embedded within the argument-validation envelope.
	if !strings.Contains(err.Message, `Argument "email" has invalid value`) {
		t.Errorf("expected argument envelope, got %q", err.Message)
	}
}

func TestScalarParseError_ValidVariable(t *testing.T) {
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:         emailTestSchema,
		RequestString:  `query ($e: Email!) { echoEmail(email: $e) }`,
		VariableValues: map[string]any{"e": "a@b.com"},
	})
	if len(result.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if got := result.Data.(map[string]any)["echoEmail"]; got != "a@b.com" {
		t.Fatalf("expected a@b.com, got %v", got)
	}
}

func TestScalarParseError_InvalidVariable(t *testing.T) {
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:         emailTestSchema,
		RequestString:  `query ($e: Email!) { echoEmail(email: $e) }`,
		VariableValues: map[string]any{"e": "not-an-email"},
	})
	err := findError(t, result.Errors, "Email must contain an @")
	if err.Type != gqlerrors.ErrorTypeInvalidInput {
		t.Errorf("expected error type %q, got %q", gqlerrors.ErrorTypeInvalidInput, err.Type)
	}
	// The custom message is embedded within the variable-validation envelope.
	if !strings.Contains(err.Message, `Variable "$e" got invalid value`) {
		t.Errorf("expected variable envelope, got %q", err.Message)
	}
}

// TestScalarParseError_NestedInInputObject ensures a scalar error nested inside
// an input object surfaces with the per-field prefix.
func TestScalarParseError_NestedInInputObject(t *testing.T) {
	inputType := graphql.NewInputObject(graphql.InputObjectConfig{
		Name: "Contact",
		Fields: graphql.InputObjectConfigFieldMap{
			"email": &graphql.InputObjectFieldConfig{
				Type: emailScalar,
			},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"addContact": &graphql.Field{
					Type: graphql.String,
					Args: graphql.FieldConfigArgument{
						"contact": &graphql.ArgumentConfig{Type: inputType},
					},
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return "ok", nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("failed to build schema: %v", err)
	}
	result := graphql.Do(context.Background(), graphql.Params{
		Schema:        schema,
		RequestString: `{ addContact(contact: {email: "nope"}) }`,
	})
	got := findError(t, result.Errors, "Email must contain an @")
	if !strings.Contains(got.Message, `In field "email"`) {
		t.Errorf("expected per-field prefix, got %q", got.Message)
	}
}
