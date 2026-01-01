package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/calilkhalil/basar/internal/cache"
	"github.com/calilkhalil/basar/internal/fetcher"
)

// testEnv sets up a temporary environment for testing.
type testEnv struct {
	tmpDir     string
	cacheDir   string
	configDir  string
	cacheFile  string
	configFile string
	sourceFile string
	origCache  string
	origConfig string
}

// setup creates temporary directories and sets environment variables.
func (e *testEnv) setup(t *testing.T) {
	t.Helper()

	e.tmpDir = t.TempDir()
	e.cacheDir = filepath.Join(e.tmpDir, "cache")
	e.configDir = filepath.Join(e.tmpDir, "config")
	e.cacheFile = filepath.Join(e.cacheDir, "basar", "banners.json")
	e.configFile = filepath.Join(e.configDir, "basar", "sources.conf")
	e.sourceFile = filepath.Join(e.tmpDir, "source.json")

	// Save original env
	e.origCache = os.Getenv("XDG_CACHE_HOME")
	e.origConfig = os.Getenv("XDG_CONFIG_HOME")

	// Set test env
	os.Setenv("XDG_CACHE_HOME", e.cacheDir)
	os.Setenv("XDG_CONFIG_HOME", e.configDir)
}

// teardown restores environment variables.
func (e *testEnv) teardown() {
	if e.origCache != "" {
		os.Setenv("XDG_CACHE_HOME", e.origCache)
	} else {
		os.Unsetenv("XDG_CACHE_HOME")
	}

	if e.origConfig != "" {
		os.Setenv("XDG_CONFIG_HOME", e.origConfig)
	} else {
		os.Unsetenv("XDG_CONFIG_HOME")
	}
}

// createSource creates a test source file with sample banner data.
func (e *testEnv) createSource(t *testing.T) {
	t.Helper()

	data := &fetcher.BannerData{
		Version: 1,
		Linux: map[string][]string{
			"Linux version 5.15.0-generic": {"https://example.com/5.15.0.json"},
			"Linux version 6.1.0-generic":  {"https://example.com/6.1.0.json"},
		},
	}

	f, err := os.Create(e.sourceFile)
	if err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(data); err != nil {
		t.Fatalf("failed to encode source file: %v", err)
	}
}

// createConfig creates a config file pointing to the test source.
func (e *testEnv) createConfig(t *testing.T) {
	t.Helper()

	configDir := filepath.Dir(e.configFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	if err := os.WriteFile(e.configFile, []byte(e.sourceFile+"\n"), 0644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}
}

// createCache creates a valid cache file.
func (e *testEnv) createCache(t *testing.T) {
	t.Helper()

	cacheDir := filepath.Dir(e.cacheFile)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	data := &fetcher.BannerData{
		Version: 1,
		Linux: map[string][]string{
			"Linux version 5.15.0-generic": {"https://example.com/5.15.0.json"},
		},
	}

	f, err := os.Create(e.cacheFile)
	if err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(data); err != nil {
		t.Fatalf("failed to encode cache file: %v", err)
	}
}

func TestParseFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		check   func(*Flags) bool
		wantErr bool
	}{
		{
			name: "no flags",
			args: []string{},
			check: func(f *Flags) bool {
				return !f.Path && !f.URI && !f.Stats && !f.Check &&
					!f.Update && !f.Clear && !f.Init && !f.Verbose && !f.Help &&
					!f.SmartUpdate && !f.Setup && !f.InstallService && !f.ConfigureVol3
			},
		},
		{
			name:  "help short",
			args:  []string{"-h"},
			check: func(f *Flags) bool { return f.Help },
		},
		{
			name:  "help long",
			args:  []string{"--help"},
			check: func(f *Flags) bool { return f.Help },
		},
		{
			name:  "path short",
			args:  []string{"-p"},
			check: func(f *Flags) bool { return f.Path },
		},
		{
			name:  "path long",
			args:  []string{"--path"},
			check: func(f *Flags) bool { return f.Path },
		},
		{
			name:  "uri short",
			args:  []string{"-u"},
			check: func(f *Flags) bool { return f.URI },
		},
		{
			name:  "uri long",
			args:  []string{"--uri"},
			check: func(f *Flags) bool { return f.URI },
		},
		{
			name:  "stats short",
			args:  []string{"-s"},
			check: func(f *Flags) bool { return f.Stats },
		},
		{
			name:  "stats long",
			args:  []string{"--stats"},
			check: func(f *Flags) bool { return f.Stats },
		},
		{
			name:  "check short",
			args:  []string{"-c"},
			check: func(f *Flags) bool { return f.Check },
		},
		{
			name:  "check long",
			args:  []string{"--check"},
			check: func(f *Flags) bool { return f.Check },
		},
		{
			name:  "update",
			args:  []string{"--update"},
			check: func(f *Flags) bool { return f.Update },
		},
		{
			name:  "smart-update",
			args:  []string{"--smart-update"},
			check: func(f *Flags) bool { return f.SmartUpdate },
		},
		{
			name:  "clear",
			args:  []string{"--clear"},
			check: func(f *Flags) bool { return f.Clear },
		},
		{
			name:  "init",
			args:  []string{"--init"},
			check: func(f *Flags) bool { return f.Init },
		},
		{
			name:  "init-config alias",
			args:  []string{"--init-config"},
			check: func(f *Flags) bool { return f.Init },
		},
		{
			name:  "setup",
			args:  []string{"--setup"},
			check: func(f *Flags) bool { return f.Setup },
		},
		{
			name:  "install-service",
			args:  []string{"--install-service"},
			check: func(f *Flags) bool { return f.InstallService },
		},
		{
			name:  "configure-vol3",
			args:  []string{"--configure-vol3"},
			check: func(f *Flags) bool { return f.ConfigureVol3 },
		},
		{
			name:  "verbose short",
			args:  []string{"-v"},
			check: func(f *Flags) bool { return f.Verbose },
		},
		{
			name:  "verbose long",
			args:  []string{"--verbose"},
			check: func(f *Flags) bool { return f.Verbose },
		},
		{
			name: "multiple flags",
			args: []string{"-v", "-s"},
			check: func(f *Flags) bool {
				return f.Verbose && f.Stats
			},
		},
		{
			name:    "unknown flag",
			args:    []string{"--unknown"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := parseFlags(tt.args)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseFlags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !tt.check(flags) {
				t.Errorf("parseFlags(%v) flag check failed", tt.args)
			}
		})
	}
}

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-h"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(-h) = %d, expected %d", code, exitOK)
	}

	output := stdout.String()
	if !strings.Contains(output, "basar") {
		t.Error("help output should contain 'basar'")
	}
	if !strings.Contains(output, "Volatility3") {
		t.Error("help output should contain 'Volatility3'")
	}
	if !strings.Contains(output, "--update") {
		t.Error("help output should contain '--update'")
	}
}

