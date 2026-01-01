package cache

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/calilkhalil/basar/internal/config"
	"github.com/calilkhalil/basar/internal/fetcher"
)

// testConfig creates a Config with temporary directories for testing.
func testConfig(t *testing.T) *config.Config {
	t.Helper()
	tmpDir := t.TempDir()

	return &config.Config{
		CacheDir:   tmpDir,
		ConfigDir:  tmpDir,
		CacheFile:  filepath.Join(tmpDir, "banners.json"),
		ConfigFile: filepath.Join(tmpDir, "sources.conf"),
		LockFile:   filepath.Join(tmpDir, ".lock"),
		TTL:        24 * time.Hour,
		Sources:    []string{},
	}
}

// createTestBannerFile creates a valid banner JSON file for testing.
func createTestBannerFile(t *testing.T, path string) {
	t.Helper()

	data := &fetcher.BannerData{
		Version: 1,
		Linux: map[string][]string{
			"Linux version 5.15.0-generic": {"https://example.com/symbols/5.15.0.json"},
			"Linux version 6.1.0-generic":  {"https://example.com/symbols/6.1.0.json"},
		},
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(data); err != nil {
		t.Fatalf("failed to encode JSON: %v", err)
	}
}

func TestNew(t *testing.T) {
	cfg := testConfig(t)
	c := New(cfg)

	if c == nil {
		t.Fatal("New() returned nil")
	}

	if c.cfg != cfg {
		t.Error("config not set correctly")
	}

	if c.fetcher == nil {
		t.Error("fetcher not initialized")
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*testing.T, *config.Config)
		ttl      time.Duration
		expected bool
	}{
		{
			name:     "no cache file",
			setup:    func(t *testing.T, cfg *config.Config) {},
			ttl:      24 * time.Hour,
			expected: false,
		},
		{
			name: "valid cache within TTL",
			setup: func(t *testing.T, cfg *config.Config) {
				createTestBannerFile(t, cfg.CacheFile)
			},
			ttl:      24 * time.Hour,
			expected: true,
		},
		{
			name: "expired cache",
			setup: func(t *testing.T, cfg *config.Config) {
				createTestBannerFile(t, cfg.CacheFile)
				// Set mtime to 2 hours ago
				oldTime := time.Now().Add(-2 * time.Hour)
				_ = os.Chtimes(cfg.CacheFile, oldTime, oldTime)
			},
			ttl:      1 * time.Hour,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.TTL = tt.ttl
			tt.setup(t, cfg)

			c := New(cfg)
			result := c.IsValid()

			if result != tt.expected {
				t.Errorf("IsValid() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestPath(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(*testing.T, *config.Config)
		expectPath bool
	}{
		{
			name:       "no cache file",
			setup:      func(t *testing.T, cfg *config.Config) {},
			expectPath: false,
		},
		{
			name: "cache file exists",
			setup: func(t *testing.T, cfg *config.Config) {
				createTestBannerFile(t, cfg.CacheFile)
			},
			expectPath: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			tt.setup(t, cfg)

			c := New(cfg)
			path, ok := c.Path()

			if ok != tt.expectPath {
				t.Errorf("Path() ok = %v, expected %v", ok, tt.expectPath)
			}

			if tt.expectPath && path != cfg.CacheFile {
				t.Errorf("Path() = %q, expected %q", path, cfg.CacheFile)
			}
		})
	}
}

func TestURI(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*testing.T, *config.Config)
		expectURI bool
	}{
		{
			name:      "no cache file",
			setup:     func(t *testing.T, cfg *config.Config) {},
			expectURI: false,
		},
		{
			name: "cache file exists",
			setup: func(t *testing.T, cfg *config.Config) {
				createTestBannerFile(t, cfg.CacheFile)
			},
			expectURI: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			tt.setup(t, cfg)

			c := New(cfg)
			uri, ok := c.URI()

			if ok != tt.expectURI {
				t.Errorf("URI() ok = %v, expected %v", ok, tt.expectURI)
			}

			if tt.expectURI {
				expected := "file://" + cfg.CacheFile
				if uri != expected {
					t.Errorf("URI() = %q, expected %q", uri, expected)
				}
			}
		})
	}
}

