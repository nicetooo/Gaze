package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	net_url "net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"Gaze/pkg/fileutil"
	"Gaze/proxy"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// proxyDeviceId tracks which device the proxy is monitoring for session events
var (
	proxyDeviceId string
	proxyDeviceMu sync.RWMutex

	// Track which session started the proxy (empty = proxy was started manually/externally)
	proxyOwnerSessionID string
	proxyOwnerMu        sync.Mutex

	// Track emitted request IDs to avoid duplicates (with TTL-based cleanup)
	emittedRequests    = make(map[string]int64) // requestId -> timestamp (unix nano)
	emittedRequestsMu  sync.Mutex
	emittedRequestsTTL = int64(5 * 60 * 1e9) // 5 minutes in nanoseconds
	emittedRequestsMax = 5000                // Max entries before cleanup
)

// SetProxyDevice sets which device the proxy is monitoring (for session events)
func (a *App) SetProxyDevice(deviceId string) {
	proxyDeviceMu.Lock()
	proxyDeviceId = deviceId
	proxyDeviceMu.Unlock()
}

// GetProxyDevice returns the currently monitored device
func (a *App) GetProxyDevice() string {
	proxyDeviceMu.RLock()
	defer proxyDeviceMu.RUnlock()
	return proxyDeviceId
}

// setProxyOwnerSession records which session started the proxy
func setProxyOwnerSession(sessionID string) {
	proxyOwnerMu.Lock()
	proxyOwnerSessionID = sessionID
	proxyOwnerMu.Unlock()
}

// getProxyOwnerSession returns the session that started the proxy
func getProxyOwnerSession() string {
	proxyOwnerMu.Lock()
	defer proxyOwnerMu.Unlock()
	return proxyOwnerSessionID
}

// clearProxyOwnerSession clears the proxy owner
func clearProxyOwnerSession() {
	proxyOwnerMu.Lock()
	proxyOwnerSessionID = ""
	proxyOwnerMu.Unlock()
}

