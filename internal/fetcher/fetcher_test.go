package fetcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendUnique(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		new      []string
		expected []string
	}{
		{
			name:     "empty slices",
			existing: []string{},
			new:      []string{},
			expected: []string{},
		},
		{
			name:     "no duplicates",
			existing: []string{"a", "b"},
			new:      []string{"c", "d"},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "with duplicates",
			existing: []string{"a", "b"},
			new:      []string{"b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "all duplicates",
			existing: []string{"a", "b"},
			new:      []string{"a", "b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "empty existing",
			existing: []string{},
			new:      []string{"a", "b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "empty new",
			existing: []string{"a", "b"},
			new:      []string{},
			expected: []string{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendUnique(tt.existing, tt.new)
			if len(result) != len(tt.expected) {
				t.Errorf("expected length %d, got %d", len(tt.expected), len(result))
			}
			for i, v := range tt.expected {
				if i >= len(result) || result[i] != v {
					t.Errorf("expected[%d] = %q, got %q", i, v, result[i])
				}
			}
		})
	}
}

func TestMerge(t *testing.T) {
	tests := []struct {
		name     string
		datasets []*BannerData
		expected *BannerData
	}{
		{
			name:     "empty datasets",
			datasets: []*BannerData{},
			expected: &BannerData{
				Version: 1,
				Linux:   make(map[string][]string),
			},
		},
		{
			name: "single dataset",
			datasets: []*BannerData{
				{
					Version: 1,
					Linux: map[string][]string{
						"banner1": {"url1", "url2"},
					},
				},
			},
			expected: &BannerData{
				Version: 1,
				Linux: map[string][]string{
					"banner1": {"url1", "url2"},
				},
			},
		},
		{
			name: "multiple datasets with unique banners",
			datasets: []*BannerData{
				{
					Version: 1,
					Linux: map[string][]string{
						"banner1": {"url1"},
					},
				},
				{
					Version: 1,
					Linux: map[string][]string{
						"banner2": {"url2"},
					},
				},
			},
			expected: &BannerData{
				Version: 1,
				Linux: map[string][]string{
					"banner1": {"url1"},
					"banner2": {"url2"},
				},
			},
		},
		{
			name: "multiple datasets with overlapping banners",
			datasets: []*BannerData{
				{
					Version: 1,
					Linux: map[string][]string{
						"banner1": {"url1", "url2"},
					},
				},
				{
					Version: 1,
					Linux: map[string][]string{
						"banner1": {"url2", "url3"},
					},
				},
			},
			expected: &BannerData{
				Version: 1,
				Linux: map[string][]string{
					"banner1": {"url1", "url2", "url3"},
				},
			},
		},
		{
			name: "nil datasets are skipped",
			datasets: []*BannerData{
				nil,
				{
					Version: 1,
					Linux: map[string][]string{
						"banner1": {"url1"},
					},
				},
				nil,
			},
			expected: &BannerData{
				Version: 1,
				Linux: map[string][]string{
					"banner1": {"url1"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Merge(tt.datasets)

			if result.Version != tt.expected.Version {
				t.Errorf("expected version %d, got %d", tt.expected.Version, result.Version)
			}

			if len(result.Linux) != len(tt.expected.Linux) {
				t.Errorf("expected %d banners, got %d", len(tt.expected.Linux), len(result.Linux))
			}

			for banner, expectedURLs := range tt.expected.Linux {
				resultURLs, ok := result.Linux[banner]
				if !ok {
					t.Errorf("expected banner %q not found", banner)
					continue
				}

				if len(resultURLs) != len(expectedURLs) {
					t.Errorf("banner %q: expected %d URLs, got %d", banner, len(expectedURLs), len(resultURLs))
					continue
				}

				for i, expectedURL := range expectedURLs {
					if i >= len(resultURLs) || resultURLs[i] != expectedURL {
						t.Errorf("banner %q URL[%d]: expected %q, got %q", banner, i, expectedURL, resultURLs[i])
					}
				}
			}
		})
	}
}

func TestIsLocalPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"file:// URL", "file:///path/to/file", true},
		{"absolute path", "/path/to/file", true},
		{"home path", "~/path/to/file", true},
		{"relative path", "path/to/file", true},
		{"HTTP URL", "http://example.com/file", false},
		{"HTTPS URL", "https://example.com/file", false},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalPath(tt.path)
			if result != tt.expected {
				t.Errorf("isLocalPath(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// --- HTTP Mock Server Tests ---

func TestNew(t *testing.T) {
	f := New()

	if f == nil {
		t.Fatal("New() returned nil")
	}

	if f.client == nil {
		t.Error("client not initialized")
	}

	if f.client.Timeout != HTTPTimeout {
		t.Errorf("client timeout = %v, expected %v", f.client.Timeout, HTTPTimeout)
	}
}

func TestFetchHTTP(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify User-Agent header
		if r.Header.Get("User-Agent") != UserAgent {
			t.Errorf("User-Agent = %q, expected %q", r.Header.Get("User-Agent"), UserAgent)
		}

		data := &BannerData{
			Version: 1,
			Linux: map[string][]string{
				"Linux version 5.15.0": {"https://example.com/5.15.0.json"},
			},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	f := New()
	ctx := context.Background()

	data, err := f.Fetch(ctx, server.URL)
	if err != nil {
		t.Fatalf("Fetch() failed: %v", err)
	}

	if data.Version != 1 {
		t.Errorf("Version = %d, expected 1", data.Version)
	}

	if len(data.Linux) != 1 {
		t.Errorf("Linux banners count = %d, expected 1", len(data.Linux))
	}
}

func TestFetchHTTPNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	f := New()
	ctx := context.Background()

	_, err := f.Fetch(ctx, server.URL)
	if err == nil {
		t.Error("Fetch() should fail on 404")
	}
}

func TestFetchHTTPInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	f := New()
	ctx := context.Background()

	_, err := f.Fetch(ctx, server.URL)
	if err == nil {
		t.Error("Fetch() should fail on invalid JSON")
	}
}

func TestFetchHTTPServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	f := New()
	ctx := context.Background()

	_, err := f.Fetch(ctx, server.URL)
	if err == nil {
		t.Error("Fetch() should fail on 500")
	}
}

func TestFetchHTTPContextCanceled(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(&BannerData{Version: 1})
	}))
	defer server.Close()

	f := New()

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.Fetch(ctx, server.URL)
	if err == nil {
		t.Error("Fetch() should fail with canceled context")
	}
}

