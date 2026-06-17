package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// ==================== MatchPattern ====================

func TestMatchPattern_Wildcard(t *testing.T) {
	tests := []struct {
		url     string
		pattern string
		want    bool
	}{
		// Match all
		{"https://example.com/api/users", "*", true},
		{"", "*", true},

		// Exact match (no wildcard)
		{"https://example.com/api", "https://example.com/api", true},
		{"https://example.com/api", "https://example.com/other", false},

		// Trailing wildcard
		{"https://example.com/api/users", "https://example.com/api/*", true},
		{"https://example.com/api/users/123", "https://example.com/api/*", true},
		{"https://example.com/other/users", "https://example.com/api/*", false},

		// Leading wildcard
		{"https://example.com/api/users", "*/api/users", true},
		{"https://other.com/api/users", "*/api/users", true},
		{"https://example.com/api/other", "*/api/users", false},

		// Both sides wildcard
		{"https://example.com/api/users/123", "*/api/*", true},
		{"https://other.com/api/v2/data", "*/api/*", true},
		{"https://example.com/web/users", "*/api/*", false},

		// Multiple wildcards
		{"https://example.com/api/v1/users", "*/api/*/users", true},
		{"https://example.com/api/v2/users", "*/api/*/users", true},
		{"https://example.com/api/v1/posts", "*/api/*/users", false},

		// Pattern without trailing * must match end exactly
		{"https://example.com/api/users", "*/api/users", true},
		{"https://example.com/api/users/extra", "*/api/users", false},
	}

	for _, tt := range tests {
		got := MatchPattern(tt.url, tt.pattern)
		if got != tt.want {
			t.Errorf("MatchPattern(%q, %q) = %v, want %v", tt.url, tt.pattern, got, tt.want)
		}
	}
}

// ==================== applyMapRemote ====================

func TestApplyMapRemote(t *testing.T) {
	tests := []struct {
		name          string
		sourcePattern string
		targetURL     string
		requestURL    string
		want          string
	}{
		{
			name:          "no wildcard - return target as-is",
			sourcePattern: "https://prod.com/api",
			targetURL:     "https://staging.com/api",
			requestURL:    "https://prod.com/api",
			want:          "https://staging.com/api",
		},
		{
			name:          "wildcard source, wildcard target - path preserved",
			sourcePattern: "https://prod.com/api/*",
			targetURL:     "https://staging.com/api/*",
			requestURL:    "https://prod.com/api/users/123",
			want:          "https://staging.com/api/users/123",
		},
		{
			name:          "wildcard source, no wildcard target",
			sourcePattern: "https://prod.com/api/*",
			targetURL:     "https://staging.com/newapi",
			requestURL:    "https://prod.com/api/users",
			want:          "https://staging.com/newapi",
		},
		{
			name:          "leading wildcard source",
			sourcePattern: "*/api/v1/*",
			targetURL:     "http://localhost:3000/api/v1/*",
			requestURL:    "https://prod.com/api/v1/users?page=1",
			want:          "http://localhost:3000/api/v1/users?page=1",
		},
		{
			name:          "target ends with slash - append tail",
			sourcePattern: "*/api/*",
			targetURL:     "http://localhost:3000/",
			requestURL:    "https://prod.com/api/users/123",
			want:          "http://localhost:3000/users/123",
		},
		{
			name:          "prefix not found in URL - return target",
			sourcePattern: "*/notfound/*",
			targetURL:     "http://localhost/fallback",
			requestURL:    "https://example.com/api/data",
			want:          "http://localhost/fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyMapRemote(tt.sourcePattern, tt.targetURL, tt.requestURL)
			if got != tt.want {
				t.Errorf("applyMapRemote(%q, %q, %q) = %q, want %q",
					tt.sourcePattern, tt.targetURL, tt.requestURL, got, tt.want)
			}
		})
	}
}

// ==================== analyzeBodyFull ====================

func TestAnalyzeBodyFull_PlainText(t *testing.T) {
	p := &ProxyServer{}
	result := p.analyzeBodyFull([]byte("hello world"), "", "text/plain")
	if result.Text != "hello world" {
		t.Errorf("Expected 'hello world', got %q", result.Text)
	}
	if result.IsBinary {
		t.Error("Plain text should not be binary")
	}
}

