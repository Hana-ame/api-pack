// Package jsonpath 实现 JSONPath 的常用子集, 零第三方依赖。
//
// 设计动机: api-pack 里只需要把「一大坨 JSON 配置中的某个子树」取出来
// (典型: 出站 model token 组), 为此引入重型 JSONPath 库不值得。
//
// 支持语法:
//
//	$                根 (可省略, 省略时按根处理)
//	.child           子节点 (键名不能含 . [ 与空白)
//	['child']        子节点, 单/双引号, 支持反斜杠转义 (键名可含任意字符)
//	[0] [-1]         数组下标, 负数从尾部数
//	.*  [*]          通配: 对象全部值 / 数组全部元素
//	..child  ..*     递归下降: 全部层级里键名为 child 的值 / 所有子值 (深度优先)
//	..['child']      同上, 引号形式
//
// 不支持: filter 表达式 [?()]、切片 [a:b]、函数。
// 命中 0 个节点不算语法错误, 返回空切片; 对类型不符的节点(如在数组上取 .key)
// 不报错、产出空。
//
// 输入文档要求是 encoding/json 解码出的通用值:
// map[string]any / []any / string / float64 / bool / nil。
package jsonpath

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type selKind uint8

const (
	selChild       selKind = iota // .key / ['key']
	selIndex                      // [n]
	selWildcard                   // .* / [*]
	selRecChild                   // ..key
	selRecWildcard                // ..*
)

type selector struct {
	kind  selKind
	name  string
	index int
}

// Get 在 doc 上求值 path, 返回全部命中节点(顺序为键字典序的文档序)。
// path 语法错误返回 error; 语法正确但无命中返回 (nil, nil)。
func Get(path string, doc any) ([]any, error) {
	sels, err := parse(path)
	if err != nil {
		return nil, err
	}
	nodes := []any{doc}
	for _, s := range sels {
		var next []any
		for _, n := range nodes {
			next = s.apply(n, next)
		}
		nodes = next
		if len(nodes) == 0 {
			return nil, nil
		}
	}
	return nodes, nil
}

// GetString 求值并要求恰好一个命中且为字符串, 否则返回 error。
func GetString(path string, doc any) (string, error) {
	nodes, err := Get(path, doc)
	if err != nil {
		return "", err
	}
	if len(nodes) != 1 {
		return "", fmt.Errorf("jsonpath %q: 期望恰好 1 个命中, 实际 %d 个", path, len(nodes))
	}
	s, ok := nodes[0].(string)
	if !ok {
		return "", fmt.Errorf("jsonpath %q: 命中值不是字符串而是 %T", path, nodes[0])
	}
	return s, nil
}

func (s selector) apply(n any, out []any) []any {
	switch s.kind {
	case selChild:
		if m, ok := n.(map[string]any); ok {
			if v, ok := m[s.name]; ok {
				out = append(out, v)
			}
		}
	case selIndex:
		if a, ok := n.([]any); ok {
			i := s.index
			if i < 0 {
				i += len(a)
			}
			if i >= 0 && i < len(a) {
				out = append(out, a[i])
			}
		}
	case selWildcard:
		switch t := n.(type) {
		case map[string]any:
			// 键排序, 保证命中顺序稳定(而不是 Go map 的随机序)
			for _, k := range sortedKeys(t) {
				out = append(out, t[k])
			}
		case []any:
			out = append(out, t...)
		}
	case selRecChild:
		walk(n, func(cur any) {
			if m, ok := cur.(map[string]any); ok {
				if v, ok := m[s.name]; ok {
					out = append(out, v)
				}
			}
		})
	case selRecWildcard:
		walkChildren(n, func(v any) { out = append(out, v) })
	}
	return out
}

// walk 深度优先访问 n 及其全部后代(含 n 自身)。
func walk(n any, fn func(any)) {
	fn(n)
	switch t := n.(type) {
	case map[string]any:
		for _, k := range sortedKeys(t) {
			walk(t[k], fn)
		}
	case []any:
		for _, v := range t {
			walk(v, fn)
		}
	}
}

