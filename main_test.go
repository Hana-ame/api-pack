package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUrlValues(t *testing.T) {
	v := make(url.Values)
	s := v.Encode()
	fmt.Println(s, len(v)) // ""
	v.Add("key", "value")
	s = v.Encode()
	fmt.Println(s, len(v)) // "key=value"
	v.Add("key2", "value2")
	s = v.Encode()
	fmt.Println(s, len(v)) // "key=value&key2=value2"
	v.Del("key2")
	s = v.Encode()
	fmt.Println(s, len(v)) // "key=value&key2=value2"
}

func TestParamsDecoding(t *testing.T) {
	// 测试 URL-safe base64 编码的 JSON 参数解析
	params := ProxyParams{
		Host:    "example.com",
		Referer: "https://referrer.com",
		Scheme:  "http",
		Origin:  "https://origin.com",
	}

	jsonBytes, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// 使用 URL-safe base64 编码（不带填充）
	base64Str := base64.RawURLEncoding.EncodeToString(jsonBytes)
	fmt.Printf("URL-safe Base64 encoded params: %s\n", base64Str)

	// 测试解码（不带填充）
	decoded, err := base64.RawURLEncoding.DecodeString(base64Str)
	if err != nil {
		t.Fatalf("URL-safe Base64 decode failed: %v", err)
	}

	var decodedParams ProxyParams
	if err := json.Unmarshal(decoded, &decodedParams); err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	if decodedParams.Host != params.Host {
		t.Errorf("Expected host %s, got %s", params.Host, decodedParams.Host)
	}
	if decodedParams.Referer != params.Referer {
		t.Errorf("Expected referer %s, got %s", params.Referer, decodedParams.Referer)
	}
	if decodedParams.Scheme != params.Scheme {
		t.Errorf("Expected scheme %s, got %s", params.Scheme, decodedParams.Scheme)
	}
	if decodedParams.Origin != params.Origin {
		t.Errorf("Expected origin %s, got %s", params.Origin, decodedParams.Origin)
	}
}

func TestRootProxyHandlerWithParams(t *testing.T) {
	// 创建一个测试请求，使用 params 参数
	params := ProxyParams{
		Host:    "test.example.com",
		Referer: "https://test-referrer.com",
		Scheme:  "https",
		Origin:  "https://test-origin.com",
	}

	jsonBytes, _ := json.Marshal(params)
	// 使用 URL-safe base64 编码（不带填充）
	base64Str := base64.RawURLEncoding.EncodeToString(jsonBytes)

	// 创建测试请求
	req := httptest.NewRequest("GET", "/test/path?params="+base64Str+"&other=value", nil)
	w := httptest.NewRecorder()

	// 这里我们只验证参数能被正确解析，不实际执行代理请求
	// 因为需要完整的 gin.Context 和依赖
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	// 验证 params 参数存在
	paramsQuery := c.Query("params")
	if paramsQuery == "" {
		t.Error("params query parameter should exist")
	}

	// 验证解码（URL-safe）
	decoded, err := base64.RawURLEncoding.DecodeString(paramsQuery)
	if err != nil {
		t.Fatalf("Failed to decode params: %v", err)
	}

	var decodedParams ProxyParams
	if err := json.Unmarshal(decoded, &decodedParams); err != nil {
		t.Fatalf("Failed to unmarshal params: %v", err)
	}

	if decodedParams.Host != "test.example.com" {
		t.Errorf("Expected host test.example.com, got %s", decodedParams.Host)
	}
}
