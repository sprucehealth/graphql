package lexer

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/sprucehealth/graphql/language/source"
)

type Test struct {
	Body     string
	Expected interface{}
}

func createSource(body string) *source.Source {
	return source.New("GraphQL", body)
}

func TestLexer_GetTokenDesc(t *testing.T) {
	expected := `Name "foo"`
	tokenDescription := GetTokenDesc(Token{
		Kind:  NAME,
		Start: 2,
		End:   5,
		Value: "foo",
	})
	if expected != tokenDescription {
		t.Errorf("Expected %v, got %v", expected, tokenDescription)
	}

	expected = `Name`
	tokenDescription = GetTokenDesc(Token{
		Kind:  NAME,
		Start: 0,
		End:   0,
		Value: "",
	})
	if expected != tokenDescription {
		t.Errorf("Expected %v, got %v", expected, tokenDescription)
	}

	expected = `String "foo"`
	tokenDescription = GetTokenDesc(Token{
		Kind:  STRING,
		Start: 2,
		End:   5,
		Value: "foo",
	})
	if expected != tokenDescription {
		t.Errorf("Expected %v, got %v", expected, tokenDescription)
	}

	expected = `String`
	tokenDescription = GetTokenDesc(Token{
		Kind:  STRING,
		Start: 0,
		End:   0,
		Value: "",
	})
	if expected != tokenDescription {
		t.Errorf("Expected %v, got %v", expected, tokenDescription)
	}

}

func TestLexer_DisallowsUncommonControlCharacters(t *testing.T) {
	tests := []Test{
		Test{
			Body: "\u0007",
			Expected: `Syntax Error GraphQL (1:1) Invalid character "\\u0007"

1: \u0007
   ^
`,
		},
	}
	for _, test := range tests {
		_, err := New(source.New("GraphQL", test.Body)).NextToken()
		if err == nil {
			t.Errorf("unexpected nil error\nexpected:\n%v\n\ngot:\n%v", test.Expected, err)
		}
		if err.Error() != test.Expected {
			t.Errorf("unexpected error.\nexpected:\n%v\n\ngot:\n%v", test.Expected, err.Error())
		}
	}
}

func TestLexer_AcceptsBOMHeader(t *testing.T) {
	tests := []Test{
		Test{
			Body: "\uFEFF foo",
			Expected: Token{
				Kind:  NAME,
				Start: 2,
				End:   5,
				Value: "foo",
			},
		},
	}
	for _, test := range tests {
		token, err := New(source.New("GraphQL", test.Body)).NextToken()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(token, test.Expected) {
			t.Errorf("unexpected token, expected: %+v, got: %+v", test.Expected, token)
		}
	}
}

func TestLexer_SkipsWhiteSpace(t *testing.T) {
	tests := []Test{
		{
			Body: `

    foo

`,
			Expected: []Token{{
				Kind:  NAME,
				Start: 6,
				End:   9,
				Value: "foo",
			}},
		},
		{
			Body: `
    #comment1
    foo#comment2
`,
			Expected: []Token{
				{
					Kind:  COMMENT,
					Start: 5,
					End:   14,
					Value: "#comment1",
				},
				{
					Kind:  NAME,
					Start: 19,
					End:   22,
					Value: "foo",
				},
				{
					Kind:  COMMENT,
					Start: 22,
					End:   31,
					Value: "#comment2",
				},
			},
		},
		{
			Body: `,,,foo,,,`,
			Expected: []Token{{
				Kind:  NAME,
				Start: 3,
				End:   6,
				Value: "foo",
			}},
		},
		{
			Body:     ``,
			Expected: ([]Token)(nil),
		},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			lex := New(source.New("", test.Body))
			var tokens []Token
			for {
				tok, err := lex.NextToken()
				if err != nil {
					t.Fatal(err)
				}
				if tok.Kind == EOF {
					break
				}
				tokens = append(tokens, tok)
			}
			if !reflect.DeepEqual(tokens, test.Expected) {
				t.Fatalf("unexpected token, expected: %+v, got: %+v, body: %s", test.Expected, tokens, test.Body)
			}
		})
	}
}

