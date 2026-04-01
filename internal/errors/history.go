// Package errors provides error history tracking functionality for CLIProxyAPI.
// It maintains an in-memory circular buffer of error records with automatic cleanup.
package errors

import (
	"sync"
	"time"
)

// ErrorRecord 表示一个错误请求记录
type ErrorRecord struct {
	Timestamp    time.Time `json:"timestamp"`
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	ErrorMessage string    `json:"error_message"`
	StatusCode   int       `json:"status_code"`
	RequestPath  string    `json:"request_path"`
}

// ErrorHistoryManager 管理错误历史记录的内存存储
type ErrorHistoryManager struct {
	mu             sync.RWMutex
	records        []ErrorRecord
	totalSizeBytes int64         // 当前估算的内存占用（字节）
	maxMemoryBytes int64         // 最大内存限制（字节）
	maxMemoryMB    int           // 用于统计显示
	cutoffTime     time.Duration // 记录保留时间
}

// NewErrorHistoryManager 创建新的错误历史管理器
func NewErrorHistoryManager(maxMemoryMB int) *ErrorHistoryManager {
	return &ErrorHistoryManager{
		records:        make([]ErrorRecord, 0),
		totalSizeBytes: 0,
		maxMemoryBytes: int64(maxMemoryMB) * 1024 * 1024,
		maxMemoryMB:    maxMemoryMB,
		cutoffTime:     8 * time.Hour, // 8小时保留期
	}
}

// recordSize 估算单条记录占用的内存大小
func (m *ErrorHistoryManager) recordSize(r ErrorRecord) int64 {
	// 基本结构体开销 + 字符串内容的实际长度
	// time.Time (24) + statusCode (8) + 字符串指针和开销 (16 * 4) + 字符串内容
	return int64(24 + 8 + 64 + len(r.Provider) + len(r.Model) + len(r.ErrorMessage) + len(r.RequestPath))
}

// AddError 添加新的错误记录
func (m *ErrorHistoryManager) AddError(provider, model, errorMessage string, statusCode int, requestPath string) {
	record := ErrorRecord{
		Timestamp:    time.Now(),
		Provider:     provider,
		Model:        model,
		ErrorMessage: errorMessage,
		StatusCode:   statusCode,
		RequestPath:  requestPath,
	}

	size := m.recordSize(record)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果单条记录就超过了总限制（极端情况），则跳过
	if size > m.maxMemoryBytes {
		return
	}

	// 1. 添加新记录
	m.records = append(m.records, record)
	m.totalSizeBytes += size

	// 2. 检查内存限制，必要时清理旧记录
	for m.totalSizeBytes > m.maxMemoryBytes && len(m.records) > 0 {
		oldRecord := m.records[0]
		m.totalSizeBytes -= m.recordSize(oldRecord)
		m.records = m.records[1:]
	}
}

// GetErrors 获取错误记录，按时间倒序排列
func (m *ErrorHistoryManager) GetErrors() []ErrorRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-m.cutoffTime)
	var result []ErrorRecord

	// 从最新的记录开始遍历（在 slice 中是尾部）
	for i := len(m.records) - 1; i >= 0; i-- {
		record := m.records[i]

		// 跳过时间上过期的记录
		if record.Timestamp.Before(cutoff) {
			continue
		}

		result = append(result, record)
	}

	return result
}

// GetRecentErrors 获取最近的N条错误记录
func (m *ErrorHistoryManager) GetRecentErrors(limit int) []ErrorRecord {
	all := m.GetErrors()
	if limit > 0 && len(all) > limit {
		return all[:limit]
	}
	return all
}

// CleanupExpiredRecords 清理过期记录
func (m *ErrorHistoryManager) CleanupExpiredRecords() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.cutoffTime)
	
	// 找到第一个未过期的记录
	firstValidIdx := -1
	for i, record := range m.records {
		if !record.Timestamp.Before(cutoff) {
			firstValidIdx = i
			break
		}
	}

	// 如果所有记录都过期了
	if firstValidIdx == -1 {
		if len(m.records) > 0 {
			m.records = m.records[:0]
			m.totalSizeBytes = 0
		}
		return
	}

	// 如果有记录过期，执行清理
	if firstValidIdx > 0 {
		// 减去丢弃记录的大小
		for i := 0; i < firstValidIdx; i++ {
			m.totalSizeBytes -= m.recordSize(m.records[i])
		}
		m.records = m.records[firstValidIdx:]
	}
}

// GetStats 获取统计信息
func (m *ErrorHistoryManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-m.cutoffTime)
	validCount := 0

	for _, record := range m.records {
		if !record.Timestamp.IsZero() && record.Timestamp.After(cutoff) {
			validCount++
		}
	}

	return map[string]interface{}{
		"total_records":     len(m.records),
		"valid_records":     validCount,
		"total_size_bytes":  m.totalSizeBytes,
		"cutoff_hours":      m.cutoffTime.Hours(),
		"max_memory_mb":     m.maxMemoryMB,
	}
}

// StartCleanupRoutine 启动定期清理协程
func (m *ErrorHistoryManager) StartCleanupRoutine(stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Minute) // 每30分钟清理一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.CleanupExpiredRecords()
		case <-stop:
			return
		}
	}
}
