package mastodonclient

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch"
	"github.com/Hana-ame/api-pack/tools/orderedmap"
)

type Client struct {
	Host          string
	Cookie        string
	Authorization string
}

func (c *Client) Upload(reader io.Reader) (id, url string, err error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	// writer.WriteField("username", "user1") // 添加普通表单字段
	part, err := writer.CreateFormFile("file", "file")
	if err != nil {
		return
	}
	_, err = io.Copy(part, reader)
	if err != nil {
		return
	}
	err = writer.Close()
	if err != nil {
		return
	}

	_, response, err := myfetch.FetchJSON(http.MethodPost, "https://"+c.Host+"/api/v1/media", http.Header{
		"User-Agent":    []string{"mastodon-client/1.0"},
		"Cookie":        []string{c.Cookie},
		"Authorization": []string{"Bearer " + c.Authorization},
		"Content-Type":  []string{writer.FormDataContentType()},
	}, body)

	return response.GetOrDefault("id", "").(string), response.GetOrDefault("url", "").(string), err
}

//	{
//	    "status": "wow",
//	    "in_reply_to_id": null,
//	    "media_ids": [
//	        "114839922529179944"
//	    ],
//	    "sensitive": false,
//	    "spoiler_text": "",
//	    "visibility": "private",
//	    "poll": null,
//	    "language": "zh"
//	}
//
// 上传文件并发送post
func (c *Client) PostStatus(status string, in_reply_to_id string, mediaIDs []string, sensitive bool, spoiler_text string, visibility string, poll any, language string) (err error) {
	o := orderedmap.New()
	o.Set("status", status)
	if in_reply_to_id != "" {
		o.Set("in_reply_to_id", nil)
	} else {
		o.Set("in_reply_to_id", in_reply_to_id)
	}
	o.Set("media_ids", mediaIDs)
	o.Set("sensitive", sensitive)
	o.Set("spoiler_text", spoiler_text)
	o.Set("visibility", visibility)
	o.Set("poll", poll)
	o.Set("language", language)

	body, err := json.Marshal(o)
	if err != nil {
		return
	}

	_, _, err = myfetch.FetchJSON(http.MethodPost, "https://"+c.Host+"/api/v1/statuses", http.Header{
		"User-Agent":    []string{"mastodon-client/1.0"},
		"Cookie":        []string{c.Cookie},
		"Authorization": []string{"Bearer " + c.Authorization},
		"Content-Type":  []string{"application/json; charset=utf-8"},
	}, bytes.NewReader(body))

	return
}
