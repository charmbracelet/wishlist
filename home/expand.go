package home

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandPath expands the given path if it starts with ~/.
func ExpandPath(p string) (string, error) {
	if !strings.HasPrefix(p, "~/") {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to expand path: %q: %w", p, err)
	}
	return filepath.Join(home, strings.TrimPrefix(p, "~/")), nil
}