// StartProxy starts the internal HTTP/HTTPS proxy
func (a *App) StartProxy(port int) (string, error) {
	LogUserAction(ActionProxyStart, "", map[string]interface{}{
		"port": port,
	})

	// Clear emitted requests map on start
	emittedRequestsMu.Lock()
	emittedRequests = make(map[string]int64)
	emittedRequestsMu.Unlock()

	err := proxy.GetProxy().Start(port, func(req proxy.RequestLog) {
		// Skip partial updates (body-only size updates during transfer)
		if req.PartialUpdate {
			return
		}

		// Skip pending requests (status=0) - wait for response
		if req.StatusCode == 0 {
			return
		}

		// Safety-net dedup: emit only once per request ID.
		// With the proxy.go fixes (logRequest notify=false when TransparentReadCloser
		// will handle it, and doneCalled guard in update), each request should only
		// reach here once. This map is a defensive safeguard.
		emittedRequestsMu.Lock()
		now := time.Now().UnixNano()
		if _, alreadyEmitted := emittedRequests[req.Id]; alreadyEmitted {
			emittedRequestsMu.Unlock()
			return
		}
		emittedRequests[req.Id] = now

		// TTL-based cleanup when map grows large
		if len(emittedRequests) > emittedRequestsMax {
			for id, ts := range emittedRequests {
				if now-ts > emittedRequestsTTL {
					delete(emittedRequests, id)
				}
			}
			if len(emittedRequests) > emittedRequestsMax {
				count := 0
				target := len(emittedRequests) / 2
				for id := range emittedRequests {
					if count >= target {
						break
					}
					delete(emittedRequests, id)
					count++
				}
			}
		}
		emittedRequestsMu.Unlock()

		// Calculate duration from ID (SessionId-TimestampNano)
		var durationMs int64
		parts := strings.Split(req.Id, "-")
		if len(parts) >= 2 {
			if startNano, err := strconv.ParseInt(parts[len(parts)-1], 10, 64); err == nil {
				durationMs = (time.Now().UnixNano() - startNano) / 1e6
			}
		}

		proxyDeviceMu.RLock()
		deviceId := proxyDeviceId
		proxyDeviceMu.RUnlock()

		// Determine level based on status code
		level := "info"
		if req.StatusCode >= 400 && req.StatusCode < 500 {
			level = "warn"
		} else if req.StatusCode >= 500 {
			level = "error"
		}

		title := fmt.Sprintf("%s %s → %d", req.Method, req.URL, req.StatusCode)
		if len(title) > 100 {
			title = title[:97] + "..."
		}

		// Decode protobuf binary bodies if applicable
		respBody := req.RespBody
		reqBody := req.Body
		isProtobuf := false
		isReqProtobuf := false

		// Try response body decoding: URL mapping takes priority over Content-Type
		if len(req.RespBodyRaw) > 0 {
			// Check if there's a URL mapping for this request
			hasMapping := getProtoRegistry().FindMessageForURL(req.URL, "response") != ""
			shouldDecode := hasMapping || isProtobufContentType(req.ContentType)

			if shouldDecode {
				decoded := getProtoDecoder().DecodeBody(req.RespBodyRaw, req.ContentType, req.URL, "response")
				if decoded != "" {
					respBody = decoded
					isProtobuf = true
				}
			}
		}

		// Try request body decoding: URL mapping takes priority over Content-Type
		if len(req.ReqBodyRaw) > 0 {
			// Check request content-type from headers
			reqContentType := ""
			if ct, ok := req.Headers["Content-Type"]; ok && len(ct) > 0 {
				reqContentType = ct[0]
			}

			// Check if there's a URL mapping for this request
			hasMapping := getProtoRegistry().FindMessageForURL(req.URL, "request") != ""
			shouldDecode := hasMapping || isProtobufContentType(reqContentType)

			if shouldDecode {
				decoded := getProtoDecoder().DecodeBody(req.ReqBodyRaw, reqContentType, req.URL, "request")
				if decoded != "" {
					reqBody = decoded
					isReqProtobuf = true
				}
			}
		}

		// Emit directly to EventPipeline (unified storage + frontend broadcast)
		detail := map[string]interface{}{
			"id":              req.Id,
			"method":          req.Method,
			"url":             req.URL,
			"statusCode":      req.StatusCode,
			"contentType":     req.ContentType,
			"bodySize":        req.BodySize,
			"duration":        durationMs,
			"isHttps":         req.IsHTTPS,
			"isWs":            req.IsWs,
			"clientIp":        req.ClientIP,
			"requestHeaders":  req.Headers,
			"requestBody":     reqBody,
			"responseHeaders": req.RespHeaders,
			"responseBody":    respBody,
			"mocked":          req.Mocked,
			"isProtobuf":      isProtobuf,
			"isReqProtobuf":   isReqProtobuf,
		}

		dataBytes, err := json.Marshal(detail)
		if err != nil {
			dataBytes = []byte("{}")
		}

		a.eventPipeline.Emit(UnifiedEvent{
			ID:        uuid.New().String(),
			DeviceID:  deviceId,
			Timestamp: time.Now().UnixMilli(),
			Source:    SourceNetwork,
			Category:  CategoryNetwork,
			Type:      "network_request",
			Level:     ParseEventLevel(level),
			Title:     title,
			Data:      dataBytes,
			Duration:  durationMs,
		})
	})
	if err != nil {
		return "", err
	}

	// Set WebSocket message callback
	proxy.GetProxy().OnWSMessage = func(msg proxy.WSMessage) {
		proxyDeviceMu.RLock()
		deviceId := proxyDeviceId
		proxyDeviceMu.RUnlock()

		// Try protobuf decoding for binary WS frames
		payload := msg.Payload
		isProtobuf := false
		if msg.IsBinary && len(msg.RawPayload) > 0 && msg.URL != "" {
			// Map WS direction to proto direction: "send" → "request", "receive" → "response"
			protoDirection := "response"
			if msg.Direction == "send" {
				protoDirection = "request"
			}
			decoded := getProtoDecoder().DecodeBody(msg.RawPayload, "", msg.URL, protoDirection)
			if decoded != "" {
				payload = decoded
				isProtobuf = true
			}
		}

		// Emit WS message to frontend via Wails event
		if a.ctx != nil {
			wailsRuntime.EventsEmit(a.ctx, "proxy-ws-message", map[string]interface{}{
				"connectionId": msg.ConnectionID,
				"direction":    msg.Direction,
				"type":         msg.Type,
				"typeName":     msg.TypeName,
				"payload":      payload,
				"payloadSize":  msg.PayloadSize,
				"isBinary":     msg.IsBinary,
				"isProtobuf":   isProtobuf,
				"timestamp":    msg.Timestamp,
			})
		}

		// Also emit as event pipeline event for session recording
		detail := map[string]interface{}{
			"connectionId": msg.ConnectionID,
			"direction":    msg.Direction,
			"type":         msg.Type,
			"typeName":     msg.TypeName,
			"payload":      payload,
			"payloadSize":  msg.PayloadSize,
			"isBinary":     msg.IsBinary,
			"isProtobuf":   isProtobuf,
		}
		dataBytes, _ := json.Marshal(detail)
		a.eventPipeline.Emit(UnifiedEvent{
			ID:        uuid.New().String(),
			DeviceID:  deviceId,
			Timestamp: msg.Timestamp,
			Source:    SourceNetwork,
			Category:  CategoryNetwork,
			Type:      "websocket_message",
			Level:     LevelDebug,
			Title:     fmt.Sprintf("WS %s: %s (%d bytes)", msg.Direction, msg.TypeName, msg.PayloadSize),
			Data:      dataBytes,
		})
	}

	// Register enabled mock rules with the proxy
	mockRulesMu.RLock()
	for _, rule := range mockRules {
		if rule.Enabled {
			proxy.GetProxy().AddMockRule(rule.ID, rule.URLPattern, rule.Method, rule.StatusCode, rule.Headers, rule.Body, rule.BodyFile, rule.Delay, toProxyConditions(rule.Conditions))
		}
	}
	mockRulesMu.RUnlock()

	// Register enabled breakpoint rules with the proxy
	breakpointRulesMu.RLock()
	for _, rule := range breakpointRules {
		if rule.Enabled {
			proxy.GetProxy().AddBreakpointRule(rule.ID, rule.URLPattern, rule.Method, rule.Phase)
		}
	}
	breakpointRulesMu.RUnlock()

	// Register enabled map remote rules with the proxy
	mapRemoteRulesMu.RLock()
	for _, rule := range mapRemoteRules {
		if rule.Enabled {
			proxy.GetProxy().AddMapRemoteRule(rule.ID, rule.SourcePattern, rule.TargetURL, rule.Method)
		}
	}
	mapRemoteRulesMu.RUnlock()

	// Register enabled rewrite rules with the proxy
	rewriteRulesMu.RLock()
	for _, rule := range rewriteRules {
		if rule.Enabled {
			proxy.GetProxy().AddRewriteRule(rule.ID, rule.URLPattern, rule.Method, rule.Phase, rule.Target, rule.HeaderName, rule.Match, rule.Replace)
		}
	}
	rewriteRulesMu.RUnlock()

	a.emitProxyStatus(true, port)
	return "Proxy started successfully", nil
}

// SetupProxyForDevice configures adb reverse and device proxy settings
// This allows the device to connect to the proxy via localhost through adb tunnel
func (a *App) SetupProxyForDevice(deviceId string, port int) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}

	// 1. Setup adb reverse: device's localhost:port -> host's localhost:port
	reverseCmd := a.newAdbCommand(nil, "-s", deviceId, "reverse", "tcp:"+strconv.Itoa(port), "tcp:"+strconv.Itoa(port))
	if out, err := reverseCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("adb reverse failed: %v, output: %s", err, string(out))
	}

	// 2. Set device proxy to localhost (traffic goes through adb tunnel)
	proxyCmd := a.newAdbCommand(nil, "-s", deviceId, "shell", "settings", "put", "global", "http_proxy", fmt.Sprintf("127.0.0.1:%d", port))
	if out, err := proxyCmd.CombinedOutput(); err != nil {
		// Try to clean up reverse on failure
		a.newAdbCommand(nil, "-s", deviceId, "reverse", "--remove", "tcp:"+strconv.Itoa(port)).Run()
		return fmt.Errorf("set proxy failed: %v, output: %s", err, string(out))
	}

	return nil
}

