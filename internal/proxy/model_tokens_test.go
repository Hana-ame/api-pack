package proxy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTokenFile 把 doc 序列化成临时 JSON 文件, 返回路径。
func writeTokenFile(t *testing.T, dir, name string, doc any) string {
	t.Helper()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	fn := filepath.Join(dir, name)
	if err := os.WriteFile(fn, b, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return fn
}

func TestParseTokenSpec(t *testing.T) {
	cases := []struct{ spec, src, path string }{
		{"/a/b.json", "/a/b.json", "$"},
		{"/a/b.json#$.x.y", "/a/b.json", "$.x.y"},
		{"env://V#$.tokens", "env://V", "$.tokens"},
		{"/a/b#c.json#$.p", "/a/b#c.json", "$.p"}, // 文件名含 #: 最后一个 # 才是分隔
		{"/a/b.json#$.a#", "/a/b.json#$.a", "$"},  // 尾部空 path 默认 $
	}
	for _, c := range cases {
		src, path := parseTokenSpec(c.spec)
		if src != c.src || path != c.path {
			t.Errorf("parseTokenSpec(%q) = (%q,%q), want (%q,%q)", c.spec, src, path, c.src, c.path)
		}
	}
}

func TestModelTokenTableFileAndGroups(t *testing.T) {
	dir := t.TempDir()
	doc := map[string]any{
		"xingyue": map[string]any{
			"tokens": map[string]any{
				"m1": "tok-m1",
				"m2": []any{"tok-a", "tok-b"},
				"*":  "tok-fallback",
			},
		},
	}
	fn := writeTokenFile(t, dir, "tokens.json", doc)
	tab, err := NewModelTokenTable(fn + "#$.xingyue.tokens")
	if err != nil {
		t.Fatalf("NewModelTokenTable: %v", err)
	}
	if tab.Len() != 3 {
		t.Fatalf("Len=%d want 3 (%s)", tab.Len(), tab.Describe())
	}
	// 精确命中单 token
	if tok, ok := tab.Token("m1"); !ok || tok != "tok-m1" {
		t.Errorf("Token(m1)=%q,%v", tok, ok)
	}
	// token 组逐次轮转
	seen := map[string]int{}
	for i := 0; i < 4; i++ {
		tok, ok := tab.Token("m2")
		if !ok {
			t.Fatalf("Token(m2) miss at round %d", i)
		}
		seen[tok]++
	}
	if seen["tok-a"] != 2 || seen["tok-b"] != 2 {
		t.Errorf("rotation unfair: %v", seen)
	}
	// 未命中走 "*" 兜底
	if tok, ok := tab.Token("nope"); !ok || tok != "tok-fallback" {
		t.Errorf("fallback Token(nope)=%q,%v", tok, ok)
	}
	// 无兜底键时未命中 ok=false
	tab2, err := NewModelTokenTable(writeTokenFile(t, dir, "no-fb.json", map[string]any{"tokens": map[string]any{"m1": "tok-m1"}}) + "#$.tokens")
	if err != nil {
		t.Fatalf("no-fb table: %v", err)
	}
	if _, ok := tab2.Token("nope"); ok {
		t.Error("Token(nope) should miss without fallback key")
	}
	// Describe 绝不泄露 token 值
	desc := tab.Describe()
	for _, secret := range []string{"tok-m1", "tok-a", "tok-b", "tok-fallback"} {
		if strings.Contains(desc, secret) {
			t.Errorf("Describe leaked token %q: %s", secret, desc)
		}
	}
}

func TestModelTokenTableHotReload(t *testing.T) {
	dir := t.TempDir()
	v1 := map[string]any{"tokens": map[string]any{"m1": "old"}}
	v2 := map[string]any{"tokens": map[string]any{"m1": "new", "m2": "extra"}}
	fn := writeTokenFile(t, dir, "rt.json", v1)
	tab, err := NewModelTokenTable(fn + "#$.tokens")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if tok, _ := tab.Token("m1"); tok != "old" {
		t.Fatalf("initial token=%q", tok)
	}
	// 重写文件并把 mtime 明确推进一分钟, ttl 缩到 1ms
	writeTokenFile(t, dir, "rt.json", v2)
	fut := time.Now().Add(time.Minute)
	if err := os.Chtimes(fn, fut, fut); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	tab.ttl = time.Millisecond
	time.Sleep(2 * time.Millisecond)
	if tok, _ := tab.Token("m1"); tok != "new" {
		t.Errorf("after reload m1=%q want new", tok)
	}
	if tok, ok := tab.Token("m2"); !ok || tok != "extra" {
		t.Errorf("after reload m2=%q,%v", tok, ok)
	}
	// 坏文件不影响旧表: 写坏 JSON, 推进 mtime, 读取只应打日志并保留旧值
	if err := os.WriteFile(fn, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("break: %v", err)
	}
	if err := os.Chtimes(fn, fut.Add(time.Minute), fut.Add(time.Minute)); err != nil {
		t.Fatalf("chtimes2: %v", err)
	}
	tab.ttl = time.Millisecond
	time.Sleep(2 * time.Millisecond)
	if tok, _ := tab.Token("m1"); tok != "new" {
		t.Errorf("broken reload must keep old table, m1=%q", tok)
	}
}

func TestModelTokenTableEnvSource(t *testing.T) {
	doc := map[string]any{"tokens": map[string]any{"m9": "env-tok"}}
	b, _ := json.Marshal(doc)
	t.Setenv("MT_TEST_JSON", string(b))
	tab, err := NewModelTokenTable("env://MT_TEST_JSON#$.tokens")
	if err != nil {
		t.Fatalf("env load: %v", err)
	}
	if tok, ok := tab.Token("m9"); !ok || tok != "env-tok" {
		t.Errorf("Token(m9)=%q,%v", tok, ok)
	}
	// env 形态不做热重载: 改 env 值不生效
	doc2 := map[string]any{"tokens": map[string]any{"m9": "changed"}}
	b2, _ := json.Marshal(doc2)
	t.Setenv("MT_TEST_JSON", string(b2))
	tab.ttl = time.Millisecond
	time.Sleep(2 * time.Millisecond)
	if tok, _ := tab.Token("m9"); tok != "env-tok" {
		t.Errorf("env source must not reload, got %q", tok)
	}
}

func TestModelTokenTableErrors(t *testing.T) {
	dir := t.TempDir()
	good := writeTokenFile(t, dir, "g.json", map[string]any{"tokens": map[string]any{"m": "t"}})
	cases := []string{
		"",                      // 空 spec
		"#$.tokens",             // 缺 source
		"env://NO_SUCH_ENV_XYZ", // env 未设置
		filepath.Join(dir, "missing.json"),
		good + "#$.nope", // jsonpath 无命中
		good + "#$..[0]", // jsonpath 语法错
	}
	for _, spec := range cases {
		if _, err := NewModelTokenTable(spec); err == nil {
			t.Errorf("spec %q expected error", spec)
		}
	}
	// 形态错: token 值为空串 / 数组里混数字
	bad1 := writeTokenFile(t, dir, "bad1.json", map[string]any{"tokens": map[string]any{"m": ""}})
	if _, err := NewModelTokenTable(bad1 + "#$.tokens"); err == nil {
		t.Error("empty token string must error")
	}
	bad2 := writeTokenFile(t, dir, "bad2.json", map[string]any{"tokens": map[string]any{"m": []any{"ok", float64(3)}}})
	if _, err := NewModelTokenTable(bad2 + "#$.tokens"); err == nil {
		t.Error("non-string in group must error")
	}
}

func TestModelTokenTableArrayMerge(t *testing.T) {
	dir := t.TempDir()
	// 数组形: [{"m1":"t1"},{"m2":"t2"}] 用通配命中两个对象逐个合并
	doc := map[string]any{"groups": []any{
		map[string]any{"m1": "t1"},
		map[string]any{"m2": "t2"},
	}}
	fn := writeTokenFile(t, dir, "arr.json", doc)
	tab, err := NewModelTokenTable(fn + "#$.groups[*]")
	if err != nil {
		t.Fatalf("array merge load: %v", err)
	}
	if tok, ok := tab.Token("m1"); !ok || tok != "t1" {
		t.Errorf("m1=%q,%v", tok, ok)
	}
	if tok, ok := tab.Token("m2"); !ok || tok != "t2" {
		t.Errorf("m2=%q,%v", tok, ok)
	}
}