func TestAnalyzeBodyFull_EmptyBody(t *testing.T) {
	p := &ProxyServer{}
	result := p.analyzeBodyFull(nil, "", "")
	if result.Text != "" {
		t.Errorf("Expected empty text for nil body, got %q", result.Text)
	}
}

func TestAnalyzeBodyFull_Gzip(t *testing.T) {
	original := "gzip compressed content"
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte(original))
	gw.Close()

	p := &ProxyServer{}
	result := p.analyzeBodyFull(buf.Bytes(), "gzip", "text/plain")
	if result.Text != original {
		t.Errorf("Expected %q after gzip decompression, got %q", original, result.Text)
	}
}

func TestAnalyzeBodyFull_Brotli(t *testing.T) {
	original := "brotli compressed content"
	var buf bytes.Buffer
	bw := brotli.NewWriter(&buf)
	bw.Write([]byte(original))
	bw.Close()

	p := &ProxyServer{}
	result := p.analyzeBodyFull(buf.Bytes(), "br", "text/plain")
	if result.Text != original {
		t.Errorf("Expected %q after brotli decompression, got %q", original, result.Text)
	}
}

func TestAnalyzeBodyFull_Deflate(t *testing.T) {
	original := "deflate compressed content"
	var buf bytes.Buffer
	fw, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	fw.Write([]byte(original))
	fw.Close()

	p := &ProxyServer{}
	result := p.analyzeBodyFull(buf.Bytes(), "deflate", "text/plain")
	if result.Text != original {
		t.Errorf("Expected %q after deflate decompression, got %q", original, result.Text)
	}
}

func TestAnalyzeBodyFull_Zstd(t *testing.T) {
	original := "zstd compressed content"
	var buf bytes.Buffer
	encoder, _ := zstd.NewWriter(&buf)
	encoder.Write([]byte(original))
	encoder.Close()

	p := &ProxyServer{}
	result := p.analyzeBodyFull(buf.Bytes(), "zstd", "text/plain")
	if result.Text != original {
		t.Errorf("Expected %q after zstd decompression, got %q", original, result.Text)
	}
}

func TestAnalyzeBodyFull_ZstdDictMonitor(t *testing.T) {
	// Test zstd/dict_monitor encoding hint
	original := "test data for dict_monitor"
	var buf bytes.Buffer
	encoder, _ := zstd.NewWriter(&buf)
	encoder.Write([]byte(original))
	encoder.Close()

	p := &ProxyServer{}
	// Should decompress successfully for standard zstd
	result := p.analyzeBodyFull(buf.Bytes(), "zstd/dict_monitor", "text/plain")
	if result.Text != original {
		t.Errorf("Expected %q after zstd/dict_monitor decompression, got %q", original, result.Text)
	}
}

func TestAnalyzeBodyFull_ZstdAutoDetect(t *testing.T) {
	// Test auto-detection of zstd without Content-Encoding header
	original := "zstd auto-detect content"
	var buf bytes.Buffer
	encoder, _ := zstd.NewWriter(&buf)
	encoder.Write([]byte(original))
	encoder.Close()
	compressed := buf.Bytes()

	// Verify magic number is present
	if len(compressed) < 4 || compressed[0] != 0x28 || compressed[1] != 0xB5 || compressed[2] != 0x2F || compressed[3] != 0xFD {
		t.Fatal("Zstd compressed data should have magic number 0x28 0xB5 0x2F 0xFD")
	}

	p := &ProxyServer{}
	// Pass empty encoding to trigger auto-detection
	result := p.analyzeBodyFull(compressed, "", "application/octet-stream")
	if result.Text != original {
		t.Errorf("Expected %q after zstd auto-detection, got %q", original, result.Text)
	}
}

func TestAnalyzeBodyFull_ZstdCorrupted(t *testing.T) {
	// Test corrupted zstd data with valid magic number but invalid content
	p := &ProxyServer{}
	// Create data with zstd magic number but corrupted payload
	corruptedData := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x00, 0x01, 0x02, 0x03} // magic + garbage

	result := p.analyzeBodyFull(corruptedData, "", "application/octet-stream")
	// Should detect zstd but fail to decompress
	if !result.IsBinary {
		t.Error("Corrupted zstd data should be marked as binary")
	}
	if !bytes.Contains([]byte(result.Text), []byte("zstd detected but decompression failed")) {
		t.Errorf("Expected error message about zstd decompression failure, got: %s", result.Text)
	}
}

