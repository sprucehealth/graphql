package main

import (
	"slices"
	"testing"
)

func TestUnexportedName(t *testing.T) {
	cases := []struct {
		s string
		e string
	}{
		{s: "", e: ""},
		{s: "foo", e: "foo"},
		{s: "FOO", e: "foo"},
		{s: "SomeID", e: "someID"},
		{s: "URLWhat", e: "urlWhat"},
		{s: "AThing", e: "aThing"},
		{s: "PageIDs", e: "pageIDs"},
	}
	for _, c := range cases {
		t.Run(c.s, func(t *testing.T) {
			v := unexportedName(c.s)
			if v != c.e {
				t.Errorf("unexportedName(%q) = %q, expected %q", c.s, v, c.e)
			}
		})
	}
}

func TestExportedName(t *testing.T) {
	cases := []struct {
		s string
		e string
	}{
		{s: "", e: ""},
		{s: "foo", e: "Foo"},
		{s: "FOO", e: "FOO"},
		{s: "id", e: "ID"},
		{s: "someId", e: "SomeID"},
		{s: "urlWhat", e: "URLWhat"},
		{s: "ids", e: "IDs"},
		{s: "pageIds", e: "PageIDs"},
		{s: "pageIDs", e: "PageIDs"},
	}
	for _, c := range cases {
		t.Run(c.s, func(t *testing.T) {
			v := exportedName(c.s)
			if v != c.e {
				t.Errorf("exportedName(%q) = %q, expected %q", c.s, v, c.e)
			}
		})
	}
}

func TestExportedCamelCase(t *testing.T) {
	cases := []struct {
		s string
		e string
	}{
		{s: "", e: ""},
		{s: "foo", e: "Foo"},
		{s: "FOO", e: "Foo"},
		{s: "foo_bar", e: "FooBar"},
		{s: "FOO_BAR", e: "FooBar"},
		{s: "FOO_ID", e: "FooID"},
		{s: "foo_id", e: "FooID"},
		{s: "foo_url", e: "FooURL"},
	}
	for _, c := range cases {
		t.Run(c.s, func(t *testing.T) {
			v := exportedCamelCase(c.s)
			if v != c.e {
				t.Errorf("exportedCamelCase(%q) = %q, expected %q", c.s, v, c.e)
			}
		})
	}
}

func TestUpperInitialisms(t *testing.T) {
	cases := []struct {
		s string
		e string
	}{
		{s: "", e: ""},
		{s: "Foo", e: "Foo"},
		{s: "FooId", e: "FooID"},
		{s: "UrlFoo", e: "URLFoo"},
		{s: "ClientMutationId", e: "ClientMutationID"},
		{s: "WhatURLIsThisId", e: "WhatURLIsThisID"},
	}
	for _, c := range cases {
		t.Run(c.s, func(t *testing.T) {
			v := camelCaseInitialisms(c.s)
			if v != c.e {
				t.Errorf("camelCaseInitialisms(%q) = %q, expected %q", c.s, v, c.e)
			}
		})
	}
}

func TestUnindentAndTrim(t *testing.T) {
	cases := []struct {
		input  string
		output []string
	}{
		{input: "", output: nil},
		{input: "  ", output: nil},
		{input: "simple", output: []string{"simple"}},
		{input: "  beginning", output: []string{"beginning"}},
		{input: "end\t ", output: []string{"end"}},
		{input: "\nempty\n\nlines\n", output: []string{"empty", "", "lines"}},
		{
			input:  "two\nlines",
			output: []string{"two", "lines"},
		},
		{
			input:  "  two indented\n  lines",
			output: []string{"two indented", "lines"},
		},
		{
			input:  "\ttwo indented\n\tlines with space at end  ",
			output: []string{"two indented", "lines with space at end"},
		},
		{
			input:  "\ttwo indented\n\t\tlines with different indentation",
			output: []string{"two indented", "\tlines with different indentation"},
		},
		{
			input:  "\n  indented empty\n\n  lines\n",
			output: []string{"indented empty", "", "lines"},
		},
	}
	for _, c := range cases {
		o := unindentAndTrim(c.input)
		if !slices.Equal(o, c.output) {
			t.Fatalf("unindentAndTrim(%q) = %v\nexpected %v",
				c.input, o, c.output)
		}
	}
}
