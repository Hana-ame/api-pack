package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	tools "github.com/Hana-ame/api-pack/Tools"
	"github.com/Hana-ame/api-pack/Tools/debug"
	myfetch "github.com/Hana-ame/api-pack/Tools/my_fetch"
	streams "github.com/Hana-ame/api-pack/Tools/my_streams"
	"github.com/antchfx/htmlquery"
)

func TestXxx(t *testing.T) {
	path := "/s/e8e1afe65b/3184830-1"
	host := "ex.nmbyd1.top"
	// host := "exhentai.org"

	header := tools.NewHeader(nil)
	header.Set(
		"Cookie",
		tools.NewSlice(
			os.Getenv("EXHENTAI_PROXY_COOKIE"),
			"pass=pass",
			"ipb_member_id=5698562; ipb_pass_hash=154e574fd19294c32f905fe187cbdad1; yay=louder; igneous=5eevdxac75hpx71cv",
		).FirstUnequal(""),
	)

	resp, err := myfetch.Fetch(
		"GET", "https://"+host+path,
		(header.Header), nil)
	if err != nil {
		debug.E("why", err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := myfetch.ResponseToReader(resp)
	if err != nil {
		debug.E("why", err.Error())
		return
	}

	data, err := io.ReadAll(body)
	if err != nil {
		debug.E("why", err.Error())
		return
	}
	doc, err := htmlquery.Parse(bytes.NewReader(data))
	if err != nil {
		return
	}

	arr := findAll(doc, "//a", "href")
	fmt.Println(arr)

	gallery, _ := streams.First(arr, func(s string) bool {
		return strings.HasPrefix(s, "/g/")
	})

	fullimg, _ := streams.First(arr, func(s string) bool {
		return strings.HasPrefix(s, "/fullimg")
	})

	fmt.Println(gallery, fullimg)

}

func TestFetchDateOfGallery(t *testing.T) {
	path := "/g/3187006/6f4d002251/"
	host := "ex.nmbyd1.top"
	// host := "exhentai.org"

	header := tools.NewHeader(nil)
	header.Set(
		"Cookie",
		tools.NewSlice(
			os.Getenv("EXHENTAI_PROXY_COOKIE"),
			"pass=pass",
			"ipb_member_id=5698562; ipb_pass_hash=154e574fd19294c32f905fe187cbdad1; yay=louder; igneous=5eevdxac75hpx71cv",
		).FirstUnequal(""),
	)

	resp, err := myfetch.Fetch(
		"GET", "https://"+host+path,
		(header.Header), nil)
	if err != nil {
		debug.E("why", err.Error())
		return
	}
	defer resp.Body.Close()

	body, err := myfetch.ResponseToReader(resp)
	if err != nil {
		debug.E("why", err.Error())
		return
	}

	data, err := io.ReadAll(body)
	if err != nil {
		debug.E("why", err.Error())
		return
	}
	doc, err := htmlquery.Parse(bytes.NewReader(data))
	if err != nil {
		return
	}

	s, _ := findOneAndSelectAttr(doc, `//*[@id="gdd"]/table/tbody/tr[1]/td[2]`, InnerText)
	fmt.Println(s)
}

func TestIs(t *testing.T) {
	a := isWithinThreeMonths("2025-01-07 11:41")
	fmt.Println(a)
}

func TestRedirect(t *testing.T) {
	r, _ := myfetch.Fetch(http.MethodGet, "https://exhentai.org/fullimg/3187006/1/89vy64cac5l/1_1736268765.png", nil, nil)
	fmt.Println(r)
}

func TestSEX(t *testing.T) {
	header := tools.NewHeader(nil)
	header.Set(
		"Cookie",
		tools.NewSlice(
			os.Getenv("EXHENTAI_PROXY_COOKIE"),
			"pass=pass",
			"ipb_member_id=5698562; ipb_pass_hash=154e574fd19294c32f905fe187cbdad1; yay=louder; igneous=5eevdxac75hpx71cv",
		).FirstUnequal(""),
	)
	{
		resp, err := myfetch.Fetch(http.MethodHead, "https://ehgt.org/w/01/751/41751-sbrofetg.webp", header.Header, nil)
		fmt.Println(err)
		fmt.Println(resp)
		for k, v := range resp.Header {
			for _, vv := range v {
				fmt.Println(k, vv)
			}
		}
	}
	{
		resp, err := myfetch.Fetch(http.MethodGet, "https://s.exhentai.org/w/01/751/41751-sbrofetg.webp", header.Header, nil)
		fmt.Println(err)
		fmt.Println(resp)
		for k, v := range resp.Header {
			for _, vv := range v {
				fmt.Println(k, vv)
			}
		}
	}
	// resp, err := myfetch.Fetch(http.MethodGet, "https://s.exhentai.org/w/01/751/41751-sbrofetg.webp", header.Header, nil)
	// fmt.Println(err)
	// fmt.Println(resp)
	//
	//	for k, v := range resp.Header {
	//		for _, vv := range v {
	//			fmt.Println(k, vv)
	//		}
	//	}
}

func TestSEXgin(t *testing.T) {
	SProxy()
}

func TestReferer(t *testing.T) {
	referer := "https://page.moonchan.xyz"
	if tools.Match(url.Parse(referer)).Result().Host == "page.moonchan.xyz" {
		fmt.Println(tools.Match(url.Parse(referer)).Result().Host)
	}
	fmt.Println(tools.Match(url.Parse(referer)).Result().Host)

}