func TestStats(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*testing.T, *config.Config)
		expectValid bool
	}{
		{
			name:        "no cache file",
			setup:       func(t *testing.T, cfg *config.Config) {},
			expectValid: false,
		},
		{
			name: "valid cache file",
			setup: func(t *testing.T, cfg *config.Config) {
				createTestBannerFile(t, cfg.CacheFile)
			},
			expectValid: true,
		},
		{
			name: "invalid JSON",
			setup: func(t *testing.T, cfg *config.Config) {
				os.MkdirAll(cfg.CacheDir, 0755)
				os.WriteFile(cfg.CacheFile, []byte("invalid json"), 0644)
			},
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			tt.setup(t, cfg)

			c := New(cfg)
			stats := c.Stats()

			if stats.Valid != tt.expectValid {
				t.Errorf("Stats().Valid = %v, expected %v", stats.Valid, tt.expectValid)
			}

			if tt.expectValid {
				if stats.Path != cfg.CacheFile {
					t.Errorf("Stats().Path = %q, expected %q", stats.Path, cfg.CacheFile)
				}
				if stats.Entries != 2 {
					t.Errorf("Stats().Entries = %d, expected 2", stats.Entries)
				}
				if stats.Size <= 0 {
					t.Errorf("Stats().Size = %d, expected > 0", stats.Size)
				}
				if stats.AgeSeconds < 0 {
					t.Errorf("Stats().AgeSeconds = %d, expected >= 0", stats.AgeSeconds)
				}
			}
		})
	}
}

func TestClear(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T, *config.Config)
		wantErr bool
	}{
		{
			name:    "no cache file (should not error)",
			setup:   func(t *testing.T, cfg *config.Config) {},
			wantErr: false,
		},
		{
			name: "cache file exists",
			setup: func(t *testing.T, cfg *config.Config) {
				createTestBannerFile(t, cfg.CacheFile)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			tt.setup(t, cfg)

			c := New(cfg)
			err := c.Clear()

			if (err != nil) != tt.wantErr {
				t.Errorf("Clear() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify file is gone
			if _, err := os.Stat(cfg.CacheFile); !os.IsNotExist(err) {
				t.Error("Clear() did not remove cache file")
			}
		})
	}
}

func TestAcquireLock(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T, *config.Config)
		wantErr bool
	}{
		{
			name:    "no existing lock",
			setup:   func(t *testing.T, cfg *config.Config) {},
			wantErr: false,
		},
		{
			name: "stale lock (should acquire)",
			setup: func(t *testing.T, cfg *config.Config) {
				_ = os.MkdirAll(cfg.CacheDir, 0755)
				_ = os.WriteFile(cfg.LockFile, []byte("12345"), 0644)
				// Set mtime to 10 minutes ago (beyond LockTimeout)
				oldTime := time.Now().Add(-10 * time.Minute)
				_ = os.Chtimes(cfg.LockFile, oldTime, oldTime)
			},
			wantErr: false,
		},
		{
			name: "fresh lock (should fail)",
			setup: func(t *testing.T, cfg *config.Config) {
				os.MkdirAll(cfg.CacheDir, 0755)
				os.WriteFile(cfg.LockFile, []byte("12345"), 0644)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			tt.setup(t, cfg)

			c := New(cfg)
			err := c.acquireLock()

			if (err != nil) != tt.wantErr {
				t.Errorf("acquireLock() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err == nil {
				// Clean up lock
				c.releaseLock()
			}
		})
	}
}

func TestReleaseLock(t *testing.T) {
	cfg := testConfig(t)
	c := New(cfg)

	// Acquire lock first
	err := c.acquireLock()
	if err != nil {
		t.Fatalf("acquireLock() failed: %v", err)
	}

	// Verify lock exists
	if _, err := os.Stat(cfg.LockFile); os.IsNotExist(err) {
		t.Fatal("lock file was not created")
	}

	// Release lock
	c.releaseLock()

	// Verify lock is gone
	if _, err := os.Stat(cfg.LockFile); !os.IsNotExist(err) {
		t.Error("releaseLock() did not remove lock file")
	}
}