// CleanupProxyForDevice removes adb reverse and clears device proxy settings
func (a *App) CleanupProxyForDevice(deviceId string, port int) error {
	if deviceId == "" {
		return nil
	}

	// 1. Clear device proxy
	a.newAdbCommand(nil, "-s", deviceId, "shell", "settings", "put", "global", "http_proxy", ":0").Run()

	// 2. Remove adb reverse
	a.newAdbCommand(nil, "-s", deviceId, "reverse", "--remove", "tcp:"+strconv.Itoa(port)).Run()

	return nil
}

// StopProxy stops the internal proxy and cleans up device settings
func (a *App) StopProxy() (string, error) {
	LogUserAction(ActionProxyStop, "", nil)

	// Get device and port before stopping
	deviceId := a.GetProxyDevice()
	port := proxy.GetProxy().GetPort()

	// Clean up device proxy settings first (before stopping proxy)
	if deviceId != "" && port > 0 {
		a.CleanupProxyForDevice(deviceId, port)
	}

	// Clear the tracked device and owner
	a.SetProxyDevice("")
	clearProxyOwnerSession()

	err := proxy.GetProxy().Stop()
	if err != nil {
		return "", err
	}
	a.emitProxyStatus(false, port)
	return "Proxy stopped successfully", nil
}

// GetProxyStatus returns true if the proxy is running
func (a *App) GetProxyStatus() bool {
	return proxy.GetProxy().IsRunning()
}

// emitProxyStatus notifies the frontend of proxy status changes
func (a *App) emitProxyStatus(running bool, port int) {
	if a.ctx != nil && !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "proxy-status-changed", map[string]interface{}{
			"running": running,
			"port":    port,
		})
	}
}

// SetProxyLimit sets the upload and download speed limits for the proxy server (bytes per second)
func (a *App) SetProxyLimit(uploadSpeed, downloadSpeed int) {
	proxy.GetProxy().SetLimits(uploadSpeed, downloadSpeed)
}

// SetProxyWSEnabled enables or disables WebSocket support
func (a *App) SetProxyWSEnabled(enabled bool) {
	proxy.GetProxy().SetWSEnabled(enabled)
}

// SetProxyMITM enables or disables HTTPS Decryption (MITM)
func (a *App) SetProxyMITM(enabled bool) {
	proxy.GetProxy().SetProxyMITM(enabled)
}

// SetMITMBypassPatterns sets the keywords/domains to bypass MITM
func (a *App) SetMITMBypassPatterns(patterns []string) {
	proxy.GetProxy().SetMITMBypassPatterns(patterns)
}

// GetMITMBypassPatterns returns the current bypass patterns
func (a *App) GetMITMBypassPatterns() []string {
	return proxy.GetProxy().GetMITMBypassPatterns()
}

// GetProxySettings returns the current proxy settings
func (a *App) GetProxySettings() map[string]interface{} {
	return map[string]interface{}{
		"wsEnabled":      proxy.GetProxy().IsWSEnabled(),
		"mitmEnabled":    proxy.GetProxy().IsMITMEnabled(),
		"bypassPatterns": proxy.GetProxy().GetMITMBypassPatterns(),
	}
}

// InstallProxyCert pushes the generated CA certificate to the device
func (a *App) InstallProxyCert(deviceId string) (string, error) {
	certPath := proxy.GetProxy().GetCertPath()
	if certPath == "" {
		return "", nil
	}

	dest := "/sdcard/Download/Gaze-CA.crt"

	cmd := a.newAdbCommand(nil, "-s", deviceId, "push", certPath, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", err
	} else {
		_ = out
	}

	return dest, nil
}

// CheckCertTrust checks if the device has trusted the proxy CA certificate
// Returns: "trusted", "not_trusted", "unknown", or "no_proxy"
func (a *App) CheckCertTrust(deviceId string) string {
	// Check if proxy is running
	p := proxy.GetProxy()
	if !p.IsRunning() {
		return "no_proxy"
	}

	// Check if MITM is enabled
	if !p.IsMITMEnabled() {
		return "unknown"
	}

	// Check recent proxy logs - if we have any successfully decrypted HTTPS requests
	// (requests with response body from HTTPS URLs), the cert is trusted
	hasDecryptedHTTPS := p.HasRecentDecryptedHTTPS()
	if hasDecryptedHTTPS {
		return "trusted"
	}

	// No decrypted HTTPS yet - could be trusted but no traffic, or not trusted
	// Return "pending" to indicate we need more traffic to determine
	return "pending"
}

// SetProxyLatency sets the artificial latency in milliseconds
func (a *App) SetProxyLatency(latencyMs int) {
	proxy.GetProxy().SetLatency(latencyMs)
}

// matchMockRuleLocal checks if a request matches any enabled mock rule.
// headers and body are used for condition evaluation in ResendRequest context.
func matchMockRuleLocal(method, url string, headers map[string]string, body string) *MockRule {
	mockRulesMu.RLock()
	defer mockRulesMu.RUnlock()

	for _, rule := range mockRules {
		if !rule.Enabled {
			continue
		}
		// Check method (empty means match all)
		if rule.Method != "" && rule.Method != method {
			continue
		}
		// Check URL pattern
		if !proxy.MatchPattern(url, rule.URLPattern) {
			continue
		}
		// Check conditions (AND logic)
		if len(rule.Conditions) > 0 && !evaluateConditionsLocal(rule.Conditions, headers, url, body) {
			continue
		}
		return rule
	}
	return nil
}

