package lexer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sprucehealth/graphql/gqlerrors"
	"github.com/sprucehealth/graphql/language/source"
)

const (
	EOF = iota + 1
	BANG
	DOLLAR
	PAREN_L
	PAREN_R
	SPREAD
	COLON
	EQUALS
	AT
	BRACKET_L
	BRACKET_R
	BRACE_L
	PIPE
	BRACE_R
	NAME
	INT
	FLOAT
	STRING
	COMMENT
)

var tokenDescription map[int]string

func init() {
	tokenDescription = make(map[int]string)
	tokenDescription[EOF] = "EOF"
	tokenDescription[BANG] = "!"
	tokenDescription[DOLLAR] = "$"
	tokenDescription[PAREN_L] = "("
	tokenDescription[PAREN_R] = ")"
	tokenDescription[SPREAD] = "..."
	tokenDescription[COLON] = ":"
	tokenDescription[EQUALS] = "="
	tokenDescription[AT] = "@"
	tokenDescription[BRACKET_L] = "["
	tokenDescription[BRACKET_R] = "]"
	tokenDescription[BRACE_L] = "{"
	tokenDescription[PIPE] = "|"
	tokenDescription[BRACE_R] = "}"
	tokenDescription[NAME] = "Name"
	tokenDescription[INT] = "Int"
	tokenDescription[FLOAT] = "Float"
	tokenDescription[STRING] = "String"
	tokenDescription[COMMENT] = "Comment"
}

// Token is a representation of a lexed Token. Value only appears for non-punctuation
// tokens: NAME, INT, FLOAT, and STRING.
type Token struct {
	Kind  int
	Start int
	End   int
	Value string
}

func (t *Token) String() string {
	return tokenDescription[t.Kind]
}

type Lexer struct {
	src      *source.Source
	body     string
	offset   offset
	rdOffset offset
	runePos  int
	ch       rune
}

type offset struct {
	bytes int
	runes int
}

func New(s *source.Source) *Lexer {
	lex := &Lexer{
		src:  s,
		body: s.Body(),
	}
	lex.nextRune()
	return lex
}

func (l *Lexer) NextToken() (Token, error) {
	token, err := l.readToken()
	if err != nil {
		return token, err
	}
	return token, nil
}

func (l *Lexer) nextRune() {
	l.offset = l.rdOffset
	if l.rdOffset.bytes >= len(l.body) {
		l.ch = 0
		return
	}
	l.offset = l.rdOffset
	r, w := rune(l.body[l.offset.bytes]), 1
	// case r == 0:
	// 	s.error(s.offset, "illegal character NUL")
	if r >= utf8.RuneSelf {
		r, w = utf8.DecodeRuneInString(l.body[l.offset.bytes:])
		// if r == utf8.RuneError && w == 1 {
		// 	s.error(s.offset, "illegal UTF-8 encoding")
		// } else if r == bom && s.offset > 0 {
		// 	s.error(s.offset, "illegal byte order mark")
		// }
	}
	l.ch = r
	l.rdOffset.bytes += w
	l.rdOffset.runes++
}

// readName reads an alphanumeric + underscore name from the source.
// [_A-Za-z][_0-9A-Za-z]*
func (l *Lexer) readName() (Token, error) {
	start := l.offset
	for {
		if !(l.ch != 0 && (l.ch == 95 || // _
			l.ch >= 48 && l.ch <= 57 || // 0-9
			l.ch >= 65 && l.ch <= 90 || // A-Z
			l.ch >= 97 && l.ch <= 122)) { // a-z
			break
		}
		l.nextRune()
	}
	return makeToken(NAME, start, l.offset, l.sliceBody(start, l.offset)), nil
}