func TestAnalyzeBodyFull_ZstdDictRequired(t *testing.T) {
	// Test zstd with dict_monitor encoding hint
	p := &ProxyServer{}
	// Simulate data that might need a dictionary - using invalid zstd data
	fakeCompressed := []byte{0x28, 0xB5, 0x2F, 0xFD, 0xFF, 0xFF} // magic + invalid

	result := p.analyzeBodyFull(fakeCompressed, "zstd/dict_monitor", "application/octet-stream")
	if !result.IsBinary {
		t.Error("Zstd data requiring dictionary should be marked as binary")
	}
	if !bytes.Contains([]byte(result.Text), []byte("requires dictionary")) {
		t.Errorf("Expected error message about dictionary requirement, got: %s", result.Text)
	}
}

func TestAnalyzeBodyFull_BinaryAfterDecompression(t *testing.T) {
	// Test data that decompresses successfully but result is binary (e.g. Protobuf)
	p := &ProxyServer{}
	// Create data with null bytes
	binaryData := []byte{0x00, 0x01, 0x02, 0x03, 0x00, 0x05}

	// Compress it with gzip
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write(binaryData)
	gw.Close()
	compressed := buf.Bytes()

	result := p.analyzeBodyFull(compressed, "gzip", "application/octet-stream")
	if !result.IsBinary {
		t.Error("Binary data after decompression should be marked as binary")
	}
	if !bytes.Contains([]byte(result.Text), []byte("after decompression")) {
		t.Errorf("Expected message about binary data after decompression, got: %s", result.Text)
	}
}

func TestAnalyzeBodyFull_BinaryDetection(t *testing.T) {
	data := []byte{0x00, 0x01, 0x02, 0x03}
	p := &ProxyServer{}
	result := p.analyzeBodyFull(data, "", "application/octet-stream")
	if !result.IsBinary {
		t.Error("Data with null bytes should be detected as binary")
	}
	if result.RawBytes == nil {
		t.Error("Binary data should have RawBytes set")
	}
}

// ==================== applyRewriteRules ====================

func TestApplyRewriteRules_BodyRewrite(t *testing.T) {
	p := &ProxyServer{
		rewriteRules: map[string]*RewriteRule{
			"r1": {
				ID:         "r1",
				URLPattern: "*",
				Phase:      "response",
				Target:     "body",
				Match:      "old-value",
				Replace:    "new-value",
				Enabled:    true,
			},
		},
	}

	body := []byte(`{"key": "old-value"}`)
	headers := http.Header{}
	newBody, headerMods := p.applyRewriteRules("GET", "https://example.com/api", "response", headers, body)

	expected := `{"key": "new-value"}`
	if string(newBody) != expected {
		t.Errorf("Expected body %q, got %q", expected, string(newBody))
	}
	if len(headerMods) != 0 {
		t.Error("Should have no header modifications")
	}
}

func TestApplyRewriteRules_HeaderRewrite(t *testing.T) {
	p := &ProxyServer{
		rewriteRules: map[string]*RewriteRule{
			"r1": {
				ID:         "r1",
				URLPattern: "*",
				Phase:      "response",
				Target:     "header",
				HeaderName: "Server",
				Match:      ".*",
				Replace:    "Gaze-Proxy",
				Enabled:    true,
			},
		},
	}

	headers := http.Header{}
	headers.Set("Server", "nginx/1.24")
	body := []byte("unchanged body")
	newBody, headerMods := p.applyRewriteRules("GET", "https://example.com/api", "response", headers, body)

	if string(newBody) != "unchanged body" {
		t.Error("Body should not be modified for header rewrite")
	}
	if headerMods["Server"] != "Gaze-Proxy" {
		t.Errorf("Expected Server header to be 'Gaze-Proxy', got %q", headerMods["Server"])
	}
}

func TestApplyRewriteRules_PhaseFiltering(t *testing.T) {
	p := &ProxyServer{
		rewriteRules: map[string]*RewriteRule{
			"r1": {
				ID:         "r1",
				URLPattern: "*",
				Phase:      "request",
				Target:     "body",
				Match:      "secret",
				Replace:    "***",
				Enabled:    true,
			},
		},
	}

	body := []byte("my secret data")
	headers := http.Header{}

	// Should NOT apply in response phase
	newBody, _ := p.applyRewriteRules("GET", "https://example.com", "response", headers, body)
	if string(newBody) != "my secret data" {
		t.Error("Request-phase rule should not apply in response phase")
	}

	// Should apply in request phase
	newBody, _ = p.applyRewriteRules("GET", "https://example.com", "request", headers, body)
	if string(newBody) != "my *** data" {
		t.Errorf("Expected 'my *** data', got %q", string(newBody))
	}
}

