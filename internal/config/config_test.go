package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseTTL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		defaultVal time.Duration
		expected   time.Duration
	}{
		{"empty string", "", 24 * time.Hour, 24 * time.Hour},
		{"valid seconds", "3600", 24 * time.Hour, 3600 * time.Second},
		{"zero", "0", 24 * time.Hour, 24 * time.Hour},
		{"negative", "-100", 24 * time.Hour, 24 * time.Hour},
		{"invalid", "abc", 24 * time.Hour, 24 * time.Hour},
		{"large value", "86400", 24 * time.Hour, 86400 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTTL(tt.input, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("parseTTL(%q, %v) = %v, expected %v", tt.input, tt.defaultVal, result, tt.expected)
			}
		})
	}
}

func TestXDGPath(t *testing.T) {
	// Save original environment
	originalCacheHome := os.Getenv("XDG_CACHE_HOME")
	originalConfigHome := os.Getenv("XDG_CONFIG_HOME")

	// Clean up after test
	defer func() {
		if originalCacheHome != "" {
			os.Setenv("XDG_CACHE_HOME", originalCacheHome)
		} else {
			os.Unsetenv("XDG_CACHE_HOME")
		}
		if originalConfigHome != "" {
			os.Setenv("XDG_CONFIG_HOME", originalConfigHome)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	tests := []struct {
		name     string
		envVar   string
		envValue string
		fallback string
		expected string
	}{
		{
			name:     "with XDG env var set",
			envVar:   "XDG_CACHE_HOME",
			envValue: "/custom/cache",
			fallback: ".cache",
			expected: "/custom/cache",
		},
		{
			name:     "without XDG env var",
			envVar:   "XDG_CACHE_HOME",
			envValue: "",
			fallback: ".cache",
			expected: "", // Will be set to home + fallback, but we can't predict home
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.envVar, tt.envValue)
			} else {
				os.Unsetenv(tt.envVar)
			}

			result := xdgPath(tt.envVar, tt.fallback)
			if tt.envValue != "" {
				if result != tt.expected {
					t.Errorf("xdgPath(%q, %q) = %q, expected %q", tt.envVar, tt.fallback, result, tt.expected)
				}
			} else {
				// Just verify it doesn't crash and returns something
				if result == "" {
					t.Errorf("xdgPath(%q, %q) returned empty string", tt.envVar, tt.fallback)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	cfg := New()

	if cfg == nil {
		t.Fatal("New() returned nil")
	}

	if cfg.CacheDir == "" {
		t.Error("CacheDir is empty")
	}

	if cfg.ConfigDir == "" {
		t.Error("ConfigDir is empty")
	}

	if cfg.CacheFile == "" {
		t.Error("CacheFile is empty")
	}

	if cfg.ConfigFile == "" {
		t.Error("ConfigFile is empty")
	}

	if cfg.LockFile == "" {
		t.Error("LockFile is empty")
	}

	if cfg.TTL == 0 {
		t.Error("TTL is zero")
	}

	// Verify paths are constructed correctly
	if cfg.CacheFile != filepath.Join(cfg.CacheDir, "banners.json") {
		t.Errorf("CacheFile should be in CacheDir, got %q", cfg.CacheFile)
	}

	if cfg.ConfigFile != filepath.Join(cfg.ConfigDir, "sources.conf") {
		t.Errorf("ConfigFile should be in ConfigDir, got %q", cfg.ConfigFile)
	}

	if cfg.LockFile != filepath.Join(cfg.CacheDir, ".lock") {
		t.Errorf("LockFile should be in CacheDir, got %q", cfg.LockFile)
	}
}

func TestInitConfig(t *testing.T) {
	// Create temporary directory for config
	tmpDir := t.TempDir()
	cfg := &Config{
		ConfigDir:  tmpDir,
		ConfigFile: filepath.Join(tmpDir, "sources.conf"),
	}

	// First call should succeed
	err := cfg.InitConfig()
	if err != nil {
		t.Fatalf("InitConfig() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cfg.ConfigFile); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Second call should fail (file already exists)
	err = cfg.InitConfig()
	if err == nil {
		t.Error("InitConfig() should fail when file already exists")
	}
}
