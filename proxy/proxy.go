package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/elazarl/goproxy"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/time/rate"
)

func copyHeader(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

// MockCondition defines an additional match condition for mock rules.
// All conditions must match (AND logic) for a rule to apply.
type MockCondition struct {
	Type     string `json:"type"`     // "header", "query", "body"
	Key      string `json:"key"`      // header name or query param name (unused for body type)
	Operator string `json:"operator"` // "equals", "contains", "regex", "exists", "not_exists"
	Value    string `json:"value"`    // expected value (unused for exists/not_exists)
}

// MockRule defines a rule for mocking HTTP responses
type MockRule struct {
	ID         string
	URLPattern string
	Method     string
	StatusCode int
	Headers    map[string]string
	Body       string
	BodyFile   string // Path to local file for response body (overrides Body if set)
	Delay      int
	Conditions []MockCondition // Additional match conditions (AND logic)
}

// MapRemoteRule defines a URL rewriting rule
type MapRemoteRule struct {
	ID            string
	SourcePattern string // URL wildcard pattern to match (e.g., "*api.prod.com/*")
	TargetURL     string // Target URL prefix (e.g., "https://api.staging.com/")
	Method        string // HTTP method to match (empty = all)
	Enabled       bool
}

// RewriteRule defines an auto-rewrite rule for request/response modification
type RewriteRule struct {
	ID         string
	URLPattern string // URL wildcard pattern to match
	Method     string // HTTP method to match (empty = all)
	Phase      string // "request", "response", or "both"
	Target     string // "header" or "body"
	HeaderName string // Header name when Target is "header" (empty for body)
	Match      string // Regex pattern to find
	Replace    string // Replacement string (supports $1, $2 etc.)
	Enabled    bool
}

// ProxyServer handles the HTTP/HTTPS logic using goproxy
type ProxyServer struct {
	server             *http.Server
	proxy              *goproxy.ProxyHttpServer
	listener           net.Listener
	mu                 sync.Mutex
	running            bool
	port               int
	OnRequest          func(RequestLog) // Callback for request logging
	OnWSMessage        func(WSMessage)  // Callback for WebSocket messages
	mitmEnabled        bool             // HTTPS Decrypt
	wsEnabled          bool             // WebSocket support
	mitmBypassPatterns []string
	certMgr            *CertManager

	upLimiter   *rate.Limiter
	downLimiter *rate.Limiter
	latency     time.Duration // Artificial latency

	mockRules      map[string]*MockRule      // Mock response rules
	mapRemoteRules map[string]*MapRemoteRule // URL rewriting rules
	rewriteRules   map[string]*RewriteRule   // Auto rewrite rules

	hasDecryptedHTTPS bool // Track if we've seen decrypted HTTPS traffic

	// Breakpoint interception
	bp breakpointState

	// reqBodyCache stores captured request bodies keyed by request ID.
	// Written by the request TransparentReadCloser, read by the response one.
	reqBodyCache   map[string]cachedReqBody
	reqBodyCacheMu sync.Mutex

	// regexCache caches compiled regular expressions to avoid recompilation per request.
	regexCache   map[string]*regexp.Regexp
	regexCacheMu sync.RWMutex

	// debugLogFile is a lazily-opened persistent handle for debugLog output.
	// debugEnabled gates all debugLog calls (default off) so the hot path
	// skips the per-request Sprintf + disk writes entirely.
	debugEnabled atomic.Bool
	debugLogMu   sync.Mutex
	debugLogFile *os.File
}

// cachedReqBody holds both the display text and raw bytes for a request body.
type cachedReqBody struct {
	text      string
	rawBytes  []byte    // non-nil when binary data detected
	createdAt time.Time // for TTL cleanup
}

// getRegexp returns a cached compiled regexp, or compiles and caches it.
func (p *ProxyServer) getRegexp(pattern string) (*regexp.Regexp, error) {
	p.regexCacheMu.RLock()
	if re, ok := p.regexCache[pattern]; ok {
		p.regexCacheMu.RUnlock()
		return re, nil
	}
	p.regexCacheMu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	p.regexCacheMu.Lock()
	if p.regexCache == nil {
		p.regexCache = make(map[string]*regexp.Regexp)
	}
	p.regexCache[pattern] = re
	p.regexCacheMu.Unlock()
	return re, nil
}

// GetPort returns the port the proxy is running on
func (p *ProxyServer) GetPort() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.port
}

// SetLatency sets the artificial latency in milliseconds
func (p *ProxyServer) SetLatency(latencyMs int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.latency = time.Duration(latencyMs) * time.Millisecond
}

// AddMockRule adds a mock response rule
func (p *ProxyServer) AddMockRule(id, urlPattern, method string, statusCode int, headers map[string]string, body, bodyFile string, delay int, conditions []MockCondition) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mockRules == nil {
		p.mockRules = make(map[string]*MockRule)
	}
	p.mockRules[id] = &MockRule{
		ID:         id,
		URLPattern: urlPattern,
		Method:     method,
		StatusCode: statusCode,
		Headers:    headers,
		Body:       body,
		BodyFile:   bodyFile,
		Delay:      delay,
		Conditions: conditions,
	}
}

// RemoveMockRule removes a mock response rule
func (p *ProxyServer) RemoveMockRule(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mockRules != nil {
		delete(p.mockRules, id)
	}
}

// AddMapRemoteRule adds or updates a URL rewriting rule
func (p *ProxyServer) AddMapRemoteRule(id, sourcePattern, targetURL, method string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mapRemoteRules == nil {
		p.mapRemoteRules = make(map[string]*MapRemoteRule)
	}
	p.mapRemoteRules[id] = &MapRemoteRule{
		ID:            id,
		SourcePattern: sourcePattern,
		TargetURL:     targetURL,
		Method:        method,
		Enabled:       true,
	}
}

// RemoveMapRemoteRule removes a URL rewriting rule
func (p *ProxyServer) RemoveMapRemoteRule(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mapRemoteRules != nil {
		delete(p.mapRemoteRules, id)
	}
}

// matchMapRemoteRule checks if a request matches any map remote rule.
// Returns the rewritten URL or empty string if no match.
func (p *ProxyServer) matchMapRemoteRule(method, requestURL string) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, rule := range p.mapRemoteRules {
		if !rule.Enabled {
			continue
		}
		if rule.Method != "" && !strings.EqualFold(rule.Method, method) {
			continue
		}
		if MatchPattern(requestURL, rule.SourcePattern) {
			// Simple replacement: replace matched prefix with target
			// If source has trailing *, capture what's after the fixed part and append to target
			return applyMapRemote(rule.SourcePattern, rule.TargetURL, requestURL)
		}
	}
	return ""
}