func TestApplyRewriteRules_BothPhase(t *testing.T) {
	p := &ProxyServer{
		rewriteRules: map[string]*RewriteRule{
			"r1": {
				ID:         "r1",
				URLPattern: "*",
				Phase:      "both",
				Target:     "body",
				Match:      "token",
				Replace:    "REDACTED",
				Enabled:    true,
			},
		},
	}

	body := []byte("bearer token here")
	headers := http.Header{}

	newBody, _ := p.applyRewriteRules("GET", "https://example.com", "request", headers, body)
	if string(newBody) != "bearer REDACTED here" {
		t.Errorf("'both' phase should apply in request, got %q", string(newBody))
	}

	newBody, _ = p.applyRewriteRules("GET", "https://example.com", "response", headers, body)
	if string(newBody) != "bearer REDACTED here" {
		t.Errorf("'both' phase should apply in response, got %q", string(newBody))
	}
}

func TestApplyRewriteRules_DisabledRule(t *testing.T) {
	p := &ProxyServer{
		rewriteRules: map[string]*RewriteRule{
			"r1": {
				ID:         "r1",
				URLPattern: "*",
				Phase:      "response",
				Target:     "body",
				Match:      "old",
				Replace:    "new",
				Enabled:    false,
			},
		},
	}

	body := []byte("old data")
	headers := http.Header{}
	newBody, _ := p.applyRewriteRules("GET", "https://example.com", "response", headers, body)
	if string(newBody) != "old data" {
		t.Error("Disabled rule should not apply")
	}
}

func TestApplyRewriteRules_MethodFiltering(t *testing.T) {
	p := &ProxyServer{
		rewriteRules: map[string]*RewriteRule{
			"r1": {
				ID:         "r1",
				URLPattern: "*",
				Method:     "POST",
				Phase:      "request",
				Target:     "body",
				Match:      "secret",
				Replace:    "***",
				Enabled:    true,
			},
		},
	}

	body := []byte("secret")
	headers := http.Header{}

	// GET should not match
	newBody, _ := p.applyRewriteRules("GET", "https://example.com", "request", headers, body)
	if string(newBody) != "secret" {
		t.Error("POST-only rule should not apply to GET")
	}

	// POST should match
	newBody, _ = p.applyRewriteRules("POST", "https://example.com", "request", headers, body)
	if string(newBody) != "***" {
		t.Errorf("POST rule should apply to POST, got %q", string(newBody))
	}
}

func TestApplyRewriteRules_URLFiltering(t *testing.T) {
	p := &ProxyServer{
		rewriteRules: map[string]*RewriteRule{
			"r1": {
				ID:         "r1",
				URLPattern: "*/api/*",
				Phase:      "response",
				Target:     "body",
				Match:      "value",
				Replace:    "replaced",
				Enabled:    true,
			},
		},
	}

	body := []byte("value")
	headers := http.Header{}

	// Matching URL
	newBody, _ := p.applyRewriteRules("GET", "https://example.com/api/data", "response", headers, body)
	if string(newBody) != "replaced" {
		t.Error("Should apply for matching URL")
	}

	// Non-matching URL
	newBody, _ = p.applyRewriteRules("GET", "https://example.com/web/page", "response", headers, body)
	if string(newBody) != "value" {
		t.Error("Should not apply for non-matching URL")
	}
}

func TestApplyRewriteRules_RegexCapture(t *testing.T) {
	p := &ProxyServer{
		rewriteRules: map[string]*RewriteRule{
			"r1": {
				ID:         "r1",
				URLPattern: "*",
				Phase:      "response",
				Target:     "body",
				Match:      `"role":"(\w+)"`,
				Replace:    `"role":"admin"`,
				Enabled:    true,
			},
		},
	}

	body := []byte(`{"role":"guest","name":"test"}`)
	headers := http.Header{}
	newBody, _ := p.applyRewriteRules("GET", "https://example.com", "response", headers, body)
	expected := `{"role":"admin","name":"test"}`
	if string(newBody) != expected {
		t.Errorf("Expected %q, got %q", expected, string(newBody))
	}
}

// ==================== getRegexp cache ====================

