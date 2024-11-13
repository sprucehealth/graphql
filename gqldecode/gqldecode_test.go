package gqldecode

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func Example() {
	type Artist struct {
		Name   string   `gql:"name,nonzero,plane0"`
		Age    int      `gql:"age"`
		Albums []string `gql:"albums"`
	}
	args := map[string]any{
		"name": "Jimi Hendrix",
		"age":  75,
		"albums": []any{
			"Are You Experienced",
			"Axis: Bold as Love",
			"Electric Ladyland",
		},
	}
	var artist Artist
	err := Decode(args, &artist)
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Printf("%+v", artist)
	// Output:
	// {Name:Jimi Hendrix Age:75 Albums:[Are You Experienced Axis: Bold as Love Electric Ladyland]}
}

type stringList []string

func TestSimple(t *testing.T) {
	input := map[string]any{
		"name":         "Gob",
		"age":          45,
		"person":       true,
		"keywords":     []any{"blacklisted", "magician", " starfish "},
		"optStringSet": "foo",
		"matchTypes":   stringList{"hello", "world"},
	}
	type simpleStruct struct {
		Name            string     `gql:"name"`
		Age             int        `gql:"age"`
		Person          bool       `gql:"person"`
		Keywords        []string   `gql:"keywords"`
		OptStringSet    *string    `gql:"optStringSet"`
		OptStringNotSet *string    `gql:"optStringNotSet"`
		MatchTypes      stringList `gql:"matchTypes"`
	}
	var st simpleStruct
	if err := Decode(input, &st); err != nil {
		t.Fatal(err)
	}
	foo := "foo"
	exp := simpleStruct{
		Name:            "Gob",
		Age:             45,
		Person:          true,
		Keywords:        []string{"blacklisted", "magician", "starfish"},
		OptStringSet:    &foo,
		OptStringNotSet: nil,
		MatchTypes:      stringList{"hello", "world"},
	}
	if !reflect.DeepEqual(exp, st) {
		t.Fatalf("Expected %+v got %+v", exp, st)
	}
}

func TestSanitization(t *testing.T) {
	input := map[string]any{
		"name": "Go\uFEFFb",
	}
	type simpleStruct struct {
		Name string `gql:"name"`
	}
	var st simpleStruct
	if err := Decode(input, &st); err != nil {
		t.Fatal(err)
	}
	exp := simpleStruct{
		Name: "Gob",
	}
	if !reflect.DeepEqual(exp, st) {
		t.Fatalf("Expected %+v got %+v", exp, st)
	}
}

func TestAliasedString(t *testing.T) {
	type enum string
	input := map[string]any{
		"type": "FOO",
		"bar":  enum("BAR"),
	}
	type simpleStruct struct {
		Type enum `gql:"type"`
		Bar  enum `gql:"bar"`
	}
	var st simpleStruct
	if err := Decode(input, &st); err != nil {
		t.Fatal(err)
	}
	exp := simpleStruct{
		Type: enum("FOO"),
		Bar:  enum("BAR"),
	}
	if !reflect.DeepEqual(exp, st) {
		t.Fatalf("Expected %+v got %+v", exp, st)
	}
}

func TestSubStruct(t *testing.T) {
	input := map[string]any{
		"ptr":         map[string]any{"foo": "bar"},
		"nonptr":      map[string]any{"foo": "123"},
		"ptrslice":    []any{map[string]any{"foo": "aaa"}},
		"nonptrslice": []any{map[string]any{"foo": "zzz"}},
	}
	type subStruct struct {
		Foo string `gql:"foo"`
	}
	type withSubStruct struct {
		Ptr         *subStruct   `gql:"ptr"`
		NonPtr      subStruct    `gql:"nonptr"`
		PtrSlice    []*subStruct `gql:"ptrslice"`
		NonPtrSlice []subStruct  `gql:"nonptrslice"`
	}
	var st withSubStruct
	if err := Decode(input, &st); err != nil {
		t.Fatal(err)
	}
	exp := withSubStruct{
		Ptr:         &subStruct{Foo: "bar"},
		NonPtr:      subStruct{Foo: "123"},
		PtrSlice:    []*subStruct{{Foo: "aaa"}},
		NonPtrSlice: []subStruct{{Foo: "zzz"}},
	}
	if !reflect.DeepEqual(exp, st) {
		t.Fatalf("Expected %+v got %+v", exp, st)
	}
}

