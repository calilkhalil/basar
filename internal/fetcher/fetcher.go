// Package fetcher handles fetching banner data from URLs and local files.
package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// HTTPTimeout is the default timeout for HTTP requests.
	HTTPTimeout = 30 * time.Second

	// UserAgent identifies this tool in HTTP requests.
	UserAgent = "basar/1.0"
)

// BannerData represents the volatility3 ISF banner format.
type BannerData struct {
	Version int                 `json:"version"`
	Linux   map[string][]string `json:"linux"`
}

// SourceMeta stores metadata for conditional requests.
type SourceMeta struct {
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// MetaCache stores metadata for all sources.
type MetaCache struct {
	Sources map[string]SourceMeta `json:"sources"`
}

// Result contains the fetch result for a single source.
type Result struct {
	Source   string
	Data     *BannerData
	Meta     *SourceMeta
	Modified bool // true if content changed, false if 304 Not Modified
	Err      error
}

// Fetcher fetches banner data from multiple sources.
type Fetcher struct {
	client *http.Client
}

// New creates a new Fetcher with default HTTP client.
func New() *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: HTTPTimeout,
		},
	}
}

// FetchAll fetches from all sources concurrently.
func (f *Fetcher) FetchAll(ctx context.Context, sources []string) []Result {
	return f.FetchAllWithMeta(ctx, sources, nil)
}

// FetchAllWithMeta fetches from all sources concurrently with conditional requests.
func (f *Fetcher) FetchAllWithMeta(ctx context.Context, sources []string, meta *MetaCache) []Result {
	results := make([]Result, len(sources))
	var wg sync.WaitGroup

	for i, src := range sources {
		wg.Add(1)
		go func(idx int, source string) {
			defer wg.Done()
			var srcMeta *SourceMeta
			if meta != nil && meta.Sources != nil {
				if m, ok := meta.Sources[source]; ok {
					srcMeta = &m
				}
			}
			data, newMeta, modified, err := f.FetchWithMeta(ctx, source, srcMeta)
			results[idx] = Result{
				Source:   source,
				Data:     data,
				Meta:     newMeta,
				Modified: modified,
				Err:      err,
			}
		}(i, src)
	}

	wg.Wait()
	return results
}

// Fetch retrieves banner data from a single source (URL or local file).
func (f *Fetcher) Fetch(ctx context.Context, source string) (*BannerData, error) {
	data, _, _, err := f.FetchWithMeta(ctx, source, nil)
	return data, err
}

// FetchWithMeta retrieves banner data with conditional request support.
// Returns: data, metadata, modified (false if 304), error
func (f *Fetcher) FetchWithMeta(ctx context.Context, source string, meta *SourceMeta) (*BannerData, *SourceMeta, bool, error) {
	if isLocalPath(source) {
		data, err := f.fetchLocal(source)
		if err != nil {
			return nil, nil, false, err
		}
		return data, &SourceMeta{UpdatedAt: time.Now()}, true, nil
	}
	return f.fetchHTTPWithMeta(ctx, source, meta)
}

// isLocalPath determines if the source is a local file path.
func isLocalPath(source string) bool {
	if strings.HasPrefix(source, "file://") {
		return true
	}
	if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "~") {
		return true
	}
	if !strings.Contains(source, "://") {
		return true
	}
	return false
}

// fetchLocal reads banner data from a local file.
func (f *Fetcher) fetchLocal(source string) (*BannerData, error) {
	path := source
	path = strings.TrimPrefix(path, "file://")

	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("expanding home dir: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer file.Close()

	var data BannerData
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, fmt.Errorf("decoding JSON: %w", err)
	}

	return &data, nil
}

// fetchHTTPWithMeta retrieves banner data via HTTP(S) with conditional request support.
func (f *Fetcher) fetchHTTPWithMeta(ctx context.Context, url string, meta *SourceMeta) (*BannerData, *SourceMeta, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, false, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", UserAgent)

	// Add conditional headers if we have metadata
	if meta != nil {
		if meta.ETag != "" {
			req.Header.Set("If-None-Match", meta.ETag)
		}
		if meta.LastModified != "" {
			req.Header.Set("If-Modified-Since", meta.LastModified)
		}
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, nil, false, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Not modified - return nil data but no error
	if resp.StatusCode == http.StatusNotModified {
		return nil, meta, false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, nil, false, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var data BannerData
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, nil, false, fmt.Errorf("decoding response: %w", err)
	}

	// Store new metadata
	newMeta := &SourceMeta{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		UpdatedAt:    time.Now(),
	}

	return &data, newMeta, true, nil
}

// Merge combines multiple BannerData into one, deduplicating URLs per banner.
func Merge(datasets []*BannerData) *BannerData {
	merged := &BannerData{
		Version: 1,
		Linux:   make(map[string][]string),
	}

	for _, data := range datasets {
		if data == nil {
			continue
		}

		for banner, urls := range data.Linux {
			merged.Linux[banner] = appendUnique(merged.Linux[banner], urls)
		}
	}

	return merged
}

// appendUnique appends items to slice, skipping duplicates.
func appendUnique(existing, new []string) []string {
	seen := make(map[string]struct{}, len(existing))
	for _, v := range existing {
		seen[v] = struct{}{}
	}

	result := existing
	for _, v := range new {
		if _, ok := seen[v]; !ok {
			result = append(result, v)
			seen[v] = struct{}{}
		}
	}

	return result
}