func TestLexer_ErrorsRespectWhitespace(t *testing.T) {
	body := `

    ?

`
	_, err := New(createSource(body)).NextToken()
	expected := "Syntax Error GraphQL (3:5) Unexpected character \"?\".\n\n2: \n3:     ?\n       ^\n4: \n"
	if err == nil {
		t.Fatalf("unexpected nil error\nexpected:\n%v\n\ngot:\n%v", expected, err)
	}
	if err.Error() != expected {
		t.Fatalf("unexpected error.\nexpected:\n%v\n\ngot:\n%v", expected, err.Error())
	}
}

func TestLexer_LexesNames(t *testing.T) {
	tests := []Test{
		{
			Body: "simple",
			Expected: Token{
				Kind:  NAME,
				Start: 0,
				End:   6,
				Value: "simple",
			},
		},
		{
			Body: "Capital",
			Expected: Token{
				Kind:  NAME,
				Start: 0,
				End:   7,
				Value: "Capital",
			},
		},
	}
	for _, test := range tests {
		token, err := New(createSource(test.Body)).NextToken()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(token, test.Expected) {
			t.Errorf("unexpected token, expected: %v, got: %v", test.Expected, token)
		}
	}
}

func TestLexer_LexesStrings(t *testing.T) {
	tests := []Test{
		{
			Body: "\"simple\"",
			Expected: Token{
				Kind:  STRING,
				Start: 0,
				End:   8,
				Value: "simple",
			},
		},
		{
			Body: "\" white space \"",
			Expected: Token{
				Kind:  STRING,
				Start: 0,
				End:   15,
				Value: " white space ",
			},
		},
		{
			Body: "\"quote \\\"\"",
			Expected: Token{
				Kind:  STRING,
				Start: 0,
				End:   10,
				Value: `quote "`,
			},
		},
		{
			Body: "\"escaped \\n\\r\\b\\t\\f\"",
			Expected: Token{
				Kind:  STRING,
				Start: 0,
				End:   20,
				Value: "escaped \n\r\b\t\f",
			},
		},
		{
			Body: `"slashes \\ \/"`,
			Expected: Token{
				Kind:  STRING,
				Start: 0,
				End:   15,
				Value: `slashes \ /`,
			},
		},
		{
			Body: "\"unicode \\u1234\\u5678\\u90AB\\uCDEF\"",
			Expected: Token{
				Kind:  STRING,
				Start: 0,
				End:   34,
				Value: "unicode \u1234\u5678\u90AB\uCDEF",
			},
		},
		{
			Body: "\"unicode фы世界\"",
			Expected: Token{
				Kind:  STRING,
				Start: 0,
				End:   14,
				Value: "unicode фы世界",
			},
		},
		{
			Body: "\"фы世界\"",
			Expected: Token{
				Kind:  STRING,
				Start: 0,
				End:   6,
				Value: "фы世界",
			},
		},
		{
			Body: "\"Has a фы世界 multi-byte character.\"",
			Expected: Token{
				Kind:  STRING,
				Start: 0,
				End:   34,
				Value: "Has a фы世界 multi-byte character.",
			},
		},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			token, err := New(source.New("", test.Body)).NextToken()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(token, test.Expected) {
				t.Fatalf("unexpected token, expected: %v, got: %v", test.Expected, token)
			}
		})
	}
}

