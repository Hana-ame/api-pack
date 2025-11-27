// moved to tools

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	tools "github.com/Hana-ame/api-pack/Tools"
	myfetch "github.com/Hana-ame/api-pack/Tools/my_fetch"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	"github.com/Hana-ame/api-pack/Tools/orderedmap"
	"github.com/gin-gonic/gin"
)

func AuthorizationMiddleWare(c *gin.Context) {
	apiKey := c.GetHeader("Authorization")
	if apiKey != "Barer nanaka" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
}

type Complation struct {
	Model            string    `json:"model,omitempty"`
	MaxTokens        int       `json:"max_tokens,omitempty"`        // ocr, qwen
	Temperature      float32   `json:"temperature,omitempty"`       // ocr, qwen
	TopP             float32   `json:"top_p,omitempty"`             // ocr, qwen
	TopK             int       `json:"top_k,omitempty"`             // ocr, qwen
	MinP             float32   `json:"min_p,omitempty"`             // qwen
	FrequencyPenalty float32   `json:"frequency_penalty,omitempty"` // ocr, qwen
	EnableThinking   bool      `json:"enable_thinking,omitempty"`   // qwen
	ThinkingBudget   int       `json:"thinking_budget,omitempty"`   // qwen
	Messages         []Message `json:"messages,omitempty"`
}

type Message struct {
	Role    string  `json:"role,omitempty"`    // "system", "user", or "assistant"
	Content Content `json:"content,omitempty"` // qwen
}

type StringContent string        // qwen
func (StringContent) isContent() {}

type StructContent struct {
	Type     string     `json:"type,omitempty"`      // ocr: image_url or text
	ImageUrl UrlMessage `json:"image_url,omitempty"` // ocr
	Text     string     `json:"text,omitempty"`      // ocr
}

func (StructContent) isContent() {}

type StructContentSlice []StructContent // ocr
func (StructContentSlice) isContent()   {}

type Content interface {
	isContent()
}

type UrlMessage struct {
	Url string `json:"url,omitempty"` //data:image/png;base64
}

// for test
// func main() {
// 	OpenaiProxy("0.0.0.0:5500")
// }

func service2key(service string) (string, string) {
	// 这里可以根据 service 的值返回不同的 API 密钥和端点
	switch service {
	case "groq":
		return os.Getenv("GROQ_API_KEY"), "https://api.groq.com/openai/v1/chat/completions"
	case "huawei-ds-v3":
		return os.Getenv("HUAWEI_API_KEY"), "https://maas-cn-southwest-2.modelarts-maas.com/v1/infers/271c9332-4aa6-4ff5-95b3-0cf8bd94c394/v1/chat/completions"
	case "huawei-ds-r1":
		return os.Getenv("HUAWEI_API_KEY"), "https://maas-cn-southwest-2.modelarts-maas.com/v1/infers/8a062fd4-7367-4ab4-a936-5eeb8fb821c4/v1/chat/completions"
	case "siliconflow":
		return os.Getenv("SILICONFLOW_API_KEY"), "https://api.siliconflow.cn/v1/chat/completions"
	default:
		return os.Getenv("GROQ_API_KEY"), "https://api.groq.com/openai/v1/chat/completions"
	}
}

// 简化api
func siliconflowDeepseekOCRHandler(c *gin.Context) {
	var requestBody *orderedmap.OrderedMap = nil
	if c.Request.Body != nil {
		defer c.Request.Body.Close()
		var err error
		requestBody, err = tools.ReaderToJSON(c.Request.Body)
		if tools.AbortWithError(c, http.StatusBadRequest, err) {
			return
		}
	}
	apikey, endpoint := service2key("siliconflow")
	imageUrl := requestBody.GetOrDefault("image_url", c.Query("image_url")).(string)
	text := requestBody.GetOrDefault("text", tools.Or(c.Query("prompt"), "free ocr")).(string)
	if imageUrl == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image_url is required"})
		return
	}
	if text == "" { // never..
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}

	payload := &Complation{
		Model:       "deepseek-ai/DeepSeek-OCR",
		MaxTokens:   4096,
		Temperature: 0,
		TopP:        0.7,
		TopK:        50,
		// MinP
		FrequencyPenalty: 0,
		// EnableThinking
		// ThinkingBudget
		Messages: []Message{
			{
				Role: "user",
				Content: StructContentSlice{{
					Type: "image_url",
					ImageUrl: UrlMessage{
						Url: imageUrl,
					},
				}, {
					Type: "text",
					Text: text,
				}},
			},
		},
	}

	payloadReader := bytes.NewReader(tools.Match(json.Marshal(payload)).Result())

	// 构建请求体
	// 添加需要的APIKEY
	headers := tools.NewHeader(c.Request.Header)
	headers.Set("Authorization", "Bearer "+apikey)
	headers.Set("Content-Type", "application/json")

	// 将收到的内容加上Authorization然后发送至endpoint
	resp, err := myfetch.Fetch(
		c.Request.Method, endpoint,
		(headers.Header), payloadReader)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	defer resp.Body.Close()

	// 必须有，不然会乱码
	tools.PatchHeader(c, resp.Header)

	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
		"X-Service": "siliconflow",
		"X-Model":   "deepseek-ai/DeepSeek-OCR",
	})

}

