package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/cheggaaa/pb/v3"
)

// 每个请求配置的结果结构体
type Result struct {
	RequestConfig RequestConfig
	// URL               string
	// Method            string
	TotalRequests     int64
	SuccessRequests   int64
	TotalTime         int64
	MaxTime           int64
	AvgTime           int64
	RequestsTimes     []int64
	RequestTimeoutNum int64
	ErrorCodes        map[int]int
	ErrorMessages     map[string]int
}

var debug bool
var configFileName string

func main() {
	// 命令行参数解析
	concurrency := flag.Int64("c", 100, "并发数")
	totalRequests := flag.Int64("n", 1000, "总请求数")
	configFile := flag.String("f", "config.json", "URL配置文件路径")
	timeout := flag.Int64("t", 20, "超时时间")
	isDebug := flag.Bool("d", false, "是否开启调试模式")
	flag.Parse()
	debug = *isDebug
	configFileName = filepath.Base(*configFile)
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
		fmt.Printf("测试完成 #%d: 总请求数=%d, 成功数=%d, 总耗时=%vms\n\n", index+1, reqResult.TotalRequests, reqResult.SuccessRequests, reqResult.TotalTime)
	}
	return results
}

// 运行单个请求配置的压力测试
func runSingleConfigTest(request RequestConfig, concurrency, totalRequests, timeout int64) Result {
	var wg sync.WaitGroup
	var mu sync.Mutex

	result := Result{
		RequestConfig: request,
		ErrorCodes:    make(map[int]int),
		ErrorMessages: make(map[string]int),
	}

	// 初始化请求处理器
	handler := NewRequestHandler(time.Duration(timeout) * time.Second)

	// startTime := time.Now()
	requestChan := make(chan struct{}, totalRequests)

	// 填充请求通道
	for range totalRequests {
		requestChan <- struct{}{}
	}
	close(requestChan)

	bar := pb.StartNew(int(totalRequests))
	totalStartTime := time.Now()
	// 创建工作协程
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range requestChan {
				reqStartTime := time.Now()
				// 使用请求处理器构建请求
				resp, _, err := handler.NewRequest(request)
				if err != nil {
					// fmt.Printf("%v\n", err)
					mu.Lock()
					result.ErrorMessages[err.Error()]++
					mu.Unlock()
					continue
				}

				// 发送请求
				// resp, err := handler.client.Do(req)

				// 处理错误情况
				if err != nil {
					// 判断超时
					if err, ok := err.(net.Error); ok && err.Timeout() {
						mu.Lock()
						result.RequestTimeoutNum++
						mu.Unlock()
					} else {
						mu.Lock()
						result.ErrorMessages[err.Error()]++
						mu.Unlock()
					}

				} else {
					// 确保响应体被读取和关闭,io.Discard 丢弃响应体内容
					// io.Copy(io.Discard, resp.Body)
					// 读取并打印内容
					body, err := io.ReadAll(resp.Body)
					resp.Body.Close()
					if err != nil {
						mu.Lock()
						result.ErrorMessages[fmt.Sprintf("读取响应体错误: %v", err)]++
						mu.Unlock()
					}
					// body = []byte{}
					if debug {
						fmt.Printf("响应体长度: %d\n响应体内容: %s\n", len(body), string(body[:350]))
					}

					if resp.StatusCode == http.StatusOK {
						elapsed := time.Since(reqStartTime).Milliseconds() // 请求耗时,单位:毫秒

						mu.Lock()
						result.RequestsTimes = append(result.RequestsTimes, elapsed)
						result.SuccessRequests += 1
						mu.Unlock()
						// atomic.AddInt64(&result.SuccessRequests, 1)
					} else if resp.StatusCode != http.StatusOK {
						mu.Lock()
						result.ErrorCodes[resp.StatusCode]++
						result.ErrorMessages[fmt.Sprintf("请求状态码错误: %d", resp.StatusCode)]++
						mu.Unlock()
					}
				}
				mu.Lock()
				result.TotalRequests += 1
				bar.Increment()
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	bar.Finish()
	result.TotalTime = time.Since(totalStartTime).Milliseconds()
	result.AvgTime = average(result.RequestsTimes)
	result.MaxTime = maxDuration(result.RequestsTimes)

	return result
}

// 显示测试结果
func showResult(results []Result) {
	jsonByte, _ := json.MarshalIndent(results, "", "    ")
	writeFile("./result."+configFileName, jsonByte)

	// 显示每个请求配置的单独结果
	for index, reqResult := range results {
		if debug {
			fmt.Printf("请求结果: %#v \n", reqResult)
		}
		fmt.Printf("====== 请求配置 #%d ======\n", index+1)
		fmt.Printf("【URL】[%s]: %s\n", reqResult.RequestConfig.Method, reqResult.RequestConfig.URL)
		fmt.Printf("【QPS】: %.2f\n\n", float64(reqResult.TotalRequests)/float64(reqResult.TotalTime)*1000)

		fmt.Printf("总请求: %d, 成功数: %d, 超时数 %d, 失败数: %d, 成功率: %.2f%%\n", reqResult.TotalRequests, reqResult.SuccessRequests, reqResult.RequestTimeoutNum, reqResult.TotalRequests-reqResult.SuccessRequests, float64(reqResult.SuccessRequests)/float64(reqResult.TotalRequests)*100)
		fmt.Printf("总耗时: %vms, 最大耗时: %vms, 平均耗时: %vms (超时不计入)\n", reqResult.TotalTime, reqResult.MaxTime, reqResult.AvgTime)

		if len(reqResult.ErrorCodes) > 0 {
			fmt.Printf("错误码: %+v\n", reqResult.ErrorCodes)
		}
		if len(reqResult.ErrorMessages) > 0 {
			fmt.Println("错误信息统计:")
			for msg, count := range reqResult.ErrorMessages {
				fmt.Printf("[%d次] %s\n", count, msg)
			}
		}
		fmt.Printf("\n")
		// 耗时分布统计
		maxMs := reqResult.MaxTime
		interval := int64(100)
		maxInterval := maxMs/interval + 1
		distribution := make([]int, maxInterval)

		for _, d := range reqResult.RequestsTimes {
			ms := d
			index := ms / interval
			if index >= maxInterval {
				index = maxInterval - 1
			}
			distribution[index]++
		}

		// 打印耗时分布
		fmt.Printf("每%dms耗时统计次数:\n", interval)
		for i := int64(0); i < maxInterval; i++ {
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
