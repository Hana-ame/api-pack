package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"os"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch"
	"github.com/Hana-ame/api-pack/tools/orderedmap"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

func service2key(service string) (string, string) {
	// 这里可以根据 service 的值返回不同的 API 密钥和端点
	switch service {
	case "chutes":
		return os.Getenv("CHUTES_API_TOKEN"), "https://llm.chutes.ai/v1/chat/completions"
	case "chutes-hidream": // text2pic
		return os.Getenv("CHUTES_API_TOKEN"), "https://chutes-hidream.chutes.ai/generate"
	case "chutes-chroma": // text2pic
		return os.Getenv("CHUTES_API_TOKEN"), "https://chutes-chroma.chutes.ai/generate"
	case "chutes-stable-flow": // text2pic
		return os.Getenv("CHUTES_API_TOKEN"), "https://chutes-stable-flow.chutes.ai/generate"
	case "chutes-infiniteyou": // text2pic
		return os.Getenv("CHUTES_API_TOKEN"), "https://chutes-infiniteyou.chutes.ai/generate"
	case "groq":
		return os.Getenv("GROQ_API_KEY"), "https://api.groq.com/openai/v1/chat/completions"
	case "huawei-ds-v3":
		return os.Getenv("HUAWEI_API_KEY"), "https://maas-cn-southwest-2.modelarts-maas.com/v1/infers/271c9332-4aa6-4ff5-95b3-0cf8bd94c394/v1/chat/completions"
	case "huawei-ds-r1":
		return os.Getenv("HUAWEI_API_KEY"), "https://maas-cn-southwest-2.modelarts-maas.com/v1/infers/8a062fd4-7367-4ab4-a936-5eeb8fb821c4/v1/chat/completions"
	default:
		return os.Getenv("GROQ_API_KEY"), "https://api.groq.com/openai/v1/chat/completions"
	}
}

func Message(role string, content any /*string Content*/) *orderedmap.OrderedMap {
	m := orderedmap.New()
	m.Set("role", role)
	m.Set("content", content)
	return m
}

// type Message struct {
// Role    string    `json:"role"` // "system", "user", or "assistant
// Content []Content `json:"content"`
// }

func Content(typ, text, imgUrl string) *orderedmap.OrderedMap {
	c := orderedmap.New()
	c.Set("type", typ)
	if text != "" {
		c.Set("text", text)
	}
	if imgUrl != "" {
		c.Set("image_url", imgUrl)
	}
	return c
}

// type Content struct {
// 	Type     string `json:"type"`                // "text" or "image_url
// 	Text     string `json:"text,omitempty"`      // For text content
// 	ImageURL string `json:"image_url,omitempty"` // For image content
// }

func Request(messages []any, model string) (int, *orderedmap.OrderedMap, error) {
	payload := orderedmap.New()

	payload.Set("model", model)
	payload.Set("messages", messages)
	payload.Set("stop", nil)
	payload.Set("temperature", 0.7)
	payload.Set("max_tokens", 8192)
	payload.Set("top_p", 1)

	apikey, endpoint := service2key("groq")
	resp, o, err := myfetch.FetchJSON(http.MethodPost,
		tools.Or( /*"https://chat.moonchan.xyz/api/echo"*/ "", endpoint),
		http.Header{
			"Authorization": []string{"Bearer " + apikey},
			"Content-Type":  []string{"application/json"}},
		bytes.NewReader(tools.Match(json.Marshal(payload)).Result()))
	if err != nil {
		return 0, nil, err
	}
	// defer resp.Body.Close()
	// fmt.Println(string(tools.Match(io.ReadAll(resp.Body)).Result()))
	return resp.StatusCode, o, err
}

// 兼容bili的那个插件，
func GinHandler(c *gin.Context) {
	//将body marshal为 orderedmap
	o := orderedmap.New()
	if err := json.NewDecoder(c.Request.Body).Decode(o); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
		return
	}

	statusCode, resp, err := Request(o.GetOrDefault("messages", []any{}).([]any), o.GetOrDefault("model", "").(string))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	respStr, _ := json.Marshal(resp)
	fmt.Println(string(respStr))
	c.JSON(statusCode, resp)
}
