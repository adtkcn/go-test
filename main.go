package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// 每个请求配置的结果结构体
type Result struct {
	URL             string
	Method          string
	TotalRequests   int64
	SuccessRequests int64
	TotalTime       time.Duration
	MaxTime         time.Duration
	AvgTime         time.Duration
	RequestsTimes   []time.Duration
	TimeOut         int64
	ErrorCodes      map[int]int
}

func main() {
	// 命令行参数解析
	concurrency := flag.Int64("c", 100, "并发数")
	totalRequests := flag.Int64("n", 1000, "总请求数")
	configFile := flag.String("f", "config.json", "URL配置文件路径")
	timeout := flag.Int64("t", 20, "超时时间")
	flag.Parse()

	// 读取配置文件
	requestList, err := ReadConfig(*configFile)
	if err != nil {
		fmt.Printf("读取配置文件%s失败: %v\n", *configFile, err)
		return
	}

	if len(requestList) == 0 {
		fmt.Println("配置文件中未找到请求配置")
		return
	}

	// 运行压力测试
	results := runTest(requestList, *concurrency, *totalRequests, *timeout)

	// 计算并显示结果
	showResult(results)
}

// 运行压力测试
func runTest(requestList []RequestConfig, concurrency, totalRequests, timeout int64) []Result {
	var results []Result

	// 顺序处理每个请求配置
	for index, request := range requestList {
		fmt.Printf("开始测试请求配置 #%d: %s %s\n", index+1, request.Method, request.URL)
		reqResult := runSingleConfigTest(request, concurrency, totalRequests, timeout)
		results = append(results, reqResult)
		fmt.Printf("测试完成 #%d: 总请求数=%d, 成功数=%d, 总耗时=%v\n\n", index+1, reqResult.TotalRequests, reqResult.SuccessRequests, reqResult.TotalTime)
	}
	return results
}

// 运行单个请求配置的压力测试
func runSingleConfigTest(request RequestConfig, concurrency, totalRequests, timeout int64) Result {
	var wg sync.WaitGroup
	var mu sync.Mutex

	progressStep := int64(totalRequests / 10)
	if progressStep < 1 {
		progressStep = 1
	}
	result := Result{
		URL:        request.URL,
		Method:     request.Method,
		ErrorCodes: make(map[int]int),
	}

	// 初始化请求处理器
	handler := NewRequestHandler(time.Duration(timeout) * time.Second)

	// startTime := time.Now()
	requestChan := make(chan struct{}, totalRequests)

	// 填充请求通道
	for i := int64(0); i < totalRequests; i++ {
		requestChan <- struct{}{}
	}
	close(requestChan)

	totalStartTime := time.Now()
	// 创建工作协程
	for i := int64(0); i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range requestChan {
				reqStartTime := time.Now()
				// 创建请求
				// var req *http.Request
				// var err error

				// 使用请求处理器构建请求
				req, err := handler.BuildRequest(request)
				if err != nil {
					fmt.Printf("%v\n", err)
					continue
				}

				// 发送请求
				resp, err := handler.client.Do(req)
				elapsed := time.Since(reqStartTime) // 请求耗时
				mu.Lock()
				result.TotalRequests += 1
				mu.Unlock()

				// 处理错误情况
				if err != nil {
					// 判断超时
					if err, ok := err.(net.Error); ok && err.Timeout() {
						mu.Lock()
						result.TimeOut++
						mu.Unlock()
					} else {
						fmt.Printf("请求错误: %v, URL: %s\n", err, req.URL)
					}

				} else {
					// 确保响应体被读取和关闭，io.Discard 丢弃响应体内容
					// io.Copy(io.Discard, resp.Body)
					// 读取并打印内容
					_, err := io.ReadAll(resp.Body)
					if err != nil {
						fmt.Printf("读取响应体错误: %v, URL: %s\n", err, req.URL)
					}
					// body = []byte{}
					// fmt.Printf("响应体长度: %d\n，前50个字符: %s\n", len(body), string(body[:50]))

					resp.Body.Close()

					if resp.StatusCode == http.StatusOK {
						mu.Lock()
						result.RequestsTimes = append(result.RequestsTimes, elapsed)
						result.SuccessRequests += 1
						mu.Unlock()
						// atomic.AddInt64(&result.SuccessRequests, 1)
					} else if resp.StatusCode != http.StatusOK {
						errCode := resp.StatusCode
						mu.Lock()
						result.ErrorCodes[errCode]++
						mu.Unlock()
						fmt.Printf("请求状态码错误: %d, URL: %s\n", resp.StatusCode, req.URL)
					}
				}
				// 检查是否达到进度报告点
				if result.TotalRequests%progressStep == 0 || result.TotalRequests == (totalRequests) {
					fmt.Printf("%s, 成功 %d,超时 %d, 进度: %d/%d\n", req.URL, result.SuccessRequests, result.TimeOut, result.TotalRequests, totalRequests)
				}
			}
		}()
	}

	wg.Wait()
	result.TotalTime = time.Since(totalStartTime)
	result.AvgTime = average(result.RequestsTimes)
	result.MaxTime = maxDuration(result.RequestsTimes)

	return result
}