func TestFetchLocal(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	data := &BannerData{
		Version: 1,
		Linux: map[string][]string{
			"banner1": {"url1"},
		},
	}

	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := json.NewEncoder(f).Encode(data); err != nil {
		t.Fatalf("failed to encode data: %v", err)
	}
	_ = f.Close()

	fetcher := New()
	ctx := context.Background()

	result, err := fetcher.Fetch(ctx, testFile)
	if err != nil {
		t.Fatalf("Fetch() failed: %v", err)
	}

	if result.Version != 1 {
		t.Errorf("Version = %d, expected 1", result.Version)
	}
}

func TestFetchLocalFileURL(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.json")

	data := &BannerData{
		Version: 1,
		Linux:   map[string][]string{},
	}

	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := json.NewEncoder(f).Encode(data); err != nil {
		t.Fatalf("failed to encode data: %v", err)
	}
	_ = f.Close()

	fetcher := New()
	ctx := context.Background()

	// Use file:// URL
	result, err := fetcher.Fetch(ctx, "file://"+testFile)
	if err != nil {
		t.Fatalf("Fetch(file://...) failed: %v", err)
	}

	if result.Version != 1 {
		t.Errorf("Version = %d, expected 1", result.Version)
	}
}

func TestFetchLocalNotFound(t *testing.T) {
	fetcher := New()
	ctx := context.Background()

	_, err := fetcher.Fetch(ctx, "/nonexistent/path/to/file.json")
	if err == nil {
		t.Error("Fetch() should fail on non-existent file")
	}
}

func TestFetchLocalInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(testFile, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	fetcher := New()
	ctx := context.Background()

	_, err := fetcher.Fetch(ctx, testFile)
	if err == nil {
		t.Error("Fetch() should fail on invalid JSON")
	}
}

func TestFetchAll(t *testing.T) {
	// Create two test servers
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := &BannerData{
			Version: 1,
			Linux: map[string][]string{
				"banner1": {"url1"},
			},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := &BannerData{
			Version: 1,
			Linux: map[string][]string{
				"banner2": {"url2"},
			},
		}
		json.NewEncoder(w).Encode(data)
	}))
	defer server2.Close()

	f := New()
	ctx := context.Background()

	sources := []string{server1.URL, server2.URL}
	results := f.FetchAll(ctx, sources)

	if len(results) != 2 {
		t.Fatalf("FetchAll() returned %d results, expected 2", len(results))
	}

	for i, r := range results {
		if r.Err != nil {
			t.Errorf("results[%d].Err = %v, expected nil", i, r.Err)
		}
		if r.Data == nil {
			t.Errorf("results[%d].Data is nil", i)
		}
	}
}

func TestFetchAllWithFailure(t *testing.T) {
	// One working server, one failing
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := &BannerData{Version: 1, Linux: map[string][]string{}}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	f := New()
	ctx := context.Background()

	sources := []string{server.URL, "http://invalid.localhost:99999"}
	results := f.FetchAll(ctx, sources)

	if len(results) != 2 {
		t.Fatalf("FetchAll() returned %d results, expected 2", len(results))
	}

	// First should succeed
	if results[0].Err != nil {
		t.Errorf("results[0].Err = %v, expected nil", results[0].Err)
	}

	// Second should fail
	if results[1].Err == nil {
		t.Error("results[1].Err should not be nil")
	}
}

func TestFetchAllEmpty(t *testing.T) {
	f := New()
	ctx := context.Background()

	results := f.FetchAll(ctx, []string{})

	if len(results) != 0 {
		t.Errorf("FetchAll([]) returned %d results, expected 0", len(results))
	}
}

func TestFetchAllPreservesOrder(t *testing.T) {
	// Create servers that respond with their index
	servers := make([]*httptest.Server, 3)
	for i := range servers {
		idx := i
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add delay to make concurrent behavior visible
			time.Sleep(time.Duration(2-idx) * 10 * time.Millisecond)
			data := &BannerData{
				Version: idx + 1,
				Linux:   map[string][]string{},
			}
			json.NewEncoder(w).Encode(data)
		}))
		defer servers[i].Close()
	}

	f := New()
	ctx := context.Background()

	sources := make([]string, len(servers))
	for i, s := range servers {
		sources[i] = s.URL
	}

	results := f.FetchAll(ctx, sources)

	// Results should be in the same order as sources
	for i, r := range results {
		if r.Source != sources[i] {
			t.Errorf("results[%d].Source = %q, expected %q", i, r.Source, sources[i])
		}
		if r.Err == nil && r.Data.Version != i+1 {
			t.Errorf("results[%d].Data.Version = %d, expected %d", i, r.Data.Version, i+1)
		}
	}
}

func TestFetchLocalHomePath(t *testing.T) {
	// Get actual home directory
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home directory")
	}

	// Create test file in temp dir within home (or skip if not writable)
	tmpDir, err := os.MkdirTemp(home, "basar-test-*")
	if err != nil {
		t.Skip("cannot create temp dir in home")
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.json")
	data := &BannerData{Version: 1, Linux: map[string][]string{}}
	f, _ := os.Create(testFile)
	_ = json.NewEncoder(f).Encode(data)
	_ = f.Close()

	// Create path relative to home
	relPath := "~" + testFile[len(home):]

	fetcher := New()
	ctx := context.Background()

	result, err := fetcher.Fetch(ctx, relPath)
	if err != nil {
		t.Fatalf("Fetch(%q) failed: %v", relPath, err)
	}

	if result.Version != 1 {
		t.Errorf("Version = %d, expected 1", result.Version)
	}
}

