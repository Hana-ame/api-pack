package main

import (
	"net/http"
	"os"

	tools "github.com/Hana-ame/api-pack/Tools"
	myfetch "github.com/Hana-ame/api-pack/Tools/my_fetch"
	middleware "github.com/Hana-ame/api-pack/Tools/my_gin_middleware"
	"github.com/gin-gonic/gin"
)

func service2key(service string) (string, string) {
	// 这里可以根据 service 的值返回不同的 API 密钥和端点
	switch service {
	case "chutes":
		return os.Getenv("CHUTES_API_TOKEN"), "https://llm.chutes.ai/v1/chat/completions"
	case "chutes-hidream":
		return os.Getenv("CHUTES_API_TOKEN"), "https://chutes-hidream.chutes.ai/generate"
	case "chutes-chroma":
		return os.Getenv("CHUTES_API_TOKEN"), "https://chutes-chroma.chutes.ai/generate"
	case "chutes-stable-flow":
		return os.Getenv("CHUTES_API_TOKEN"), "https://chutes-stable-flow.chutes.ai/generate"
	case "chutes-infiniteyou":
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

func OpenaiProxy(addr string) {

	r := gin.Default()

	// 设置 CORS 头
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.ProxyMiddleware())

	// 定义一个简单的 GET 路由
	r.Any("/*any", func(c *gin.Context) {
		if c.Request.Body != nil {
			defer c.Request.Body.Close()
		}
		service := tools.Or(c.GetHeader("X-Service"), c.Query("service"))
		apikey, endpoint := (service2key(service))
		// 添加需要的APIKEY
		headers := tools.NewHeader(c.Request.Header)
		headers.Add("Authorization", "Bearer "+apikey)
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
		tools.CopyHeader(c, resp.Header)

		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, map[string]string{
			"X-Service": tools.Or(service, "groq"),
		})

	})

	// 启动服务器
	r.Run(addr) // 在 8080 端口启动服务

}