// evaluateConditionsLocal evaluates conditions using local string data (for ResendRequest).
func evaluateConditionsLocal(conditions []MockCondition, headers map[string]string, rawURL, body string) bool {
	for _, cond := range conditions {
		switch cond.Type {
		case "header":
			val, exists := headers[cond.Key]
			if !matchOperatorLocal(cond.Operator, val, cond.Value, exists) {
				return false
			}
		case "query":
			// Parse query params from URL
			if idx := strings.IndexByte(rawURL, '?'); idx >= 0 {
				params, _ := net_url.ParseQuery(rawURL[idx+1:])
				vals, exists := params[cond.Key]
				val := ""
				if len(vals) > 0 {
					val = vals[0]
				}
				if !matchOperatorLocal(cond.Operator, val, cond.Value, exists) {
					return false
				}
			} else {
				// No query string
				if cond.Operator == "exists" {
					return false
				}
				if cond.Operator != "not_exists" {
					return false
				}
			}
		case "body":
			if !matchOperatorLocal(cond.Operator, body, cond.Value, body != "") {
				return false
			}
		}
	}
	return true
}

// matchOperatorLocal applies a match operator to compare values.
func matchOperatorLocal(operator, actual, expected string, exists bool) bool {
	switch operator {
	case "equals":
		return actual == expected
	case "contains":
		return strings.Contains(actual, expected)
	case "regex":
		re, err := getCachedRegexpLocal(expected)
		if err != nil {
			return false
		}
		return re.MatchString(actual)
	case "exists":
		return exists
	case "not_exists":
		return !exists
	default:
		return actual == expected
	}
}

var (
	localRegexCache   = make(map[string]*regexp.Regexp)
	localRegexCacheMu sync.RWMutex
)

func getCachedRegexpLocal(pattern string) (*regexp.Regexp, error) {
	localRegexCacheMu.RLock()
	if re, ok := localRegexCache[pattern]; ok {
		localRegexCacheMu.RUnlock()
		return re, nil
	}
	localRegexCacheMu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	localRegexCacheMu.Lock()
	localRegexCache[pattern] = re
	localRegexCacheMu.Unlock()
	return re, nil
}

// ResendRequest sends an HTTP request with optional modifications
// Returns the response status, headers, and body
// Checks mock rules first, then sends actual request if no match
func (a *App) ResendRequest(method, url string, headers map[string]string, body string) (map[string]interface{}, error) {
	// Check for matching mock rule first (works without proxy running)
	if mockRule := matchMockRuleLocal(method, url, headers, body); mockRule != nil {
		// Apply mock delay
		if mockRule.Delay > 0 {
			time.Sleep(time.Duration(mockRule.Delay) * time.Millisecond)
		}

		// Build mock response headers
		respHeaders := make(map[string]string)
		for k, v := range mockRule.Headers {
			respHeaders[k] = v
		}
		if respHeaders["Content-Type"] == "" {
			respHeaders["Content-Type"] = "application/json"
		}

		return map[string]interface{}{
			"statusCode":  mockRule.StatusCode,
			"status":      fmt.Sprintf("%d %s", mockRule.StatusCode, http.StatusText(mockRule.StatusCode)),
			"headers":     respHeaders,
			"body":        mockRule.Body,
			"bodySize":    len(mockRule.Body),
			"duration":    int64(mockRule.Delay),
			"contentType": respHeaders["Content-Type"],
			"mocked":      true,
		}, nil
	}

	// No mock match, send actual request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var reqBody io.Reader
	if body != "" {
		reqBody = bytes.NewBufferString(body)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Set headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	duration := time.Since(startTime).Milliseconds()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Build response headers map
	respHeaders := make(map[string]string)
	for k, v := range resp.Header {
		respHeaders[k] = strings.Join(v, ", ")
	}

	return map[string]interface{}{
		"statusCode":  resp.StatusCode,
		"status":      resp.Status,
		"headers":     respHeaders,
		"body":        string(respBody),
		"bodySize":    len(respBody),
		"duration":    duration,
		"contentType": resp.Header.Get("Content-Type"),
	}, nil
}

// MockCondition defines an additional match condition for mock rules.
type MockCondition struct {
	Type     string `json:"type"`     // "header", "query", "body"
	Key      string `json:"key"`      // header name or query param name (unused for body type)
	Operator string `json:"operator"` // "equals", "contains", "regex", "exists", "not_exists"
	Value    string `json:"value"`    // expected value (unused for exists/not_exists)
}

// MockRule defines a rule for mocking HTTP responses
type MockRule struct {
	ID          string            `json:"id"`
	URLPattern  string            `json:"urlPattern"`  // URL pattern to match (supports * wildcard)
	Method      string            `json:"method"`      // HTTP method to match (empty = all)
	StatusCode  int               `json:"statusCode"`  // Response status code
	Headers     map[string]string `json:"headers"`     // Response headers
	Body        string            `json:"body"`        // Response body
	BodyFile    string            `json:"bodyFile"`    // Path to local file for response body (overrides Body if set)
	Delay       int               `json:"delay"`       // Delay in milliseconds before responding
	Enabled     bool              `json:"enabled"`     // Whether this rule is active
	Description string            `json:"description"` // Optional description
	CreatedAt   int64             `json:"createdAt"`   // Unix milliseconds, for stable ordering
	Conditions  []MockCondition   `json:"conditions"`  // Additional match conditions (AND logic)
}

// toProxyConditions converts app-level conditions to proxy-level conditions.
func toProxyConditions(conditions []MockCondition) []proxy.MockCondition {
	if len(conditions) == 0 {
		return nil
	}
	result := make([]proxy.MockCondition, len(conditions))
	for i, c := range conditions {
		result[i] = proxy.MockCondition{
			Type:     c.Type,
			Key:      c.Key,
			Operator: c.Operator,
			Value:    c.Value,
		}
	}
	return result
}

var (
	mockRules   = make(map[string]*MockRule)
	mockRulesMu sync.RWMutex
	// Set when the rules file on disk failed to parse; full-overwrite saves are
	// refused until restart so a half-loaded (empty) map can't destroy user data.
	mockRulesLoadFailed bool
)

// logSaveErr logs a rules persistence failure from call sites whose Wails
// binding signature has no error return.
func logSaveErr(kind string, err error) {
	if err != nil {
		LogError("proxy_rules").Err(err).Str("kind", kind).Msg("Failed to persist rules")
	}
}

// quarantineCorruptConfig preserves an unparseable config/rules file as
// <path>.corrupt-<ts> and logs the outcome. Returns true if the file was moved.
func quarantineCorruptConfig(kind, path string, parseErr error) bool {
	target, renameErr := fileutil.QuarantineCorrupt(path)
	if renameErr != nil {
		LogError("config_persist").Err(parseErr).Str("kind", kind).Str("path", path).
			Msgf("Config file corrupt and quarantine failed: %v", renameErr)
		return false
	}
	LogError("config_persist").Err(parseErr).Str("kind", kind).Str("preserved", target).
		Msg("Config file corrupt; preserved on disk, saves disabled until restart")
	return true
}

// getMockRulesPath returns the path to the mock rules JSON file
func getMockRulesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".adbGUI", "mock_rules.json")
}

