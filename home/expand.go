package home

import (
	"fmt"
	"log"
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

// MustExpandPath expands the given path and panics on error.
func MustExpandPath(p string) string {
	path, err := ExpandPath(p)
	if err != nil {
		log.Fatalln("unexpected error:", err)
	}
	return path
}
