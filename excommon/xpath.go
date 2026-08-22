// Package excommon 提供 exhentai / exhentai_modify / exhentai_stream
// 三个变体共用的辅助工具（XPath 取值、HTML 内容替换）。
// 三个包原本各持有一份逐字节相同的拷贝，现统一收敛到这里。
package excommon

import (
	"errors"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

// toolkik: xpath

const InnerText string = "INNER_TEXT"

func FindOneAndSelectAttr(top *html.Node, expr string, name string) (v string, err error) {
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

func FindAll(top *html.Node, expr, name string) (v []string) {
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