func TestLexer_ReportsUsefulStringErrors(t *testing.T) {
	tests := []Test{
		{
			Body: "\"no end quote",
			Expected: `Syntax Error GraphQL (1:14) Unterminated string.

1: "no end quote
                ^
`,
		},
		{
			Body: "\"multi\nline\"",
			Expected: `Syntax Error GraphQL (1:7) Unterminated string.

1: "multi
         ^
2: line"
`,
		},
		{
			Body: "\"multi\rline\"",
			Expected: `Syntax Error GraphQL (1:7) Unterminated string.

1: "multi
         ^
2: line"
`,
		},
		{
			Body: "\"bad \\z esc\"",
			Expected: `Syntax Error GraphQL (1:7) Invalid character escape sequence: \z.

1: "bad \z esc"
         ^
`,
		},
		{
			Body: "\"bad \\x esc\"",
			Expected: `Syntax Error GraphQL (1:7) Invalid character escape sequence: \x.

1: "bad \x esc"
         ^
`,
		},
		{
			Body: "\"bad \\u1 esc\"",
			Expected: `Syntax Error GraphQL (1:7) Invalid character escape sequence: \u1 es

1: "bad \u1 esc"
         ^
`,
		},
		{
			Body: "\"bad \\u0XX1 esc\"",
			Expected: `Syntax Error GraphQL (1:7) Invalid character escape sequence: \u0XX1

1: "bad \u0XX1 esc"
         ^
`,
		},
		{
			Body: "\"bad \\uXXXX esc\"",
			Expected: `Syntax Error GraphQL (1:7) Invalid character escape sequence: \uXXXX

1: "bad \uXXXX esc"
         ^
`,
		},
		{
			Body: "\"bad \\uFXXX esc\"",
			Expected: `Syntax Error GraphQL (1:7) Invalid character escape sequence: \uFXXX

1: "bad \uFXXX esc"
         ^
`,
		},
		{
			Body: "\"bad \\uXXXF esc\"",
			Expected: `Syntax Error GraphQL (1:7) Invalid character escape sequence: \uXXXF

1: "bad \uXXXF esc"
         ^
`,
		},
		{
			Body: "\"bad \\u123",
			Expected: `Syntax Error GraphQL (1:7) Invalid character escape sequence: \u123

1: "bad \u123
         ^
`,
		},
		{
			// some unicode chars take more than one column of text
			// current implementation does not handle this
			Body: "\"bфы世ыы𠱸d \\uXXXF esc\"",
			Expected: `Syntax Error GraphQL (1:12) Invalid character escape sequence: \uXXXF

1: "bфы世ыы𠱸d \uXXXF esc"
              ^
`,
		},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			tok, err := New(createSource(test.Body)).NextToken()
			if err == nil {
				t.Fatalf("unexpected nil error\nexpected error: %v\ngot token: %#+v", test.Expected, tok)
			}
			if err.Error() != test.Expected {
				t.Fatalf("unexpected error.\nexpected:\n%v\n\ngot:\n%v", test.Expected, err.Error())
			}
		})
	}
}

func TestLexer_LexesNumbers(t *testing.T) {
	tests := []Test{
		{
			Body: "4",
			Expected: Token{
				Kind:  INT,
				Start: 0,
				End:   1,
				Value: "4",
			},
		},
		{
			Body: "4.123",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   5,
				Value: "4.123",
			},
		},
		{
			Body: "-4",
			Expected: Token{
				Kind:  INT,
				Start: 0,
				End:   2,
				Value: "-4",
			},
		},
		{
			Body: "9",
			Expected: Token{
				Kind:  INT,
				Start: 0,
				End:   1,
				Value: "9",
			},
		},
		{
			Body: "0",
			Expected: Token{
				Kind:  INT,
				Start: 0,
				End:   1,
				Value: "0",
			},
		},
		{
			Body: "-4.123",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   6,
				Value: "-4.123",
			},
		},
		{
			Body: "0.123",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   5,
				Value: "0.123",
			},
		},
		{
			Body: "123e4",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   5,
				Value: "123e4",
			},
		},
		{
			Body: "123E4",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   5,
				Value: "123E4",
			},
		},
		{
			Body: "123e-4",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   6,
				Value: "123e-4",
			},
		},
		{
			Body: "123e+4",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   6,
				Value: "123e+4",
			},
		},
		{
			Body: "-1.123e4",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   8,
				Value: "-1.123e4",
			},
		},
		{
			Body: "-1.123E4",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   8,
				Value: "-1.123E4",
			},
		},
		{
			Body: "-1.123e-4",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   9,
				Value: "-1.123e-4",
			},
		},
		{
			Body: "-1.123e+4",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   9,
				Value: "-1.123e+4",
			},
		},
		{
			Body: "-1.123e4567",
			Expected: Token{
				Kind:  FLOAT,
				Start: 0,
				End:   11,
				Value: "-1.123e4567",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.Body, func(t *testing.T) {
			token, err := New(createSource(test.Body)).NextToken()
			if err != nil {
				t.Fatalf("unexpected error: %v, test: %s", err, test)
			}
			if !reflect.DeepEqual(token, test.Expected) {
				t.Fatalf("unexpected token, expected: %+v, got: %+v, test: %+v", test.Expected, token, test)
			}
		})
	}
}

