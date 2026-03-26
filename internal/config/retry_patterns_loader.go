package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// LoadRetryPatterns attempts to load and parse retryable error patterns from a separate YAML file.
func (cfg *Config) LoadRetryPatterns(mainConfigPath string) {
	if cfg == nil {
		return
	}
	patternsFile := strings.TrimSpace(cfg.RetryPatternsFile)
	if patternsFile == "" {
		if mainConfigPath != "" {
			dir := filepath.Dir(mainConfigPath)
			patternsFile = filepath.Join(dir, "retry_patterns.yaml")
		} else {
			patternsFile = "retry_patterns.yaml"
		}
	}

	data, err := os.ReadFile(patternsFile)
	if err != nil {
		if !os.IsNotExist(err) && !errors.Is(err, syscall.EISDIR) {
			log.WithError(err).Warnf("failed to read retry patterns file: %s", patternsFile)
		}
		return
	}

	var patterns map[string][]any
	if err := yaml.Unmarshal(data, &patterns); err != nil {
		log.WithError(err).Errorf("failed to parse retry patterns file: %s", patternsFile)
		return
	}

	cfg.RetryPatterns = patterns
	log.Debugf("successfully loaded %d retry patterns from: %s", len(patterns), patternsFile)
}
