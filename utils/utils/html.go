package tools

import (
	"errors"

	"github.com/antchfx/htmlquery"
	"golang.org/x/net/html"
)

const InnerText string = "INNER_TEXT"

// expr is xpath
func FindAttr(top *html.Node, xpath string, name string) (v string, err error) {
	elem := htmlquery.FindOne(top, xpath)
	if elem == nil {
		err = errors.New(xpath + ":" + name + "is null")
		return
	}
	if name == InnerText {
		v = htmlquery.InnerText(elem)
	} else {
		v = htmlquery.SelectAttr(elem, name)
	}
	return
}

func FindAll(top *html.Node, xpath, name string) (v Slice[string]) {
	elemArray := htmlquery.Find(top, xpath)
	v = make(Slice[string], len(elemArray))
	for i, e := range elemArray {
		if name == InnerText {
			v[i] = htmlquery.InnerText(e)
		} else {
			v[i] = htmlquery.SelectAttr(e, name)
		}
	}
	return
}