func TestLexer_ReportsUsefulNumberErrors(t *testing.T) {
	tests := []Test{
		{
			Body: "00",
			Expected: `Syntax Error GraphQL (1:2) Invalid number, unexpected digit after 0: "0".

1: 00
    ^
`,
		},
		{
			Body: "+1",
			Expected: `Syntax Error GraphQL (1:1) Unexpected character "+".

1: +1
   ^
`,
		},
		{
			Body: "1.",
			Expected: `Syntax Error GraphQL (1:3) Invalid number, expected digit but got: EOF.

1: 1.
     ^
`,
		},
		{
			Body: ".123",
			Expected: `Syntax Error GraphQL (1:1) Unexpected character ".".

1: .123
   ^
`,
		},
		{
			Body: "1.A",
			Expected: `Syntax Error GraphQL (1:3) Invalid number, expected digit but got: "A".

1: 1.A
     ^
`,
		},
		{
			Body: "-A",
			Expected: `Syntax Error GraphQL (1:2) Invalid number, expected digit but got: "A".

1: -A
    ^
`,
		},
		{
			Body: "1.0e",
			Expected: `Syntax Error GraphQL (1:5) Invalid number, expected digit but got: EOF.

1: 1.0e
       ^
`,
		},
		{
			Body: "1.0eA",
			Expected: `Syntax Error GraphQL (1:5) Invalid number, expected digit but got: "A".

1: 1.0eA
       ^
`,
		},
	}
	for _, test := range tests {
		_, err := New(createSource(test.Body)).NextToken()
		if err == nil {
			t.Errorf("unexpected nil error\nexpected:\n%v\n\ngot:\n%v", test.Expected, err)
		}
		if err.Error() != test.Expected {
			t.Errorf("unexpected error.\nexpected:\n%v\n\ngot:\n%v", test.Expected, err.Error())
		}
	}
}