// applyMapRemote applies URL rewriting based on source pattern and target URL.
// Supports wildcard (*) substitution: the part matched by the last * in source
// is appended to the target URL.
func applyMapRemote(sourcePattern, targetURL, requestURL string) string {
	// If no wildcard in source, just return target as-is
	if !strings.Contains(sourcePattern, "*") {
		return targetURL
	}

	// Find the fixed prefix before the last * in source pattern
	lastStar := strings.LastIndex(sourcePattern, "*")
	prefix := sourcePattern[:lastStar]
	// Remove leading * characters from prefix for matching
	prefix = strings.TrimLeft(prefix, "*")

	// Find where the prefix appears in the request URL
	idx := strings.Index(requestURL, prefix)
	if idx < 0 {
		return targetURL
	}

	// The "tail" is everything after the prefix in the request URL
	tail := requestURL[idx+len(prefix):]

	// If target ends with *, replace it with the tail
	if strings.HasSuffix(targetURL, "*") {
		return targetURL[:len(targetURL)-1] + tail
	}

	// If target ends with /, append the tail
	if strings.HasSuffix(targetURL, "/") {
		return targetURL + tail
	}

	// Otherwise just return target
	return targetURL
}

// AddRewriteRule adds or updates an auto rewrite rule
func (p *ProxyServer) AddRewriteRule(id, urlPattern, method, phase, target, headerName, match, replace string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rewriteRules == nil {
		p.rewriteRules = make(map[string]*RewriteRule)
	}
	p.rewriteRules[id] = &RewriteRule{
		ID:         id,
		URLPattern: urlPattern,
		Method:     method,
		Phase:      phase,
		Target:     target,
		HeaderName: headerName,
		Match:      match,
		Replace:    replace,
		Enabled:    true,
	}
}

// RemoveRewriteRule removes an auto rewrite rule
func (p *ProxyServer) RemoveRewriteRule(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rewriteRules != nil {
		delete(p.rewriteRules, id)
	}
}

// applyRewriteRules applies matching rewrite rules to headers or body content.
// phase should be "request" or "response".
// Returns the (possibly modified) body and a map of header modifications.
func (p *ProxyServer) applyRewriteRules(method, requestURL, phase string, headers http.Header, body []byte) ([]byte, map[string]string) {
	p.mu.Lock()
	rules := make([]*RewriteRule, 0, len(p.rewriteRules))
	for _, rule := range p.rewriteRules {
		rules = append(rules, rule)
	}
	p.mu.Unlock()

	modifiedBody := body
	headerMods := map[string]string{}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.Phase != "both" && rule.Phase != phase {
			continue
		}
		if rule.Method != "" && !strings.EqualFold(rule.Method, method) {
			continue
		}
		if !MatchPattern(requestURL, rule.URLPattern) {
			continue
		}

		re, err := p.getRegexp(rule.Match)
		if err != nil {
			continue // skip invalid regex
		}

		if rule.Target == "header" {
			// Rewrite specific header
			if rule.HeaderName != "" {
				val := headers.Get(rule.HeaderName)
				if val != "" {
					newVal := re.ReplaceAllString(val, rule.Replace)
					if newVal != val {
						headerMods[rule.HeaderName] = newVal
					}
				}
			}
		} else {
			// Rewrite body
			if len(modifiedBody) > 0 {
				newBody := re.ReplaceAll(modifiedBody, []byte(rule.Replace))
				modifiedBody = newBody
			}
		}
	}

	return modifiedBody, headerMods
}

// hasRewriteRules checks if there are any enabled rewrite rules for the given phase.
func (p *ProxyServer) hasRewriteRules(phase string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, rule := range p.rewriteRules {
		if rule.Enabled && (rule.Phase == "both" || rule.Phase == phase) {
			return true
		}
	}
	return false
}

// matchMockRule checks if a request matches any mock rule and returns it.
// It accepts the full http.Request to evaluate header, query, and body conditions.
// bodyBytes is the pre-read request body (may be nil if no rules need body matching).
func (p *ProxyServer) matchMockRule(r *http.Request, bodyBytes []byte) *MockRule {
	p.mu.Lock()
	defer p.mu.Unlock()

	method := r.Method
	url := r.URL.String()

	for _, rule := range p.mockRules {
		// Check method (empty means match all)
		if rule.Method != "" && rule.Method != method {
			continue
		}

		// Check URL pattern (supports * wildcard)
		if !MatchPattern(url, rule.URLPattern) {
			continue
		}

		// Check additional conditions (AND logic)
		if len(rule.Conditions) > 0 && !evaluateConditions(rule.Conditions, r, bodyBytes) {
			continue
		}

		return rule
	}
	return nil
}

// hasBodyConditions checks if any registered mock rule has body-type conditions.
func (p *ProxyServer) hasBodyConditions() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, rule := range p.mockRules {
		for _, c := range rule.Conditions {
			if c.Type == "body" {
				return true
			}
		}
	}
	return false
}

// evaluateConditions checks all conditions against the request (AND logic).
func evaluateConditions(conditions []MockCondition, r *http.Request, bodyBytes []byte) bool {
	for _, cond := range conditions {
		if !evaluateCondition(cond, r, bodyBytes) {
			return false
		}
	}
	return true
}

// evaluateCondition checks a single condition against the request.
func evaluateCondition(cond MockCondition, r *http.Request, bodyBytes []byte) bool {
	switch cond.Type {
	case "header":
		return evaluateHeaderCondition(cond, r.Header)
	case "query":
		return evaluateQueryCondition(cond, r.URL.Query())
	case "body":
		return evaluateBodyCondition(cond, bodyBytes)
	default:
		return true // unknown condition type → pass
	}
}

func evaluateHeaderCondition(cond MockCondition, headers http.Header) bool {
	value := headers.Get(cond.Key)
	exists := value != "" || headers[http.CanonicalHeaderKey(cond.Key)] != nil
	return matchOperator(cond.Operator, value, cond.Value, exists)
}

func evaluateQueryCondition(cond MockCondition, query map[string][]string) bool {
	values, exists := query[cond.Key]
	value := ""
	if len(values) > 0 {
		value = values[0]
	}
	return matchOperator(cond.Operator, value, cond.Value, exists)
}

func evaluateBodyCondition(cond MockCondition, bodyBytes []byte) bool {
	body := string(bodyBytes)
	return matchOperator(cond.Operator, body, cond.Value, len(bodyBytes) > 0)
}

// package-level regex cache for use in standalone functions
var (
	pkgRegexCache   = make(map[string]*regexp.Regexp)
	pkgRegexCacheMu sync.RWMutex
)

func getCachedRegexp(pattern string) (*regexp.Regexp, error) {
	pkgRegexCacheMu.RLock()
	if re, ok := pkgRegexCache[pattern]; ok {
		pkgRegexCacheMu.RUnlock()
		return re, nil
	}
	pkgRegexCacheMu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	pkgRegexCacheMu.Lock()
	pkgRegexCache[pattern] = re
	pkgRegexCacheMu.Unlock()
	return re, nil
}

// matchOperator applies an operator to compare actual vs expected values.
func matchOperator(operator, actual, expected string, exists bool) bool {
	switch operator {
	case "equals":
		return actual == expected
	case "contains":
		return strings.Contains(actual, expected)
	case "regex":
		re, err := getCachedRegexp(expected)
		if err != nil {
			return false
		}
		return re.MatchString(actual)
	case "exists":
		return exists
	case "not_exists":
		return !exists
	default:
		return actual == expected // default to equals
	}
}