// 简化api
func siliconflowGLM49B0414TranslationHandler(c *gin.Context) {
	var requestBody []byte
	if c.Request.Body != nil {
		defer c.Request.Body.Close()
		var err error
		requestBody, err = io.ReadAll(c.Request.Body)
		if tools.AbortWithError(c, http.StatusBadRequest, err) {
			return
		}
	}
	apikey, endpoint := service2key("siliconflow")
	text := tools.Or(string(requestBody), c.Query("text"))
	if text == "" { // never..
		c.JSON(http.StatusBadRequest, gin.H{"error": "body is required"})
		return
	}

	payload := &Complation{
		Model:       "THUDM/GLM-4-9B-0414",
		MaxTokens:   4096,
		Temperature: 0.95,
		TopP:        0.7,
		TopK:        50,
		// MinP
		FrequencyPenalty: 0,
		// EnableThinking
		// ThinkingBudget
		Messages: []Message{
			{
				Role:    "system",
				Content: StringContent("as a pro translator, translate the paragraph below into Chinese. Please don't translate any names."),
			}, {
				Role:    "user",
				Content: StringContent(text),
			},
		},
	}

	payloadReader := bytes.NewReader(tools.Match(json.Marshal(payload)).Result())

	// 构建请求体
	// 添加需要的APIKEY
	headers := tools.NewHeader(c.Request.Header)
	headers.Set("Authorization", "Bearer "+apikey)
	headers.Set("Content-Type", "application/json")

	// 将收到的内容加上Authorization然后发送至endpoint
	resp, err := myfetch.Fetch(
		c.Request.Method, endpoint,
		(headers.Header), payloadReader)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	defer resp.Body.Close()

	// 必须有，不然会乱码
	tools.PatchHeader(c, resp.Header)

	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
		"X-Service": "siliconflow",
		"X-Model":   "THUDM/GLM-4-9B-0414",
	})

}

// 简化api
func siliconflowQwen257BTranslationHandler(c *gin.Context) {
	var requestBody []byte
	if c.Request.Body != nil {
		defer c.Request.Body.Close()
		var err error
		requestBody, err = io.ReadAll(c.Request.Body)
		if tools.AbortWithError(c, http.StatusBadRequest, err) {
			return
		}
	}
	apikey, endpoint := service2key("siliconflow")
	text := tools.Or(string(requestBody), c.Query("text"))
	if text == "" { // never..
		c.JSON(http.StatusBadRequest, gin.H{"error": "body is required"})
		return
	}

	payload := &Complation{
		Model:       "Qwen/Qwen2.5-7B-Instruct",
		MaxTokens:   2048,
		Temperature: 0,
		TopP:        0.1,
		TopK:        1,
		// MinP
		FrequencyPenalty: 0,
		// EnableThinking
		// ThinkingBudget
		Messages: []Message{
			{
				Role:    "system",
				Content: StringContent("作为专业翻译，将用户输入的文本翻译为中文。\n请不要进行思考，直接输出翻译。"),
			}, {
				Role:    "user",
				Content: StringContent(text),
			},
		},
	}

	payloadReader := bytes.NewReader(tools.Match(json.Marshal(payload)).Result())

	// 构建请求体
	// 添加需要的APIKEY
	headers := tools.NewHeader(c.Request.Header)
	headers.Set("Authorization", "Bearer "+apikey)
	headers.Set("Content-Type", "application/json")

	// 将收到的内容加上Authorization然后发送至endpoint
	resp, err := myfetch.Fetch(
		c.Request.Method, endpoint,
		(headers.Header), payloadReader)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	defer resp.Body.Close()

	// 必须有，不然会乱码
	tools.PatchHeader(c, resp.Header)

	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
		"X-Service": "siliconflow",
		"X-Model":   "qwen2.5-7b-instruct",
	})

}

