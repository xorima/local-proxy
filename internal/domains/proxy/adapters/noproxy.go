package adapters

import (
	"path/filepath"
	"strings"
)

type NoProxyMatcher struct {
	patterns []string
}

func NewNoProxyMatcher(patterns []string) *NoProxyMatcher {
	return &NoProxyMatcher{patterns: patterns}
}

func (m *NoProxyMatcher) Match(target string) bool {
	for _, p := range m.patterns {
		matched, _ := filepath.Match(p, target)
		if matched {
			return true
		}
		if strings.Contains(target, p) {
			return true
		}
	}
	return false
}
