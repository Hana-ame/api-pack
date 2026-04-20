package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
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
	FreeModels    map[string]bool
	FreeModelsAll bool
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

		var model string
		if len(config.FreeModels) > 0 {
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
		}

		// Setup headers
		headers := tools.NewHeader(c.Request.Header)
		auth := headers.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			if config.APIKey != "" && (config.FreeModelsAll || config.FreeModels[model]) {
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

		// DELETE all cf-* headers
		for k := range resp.Header {
			if strings.HasPrefix(strings.ToLower(k), "cf-") {
				resp.Header.Del(k)
			}
		}

		// Return response
		if resp.StatusCode != http.StatusOK {
			reader, err := myfetch.ResponseToReader(resp)
			if err != nil {
				reader = resp.Body
			}
			respBody, err := io.ReadAll(reader)
			if err == nil {
				// Sort and format request headers
				var reqHeaderLines []string
				for k, v := range headers.Header {
					reqHeaderLines = append(reqHeaderLines, fmt.Sprintf("%s: %s", k, strings.Join(v, ", ")))
				}
				sort.Strings(reqHeaderLines)
				reqHeaders := strings.Join(reqHeaderLines, "\n")

				// Sort and format response headers
				var respHeaderLines []string
				for k, v := range resp.Header {
					respHeaderLines = append(respHeaderLines, fmt.Sprintf("%s: %s", k, strings.Join(v, ", ")))
				}
				sort.Strings(respHeaderLines)
				respHeaders := strings.Join(respHeaderLines, "\n")

				c.Header("Content-Type", "text/plain; charset=utf-8")
				c.String(resp.StatusCode, fmt.Sprintf("[Request Headers]\n%s\n\n[Response Headers]\n%s\n\n[Body]\n%s", reqHeaders, respHeaders, string(respBody)))
				return
			}
		}

		tools.PatchHeader(c, resp.Header)
		c.DataFromReader(
			resp.StatusCode,
			resp.ContentLength,
			resp.Header.Get("Content-Type"),
			resp.Body,
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
