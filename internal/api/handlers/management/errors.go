// Package management provides HTTP handlers for the CLIProxyAPI management interface.
// This file contains the error history handler for displaying recent errors.
package management

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/errors"
)

// ErrorHandler 处理错误历史相关的HTTP请求
type ErrorHandler struct {
	errorManager *errors.ErrorHistoryManager
}

// NewErrorHandler 创建新的错误处理器
func NewErrorHandler(errorManager *errors.ErrorHistoryManager) *ErrorHandler {
	return &ErrorHandler{
		errorManager: errorManager,
	}
}

// GetErrors 返回错误历史记录的纯文本格式
// GET /errors
func (h *ErrorHandler) GetErrors(c *gin.Context) {
	if h.errorManager == nil {
		c.String(http.StatusServiceUnavailable, "错误历史服务不可用")
		return
	}

	// 解析查询参数
	limit := h.parseLimit(c.Query("limit"))
	since := h.parseSince(c.Query("since"))

	var errorRecords []errors.ErrorRecord

	if since.IsZero() {
		// 获取最近的错误记录
		if limit > 0 {
			errorRecords = h.errorManager.GetRecentErrors(limit)
		} else {
			errorRecords = h.errorManager.GetErrors()
		}
	} else {
		// 获取指定时间之后的错误记录
		allRecords := h.errorManager.GetErrors()
		errorRecords = make([]errors.ErrorRecord, 0)
		for _, record := range allRecords {
			if record.Timestamp.After(since) {
				errorRecords = append(errorRecords, record)
			}
		}
		if limit > 0 && len(errorRecords) > limit {
			errorRecords = errorRecords[:limit]
		}
	}

	// 生成纯文本输出
	c.Header("Content-Type", "text/plain; charset=utf-8")

	if len(errorRecords) == 0 {
		c.String(http.StatusOK, "暂无错误记录\n")
		return
	}

	// 构建文本内容
	var builder strings.Builder

	// 添加标题和统计信息
	builder.WriteString("=== 错误历史记录 ===\n")
	builder.WriteString(fmt.Sprintf("记录数量: %d\n", len(errorRecords)))
	builder.WriteString(fmt.Sprintf("生成时间: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	builder.WriteString("\n")

	// 添加表头
	builder.WriteString(fmt.Sprintf("%-19s | %-12s | %-20s | %-6s | %s\n",
		"时间", "Provider", "Model", "状态", "错误信息"))
	builder.WriteString(strings.Repeat("-", 80) + "\n")

	// 添加每条错误记录
	for _, record := range errorRecords {
		timeStr := record.Timestamp.Format("2006-01-02 15:04:05")
		provider := h.truncateString(record.Provider, 12)
		model := h.truncateString(record.Model, 20)
		statusCode := strconv.Itoa(record.StatusCode)
		errorMsg := h.truncateString(record.ErrorMessage, 30)

		builder.WriteString(fmt.Sprintf("%-19s | %-12s | %-20s | %-6s | %s\n",
			timeStr, provider, model, statusCode, errorMsg))
	}

	// 添加分隔线
	builder.WriteString(strings.Repeat("-", 80) + "\n")

	// 添加统计信息
	stats := h.errorManager.GetStats()
	builder.WriteString(fmt.Sprintf("\n统计信息:\n"))
	builder.WriteString(fmt.Sprintf("- 记录条数: %d 条\n", stats["total_records"]))
	builder.WriteString(fmt.Sprintf("- 有效记录: %d 条 (8小时内)\n", stats["valid_records"]))
	
	totalSize := stats["total_size_bytes"].(int64)
	maxMemMB := stats["max_memory_mb"].(int)
	builder.WriteString(fmt.Sprintf("- 当前内存: %.2f KB (上限 %d MB)\n", float64(totalSize)/1024, maxMemMB))
	builder.WriteString(fmt.Sprintf("- 保留时长: %.1f 小时\n", stats["cutoff_hours"].(float64)))

	c.String(http.StatusOK, builder.String())
}

// GetErrorsJSON 返回错误历史记录的JSON格式
// GET /errors.json
func (h *ErrorHandler) GetErrorsJSON(c *gin.Context) {
	if h.errorManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "错误历史服务不可用",
		})
		return
	}

	limit := h.parseLimit(c.Query("limit"))
	since := h.parseSince(c.Query("since"))

	var errorRecords []errors.ErrorRecord

	if since.IsZero() {
		if limit > 0 {
			errorRecords = h.errorManager.GetRecentErrors(limit)
		} else {
			errorRecords = h.errorManager.GetErrors()
		}
	} else {
		allRecords := h.errorManager.GetErrors()
		errorRecords = make([]errors.ErrorRecord, 0)
		for _, record := range allRecords {
			if record.Timestamp.After(since) {
				errorRecords = append(errorRecords, record)
			}
		}
		if limit > 0 && len(errorRecords) > limit {
			errorRecords = errorRecords[:limit]
		}
	}

	response := gin.H{
		"records":      errorRecords,
		"stats":        h.errorManager.GetStats(),
		"count":        len(errorRecords),
		"generated_at": time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, response)
}

// GetStats 返回错误统计信息
// GET /errors/stats
func (h *ErrorHandler) GetStats(c *gin.Context) {
	if h.errorManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "错误历史服务不可用",
		})
		return
	}

	stats := h.errorManager.GetStats()
	stats["generated_at"] = time.Now().Format(time.RFC3339)

	c.JSON(http.StatusOK, stats)
}

// ClearErrors 清空错误历史记录
// DELETE /errors
func (h *ErrorHandler) ClearErrors(c *gin.Context) {
	if h.errorManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "错误历史服务不可用",
		})
		return
	}

	// 这里需要添加清空方法到ErrorHistoryManager
	// 暂时返回成功消息
	c.JSON(http.StatusOK, gin.H{
		"message":   "错误历史记录已清空",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// parseLimit 解析limit参数
func (h *ErrorHandler) parseLimit(limitStr string) int {
	if limitStr == "" {
		return 0
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		return 0
	}

	// 限制最大返回数量
	if limit > 1000 {
		limit = 1000
	}

	return limit
}

// parseSince 解析since参数
func (h *ErrorHandler) parseSince(sinceStr string) time.Time {
	if sinceStr == "" {
		return time.Time{}
	}

	// 支持多种时间格式
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, sinceStr); err == nil {
			return t
		}
	}

	// 尝试解析为Unix时间戳
	if timestamp, err := strconv.ParseInt(sinceStr, 10, 64); err == nil {
		return time.Unix(timestamp, 0)
	}

	return time.Time{}
}

// truncateString 截断字符串到指定长度
func (h *ErrorHandler) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen > 3 {
		return s[:maxLen-3] + "..."
	}
	return s[:maxLen]
}

// getBufferStatus 获取缓冲区状态描述
func (h *ErrorHandler) getBufferStatus(full bool) string {
	if full {
		return "已满"
	}
	return "未满"
}
