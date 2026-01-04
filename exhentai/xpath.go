package exhentai

import (
	"bytes"
	"errors"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// toolkik: xpath

const InnerText string = "INNER_TEXT"

func findOneAndSelectAttr(top *html.Node, expr string, name string) (v string, err error) {
	elem := htmlquery.FindOne(top, expr)
	if elem == nil {
		err = errors.New(expr + ":" + name + "is null")
		return
	}
	if name == InnerText {
		v = htmlquery.InnerText(elem)
	} else {
		v = htmlquery.SelectAttr(elem, name)
	}
	return
}

func findAll(top *html.Node, expr, name string) (v []string) {
	elemArray := htmlquery.Find(top, expr)
	v = make([]string, len(elemArray))
	for i, e := range elemArray {
		if name == InnerText {
			v[i] = htmlquery.InnerText(e)
		} else {
			v[i] = htmlquery.SelectAttr(e, name)
		}
	}
	return
}

// InjectExternalScript 统一注入外部脚本
// 该函数替代了原有的 addReloadCoverButton, addWaterFallViewButton, addInlineChatRoom, addFloatingIframeAtRightBottom
// 通过在 </head> 闭合标签前插入 <script src="https://script" defer></script> 实现
func InjectExternalScript(html []byte) []byte {
	// 定义要插入的脚本标签
	const scriptTag = `<script src="https://script" defer></script>`
	const headCloseTag = "</head>"

	// 查找 </head> 标签的位置
	if !bytes.Contains(html, []byte(headCloseTag)) {
		// 如果没有找到 </head>，则尝试插入到 <body> 之前或直接追加（视具体 HTML 结构容错需求而定）
		// 这里选择直接返回原 HTML 或追加到末尾，通常 ExHentai 页面都有 head
		return html
	}

	// 执行替换：将 </head> 替换为 <script ...></script></head>
	// 这样可以确保脚本位于 head 区域内
	return bytes.Replace(html, []byte(headCloseTag), []byte(scriptTag+headCloseTag), 1)
}