// MatchPattern checks if a URL matches a pattern with * wildcards
func MatchPattern(url, pattern string) bool {
	// Simple wildcard matching
	if pattern == "*" {
		return true
	}

	// Split pattern by *
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		// No wildcard, exact match
		return url == pattern
	}

	// Check if URL matches pattern with wildcards
	pos := 0
	firstPartProcessed := false
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(url[pos:], part)
		if idx == -1 {
			return false
		}
		// First non-empty part must match at start ONLY if pattern doesn't start with *
		if !firstPartProcessed && i == 0 && idx != 0 {
			// Pattern doesn't start with *, so first part must be at the beginning
			return false
		}
		firstPartProcessed = true
		pos += idx + len(part)
	}

	// If pattern doesn't end with *, URL must end exactly
	if !strings.HasSuffix(pattern, "*") && pos != len(url) {
		return false
	}

	return true
}

func (p *ProxyServer) simulateLatency() {
	p.mu.Lock()
	latency := p.latency
	p.mu.Unlock()

	if latency > 0 {
		time.Sleep(latency)
	}
}

// SetDebugLogging toggles proxy debug logging to .log/proxy_debug.log.
// Disabled by default; disabling also closes the current log handle.
func (p *ProxyServer) SetDebugLogging(enabled bool) {
	p.debugEnabled.Store(enabled)
	if !enabled {
		p.debugLogMu.Lock()
		if p.debugLogFile != nil {
			p.debugLogFile.Close()
			p.debugLogFile = nil
		}
		p.debugLogMu.Unlock()
	}
}

