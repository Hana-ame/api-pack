package proxy

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Hana-ame/api-pack/pkg/debug"
	"github.com/Hana-ame/api-pack/pkg/jsonpath"
)

// fallbackKey 是 token 表里的兜底模型键: 请求体 model 未精确命中时使用。
const fallbackKey = "*"

const defaultReloadTTL = 5 * time.Second

// ModelTokenTable 按 env 给的 spec 从 JSON 文档加载「模型名 -> 出站 token 组」表,
// 供 GenericProxyHandler 按请求的 model 注入不同的上游 Authorization override。
//
// spec 格式: "<source>[#<jsonpath>]"（'#' 取最后一个; jsonpath 省略时默认 "$"）
//
//	source 形态:
//	  /abs/path/tokens.json   本地 JSON 文件（mtime/大小变化自动热重载, 轮换 token 不用重启）
//	  tokens.json             相对路径, 按进程 cwd 解析
//	  env://VAR_NAME          JSON 文档内联在该环境变量里（不热重载, env 进程内不变）
//
// JSONPath 求值结果的合法形态（见 buildGroups）:
//
//	{"model-a": "tok1", "model-b": ["tok2","tok3"], "*": "tokFallback"}
//	数组: [{"model-a":"t1"},{"model-b":"t2"}] 多对象合并; 通配/递归路径命中的多个
//	对象节点同样逐个合并。值=字符串为单 token; 值=字符串数组为 token 组, 逐次轮转。
//
// 任何日志/错误输出只出现模型名与数量, 绝不带 token 值。
type ModelTokenTable struct {
	spec string

	fromEnv  bool
	srcPath  string // 文件路径 或 env:// 剥掉前缀后的变量名
	jsonPath string
	ttl      time.Duration

	mu        sync.RWMutex
	groups    map[string]*tokenGroup
	models    []string // 排序后的模型键, 仅用于 Describe/日志
	lastCheck time.Time
	mtime     time.Time
	size      int64
}

type tokenGroup struct {
	tokens []string
	cur    atomic.Uint64
}

func (g *tokenGroup) next() string {
	return g.tokens[g.cur.Add(1)%uint64(len(g.tokens))]
}

// NewModelTokenTable 解析 spec 并加载一次。加载失败返回 error（调用方决定是否降级）,
// 宁可不启动也不带错 key 上线。
func NewModelTokenTable(spec string) (*ModelTokenTable, error) {
	src, path := parseTokenSpec(spec)
	if src == "" {
		return nil, fmt.Errorf("model token 配置为空(期望 <文件路径|env://VAR>[#$.json.path] 形式): %q", spec)
	}
	t := &ModelTokenTable{spec: spec, jsonPath: path, ttl: defaultReloadTTL}

	var data []byte
	switch {
	case strings.HasPrefix(src, "env://"):
		name := strings.TrimPrefix(src, "env://")
		if name == "" {
			return nil, fmt.Errorf("env:// 后变量名为空: %q", spec)
		}
		t.fromEnv = true
		t.srcPath = name
		raw := os.Getenv(name)
		if strings.TrimSpace(raw) == "" {
			return nil, fmt.Errorf("环境变量 %s 未设置或为空", name)
		}
		data = []byte(raw)
	default:
		t.srcPath = src
		b, err := os.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("读取 model token JSON 失败: %w", err)
		}
		data = b
		if fi, err := os.Stat(src); err == nil {
			t.mtime, t.size = fi.ModTime(), fi.Size()
		}
	}

	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("model token JSON 解析失败(source=%s): %w", t.source(), err)
	}
	nodes, err := jsonpath.Get(t.jsonPath, doc)
	if err != nil {
		return nil, fmt.Errorf("jsonpath 求值失败: %w", err)
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("jsonpath %q 在文档中无命中", t.jsonPath)
	}
	groups, err := buildGroups(nodes)
	if err != nil {
		return nil, err
	}

	t.lastCheck = time.Now()
	t.setGroupsLocked(groups)
	return t, nil
}

// Token 返回该模型的出站 token; 组内多 token 时逐次轮转。
// 未精确命中则查兜底键 "*"; 都没有返回 ok=false（调用方走原有透传逻辑）。
func (t *ModelTokenTable) Token(model string) (string, bool) {
	if t == nil || model == "" {
		return "", false
	}
	t.maybeReload()
	t.mu.RLock()
	g := t.groups[model]
	if g == nil {
		g = t.groups[fallbackKey]
	}
	t.mu.RUnlock()
	if g == nil {
		return "", false
	}
	return g.next(), true
}

// Len 返回表内条目数（含兜底键）。
func (t *ModelTokenTable) Len() int {
	if t == nil {
		return 0
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.groups)
}

