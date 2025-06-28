package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// 请求配置结构体，用于从JSON文件读取请求信息
type RequestConfig struct {
	URL     string                 `json:"url"`
	Method  string                 `json:"method,omitempty"`
	Params  map[string]interface{} `json:"params,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Headers map[string]string      `json:"headers,omitempty"`
}

// RequestHandler 请求处理器结构体
type RequestHandler struct {
	client         *http.Client
	defaultHeaders map[string]string
}

// NewRequestHandler 创建新的请求处理器
func NewRequestHandler(timeout time.Duration) *RequestHandler {
	return &RequestHandler{
		client: &http.Client{
			Timeout: timeout,
		},
		defaultHeaders: map[string]string{
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
		},
	}
}

// BuildRequest
func (h *RequestHandler) BuildRequest(config RequestConfig) (*http.Request, error) {
	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, fmt.Errorf("URL解析错误: %v", err)
	}

	h.processURLParams(parsedURL, config.Params)
	reqBody, err := h.createRequestBody(config.Data)
	if err != nil {
		return nil, err
	}

	method := h.getMethod(config.Method)
	req, err := http.NewRequest(method, parsedURL.String(), reqBody)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	h.setRequestHeaders(req, config.Headers)
	return req, nil
}

func (h *RequestHandler) processURLParams(u *url.URL, params map[string]interface{}) {
	if params != nil {
		query := u.Query()
		for key, value := range params {
			query.Add(key, fmt.Sprintf("%v", value))
		}
		u.RawQuery = query.Encode()
	}
}

func (h *RequestHandler) createRequestBody(data map[string]interface{}) (io.Reader, error) {
	if data == nil {
		return nil, nil
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("数据序列化错误: %v", err)
	}
	return bytes.NewBuffer(dataBytes), nil
}

func (h *RequestHandler) getMethod(method string) string {
	if method == "" {
		return "GET"
	}
	return method
}

func (h *RequestHandler) setRequestHeaders(req *http.Request, headers map[string]string) {
	for k, v := range h.defaultHeaders {
		if _, exists := headers[k]; !exists {
			req.Header.Set(k, v)
		}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
}

// 读取JSON配置文件
func ReadConfig(filePath string) ([]RequestConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var requestList []RequestConfig
	if err := json.Unmarshal(data, &requestList); err != nil {
		return nil, err
	}

	return requestList, nil
}
