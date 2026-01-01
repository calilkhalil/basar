// Package config handles XDG-compliant configuration and paths.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultSources contains the upstream ISF banner repositories.
var DefaultSources = []string{
	"https://raw.githubusercontent.com/Abyss-W4tcher/volatility3-symbols/master/banners/banners.json",
	"https://raw.githubusercontent.com/leludo84/vol3-linux-profiles/main/banners-isf.json",
}

const (
	// DefaultTTL is the default cache validity duration.
	DefaultTTL = 24 * time.Hour

	// AppName is used for XDG directory names.
	AppName = "basar"
)

// Config holds application configuration.
type Config struct {
	CacheDir   string
	ConfigDir  string
	CacheFile  string
	ConfigFile string
	LockFile   string
	TTL        time.Duration
	Sources    []string
}

// New creates a Config with XDG-compliant paths.
func New() *Config {
	cacheDir := xdgPath("XDG_CACHE_HOME", ".cache")
	configDir := xdgPath("XDG_CONFIG_HOME", ".config")

	cfg := &Config{
		CacheDir:  filepath.Join(cacheDir, AppName),
		ConfigDir: filepath.Join(configDir, AppName),
		TTL:       parseTTL(os.Getenv("BASAR_TTL"), DefaultTTL),
	}

	cfg.CacheFile = filepath.Join(cfg.CacheDir, "banners.json")
	cfg.ConfigFile = filepath.Join(cfg.ConfigDir, "sources.conf")
	cfg.LockFile = filepath.Join(cfg.CacheDir, ".lock")
	cfg.Sources = cfg.loadSources()

	return cfg
}

// xdgPath returns the XDG base directory or falls back to home + fallback.
func xdgPath(envVar, fallback string) string {
	if dir := os.Getenv(envVar); dir != "" {
		return dir
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "/"
	}

	return filepath.Join(home, fallback)
}

// parseTTL parses a TTL string as seconds, returning defaultVal on failure.
func parseTTL(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}

	// Try parsing as seconds (integer)
	var secs int64
	if _, err := fmt.Sscanf(s, "%d", &secs); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}

	return defaultVal
}

// loadSources reads sources from config file or returns defaults.
func (c *Config) loadSources() []string {
	f, err := os.Open(c.ConfigFile)
	if err != nil {
		return DefaultSources
	}
	defer f.Close()

	var sources []string
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sources = append(sources, line)
	}

	if len(sources) == 0 {
		return DefaultSources
	}

	return sources
}

// InitConfig creates the default configuration file.
// Returns error if file already exists.
func (c *Config) InitConfig() error {
	if _, err := os.Stat(c.ConfigFile); err == nil {
		return fmt.Errorf("config already exists: %s", c.ConfigFile)
	}

	if err := os.MkdirAll(c.ConfigDir, 0755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	f, err := os.Create(c.ConfigFile)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()

	f.WriteString("# basar sources configuration\n")
	f.WriteString("# One URL or local path per line\n")
	f.WriteString("# Lines starting with # are comments\n\n")

	for _, src := range DefaultSources {
		f.WriteString(src + "\n")
	}

	return nil
}