// Describe 返回不含 token 值的概览, 用于启动日志。
func (t *ModelTokenTable) Describe() string {
	if t == nil {
		return "nil"
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return fmt.Sprintf("source=%s jsonpath=%q models=%v", t.source(), t.jsonPath, t.models)
}

func (t *ModelTokenTable) source() string {
	if t.fromEnv {
		return "env://" + t.srcPath
	}
	return "file:" + t.srcPath
}

// ---- 加载与热重载 ----

func (t *ModelTokenTable) maybeReload() {
	if t.fromEnv || t.ttl <= 0 {
		return
	}
	t.mu.RLock()
	stale := time.Since(t.lastCheck) >= t.ttl
	t.mu.RUnlock()
	if !stale {
		return
	}
	if err := t.reloadIfChanged(); err != nil {
		debug.W("model-tokens", "热重载失败, 沿用旧表: "+err.Error())
	}
}

func (t *ModelTokenTable) reloadIfChanged() error {
	fi, err := os.Stat(t.srcPath)
	if err != nil {
		return err
	}
	t.mu.RLock()
	changed := !fi.ModTime().Equal(t.mtime) || fi.Size() != t.size
	t.mu.RUnlock()
	if !changed {
		t.mu.Lock()
		t.lastCheck = time.Now()
		t.mu.Unlock()
		return nil
	}
	data, err := os.ReadFile(t.srcPath)
	if err != nil {
		return err
	}
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("JSON 解析失败: %w", err)
	}
	nodes, err := jsonpath.Get(t.jsonPath, doc)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return fmt.Errorf("jsonpath %q 无命中", t.jsonPath)
	}
	groups, err := buildGroups(nodes)
	if err != nil {
		return err
	}
	t.mu.Lock()
	t.setGroupsLocked(groups)
	t.lastCheck = time.Now()
	t.mtime, t.size = fi.ModTime(), fi.Size()
	t.mu.Unlock()
	debug.I("model-tokens", "已热重载: "+t.Describe())
	return nil
}

// setGroupsLocked 调用方必须持有写锁。
func (t *ModelTokenTable) setGroupsLocked(m map[string][]string) {
	groups := make(map[string]*tokenGroup, len(m))
	models := make([]string, 0, len(m))
	for k, toks := range m {
		groups[k] = &tokenGroup{tokens: toks}
		models = append(models, k)
	}
	sort.Strings(models)
	t.groups, t.models = groups, models
}

// ---- spec 解析与形态校验 ----

// parseTokenSpec 以最后一个 '#' 切分 source 与 jsonpath。
func parseTokenSpec(spec string) (src, jsonPath string) {
	src, jsonPath = spec, "$"
	if i := strings.LastIndex(spec, "#"); i >= 0 {
		src, jsonPath = spec[:i], spec[i+1:]
	}
	src = strings.TrimSpace(src)
	jsonPath = strings.TrimSpace(jsonPath)
	if jsonPath == "" {
		jsonPath = "$"
	}
	return src, jsonPath
}

// buildGroups 把 jsonpath 命中的节点折叠成 模型->token 列表。
// 接受: 对象{model: string|[...]string} / 多对象（含数组元素、通配命中）逐个合并。
func buildGroups(nodes []any) (map[string][]string, error) {
	out := map[string][]string{}
	for _, n := range nodes {
		switch t := n.(type) {
		case map[string]any:
			for k, v := range t {
				toks, err := toTokens(k, v)
				if err != nil {
					return nil, err
				}
				out[k] = toks
			}
		case []any:
			for _, e := range t {
				m, ok := e.(map[string]any)
				if !ok {
					return nil, fmt.Errorf("命中数组元素必须是 {model: token} 对象, 实际 %T", e)
				}
				for k, v := range m {
					toks, err := toTokens(k, v)
					if err != nil {
						return nil, err
					}
					out[k] = toks
				}
			}
		default:
			return nil, fmt.Errorf("jsonpath 命中值为 %T, 期望 模型->token 对象", n)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("token 表为空")
	}
	return out, nil
}

func toTokens(model string, v any) ([]string, error) {
	switch t := v.(type) {
	case string:
		if t == "" {
			return nil, fmt.Errorf("模型 %q 的 token 为空串", model)
		}
		return []string{t}, nil
	case []any:
		if len(t) == 0 {
			return nil, fmt.Errorf("模型 %q 的 token 组为空", model)
		}
		toks := make([]string, 0, len(t))
		for _, e := range t {
			s, ok := e.(string)
			if !ok || s == "" {
				return nil, fmt.Errorf("模型 %q 的 token 组含非字符串/空值", model)
			}
			toks = append(toks, s)
		}
		return toks, nil
	default:
		return nil, fmt.Errorf("模型 %q 的 token 值是 %T, 期望 字符串或字符串数组", model, v)
	}
}
