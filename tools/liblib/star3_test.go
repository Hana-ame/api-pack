package liblib

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestText2Image(t *testing.T) {
	s, e := Text2Image("1 girl,lotus leaf,masterpiece,best quality,finely detail,highres,8k,beautiful and aesthetic,no watermark",
		768, 1024, 1, 30, nil)
	fmt.Println(e)
	fmt.Println(s)
}

func TestStatus(t *testing.T) {
	id := "2263b04dc2094c289c5b3d00262e46cd"
	id = "22daefe2653b4c20812921cebdc4891a"
	// id = "ad8c3ca181f84aaabe3ada1ada72f1a8"
	id = "e114bf066fe84928b1db8e82101add71"
	o, e := Status(id)
	// fmt.Println(o)
	fmt.Println(e)
	b, _ := json.MarshalIndent(o, "", "  ")
	fmt.Println(string(b))
}

func TestImg2Img(t *testing.T) {
	o, e := Image2Image("1 girl,cat girl,masterpiece,best quality,finely detail,highres,8k,beautiful and aesthetic,no watermark",
		"https://upload.moonchan.xyz/api/01LLWEUU7HP6AU23DCMBDJP6OJT5MVEQOO/56_169620_4ea6a18fe74c0ef.jpg",
		512, 512, 1, 30, nil)
	fmt.Println(e)
	b, _ := json.MarshalIndent(o, "", "  ")
	fmt.Println(string(b))
}