// readNumber reads a number token from the source file, either a float
// or an int depending on whether a decimal point appears.
// Int:   -?(0|[1-9][0-9]*)
// Float: -?(0|[1-9][0-9]*)(\.[0-9]+)?((E|e)(+|-)?[0-9]+)?
func (l *Lexer) readNumber() (Token, error) {
	start := l.offset
	isFloat := false
	if l.ch == '-' {
		l.nextRune()
	}
	if l.ch == '0' {
		l.nextRune()
		if l.ch >= '0' && l.ch <= '9' {
			description := fmt.Sprintf("Invalid number, unexpected digit after 0: %v.", printCharCode(l.ch))
			return Token{}, gqlerrors.NewSyntaxError(l.src, l.offset.runes, description)
		}
	} else {
		err := l.readDigits()
		if err != nil {
			return Token{}, err
		}
	}
	if l.ch == '.' {
		isFloat = true
		l.nextRune()
		err := l.readDigits()
		if err != nil {
			return Token{}, err
		}
	}
	if l.ch == 'E' || l.ch == 'e' {
		isFloat = true
		l.nextRune()
		if l.ch == '+' || l.ch == '-' {
			l.nextRune()
		}
		err := l.readDigits()
		if err != nil {
			return Token{}, err
		}
	}
	kind := INT
	if isFloat {
		kind = FLOAT
	}
	return makeToken(kind, start, l.offset, l.sliceBody(start, l.offset)), nil
}

// Returns the new position in the source after reading digits.
func (l *Lexer) readDigits() error {
	if l.ch < '0' || l.ch > '9' {
		var description string
		if l.ch != 0 {
			description = fmt.Sprintf("Invalid number, expected digit but got: %v.", printCharCode(l.ch))
		} else {
			description = "Invalid number, expected digit but got: EOF."
		}
		return gqlerrors.NewSyntaxError(l.src, l.offset.runes, description)
	}
	for l.ch >= '0' && l.ch <= '9' {
		l.nextRune()
	}
	return nil
}

func (l *Lexer) readString() (Token, error) {
	start := l.offset
	chunkStart := l.rdOffset
	value := make([]string, 0, 16)
	for {
		l.nextRune()

		if l.ch == 0 || l.ch == '"' || l.ch == 10 || l.ch == 13 {
			break
		}
		if l.ch < 0x0020 && l.ch != 0x0009 {
			return Token{}, gqlerrors.NewSyntaxError(l.src, l.offset.runes, fmt.Sprintf(`Invalid character within String: %v.`, printCharCode(l.ch)))
		}
		if l.ch == '\\' {
			value = append(value, l.sliceBody(chunkStart, l.offset))
			l.nextRune()
			switch l.ch {
			case '"':
				value = append(value, "\"")
			case '/':
				value = append(value, "/")
			case '\\':
				value = append(value, "\\")
			case 'b':
				value = append(value, "\b")
			case 'f':
				value = append(value, "\f")
			case 'n':
				value = append(value, "\n")
			case 'r':
				value = append(value, "\r")
			case 't':
				value = append(value, "\t")
			case 'u':
				offs := l.rdOffset
				l.nextRune()
				u1 := l.ch
				l.nextRune()
				u2 := l.ch
				l.nextRune()
				u3 := l.ch
				l.nextRune()
				u4 := l.ch
				charCode := uniCharCode(u1, u2, u3, u4)
				if charCode < 0 {
					return Token{}, gqlerrors.NewSyntaxError(l.src, offs.runes-1, fmt.Sprintf(`Invalid character escape sequence: \u%s`, l.sliceBody(offs, l.rdOffset)))
				}
				value = append(value, string(charCode))
			default:
				return Token{}, gqlerrors.NewSyntaxError(l.src, l.offset.runes, fmt.Sprintf(`Invalid character escape sequence: \%c.`, l.ch))
			}
			chunkStart = l.rdOffset
		}
	}
	if l.ch != '"' {
		return Token{}, gqlerrors.NewSyntaxError(l.src, l.offset.runes, "Unterminated string.")
	}
	if chunkStart.bytes != l.offset.bytes {
		value = append(value, l.sliceBody(chunkStart, l.offset))
	}
	l.nextRune()
	return makeToken(STRING, start, l.offset, strings.Join(value, "")), nil
}

// Converts four hexidecimal chars to the integer that the
// string represents. For example, uniCharCode('0','0','0','f')
// will return 15, and uniCharCode('0','0','f','f') returns 255.
// Returns a negative number on error, if a char was invalid.
// This is implemented by noting that char2hex() returns -1 on error,
// which means the result of ORing the char2hex() will also be negative.
func uniCharCode(a, b, c, d rune) rune {
	return rune(char2hex(a)<<12 | char2hex(b)<<8 | char2hex(c)<<4 | char2hex(d))
}