func TestWrite(t *testing.T) {
	cfg := testConfig(t)
	c := New(cfg)

	data := &fetcher.BannerData{
		Version: 1,
		Linux: map[string][]string{
			"banner1": {"url1", "url2"},
			"banner2": {"url3"},
		},
	}

	err := c.write(data)
	if err != nil {
		t.Fatalf("write() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cfg.CacheFile); os.IsNotExist(err) {
		t.Fatal("cache file was not created")
	}

	// Verify content
	content, err := os.ReadFile(cfg.CacheFile)
	if err != nil {
		t.Fatalf("failed to read cache file: %v", err)
	}

	var result fetcher.BannerData
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("failed to unmarshal cache file: %v", err)
	}

	if result.Version != data.Version {
		t.Errorf("Version = %d, expected %d", result.Version, data.Version)
	}

	if len(result.Linux) != len(data.Linux) {
		t.Errorf("Linux banners count = %d, expected %d", len(result.Linux), len(data.Linux))
	}
}

func TestUpdateWithLocalSource(t *testing.T) {
	cfg := testConfig(t)

	// Create a local source file
	sourceFile := filepath.Join(cfg.ConfigDir, "source.json")
	createTestBannerFile(t, sourceFile)

	cfg.Sources = []string{sourceFile}

	c := New(cfg)
	ctx := context.Background()

	err := c.Update(ctx, true)
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	// Verify cache was created
	if !c.IsValid() {
		t.Error("cache should be valid after Update")
	}

	stats := c.Stats()
	if stats.Entries != 2 {
		t.Errorf("Stats().Entries = %d, expected 2", stats.Entries)
	}
}

func TestUpdateSkipsWhenValid(t *testing.T) {
	cfg := testConfig(t)

	// Create existing valid cache
	createTestBannerFile(t, cfg.CacheFile)

	// Use empty sources - if update runs, it will fail
	cfg.Sources = []string{}

	c := New(cfg)
	ctx := context.Background()

	// Non-forced update should skip
	err := c.Update(ctx, false)
	if err != nil {
		t.Errorf("Update(force=false) should skip when cache is valid: %v", err)
	}
}

func TestUpdateAllSourcesFailed(t *testing.T) {
	cfg := testConfig(t)

	// Use non-existent source
	cfg.Sources = []string{"/nonexistent/path/to/file.json"}

	c := New(cfg)
	ctx := context.Background()

	err := c.Update(ctx, true)
	if err == nil {
		t.Error("Update() should fail when all sources fail")
	}

	expectedMsg := "all sources failed"
	if err.Error() != expectedMsg {
		t.Errorf("Update() error = %q, expected %q", err.Error(), expectedMsg)
	}
}

