package graphql_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/language/ast"
	"github.com/sprucehealth/graphql/testutil"
)

type subscriptionTestEvent struct {
	Seq  int    `json:"seq"`
	Text string `json:"text"`
}

func subscriptionTestSchema(t *testing.T, subscribe graphql.FieldSubscribeFn) graphql.Schema {
	t.Helper()
	eventType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Event",
		Fields: graphql.Fields{
			"seq":  &graphql.Field{Type: graphql.Int},
			"text": &graphql.Field{Type: graphql.String},
			"boom": &graphql.Field{
				Type: graphql.String,
				Resolve: func(context.Context, graphql.ResolveParams) (any, error) {
					return nil, errors.New("boom")
				},
			},
		},
	})
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Query",
			Fields: graphql.Fields{"a": &graphql.Field{Type: graphql.String}},
		}),
		Subscription: graphql.NewObject(graphql.ObjectConfig{
			Name: "Subscription",
			Fields: graphql.Fields{
				"events": &graphql.Field{
					Type: eventType,
					Args: graphql.FieldConfigArgument{
						"count": &graphql.ArgumentConfig{Type: graphql.Int},
					},
					Subscribe: subscribe,
					Resolve: func(_ context.Context, p graphql.ResolveParams) (any, error) {
						return p.Source, nil
					},
				},
				"silent": &graphql.Field{Type: graphql.String},
			},
		}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return schema
}

func countingSubscribe(t *testing.T) graphql.FieldSubscribeFn {
	t.Helper()
	return func(ctx context.Context, p graphql.ResolveParams) (<-chan any, error) {
		count, _ := p.Args["count"].(int)
		ch := make(chan *subscriptionTestEvent)
		go func() {
			defer close(ch)
			for i := range count {
				select {
				case <-ctx.Done():
					return
				case ch <- &subscriptionTestEvent{Seq: i, Text: "event"}:
				}
			}
		}()
		return graphql.EventStream(ctx, ch), nil
	}
}

func collectResults(t *testing.T, out <-chan *graphql.Result) []*graphql.Result {
	t.Helper()
	var results []*graphql.Result
	timeout := time.After(5 * time.Second)
	for {
		select {
		case r, ok := <-out:
			if !ok {
				return results
			}
			results = append(results, r)
		case <-timeout:
			t.Fatalf("timed out waiting for the result stream to close; got %d results", len(results))
		}
	}
}

func TestSubscribe_ExecutesEachEvent(t *testing.T) {
	schema := subscriptionTestSchema(t, countingSubscribe(t))
	doc := testutil.TestParse(t, `subscription S($n: Int) { events(count: $n) { seq text } }`)

	var eventContexts int
	out, err := graphql.Subscribe(context.Background(), graphql.SubscribeParams{
		Schema: schema,
		AST:    doc,
		Args:   map[string]any{"n": 3},
		EventContext: func(ctx context.Context) (context.Context, context.CancelFunc) {
			eventContexts++
			return context.WithTimeout(ctx, time.Second)
		},
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	results := collectResults(t, out)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if len(r.Errors) != 0 {
			t.Fatalf("result %d has errors: %v", i, r.Errors)
		}
		want := map[string]any{"events": map[string]any{"seq": i, "text": "event"}}
		if !reflect.DeepEqual(want, r.Data) {
			t.Fatalf("result %d: %s", i, testutil.Diff(want, r.Data))
		}
	}
	if eventContexts != 3 {
		t.Fatalf("expected EventContext per event, got %d calls", eventContexts)
	}
}

func TestSubscribe_FieldErrorsStayPerEvent(t *testing.T) {
	schema := subscriptionTestSchema(t, countingSubscribe(t))
	doc := testutil.TestParse(t, `subscription { events(count: 2) { seq boom } }`)
	out, err := graphql.Subscribe(context.Background(), graphql.SubscribeParams{Schema: schema, AST: doc})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	results := collectResults(t, out)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for i, r := range results {
		if len(r.Errors) != 1 || r.Errors[0].Message != "boom" {
			t.Fatalf("result %d: expected the boom field error, got %v", i, r.Errors)
		}
		data := r.Data.(map[string]any)["events"].(map[string]any)
		if data["seq"] != i || data["boom"] != nil {
			t.Fatalf("result %d: unexpected data %v", i, data)
		}
	}
}

func TestSubscribe_CancelClosesStream(t *testing.T) {
	schema := subscriptionTestSchema(t, countingSubscribe(t))
	doc := testutil.TestParse(t, `subscription { events(count: 1000) { seq } }`)
	ctx, cancel := context.WithCancel(context.Background())
	out, err := graphql.Subscribe(ctx, graphql.SubscribeParams{Schema: schema, AST: doc})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if r := <-out; r == nil || len(r.Errors) != 0 {
		t.Fatalf("expected a first clean result, got %v", r)
	}
	cancel()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case _, ok := <-out:
			if !ok {
				return
			}
		case <-timeout:
			t.Fatal("result stream did not close after cancel")
		}
	}
}

func TestSubscribe_RequestErrors(t *testing.T) {
	schema := subscriptionTestSchema(t, countingSubscribe(t))
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"two root fields", `subscription { events { seq } silent }`, "Subscription operations must have exactly one root field"},
		{"no subscribe fn", `subscription { silent }`, `Subscription field "silent" does not provide an event stream`},
		{"not a subscription", `query { a }`, "Subscribe can only execute subscription operations"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := testutil.TestParse(t, c.query)
			out, err := graphql.Subscribe(context.Background(), graphql.SubscribeParams{Schema: schema, AST: doc})
			if err == nil {
				t.Fatalf("expected an error, got a stream %v", out)
			}
			if err.Error() != c.want {
				t.Fatalf("expected %q, got %q", c.want, err.Error())
			}
		})
	}
}

func TestSubscribe_DirectiveHandlerRunsOnce(t *testing.T) {
	schema := subscriptionTestSchema(t, countingSubscribe(t))
	doc := testutil.TestParse(t, `subscription { events(count: 2) { seq } }`)
	out, err := graphql.Subscribe(context.Background(), graphql.SubscribeParams{
		Schema: schema,
		AST:    doc,
		FieldDefinitionDirectiveHandler: func(context.Context, *ast.Directive, *graphql.FieldDefinition) error {
			return errors.New("blocked")
		},
	})
	if err != nil || out == nil {
		t.Fatalf("no directives on the field, expected a stream: %v", err)
	}
	collectResults(t, out)
}

func TestSubscribe_PanicInSubscribeIsAnError(t *testing.T) {
	schema := subscriptionTestSchema(t, func(context.Context, graphql.ResolveParams) (<-chan any, error) {
		panic("kaboom")
	})
	doc := testutil.TestParse(t, `subscription { events { seq } }`)
	if _, err := graphql.Subscribe(context.Background(), graphql.SubscribeParams{Schema: schema, AST: doc}); err == nil {
		t.Fatal("expected the panic to surface as an error")
	}
}
