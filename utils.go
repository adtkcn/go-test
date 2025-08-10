package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Response struct {
	Status int                    `json:"status"`
	Data   map[string]interface{} `json:"field"`
}

// 请求配置结构体，用于从JSON文件读取请求信息
type RequestConfig struct {
	URL      string                 `json:"url"`
	Method   string                 `json:"method,omitempty"`
	Params   map[string]interface{} `json:"params,omitempty"`
	Data     any                    `json:"data,omitempty"`
	Headers  map[string]string      `json:"headers,omitempty"`
	Response Response               `json:"response"`
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
func (h *RequestHandler) NewRequest(config RequestConfig) (*http.Response, *http.Client, error) {
	parsedURL, err := url.Parse(config.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("URL解析错误: %v", err)
	}

	h.processURLParams(parsedURL, config.Params)
	reqBody, err := h.createRequestBody(config.Data)
	if err != nil {
		return nil, nil, err
	}

	method := h.getMethod(config.Method)
	req, err := http.NewRequest(method, parsedURL.String(), reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("创建请求失败: %v", err)
	}

	h.setRequestHeaders(req, config.Headers)

	// 发送请求
	resp, err := h.client.Do(req)

	return resp, h.client, err
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

func (h *RequestHandler) createRequestBody(data any) (io.Reader, error) {
	if data == nil {
		return nil, nil
	}
	// 判断data为字符串
	if str, ok := data.(string); ok {
		return strings.NewReader(str), nil
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
	//取文件名称,是否存在

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("文件不存在: %v", err)
	}

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

func writeFile(filePath string, data []byte) error {
	err := os.WriteFile(filePath, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

// 毫秒大于1000时转秒，带单位ms或者s
func MsToSeconds(ms int64) string {
	if ms > 1000 {
		return fmt.Sprintf("%.3f", float64(ms)/1000) + "s"
	}
	return fmt.Sprintf("%d", ms) + "ms"
}
func average(durations []int64) int64 {
	if len(durations) == 0 {
		return 0
	}

	var total int64
	for _, d := range durations {
		total += d
	}
	return total / int64(len(durations))
}

func maxDuration(durations []int64) int64 {
	max := int64(0)
	for _, d := range durations {
		if d > max {
			max = d
		}
	}
	return max
}