func TestEnsure(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*testing.T, *config.Config)
		wantErr bool
	}{
		{
			name: "cache already valid",
			setup: func(t *testing.T, cfg *config.Config) {
				createTestBannerFile(t, cfg.CacheFile)
			},
			wantErr: false,
		},
		{
			name: "cache needs update with valid source",
			setup: func(t *testing.T, cfg *config.Config) {
				sourceFile := filepath.Join(cfg.ConfigDir, "source.json")
				createTestBannerFile(t, sourceFile)
				cfg.Sources = []string{sourceFile}
			},
			wantErr: false,
		},
		{
			name: "cache needs update but sources fail",
			setup: func(t *testing.T, cfg *config.Config) {
				cfg.Sources = []string{"/nonexistent/file.json"}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testConfig(t)
			tt.setup(t, cfg)

			c := New(cfg)
			ctx := context.Background()

			err := c.Ensure(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Ensure() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdateWithContextCancellation(t *testing.T) {
	cfg := testConfig(t)

	// Create a valid local source
	sourceFile := filepath.Join(cfg.ConfigDir, "source.json")
	createTestBannerFile(t, sourceFile)
	cfg.Sources = []string{sourceFile}

	c := New(cfg)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Update should still work for local files (context mainly affects HTTP)
	err := c.Update(ctx, true)

	// Local file fetching doesn't use context, so this should succeed
	if err != nil {
		t.Logf("Update with cancelled context: %v (expected for HTTP sources)", err)
	}
}

func TestUpdateMergesMultipleSources(t *testing.T) {
	cfg := testConfig(t)

	// Create two source files with different banners
	source1 := filepath.Join(cfg.ConfigDir, "source1.json")
	source2 := filepath.Join(cfg.ConfigDir, "source2.json")

	// Source 1
	data1 := &fetcher.BannerData{
		Version: 1,
		Linux: map[string][]string{
			"banner1": {"url1"},
		},
	}
	f1, _ := os.Create(source1)
	_ = json.NewEncoder(f1).Encode(data1)
	_ = f1.Close()

	// Source 2
	data2 := &fetcher.BannerData{
		Version: 1,
		Linux: map[string][]string{
			"banner2": {"url2"},
		},
	}
	f2, _ := os.Create(source2)
	_ = json.NewEncoder(f2).Encode(data2)
	_ = f2.Close()

	cfg.Sources = []string{source1, source2}

	c := New(cfg)
	ctx := context.Background()

	err := c.Update(ctx, true)
	if err != nil {
		t.Fatalf("Update() failed: %v", err)
	}

	stats := c.Stats()
	if stats.Entries != 2 {
		t.Errorf("Stats().Entries = %d, expected 2 (merged from 2 sources)", stats.Entries)
	}
}

func TestSmartUpdate(t *testing.T) {
	cfg := testConfig(t)

	// Create a source file
	sourceFile := filepath.Join(cfg.ConfigDir, "source.json")
	createTestBannerFile(t, sourceFile)
	cfg.Sources = []string{sourceFile}

	c := New(cfg)
	ctx := context.Background()

	// First smart update - should update
	updated, err := c.SmartUpdate(ctx, false)
	if err != nil {
		t.Fatalf("SmartUpdate() failed: %v", err)
	}
	if !updated {
		t.Error("first SmartUpdate should return updated=true")
	}

	// Verify cache was created
	if !c.IsValid() {
		t.Error("cache should be valid after SmartUpdate")
	}
}

func TestSmartUpdateNoChange(t *testing.T) {
	cfg := testConfig(t)

	// Create and populate cache first
	sourceFile := filepath.Join(cfg.ConfigDir, "source.json")
	createTestBannerFile(t, sourceFile)
	cfg.Sources = []string{sourceFile}

	c := New(cfg)
	ctx := context.Background()

	// First update
	c.Update(ctx, true)

	// Second smart update - local files always report modified
	// (conditional requests only work with HTTP)
	updated, err := c.SmartUpdate(ctx, false)
	if err != nil {
		t.Fatalf("SmartUpdate() failed: %v", err)
	}

	// Local files always appear modified since there's no ETag/Last-Modified
	if !updated {
		t.Log("SmartUpdate with local files always reports updated")
	}
}

func TestLoadAndSaveMeta(t *testing.T) {
	cfg := testConfig(t)
	c := New(cfg)

	// Initially empty
	meta := c.loadMeta()
	if len(meta.Sources) != 0 {
		t.Errorf("initial meta should be empty, got %d sources", len(meta.Sources))
	}

	// Save some metadata
	meta.Sources["http://example.com"] = fetcher.SourceMeta{
		ETag:      `"abc123"`,
		UpdatedAt: time.Now(),
	}

	if err := c.saveMeta(meta); err != nil {
		t.Fatalf("saveMeta failed: %v", err)
	}

	// Load it back
	loaded := c.loadMeta()
	if len(loaded.Sources) != 1 {
		t.Errorf("loaded meta should have 1 source, got %d", len(loaded.Sources))
	}

	if loaded.Sources["http://example.com"].ETag != `"abc123"` {
		t.Error("ETag not preserved")
	}
}

func TestConfigureVolatility3(t *testing.T) {
	cfg := testConfig(t)

	// Point vol3 config to temp dir
	home := cfg.CacheDir // Use temp dir as fake home
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer os.Setenv("HOME", origHome)

	c := New(cfg)

	err := c.ConfigureVolatility3()
	if err != nil {
		t.Fatalf("ConfigureVolatility3 failed: %v", err)
	}

	// Check file was created
	vol3Config := filepath.Join(home, ".volatility3.yaml")
	content, err := os.ReadFile(vol3Config)
	if err != nil {
		t.Fatalf("could not read vol3 config: %v", err)
	}

	if !strings.Contains(string(content), "remote_isf_url") {
		t.Error("vol3 config should contain remote_isf_url")
	}

	if !strings.Contains(string(content), "file://") {
		t.Error("vol3 config should contain file:// URI")
	}
}

func TestConfigureVolatility3AlreadyExists(t *testing.T) {
	cfg := testConfig(t)

	home := cfg.CacheDir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer os.Setenv("HOME", origHome)

	// Create existing config with remote_isf_url
	vol3Config := filepath.Join(home, ".volatility3.yaml")
	os.WriteFile(vol3Config, []byte("remote_isf_url: http://other.com\n"), 0644)

	c := New(cfg)

	err := c.ConfigureVolatility3()
	if err == nil {
		t.Error("should error when remote_isf_url already exists")
	}
}
