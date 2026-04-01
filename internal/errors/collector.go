// Package errors provides error collection functionality for CLIProxyAPI.
// This file contains the error collector that extracts error information from HTTP responses.
package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/interfaces"
)

// ErrorCollector 负责从HTTP请求和响应中收集错误信息
type ErrorCollector struct {
	manager *ErrorHistoryManager
}

// NewErrorCollector 创建新的错误收集器
func NewErrorCollector(manager *ErrorHistoryManager) *ErrorCollector {
	return &ErrorCollector{
		manager: manager,
	}
}

// CollectError 从HTTP上下文中收集错误信息
func (c *ErrorCollector) CollectError(ctx *gin.Context, statusCode int, responseBody []byte) {
	if statusCode < http.StatusBadRequest {
		return // 只记录4xx和5xx错误
	}

	provider := c.extractProvider(ctx)
	model := c.extractModel(ctx, responseBody)
	errorMessage := c.extractErrorMessage(ctx, statusCode, responseBody)
	requestPath := ctx.Request.URL.Path

	c.manager.AddError(provider, model, errorMessage, statusCode, requestPath)
}

// extractProvider 从请求上下文中提取provider信息
func (c *ErrorCollector) extractProvider(ctx *gin.Context) string {
	// 1. 优先从执行层注入的 API_PROVIDER 获取
	if provider, exists := ctx.Get("API_PROVIDER"); exists {
		if p, ok := provider.(string); ok && p != "" {
			return p
		}
	}

	// 2. 尝试从认证上下文获取 (旧逻辑兼容)
	if auth, exists := ctx.Get("auth"); exists {
		if authInfo, ok := auth.(interface{ GetProvider() string }); ok {
			if provider := authInfo.GetProvider(); provider != "" {
				return provider
			}
		}
	}

	// 3. 从请求路径推断 (兜底逻辑)
	path := ctx.Request.URL.Path
	if strings.Contains(path, "/anthropic") || strings.Contains(path, "/claude") {
		return "claude"
	}
	if strings.Contains(path, "/openai") || strings.Contains(path, "/chat/completions") {
		return "openai"
	}
	if strings.Contains(path, "/gemini") || strings.Contains(path, "/vertex") {
		return "gemini"
	}

	return "unknown"
}

// extractModel 从请求和响应中提取model信息
func (c *ErrorCollector) extractModel(ctx *gin.Context, responseBody []byte) string {
	// 1. 优先从执行层注入的 API_MODEL 获取
	if model, exists := ctx.Get("API_MODEL"); exists {
		if m, ok := model.(string); ok && m != "" {
			return m
		}
	}

	// 2. 尝试从请求上下文获取 (旧逻辑兼容)
	if model, exists := ctx.Get("model"); exists {
		if modelStr, ok := model.(string); ok && modelStr != "" {
			return modelStr
		}
	}

	// 3. 尝试从响应体反查
	if len(responseBody) > 0 {
		if model := c.extractModelFromResponse(responseBody); model != "" {
			return model
		}
	}

	return "unknown"
}

// extractModelFromBody 从请求体中提取model字段
func (c *ErrorCollector) extractModelFromBody(body []byte) string {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}

	if model, ok := req["model"]; ok {
		if modelStr, ok := model.(string); ok {
			return modelStr
		}
	}

	return ""
}

// extractModelFromResponse 从响应体中提取model信息
func (c *ErrorCollector) extractModelFromResponse(body []byte) string {
	var resp map[string]interface{}
	if err := json.Unmarshal(body, &resp); err != nil {
		return ""
	}

	// 尝试从不同位置获取model
	if model, ok := resp["model"]; ok {
		if modelStr, ok := model.(string); ok {
			return modelStr
		}
	}

	// 检查error对象中是否包含model信息
	if errorObj, ok := resp["error"]; ok {
		if errorMap, ok := errorObj.(map[string]interface{}); ok {
			if model, ok := errorMap["model"]; ok {
				if modelStr, ok := model.(string); ok {
					return modelStr
				}
			}
		}
	}

	return ""
}