// saveMockRules saves mock rules to disk
func saveMockRules() error {
	if mockRulesLoadFailed {
		return fmt.Errorf("mock rules file failed to load at startup; refusing to overwrite (restart to retry)")
	}
	rules := make([]*MockRule, 0, len(mockRules))
	for _, rule := range mockRules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt < rules[j].CreatedAt
	})

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}

	path := getMockRulesPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return fileutil.WriteFileAtomic(path, data, 0644)
}

// LoadMockRules loads mock rules from disk (called on app startup)
func (a *App) LoadMockRules() error {
	mockRulesMu.Lock()
	defer mockRulesMu.Unlock()

	path := getMockRulesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No saved rules, that's fine
		}
		return err
	}

	var rules []*MockRule
	if err := json.Unmarshal(data, &rules); err != nil {
		mockRulesLoadFailed = true
		quarantineCorruptConfig("mock", path, err)
		return err
	}
	mockRulesLoadFailed = false

	mockRules = make(map[string]*MockRule)
	for _, rule := range rules {
		mockRules[rule.ID] = rule
		// Register enabled rules with proxy if it's running
		if rule.Enabled && proxy.GetProxy().IsRunning() {
			proxy.GetProxy().AddMockRule(rule.ID, rule.URLPattern, rule.Method, rule.StatusCode, rule.Headers, rule.Body, rule.BodyFile, rule.Delay, toProxyConditions(rule.Conditions))
		}
	}

	return nil
}

// AddMockRule adds a new mock response rule
func (a *App) AddMockRule(rule MockRule) string {
	mockRulesMu.Lock()
	defer mockRulesMu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	if rule.CreatedAt == 0 {
		rule.CreatedAt = time.Now().UnixMilli()
	}
	rule.Enabled = true
	mockRules[rule.ID] = &rule

	// Register with proxy
	proxy.GetProxy().AddMockRule(rule.ID, rule.URLPattern, rule.Method, rule.StatusCode, rule.Headers, rule.Body, rule.BodyFile, rule.Delay, toProxyConditions(rule.Conditions))

	// Persist to disk
	logSaveErr("mock", saveMockRules())

	return rule.ID
}

// UpdateMockRule updates an existing mock rule
func (a *App) UpdateMockRule(rule MockRule) error {
	mockRulesMu.Lock()
	defer mockRulesMu.Unlock()

	if _, exists := mockRules[rule.ID]; !exists {
		return fmt.Errorf("rule not found: %s", rule.ID)
	}

	mockRules[rule.ID] = &rule

	// Update in proxy
	proxy.GetProxy().RemoveMockRule(rule.ID)
	if rule.Enabled {
		proxy.GetProxy().AddMockRule(rule.ID, rule.URLPattern, rule.Method, rule.StatusCode, rule.Headers, rule.Body, rule.BodyFile, rule.Delay, toProxyConditions(rule.Conditions))
	}

	// Persist to disk
	return saveMockRules()
}

// RemoveMockRule removes a mock response rule
func (a *App) RemoveMockRule(ruleID string) {
	mockRulesMu.Lock()
	defer mockRulesMu.Unlock()

	delete(mockRules, ruleID)
	proxy.GetProxy().RemoveMockRule(ruleID)

	// Persist to disk
	logSaveErr("mock", saveMockRules())
}

// ========================================
// Auto Rewrite Rules Management
// ========================================