func (p *ProxyServer) debugLog(format string, args ...interface{}) {
	if !p.debugEnabled.Load() {
		return
	}
	// Keep a persistent handle (lazily opened) instead of open/close per line:
	// appending to an open file is microseconds, far cheaper than the previous
	// per-call goroutine + MkdirAll + open/close, and lines stay in order.
	msg := fmt.Sprintf("[%s] ", time.Now().Format("15:04:05.000")) + fmt.Sprintf(format, args...) + "\n"
	p.debugLogMu.Lock()
	defer p.debugLogMu.Unlock()
	// Re-check under the lock: a concurrent SetDebugLogging(false) may have
	// closed the handle between the fast-path check and lock acquisition;
	// without this an in-flight call would lazily reopen it while disabled
	if !p.debugEnabled.Load() {
		return
	}
	if p.debugLogFile == nil {
		if err := os.MkdirAll(".log", 0755); err != nil {
			return
		}
		f, err := os.OpenFile(filepath.Join(".log", "proxy_debug.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return
		}
		p.debugLogFile = f
	}
	p.debugLogFile.WriteString(msg)
}

// RequestLog contains details about a proxied request
type RequestLog struct {
	Id            string              `json:"id"`
	Time          string              `json:"time"`
	ClientIP      string              `json:"clientIp"`
	Method        string              `json:"method"`
	URL           string              `json:"url"`
	IsHTTPS       bool                `json:"isHttps"`
	Headers       map[string][]string `json:"headers"`     // Request Headers
	Body          string              `json:"previewBody"` // Request Body
	RespHeaders   map[string][]string `json:"respHeaders"` // Response Headers
	RespBody      string              `json:"respBody"`    // Response Body
	RespBodyRaw   []byte              `json:"-"`           // Raw response body bytes (for binary/protobuf decoding, not serialized)
	ReqBodyRaw    []byte              `json:"-"`           // Raw request body bytes (for binary/protobuf decoding, not serialized)
	StatusCode    int                 `json:"statusCode"`
	ContentType   string              `json:"contentType"`
	BodySize      int64               `json:"bodySize"`
	IsWs          bool                `json:"isWs"`
	PartialUpdate bool                `json:"partialUpdate"` // If true, only update specific fields in UI
	Mocked        bool                `json:"mocked"`        // If true, response was from mock rule
}

var currentProxy *ProxyServer
var proxyLock sync.Mutex

func GetProxy() *ProxyServer {
	proxyLock.Lock()
	defer proxyLock.Unlock()
	if currentProxy == nil {
		currentProxy = &ProxyServer{
			wsEnabled:   true, // Default ON
			mitmEnabled: true, // Default ON to match UI
			mitmBypassPatterns: []string{
				"cdn", "static", "img", "image", "video", "asset",
				"akamai", "byte", "tos-", "mon.", "snssdk",
			},
			reqBodyCache: make(map[string]cachedReqBody),
		}
	}
	return currentProxy
}

func (p *ProxyServer) Start(port int, onRequest func(RequestLog)) error {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return fmt.Errorf("proxy already running")
	}
	p.OnRequest = onRequest
	p.port = port
	p.hasDecryptedHTTPS = false // Reset on start

	// Initialize CertManager
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".adbGUI")
	_ = os.MkdirAll(dataDir, 0755)
	p.certMgr = NewCertManager(dataDir)
	if err := p.certMgr.EnsureCert(); err != nil {
		log.Printf("[Proxy] Warning: Failed to ensure CA cert: %v", err)
	}
	if err := p.certMgr.LoadToGoproxy(); err != nil {
		return fmt.Errorf("failed to load CA cert: %v", err)
	}

	// Initialize goproxy
	p.proxy = goproxy.NewProxyHttpServer()
	p.proxy.Verbose = false

	// CRITICAL: Disable automatic decompression.
	// Go's DefaultTransport automatically decompresses gzip, which breaks
	// binary transparency for Apps that expect original compressed bytes.
	// CRITICAL: Disable automatic decompression.
	// This ensures the proxy is a transparent pipe. We handle decompression
	// for UI display separately in the background to avoid interfering with
	// the actual data stream.
	p.proxy.Tr = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true, // CRITICAL: Stop Go from decompressing and changing headers
		ForceAttemptHTTP2:     true, // Support H2 to match modern app expectations
	}

	// Configure Connect Logic (MITM vs Tunnel)
	p.proxy.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		p.debugLog("CONNECT: %s (Session: %d)", host, ctx.Session)
		p.simulateLatency() // Simulate network RTT for connection

		p.mu.Lock()
		mitm := p.mitmEnabled
		p.mu.Unlock()

		if mitm {
			// 🚀 DYNAMIC HEURISTIC:
			// Use user-configurable patterns to bypass MITM for sensitive domains (CDNs, etc.)
			hostLower := strings.ToLower(host)

			p.mu.Lock()
			patterns := p.mitmBypassPatterns
			p.mu.Unlock()

			shouldBypass := false
			for _, pat := range patterns {
				if strings.Contains(hostLower, pat) {
					shouldBypass = true
					break
				}
			}

			if shouldBypass {
				p.debugLog("  -> PASS-THROUGH (Bypassed by pattern %s)", host)
				return goproxy.OkConnect, host
			}

			p.debugLog("  -> MITM (Decrypting %s)", host)
			return goproxy.MitmConnect, host
		}

		// Fallback for non-MITM mode: Apply rate limits via Hijack
		p.debugLog("  -> HIJACK (Tunneling %s with rate limit)", host)
		return &goproxy.ConnectAction{
			Action: goproxy.ConnectHijack,
			Hijack: p.handleHijackConnect,
		}, host
	})

	// Configure Request Handling (WS Check, Request Rate Limit)
	p.proxy.OnRequest().DoFunc(func(r *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		id := fmt.Sprintf("%d-%d", ctx.Session, time.Now().UnixNano())
		ctx.UserData = id // Sync ID to response

		p.debugLog("REQ: %s %s (Session: %d, ID: %s)", r.Method, r.URL.String(), ctx.Session, id)
		p.simulateLatency()

		p.mu.Lock()
		wsAllowed := p.wsEnabled
		upLimiter := p.upLimiter
		p.mu.Unlock()

		// Log request immediately (notify=false: StatusCode=0, always filtered by bridge)
		p.logRequest(id, r, nil, false)

		// 1. Transparent Request Body Copy
		// Use a Tee-like wrapper to capture the request body as it's sent to the server.
		if r.Body != nil && r.Body != http.NoBody {
			r.Body = &TransparentReadCloser{
				rc:          r.Body,
				p:           p,
				id:          id,
				isReq:       true,
				captured:    new(bytes.Buffer),
				limit:       100 * 1024 * 1024, // Full capture (100MB per request)
				encoding:    r.Header.Get("Content-Encoding"),
				contentType: r.Header.Get("Content-Type"),
			}
		}

		// Check WS Block
		if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" && !wsAllowed {
			p.debugLog("  -> WS BLOCKED")
			return r, goproxy.NewResponse(r, goproxy.ContentTypeText, http.StatusForbidden, "WebSocket disabled")
		}

		// Apply Upload Limit to Request Body
		if upLimiter != nil && r.Body != nil {
			p.debugLog("  -> APPLY UPLOAD LIMIT")
			r.Body = &RateLimitedReadCloser{
				rc:      r.Body,
				limiter: upLimiter,
			}
		}

		// Check for Map Remote (URL rewriting) rules
		if newURL := p.matchMapRemoteRule(r.Method, r.URL.String()); newURL != "" {
			p.debugLog("  -> MAP REMOTE: %s -> %s", r.URL.String(), newURL)
			parsedURL, err := url.Parse(newURL)
			if err == nil {
				r.URL = parsedURL
				r.Host = parsedURL.Host
			}
		}

		// Apply auto rewrite rules (request phase)
		if p.hasRewriteRules("request") {
			var reqBody []byte
			if r.Body != nil && r.Body != http.NoBody {
				reqBody, _ = io.ReadAll(r.Body)
			}
			newBody, headerMods := p.applyRewriteRules(r.Method, r.URL.String(), "request", r.Header, reqBody)
			for k, v := range headerMods {
				r.Header.Set(k, v)
			}
			if len(reqBody) > 0 {
				r.Body = io.NopCloser(bytes.NewReader(newBody))
				r.ContentLength = int64(len(newBody))
			}
		}

		// Read request body for condition matching (only if needed)
		var mockBodyBytes []byte
		if p.hasBodyConditions() && r.Body != nil && r.Body != http.NoBody {
			mockBodyBytes, _ = io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(mockBodyBytes)) // reset for upstream
		}

		// Check for mock rules
		if mockRule := p.matchMockRule(r, mockBodyBytes); mockRule != nil {
			p.debugLog("  -> MOCK RESPONSE (Rule: %s)", mockRule.ID)

			// Mark as mocked in UserData
			ctx.UserData = id + "|mocked"

			// Apply mock delay
			if mockRule.Delay > 0 {
				time.Sleep(time.Duration(mockRule.Delay) * time.Millisecond)
			}

			// Determine response body: from file or inline
			var mockBody string
			if mockRule.BodyFile != "" {
				fileData, err := os.ReadFile(mockRule.BodyFile)
				if err != nil {
					p.debugLog("Failed to read body file %s: %v, falling back to inline body", mockRule.BodyFile, err)
					mockBody = mockRule.Body
				} else {
					mockBody = string(fileData)
				}
			} else {
				mockBody = mockRule.Body
			}

			// Create mock response
			mockResp := &http.Response{
				StatusCode: mockRule.StatusCode,
				Status:     fmt.Sprintf("%d %s", mockRule.StatusCode, http.StatusText(mockRule.StatusCode)),
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(mockBody)),
				Request:    r,
			}

			// Set mock headers
			for k, v := range mockRule.Headers {
				mockResp.Header.Set(k, v)
			}

			// Set Content-Length if not set
			if mockResp.Header.Get("Content-Length") == "" {
				mockResp.Header.Set("Content-Length", fmt.Sprintf("%d", len(mockBody)))
			}

			return r, mockResp
		}

		// Check for breakpoint rules (request phase)
		if p.hasBreakpointRules() {
			if bpRule := p.matchBreakpointRule(r, "request"); bpRule != nil {
				if p.pendingBreakpointCount() >= maxPendingBreakpoints {
					// Pool full: pass through, but record it so the UI can surface
					// the skip instead of silently ignoring the rule
					p.notifyBreakpointCapacity(bpRule.ID, r.URL.String(), "request")
					p.debugLog("  -> BREAKPOINT SKIPPED (pool full, rule: %s)", bpRule.ID)
				} else {
					// Ensure we have the body for display
					var bpReqBody []byte
					if mockBodyBytes != nil {
						bpReqBody = mockBodyBytes
					} else if r.Body != nil && r.Body != http.NoBody {
						bpReqBody, _ = io.ReadAll(r.Body)
						r.Body = io.NopCloser(bytes.NewReader(bpReqBody))
					}

					bpID := fmt.Sprintf("bp-%d-%d", ctx.Session, time.Now().UnixNano())
					bp := &pendingBreakpoint{
						Info: PendingBreakpointInfo{
							ID:        bpID,
							RuleID:    bpRule.ID,
							Phase:     "request",
							Method:    r.Method,
							URL:       r.URL.String(),
							Headers:   copyHeader(r.Header),
							Body:      string(bpReqBody),
							CreatedAt: time.Now().UnixMilli(),
						},
						Ch: make(chan BreakpointResolution, 1),
					}

					p.addPendingBreakpoint(bp)
					p.notifyBreakpointHit(bp.Info)
					p.debugLog("  -> BREAKPOINT HIT (request phase, rule: %s)", bpRule.ID)

					// Block until user resolves or timeout
					select {
					case resolution := <-bp.Ch:
						p.removePendingBreakpoint(bpID)
						if resolution.Action == "drop" {
							p.debugLog("  -> BREAKPOINT DROPPED")
							// Mark as dropped so response handler skips breakpoint matching
							ctx.UserData = id + "|bp-dropped"
							return r, goproxy.NewResponse(r, goproxy.ContentTypeText, http.StatusBadGateway, "Dropped by breakpoint")
						}
						// Apply modifications
						if resolution.ModifiedURL != "" {
							if parsed, err := url.Parse(resolution.ModifiedURL); err == nil {
								r.URL = parsed
								r.Host = parsed.Host
							}
						}
						if resolution.ModifiedMethod != "" {
							r.Method = resolution.ModifiedMethod
						}
						if resolution.ModifiedHeaders != nil {
							for k, v := range resolution.ModifiedHeaders {
								r.Header.Set(k, v)
							}
						}
						if resolution.ModifiedBody != "" {
							r.Body = io.NopCloser(strings.NewReader(resolution.ModifiedBody))
							r.ContentLength = int64(len(resolution.ModifiedBody))
						}
						p.debugLog("  -> BREAKPOINT FORWARDED")

					case <-time.After(breakpointTimeout):
						p.removePendingBreakpoint(bpID)
						p.bp.timeoutForwards.Add(1)
						p.notifyBreakpointResolved(bpID, "timeout")
						p.debugLog("  -> BREAKPOINT TIMEOUT (auto-forward)")
					}
				}
			}
		}

		return r, nil
	})

	// Configure Response Handling (Logging, Download Rate Limit)
	p.proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		userData, _ := ctx.UserData.(string)
		id := userData
		mocked := false
		bpDropped := false
		if strings.HasSuffix(userData, "|mocked") {
			id = strings.TrimSuffix(userData, "|mocked")
			mocked = true
		}
		if strings.HasSuffix(userData, "|bp-dropped") {
			id = strings.TrimSuffix(userData, "|bp-dropped")
			bpDropped = true
		}
		if id == "" {
			id = fmt.Sprintf("%d-%d", ctx.Session, time.Now().UnixNano())
		}

		req := resp.Request
		if req == nil {
			req = ctx.Req
		}

		p.debugLog("RESP: %d for %s (Proto: %s, Type: %s, Mocked: %v)", resp.StatusCode, id, resp.Proto, resp.Header.Get("Content-Type"), mocked)

		// Track if we've successfully decrypted HTTPS traffic
		// This is used to verify certificate trust status
		if req != nil && req.URL != nil && req.URL.Scheme == "https" && resp.StatusCode > 0 {
			p.mu.Lock()
			p.hasDecryptedHTTPS = true
			p.mu.Unlock()
		}

		// Determine if TransparentReadCloser will handle the final OnRequest callback.
		// If so, logRequest must NOT notify (to avoid duplicate events entering DB).
		isWS := resp.StatusCode == 101 || (req != nil && strings.ToLower(req.Header.Get("Upgrade")) == "websocket")
		willWrapBody := resp.Body != nil && !isWS && (req == nil || req.Method != "CONNECT")

		// Update log with response headers
		// notify=!willWrapBody: only fire OnRequest if TransparentReadCloser won't do it
		log := p.logRequest(id, req, resp, !willWrapBody)
		log.Mocked = mocked
		_ = log // Ensure used if Body is nil

		p.mu.Lock()
		downLimiter := p.downLimiter
		p.mu.Unlock()

		// Check for response breakpoint (before TransparentReadCloser wrapping)
		// Skip if request was dropped by a breakpoint (bpDropped) to avoid phantom response breakpoints
		breakpointed := false
		if !mocked && !bpDropped && !isWS && req != nil && req.Method != "CONNECT" &&
			p.hasBreakpointRules() {

			if bpRule := p.matchBreakpointRule(req, "response"); bpRule != nil &&
				p.pendingBreakpointCount() >= maxPendingBreakpoints {
				// Pool full: pass through, but record it so the UI can surface
				// the skip instead of silently ignoring the rule
				p.notifyBreakpointCapacity(bpRule.ID, req.URL.String(), "response")
				p.debugLog("  -> BREAKPOINT SKIPPED (pool full, rule: %s)", bpRule.ID)
			} else if bpRule != nil {
				breakpointed = true

				// Read full response body for display
				var respBodyBytes []byte
				if resp.Body != nil {
					respBodyBytes, _ = io.ReadAll(resp.Body)
					resp.Body.Close()
				}

				// Analyze body for display
				respBodyText := ""
				if len(respBodyBytes) > 0 {
					encoding := ""
					if vv, ok := resp.Header["Content-Encoding"]; ok && len(vv) > 0 {
						encoding = vv[0]
					}
					respBodyText = p.analyzeBody(respBodyBytes, encoding, resp.Header.Get("Content-Type"))
				}

				// Get cached request body
				reqBodyText := ""
				p.reqBodyCacheMu.Lock()
				if cached, ok := p.reqBodyCache[id]; ok {
					reqBodyText = cached.text
				}
				p.reqBodyCacheMu.Unlock()

				bpID := fmt.Sprintf("bp-%d-%d", ctx.Session, time.Now().UnixNano())
				bp := &pendingBreakpoint{
					Info: PendingBreakpointInfo{
						ID:          bpID,
						RuleID:      bpRule.ID,
						Phase:       "response",
						Method:      req.Method,
						URL:         req.URL.String(),
						Headers:     copyHeader(req.Header),
						Body:        reqBodyText,
						StatusCode:  resp.StatusCode,
						RespHeaders: copyHeader(resp.Header),
						RespBody:    respBodyText,
						CreatedAt:   time.Now().UnixMilli(),
					},
					Ch: make(chan BreakpointResolution, 1),
				}

				p.addPendingBreakpoint(bp)
				p.notifyBreakpointHit(bp.Info)
				p.debugLog("  -> BREAKPOINT HIT (response phase, rule: %s)", bpRule.ID)

				// Block until user resolves or timeout
				resolvedBody := string(respBodyBytes)
				select {
				case resolution := <-bp.Ch:
					p.removePendingBreakpoint(bpID)
					if resolution.Action == "drop" {
						resp.StatusCode = 502
						resp.Status = "502 Bad Gateway"
						resolvedBody = "Dropped by breakpoint"
					} else {
						if resolution.ModifiedStatusCode > 0 {
							resp.StatusCode = resolution.ModifiedStatusCode
							resp.Status = fmt.Sprintf("%d %s", resolution.ModifiedStatusCode, http.StatusText(resolution.ModifiedStatusCode))
						}
						if resolution.ModifiedRespHeaders != nil {
							for k, v := range resolution.ModifiedRespHeaders {
								resp.Header.Set(k, v)
							}
						}
						if resolution.ModifiedRespBody != "" {
							resolvedBody = resolution.ModifiedRespBody
						}
					}
					p.debugLog("  -> BREAKPOINT RESOLVED (%s)", resolution.Action)

				case <-time.After(breakpointTimeout):
					p.removePendingBreakpoint(bpID)
					p.bp.timeoutForwards.Add(1)
					p.notifyBreakpointResolved(bpID, "timeout")
					p.debugLog("  -> BREAKPOINT TIMEOUT (auto-forward)")
				}

				// Set the resolved body
				resp.Body = io.NopCloser(strings.NewReader(resolvedBody))
				resp.ContentLength = int64(len(resolvedBody))

				// Send complete log via OnRequest (TransparentReadCloser won't run)
				go func() {
					if p.OnRequest == nil {
						return
					}
					logToSend := log
					logToSend.BodySize = int64(len(resolvedBody))
					logToSend.RespBody = resolvedBody
					logToSend.Mocked = mocked

					// Merge cached request body
					p.reqBodyCacheMu.Lock()
					if cached, ok := p.reqBodyCache[id]; ok {
						logToSend.Body = cached.text
						if len(cached.rawBytes) > 0 {
							logToSend.ReqBodyRaw = cached.rawBytes
						}
						delete(p.reqBodyCache, id)
					}
					p.reqBodyCacheMu.Unlock()

					p.OnRequest(logToSend)
				}()
			}
		}

		// Apply auto rewrite rules (response phase)
		if !breakpointed && !mocked && req != nil && p.hasRewriteRules("response") {
			if resp.Body != nil && resp.Body != http.NoBody {
				respBodyBytes, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				newBody, headerMods := p.applyRewriteRules(req.Method, req.URL.String(), "response", resp.Header, respBodyBytes)
				for k, v := range headerMods {
					resp.Header.Set(k, v)
				}
				resp.Body = io.NopCloser(bytes.NewReader(newBody))
				resp.ContentLength = int64(len(newBody))
			} else {
				// No body, just rewrite headers
				_, headerMods := p.applyRewriteRules(req.Method, req.URL.String(), "response", resp.Header, nil)
				for k, v := range headerMods {
					resp.Header.Set(k, v)
				}
			}
		}

		if !breakpointed && resp.Body != nil {
			rc := resp.Body

			// 2. Universal Transparent Mirroring
			// We wrap EVERYTHING to ensure we have size monitoring and logs.
			// But we use a 'Shadow Capture' limit to avoid messing with heavy binaries.

			contentType := resp.Header.Get("Content-Type")
			isBinary := strings.Contains(contentType, "image/") ||
				strings.Contains(contentType, "video/") ||
				strings.Contains(contentType, "audio/")

			captureLimit := 100 * 1024 * 1024 // 100MB: Practically full capture for API
			if isBinary {
				captureLimit = 0 // Mirrors the flow without storing any bytes
			}

			if !isWS && (req == nil || req.Method != "CONNECT") {
				encoding := ""
				if resp != nil {
					encoding = resp.Header.Get("Content-Encoding")
				}
				contentType := ""
				if resp != nil {
					contentType = resp.Header.Get("Content-Type")
				}
				rc = &TransparentReadCloser{
					rc:          rc,
					p:           p,
					id:          id,
					isReq:       false,
					log:         log,
					captured:    new(bytes.Buffer),
					limit:       int64(captureLimit),
					encoding:    encoding,
					contentType: contentType,
				}
			}

			if downLimiter != nil {
				rc = &RateLimitedReadCloser{
					rc:      rc,
					limiter: downLimiter,
				}
			}
			resp.Body = rc
		}

		// Intercept WebSocket frames for inspection
		if isWS && resp.Body != nil && p.OnWSMessage != nil {
			if rwc, ok := resp.Body.(io.ReadWriteCloser); ok {
				wsURL := ""
				if req != nil && req.URL != nil {
					wsURL = req.URL.String()
				}
				interceptor := NewWSInterceptor(rwc, id, wsURL, p.OnWSMessage)
				resp.Body = interceptor
				p.debugLog("  -> WS frame interceptor installed for %s (url=%s)", id, wsURL)
			}
		}

		return resp
	})

	p.mu.Unlock()

	// Bind to localhost only for security - external access via adb reverse
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	p.listener = ln // Raw listener now

	p.server = &http.Server{
		Handler: p.proxy,
	}

	p.mu.Lock()
	p.running = true
	log.Printf("[Proxy] Started on port %d", p.port)
	p.mu.Unlock()

	go func() {
		err := p.server.Serve(p.listener)
		log.Printf("[Proxy] Serve() returned: %v", err)
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	// Periodic cleanup of stale reqBodyCache entries (older than 2 minutes)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			p.mu.Lock()
			running := p.running
			p.mu.Unlock()
			if !running {
				return
			}
			p.cleanupReqBodyCache(2 * time.Minute)
		}
	}()

	return nil
}

