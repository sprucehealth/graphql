package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/sprucehealth/graphql"
	"github.com/sprucehealth/graphql/testutil"
)

func main() {
	http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()["query"][0]
		result := graphql.Do(r.Context(), graphql.Params{
			Schema:        testutil.StarWarsSchema,
			RequestString: query,
		})
		_ = json.NewEncoder(w).Encode(result)
	})
	fmt.Println("Now server is running on port 8080")
	fmt.Println("Test with Get      : curl -g 'http://localhost:8080/graphql?query={hero{name}}'")
	_ = http.ListenAndServe(":8080", nil)
}