func TestLexer_LexesPunctuation(t *testing.T) {
	tests := []Test{
		{
			Body: "!",
			Expected: Token{
				Kind:  BANG,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: "$",
			Expected: Token{
				Kind:  DOLLAR,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: "(",
			Expected: Token{
				Kind:  PAREN_L,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: ")",
			Expected: Token{
				Kind:  PAREN_R,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: "...",
			Expected: Token{
				Kind:  SPREAD,
				Start: 0,
				End:   3,
				Value: "",
			},
		},
		{
			Body: ":",
			Expected: Token{
				Kind:  COLON,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: "=",
			Expected: Token{
				Kind:  EQUALS,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: "@",
			Expected: Token{
				Kind:  AT,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: "[",
			Expected: Token{
				Kind:  BRACKET_L,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: "]",
			Expected: Token{
				Kind:  BRACKET_R,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: "{",
			Expected: Token{
				Kind:  BRACE_L,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: "|",
			Expected: Token{
				Kind:  PIPE,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
		{
			Body: "}",
			Expected: Token{
				Kind:  BRACE_R,
				Start: 0,
				End:   1,
				Value: "",
			},
		},
	}
	for _, test := range tests {
		token, err := New(createSource(test.Body)).NextToken()
		if err != nil {
			t.Errorf("unexpected error :%v, test: %v", err, test)
		}
		if !reflect.DeepEqual(token, test.Expected) {
			t.Errorf("unexpected token, expected: %v, got: %v, test: %v", test.Expected, token, test)
		}
	}
}

func TestLexer_ReportsUsefulUnknownCharacterError(t *testing.T) {
	tests := []Test{
		{
			Body: "..",
			Expected: `Syntax Error GraphQL (1:1) Unexpected character ".".

1: ..
   ^
`,
		},
		{
			Body: "?",
			Expected: `Syntax Error GraphQL (1:1) Unexpected character "?".

1: ?
   ^
`,
		},
		{
			Body: "\u203B",
			Expected: `Syntax Error GraphQL (1:1) Unexpected character "\\u203B".

1: ※
   ^
`,
		},
		{
			Body: "\u203b",
			Expected: `Syntax Error GraphQL (1:1) Unexpected character "\\u203B".

1: ※
   ^
`,
		},
		{
			Body: "ф",
			Expected: `Syntax Error GraphQL (1:1) Unexpected character "\\u0444".

1: ф
   ^
`,
		},
	}
	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			tok, err := New(createSource(test.Body)).NextToken()
			if err == nil {
				t.Fatalf("unexpected nil error\nexpected error: %v\ngot token: %#+v", test.Expected, tok)
			}
			if err.Error() != test.Expected {
				t.Fatalf("unexpected error.\nexpected:\n%v\n\ngot:\n%v", test.Expected, err.Error())
			}
		})
	}
}

func TestLexer_ReportsUsefulInformationForDashesInNames(t *testing.T) {
	q := "a-b"
	lexer := New(createSource(q))
	firstToken, err := lexer.NextToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	firstTokenExpected := Token{
		Kind:  NAME,
		Start: 0,
		End:   1,
		Value: "a",
	}
	if !reflect.DeepEqual(firstToken, firstTokenExpected) {
		t.Fatalf("unexpected token, expected: %v, got: %v", firstTokenExpected, firstToken)
	}
	errExpected := `Syntax Error GraphQL (1:3) Invalid number, expected digit but got: "b".

1: a-b
     ^
`
	token, err := lexer.NextToken()
	if err == nil {
		t.Fatalf("unexpected nil error: %v", err)
	}
	if err.Error() != errExpected {
		t.Fatalf("unexpected error, token:%v\nexpected:\n%v\n\ngot:\n%v", token, errExpected, err.Error())
	}
}

func TestFullDocument(t *testing.T) {
	body := `
		# Comment
		type SomeType {
			# more comments
			# more more more
			field: [Int]!
			foo(a Int = 123): String
		}
		fragment basicType on __Type {
			kind
			name
			description
			ofType {
				kind
				name
				description
			}
		}
		query _ {
			this(some: "foo\u0034bar", thing: 1.123) {
				abc(foo: "bar bar bar bar \t woo woo \n \n wha wha") {
					xyz
					... on Foo {
						id
					}
				}
			}
		}
	`
	tokens := []Token{
		{Kind: COMMENT, Start: 3, End: 12, Value: "# Comment"},
		{Kind: NAME, Start: 15, End: 19, Value: "type"},
		{Kind: NAME, Start: 20, End: 28, Value: "SomeType"},
		{Kind: BRACE_L, Start: 29, End: 30, Value: ""},
		{Kind: COMMENT, Start: 34, End: 49, Value: "# more comments"},
		{Kind: COMMENT, Start: 53, End: 69, Value: "# more more more"},
		{Kind: NAME, Start: 73, End: 78, Value: "field"},
		{Kind: COLON, Start: 78, End: 79, Value: ""},
		{Kind: BRACKET_L, Start: 80, End: 81, Value: ""},
		{Kind: NAME, Start: 81, End: 84, Value: "Int"},
		{Kind: BRACKET_R, Start: 84, End: 85, Value: ""},
		{Kind: BANG, Start: 85, End: 86, Value: ""},
		{Kind: NAME, Start: 90, End: 93, Value: "foo"},
		{Kind: PAREN_L, Start: 93, End: 94, Value: ""},
		{Kind: NAME, Start: 94, End: 95, Value: "a"},
		{Kind: NAME, Start: 96, End: 99, Value: "Int"},
		{Kind: EQUALS, Start: 100, End: 101, Value: ""},
		{Kind: INT, Start: 102, End: 105, Value: "123"},
		{Kind: PAREN_R, Start: 105, End: 106, Value: ""},
		{Kind: COLON, Start: 106, End: 107, Value: ""},
		{Kind: NAME, Start: 108, End: 114, Value: "String"},
		{Kind: BRACE_R, Start: 117, End: 118, Value: ""},
		{Kind: NAME, Start: 121, End: 129, Value: "fragment"},
		{Kind: NAME, Start: 130, End: 139, Value: "basicType"},
		{Kind: NAME, Start: 140, End: 142, Value: "on"},
		{Kind: NAME, Start: 143, End: 149, Value: "__Type"},
		{Kind: BRACE_L, Start: 150, End: 151, Value: ""},
		{Kind: NAME, Start: 155, End: 159, Value: "kind"},
		{Kind: NAME, Start: 163, End: 167, Value: "name"},
		{Kind: NAME, Start: 171, End: 182, Value: "description"},
		{Kind: NAME, Start: 186, End: 192, Value: "ofType"},
		{Kind: BRACE_L, Start: 193, End: 194, Value: ""},
		{Kind: NAME, Start: 199, End: 203, Value: "kind"},
		{Kind: NAME, Start: 208, End: 212, Value: "name"},
		{Kind: NAME, Start: 217, End: 228, Value: "description"},
		{Kind: BRACE_R, Start: 232, End: 233, Value: ""},
		{Kind: BRACE_R, Start: 236, End: 237, Value: ""},
		{Kind: NAME, Start: 240, End: 245, Value: "query"},
		{Kind: NAME, Start: 246, End: 247, Value: "_"},
		{Kind: BRACE_L, Start: 248, End: 249, Value: ""},
		{Kind: NAME, Start: 253, End: 257, Value: "this"},
		{Kind: PAREN_L, Start: 257, End: 258, Value: ""},
		{Kind: NAME, Start: 258, End: 262, Value: "some"},
		{Kind: COLON, Start: 262, End: 263, Value: ""},
		{Kind: STRING, Start: 264, End: 278, Value: "foo\u0034bar"},
		{Kind: NAME, Start: 280, End: 285, Value: "thing"},
		{Kind: COLON, Start: 285, End: 286, Value: ""},
		{Kind: FLOAT, Start: 287, End: 292, Value: "1.123"},
		{Kind: PAREN_R, Start: 292, End: 293, Value: ""},
		{Kind: BRACE_L, Start: 294, End: 295, Value: ""},
		{Kind: NAME, Start: 300, End: 303, Value: "abc"},
		{Kind: PAREN_L, Start: 303, End: 304, Value: ""},
		{Kind: NAME, Start: 304, End: 307, Value: "foo"},
		{Kind: COLON, Start: 307, End: 308, Value: ""},
		{Kind: STRING, Start: 309, End: 351, Value: "bar bar bar bar \t woo woo \n \n wha wha"},
		{Kind: PAREN_R, Start: 351, End: 352, Value: ""},
		{Kind: BRACE_L, Start: 353, End: 354, Value: ""},
		{Kind: NAME, Start: 360, End: 363, Value: "xyz"},
		{Kind: SPREAD, Start: 369, End: 372, Value: ""},
		{Kind: NAME, Start: 373, End: 375, Value: "on"},
		{Kind: NAME, Start: 376, End: 379, Value: "Foo"},
		{Kind: BRACE_L, Start: 380, End: 381, Value: ""},
		{Kind: NAME, Start: 388, End: 390, Value: "id"},
		{Kind: BRACE_R, Start: 396, End: 397, Value: ""},
		{Kind: BRACE_R, Start: 402, End: 403, Value: ""},
		{Kind: BRACE_R, Start: 407, End: 408, Value: ""},
		{Kind: BRACE_R, Start: 411, End: 412, Value: ""},
	}
	lex := New(createSource(body))
	ix := 0
	for {
		tok, err := lex.NextToken()
		if err != nil {
			t.Fatal(err)
		}
		if tok.Kind == EOF {
			break
		}
		if ix >= len(tokens) {
			t.Errorf("Received unexpected token %#+v", tok)
		} else if tok != tokens[ix] {
			t.Fatalf("Expected %+v got %+v at index %d", tokens[ix], tok, ix)
		}
		ix++
	}
}

func BenchmarkLexer(b *testing.B) {
	body := `
		# Comment
		type SomeType {
			# more comments
			# more more more
			field: [Int]!
			foo(a Int = 123): String
		}
		fragment basicType on __Type {
			kind
			name
			description
			ofType {
				kind
				name
				description
			}
		}
		query _ {
			this(some: "foo\u0034bar", thing: 1.123) {
				abc(foo: "bar bar bar bar \t woo woo \n \n wha wha") {
					xyz
				}
			}
		}
	`
	source := createSource(body)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lex := New(source)
		for {
			tok, err := lex.NextToken()
			if err != nil {
				b.Fatal(err)
			}
			if tok.Kind == EOF {
				break
			}
		}
	}
}

func BenchmarkLexerAndSourceCreation(b *testing.B) {
	body := `
		# Comment
		type SomeType {
			# more comments
			# more more more
			field: [Int]!
			foo(a Int = 123): String
		}
		fragment basicType on __Type {
			kind
			name
			description
			ofType {
				kind
				name
				description
			}
		}
		query _ {
			this(some: "foo\u0034bar", thing: 1.123) {
				abc(foo: "bar bar bar bar \t woo woo \n \n wha wha") {
					xyz
				}
			}
		}
	`
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lex := New(createSource(body))
		for {
			tok, err := lex.NextToken()
			if err != nil {
				b.Fatal(err)
			}
			if tok.Kind == EOF {
				break
			}
		}
	}
}
