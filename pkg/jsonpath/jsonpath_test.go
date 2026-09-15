package jsonpath

import (
	"encoding/json"
	"reflect"
	"testing"
)

func mustDoc(t *testing.T, s string) any {
	t.Helper()
	var doc any
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return doc
}

func TestGetShapes(t *testing.T) {
	// 文档用 Go 字面量构造, 值语义与 encoding/json 解码一致(map[string]any/[]any/float64/string)
	doc := map[string]any{
		"a":    map[string]any{"b": map[string]any{"c": float64(1)}, "s": "x"},
		"a.b":  "dv",
		"list": []any{float64(10), float64(20), float64(30)},
		"obj":  map[string]any{"x": "sx", "y": "sy"},
		"deep": map[string]any{"l1": map[string]any{
			"name": "n1",
			"l2": map[string]any{
				"name": "n2",
				"arr":  []any{map[string]any{"name": "n3"}},
			},
		}},
	}

	cases := []struct {
		path string
		want []any
	}{
		{"$.a.b.c", []any{float64(1)}},
		{"$.a.s", []any{"x"}},
		{"$['a.b']", []any{"dv"}},
		{"$.list[1]", []any{float64(20)}},
		{"$.list[-1]", []any{float64(30)}},
		{"$.obj.*", []any{"sx", "sy"}}, // 键字典序, 稳定
		{"$.list[*]", []any{float64(10), float64(20), float64(30)}},
		{"$.list.foo", nil}, // 在数组上取子键 -> 空(不是错误)
		{"$.obj[*]", []any{"sx", "sy"}},
		{"$..name", []any{"n1", "n2", "n3"}},
		{"$..['name']", []any{"n1", "n2", "n3"}},
		{"$.a.missing", nil},
		{"$", []any{doc}},
		{"a.b.c", []any{float64(1)}}, // 宽松模式: 省略 $.
		{"$.list[3]", nil},           // 越界 -> 空
	}
	for _, tc := range cases {
		got, err := Get(tc.path, doc)
		if err != nil {
			t.Errorf("Get(%q): unexpected error %v", tc.path, err)
			continue
		}
		if len(got) == 0 && tc.want == nil {
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("Get(%q) = %#v, want %#v", tc.path, got, tc.want)
		}
	}
}

func TestGetErrors(t *testing.T) {
	doc := map[string]any{"a": float64(1)}
	for _, bad := range []string{"$.a.", "$.a[", "$[", "$['unclosed", "$.a[?(@.b)]", "$.a[1x]", "$.a]", "$.a['k'", "$..[0]"} {
		if _, err := Get(bad, doc); err == nil {
			t.Errorf("Get(%q): expected parse error, got nil", bad)
		}
	}
}

func TestGetString(t *testing.T) {
	doc := map[string]any{
		"a":   map[string]any{"tok": "sk-1"},
		"n":   float64(3),
		"lst": []any{"t1", "t2"},
	}
	if s, err := GetString("$.a.tok", doc); err != nil || s != "sk-1" {
		t.Errorf("GetString valid: %q, %v", s, err)
	}
	if _, err := GetString("$.n", doc); err == nil {
		t.Error("GetString on number should error")
	}
	if _, err := GetString("$.missing", doc); err == nil {
		t.Error("GetString on zero hits should error")
	}
	if _, err := GetString("$.lst[*]", doc); err == nil {
		t.Error("GetString on multiple hits should error")
	}
}
