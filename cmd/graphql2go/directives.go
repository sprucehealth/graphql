package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type directives map[string]string

func parseDirectives(s string) (string, directives, error) {
	startIx := strings.IndexByte(s, '[')
	endIx := strings.IndexByte(s, ']')
	if startIx < 0 || endIx <= startIx {
		return s, nil, nil
	}
	ds := s[startIx+1 : endIx]
	s = strings.TrimSpace(strings.TrimSpace(s[:startIx]) + " " + strings.TrimSpace(s[endIx+1:]))
	dirs := directives(make(map[string]string))
	scn := &directiveScanner{s: ds}
	keyTok, key, keyPos := scn.nextToken()
parser:
	for keyTok != tokEOF {
		if keyTok != tokString {
			return "", nil, fmt.Errorf("bad token at %d in %q", keyPos, ds)
		}
		t, _, p := scn.nextToken()
		switch t {
		case tokEOF:
			dirs[key] = ""
			break parser
		case tokEqual:
			t, v, p := scn.nextToken()
			if t != tokString {
				return "", nil, fmt.Errorf("bad token at %d in %q", p, ds)
			}
			dirs[key] = v
		case tokComma:
		case tokIllegal:
			return "", nil, fmt.Errorf("bad token at %d in %q", p, ds)
		default:
			break parser
		}
		keyTok, key, keyPos = scn.nextToken()
	}
	return s, dirs, nil
}

type directiveScanner struct {
	s string
	i int
}

type token int

const (
	tokIllegal token = iota
	tokEOF
	tokString
	tokComma
	tokEqual
)

func (l *directiveScanner) nextToken() (token, string, int) {
	// Skip whitespace
	for len(l.s) > l.i && l.s[l.i] == ' ' {
		l.i++
	}
	if l.i >= len(l.s) {
		return tokEOF, "", l.i
	}
	r, n := utf8.DecodeRuneInString(l.s[l.i:])
	l.i += n
	switch r {
	case '=':
		return tokEqual, "=", l.i - n
	case ',':
		return tokComma, ",", l.i - n
	case '"':
		start := l.i - n
		escaped := false
		for l.i < len(l.s) {
			r, n := utf8.DecodeRuneInString(l.s[l.i:])
			l.i += n
			if r == '\\' && !escaped {
				escaped = true
				continue
			}
			if r == '"' && !escaped {
				v, err := strconv.Unquote(l.s[start:l.i])
				if err != nil {
					return tokIllegal, "", start
				}
				return tokString, v, start
			}
			escaped = false
		}
		return tokIllegal, "", l.i - n
	}
	if isIdent(r) {
		start := l.i - n
		for l.i < len(l.s) {
			r, n := utf8.DecodeRuneInString(l.s[l.i:])
			if !isIdent(r) {
				break
			}
			l.i += n
		}
		return tokString, l.s[start:l.i], start
	}
	return tokIllegal, "", l.i - n
}

func isIdent(r rune) bool {
	return (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_'
}