// RewriteRule (app-level) defines an auto-rewrite rule
type RewriteRule struct {
	ID          string `json:"id"`
	URLPattern  string `json:"urlPattern"`
	Method      string `json:"method"`     // empty = match all
	Phase       string `json:"phase"`      // "request", "response", or "both"
	Target      string `json:"target"`     // "header" or "body"
	HeaderName  string `json:"headerName"` // header name when target is "header"
	Match       string `json:"match"`      // regex pattern
	Replace     string `json:"replace"`    // replacement string
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"createdAt"`
}

var (
	rewriteRules           = make(map[string]*RewriteRule)
	rewriteRulesMu         sync.RWMutex
	rewriteRulesLoadFailed bool
)

func getRewriteRulesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".adbGUI", "rewrite_rules.json")
}

func saveRewriteRules() error {
	if rewriteRulesLoadFailed {
		return fmt.Errorf("rewrite rules file failed to load at startup; refusing to overwrite (restart to retry)")
	}
	rules := make([]*RewriteRule, 0, len(rewriteRules))
	for _, rule := range rewriteRules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt < rules[j].CreatedAt
	})

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}

	path := getRewriteRulesPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(path, data, 0644)
}

// LoadRewriteRules loads rewrite rules from disk (called on app startup)
func (a *App) LoadRewriteRules() error {
	rewriteRulesMu.Lock()
	defer rewriteRulesMu.Unlock()

	path := getRewriteRulesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // file not found is ok
	}

	var rules []*RewriteRule
	if err := json.Unmarshal(data, &rules); err != nil {
		rewriteRulesLoadFailed = true
		quarantineCorruptConfig("rewrite", path, err)
		return err
	}
	rewriteRulesLoadFailed = false
	rewriteRules = make(map[string]*RewriteRule)
	for _, rule := range rules {
		rewriteRules[rule.ID] = rule
	}
	return nil
}

func (a *App) AddRewriteRule(urlPattern, method, phase, target, headerName, match, replace, description string) string {
	rewriteRulesMu.Lock()
	defer rewriteRulesMu.Unlock()

	rule := RewriteRule{
		ID:          fmt.Sprintf("rw-%d", time.Now().UnixNano()),
		URLPattern:  urlPattern,
		Method:      method,
		Phase:       phase,
		Target:      target,
		HeaderName:  headerName,
		Match:       match,
		Replace:     replace,
		Enabled:     true,
		Description: description,
		CreatedAt:   time.Now().UnixMilli(),
	}
	rewriteRules[rule.ID] = &rule

	proxy.GetProxy().AddRewriteRule(rule.ID, rule.URLPattern, rule.Method, rule.Phase, rule.Target, rule.HeaderName, rule.Match, rule.Replace)
	logSaveErr("rewrite", saveRewriteRules())
	return rule.ID
}

func (a *App) UpdateRewriteRule(id, urlPattern, method, phase, target, headerName, match, replace string, enabled bool, description string) error {
	rewriteRulesMu.Lock()
	defer rewriteRulesMu.Unlock()

	rule, exists := rewriteRules[id]
	if !exists {
		return fmt.Errorf("rule not found: %s", id)
	}

	rule.URLPattern = urlPattern
	rule.Method = method
	rule.Phase = phase
	rule.Target = target
	rule.HeaderName = headerName
	rule.Match = match
	rule.Replace = replace
	rule.Enabled = enabled
	rule.Description = description

	proxy.GetProxy().RemoveRewriteRule(id)
	if enabled {
		proxy.GetProxy().AddRewriteRule(id, urlPattern, method, phase, target, headerName, match, replace)
	}

	return saveRewriteRules()
}

func (a *App) RemoveRewriteRule(ruleID string) {
	rewriteRulesMu.Lock()
	defer rewriteRulesMu.Unlock()

	delete(rewriteRules, ruleID)
	proxy.GetProxy().RemoveRewriteRule(ruleID)
	logSaveErr("rewrite", saveRewriteRules())
}

func (a *App) GetRewriteRules() []*RewriteRule {
	rewriteRulesMu.RLock()
	defer rewriteRulesMu.RUnlock()

	rules := make([]*RewriteRule, 0, len(rewriteRules))
	for _, rule := range rewriteRules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt < rules[j].CreatedAt
	})
	return rules
}

func (a *App) ToggleRewriteRule(ruleID string, enabled bool) error {
	rewriteRulesMu.Lock()
	defer rewriteRulesMu.Unlock()

	rule, exists := rewriteRules[ruleID]
	if !exists {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	rule.Enabled = enabled
	if enabled {
		proxy.GetProxy().AddRewriteRule(rule.ID, rule.URLPattern, rule.Method, rule.Phase, rule.Target, rule.HeaderName, rule.Match, rule.Replace)
	} else {
		proxy.GetProxy().RemoveRewriteRule(rule.ID)
	}

	return saveRewriteRules()
}

// ExportMockRules returns all mock rules as a JSON string for export
func (a *App) ExportMockRules() (string, error) {
	mockRulesMu.RLock()
	defer mockRulesMu.RUnlock()

	rules := make([]*MockRule, 0, len(mockRules))
	for _, rule := range mockRules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt < rules[j].CreatedAt
	})

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal mock rules: %w", err)
	}
	return string(data), nil
}

// ImportMockRules imports mock rules from a JSON string, merging with existing rules
func (a *App) ImportMockRules(jsonStr string) (int, error) {
	var imported []*MockRule
	if err := json.Unmarshal([]byte(jsonStr), &imported); err != nil {
		return 0, fmt.Errorf("invalid JSON format: %w", err)
	}

	mockRulesMu.Lock()
	defer mockRulesMu.Unlock()

	added := 0
	for _, rule := range imported {
		if rule.URLPattern == "" || rule.StatusCode == 0 {
			continue // skip invalid rules
		}
		// Generate new ID to avoid conflicts
		rule.ID = fmt.Sprintf("mock-%d-%d", time.Now().UnixNano(), added)
		rule.CreatedAt = time.Now().UnixMilli()
		mockRules[rule.ID] = rule
		added++

		// Register with proxy if enabled and proxy is running
		if rule.Enabled {
			proxy.GetProxy().AddMockRule(rule.ID, rule.URLPattern, rule.Method, rule.StatusCode, rule.Headers, rule.Body, rule.BodyFile, rule.Delay, toProxyConditions(rule.Conditions))
		}
	}

	if added > 0 {
		if err := saveMockRules(); err != nil {
			return added, err
		}
	}

	return added, nil
}

// ========================================
// Map Remote Rules Management
// ========================================

// MapRemoteRule (app-level) defines a URL rewriting rule
type MapRemoteRule struct {
	ID            string `json:"id"`
	SourcePattern string `json:"sourcePattern"` // URL wildcard pattern to match
	TargetURL     string `json:"targetUrl"`     // Target URL to redirect to
	Method        string `json:"method"`        // HTTP method to match (empty = all)
	Enabled       bool   `json:"enabled"`
	Description   string `json:"description"`
	CreatedAt     int64  `json:"createdAt"`
}

var (
	mapRemoteRules           = make(map[string]*MapRemoteRule)
	mapRemoteRulesMu         sync.RWMutex
	mapRemoteRulesLoadFailed bool
)

func getMapRemoteRulesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".adbGUI", "map_remote_rules.json")
}

func saveMapRemoteRules() error {
	if mapRemoteRulesLoadFailed {
		return fmt.Errorf("map remote rules file failed to load at startup; refusing to overwrite (restart to retry)")
	}
	rules := make([]*MapRemoteRule, 0, len(mapRemoteRules))
	for _, rule := range mapRemoteRules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt < rules[j].CreatedAt
	})
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	path := getMapRemoteRulesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(path, data, 0644)
}

// LoadMapRemoteRules loads map remote rules from disk (called on app startup)
func (a *App) LoadMapRemoteRules() error {
	mapRemoteRulesMu.Lock()
	defer mapRemoteRulesMu.Unlock()

	path := getMapRemoteRulesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var rules []*MapRemoteRule
	if err := json.Unmarshal(data, &rules); err != nil {
		mapRemoteRulesLoadFailed = true
		quarantineCorruptConfig("map_remote", path, err)
		return err
	}
	mapRemoteRulesLoadFailed = false
	mapRemoteRules = make(map[string]*MapRemoteRule)
	for _, rule := range rules {
		mapRemoteRules[rule.ID] = rule
		// Register enabled rules with proxy if it's running
		if rule.Enabled && proxy.GetProxy().IsRunning() {
			proxy.GetProxy().AddMapRemoteRule(rule.ID, rule.SourcePattern, rule.TargetURL, rule.Method)
		}
	}
	return nil
}

func (a *App) AddMapRemoteRule(sourcePattern, targetURL, method, description string) string {
	mapRemoteRulesMu.Lock()
	defer mapRemoteRulesMu.Unlock()

	rule := MapRemoteRule{
		ID:            fmt.Sprintf("map-%d", time.Now().UnixNano()),
		SourcePattern: sourcePattern,
		TargetURL:     targetURL,
		Method:        method,
		Enabled:       true,
		Description:   description,
		CreatedAt:     time.Now().UnixMilli(),
	}
	mapRemoteRules[rule.ID] = &rule

	proxy.GetProxy().AddMapRemoteRule(rule.ID, rule.SourcePattern, rule.TargetURL, rule.Method)
	logSaveErr("map_remote", saveMapRemoteRules())

	return rule.ID
}

func (a *App) UpdateMapRemoteRule(id, sourcePattern, targetURL, method string, enabled bool, description string) error {
	mapRemoteRulesMu.Lock()
	defer mapRemoteRulesMu.Unlock()

	rule, exists := mapRemoteRules[id]
	if !exists {
		return fmt.Errorf("rule not found: %s", id)
	}

	rule.SourcePattern = sourcePattern
	rule.TargetURL = targetURL
	rule.Method = method
	rule.Enabled = enabled
	rule.Description = description

	// Update proxy
	proxy.GetProxy().RemoveMapRemoteRule(id)
	if enabled {
		proxy.GetProxy().AddMapRemoteRule(id, sourcePattern, targetURL, method)
	}

	return saveMapRemoteRules()
}

func (a *App) RemoveMapRemoteRule(ruleID string) {
	mapRemoteRulesMu.Lock()
	defer mapRemoteRulesMu.Unlock()

	delete(mapRemoteRules, ruleID)
	proxy.GetProxy().RemoveMapRemoteRule(ruleID)
	logSaveErr("map_remote", saveMapRemoteRules())
}

func (a *App) GetMapRemoteRules() []*MapRemoteRule {
	mapRemoteRulesMu.RLock()
	defer mapRemoteRulesMu.RUnlock()

	rules := make([]*MapRemoteRule, 0, len(mapRemoteRules))
	for _, rule := range mapRemoteRules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt < rules[j].CreatedAt
	})
	return rules
}

func (a *App) ToggleMapRemoteRule(ruleID string, enabled bool) error {
	mapRemoteRulesMu.Lock()
	defer mapRemoteRulesMu.Unlock()

	rule, exists := mapRemoteRules[ruleID]
	if !exists {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	rule.Enabled = enabled
	if enabled {
		proxy.GetProxy().AddMapRemoteRule(rule.ID, rule.SourcePattern, rule.TargetURL, rule.Method)
	} else {
		proxy.GetProxy().RemoveMapRemoteRule(rule.ID)
	}

	return saveMapRemoteRules()
}

// ========================================
// Breakpoint Rules Management
// ========================================

// BreakpointRule (app-level) extends the proxy BreakpointRule with UI metadata
type BreakpointRule struct {
	ID          string `json:"id"`
	URLPattern  string `json:"urlPattern"`
	Method      string `json:"method"` // empty = match all
	Phase       string `json:"phase"`  // "request", "response", "both"
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"createdAt"`
}