// cleanupReqBodyCache removes entries older than maxAge.
func (p *ProxyServer) cleanupReqBodyCache(maxAge time.Duration) {
	p.reqBodyCacheMu.Lock()
	defer p.reqBodyCacheMu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	for id, entry := range p.reqBodyCache {
		if entry.createdAt.Before(cutoff) {
			delete(p.reqBodyCache, id)
		}
	}
}

// handleHijackConnect implements a custom TCP tunnel to support rate limiting without MITM
func (p *ProxyServer) handleHijackConnect(req *http.Request, clientConn net.Conn, ctx *goproxy.ProxyCtx) {
	// 1. Dial destination
	destConn, err := net.DialTimeout("tcp", req.Host, 10*time.Second)
	if err != nil {
		clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		clientConn.Close()
		return
	}

	// 2. Respond 200 OK to client to establish tunnel
	clientConn.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))

	// 3. Rate Limiters
	p.mu.Lock()
	up := p.upLimiter
	down := p.downLimiter
	p.mu.Unlock()

	// 4. Bidirectional Copy
	// Client -> Dest (Upload)
	go p.transfer(destConn, clientConn, up)
	// Dest -> Client (Download)
	go p.transfer(clientConn, destConn, down)
}

func (p *ProxyServer) transfer(dst net.Conn, src net.Conn, limiter *rate.Limiter) {
	defer dst.Close()
	defer src.Close()

	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			// Apply Limit
			if limiter != nil {
				ctx := context.Background()
				burst := limiter.Burst()
				remaining := n
				for remaining > 0 {
					take := remaining
					if take > burst {
						take = burst
					}
					if err := limiter.WaitN(ctx, take); err != nil {
						// context canceled or error
						return
					}
					remaining -= take
				}
			}
			// Write
			if _, wErr := dst.Write(buf[:n]); wErr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
}