// walkChildren 深度优先访问 n 的后代, 对每个对象/数组枚举其全部直接子值。
func walkChildren(n any, fn func(any)) {
	switch t := n.(type) {
	case map[string]any:
		for _, k := range sortedKeys(t) {
			fn(t[k])
			walkChildren(t[k], fn)
		}
	case []any:
		for _, v := range t {
			fn(v)
			walkChildren(v, fn)
		}
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 插入排序: 配置对象键量小, 不值得 import sort
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// ---- 路径解析 ----

func parse(path string) ([]selector, error) {
	p := &parser{in: path}
	return p.parse()
}

type parser struct {
	in  string
	pos int
}

func (p *parser) errf(format string, a ...any) error {
	return fmt.Errorf("jsonpath %q: %s (位置 %d)", p.in, fmt.Sprintf(format, a...), p.pos)
}

func (p *parser) eof() bool { return p.pos >= len(p.in) }

func (p *parser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.in[p.pos]
}

func (p *parser) parse() ([]selector, error) {
	p.skipSpace()
	var sels []selector
	if p.peek() == '$' {
		p.pos++
	}
	first := true
	for {
		p.skipSpace()
		if p.eof() {
			break
		}
		switch {
		case strings.HasPrefix(p.in[p.pos:], ".."):
			p.pos += 2
			s, err := p.parseDescend()
			if err != nil {
				return nil, err
			}
			sels = append(sels, s)
		case p.peek() == '.':
			p.pos++
			s, err := p.parseDotChild()
			if err != nil {
				return nil, err
			}
			sels = append(sels, s)
		case p.peek() == '[':
			s, err := p.parseBracket()
			if err != nil {
				return nil, err
			}
			sels = append(sels, s)
		case first && (isNameChar(p.peek())):
			// 宽松模式: "a.b" 等价 "$.a.b"
			s, err := p.parseDotChild()
			if err != nil {
				return nil, err
			}
			sels = append(sels, s)
		default:
			return nil, p.errf("意外字符 %q", string(p.peek()))
		}
		first = false
	}
	return sels, nil
}

func isNameChar(c byte) bool {
	return c > ' ' && c != '.' && c != '[' && c != ']' && c != '\'' && c != '"'
}

// parseDotChild: '.' 之后的 名字 或 '*'。
func (p *parser) parseDotChild() (selector, error) {
	if p.peek() == '*' {
		p.pos++
		return selector{kind: selWildcard}, nil
	}
	start := p.pos
	for !p.eof() && isNameChar(p.in[p.pos]) {
		p.pos++
	}
	if p.pos == start {
		return selector{}, p.errf("'.' 后缺少键名")
	}
	return selector{kind: selChild, name: p.in[start:p.pos]}, nil
}

// parseDescend: '..' 之后的 名字 / '*' / ['key']。
func (p *parser) parseDescend() (selector, error) {
	if p.peek() == '[' {
		s, err := p.parseBracket()
		if err != nil {
			return selector{}, err
		}
		switch s.kind {
		case selChild:
			return selector{kind: selRecChild, name: s.name}, nil
		case selWildcard:
			return selector{kind: selRecWildcard}, nil
		default:
			return selector{}, p.errf("..[n] 数字下标递归不支持")
		}
	}
	if p.peek() == '*' {
		p.pos++
		return selector{kind: selRecWildcard}, nil
	}
	start := p.pos
	for !p.eof() && isNameChar(p.in[p.pos]) {
		p.pos++
	}
	if p.pos == start {
		return selector{}, p.errf("'..' 后缺少键名")
	}
	return selector{kind: selRecChild, name: p.in[start:p.pos]}, nil
}

// parseBracket: 整个 '[...]'。
func (p *parser) parseBracket() (selector, error) {
	p.pos++ // '['
	p.skipSpace()
	switch {
	case p.peek() == '*':
		p.pos++
		if err := p.closeBracket(); err != nil {
			return selector{}, err
		}
		return selector{kind: selWildcard}, nil
	case p.peek() == '\'' || p.peek() == '"':
		name, err := p.parseQuoted()
		if err != nil {
			return selector{}, err
		}
		if err := p.closeBracket(); err != nil {
			return selector{}, err
		}
		return selector{kind: selChild, name: name}, nil
	default:
		start := p.pos
		for !p.eof() && (p.in[p.pos] == '-' || (p.in[p.pos] >= '0' && p.in[p.pos] <= '9')) {
			p.pos++
		}
		text := p.in[start:p.pos]
		if text == "" || text == "-" {
			return selector{}, p.errf("[] 内期望 引号键名 / 数字下标 / *")
		}
		i, err := strconv.Atoi(text)
		if err != nil {
			return selector{}, p.errf("非法下标 %q", text)
		}
		if err := p.closeBracket(); err != nil {
			return selector{}, err
		}
		return selector{kind: selIndex, index: i}, nil
	}
}

func (p *parser) skipSpace() {
	for !p.eof() && unicode.IsSpace(rune(p.in[p.pos])) {
		p.pos++
	}
}

func (p *parser) closeBracket() error {
	p.skipSpace()
	if p.peek() != ']' {
		return p.errf("缺少 ']'")
	}
	p.pos++
	return nil
}

// parseQuoted 解析单/双引号字符串, 支持 \\ \" \' 转义, 其余原样保留。
func (p *parser) parseQuoted() (string, error) {
	q := p.in[p.pos]
	p.pos++
	var b strings.Builder
	for !p.eof() {
		c := p.in[p.pos]
		if c == '\\' {
			if p.pos+1 >= len(p.in) {
				return "", p.errf("悬挂的反斜杠")
			}
			n := p.in[p.pos+1]
			switch n {
			case '\\', '"', '\'':
				b.WriteByte(n)
			default:
				b.WriteByte('\\')
				b.WriteByte(n)
			}
			p.pos += 2
			continue
		}
		p.pos++
		if c == q {
			return b.String(), nil
		}
		b.WriteByte(c)
	}
	return "", p.errf("引号未闭合")
}