var (
	breakpointRules           = make(map[string]*BreakpointRule)
	breakpointRulesMu         sync.RWMutex
	breakpointRulesLoadFailed bool
)

// getBreakpointRulesPath returns the path to the breakpoint rules JSON file
func getBreakpointRulesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".adbGUI", "breakpoint_rules.json")
}

// saveBreakpointRules saves breakpoint rules to disk
func saveBreakpointRules() error {
	if breakpointRulesLoadFailed {
		return fmt.Errorf("breakpoint rules file failed to load at startup; refusing to overwrite (restart to retry)")
	}
	rules := make([]*BreakpointRule, 0, len(breakpointRules))
	for _, rule := range breakpointRules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt < rules[j].CreatedAt
	})

	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}

	path := getBreakpointRulesPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return fileutil.WriteFileAtomic(path, data, 0644)
}

// LoadBreakpointRules loads breakpoint rules from disk (called on app startup)
func (a *App) LoadBreakpointRules() error {
	breakpointRulesMu.Lock()
	defer breakpointRulesMu.Unlock()

	path := getBreakpointRulesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var rules []*BreakpointRule
	if err := json.Unmarshal(data, &rules); err != nil {
		breakpointRulesLoadFailed = true
		quarantineCorruptConfig("breakpoint", path, err)
		return err
	}
	breakpointRulesLoadFailed = false

	breakpointRules = make(map[string]*BreakpointRule)
	for _, rule := range rules {
		breakpointRules[rule.ID] = rule
		// Register enabled rules with proxy if it's running
		if rule.Enabled && proxy.GetProxy().IsRunning() {
			proxy.GetProxy().AddBreakpointRule(rule.ID, rule.URLPattern, rule.Method, rule.Phase)
		}
	}

	return nil
}

// AddBreakpointRule adds a new breakpoint rule
func (a *App) AddBreakpointRule(rule BreakpointRule) string {
	breakpointRulesMu.Lock()
	defer breakpointRulesMu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	if rule.CreatedAt == 0 {
		rule.CreatedAt = time.Now().UnixMilli()
	}
	rule.Enabled = true
	breakpointRules[rule.ID] = &rule

	// Register with proxy engine
	proxy.GetProxy().AddBreakpointRule(rule.ID, rule.URLPattern, rule.Method, rule.Phase)

	// Persist to disk
	logSaveErr("breakpoint", saveBreakpointRules())

	return rule.ID
}

