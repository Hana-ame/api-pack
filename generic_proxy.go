package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	myfetch "github.com/Hana-ame/api-pack/tools/my_fetch/v2"
	middleware "github.com/Hana-ame/api-pack/tools/my_gin_middleware"
	tools "github.com/Hana-ame/api-pack/tools/utils"
	"github.com/gin-gonic/gin"
)

// ProxyConfig defines the configuration for a single proxy target
type ProxyConfig struct {
	Name          string
	Endpoint      string
	APIKey        string
	AllowedModels map[string]bool
	Models        map[string]ModelInfo
	CustomHeaders map[string]string
}

type ModelInfo struct {
	Name string
}

// GenericProxyHandler returns a gin handler that proxies requests based on the provided config
func GenericProxyHandler(config ProxyConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "failed to read request body",
				"headers": c.Request.Header,
			})
			return
		}
		c.Request.Body.Close()

		// Model validation if AllowedModels is specified
		if len(config.AllowedModels) > 0 {
			reqMap, err := tools.ReaderToJSON(bytes.NewReader(bodyBytes))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "invalid JSON",
					"headers": c.Request.Header,
				})
				return
			}

			model, ok := reqMap.GetOrDefault("model", "").(string)
			if !ok || model == "" {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "model field is required",
					"headers": c.Request.Header,
				})
				return
			}

			if !config.AllowedModels[model] {
				c.JSON(http.StatusForbidden, gin.H{"error": "model not allowed"})
				return
			}
		}

		// Setup headers
		headers := tools.NewHeader(c.Request.Header)
		auth := headers.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			if config.APIKey != "" {
				headers.Set("Authorization", "Bearer "+config.APIKey)
			}
		}
		headers.Set("Content-Type", "application/json")

		for k, v := range config.CustomHeaders {
			headers.Set(k, v)
		}

		// Forward request
		targetURL := config.Endpoint + c.Request.URL.Path
		if targetURL == config.Endpoint {
			// Handle cases where Endpoint might not have trailing slash and path is empty
		} else if !strings.HasSuffix(config.Endpoint, "/") && !strings.HasPrefix(c.Request.URL.Path, "/") {
			targetURL = config.Endpoint + "/" + c.Request.URL.Path
		}

		resp, err := myfetch.Fetch(
			c.Request.Method,
			targetURL,
			headers.Header,
			bytes.NewReader(bodyBytes),
		)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "upstream request failed: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		// Return response
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read upstream response body"})
			return
		}

		if resp.StatusCode != http.StatusOK {
			var jsonBody interface{}
			if err := json.Unmarshal(respBody, &jsonBody); err == nil {
				c.JSON(resp.StatusCode, gin.H{
					"body":    jsonBody,
					"headers": resp.Header,
				})
				return
			}
		}

		tools.PatchHeader(c, resp.Header)
		c.DataFromReader(
			resp.StatusCode,
			int64(len(respBody)),
			resp.Header.Get("Content-Type"),
			bytes.NewReader(respBody),
			map[string]string{"X-Service": config.Name},
		)
	}
}

// RunProxyRouter starts a gin server that proxies all requests to the provided config
func RunProxyRouter(addr string, config ProxyConfig) {
	if addr == "" {
		return
	}

	r := gin.Default()
	r.Use(middleware.CORSMiddleware())

	r.Any("/*any", GenericProxyHandler(config))

	r.Run(addr)
}
