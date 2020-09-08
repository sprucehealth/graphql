package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"context"

	"github.com/sprucehealth/graphql"
)

var Schema graphql.Schema

var userType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
		},
	},
)

var queryType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"me": &graphql.Field{
				Type: userType,
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (interface{}, error) {
					return ctx.Value("currentUser"), nil
				},
			},
		},
	})

func graphqlHandler(w http.ResponseWriter, r *http.Request) {
	user := struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{1, "cool user"}
	result := graphql.Do(context.WithValue(r.Context(), "currentUser", user), graphql.Params{
		Schema:        Schema,
		RequestString: r.URL.Query()["query"][0],
	})
	if len(result.Errors) > 0 {
		log.Printf("wrong result, unexpected errors: %v", result.Errors)
		return
	}
	json.NewEncoder(w).Encode(result)
}

func main() {
	http.HandleFunc("/graphql", graphqlHandler)
	fmt.Println("Now server is running on port 8080")
	fmt.Println("Test with Get      : curl -g 'http://localhost:8080/graphql?query={me{id,name}}'")
	http.ListenAndServe(":8080", nil)
}

func init() {
	s, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: queryType,
	})
	if err != nil {
		log.Fatalf("failed to create schema, error: %v", err)
	}
	Schema = s
}