// UpdateBreakpointRule updates an existing breakpoint rule
func (a *App) UpdateBreakpointRule(rule BreakpointRule) error {
	breakpointRulesMu.Lock()
	defer breakpointRulesMu.Unlock()

	if _, exists := breakpointRules[rule.ID]; !exists {
		return fmt.Errorf("breakpoint rule not found: %s", rule.ID)
	}

	breakpointRules[rule.ID] = &rule

	// Update in proxy engine
	proxy.GetProxy().RemoveBreakpointRule(rule.ID)
	if rule.Enabled {
		proxy.GetProxy().AddBreakpointRule(rule.ID, rule.URLPattern, rule.Method, rule.Phase)
	}

	return saveBreakpointRules()
}

// RemoveBreakpointRule removes a breakpoint rule
func (a *App) RemoveBreakpointRule(ruleID string) {
	breakpointRulesMu.Lock()
	defer breakpointRulesMu.Unlock()

	delete(breakpointRules, ruleID)
	proxy.GetProxy().RemoveBreakpointRule(ruleID)

	logSaveErr("breakpoint", saveBreakpointRules())
}

// GetBreakpointRules returns all breakpoint rules, sorted by creation time
func (a *App) GetBreakpointRules() []*BreakpointRule {
	breakpointRulesMu.RLock()
	defer breakpointRulesMu.RUnlock()

	rules := make([]*BreakpointRule, 0, len(breakpointRules))
	for _, rule := range breakpointRules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt < rules[j].CreatedAt
	})
	return rules
}

// ToggleBreakpointRule enables or disables a breakpoint rule
func (a *App) ToggleBreakpointRule(ruleID string, enabled bool) error {
	breakpointRulesMu.Lock()
	defer breakpointRulesMu.Unlock()

	rule, exists := breakpointRules[ruleID]
	if !exists {
		return fmt.Errorf("breakpoint rule not found: %s", ruleID)
	}

	rule.Enabled = enabled

	if enabled {
		proxy.GetProxy().AddBreakpointRule(rule.ID, rule.URLPattern, rule.Method, rule.Phase)
	} else {
		proxy.GetProxy().RemoveBreakpointRule(rule.ID)
	}

	return saveBreakpointRules()
}

// ResolveBreakpoint resolves a pending breakpoint with the user's action
func (a *App) ResolveBreakpoint(breakpointID string, action string, modifications map[string]interface{}) error {
	resolution := proxy.BreakpointResolution{
		Action: action,
	}

	// Parse modifications
	if modifications != nil {
		if v, ok := modifications["method"].(string); ok {
			resolution.ModifiedMethod = v
		}
		if v, ok := modifications["url"].(string); ok {
			resolution.ModifiedURL = v
		}
		if v, ok := modifications["body"].(string); ok {
			resolution.ModifiedBody = v
		}
		if v, ok := modifications["headers"].(map[string]interface{}); ok {
			resolution.ModifiedHeaders = make(map[string]string)
			for k, val := range v {
				if s, ok := val.(string); ok {
					resolution.ModifiedHeaders[k] = s
				}
			}
		}
		// Response modifications
		if v, ok := modifications["statusCode"].(float64); ok {
			resolution.ModifiedStatusCode = int(v)
		}
		if v, ok := modifications["respBody"].(string); ok {
			resolution.ModifiedRespBody = v
		}
		if v, ok := modifications["respHeaders"].(map[string]interface{}); ok {
			resolution.ModifiedRespHeaders = make(map[string]string)
			for k, val := range v {
				if s, ok := val.(string); ok {
					resolution.ModifiedRespHeaders[k] = s
				}
			}
		}
	}

	return proxy.GetProxy().ResolveBreakpoint(breakpointID, resolution)
}

// GetPendingBreakpoints returns all pending (paused) breakpoints
func (a *App) GetPendingBreakpoints() []proxy.PendingBreakpointInfo {
	return proxy.GetProxy().GetPendingBreakpoints()
}

// ForwardAllBreakpoints forwards all pending breakpoints immediately
func (a *App) ForwardAllBreakpoints() {
	proxy.GetProxy().ForwardAllBreakpoints()
}

// SetupBreakpointCallbacks registers Wails event callbacks for breakpoint notifications
func (a *App) SetupBreakpointCallbacks() {
	proxy.GetProxy().SetBreakpointHitCallback(func(info proxy.PendingBreakpointInfo) {
		if a.ctx != nil && !a.mcpMode {
			wailsRuntime.EventsEmit(a.ctx, "breakpoint-hit", info)
		}
	})
	proxy.GetProxy().SetBreakpointResolvedCallback(func(id string, reason string) {
		if a.ctx != nil && !a.mcpMode {
			wailsRuntime.EventsEmit(a.ctx, "breakpoint-resolved", map[string]string{
				"id":     id,
				"reason": reason,
			})
		}
	})
	proxy.GetProxy().SetBreakpointCapacityCallback(func(ruleID, url, phase string) {
		if a.ctx != nil && !a.mcpMode {
			wailsRuntime.EventsEmit(a.ctx, "proxy-breakpoint-capacity", map[string]string{
				"ruleId": ruleID,
				"url":    url,
				"phase":  phase,
			})
		}
	})
}

// GetMockRules returns all mock response rules, sorted by creation time (oldest first)
func (a *App) GetMockRules() []*MockRule {
	mockRulesMu.RLock()
	defer mockRulesMu.RUnlock()

	rules := make([]*MockRule, 0, len(mockRules))
	for _, rule := range mockRules {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].CreatedAt < rules[j].CreatedAt
	})
	return rules
}

// ToggleMockRule enables or disables a mock rule
func (a *App) ToggleMockRule(ruleID string, enabled bool) error {
	mockRulesMu.Lock()
	defer mockRulesMu.Unlock()

	rule, exists := mockRules[ruleID]
	if !exists {
		return fmt.Errorf("rule not found: %s", ruleID)
	}

	rule.Enabled = enabled

	if enabled {
		proxy.GetProxy().AddMockRule(rule.ID, rule.URLPattern, rule.Method, rule.StatusCode, rule.Headers, rule.Body, rule.BodyFile, rule.Delay, toProxyConditions(rule.Conditions))
	} else {
		proxy.GetProxy().RemoveMockRule(rule.ID)
	}

	// Persist to disk
	return saveMockRules()
}
