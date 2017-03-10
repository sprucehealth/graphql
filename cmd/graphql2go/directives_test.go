package main

import (
	"reflect"
	"testing"
)

func TestDirectiveScanner(t *testing.T) {
	type tok struct {
		t token
		v string
	}
	cases := []struct {
		s string
		t []tok
	}{
		{s: "", t: nil},
		{s: "foo", t: []tok{{t: tokString, v: "foo"}}},
		{s: "  foo=bar", t: []tok{
			{t: tokString, v: "foo"},
			{t: tokEqual, v: "="},
			{t: tokString, v: "bar"},
		}},
		{s: "foo=\"bar roo\"   ", t: []tok{
			{t: tokString, v: "foo"},
			{t: tokEqual, v: "="},
			{t: tokString, v: "bar roo"},
		}},
		{s: "foo=bar,  abc=xyz", t: []tok{
			{t: tokString, v: "foo"},
			{t: tokEqual, v: "="},
			{t: tokString, v: "bar"},
			{t: tokComma, v: ","},
			{t: tokString, v: "abc"},
			{t: tokEqual, v: "="},
			{t: tokString, v: "xyz"},
		}},
	}
	for _, c := range cases {
		scn := &directiveScanner{s: c.s}
		var toks []tok
		for {
			tk, v, _ := scn.nextToken()
			if tk == tokIllegal {
				t.Fatalf("Received illegal token for %q", c.s)
			}
			if tk == tokEOF {
				break
			}
			toks = append(toks, tok{t: tk, v: v})
		}
		if !reflect.DeepEqual(toks, c.t) {
			t.Errorf("directiveScanner{%q} = %+v, expected %+v", c.s, toks, c.t)
		}
	}
}

func TestParseDirectives(t *testing.T) {
	cases := []struct {
		s string
		e string
		d directives
	}{
		{s: "", e: "", d: nil},
		{s: "[", e: "[", d: nil},
		{s: "]", e: "]", d: nil},
		{s: "[]", e: "", d: map[string]string{}},
		{s: "[novalue]", e: "", d: map[string]string{"novalue": ""}},
		{s: "[key=value]", e: "", d: map[string]string{"key": "value"}},
		{s: "[key=\"quoted value\"]", e: "", d: map[string]string{"key": "quoted value"}},
		{s: "[novalue] foo", e: "foo", d: map[string]string{"novalue": ""}},
		{s: "foo [novalue]", e: "foo", d: map[string]string{"novalue": ""}},
		{s: "foo [novalue] bar", e: "foo bar", d: map[string]string{"novalue": ""}},
	}
	for _, c := range cases {
		e, d, err := parseDirectives(c.s)
		if err != nil {
			t.Fatalf("failed for %q: %s", c.s, err)
		}
		if !reflect.DeepEqual(d, c.d) {
			t.Errorf("parseDirectives(%q) = %+v, expected %+v", c.s, d, c.d)
		}
		if e != c.e {
			t.Errorf("parseDirectives(%q) = %s, expected %s", c.s, e, c.e)
		}
	}
}
