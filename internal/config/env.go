// @metrics-project: metrics
// @metrics-path: internal/config/env.go
// Package config provides environment variable helpers for Metrics.
package config

import (
	"os"
	"path/filepath"
)

const (
	DefaultHTTPAddr  = "127.0.0.1:8083"
	DefaultNexusAddr = "http://127.0.0.1:8080"
	DefaultAtlasAddr = "http://127.0.0.1:8081"
	DefaultForgeAddr = "http://127.0.0.1:8082"
)

// EnvOrDefault returns the env var value or fallback.
func EnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ExpandHome replaces leading ~/ with the user home directory.
func ExpandHome(path string) string {
	if len(path) < 2 || path[:2] != "~/" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
