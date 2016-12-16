// +build gofuzz

package lexer

import "github.com/sprucehealth/graphql/language/source"

func Fuzz(data []byte) int {
	lex := New(source.New("", string(data)))
	for {
		tok, err := lex.NextToken()
		if err != nil {
			return 0
		}
		if tok.Kind == EOF {
			break
		}
	}
	return 1
}
