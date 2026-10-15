package graphql_test

import (
	"testing"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/testutil"
)

func TestValidate_SingleFieldSubscriptions_ValidSubscription(t *testing.T) {
	testutil.ExpectPassesRule(t, graphql.SingleFieldSubscriptionsRule, `
      subscription ImportantEmails {
        importantEmails
      }
    `)
}

func TestValidate_SingleFieldSubscriptions_ValidWithFragment(t *testing.T) {
	testutil.ExpectPassesRule(t, graphql.SingleFieldSubscriptionsRule, `
      subscription ImportantEmails {
        ...newEmails
      }
      fragment newEmails on Subscription {
        importantEmails
      }
    `)
}

func TestValidate_SingleFieldSubscriptions_QueriesAreUnaffected(t *testing.T) {
	testutil.ExpectPassesRule(t, graphql.SingleFieldSubscriptionsRule, `
      query { a b __typename }
    `)
}

func TestValidate_SingleFieldSubscriptions_TwoFields(t *testing.T) {
	testutil.ExpectFailsRule(t, graphql.SingleFieldSubscriptionsRule, `
      subscription ImportantEmails {
        importantEmails
        notImportantEmails
      }
    `, []gqlerrors.FormattedError{
		testutil.RuleError(`Subscription "ImportantEmails" must select only one top level field.`, 3, 9, 4, 9),
	})
}

func TestValidate_SingleFieldSubscriptions_TwoFieldsViaFragment(t *testing.T) {
	testutil.ExpectFailsRule(t, graphql.SingleFieldSubscriptionsRule, `
      subscription ImportantEmails {
        importantEmails
        ...other
      }
      fragment other on Subscription {
        notImportantEmails
      }
    `, []gqlerrors.FormattedError{
		testutil.RuleError(`Subscription "ImportantEmails" must select only one top level field.`, 3, 9, 7, 9),
	})
}

func TestValidate_SingleFieldSubscriptions_AnonymousTwoFields(t *testing.T) {
	testutil.ExpectFailsRule(t, graphql.SingleFieldSubscriptionsRule, `
      subscription {
        importantEmails
        notImportantEmails
      }
    `, []gqlerrors.FormattedError{
		testutil.RuleError(`Anonymous Subscription must select only one top level field.`, 3, 9, 4, 9),
	})
}

func TestValidate_SingleFieldSubscriptions_Introspection(t *testing.T) {
	testutil.ExpectFailsRule(t, graphql.SingleFieldSubscriptionsRule, `
      subscription ImportantEmails {
        __typename
      }
    `, []gqlerrors.FormattedError{
		testutil.RuleError(`Subscription "ImportantEmails" must not select an introspection top level field.`, 3, 9),
	})
}