// Converts a hex character to its integer value.
// '0' becomes 0, '9' becomes 9
// 'A' becomes 10, 'F' becomes 15
// 'a' becomes 10, 'f' becomes 15
// Returns -1 on error.
func char2hex(a rune) int {
	switch {
	case a >= '0' && a <= '9': // 0-9
		return int(a) - '0'
	case a >= 'A' && a <= 'F': // A-F
		return int(a) + 10 - 'A'
	case a >= 'a' && a <= 'f': // a-f
		return int(a) + 10 - 'a'
	}
	return -1
}

func makeToken(kind int, start, end offset, value string) Token {
	return Token{Kind: kind, Start: start.runes, End: end.runes, Value: value}
}

func printCharCode(code rune) string {
	// NaN/undefined represents access beyond the end of the file.
	if code < 0 {
		return "<EOF>"
	}
	// print as ASCII for printable range
	if code >= 0x0020 && code < 0x007F {
		return fmt.Sprintf(`"%c"`, code)
	}
	// Otherwise print the escaped form. e.g. `"\\u0007"`
	return fmt.Sprintf(`"\\u%04X"`, code)
}

func (l *Lexer) readToken() (Token, error) {
	l.skipWhitespace()
	if l.ch == 0 {
		return makeToken(EOF, l.rdOffset, l.rdOffset, ""), nil
	}
	// SourceCharacter
	if l.ch < 0x0020 && l.ch != 0x0009 && l.ch != 0x000A && l.ch != 0x000D {
		return Token{}, gqlerrors.NewSyntaxError(l.src, l.offset.runes, fmt.Sprintf(`Invalid character %v`, printCharCode(l.ch)))
	}
	startOffset := l.offset
	ch := l.ch
	switch {
	case isLetter(ch):
		return l.readName()
	case isDigit(ch) || ch == '-':
		return l.readNumber()
	case ch == '"':
		return l.readString()
	default:
		l.nextRune() // always make progress
		switch ch {
		case '!':
			return makeToken(BANG, startOffset, l.offset, ""), nil
		case '$':
			return makeToken(DOLLAR, startOffset, l.offset, ""), nil
		case '(':
			return makeToken(PAREN_L, startOffset, l.offset, ""), nil
		case ')':
			return makeToken(PAREN_R, startOffset, l.offset, ""), nil
		case '.':
			if l.ch == '.' {
				l.nextRune()
				if l.ch == '.' {
					l.nextRune()
					return makeToken(SPREAD, startOffset, l.offset, ""), nil
				}
			}
			break
		case ':':
			return makeToken(COLON, startOffset, l.offset, ""), nil
		case '=':
			return makeToken(EQUALS, startOffset, l.offset, ""), nil
		case '@':
			return makeToken(AT, startOffset, l.offset, ""), nil
		case '[':
			return makeToken(BRACKET_L, startOffset, l.offset, ""), nil
		case ']':
			return makeToken(BRACKET_R, startOffset, l.offset, ""), nil
		case '{':
			return makeToken(BRACE_L, startOffset, l.offset, ""), nil
		case '|':
			return makeToken(PIPE, startOffset, l.offset, ""), nil
		case '}':
			return makeToken(BRACE_R, startOffset, l.offset, ""), nil
		case '#':
			for {
				if l.ch == '\n' || l.ch == '\r' || l.ch == 0 {
					break
				}
				l.nextRune()
			}
			return makeToken(COMMENT, startOffset, l.offset, strings.TrimSpace(l.sliceBody(startOffset, l.offset))), nil
		}
	}
	description := fmt.Sprintf("Unexpected character %v.", printCharCode(ch))
	return Token{}, gqlerrors.NewSyntaxError(l.src, startOffset.runes, description)
}

func (l *Lexer) sliceBody(start, end offset) string {
	return l.body[start.bytes:end.bytes]
}

func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9' || ch >= utf8.RuneSelf && unicode.IsDigit(ch)
}

// Reads from body starting at startPosition until it finds a non-whitespace
// or commented character, then returns the position of that character for lexing.
// lexing.
func (l *Lexer) skipWhitespace() {
	for {
		switch l.ch {
		case 0xFEFF, ' ', ',', '\n', '\r', '\t':
		default:
			return
		}
		l.nextRune()
	}
}

func GetTokenDesc(token Token) string {
	if token.Value == "" {
		return GetTokenKindDesc(token.Kind)
	}
	return fmt.Sprintf("%s %q", GetTokenKindDesc(token.Kind), token.Value)
}

func GetTokenKindDesc(kind int) string {
	return tokenDescription[kind]
}