func (p *ProxyServer) logRequest(id string, r *http.Request, resp *http.Response, notify bool) RequestLog {
	if p.OnRequest == nil || r == nil {
		return RequestLog{}
	}

	host := ""
	if r.RemoteAddr != "" {
		h, _, err := net.SplitHostPort(r.RemoteAddr)
		if err == nil {
			host = h
		} else {
			host = r.RemoteAddr
		}
	}

	isHTTPS := false
	if r.URL != nil {
		isHTTPS = r.URL.Scheme == "https"
	}

	method := r.Method
	if strings.ToLower(r.Header.Get("Upgrade")) == "websocket" {
		method = "WS"
	}

	// Clone request headers to avoid data race
	reqHeaders := copyHeader(r.Header)
	urlStr := ""
	if r.URL != nil {
		urlStr = r.URL.String()
	}

	// Extract Response details
	statusCode := 0
	var contentType string
	var bodySize int64
	var respHeaders map[string][]string

	if resp != nil {
		statusCode = resp.StatusCode
		contentType = resp.Header.Get("Content-Type")
		bodySize = resp.ContentLength
		respHeaders = copyHeader(resp.Header)
		// Don't cap bodySize at 0 if it's -1 (chunked)
	} else if method == "CONNECT" {
		statusCode = 200 // Connection Established
	}

	log := RequestLog{
		Id:          id,
		Time:        time.Now().Format("2006-01-02 15:04:05"),
		ClientIP:    host,
		Method:      method,
		URL:         urlStr,
		IsHTTPS:     isHTTPS,
		Headers:     reqHeaders,
		RespHeaders: respHeaders,
		StatusCode:  statusCode,
		ContentType: contentType,
		BodySize:    bodySize,
		IsWs:        method == "WS",
	}

	// Emit log to UI in background, only if notify is true.
	// When TransparentReadCloser will wrap the response body, it sends the
	// final (complete) callback itself, so logRequest should NOT notify to
	// avoid duplicate events.
	go func() {
		if p.OnRequest != nil && notify {
			p.OnRequest(log)
		}
	}()

	return log
}

