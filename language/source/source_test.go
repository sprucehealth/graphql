package source

import (
	"reflect"
	"testing"
)

func TestSourcePosition(t *testing.T) {
	src := New("", "\nfoo\nbar")
	cases := []struct {
		ix int
		ps Position
	}{
		{ix: 0, ps: Position{Offset: 0, Line: 1, Column: 1}},
		{ix: 1, ps: Position{Offset: 1, Line: 2, Column: 1}},
		{ix: 2, ps: Position{Offset: 2, Line: 2, Column: 2}},
		{ix: 4, ps: Position{Offset: 4, Line: 2, Column: 4}},
		{ix: 7, ps: Position{Offset: 7, Line: 3, Column: 3}},
		{ix: 8, ps: Position{Offset: 8, Line: 3, Column: 4}},
	}
	for _, c := range cases {
		v := src.Position(c.ix)
		if !reflect.DeepEqual(v, c.ps) {
			t.Errorf("src.Position(%d) = %#+v, expected %#+v", c.ix, v, c.ps)
		}
	}
}

func TestStringToLineIndex(t *testing.T) {
	cases := []struct {
		st string
		ix []int
	}{
		{st: "", ix: []int{0}},
		{st: "foo", ix: []int{0}},
		{st: "\nfoo", ix: []int{0, 1}},
		{st: "foo\nbar", ix: []int{0, 4}},
		{st: "\n", ix: []int{0, 1}},
		{st: "foo\n", ix: []int{0, 4}},
		{st: "foo\nbar\n", ix: []int{0, 4, 8}},
	}
	for _, c := range cases {
		v := stringToLineIndex(c.st)
		if !reflect.DeepEqual(v, c.ix) {
			t.Errorf("stringToLineIndex(%q) = %+v, expected %v", c.st, v, c.ix)
		}
	}
}
