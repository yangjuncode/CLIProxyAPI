// +build ignore

// 这是一个简单的集成测试，用于测试错误页面功能
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	// 测试错误页面功能
	baseURL := "http://localhost:8080" // 假设服务器运行在8080端口

	// 1. 测试一个会产生错误的请求
	fmt.Println("测试产生错误的请求...")
	testErrorRequest(baseURL)

	// 2. 等待一秒让错误记录被处理
	time.Sleep(1 * time.Second)

	// 3. 测试错误页面
	fmt.Println("\n测试错误页面...")
	testErrorsPage(baseURL)

	// 4. 测试JSON格式的错误页面
	fmt.Println("\n测试JSON格式的错误页面...")
	testErrorsJSON(baseURL)

	// 5. 测试统计信息
	fmt.Println("\n测试错误统计信息...")
	testErrorsStats(baseURL)
}

func testErrorRequest(baseURL string) {
	// 发送一个无效的请求来产生错误
	reqBody := map[string]interface{}{
		"model": "invalid-model",
		"messages": []map[string]string{
			{"role": "user", "content": "hello"},
		},
	}

	jsonBody, _ := json.Marshal(reqBody)
	resp, err := http.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		fmt.Printf("请求失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("状态码: %d\n", resp.StatusCode)
	fmt.Printf("响应: %s\n", string(body))
}

func testErrorsPage(baseURL string) {
	resp, err := http.Get(baseURL + "/errors")
	if err != nil {
		fmt.Printf("获取错误页面失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("错误页面响应状态: %d\n", resp.StatusCode)
	fmt.Printf("错误页面内容:\n%s\n", string(body))
}

func testErrorsJSON(baseURL string) {
	resp, err := http.Get(baseURL + "/errors.json")
	if err != nil {
		fmt.Printf("获取JSON错误页面失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("JSON错误页面响应状态: %d\n", resp.StatusCode)
	fmt.Printf("JSON错误页面内容:\n%s\n", string(body))
}

func testErrorsStats(baseURL string) {
	resp, err := http.Get(baseURL + "/v0/management/errors/stats")
	if err != nil {
		fmt.Printf("获取错误统计失败: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("错误统计响应状态: %d\n", resp.StatusCode)
	fmt.Printf("错误统计内容:\n%s\n", string(body))
}