func (p *ProxyServer) Stop() error {
	// Forward all pending breakpoints first (unblock waiting goroutines)
	p.ForwardAllBreakpoints()

	p.mu.Lock()
	if !p.running || p.server == nil {
		p.mu.Unlock()
		return nil
	}

	// Immediately close the listener to release the port
	// This prevents "address already in use" errors on quick restart
	if p.listener != nil {
		p.listener.Close()
	}

	server := p.server
	p.running = false // Set running to false immediately
	p.mu.Unlock()

	// Clear request body cache
	p.reqBodyCacheMu.Lock()
	p.reqBodyCache = make(map[string]cachedReqBody)
	p.reqBodyCacheMu.Unlock()

	// Close the debug log handle (in-flight debugLog calls reopen it lazily)
	p.debugLogMu.Lock()
	if p.debugLogFile != nil {
		p.debugLogFile.Close()
		p.debugLogFile = nil
	}
	p.debugLogMu.Unlock()

	// Gracefully shutdown the server (wait for existing connections)
	// This call is safe outside the lock and won't block port reuse
	return server.Shutdown(context.Background())
}

func (p *ProxyServer) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *ProxyServer) SetLimits(uploadSpeed, downloadSpeed int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Low burst to ensure even small requests feel the throttle
	// But enough to handle basic TCP headers/segments efficiently
	// 4KB burst means we enforce check every 4KB
	const burstSize = 4 * 1024

	if uploadSpeed > 0 {
		p.upLimiter = rate.NewLimiter(rate.Limit(uploadSpeed), burstSize)
	} else {
		p.upLimiter = nil
	}

	if downloadSpeed > 0 {
		p.downLimiter = rate.NewLimiter(rate.Limit(downloadSpeed), burstSize)
	} else {
		p.downLimiter = nil
	}
}

func (p *ProxyServer) SetWSEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.wsEnabled = enabled
}

func (p *ProxyServer) IsWSEnabled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.wsEnabled
}

func (p *ProxyServer) SetProxyMITM(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mitmEnabled = enabled
	p.debugLog("PROXY MITM: %v", enabled)
}

func (p *ProxyServer) SetMITMBypassPatterns(patterns []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mitmBypassPatterns = patterns
	p.debugLog("PROXY MITM Bypass Patterns Updated: %v", patterns)
}

func (p *ProxyServer) GetMITMBypassPatterns() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mitmBypassPatterns
}

func (p *ProxyServer) IsMITMEnabled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mitmEnabled
}

func (p *ProxyServer) GetCertPath() string {
	if p.certMgr != nil {
		return p.certMgr.CertPath
	}
	return ""
}

// HasRecentDecryptedHTTPS checks if there are any successfully decrypted HTTPS requests
func (p *ProxyServer) HasRecentDecryptedHTTPS() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.hasDecryptedHTTPS
}

// --- Helpers ---

// RateLimitedReadCloser wraps an io.ReadCloser with rate limiting
type RateLimitedReadCloser struct {
	rc      io.ReadCloser
	limiter *rate.Limiter
}

func (r *RateLimitedReadCloser) Read(p []byte) (n int, err error) {
	n, err = r.rc.Read(p)
	if n > 0 && r.limiter != nil {
		ctx := context.Background()
		burst := r.limiter.Burst()
		remaining := n
		for remaining > 0 {
			take := remaining
			if take > burst {
				take = burst
			}
			if wErr := r.limiter.WaitN(ctx, take); wErr != nil {
				// Should we fail? Yes, probably context canceled.
				break
			}
			remaining -= take
		}
	}
	return
}

func (r *RateLimitedReadCloser) Close() error {
	return r.rc.Close()
}

// MultiReadCloser aids in restoring the body after peeking
type MultiReadCloser struct {
	io.Reader
	Closer io.Closer
}

func (m *MultiReadCloser) Close() error {
	return m.Closer.Close()
}

// TransparentReadCloser captures body and counts size during transfer without affecting the stream
type TransparentReadCloser struct {
	rc          io.ReadCloser
	p           *ProxyServer
	id          string
	isReq       bool
	log         RequestLog // Cached log metadata for response updates
	captured    *bytes.Buffer
	limit       int64
	totalSize   int64
	lastUpdate  time.Time
	doneCalled  bool // prevents double update(true) from Read EOF + Close
	mu          sync.Mutex
	encoding    string // Content-Encoding header value
	contentType string // Content-Type header value
}

func (r *TransparentReadCloser) Read(p []byte) (n int, err error) {
	n, err = r.rc.Read(p)
	if n > 0 {
		r.mu.Lock()
		r.totalSize += int64(n)
		// Capture copy for analysis (up to limit)
		if r.limit > 0 && int64(r.captured.Len()) < r.limit {
			toWrite := n
			if int64(r.captured.Len())+int64(toWrite) > r.limit {
				toWrite = int(r.limit - int64(r.captured.Len()))
			}
			r.captured.Write(p[:toWrite])
		}

		// Update UI infrequently (every 512KB) or on timer
		shouldUpdate := r.totalSize%(512*1024) < int64(n) || time.Since(r.lastUpdate) > 2*time.Second
		r.mu.Unlock()

		if shouldUpdate {
			r.update(false)
		}
	}
	if err == io.EOF {
		r.update(true)
	}
	return n, err
}

func (r *TransparentReadCloser) update(done bool) {
	r.mu.Lock()
	if done {
		if r.doneCalled {
			r.mu.Unlock()
			return
		}
		r.doneCalled = true
	}
	r.lastUpdate = time.Now()
	size := r.totalSize

	var capturedCopy []byte
	// Only capture on final update or if limit reached
	if (done || (r.limit > 0 && int64(r.captured.Len()) >= r.limit)) && r.captured.Len() > 0 {
		capturedCopy = append([]byte(nil), r.captured.Bytes()...)
	}
	r.mu.Unlock()

	// Update UI asynchronously in background
	go func() {
		if r.p.OnRequest == nil {
			return
		}

		if r.isReq {
			// Store captured request body into cache for the response handler to pick up.
			// We no longer send a PartialUpdate here; instead the response's final
			// update will include the request body from the cache.
			if len(capturedCopy) > 0 {
				result := r.p.analyzeBodyFull(capturedCopy, r.encoding, r.contentType)
				r.p.reqBodyCacheMu.Lock()
				r.p.reqBodyCache[r.id] = cachedReqBody{
					text:      result.Text,
					rawBytes:  result.RawBytes, // non-nil for binary (e.g. protobuf)
					createdAt: time.Now(),
				}
				r.p.reqBodyCacheMu.Unlock()
			}
		} else {
			// For Response:
			// If not done, only send a PartialUpdate with the current size to avoid overwriting headers
			if !done {
				r.p.OnRequest(RequestLog{
					Id:            r.id,
					BodySize:      size,
					PartialUpdate: true,
				})
				return
			}

			// Final update: Send the full log (or a meaningful subset)
			logToSend := r.log
			logToSend.BodySize = size
			if len(capturedCopy) > 0 {
				// Use encoding/contentType saved from response headers
				result := r.p.analyzeBodyFull(capturedCopy, r.encoding, r.contentType)
				logToSend.RespBody = result.Text
				if result.IsBinary {
					logToSend.RespBodyRaw = result.RawBytes
				}
			}

			// Merge cached request body (captured by the request TransparentReadCloser)
			r.p.reqBodyCacheMu.Lock()
			if cached, ok := r.p.reqBodyCache[r.id]; ok {
				logToSend.Body = cached.text
				if len(cached.rawBytes) > 0 {
					logToSend.ReqBodyRaw = cached.rawBytes
				}
				delete(r.p.reqBodyCache, r.id)
			}
			r.p.reqBodyCacheMu.Unlock()

			r.p.OnRequest(logToSend)
		}
	}()
}