func proxyHandler(c *gin.Context) {
	if c.Request.Body != nil {
		defer c.Request.Body.Close()
	}
	service := tools.Or(c.Param("service"), c.GetHeader("X-Service"), c.Query("service"))
	apikey, endpoint := (service2key(service))
	// 添加需要的APIKEY
	headers := tools.NewHeader(c.Request.Header)
	headers.Set("Authorization", "Bearer "+apikey)
	headers.Set("Content-Type", "application/json")

	// 将收到的内容加上Authorization然后发送至endpoint
	resp, err := myfetch.Fetch(
		c.Request.Method, endpoint,
		(headers.Header), c.Request.Body)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}
	defer resp.Body.Close()

	// 必须有，不然会乱码
	tools.PatchHeader(c, resp.Header)

	c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
		"X-Service": tools.Or(service, "groq"),
	})

}

func OpenaiProxy(addr string) {

	r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	r.POST("/:service", AuthorizationMiddleWare, proxyHandler)
	r.POST("/siliconflow/deepseek-ocr", siliconflowDeepseekOCRHandler)
	r.GET("/siliconflow/deepseek-ocr", func(ctx *gin.Context) {
		ctx.Redirect(301, "https://page.moonchan.xyz/?url=https://pastebin.com/raw/Zx8hczQg#markdown-parser")
	})
	r.POST("/siliconflow/qwen2.5-7b-Instruct/translate", siliconflowQwen257BTranslationHandler)
	r.GET("/siliconflow/qwen2.5-7b-Instruct/translate", func(ctx *gin.Context) {
		ctx.Redirect(301, "https://page.moonchan.xyz/?url=https://pastebin.com/raw/AaPVAhXG#markdown-parser")
	})
	r.POST("/siliconflow/GLM-4-9B-0414/translate", siliconflowGLM49B0414TranslationHandler)
	r.GET("/siliconflow/GLM-4-9B-0414/translate", func(ctx *gin.Context) {
		ctx.Redirect(301, "https://page.moonchan.xyz/?url=https://pastebin.com/raw/AaPVAhXG#markdown-parser")
	})

	r.NoRoute(proxyHandler)

	// 启动服务器
	r.Run(addr) // chat.moonchan.xyz

}

// 删除了一些配置使得能适配 groq 和 沉浸式翻译
// 运行在"127.25.11.6:8080" helper.moonchan.xyz
func OpenaiProxyAlt(addr string) {
	r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	r.NoRoute(func(c *gin.Context) {
		var body *orderedmap.OrderedMap = nil
		if c.Request.Body != nil {
			defer c.Request.Body.Close()
			var err error
			body, err = tools.ReaderToJSON(c.Request.Body)
			if tools.AbortWithError(c, http.StatusBadRequest, err) {
				return
			}
		}
		service := tools.Or(c.GetHeader("X-Service"), c.Query("service"))
		apikey, endpoint := (service2key(service))
		// 添加需要的APIKEY
		headers := tools.NewHeader(c.Request.Header)
		headers.Set("Authorization", "Bearer "+apikey)
		headers.Set("Content-Type", "application/json")

		if body != nil {
			body.Delete("chat_template_kwargs")
			body.Delete("enable_thinking")
		}

		var jsonBody []byte
		if body != nil {
			var err error
			jsonBody, err = json.Marshal(body)
			if tools.AbortWithError(c, http.StatusBadRequest, err) {
				return
			}
		}

		log.Printf("%s\n", string(jsonBody))

		// 将收到的内容加上Authorization然后发送至endpoint
		resp, err := myfetch.Fetch(
			c.Request.Method, endpoint,
			(headers.Header), bytes.NewReader(jsonBody))
		if err != nil {
			c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		defer resp.Body.Close()

		// 必须有，不然会乱码
		tools.PatchHeader(c, resp.Header)

		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
			"X-Service": tools.Or(service, "groq"),
		})

	})

	// 启动服务器
	r.Run(addr) // 在 8080 端口启动服务
}