// --- Conditional Request Tests ---

func TestFetchHTTPWithETag(t *testing.T) {
	etag := `"abc123"`
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// Check if client sent If-None-Match
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		// First request - send data with ETag
		w.Header().Set("ETag", etag)
		data := &BannerData{Version: 1, Linux: map[string][]string{}}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	f := New()
	ctx := context.Background()

	// First fetch - should get data
	data, meta, modified, err := f.FetchWithMeta(ctx, server.URL, nil)
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	if data == nil {
		t.Fatal("first fetch should return data")
	}
	if !modified {
		t.Error("first fetch should be modified")
	}
	if meta.ETag != etag {
		t.Errorf("ETag = %q, expected %q", meta.ETag, etag)
	}

	// Second fetch with meta - should get 304
	data2, meta2, modified2, err := f.FetchWithMeta(ctx, server.URL, meta)
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	if data2 != nil {
		t.Error("second fetch should return nil data on 304")
	}
	if modified2 {
		t.Error("second fetch should not be modified")
	}
	if meta2 != meta {
		t.Error("second fetch should return same meta on 304")
	}

	if callCount != 2 {
		t.Errorf("expected 2 server calls, got %d", callCount)
	}
}

func TestFetchHTTPWithLastModified(t *testing.T) {
	lastMod := "Wed, 01 Jan 2025 00:00:00 GMT"
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if r.Header.Get("If-Modified-Since") == lastMod {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Last-Modified", lastMod)
		data := &BannerData{Version: 1, Linux: map[string][]string{}}
		json.NewEncoder(w).Encode(data)
	}))
	defer server.Close()

	f := New()
	ctx := context.Background()

	// First fetch
	data, meta, _, err := f.FetchWithMeta(ctx, server.URL, nil)
	if err != nil {
		t.Fatalf("first fetch failed: %v", err)
	}
	if data == nil {
		t.Fatal("first fetch should return data")
	}
	if meta.LastModified != lastMod {
		t.Errorf("LastModified = %q, expected %q", meta.LastModified, lastMod)
	}

	// Second fetch - should get 304
	_, _, modified, err := f.FetchWithMeta(ctx, server.URL, meta)
	if err != nil {
		t.Fatalf("second fetch failed: %v", err)
	}
	if modified {
		t.Error("second fetch should not be modified")
	}
}

func TestFetchAllWithMeta(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"s1"`)
		data := &BannerData{Version: 1, Linux: map[string][]string{"b1": {"u1"}}}
		json.NewEncoder(w).Encode(data)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"s2"`)
		data := &BannerData{Version: 1, Linux: map[string][]string{"b2": {"u2"}}}
		json.NewEncoder(w).Encode(data)
	}))
	defer server2.Close()

	f := New()
	ctx := context.Background()

	sources := []string{server1.URL, server2.URL}
	results := f.FetchAllWithMeta(ctx, sources, nil)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for i, r := range results {
		if r.Err != nil {
			t.Errorf("results[%d].Err = %v", i, r.Err)
		}
		if r.Data == nil {
			t.Errorf("results[%d].Data is nil", i)
		}
		if r.Meta == nil {
			t.Errorf("results[%d].Meta is nil", i)
		}
		if !r.Modified {
			t.Errorf("results[%d].Modified should be true", i)
		}
	}
}

func TestMetaCache(t *testing.T) {
	meta := &MetaCache{
		Sources: map[string]SourceMeta{
			"http://example.com/1": {ETag: `"abc"`, UpdatedAt: time.Now()},
			"http://example.com/2": {LastModified: "Wed, 01 Jan 2025", UpdatedAt: time.Now()},
		},
	}

	if len(meta.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(meta.Sources))
	}

	if meta.Sources["http://example.com/1"].ETag != `"abc"` {
		t.Error("ETag not stored correctly")
	}
}