func TestGetRegexp_Caching(t *testing.T) {
	p := &ProxyServer{}

	r1, err1 := p.getRegexp("test.*pattern")
	if err1 != nil {
		t.Fatalf("Failed to compile regex: %v", err1)
	}

	r2, err2 := p.getRegexp("test.*pattern")
	if err2 != nil {
		t.Fatalf("Failed to get cached regex: %v", err2)
	}

	if r1 != r2 {
		t.Error("Should return the same cached regexp instance")
	}
}

func TestGetRegexp_InvalidPattern(t *testing.T) {
	p := &ProxyServer{}

	_, err := p.getRegexp("[invalid")
	if err == nil {
		t.Error("Should return error for invalid regex")
	}
}

// ==================== helper: getRegexp needs to exist ====================
// Check if getRegexp is accessible (it's on ProxyServer)

func compressGzip(data []byte) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write(data)
	gw.Close()
	return buf.Bytes()
}

func compressBrotli(data []byte) []byte {
	var buf bytes.Buffer
	bw := brotli.NewWriter(&buf)
	bw.Write(data)
	bw.Close()
	return buf.Bytes()
}

func compressDeflate(data []byte) []byte {
	var buf bytes.Buffer
	fw, _ := flate.NewWriter(&buf, flate.DefaultCompression)
	fw.Write(data)
	fw.Close()
	return buf.Bytes()
}

// Verify all three compression formats produce different compressed data
func TestCompressionFormats_Distinct(t *testing.T) {
	original := []byte("The quick brown fox jumps over the lazy dog. This is test data for compression.")

	gz := compressGzip(original)
	br := compressBrotli(original)
	df := compressDeflate(original)

	if bytes.Equal(gz, br) || bytes.Equal(gz, df) || bytes.Equal(br, df) {
		t.Error("Different compression formats should produce different outputs")
	}

	p := &ProxyServer{}

	// All three should decompress to the same original
	r1 := p.analyzeBodyFull(gz, "gzip", "text/plain")
	r2 := p.analyzeBodyFull(br, "br", "text/plain")
	r3 := p.analyzeBodyFull(df, "deflate", "text/plain")

	if r1.Text != string(original) {
		t.Errorf("Gzip decompression failed: %q", r1.Text)
	}
	if r2.Text != string(original) {
		t.Errorf("Brotli decompression failed: %q", r2.Text)
	}
	if r3.Text != string(original) {
		t.Errorf("Deflate decompression failed: %q", r3.Text)
	}
}

// Verify corrupted data doesn't panic
func TestAnalyzeBodyFull_CorruptedCompression(t *testing.T) {
	p := &ProxyServer{}

	// Feed invalid data as gzip - should not panic, may return raw or partial
	result := p.analyzeBodyFull([]byte("not gzip data"), "gzip", "text/plain")
	_ = result // just ensure no panic

	result = p.analyzeBodyFull([]byte("not brotli data"), "br", "text/plain")
	_ = result

	result = p.analyzeBodyFull([]byte("not deflate data"), "deflate", "text/plain")
	_ = result
}

// ==================== MapRemoteRule on ProxyServer ====================

func TestProxyServer_MapRemoteAddAndMatch(t *testing.T) {
	p := &ProxyServer{}

	// Add rule
	p.AddMapRemoteRule("r1", "*/api/*", "http://localhost/*", "GET")

	// Should match GET
	result := p.matchMapRemoteRule("GET", "https://prod.com/api/users")
	if result == "" {
		t.Error("Should match GET /api/users")
	}

	// Should not match POST (method filter)
	result = p.matchMapRemoteRule("POST", "https://prod.com/api/users")
	if result != "" {
		t.Error("Should not match POST when rule is GET-only")
	}

	// Should not match non-api URL
	result = p.matchMapRemoteRule("GET", "https://prod.com/web/page")
	if result != "" {
		t.Error("Should not match non-api URL")
	}

	// Remove and verify
	p.RemoveMapRemoteRule("r1")
	result = p.matchMapRemoteRule("GET", "https://prod.com/api/users")
	if result != "" {
		t.Error("Should not match after rule removed")
	}
}

func TestProxyServer_MapRemoteNoMethodFilter(t *testing.T) {
	p := &ProxyServer{}
	p.AddMapRemoteRule("r1", "*/api/*", "http://localhost/*", "")

	// Should match any method
	if p.matchMapRemoteRule("GET", "https://prod.com/api/data") == "" {
		t.Error("Should match GET with empty method filter")
	}
	if p.matchMapRemoteRule("POST", "https://prod.com/api/data") == "" {
		t.Error("Should match POST with empty method filter")
	}
}

