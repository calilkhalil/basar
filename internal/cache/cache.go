// Package cache manages the local ISF banner cache with file locking.
package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/calilkhalil/basar/internal/config"
	"github.com/calilkhalil/basar/internal/fetcher"
)

const (
	// LockTimeout is max age of a stale lock file before override.
	LockTimeout = 5 * time.Minute

	// FileMode for created files.
	FileMode = 0644

	// DirMode for created directories.
	DirMode = 0755
)

// ErrLocked indicates another process holds the lock.
var ErrLocked = errors.New("cache is locked by another process")

// Stats contains cache statistics.
type Stats struct {
	Valid      bool      `json:"valid"`
	Path       string    `json:"path,omitempty"`
	Entries    int       `json:"entries,omitempty"`
	Size       int64     `json:"size,omitempty"`
	AgeSeconds int       `json:"age_seconds,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

// Cache manages the ISF banner cache.
type Cache struct {
	cfg     *config.Config
	fetcher *fetcher.Fetcher
}

// New creates a new Cache instance.
func New(cfg *config.Config) *Cache {
	return &Cache{
		cfg:     cfg,
		fetcher: fetcher.New(),
	}
}

// IsValid checks if cache exists and is within TTL.
func (c *Cache) IsValid() bool {
	info, err := os.Stat(c.cfg.CacheFile)
	if err != nil {
		return false
	}

	age := time.Since(info.ModTime())
	return age < c.cfg.TTL
}

// Path returns the cache file path if it exists.
func (c *Cache) Path() (string, bool) {
	if _, err := os.Stat(c.cfg.CacheFile); err != nil {
		return "", false
	}
	return c.cfg.CacheFile, true
}

// URI returns the file:// URI for volatility3 -u flag.
func (c *Cache) URI() (string, bool) {
	path, ok := c.Path()
	if !ok {
		return "", false
	}
	return "file://" + path, true
}

// Stats returns cache statistics.
func (c *Cache) Stats() Stats {
	info, err := os.Stat(c.cfg.CacheFile)
	if err != nil {
		return Stats{Valid: false}
	}

	data, err := os.ReadFile(c.cfg.CacheFile)
	if err != nil {
		return Stats{Valid: false}
	}

	var banners fetcher.BannerData
	if err := json.Unmarshal(data, &banners); err != nil {
		return Stats{Valid: false}
	}

	return Stats{
		Valid:      true,
		Path:       c.cfg.CacheFile,
		Entries:    len(banners.Linux),
		Size:       info.Size(),
		AgeSeconds: int(time.Since(info.ModTime()).Seconds()),
		UpdatedAt:  info.ModTime(),
	}
}

// loadMeta loads source metadata from cache.
func (c *Cache) loadMeta() *fetcher.MetaCache {
	metaFile := filepath.Join(c.cfg.CacheDir, "meta.json")
	data, err := os.ReadFile(metaFile)
	if err != nil {
		return &fetcher.MetaCache{Sources: make(map[string]fetcher.SourceMeta)}
	}

	var meta fetcher.MetaCache
	if err := json.Unmarshal(data, &meta); err != nil {
		return &fetcher.MetaCache{Sources: make(map[string]fetcher.SourceMeta)}
	}

	if meta.Sources == nil {
		meta.Sources = make(map[string]fetcher.SourceMeta)
	}

	return &meta
}

// saveMeta saves source metadata to cache.
func (c *Cache) saveMeta(meta *fetcher.MetaCache) error {
	metaFile := filepath.Join(c.cfg.CacheDir, "meta.json")

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metaFile, data, FileMode)
}

// SmartUpdate updates cache only if sources have changed.
// Returns: updated (bool), error
func (c *Cache) SmartUpdate(ctx context.Context, verbose bool) (bool, error) {
	if err := c.acquireLock(); err != nil {
		return false, err
	}
	defer c.releaseLock()

	meta := c.loadMeta()
	results := c.fetcher.FetchAllWithMeta(ctx, c.cfg.Sources, meta)

	var datasets []*fetcher.BannerData
	anyModified := false
	newMeta := &fetcher.MetaCache{Sources: make(map[string]fetcher.SourceMeta)}

	for _, r := range results {
		if r.Err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "source %s: %v\n", r.Source, r.Err)
			}
			// Keep old metadata for failed sources
			if old, ok := meta.Sources[r.Source]; ok {
				newMeta.Sources[r.Source] = old
			}
			continue
		}

		if r.Meta != nil {
			newMeta.Sources[r.Source] = *r.Meta
		}

		if r.Modified && r.Data != nil {
			datasets = append(datasets, r.Data)
			anyModified = true
			if verbose {
				fmt.Fprintf(os.Stderr, "source %s: updated\n", r.Source)
			}
		} else if !r.Modified {
			if verbose {
				fmt.Fprintf(os.Stderr, "source %s: not modified\n", r.Source)
			}
			// Load existing data for unmodified sources
			if existing := c.loadExistingBanners(); existing != nil {
				datasets = append(datasets, existing)
			}
		}
	}

	// Save metadata regardless
	c.saveMeta(newMeta)

	if !anyModified && c.IsValid() {
		return false, nil
	}

	if len(datasets) == 0 {
		return false, errors.New("all sources failed")
	}

	merged := fetcher.Merge(datasets)
	if err := c.write(merged); err != nil {
		return false, err
	}

	return anyModified, nil
}

// loadExistingBanners loads current cached banners.
func (c *Cache) loadExistingBanners() *fetcher.BannerData {
	data, err := os.ReadFile(c.cfg.CacheFile)
	if err != nil {
		return nil
	}

	var banners fetcher.BannerData
	if err := json.Unmarshal(data, &banners); err != nil {
		return nil
	}

	return &banners
}

// Update refreshes the cache from configured sources.
// If force is false, skips update if cache is valid.
func (c *Cache) Update(ctx context.Context, force bool) error {
	if !force && c.IsValid() {
		return nil
	}

	if err := c.acquireLock(); err != nil {
		return err
	}
	defer c.releaseLock()

	results := c.fetcher.FetchAll(ctx, c.cfg.Sources)

	var datasets []*fetcher.BannerData
	for _, r := range results {
		if r.Err != nil {
			continue
		}
		datasets = append(datasets, r.Data)
	}

	if len(datasets) == 0 {
		return errors.New("all sources failed")
	}

	merged := fetcher.Merge(datasets)

	return c.write(merged)
}

// Ensure guarantees a valid cache exists, updating if necessary.
func (c *Cache) Ensure(ctx context.Context) error {
	if c.IsValid() {
		return nil
	}
	return c.Update(ctx, false)
}

// acquireLock attempts to acquire an exclusive lock.
func (c *Cache) acquireLock() error {
	if err := os.MkdirAll(c.cfg.CacheDir, DirMode); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	info, err := os.Stat(c.cfg.LockFile)
	if err == nil {
		// Lock exists - check if stale
		if time.Since(info.ModTime()) < LockTimeout {
			return ErrLocked
		}
		// Stale lock - remove it
		os.Remove(c.cfg.LockFile)
	}

	pid := strconv.Itoa(os.Getpid())
	if err := os.WriteFile(c.cfg.LockFile, []byte(pid), FileMode); err != nil {
		return fmt.Errorf("creating lock: %w", err)
	}

	return nil
}

// releaseLock removes the lock file.
func (c *Cache) releaseLock() {
	os.Remove(c.cfg.LockFile)
}

// write atomically writes banner data to cache file.
func (c *Cache) write(data *fetcher.BannerData) error {
	if err := os.MkdirAll(c.cfg.CacheDir, DirMode); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	tmp := c.cfg.CacheFile + ".tmp"

	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, FileMode)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("encoding JSON: %w", err)
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return fmt.Errorf("syncing file: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("closing file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmp, c.cfg.CacheFile); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("renaming cache file: %w", err)
	}

	return nil
}

// Clear removes the cache file.
func (c *Cache) Clear() error {
	if err := os.Remove(c.cfg.CacheFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing cache: %w", err)
	}
	return nil
}

// ConfigureVolatility3 adds basar to volatility3 config.
func (c *Cache) ConfigureVolatility3() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	vol3Config := filepath.Join(home, ".volatility3.yaml")
	uri, ok := c.URI()
	if !ok {
		// Cache doesn't exist yet, use the expected path
		uri = "file://" + c.cfg.CacheFile
	}

	content := fmt.Sprintf("# Added by basar\nremote_isf_url: %s\n", uri)

	// Check if file exists
	if _, err := os.Stat(vol3Config); err == nil {
		// File exists, check if already configured
		existing, err := os.ReadFile(vol3Config)
		if err != nil {
			return fmt.Errorf("reading volatility3 config: %w", err)
		}

		if contains(string(existing), "remote_isf_url") {
			// Already has remote_isf_url, update it
			// For simplicity, just append a comment
			return fmt.Errorf("volatility3 config already has remote_isf_url, please update manually: %s", vol3Config)
		}

		// Append to existing file
		f, err := os.OpenFile(vol3Config, os.O_APPEND|os.O_WRONLY, FileMode)
		if err != nil {
			return fmt.Errorf("opening volatility3 config: %w", err)
		}
		defer f.Close()

		if _, err := f.WriteString("\n" + content); err != nil {
			return fmt.Errorf("writing volatility3 config: %w", err)
		}
	} else {
		// Create new file
		if err := os.WriteFile(vol3Config, []byte(content), FileMode); err != nil {
			return fmt.Errorf("creating volatility3 config: %w", err)
		}
	}

	return nil
}

// InstallService installs systemd user timer for automatic updates.
func (c *Cache) InstallService() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("systemd service only supported on Linux")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home dir: %w", err)
	}

	systemdDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(systemdDir, DirMode); err != nil {
		return fmt.Errorf("creating systemd dir: %w", err)
	}

	// Find basar binary
	basarPath, err := exec.LookPath("basar")
	if err != nil {
		// Try common locations
		basarPath = filepath.Join(home, ".local", "bin", "basar")
		if _, err := os.Stat(basarPath); err != nil {
			basarPath = "/usr/local/bin/basar"
		}
	}

	// Service file
	serviceContent := fmt.Sprintf(`[Unit]
Description=Update basar ISF symbol cache
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=%s --smart-update
Nice=19
IOSchedulingClass=idle

[Install]
WantedBy=default.target
`, basarPath)

	servicePath := filepath.Join(systemdDir, "basar.service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), FileMode); err != nil {
		return fmt.Errorf("writing service file: %w", err)
	}

	// Timer file - runs on 1st and 15th of each month
	timerContent := `[Unit]
Description=Update basar ISF symbol cache periodically

[Timer]
OnCalendar=*-*-01,15 06:00:00
RandomizedDelaySec=3600
Persistent=true

[Install]
WantedBy=timers.target
`

	timerPath := filepath.Join(systemdDir, "basar.timer")
	if err := os.WriteFile(timerPath, []byte(timerContent), FileMode); err != nil {
		return fmt.Errorf("writing timer file: %w", err)
	}

	// Enable and start timer
	if err := exec.Command("systemctl", "--user", "daemon-reload").Run(); err != nil {
		return fmt.Errorf("daemon-reload failed: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "enable", "basar.timer").Run(); err != nil {
		return fmt.Errorf("enabling timer failed: %w", err)
	}

	if err := exec.Command("systemctl", "--user", "start", "basar.timer").Run(); err != nil {
		return fmt.Errorf("starting timer failed: %w", err)
	}

	return nil
}

// Setup performs complete setup: config, update, vol3 config, service.
func (c *Cache) Setup(ctx context.Context, verbose bool) error {
	// 1. Initialize config if needed
	if _, err := os.Stat(c.cfg.ConfigFile); os.IsNotExist(err) {
		if err := c.cfg.InitConfig(); err != nil {
			return fmt.Errorf("creating config: %w", err)
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "created config: %s\n", c.cfg.ConfigFile)
		}
	}

	// 2. Initial update
	if verbose {
		fmt.Fprintf(os.Stderr, "updating cache from %d sources...\n", len(c.cfg.Sources))
	}
	if err := c.Update(ctx, true); err != nil {
		return fmt.Errorf("updating cache: %w", err)
	}
	if verbose {
		stats := c.Stats()
		fmt.Fprintf(os.Stderr, "cached %d banners\n", stats.Entries)
	}

	// 3. Configure volatility3
	if err := c.ConfigureVolatility3(); err != nil {
		if verbose {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		}
	} else if verbose {
		fmt.Fprintf(os.Stderr, "configured volatility3\n")
	}

	// 4. Install systemd service (Linux only)
	if runtime.GOOS == "linux" {
		if err := c.InstallService(); err != nil {
			if verbose {
				fmt.Fprintf(os.Stderr, "warning: service install failed: %v\n", err)
			}
		} else if verbose {
			fmt.Fprintf(os.Stderr, "installed systemd timer (runs twice monthly)\n")
		}
	}

	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
