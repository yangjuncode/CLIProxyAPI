package executor

import (
	"bytes"
	"strings"
)

// findRetryableError checks if the response data contains any patterns that should trigger a retry.
func (e *CodexExecutor) findRetryableError(data []byte) (string, bool) {
	if e.cfg == nil || len(e.cfg.RetryPatterns) == 0 {
		return "", false
	}
	if patterns, ok := e.cfg.RetryPatterns["codex"]; ok {
		if match, matched := matchRetryPattern(data, patterns); matched {
			return match, true
		}
	}
	if patterns, ok := e.cfg.RetryPatterns["global"]; ok {
		if match, matched := matchRetryPattern(data, patterns); matched {
			return match, true
		}
	}
	return "", false
}

// matchRetryPattern checks a data slice against a list of patterns (string or []string).
func matchRetryPattern(data []byte, patterns []any) (string, bool) {
	for _, p := range patterns {
		switch v := p.(type) {
		case string:
			if bytes.Contains(data, []byte(v)) {
				return v, true
			}
		case []any:
			matchAll := true
			var matchStrs []string
			for _, sub := range v {
				if s, ok := sub.(string); ok {
					matchStrs = append(matchStrs, s)
					if !bytes.Contains(data, []byte(s)) {
						matchAll = false
						break
					}
				}
			}
			if matchAll && len(matchStrs) > 0 {
				return "[" + strings.Join(matchStrs, ", ") + "]", true
			}
		}
	}
	return "", false
}
