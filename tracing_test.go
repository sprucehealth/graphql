package graphql_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/sprucehealth/graphql"
)

func TestTracing(t *testing.T) {
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name: "RootQueryType",
			Fields: graphql.Fields{
				"object": &graphql.Field{
					Type: graphql.NewNonNull(graphql.NewObject(graphql.ObjectConfig{
						Name: "First",
						Fields: graphql.Fields{
							"first": &graphql.Field{
								Type: graphql.NewNonNull(graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
									Name: "Seconds",
									Fields: graphql.Fields{
										"second": &graphql.Field{
											Type: graphql.Boolean,
											Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
												return true, nil
											},
										},
									},
								}))),
								Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
									return []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, nil
								},
							},
						},
					})),
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return true, nil
					},
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("wrong result, unexpected errors: %v", err.Error())
	}
	query := "{ object { first { second }} }"

	for range 10 {
		tr := graphql.NewCountingTracer(true)
		result := graphql.Do(context.Background(), graphql.Params{
			Schema:        schema,
			RequestString: query,
			Tracer:        tr,
		})
		if len(result.Errors) > 0 {
			t.Fatalf("wrong result, unexpected errors: %v", result.Errors)
		}
		var traces []*graphql.TracePathCount
		for _, tr := range tr.IterTraces() {
			t.Logf("%s %d executions, %s total duration, %s max duration, %s average duration\n",
				strings.Join(tr.Path, "."), tr.Count, tr.TotalDuration, tr.MaxDuration, tr.TotalDuration/time.Duration(tr.Count))
			traces = append(traces, tr)
		}
		if len(traces) != 3 {
			t.Logf("Expected 3 traces, got %d", len(traces))
		}
		// Assume the order of execution which should always be consistent unless the executor changes.
		for i, expCount := range []int{1, 1, 10} {
			trace := traces[i]
			if trace.Count != expCount {
				t.Logf("Expected count of %d for %v, got %d", expCount, trace.Path, trace.Count)
			}
		}
		tr.Recycle()
	}
}