func TestRunInit(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--init"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(--init) = %d, expected %d; stderr: %s", code, exitOK, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "sources.conf") {
		t.Errorf("init output should contain config path, got: %s", output)
	}

	// Verify file was created
	if _, err := os.Stat(env.configFile); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestRunInitAlreadyExists(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	// Create config first
	env.createConfig(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--init"}, &stdout, &stderr)

	if code != exitError {
		t.Errorf("run(--init) with existing config = %d, expected %d", code, exitError)
	}

	if !strings.Contains(stderr.String(), "already exists") {
		t.Errorf("stderr should mention 'already exists', got: %s", stderr.String())
	}
}

func TestRunClear(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	// Create cache first
	env.createCache(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--clear"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(--clear) = %d, expected %d; stderr: %s", code, exitOK, stderr.String())
	}

	// Verify cache was removed
	if _, err := os.Stat(env.cacheFile); !os.IsNotExist(err) {
		t.Error("cache file was not removed")
	}
}

func TestRunCheckValid(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	// Create valid cache
	env.createCache(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-c"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(-c) with valid cache = %d, expected %d", code, exitOK)
	}
}

func TestRunCheckInvalid(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	// No cache file = invalid

	var stdout, stderr bytes.Buffer
	code := run([]string{"-c"}, &stdout, &stderr)

	if code != exitInvalid {
		t.Errorf("run(-c) with no cache = %d, expected %d", code, exitInvalid)
	}
}

func TestRunStats(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	// Create cache
	env.createCache(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-s"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(-s) = %d, expected %d; stderr: %s", code, exitOK, stderr.String())
	}

	// Verify JSON output
	var stats cache.Stats
	if err := json.Unmarshal(stdout.Bytes(), &stats); err != nil {
		t.Errorf("failed to parse stats JSON: %v", err)
	}

	if !stats.Valid {
		t.Error("stats.Valid should be true")
	}
}

func TestRunStatsNoCache(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	var stdout, stderr bytes.Buffer
	code := run([]string{"-s"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(-s) = %d, expected %d", code, exitOK)
	}

	var stats cache.Stats
	if err := json.Unmarshal(stdout.Bytes(), &stats); err != nil {
		t.Errorf("failed to parse stats JSON: %v", err)
	}

	if stats.Valid {
		t.Error("stats.Valid should be false when no cache exists")
	}
}

func TestRunUpdate(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	// Create source and config
	env.createSource(t)
	env.createConfig(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--update"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(--update) = %d, expected %d; stderr: %s", code, exitOK, stderr.String())
	}

	// Verify cache was created
	if _, err := os.Stat(env.cacheFile); os.IsNotExist(err) {
		t.Error("cache file was not created")
	}
}