// ==================== RewriteRule CRUD on ProxyServer ====================

func TestProxyServer_RewriteRuleCRUD(t *testing.T) {
	p := &ProxyServer{}

	// Add
	p.AddRewriteRule("rw1", "*/api/*", "", "response", "body", "", "old", "new")

	// Verify it works
	body := []byte("old value")
	newBody, _ := p.applyRewriteRules("GET", "https://example.com/api/data", "response", http.Header{}, body)
	if string(newBody) != "new value" {
		t.Errorf("Rewrite should work after add, got %q", string(newBody))
	}

	// Remove
	p.RemoveRewriteRule("rw1")
	newBody, _ = p.applyRewriteRules("GET", "https://example.com/api/data", "response", http.Header{}, body)
	if string(newBody) != "old value" {
		t.Error("Rewrite should not apply after remove")
	}
}

// ==================== hasRewriteRules ====================

func TestHasRewriteRules(t *testing.T) {
	p := &ProxyServer{
		rewriteRules: map[string]*RewriteRule{
			"r1": {ID: "r1", Phase: "request", Enabled: true},
			"r2": {ID: "r2", Phase: "response", Enabled: false},
		},
	}

	if !p.hasRewriteRules("request") {
		t.Error("Should have enabled request rules")
	}
	if p.hasRewriteRules("response") {
		t.Error("Should not have enabled response rules (r2 is disabled)")
	}
}

// ==================== Debug logging switch ====================

func TestDebugLog_DisabledByDefault(t *testing.T) {
	p := &ProxyServer{}
	p.debugLog("should be dropped %d", 1)

	p.debugLogMu.Lock()
	defer p.debugLogMu.Unlock()
	if p.debugLogFile != nil {
		t.Error("debugLog should not open a file while debug logging is disabled")
	}
}

func TestSetDebugLogging_EnableDisable(t *testing.T) {
	tmp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	p := &ProxyServer{}
	p.SetDebugLogging(true)
	p.debugLog("hello %s", "world")

	logPath := filepath.Join(tmp, ".log", "proxy_debug.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected debug log file after enabling: %v", err)
	}
	if !strings.Contains(string(data), "hello world") {
		t.Errorf("log content = %q, want to contain %q", data, "hello world")
	}

	// Disabling closes the handle and short-circuits further writes
	p.SetDebugLogging(false)
	p.debugLogMu.Lock()
	if p.debugLogFile != nil {
		t.Error("disabling debug logging should close the log handle")
	}
	p.debugLogMu.Unlock()

	p.debugLog("dropped line")
	data, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "dropped line") {
		t.Error("debugLog should not write after disabling")
	}
}

// ==================== Breakpoint capacity observability ====================

func TestBreakpointCapacityNotify(t *testing.T) {
	p := &ProxyServer{}

	type capacityHit struct {
		ruleID, url, phase string
	}
	hits := make(chan capacityHit, 1)
	p.SetBreakpointCapacityCallback(func(ruleID, url, phase string) {
		hits <- capacityHit{ruleID, url, phase}
	})

	p.notifyBreakpointCapacity("rule-1", "https://example.com/api", "request")

	select {
	case hit := <-hits:
		if hit.ruleID != "rule-1" || hit.url != "https://example.com/api" || hit.phase != "request" {
			t.Errorf("unexpected callback args: %+v", hit)
		}
	case <-time.After(time.Second):
		t.Fatal("capacity callback not invoked")
	}

	skips, timeouts := p.BreakpointStats()
	if skips != 1 {
		t.Errorf("capacitySkips = %d, want 1", skips)
	}
	if timeouts != 0 {
		t.Errorf("timeoutForwards = %d, want 0", timeouts)
	}
}

func TestBreakpointCapacityNotify_NoCallback(t *testing.T) {
	p := &ProxyServer{}
	// Must not panic and must still count without a registered callback
	p.notifyBreakpointCapacity("rule-1", "https://example.com/api", "response")
	p.notifyBreakpointCapacity("rule-1", "https://example.com/api", "response")

	skips, _ := p.BreakpointStats()
	if skips != 2 {
		t.Errorf("capacitySkips = %d, want 2", skips)
	}
}

// Helper to suppress unused import warnings
var _ = io.Discard
