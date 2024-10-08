package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/sprucehealth/graphql"
)

/*****************************************************************************/
/* Shared data variables to allow dynamic reloads
/*****************************************************************************/

var schema graphql.Schema

const jsonDataFile = "data.json"

func handleSIGUSR1(c chan os.Signal) {
	for {
		<-c
		fmt.Printf("Caught SIGUSR1. Reloading %s\n", jsonDataFile)
		err := importJSONDataFromFile(jsonDataFile)
		if err != nil {
			fmt.Printf("Error: %s\n", err.Error())
			return
		}
	}
}

func filterUser(data []map[string]any, args map[string]any) map[string]any {
	for _, user := range data {
		for k, v := range args {
			if user[k] != v {
				goto nextuser
			}
			return user
		}

	nextuser:
	}
	return nil
}

func executeQuery(ctx context.Context, query string, schema graphql.Schema) *graphql.Result {
	result := graphql.Do(ctx, graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) > 0 {
		fmt.Printf("wrong result, unexpected errors: %v\n", result.Errors)
	}
	return result
}

func importJSONDataFromFile(fileName string) error {
	content, err := os.ReadFile(fileName)
	if err != nil {
		return err
	}

	var data []map[string]any

	err = json.Unmarshal(content, &data)
	if err != nil {
		return err
	}

	fields := make(graphql.Fields)
	args := make(graphql.FieldConfigArgument)
	for _, item := range data {
		for k := range item {
			fields[k] = &graphql.Field{
				Type: graphql.String,
			}
			args[k] = &graphql.ArgumentConfig{
				Type: graphql.String,
			}
		}
	}

	var userType = graphql.NewObject(
		graphql.ObjectConfig{
			Name:   "User",
			Fields: fields,
		},
	)

	var queryType = graphql.NewObject(
		graphql.ObjectConfig{
			Name: "Query",
			Fields: graphql.Fields{
				"user": &graphql.Field{
					Type: userType,
					Args: args,
					Resolve: func(ctx context.Context, p graphql.ResolveParams) (any, error) {
						return filterUser(data, p.Args), nil
					},
				},
			},
		})

	schema, _ = graphql.NewSchema(
		graphql.SchemaConfig{
			Query: queryType,
		},
	)

	return nil
}

func main() {
	// Catch SIGUSR1 and reload the data file
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGUSR1)
	go handleSIGUSR1(c)

	err := importJSONDataFromFile(jsonDataFile)
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}

	http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		result := executeQuery(r.Context(), r.URL.Query()["query"][0], schema)
		_ = json.NewEncoder(w).Encode(result)
	})

	fmt.Println("Now server is running on port 8080")
	fmt.Println("Test with Get      : curl -g 'http://localhost:8080/graphql?query={user(name:\"Dan\"){id,surname}}'")
	fmt.Printf("Reload json file   : kill -SIGUSR1 %s\n", strconv.Itoa(os.Getpid()))
	_ = http.ListenAndServe(":8080", nil)
}