// 显示测试结果
func showResult(results []Result) {
	// 显示每个请求配置的单独结果
	for index, reqResult := range results {
		fmt.Printf("====== 请求配置 #%d 结果 ======\n", index+1)

		// fmt.Printf("平均DNS时间: %v\n", average(reqResult.DNSTimes))
		// fmt.Printf("最大DNS时间: %v\n", maxDuration(reqResult.DNSTimes))

		fmt.Printf("【%s】URL: %s\n", reqResult.Method, reqResult.URL)

		fmt.Printf("总请求数: %d，成功数: %d, 超时 %d, 失败数: %d, 成功率: %.2f%%\n", reqResult.TotalRequests, reqResult.SuccessRequests, reqResult.TimeOut, reqResult.TotalRequests-reqResult.SuccessRequests, float64(reqResult.SuccessRequests)/float64(reqResult.TotalRequests)*100)
		// fmt.Printf("成功率: %.2f%%\n", float64(reqResult.SuccessRequests)/float64(reqResult.TotalRequests)*100)
		fmt.Printf("总耗时: %v，最大耗时: %v, 平均耗时: %v (超时不计入)\n", reqResult.TotalTime, reqResult.MaxTime, reqResult.AvgTime)

		if len(reqResult.ErrorCodes) > 0 {
			fmt.Printf("错误码: %+v\n", reqResult.ErrorCodes)
		}
		fmt.Printf("Req/s: %+v\n\n", float64(reqResult.TotalRequests)/reqResult.TotalTime.Seconds())
		// 耗时分布统计
		maxMs := int(reqResult.MaxTime.Milliseconds())
		interval := 100
		maxInterval := maxMs/interval + 1
		distribution := make([]int, maxInterval)

		for _, d := range reqResult.RequestsTimes {
			ms := int(d.Milliseconds())
			index := ms / interval
			if index >= maxInterval {
				index = maxInterval - 1
			}
			distribution[index]++
		}

		// 打印耗时分布
		fmt.Printf("每%dms耗时统计次数:\n", interval)
		for i := 0; i < maxInterval; i++ {
			start := i * interval
			end := (i+1)*interval - 1
			if distribution[i] == 0 {
				continue
			}
			if i == maxInterval-1 {
				fmt.Printf("%dms+: %d次\n", start, distribution[i])
			} else {
				fmt.Printf("%d-%dms: %d次\n", start, end, distribution[i])
			}
		}

		// fmt.Printf("响应时间: %+v\n", reqResult.RequestsTimes)
		fmt.Printf("\n")
	}
}

func average(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return time.Duration(0)
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func maxDuration(durations []time.Duration) time.Duration {
	max := time.Duration(0)
	for _, d := range durations {
		if d > max {
			max = d
		}
	}
	return max
}
