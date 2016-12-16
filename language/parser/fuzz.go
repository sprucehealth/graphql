// +build gofuzz

package parser

import "github.com/sprucehealth/graphql/language/source"

func Fuzz(data []byte) int {
	_, err := Parse(ParseParams{
		Source: source.New("GraphQL", string(data)),
	})
	if err != nil {
		return 0
	}
	return 1
}
