package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/sprucehealth/graphql"
)

type user struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

var data map[string]user

/*
Create User object type with fields "id" and "name" by using GraphQLObjectTypeConfig:
  - Name: name of object type
  - Fields: a map of fields by using GraphQLFields

Setup type of field use GraphQLFieldConfig
*/
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

/*
Create Query object type with fields "user" has type [userType] by using GraphQLObjectTypeConfig:
  - Name: name of object type
  - Fields: a map of fields by using GraphQLFields

Setup type of field use GraphQLFieldConfig to define:
  - Type: type of field
  - Args: arguments to query with current field
  - Resolve: function to query data using params from [Args] and return value with current type
*/
var queryType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"user": &graphql.Field{
				Type: userType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
					idQuery, isOK := p.Args["id"].(string)
					if isOK {
						return data[idQuery], nil
					}
					return nil, nil
				},
			},
		},
	})

var schema, _ = graphql.NewSchema(
	graphql.SchemaConfig{
		Query: queryType,
	},
)

func executeQuery(ctx context.Context, query string, schema graphql.Schema) *graphql.Result {
	result := graphql.Do(ctx, graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) > 0 {
		fmt.Printf("wrong result, unexpected errors: %v", result.Errors)
	}
	return result
}

func main() {
	_ = importJSONDataFromFile("data.json", &data)

	http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		result := executeQuery(r.Context(), r.URL.Query()["query"][0], schema)
		_ = json.NewEncoder(w).Encode(result)
	})

	fmt.Println("Now server is running on port 8080")
	fmt.Println("Test with Get      : curl -g 'http://localhost:8080/graphql?query={user(id:\"1\"){name}}'")
	_ = http.ListenAndServe(":8080", nil)
}

// Helper function to import json from file to map
func importJSONDataFromFile(fileName string, result any) (isOK bool) {
	isOK = true
	content, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Print("Error:", err)
		isOK = false
	}
	err = json.Unmarshal(content, result)
	if err != nil {
		isOK = false
		fmt.Print("Error:", err)
	}
	return
}
