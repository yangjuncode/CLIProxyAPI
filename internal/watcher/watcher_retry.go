package watcher

import (
	"path/filepath"
	"strings"
)

// getRetryPatternsPath returns the absolute path to the retry patterns file.
func (w *Watcher) getRetryPatternsPath() string {
	w.clientsMutex.RLock()
	cfg := w.config
	w.clientsMutex.RUnlock()

	if cfg == nil {
		return ""
	}
	patternsFile := strings.TrimSpace(cfg.RetryPatternsFile)
	if patternsFile != "" {
		if filepath.IsAbs(patternsFile) {
			return patternsFile
		}
		return filepath.Join(filepath.Dir(w.configPath), patternsFile)
	}
	return filepath.Join(filepath.Dir(w.configPath), "retry_patterns.yaml")
}