// extractErrorMessage 从响应中提取错误消息
func (c *ErrorCollector) extractErrorMessage(ctx *gin.Context, statusCode int, responseBody []byte) string {
	// 1. 优先检查 API_RESPONSE_ERROR (由 BaseAPIHandler 注入)
	if apiErrors, exists := ctx.Get("API_RESPONSE_ERROR"); exists {
		if errs, ok := apiErrors.([]*interfaces.ErrorMessage); ok && len(errs) > 0 {
			var msgs []string
			for _, e := range errs {
				if e != nil && e.Error != nil {
					msgs = append(msgs, e.Error.Error())
				}
			}
			if len(msgs) > 0 {
				return fmt.Sprintf("HTTP %d: %s", statusCode, strings.Join(msgs, "; "))
			}
		}
	}
	if len(responseBody) == 0 {
		return fmt.Sprintf("HTTP %d", statusCode)
	}

	// 尝试解析JSON错误响应
	var resp map[string]interface{}
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		// 如果不是JSON，返回原始文本的前200个字符
		text := strings.TrimSpace(string(responseBody))
		if len(text) > 200 {
			text = text[:200] + "..."
		}
		return fmt.Sprintf("HTTP %d: %s", statusCode, text)
	}

	// 提取错误消息
	if errorObj, ok := resp["error"]; ok {
		if errorStr, ok := errorObj.(string); ok {
			return fmt.Sprintf("HTTP %d: %s", statusCode, errorStr)
		}

		if errorMap, ok := errorObj.(map[string]interface{}); ok {
			if message, ok := errorMap["message"]; ok {
				if msgStr, ok := message.(string); ok {
					return fmt.Sprintf("HTTP %d: %s", statusCode, msgStr)
				}
			}

			// 尝试其他可能的错误字段
			if desc, ok := errorMap["description"]; ok {
				if descStr, ok := desc.(string); ok {
					return fmt.Sprintf("HTTP %d: %s", statusCode, descStr)
				}
			}
		}
	}

	// 检查其他可能的错误字段
	if message, ok := resp["message"]; ok {
		if msgStr, ok := message.(string); ok {
			return fmt.Sprintf("HTTP %d: %s", statusCode, msgStr)
		}
	}

	if detail, ok := resp["detail"]; ok {
		if detailStr, ok := detail.(string); ok {
			return fmt.Sprintf("HTTP %d: %s", statusCode, detailStr)
		}
	}

	// 如果没有找到明确的错误消息，返回状态码和响应的前100个字符
	text := strings.TrimSpace(string(responseBody))
	if len(text) > 100 {
		text = text[:100] + "..."
	}
	return fmt.Sprintf("HTTP %d: %s", statusCode, text)
}

// Middleware 创建Gin中间件用于自动收集错误
func (c *ErrorCollector) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 让请求继续处理
		ctx.Next()

		// 检查响应状态码
		statusCode := ctx.Writer.Status()
		if statusCode >= http.StatusBadRequest {
			// 获取响应体
			responseBody := c.getResponseBody(ctx)

			// 收集错误信息
			c.CollectError(ctx, statusCode, responseBody)
		}
	}
}

// getResponseBody 从上下文中获取响应体
func (c *ErrorCollector) getResponseBody(ctx *gin.Context) []byte {
	// 尝试从response writer wrapper获取响应体
	if wrapper, exists := ctx.Get("response_writer"); exists {
		if writer, ok := wrapper.(interface{ GetBody() []byte }); ok {
			return writer.GetBody()
		}
	}

	// 尝试从gin.Context获取记录的响应体
	if body, exists := ctx.Get("response_body"); exists {
		if bodyBytes, ok := body.([]byte); ok {
			return bodyBytes
		}
	}

	// 尝试从ResponseWriterWrapper获取（如果有）
	if writer, ok := ctx.Writer.(interface{ GetResponseBody() []byte }); ok {
		return writer.GetResponseBody()
	}

	return nil
}
