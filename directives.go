package graphql

import "github.com/sprucehealth/graphql/gqlerrors"

const (
	DirectiveLocationQuery              = "QUERY"
	DirectiveLocationMutation           = "MUTATION"
	DirectiveLocationSubscription       = "SUBSCRIPTION"
	DirectiveLocationField              = "FIELD"
	DirectiveLocationFragmentDefinition = "FRAGMENT_DEFINITION"
	DirectiveLocationFragmentSpread     = "FRAGMENT_SPREAD"
	DirectiveLocationInlineFragment     = "INLINE_FRAGMENT"
)

// Directive structs are used by the GraphQL runtime as a way of modifying execution
// behavior. Type system creators will usually not create these directly.
type Directive struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Locations   []string    `json:"locations"`
	Args        []*Argument `json:"args"`

	err error
}

func NewDirective(config *Directive) *Directive {
	if config == nil {
		config = &Directive{}
	}
	dir := &Directive{}

	// Ensure directive is named
	if config.Name == "" {
		dir.err = gqlerrors.NewFormattedError("Directive must be named.")
		return dir
	}

	// Ensure directive name is valid
	var err = assertValidName(config.Name)
	if err != nil {
		dir.err = err
		return dir
	}

	// Ensure locations are provided for directive
	if len(config.Locations) == 0 {
		dir.err = gqlerrors.NewFormattedError("Must provide locations for directive.")
		return dir
	}

	dir.Name = config.Name
	dir.Description = config.Description
	dir.Locations = config.Locations
	dir.Args = config.Args
	return dir
}

// IncludeDirective is used to conditionally include fields or fragments
var IncludeDirective = NewDirective(&Directive{
	Name: "include",
	Description: "Directs the executor to include this field or fragment only when " +
		"the `if` argument is true.",
	Locations: []string{
		DirectiveLocationField,
		DirectiveLocationFragmentSpread,
		DirectiveLocationInlineFragment,
	},
	Args: []*Argument{
		{
			PrivateName:        "if",
			Type:               NewNonNull(Boolean),
			PrivateDescription: "Included when true.",
		},
	},
})

// SkipDirective Used to conditionally skip (exclude) fields or fragments
var SkipDirective = NewDirective(&Directive{
	Name: "skip",
	Description: "Directs the executor to skip this field or fragment when the `if` " +
		"argument is true.",
	Args: []*Argument{
		{
			PrivateName:        "if",
			Type:               NewNonNull(Boolean),
			PrivateDescription: "Skipped when true.",
		},
	},
	Locations: []string{
		DirectiveLocationField,
		DirectiveLocationFragmentSpread,
		DirectiveLocationInlineFragment,
	},
})
