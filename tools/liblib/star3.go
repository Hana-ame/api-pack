package liblib

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch"
	"github.com/Hana-ame/api-pack/tools/orderedmap"
)

const Accesskey = "6cysi5F8DUg9L8dntQxfSg"
const Secretkey = "WgdHoQ70PstycjjPkH-3smOk2OA1zTrK"

func HmacSHA1(key, data string) []byte {
	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(data))
	return (mac.Sum(nil))
}

func geturl(path string) string {
	timestamp := time.Now().UnixMilli()
	signatureNonce := "testtesttesttest"
	str := fmt.Sprintf("%s&%d&%s", path, timestamp, signatureNonce)
	encoder := base64.URLEncoding.WithPadding(base64.NoPadding)
	signature := encoder.EncodeToString(HmacSHA1(Secretkey, str))
	return fmt.Sprintf("%s?AccessKey=%s&Signature=%s&Timestamp=%d&SignatureNonce=%s", path, Accesskey, signature, timestamp, signatureNonce)
}

const API_HOST = "https://openapi.liblibai.cloud"

func Text2Image(prompt string, width, height int, imgCount, steps int, controlNet *orderedmap.OrderedMap) (*orderedmap.OrderedMap, error) {
	endpoint := API_HOST + geturl("/api/generate/webui/text2img/ultra")

	generateParams := orderedmap.New()
	generateParams.Set("prompt", prompt)
	generateParams.Set("imageSize", map[string]int{"width": width, "height": height})
	generateParams.Set("imgCount", imgCount)
	generateParams.Set("steps", steps)
	if controlNet != nil {
		generateParams.Set("controlNet", controlNet)
	}

	o := orderedmap.New()
	o.Set("templateUuid", "5d7e67009b344550bc1aa6ccbfa1d7f4")
	o.Set("generateParams", generateParams)

	body, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}

	fmt.Println(endpoint)
	fmt.Println(string(body))

	resp, err := myfetch.Fetch(http.MethodPost, endpoint, http.Header{"Content-Type": []string{"application/json"}}, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data := orderedmap.New()
	err = json.NewDecoder(resp.Body).Decode(&data)
	return data, err
}

func Status(generatedUUID string) (*orderedmap.OrderedMap, error) {
	endpoint := API_HOST + geturl("/api/generate/webui/status")

	o := orderedmap.New()
	o.Set("generateUuid", generatedUUID)

	body, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}

	resp, err := myfetch.Fetch(http.MethodPost, endpoint, http.Header{"Content-Type": []string{"application/json"}}, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data := orderedmap.New()
	err = json.NewDecoder(resp.Body).Decode(&data)
	return data, err
}

func Image2Image(
	prompt string,
	sourceImage string,
	width, height int,
	imgCount, steps int,
	controlNet *orderedmap.OrderedMap,
) (*orderedmap.OrderedMap, error) {

	endpoint := API_HOST + geturl("/api/generate/webui/img2img/ultra")

	generateParams := orderedmap.New()
	generateParams.Set("prompt", prompt)
	generateParams.Set("sourceImage", sourceImage)
	generateParams.Set("width", width)
	generateParams.Set("height", height)
	generateParams.Set("imgCount", imgCount)
	generateParams.Set("steps", steps)
	if controlNet != nil {
		generateParams.Set("controlNet", controlNet)
	}

	o := orderedmap.New()
	o.Set("templateUuid", "07e00af4fc464c7ab55ff906f8acf1b7")
	o.Set("generateParams", generateParams)

	body, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}

	fmt.Println(endpoint)
	fmt.Println(string(body))

	resp, err := myfetch.Fetch(http.MethodPost, endpoint, http.Header{"Content-Type": []string{"application/json"}}, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data := orderedmap.New()
	err = json.NewDecoder(resp.Body).Decode(&data)
	return data, err
}