func TestPlane0Validation(t *testing.T) {
	// Allow plane0

	input := map[string]any{
		"name": "Foo",
	}
	type plane0Struct struct {
		Name string `gql:"name,plane0"`
	}
	st := &plane0Struct{}
	if err := Decode(input, st); err != nil {
		t.Fatal(err)
	}
	exp := &plane0Struct{
		Name: "Foo",
	}
	if !reflect.DeepEqual(exp, st) {
		t.Fatalf("Expected %+v got %+v", exp, st)
	}

	// Non plane0 should fail

	input = map[string]any{
		"name": "Foo👀",
	}
	var validationError *ValidationFailedError
	err := Decode(input, st)
	if !errors.As(err, &validationError) {
		t.Fatalf("Expected *ValidationFailedError got %T", err)
	} else if validationError.Field != "name" {
		t.Fatalf("Expected field 'name' got %q", validationError.Field)
	}
}

type uppercaseString string

func (s *uppercaseString) DecodeGQL(v any) error {
	*s = uppercaseString(strings.ToUpper(v.(string)))
	return nil
}

type timestamp struct {
	Time time.Time
}

func (t *timestamp) DecodeGQL(v any) error {
	tm := v.(map[string]any)
	t.Time = time.Unix(tm["seconds"].(int64), tm["nanoseconds"].(int64))
	return nil
}

func TestCustomDecoder(t *testing.T) {
	input := map[string]any{
		"name": "Foo",
		"time": map[string]any{
			"seconds":     int64(7),
			"nanoseconds": int64(13),
		},
	}
	type testStruct struct {
		Name uppercaseString `gql:"name"`
		Time *timestamp      `gql:"time"`
	}
	st := &testStruct{}
	if err := Decode(input, st); err != nil {
		t.Fatal(err)
	}
	exp := &testStruct{
		Name: "FOO",
		Time: &timestamp{Time: time.Unix(7, 13)},
	}
	if !reflect.DeepEqual(exp, st) {
		t.Fatalf("Expected %+v got %+v", exp, st)
	}
}

func TestTimestamp(t *testing.T) {
	in := map[string]any{
		"timestampFloat": 1000000010.5,
		"timestampInt":   1000000000,
	}
	type outStruct struct {
		TimestampFloat time.Time  `gql:"timestampFloat"`
		TimestampInt   *time.Time `gql:"timestampInt"`
	}
	var out outStruct
	if err := Decode(in, &out); err != nil {
		t.Fatal(err)
	}
	tm := time.Unix(1000000000, 0).UTC()
	exp := outStruct{
		TimestampFloat: time.Unix(1000000010, int64(500*time.Millisecond)).UTC(),
		TimestampInt:   &tm,
	}
	if !reflect.DeepEqual(exp, out) {
		t.Fatalf("Expected %+v got %+v", exp, out)
	}
}

func TestEmbeddedStruct(t *testing.T) {
	type outSubStruct struct {
		A string `gql:"a"`
		B string `gql:"b"`
	}
	type outStruct struct {
		SubStruct outSubStruct `gql:"subStruct"`
	}

	in := map[string]any{
		"subStruct": &outSubStruct{
			A: "A1",
			B: "B1",
		},
	}

	var out outStruct
	if err := Decode(in, &out); err != nil {
		t.Fatal(err)
	}

	exp := outStruct{
		SubStruct: outSubStruct{
			A: "A1",
			B: "B1",
		},
	}
	if !reflect.DeepEqual(exp, out) {
		t.Fatalf("Expected %+v got %+v", exp, out)
	}
}

func BenchmarkDecode(b *testing.B) {
	input := map[string]any{
		"string":     "vvvvv",
		"int":        123123,
		"stringList": []any{"abc", "foo"},
		"struct": map[string]any{
			"field": "string",
		},
	}
	type benchmarkSubStruct struct {
		Field string `gql:"field"`
	}
	type benchmarkStruct struct {
		String     string              `gql:"string"`
		Int        int                 `gql:"int"`
		StringList []string            `gql:"stringList"`
		Struct     *benchmarkSubStruct `gql:"struct"`
	}
	b.ReportAllocs()
	b.ResetTimer()
	var st benchmarkStruct
	for i := 0; i < b.N; i++ {
		if err := Decode(input, &st); err != nil {
			b.Fatal(err)
		}
	}
}