func (r *TransparentReadCloser) Close() error {
	r.update(true)
	return r.rc.Close()
}

// AnalyzedBody holds the result of body analysis.
type AnalyzedBody struct {
	Text     string // String representation for display
	RawBytes []byte // Raw decompressed bytes (set when binary data detected)
	IsBinary bool   // Whether the data is binary
}

// analyzeBody handles decompression and string conversion for UI display in a non-interfering way.
func (p *ProxyServer) analyzeBody(data []byte, encoding string, contentType string) string {
	result := p.analyzeBodyFull(data, encoding, contentType)
	return result.Text
}

// analyzeBodyFull handles decompression and returns both text and raw bytes.
// Raw bytes are preserved when binary data is detected (e.g. protobuf).
func (p *ProxyServer) analyzeBodyFull(data []byte, encoding string, contentType string) AnalyzedBody {
	if len(data) == 0 {
		return AnalyzedBody{}
	}

	raw := data
	decompressed := false // Track if decompression succeeded
	// 1. Decompress if needed (Shadow copy only)
	// First check Content-Encoding header
	if strings.Contains(encoding, "gzip") {
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err == nil {
			decompressedData, err := io.ReadAll(gr)
			if err == nil {
				raw = decompressedData
				decompressed = true
			} else {
				log.Printf("[analyzeBodyFull] Gzip decompression failed: %v", err)
			}
			gr.Close()
		} else {
			log.Printf("[analyzeBodyFull] Gzip reader creation failed: %v", err)
		}
	} else if strings.Contains(encoding, "br") {
		br := brotli.NewReader(bytes.NewReader(data))
		decompressedData, err := io.ReadAll(br)
		if err == nil {
			raw = decompressedData
			decompressed = true
		} else {
			log.Printf("[analyzeBodyFull] Brotli decompression failed: %v", err)
		}
	} else if strings.Contains(encoding, "deflate") {
		fr := flate.NewReader(bytes.NewReader(data))
		decompressedData, err := io.ReadAll(fr)
		if err == nil {
			raw = decompressedData
			decompressed = true
		} else {
			log.Printf("[analyzeBodyFull] Deflate decompression failed: %v", err)
		}
		fr.Close()
	} else if strings.Contains(encoding, "zstd") {
		// Try standard zstd decompression
		decoder, err := zstd.NewReader(bytes.NewReader(data))
		if err == nil {
			decompressedData, err := io.ReadAll(decoder)
			decoder.Close()
			if err == nil {
				raw = decompressedData
				decompressed = true
			} else {
				// Decompression failed - return error message directly
				log.Printf("[analyzeBodyFull] Zstd decompression FAILED: %v", err)
				errMsg := fmt.Sprintf("[zstd decompression failed: %v, %d bytes]", err, len(data))
				if strings.Contains(encoding, "dict") {
					errMsg = fmt.Sprintf("[zstd requires dictionary (%s), %d bytes - cannot decompress without dictionary file]", encoding, len(data))
					log.Printf("[analyzeBodyFull] Zstd needs dictionary: %s", encoding)
				}
				return AnalyzedBody{
					Text:     errMsg,
					RawBytes: append([]byte(nil), data...), // Keep original compressed data
					IsBinary: true,
				}
			}
		} else {
			// Decoder creation failed - return error message directly
			log.Printf("[analyzeBodyFull] Zstd decoder init FAILED: %v", err)
			errMsg := fmt.Sprintf("[zstd decoder init failed: %v, %d bytes]", err, len(data))
			if strings.Contains(encoding, "dict") {
				errMsg = fmt.Sprintf("[zstd requires dictionary (%s), %d bytes - decoder cannot be initialized]", encoding, len(data))
				log.Printf("[analyzeBodyFull] Zstd decoder needs dictionary: %s", encoding)
			}
			return AnalyzedBody{
				Text:     errMsg,
				RawBytes: append([]byte(nil), data...), // Keep original compressed data
				IsBinary: true,
			}
		}
	} else {
		// Auto-detect compression if Content-Encoding not set

		// 1. Check gzip magic number (0x1f 0x8b)
		if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
			gr, err := gzip.NewReader(bytes.NewReader(data))
			if err == nil {
				decompressedData, err := io.ReadAll(gr)
				if err == nil {
					raw = decompressedData
					decompressed = true
				} else {
					log.Printf("[analyzeBodyFull] Gzip auto-detect decompression failed: %v", err)
				}
				gr.Close()
			} else {
				log.Printf("[analyzeBodyFull] Gzip auto-detect reader creation failed: %v", err)
			}
		}

		// 2. Check zstd magic number (0x28 0xB5 0x2F 0xFD)
		if !decompressed && len(data) >= 4 && data[0] == 0x28 && data[1] == 0xB5 && data[2] == 0x2F && data[3] == 0xFD {
			decoder, err := zstd.NewReader(bytes.NewReader(data))
			if err == nil {
				decompressedData, err := io.ReadAll(decoder)
				decoder.Close()
				if err == nil {
					raw = decompressedData
					decompressed = true
				} else {
					// Auto-detected zstd but decompression failed
					log.Printf("[analyzeBodyFull] Zstd auto-detect decompression FAILED: %v", err)
					return AnalyzedBody{
						Text:     fmt.Sprintf("[zstd detected but decompression failed: %v, %d bytes]", err, len(data)),
						RawBytes: append([]byte(nil), data...),
						IsBinary: true,
					}
				}
			} else {
				// Auto-detected zstd but decoder init failed
				log.Printf("[analyzeBodyFull] Zstd auto-detect decoder init FAILED: %v", err)
				return AnalyzedBody{
					Text:     fmt.Sprintf("[zstd detected but decoder init failed: %v, %d bytes]", err, len(data)),
					RawBytes: append([]byte(nil), data...),
					IsBinary: true,
				}
			}
		}

	}

	// 2. Binary Detection
	isBinary := false
	limit := len(raw)
	if limit > 512 {
		limit = 512
	}
	for i := 0; i < limit; i++ {
		if raw[i] == 0 {
			isBinary = true
			break
		}
	}

	if isBinary {
		// Provide more context about the binary data
		msg := fmt.Sprintf("[Binary Data: %d bytes]", len(raw))
		if decompressed {
			// Successfully decompressed but result is binary (e.g. Protobuf)
			msg = fmt.Sprintf("[Binary Data after decompression: %d bytes - possibly Protobuf/MessagePack/etc]", len(raw))
		} else if encoding != "" {
			// Had encoding header but data still binary (decompression might have failed silently)
			msg = fmt.Sprintf("[Binary Data: %d bytes - encoding: %s]", len(raw), encoding)
		}
		return AnalyzedBody{
			Text:     msg,
			RawBytes: append([]byte(nil), raw...), // copy
			IsBinary: true,
		}
	}

	// 3. String Truncation removed: Send full string to UI as requested
	return AnalyzedBody{
		Text: string(raw),
	}
}
