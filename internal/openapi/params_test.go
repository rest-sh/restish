package openapi

import (
	"reflect"
	"testing"
)

func TestSerializeQueryParamStyles(t *testing.T) {
	explode := true
	parts, err := SerializeQueryParam(Param{Name: "tag", In: "query", Type: "array", Style: "form", Explode: &explode}, ArrayValue([]string{"red", "blue"}))
	if err != nil {
		t.Fatalf("form exploded array: %v", err)
	}
	if want := []QueryParam{{Name: "tag", Value: "red"}, {Name: "tag", Value: "blue"}}; !reflect.DeepEqual(parts, want) {
		t.Fatalf("form exploded array = %#v, want %#v", parts, want)
	}

	parts, err = SerializeQueryParam(Param{Name: "ids", In: "query", Type: "array", Style: "spaceDelimited"}, ArrayValue([]string{"a", "b"}))
	if err != nil {
		t.Fatalf("space delimited array: %v", err)
	}
	if want := []QueryParam{{Name: "ids", Value: "a b"}}; !reflect.DeepEqual(parts, want) {
		t.Fatalf("space delimited array = %#v, want %#v", parts, want)
	}

	parts, err = SerializeQueryParam(Param{Name: "filter", In: "query", Type: "object", Style: "deepObject"}, ObjectValue([]Field{{Key: "limit", Value: "10"}}))
	if err != nil {
		t.Fatalf("deep object: %v", err)
	}
	if want := []QueryParam{{Name: "filter[limit]", Value: "10"}}; !reflect.DeepEqual(parts, want) {
		t.Fatalf("deep object = %#v, want %#v", parts, want)
	}

	parts, err = SerializeQueryParam(Param{Name: "expand", In: "query", Type: "array", Style: "deepObject", Explode: &explode}, ArrayValue([]string{"data.customer", "data.invoice"}))
	if err != nil {
		t.Fatalf("deep object exploded array: %v", err)
	}
	if want := []QueryParam{{Name: "expand[]", Value: "data.customer"}, {Name: "expand[]", Value: "data.invoice"}}; !reflect.DeepEqual(parts, want) {
		t.Fatalf("deep object exploded array = %#v, want %#v", parts, want)
	}
}

func TestSerializePathHeaderCookieParamStyles(t *testing.T) {
	explode := true
	path, err := SerializePathParam(Param{Name: "id", In: "path", Type: "array", Style: "matrix", Explode: &explode}, ArrayValue([]string{"a", "b"}))
	if err != nil {
		t.Fatalf("matrix array: %v", err)
	}
	if path != ";id=a;id=b" {
		t.Fatalf("matrix array = %q, want ;id=a;id=b", path)
	}

	path, err = SerializePathParam(Param{Name: "filter", In: "path", Type: "object", Style: "label", Explode: &explode}, ObjectValue([]Field{{Key: "q", Value: "cats"}}))
	if err != nil {
		t.Fatalf("label object: %v", err)
	}
	if path != ".q=cats" {
		t.Fatalf("label object = %q, want .q=cats", path)
	}

	explode = false
	path, err = SerializePathParam(Param{Name: "id", In: "path", Type: "array", Style: "label", Explode: &explode}, ArrayValue([]string{"a", "b"}))
	if err != nil {
		t.Fatalf("label array: %v", err)
	}
	if path != ".a,b" {
		t.Fatalf("label array = %q, want .a,b", path)
	}

	path, err = SerializePathParam(Param{Name: "filter", In: "path", Type: "object", Style: "label", Explode: &explode}, ObjectValue([]Field{{Key: "name", Value: "Ada"}, {Key: "role", Value: "admin"}}))
	if err != nil {
		t.Fatalf("label object not exploded: %v", err)
	}
	if path != ".name,Ada,role,admin" {
		t.Fatalf("label object not exploded = %q, want .name,Ada,role,admin", path)
	}

	explode = true
	headers, err := SerializeHeaderParam(Param{Name: "X-IDs", In: "header", Type: "array"}, ArrayValue([]string{"a", "b"}))
	if err != nil {
		t.Fatalf("header array: %v", err)
	}
	if want := []string{"a,b"}; !reflect.DeepEqual(headers, want) {
		t.Fatalf("header array = %#v, want %#v", headers, want)
	}

	cookies, err := SerializeCookieParam(Param{Name: "session", In: "cookie", Type: "array", Explode: &explode}, ArrayValue([]string{"a", "b"}))
	if err != nil {
		t.Fatalf("cookie array: %v", err)
	}
	if want := []string{"session=a", "session=b"}; !reflect.DeepEqual(cookies, want) {
		t.Fatalf("cookie array = %#v, want %#v", cookies, want)
	}
}

func TestNormalizeArrayValuesSplitsEscapedComma(t *testing.T) {
	got := NormalizeArrayValues([]string{`red\,blue, green`})
	want := []string{"red,blue", "green"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeArrayValues = %#v, want %#v", got, want)
	}
}