func TestRunUpdateVerbose(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	env.createSource(t)
	env.createConfig(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--update", "-v"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(--update -v) = %d, expected %d; stderr: %s", code, exitOK, stderr.String())
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "updating from") {
		t.Errorf("verbose output should contain 'updating from', got: %s", errOutput)
	}
	if !strings.Contains(errOutput, "cached") {
		t.Errorf("verbose output should contain 'cached', got: %s", errOutput)
	}
}

func TestRunUpdateNoSources(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	// Create config with non-existent source
	configDir := filepath.Dir(env.configFile)
	_ = os.MkdirAll(configDir, 0755)
	_ = os.WriteFile(env.configFile, []byte("/nonexistent/file.json\n"), 0644)

	var stdout, stderr bytes.Buffer
	code := run([]string{"--update"}, &stdout, &stderr)

	if code != exitError {
		t.Errorf("run(--update) with bad sources = %d, expected %d", code, exitError)
	}
}

func TestRunPath(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	env.createSource(t)
	env.createConfig(t)
	env.createCache(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-p"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(-p) = %d, expected %d; stderr: %s", code, exitOK, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.HasSuffix(output, "banners.json") {
		t.Errorf("path output should end with banners.json, got: %s", output)
	}
}

func TestRunPathNoCache(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	// Create config with non-existent source (so ensure fails)
	configDir := filepath.Dir(env.configFile)
	_ = os.MkdirAll(configDir, 0755)
	_ = os.WriteFile(env.configFile, []byte("/nonexistent/file.json\n"), 0644)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-p"}, &stdout, &stderr)

	// Should fail because Ensure fails
	if code != exitError {
		t.Errorf("run(-p) with no cache = %d, expected %d", code, exitError)
	}
}

func TestRunURI(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	env.createSource(t)
	env.createConfig(t)
	env.createCache(t)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-u"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(-u) = %d, expected %d; stderr: %s", code, exitOK, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(output, "file://") {
		t.Errorf("URI output should start with file://, got: %s", output)
	}
	if !strings.HasSuffix(output, "banners.json") {
		t.Errorf("URI output should end with banners.json, got: %s", output)
	}
}

func TestRunDefaultURI(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	env.createSource(t)
	env.createConfig(t)
	env.createCache(t)

	var stdout, stderr bytes.Buffer
	// No flags = default URI output
	code := run([]string{}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run() = %d, expected %d; stderr: %s", code, exitOK, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	if !strings.HasPrefix(output, "file://") {
		t.Errorf("default output should be URI starting with file://, got: %s", output)
	}
}

func TestRunVerboseFromEnv(t *testing.T) {
	env := &testEnv{}
	env.setup(t)
	defer env.teardown()

	env.createSource(t)
	env.createConfig(t)

	// Set verbose via environment
	origVerbose := os.Getenv("BASAR_VERBOSE")
	os.Setenv("BASAR_VERBOSE", "1")
	defer func() {
		if origVerbose != "" {
			os.Setenv("BASAR_VERBOSE", origVerbose)
		} else {
			os.Unsetenv("BASAR_VERBOSE")
		}
	}()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--update"}, &stdout, &stderr)

	if code != exitOK {
		t.Errorf("run(--update) = %d, expected %d", code, exitOK)
	}

	errOutput := stderr.String()
	if !strings.Contains(errOutput, "updating from") {
		t.Errorf("BASAR_VERBOSE=1 should enable verbose output, got: %s", errOutput)
	}
}

func TestRunInvalidFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--invalid-flag"}, &stdout, &stderr)

	if code != exitError {
		t.Errorf("run(--invalid-flag) = %d, expected %d", code, exitError)
	}

	if stderr.Len() == 0 {
		t.Error("invalid flag should produce error message")
	}
}

func TestPrintUsage(t *testing.T) {
	var buf bytes.Buffer
	printUsage(&buf)

	output := buf.String()

	expectedStrings := []string{
		"basar",
		"Volatility3",
		"--path",
		"--uri",
		"--stats",
		"--check",
		"--update",
		"--smart-update",
		"--clear",
		"--init",
		"--setup",
		"--install-service",
		"--configure-vol3",
		"--verbose",
		"--help",
		"BASAR_TTL",
		"BASAR_VERBOSE",
		"sources.conf",
	}

	for _, s := range expectedStrings {
		if !strings.Contains(output, s) {
			t.Errorf("usage should contain %q", s)
		}
	}
}

func TestExitCodes(t *testing.T) {
	if exitOK != 0 {
		t.Errorf("exitOK = %d, expected 0", exitOK)
	}
	if exitError != 1 {
		t.Errorf("exitError = %d, expected 1", exitError)
	}
	if exitInvalid != 2 {
		t.Errorf("exitInvalid = %d, expected 2", exitInvalid)
	}
}
