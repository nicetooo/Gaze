package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerProxyTools registers proxy management tools
func (s *MCPServer) registerProxyTools() {
	// proxy_start - Start the proxy
	s.server.AddTool(
		mcp.NewTool("proxy_start",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Start the HTTP/HTTPS proxy for network interception"),
			mcp.WithNumber("port",
				mcp.Description("Port to listen on (default: 8080)"),
			),
		),
		s.handleProxyStart,
	)

	// proxy_stop - Stop the proxy
	s.server.AddTool(
		mcp.NewTool("proxy_stop",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Stop the HTTP/HTTPS proxy"),
		),
		s.handleProxyStop,
	)

	// proxy_status - Get proxy status
	s.server.AddTool(
		mcp.NewTool("proxy_status",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Get the current proxy status"),
		),
		s.handleProxyStatus,
	)

	// mock_rule_list - List all mock rules
	s.server.AddTool(
		mcp.NewTool("mock_rule_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all HTTP mock response rules.

Returns all configured mock rules with their URL patterns, methods, status codes,
response bodies, delays, and enabled states. Rules are sorted by creation time.`),
		),
		s.handleMockRuleList,
	)

	// mock_rule_add - Add a mock rule
	s.server.AddTool(
		mcp.NewTool("mock_rule_add",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Add a new HTTP mock response rule.

When the proxy intercepts a request matching the URL pattern (and optional method),
it returns the configured mock response instead of forwarding to the real server.

URL patterns support * wildcard:
  - "*/api/users*" matches any URL containing /api/users
  - "https://example.com/api/*" matches paths under /api/
  - "*" matches all URLs

Examples:
  Mock a JSON API: urlPattern="*/api/login", statusCode=200, body='{"token":"abc"}'
  Simulate error:  urlPattern="*/api/data", statusCode=500, body='{"error":"Internal Server Error"}'
  Add delay:       urlPattern="*/api/slow", statusCode=200, body='{}', delay=3000`),
			mcp.WithString("url_pattern",
				mcp.Required(),
				mcp.Description("URL wildcard pattern to match (e.g., '*/api/users*')"),
			),
			mcp.WithString("method",
				mcp.Description("HTTP method to match (empty = match all). e.g., 'GET', 'POST'"),
			),
			mcp.WithNumber("status_code",
				mcp.Required(),
				mcp.Description("HTTP status code for the mock response (e.g., 200, 404, 500)"),
			),
			mcp.WithString("body",
				mcp.Description("Response body content (JSON, XML, HTML, or plain text)"),
			),
			mcp.WithNumber("delay",
				mcp.Description("Delay in milliseconds before responding (default: 0)"),
			),
			mcp.WithString("headers_json",
				mcp.Description(`Response headers as JSON object, e.g., '{"Content-Type":"application/json","X-Custom":"value"}'`),
			),
			mcp.WithString("description",
				mcp.Description("Optional description of what this mock rule does"),
			),
			mcp.WithString("body_file",
				mcp.Description("Path to a local file to use as response body (Map Local). When set, file content is used instead of 'body' field. The file is read on each request."),
			),
			mcp.WithString("conditions_json",
				mcp.Description(`Optional JSON array of match conditions (AND logic). Each condition:
  {"type":"header|query|body", "key":"header-name or param-name", "operator":"equals|contains|regex|exists|not_exists", "value":"expected"}
Example: '[{"type":"header","key":"Authorization","operator":"exists"},{"type":"query","key":"page","operator":"equals","value":"1"}]'`),
			),
		),
		s.handleMockRuleAdd,
	)

	// mock_rule_update - Update a mock rule
	s.server.AddTool(
		mcp.NewTool("mock_rule_update",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Update an existing mock rule. All fields are replaced with the new values.`),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the mock rule to update"),
			),
			mcp.WithString("url_pattern",
				mcp.Required(),
				mcp.Description("URL wildcard pattern to match"),
			),
			mcp.WithString("method",
				mcp.Description("HTTP method to match (empty = match all)"),
			),
			mcp.WithNumber("status_code",
				mcp.Required(),
				mcp.Description("HTTP status code for the mock response"),
			),
			mcp.WithString("body",
				mcp.Description("Response body content"),
			),
			mcp.WithNumber("delay",
				mcp.Description("Delay in milliseconds before responding"),
			),
			mcp.WithString("headers_json",
				mcp.Description("Response headers as JSON object"),
			),
			mcp.WithBoolean("enabled",
				mcp.Description("Whether this rule is active (default: true)"),
			),
			mcp.WithString("description",
				mcp.Description("Optional description"),
			),
			mcp.WithString("body_file",
				mcp.Description("Path to a local file to use as response body (Map Local). When set, file content is used instead of 'body' field."),
			),
			mcp.WithString("conditions_json",
				mcp.Description(`Optional JSON array of match conditions (AND logic). Same format as mock_rule_add.`),
			),
		),
		s.handleMockRuleUpdate,
	)

	// mock_rule_remove - Remove a mock rule
	s.server.AddTool(
		mcp.NewTool("mock_rule_remove",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("Remove a mock rule by ID."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the mock rule to remove"),
			),
		),
		s.handleMockRuleRemove,
	)

	// mock_rule_toggle - Enable/disable a mock rule
	s.server.AddTool(
		mcp.NewTool("mock_rule_toggle",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Enable or disable a mock rule without removing it."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the mock rule to toggle"),
			),
			mcp.WithBoolean("enabled",
				mcp.Required(),
				mcp.Description("Whether to enable (true) or disable (false) the rule"),
			),
		),
		s.handleMockRuleToggle,
	)

	// mock_rule_export - Export all mock rules as JSON
	s.server.AddTool(
		mcp.NewTool("mock_rule_export",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Export all mock rules as a JSON string. Useful for sharing rules across teams or backing up configurations."),
		),
		s.handleMockRuleExport,
	)

	// mock_rule_import - Import mock rules from JSON
	s.server.AddTool(
		mcp.NewTool("mock_rule_import",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Import mock rules from a JSON string. Rules are merged with existing rules (new IDs are generated to avoid conflicts). Invalid rules (missing urlPattern or statusCode) are skipped."),
			mcp.WithString("json",
				mcp.Required(),
				mcp.Description("JSON array of mock rules to import"),
			),
		),
		s.handleMockRuleImport,
	)

	// proxy_configure - Configure proxy settings
	s.server.AddTool(
		mcp.NewTool("proxy_configure",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Configure proxy settings. All parameters are optional — only provided settings will be changed.

Settings:
- mitm: Enable/disable HTTPS MITM decryption
- ws_enabled: Enable/disable WebSocket pass-through
- upload_speed: Upload speed limit in KB/s (0 = unlimited)
- download_speed: Download speed limit in KB/s (0 = unlimited)
- latency: Artificial latency in milliseconds (0 = no latency)
- bypass_patterns: JSON array of domain patterns to bypass MITM (replaces existing list)

Examples:
  Enable MITM: mitm=true
  Throttle to 100KB/s: upload_speed=100, download_speed=100
  Add 200ms latency: latency=200
  Set bypass: bypass_patterns='["cdn","static","*.google.com"]'`),
			mcp.WithBoolean("mitm",
				mcp.Description("Enable/disable HTTPS MITM decryption"),
			),
			mcp.WithBoolean("ws_enabled",
				mcp.Description("Enable/disable WebSocket pass-through"),
			),
			mcp.WithNumber("upload_speed",
				mcp.Description("Upload speed limit in KB/s (0 = unlimited)"),
			),
			mcp.WithNumber("download_speed",
				mcp.Description("Download speed limit in KB/s (0 = unlimited)"),
			),
			mcp.WithNumber("latency",
				mcp.Description("Artificial latency in milliseconds (0 = none)"),
			),
			mcp.WithString("bypass_patterns",
				mcp.Description(`JSON array of MITM bypass domain patterns, e.g. '["cdn","*.google.com"]'. Replaces existing list.`),
			),
		),
		s.handleProxyConfigure,
	)

	// proxy_settings - Get current proxy settings
	s.server.AddTool(
		mcp.NewTool("proxy_settings",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Get current proxy settings including MITM state, WebSocket state, bypass patterns, and connected device."),
		),
		s.handleProxySettings,
	)

	// proxy_device_setup - Set up proxy on a device
	s.server.AddTool(
		mcp.NewTool("proxy_device_setup",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription(`Set up the proxy on a connected Android device.

This performs:
1. Sets the proxy device ID
2. Creates an adb reverse tunnel (device port -> host proxy port)
3. Configures the device HTTP proxy settings

The device will route HTTP/HTTPS traffic through the proxy after setup.`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device serial number"),
			),
			mcp.WithNumber("port",
				mcp.Description("Proxy port (default: 8080)"),
			),
		),
		s.handleProxyDeviceSetup,
	)

	// proxy_device_cleanup - Remove proxy from device
	s.server.AddTool(
		mcp.NewTool("proxy_device_cleanup",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Remove proxy configuration from an Android device.

This performs:
1. Removes the adb reverse tunnel
2. Clears the device HTTP proxy settings
3. Clears the proxy device ID`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device serial number"),
			),
			mcp.WithNumber("port",
				mcp.Description("Proxy port used during setup (default: 8080)"),
			),
		),
		s.handleProxyDeviceCleanup,
	)

	// proxy_cert_install - Push CA certificate to device
	s.server.AddTool(
		mcp.NewTool("proxy_cert_install",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Push the proxy CA certificate to an Android device for HTTPS interception.

Pushes the CA certificate to /sdcard/Download/Gaze-CA.crt.
After pushing, the user must install it on the device:
  Settings > Security > Install from storage > Select Gaze-CA.crt`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device serial number"),
			),
		),
		s.handleProxyCertInstall,
	)

	// proxy_cert_trust_check - Check certificate trust status
	s.server.AddTool(
		mcp.NewTool("proxy_cert_trust_check",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Check if a device trusts the proxy CA certificate.

Returns "trusted", "untrusted", or "unknown" based on whether
recent HTTPS traffic has been successfully decrypted.`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device serial number"),
			),
		),
		s.handleProxyCertTrustCheck,
	)

	// breakpoint_rule_add - Add a breakpoint rule
	s.server.AddTool(
		mcp.NewTool("breakpoint_rule_add",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Add a new breakpoint rule for intercepting HTTP requests/responses.

When the proxy intercepts a request or response matching the rule, it pauses execution
and waits for manual resolution (forward, drop, or modify). This is useful for debugging
API calls, testing error scenarios, or modifying requests/responses on the fly.

URL patterns support * wildcard (same as mock rules):
  - "*/api/users*" matches any URL containing /api/users
  - "https://example.com/api/*" matches paths under /api/
  - "*" matches all URLs

Phase options:
  - "request": pause before the request is sent to the server
  - "response": pause after receiving the response before forwarding to client
  - "both": pause at both request and response phases

Examples:
  Intercept login requests: url_pattern="*/api/login", method="POST", phase="request"
  Intercept all API responses: url_pattern="*/api/*", phase="response"
  Intercept everything: url_pattern="*", phase="both"`),
			mcp.WithString("url_pattern",
				mcp.Required(),
				mcp.Description("URL wildcard pattern to match (e.g., '*/api/users*')"),
			),
			mcp.WithString("method",
				mcp.Description("HTTP method to match (empty = match all). e.g., 'GET', 'POST'"),
			),
			mcp.WithString("phase",
				mcp.Required(),
				mcp.Description("When to intercept: 'request', 'response', or 'both'"),
			),
			mcp.WithString("description",
				mcp.Description("Optional description of what this breakpoint rule does"),
			),
		),
		s.handleBreakpointRuleAdd,
	)

	// breakpoint_rule_remove - Remove a breakpoint rule
	s.server.AddTool(
		mcp.NewTool("breakpoint_rule_remove",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("Remove a breakpoint rule by ID. Any pending breakpoints from this rule will continue to wait for resolution."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the breakpoint rule to remove"),
			),
		),
		s.handleBreakpointRuleRemove,
	)

	// breakpoint_rule_list - List all breakpoint rules
	s.server.AddTool(
		mcp.NewTool("breakpoint_rule_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all breakpoint rules.

Returns all configured breakpoint rules with their URL patterns, methods, phases,
enabled states, and descriptions. Rules are sorted by creation time.`),
		),
		s.handleBreakpointRuleList,
	)

	// breakpoint_rule_toggle - Enable/disable a breakpoint rule
	s.server.AddTool(
		mcp.NewTool("breakpoint_rule_toggle",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Enable or disable a breakpoint rule without removing it. Disabled rules will not intercept any requests."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the breakpoint rule to toggle"),
			),
			mcp.WithBoolean("enabled",
				mcp.Required(),
				mcp.Description("Whether to enable (true) or disable (false) the rule"),
			),
		),
		s.handleBreakpointRuleToggle,
	)

	// breakpoint_resolve - Resolve a pending breakpoint
	s.server.AddTool(
		mcp.NewTool("breakpoint_resolve",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Resolve a pending (paused) breakpoint with an action.

Actions:
  - "forward": continue with the original (or modified) request/response
  - "drop": abort the request entirely

Optional modifications (JSON object) can be provided to alter the request or response before forwarding:
  Request phase: {"method":"POST", "url":"...", "headers":{"Key":"Value"}, "body":"..."}
  Response phase: {"statusCode":200, "respHeaders":{"Key":"Value"}, "respBody":"..."}

Example:
  Forward with modified header: action="forward", modifications_json='{"headers":{"Authorization":"Bearer new-token"}}'
  Drop request: action="drop"`),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the pending breakpoint to resolve"),
			),
			mcp.WithString("action",
				mcp.Required(),
				mcp.Description("Action to take: 'forward' or 'drop'"),
			),
			mcp.WithString("modifications_json",
				mcp.Description(`Optional JSON object with modifications to apply before forwarding.
Request phase keys: "method", "url", "headers" (object), "body"
Response phase keys: "statusCode" (number), "respHeaders" (object), "respBody"`),
			),
		),
		s.handleBreakpointResolve,
	)

	// breakpoint_pending_list - List pending breakpoints
	s.server.AddTool(
		mcp.NewTool("breakpoint_pending_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all pending (paused) breakpoints waiting for resolution.

Returns details about each paused request/response including the method, URL, headers,
body, and for response-phase breakpoints also the status code and response body.
Each pending breakpoint has an ID that can be used with breakpoint_resolve.

Pending breakpoints will auto-forward after 120 seconds if not resolved.
Maximum 20 concurrent pending breakpoints are allowed.`),
		),
		s.handleBreakpointPendingList,
	)

	// breakpoint_rule_update - Update an existing breakpoint rule
	s.server.AddTool(
		mcp.NewTool("breakpoint_rule_update",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Update an existing breakpoint rule. All fields are replaced with the new values."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the breakpoint rule to update"),
			),
			mcp.WithString("url_pattern",
				mcp.Required(),
				mcp.Description("URL wildcard pattern to match (e.g., '*/api/users*')"),
			),
			mcp.WithString("phase",
				mcp.Required(),
				mcp.Description("When to intercept: 'request', 'response', or 'both'"),
			),
			mcp.WithString("method",
				mcp.Description("HTTP method to match (empty = match all). e.g., 'GET', 'POST'"),
			),
			mcp.WithBoolean("enabled",
				mcp.Description("Whether this rule is active (default: true)"),
			),
			mcp.WithString("description",
				mcp.Description("Optional description of what this breakpoint rule does"),
			),
		),
		s.handleBreakpointRuleUpdate,
	)

	// breakpoint_forward_all - Forward all pending breakpoints
	s.server.AddTool(
		mcp.NewTool("breakpoint_forward_all",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Forward all pending (paused) breakpoints immediately.

Resolves every currently pending breakpoint with the "forward" action,
allowing all paused requests/responses to continue to their destination.
This is equivalent to clicking "Forward All" in the UI.`),
		),
		s.handleBreakpointForwardAll,
	)

	// --- Map Remote Rules ---

	// map_remote_add - Add a map remote rule
	s.server.AddTool(
		mcp.NewTool("map_remote_add",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Add a new URL redirect (Map Remote) rule.

When the proxy intercepts a request matching the source URL pattern, it rewrites
the URL to the target before forwarding. This is useful for redirecting API calls
to a different server, staging environment, or local development server.

URL patterns support * wildcard:
  - "*/api/v1/*" matches any URL containing /api/v1/
  - "https://prod.example.com/*" matches all paths on prod
  - "*" matches all URLs

The target URL can also contain * as a placeholder for the matched wildcard portion.

Examples:
  Redirect API to staging: source="https://prod.example.com/api/*", target="https://staging.example.com/api/*"
  Redirect to localhost:   source="*/api/*", target="http://localhost:3000/api/*"`),
			mcp.WithString("source_pattern",
				mcp.Required(),
				mcp.Description("Source URL wildcard pattern to match (e.g., '*/api/v1/*')"),
			),
			mcp.WithString("target_url",
				mcp.Required(),
				mcp.Description("Target URL to redirect to (e.g., 'http://localhost:3000/api/*')"),
			),
			mcp.WithString("method",
				mcp.Description("HTTP method to match (empty = match all). e.g., 'GET', 'POST'"),
			),
			mcp.WithString("description",
				mcp.Description("Optional description of what this rule does"),
			),
		),
		s.handleMapRemoteAdd,
	)

	// map_remote_update - Update a map remote rule
	s.server.AddTool(
		mcp.NewTool("map_remote_update",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Update an existing map remote rule. All fields are replaced with the new values."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the map remote rule to update"),
			),
			mcp.WithString("source_pattern",
				mcp.Required(),
				mcp.Description("Source URL wildcard pattern to match"),
			),
			mcp.WithString("target_url",
				mcp.Required(),
				mcp.Description("Target URL to redirect to"),
			),
			mcp.WithString("method",
				mcp.Description("HTTP method to match (empty = match all)"),
			),
			mcp.WithBoolean("enabled",
				mcp.Description("Whether this rule is active (default: true)"),
			),
			mcp.WithString("description",
				mcp.Description("Optional description"),
			),
		),
		s.handleMapRemoteUpdate,
	)

	// map_remote_remove - Remove a map remote rule
	s.server.AddTool(
		mcp.NewTool("map_remote_remove",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("Remove a map remote rule by ID."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the map remote rule to remove"),
			),
		),
		s.handleMapRemoteRemove,
	)

	// map_remote_list - List all map remote rules
	s.server.AddTool(
		mcp.NewTool("map_remote_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all URL redirect (Map Remote) rules.

Returns all configured map remote rules with their source patterns, target URLs,
methods, enabled states, and descriptions. Rules are sorted by creation time.`),
		),
		s.handleMapRemoteList,
	)

	// map_remote_toggle - Enable/disable a map remote rule
	s.server.AddTool(
		mcp.NewTool("map_remote_toggle",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Enable or disable a map remote rule without removing it."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the map remote rule to toggle"),
			),
			mcp.WithBoolean("enabled",
				mcp.Required(),
				mcp.Description("Whether to enable (true) or disable (false) the rule"),
			),
		),
		s.handleMapRemoteToggle,
	)

	// --- Auto Rewrite Rules ---

	// rewrite_rule_add - Add a rewrite rule
	s.server.AddTool(
		mcp.NewTool("rewrite_rule_add",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Add a new auto-rewrite rule for modifying request/response headers or body.

When the proxy intercepts a request/response matching the URL pattern,
it applies regex find-and-replace on the specified target (header or body)
without requiring manual breakpoint intervention.

URL patterns support * wildcard (same as mock rules).

Phase options:
  - "request": rewrite before sending to server
  - "response": rewrite before forwarding to client
  - "both": rewrite at both phases

Target options:
  - "header": rewrite a specific header value (specify header_name)
  - "body": rewrite the request/response body content

Examples:
  Remove server header: url_pattern="*", phase="response", target="header", header_name="Server", match=".*", replace=""
  Replace API key in body: url_pattern="*/api/*", phase="request", target="body", match="old-key-123", replace="new-key-456"
  Modify JSON field: url_pattern="*/api/users*", phase="response", target="body", match="\"role\":\"guest\"", replace="\"role\":\"admin\""`),
			mcp.WithString("url_pattern",
				mcp.Required(),
				mcp.Description("URL wildcard pattern to match (e.g., '*/api/*')"),
			),
			mcp.WithString("phase",
				mcp.Required(),
				mcp.Description("When to rewrite: 'request', 'response', or 'both'"),
			),
			mcp.WithString("target",
				mcp.Required(),
				mcp.Description("What to rewrite: 'header' or 'body'"),
			),
			mcp.WithString("match",
				mcp.Required(),
				mcp.Description("Regex pattern to find (e.g., 'old-value')"),
			),
			mcp.WithString("replace",
				mcp.Required(),
				mcp.Description("Replacement string (supports $1, $2 for capture groups)"),
			),
			mcp.WithString("method",
				mcp.Description("HTTP method to match (empty = match all)"),
			),
			mcp.WithString("header_name",
				mcp.Description("Header name to rewrite (required when target is 'header')"),
			),
			mcp.WithString("description",
				mcp.Description("Optional description of what this rewrite rule does"),
			),
		),
		s.handleRewriteRuleAdd,
	)

	// rewrite_rule_update - Update a rewrite rule
	s.server.AddTool(
		mcp.NewTool("rewrite_rule_update",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Update an existing rewrite rule. All fields are replaced with the new values."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the rewrite rule to update"),
			),
			mcp.WithString("url_pattern",
				mcp.Required(),
				mcp.Description("URL wildcard pattern to match"),
			),
			mcp.WithString("phase",
				mcp.Required(),
				mcp.Description("When to rewrite: 'request', 'response', or 'both'"),
			),
			mcp.WithString("target",
				mcp.Required(),
				mcp.Description("What to rewrite: 'header' or 'body'"),
			),
			mcp.WithString("match",
				mcp.Required(),
				mcp.Description("Regex pattern to find"),
			),
			mcp.WithString("replace",
				mcp.Required(),
				mcp.Description("Replacement string"),
			),
			mcp.WithString("method",
				mcp.Description("HTTP method to match (empty = match all)"),
			),
			mcp.WithString("header_name",
				mcp.Description("Header name to rewrite (required when target is 'header')"),
			),
			mcp.WithBoolean("enabled",
				mcp.Description("Whether this rule is active (default: true)"),
			),
			mcp.WithString("description",
				mcp.Description("Optional description"),
			),
		),
		s.handleRewriteRuleUpdate,
	)

	// rewrite_rule_remove - Remove a rewrite rule
	s.server.AddTool(
		mcp.NewTool("rewrite_rule_remove",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("Remove a rewrite rule by ID."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the rewrite rule to remove"),
			),
		),
		s.handleRewriteRuleRemove,
	)

	// rewrite_rule_list - List all rewrite rules
	s.server.AddTool(
		mcp.NewTool("rewrite_rule_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all auto-rewrite rules.

Returns all configured rewrite rules with their URL patterns, phases, targets,
match/replace patterns, enabled states, and descriptions.`),
		),
		s.handleRewriteRuleList,
	)

	// rewrite_rule_toggle - Enable/disable a rewrite rule
	s.server.AddTool(
		mcp.NewTool("rewrite_rule_toggle",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Enable or disable a rewrite rule without removing it."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the rewrite rule to toggle"),
			),
			mcp.WithBoolean("enabled",
				mcp.Required(),
				mcp.Description("Whether to enable (true) or disable (false) the rule"),
			),
		),
		s.handleRewriteRuleToggle,
	)

	// resend_request - Resend an HTTP request
	s.server.AddTool(
		mcp.NewTool("resend_request",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription(`Send an HTTP request with optional modifications. Useful for testing APIs.

Mock rules are checked first — if a matching mock rule exists, the mock response
is returned without making a real network request.

Examples:
  GET request:  method="GET", url="https://api.example.com/users"
  POST request: method="POST", url="https://api.example.com/login",
                headers_json='{"Content-Type":"application/json"}',
                body='{"username":"test","password":"pass"}'`),
			mcp.WithString("method",
				mcp.Required(),
				mcp.Description("HTTP method (GET, POST, PUT, DELETE, PATCH, etc.)"),
			),
			mcp.WithString("url",
				mcp.Required(),
				mcp.Description("Full URL to send the request to"),
			),
			mcp.WithString("headers_json",
				mcp.Description("Request headers as JSON object, e.g., '{\"Content-Type\":\"application/json\"}'"),
			),
			mcp.WithString("body",
				mcp.Description("Request body content"),
			),
		),
		s.handleResendRequest,
	)
}

// Tool handlers

func (s *MCPServer) handleProxyStart(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	port := 8080
	if p, ok := args["port"].(float64); ok {
		port = int(p)
	}

	result, err := s.app.StartProxy(port)
	if err != nil {
		return nil, fmt.Errorf("failed to start proxy: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Proxy started on port %d\n%s", port, result)),
		},
	}, nil
}

func (s *MCPServer) handleProxyStop(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	result, err := s.app.StopProxy()
	if err != nil {
		return nil, fmt.Errorf("failed to stop proxy: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Proxy stopped\n%s", result)),
		},
	}, nil
}

func (s *MCPServer) handleProxyStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	running := s.app.GetProxyStatus()
	status := "stopped"
	if running {
		status = "running"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Proxy status: %s", status)),
		},
	}, nil
}

// --- Mock Rule Handlers ---

func (s *MCPServer) handleMockRuleList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rules := s.app.GetMockRules()

	if len(rules) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("No mock rules configured.")},
		}, nil
	}

	data, _ := json.MarshalIndent(rules, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Mock rules (%d):\n%s", len(rules), string(data)))},
	}, nil
}

func (s *MCPServer) handleMockRuleAdd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	urlPattern, _ := args["url_pattern"].(string)
	if urlPattern == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: url_pattern is required")}, IsError: true}, nil
	}
	statusCode := 200
	if sc, ok := args["status_code"].(float64); ok {
		statusCode = int(sc)
	}

	method, _ := args["method"].(string)
	body, _ := args["body"].(string)
	bodyFile, _ := args["body_file"].(string)
	description, _ := args["description"].(string)
	delay := 0
	if d, ok := args["delay"].(float64); ok {
		delay = int(d)
	}

	var headers map[string]string
	if hj, ok := args["headers_json"].(string); ok && hj != "" {
		if err := json.Unmarshal([]byte(hj), &headers); err != nil {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing headers_json: %v", err))}, IsError: true}, nil
		}
	}

	var conditions []MCPMockCondition
	if cj, ok := args["conditions_json"].(string); ok && cj != "" {
		if err := json.Unmarshal([]byte(cj), &conditions); err != nil {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing conditions_json: %v", err))}, IsError: true}, nil
		}
	}

	id := s.app.AddMockRule(urlPattern, method, statusCode, headers, body, bodyFile, delay, description, conditions)

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Mock rule added successfully.\nID: %s\nPattern: %s\nStatus: %d", id, urlPattern, statusCode))},
	}, nil
}

func (s *MCPServer) handleMockRuleUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}
	urlPattern, _ := args["url_pattern"].(string)
	if urlPattern == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: url_pattern is required")}, IsError: true}, nil
	}
	statusCode := 200
	if sc, ok := args["status_code"].(float64); ok {
		statusCode = int(sc)
	}

	method, _ := args["method"].(string)
	body, _ := args["body"].(string)
	bodyFile, _ := args["body_file"].(string)
	description, _ := args["description"].(string)
	delay := 0
	if d, ok := args["delay"].(float64); ok {
		delay = int(d)
	}
	enabled := true
	if e, ok := args["enabled"].(bool); ok {
		enabled = e
	}

	var headers map[string]string
	if hj, ok := args["headers_json"].(string); ok && hj != "" {
		if err := json.Unmarshal([]byte(hj), &headers); err != nil {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing headers_json: %v", err))}, IsError: true}, nil
		}
	}

	var conditions []MCPMockCondition
	if cj, ok := args["conditions_json"].(string); ok && cj != "" {
		if err := json.Unmarshal([]byte(cj), &conditions); err != nil {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing conditions_json: %v", err))}, IsError: true}, nil
		}
	}

	if err := s.app.UpdateMockRule(id, urlPattern, method, statusCode, headers, body, bodyFile, delay, enabled, description, conditions); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Mock rule updated: %s", id))},
	}, nil
}

func (s *MCPServer) handleMockRuleRemove(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}

	s.app.RemoveMockRule(id)

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Mock rule removed: %s", id))},
	}, nil
}

func (s *MCPServer) handleMockRuleToggle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}
	enabled, _ := args["enabled"].(bool)

	if err := s.app.ToggleMockRule(id, enabled); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}

	state := "disabled"
	if enabled {
		state = "enabled"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Mock rule %s: %s", state, id))},
	}, nil
}

func (s *MCPServer) handleMockRuleExport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	jsonStr, err := s.app.ExportMockRules()
	if err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(jsonStr)},
	}, nil
}

func (s *MCPServer) handleMockRuleImport(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	jsonStr, _ := args["json"].(string)
	if jsonStr == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: json is required")}, IsError: true}, nil
	}

	count, err := s.app.ImportMockRules(jsonStr)
	if err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Successfully imported %d mock rules", count))},
	}, nil
}

// --- Breakpoint Rule Handlers ---

func (s *MCPServer) handleBreakpointRuleAdd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	urlPattern, _ := args["url_pattern"].(string)
	if urlPattern == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: url_pattern is required")}, IsError: true}, nil
	}
	phase, _ := args["phase"].(string)
	if phase == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: phase is required (request, response, or both)")}, IsError: true}, nil
	}
	if phase != "request" && phase != "response" && phase != "both" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: phase must be 'request', 'response', or 'both'")}, IsError: true}, nil
	}

	method, _ := args["method"].(string)
	description, _ := args["description"].(string)

	id := s.app.AddBreakpointRule(urlPattern, method, phase, description)

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Breakpoint rule added successfully.\nID: %s\nPattern: %s\nPhase: %s", id, urlPattern, phase))},
	}, nil
}

func (s *MCPServer) handleBreakpointRuleRemove(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}

	s.app.RemoveBreakpointRule(id)

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Breakpoint rule removed: %s", id))},
	}, nil
}

func (s *MCPServer) handleBreakpointRuleList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rules := s.app.GetBreakpointRules()

	if len(rules) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("No breakpoint rules configured.")},
		}, nil
	}

	data, _ := json.MarshalIndent(rules, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Breakpoint rules (%d):\n%s", len(rules), string(data)))},
	}, nil
}

func (s *MCPServer) handleBreakpointRuleToggle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}
	enabled, _ := args["enabled"].(bool)

	if err := s.app.ToggleBreakpointRule(id, enabled); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}

	state := "disabled"
	if enabled {
		state = "enabled"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Breakpoint rule %s: %s", state, id))},
	}, nil
}

func (s *MCPServer) handleBreakpointResolve(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}
	action, _ := args["action"].(string)
	if action == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: action is required (forward or drop)")}, IsError: true}, nil
	}
	if action != "forward" && action != "drop" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: action must be 'forward' or 'drop'")}, IsError: true}, nil
	}

	var modifications map[string]interface{}
	if mj, ok := args["modifications_json"].(string); ok && mj != "" {
		if err := json.Unmarshal([]byte(mj), &modifications); err != nil {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing modifications_json: %v", err))}, IsError: true}, nil
		}
	}

	if err := s.app.ResolveBreakpoint(id, action, modifications); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}

	msg := fmt.Sprintf("Breakpoint resolved: %s (action: %s)", id, action)
	if modifications != nil {
		modData, _ := json.Marshal(modifications)
		msg += fmt.Sprintf("\nModifications: %s", string(modData))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(msg)},
	}, nil
}

func (s *MCPServer) handleBreakpointPendingList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pending := s.app.GetPendingBreakpoints()

	if len(pending) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("No pending breakpoints.")},
		}, nil
	}

	data, _ := json.MarshalIndent(pending, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Pending breakpoints (%d):\n%s", len(pending), string(data)))},
	}, nil
}

func (s *MCPServer) handleBreakpointRuleUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}
	urlPattern, _ := args["url_pattern"].(string)
	if urlPattern == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: url_pattern is required")}, IsError: true}, nil
	}
	phase, _ := args["phase"].(string)
	if phase == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: phase is required (request, response, or both)")}, IsError: true}, nil
	}
	if phase != "request" && phase != "response" && phase != "both" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: phase must be 'request', 'response', or 'both'")}, IsError: true}, nil
	}

	method, _ := args["method"].(string)
	description, _ := args["description"].(string)
	enabled := true
	if e, ok := args["enabled"].(bool); ok {
		enabled = e
	}

	if err := s.app.UpdateBreakpointRule(id, urlPattern, method, phase, enabled, description); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Breakpoint rule updated: %s\nPattern: %s\nPhase: %s\nEnabled: %v", id, urlPattern, phase, enabled))},
	}, nil
}

func (s *MCPServer) handleBreakpointForwardAll(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pending := s.app.GetPendingBreakpoints()
	count := len(pending)

	s.app.ForwardAllBreakpoints()

	if count == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("No pending breakpoints to forward.")},
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Forwarded %d pending breakpoint(s).", count))},
	}, nil
}

// --- Map Remote Rule Handlers ---

func (s *MCPServer) handleMapRemoteAdd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	sourcePattern, _ := args["source_pattern"].(string)
	if sourcePattern == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: source_pattern is required")}, IsError: true}, nil
	}
	targetURL, _ := args["target_url"].(string)
	if targetURL == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: target_url is required")}, IsError: true}, nil
	}
	method, _ := args["method"].(string)
	description, _ := args["description"].(string)

	id := s.app.AddMapRemoteRule(sourcePattern, targetURL, method, description)

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Map remote rule added successfully.\nID: %s\nSource: %s\nTarget: %s", id, sourcePattern, targetURL))},
	}, nil
}

func (s *MCPServer) handleMapRemoteUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}
	sourcePattern, _ := args["source_pattern"].(string)
	if sourcePattern == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: source_pattern is required")}, IsError: true}, nil
	}
	targetURL, _ := args["target_url"].(string)
	if targetURL == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: target_url is required")}, IsError: true}, nil
	}
	method, _ := args["method"].(string)
	description, _ := args["description"].(string)
	enabled := true
	if e, ok := args["enabled"].(bool); ok {
		enabled = e
	}

	if err := s.app.UpdateMapRemoteRule(id, sourcePattern, targetURL, method, enabled, description); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Map remote rule updated: %s\nSource: %s → Target: %s", id, sourcePattern, targetURL))},
	}, nil
}

func (s *MCPServer) handleMapRemoteRemove(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}

	s.app.RemoveMapRemoteRule(id)

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Map remote rule removed: %s", id))},
	}, nil
}

func (s *MCPServer) handleMapRemoteList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rules := s.app.GetMapRemoteRules()

	if len(rules) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("No map remote rules configured.")},
		}, nil
	}

	data, _ := json.MarshalIndent(rules, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Map remote rules (%d):\n%s", len(rules), string(data)))},
	}, nil
}

func (s *MCPServer) handleMapRemoteToggle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}
	enabled, _ := args["enabled"].(bool)

	if err := s.app.ToggleMapRemoteRule(id, enabled); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}

	state := "disabled"
	if enabled {
		state = "enabled"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Map remote rule %s: %s", state, id))},
	}, nil
}

// === Rewrite Rule Handlers ===

func (s *MCPServer) handleRewriteRuleAdd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	urlPattern, _ := args["url_pattern"].(string)
	if urlPattern == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: url_pattern is required")}, IsError: true}, nil
	}
	phase, _ := args["phase"].(string)
	if phase == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: phase is required")}, IsError: true}, nil
	}
	target, _ := args["target"].(string)
	if target == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: target is required")}, IsError: true}, nil
	}
	match, _ := args["match"].(string)
	if match == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: match is required")}, IsError: true}, nil
	}
	replace, _ := args["replace"].(string)
	method, _ := args["method"].(string)
	headerName, _ := args["header_name"].(string)
	description, _ := args["description"].(string)

	id := s.app.AddRewriteRule(urlPattern, method, phase, target, headerName, match, replace, description)

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Rewrite rule added.\nID: %s\nPattern: %s\nPhase: %s\nTarget: %s\nMatch: %s\nReplace: %s", id, urlPattern, phase, target, match, replace))},
	}, nil
}

func (s *MCPServer) handleRewriteRuleUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}
	urlPattern, _ := args["url_pattern"].(string)
	if urlPattern == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: url_pattern is required")}, IsError: true}, nil
	}
	phase, _ := args["phase"].(string)
	if phase == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: phase is required")}, IsError: true}, nil
	}
	target, _ := args["target"].(string)
	if target == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: target is required")}, IsError: true}, nil
	}
	match, _ := args["match"].(string)
	if match == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: match is required")}, IsError: true}, nil
	}
	replace, _ := args["replace"].(string)
	method, _ := args["method"].(string)
	headerName, _ := args["header_name"].(string)
	description, _ := args["description"].(string)
	enabled := true
	if e, ok := args["enabled"].(bool); ok {
		enabled = e
	}

	if err := s.app.UpdateRewriteRule(id, urlPattern, method, phase, target, headerName, match, replace, enabled, description); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Rewrite rule updated: %s", id))},
	}, nil
}

func (s *MCPServer) handleRewriteRuleRemove(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}
	s.app.RemoveRewriteRule(id)
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Rewrite rule removed: %s", id))},
	}, nil
}

func (s *MCPServer) handleRewriteRuleList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	rules := s.app.GetRewriteRules()
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(data))},
	}, nil
}

func (s *MCPServer) handleRewriteRuleToggle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)
	if id == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: id is required")}, IsError: true}, nil
	}
	enabled, _ := args["enabled"].(bool)
	if err := s.app.ToggleRewriteRule(id, enabled); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))}, IsError: true}, nil
	}
	state := "disabled"
	if enabled {
		state = "enabled"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Rewrite rule %s: %s", state, id))},
	}, nil
}

func (s *MCPServer) handleResendRequest(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	method, _ := args["method"].(string)
	if method == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: method is required")}, IsError: true}, nil
	}
	url, _ := args["url"].(string)
	if url == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: url is required")}, IsError: true}, nil
	}
	body, _ := args["body"].(string)

	var headers map[string]string
	if hj, ok := args["headers_json"].(string); ok && hj != "" {
		if err := json.Unmarshal([]byte(hj), &headers); err != nil {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing headers_json: %v", err))}, IsError: true}, nil
		}
	}

	result, err := s.app.ResendRequest(method, url, headers, body)
	if err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Request failed: %v", err))}, IsError: true}, nil
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(data))},
	}, nil
}

// --- Proxy Configuration Handlers ---

func (s *MCPServer) handleProxyConfigure(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	var changes []string

	if mitm, ok := args["mitm"].(bool); ok {
		s.app.SetProxyMITM(mitm)
		changes = append(changes, fmt.Sprintf("MITM: %v", mitm))
	}
	if ws, ok := args["ws_enabled"].(bool); ok {
		s.app.SetProxyWSEnabled(ws)
		changes = append(changes, fmt.Sprintf("WebSocket: %v", ws))
	}

	ulSet, dlSet := false, false
	ul, dl := 0, 0
	if u, ok := args["upload_speed"].(float64); ok {
		ul = int(u)
		ulSet = true
	}
	if d, ok := args["download_speed"].(float64); ok {
		dl = int(d)
		dlSet = true
	}
	if ulSet || dlSet {
		s.app.SetProxyLimit(ul, dl)
		changes = append(changes, fmt.Sprintf("Speed limit: UL=%dKB/s DL=%dKB/s", ul, dl))
	}

	if lat, ok := args["latency"].(float64); ok {
		s.app.SetProxyLatency(int(lat))
		changes = append(changes, fmt.Sprintf("Latency: %dms", int(lat)))
	}

	if bp, ok := args["bypass_patterns"].(string); ok && bp != "" {
		var patterns []string
		if err := json.Unmarshal([]byte(bp), &patterns); err != nil {
			return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing bypass_patterns: %v", err))}, IsError: true}, nil
		}
		s.app.SetMITMBypassPatterns(patterns)
		changes = append(changes, fmt.Sprintf("Bypass patterns: %v", patterns))
	}

	if len(changes) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("No settings changed. Provide at least one parameter.")},
		}, nil
	}

	msg := "Proxy settings updated:\n"
	for _, c := range changes {
		msg += "  - " + c + "\n"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(msg)},
	}, nil
}

func (s *MCPServer) handleProxySettings(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	settings := s.app.GetProxySettings()
	settings["running"] = s.app.GetProxyStatus()
	settings["proxyDevice"] = s.app.GetProxyDevice()
	settings["bypassPatterns"] = s.app.GetMITMBypassPatterns()

	data, _ := json.MarshalIndent(settings, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(data))},
	}, nil
}

func (s *MCPServer) handleProxyDeviceSetup(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceId, _ := args["device_id"].(string)
	if deviceId == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")}, IsError: true}, nil
	}
	port := 8080
	if p, ok := args["port"].(float64); ok {
		port = int(p)
	}

	s.app.SetProxyDevice(deviceId)
	if err := s.app.SetupProxyForDevice(deviceId, port); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error setting up proxy on device: %v", err))}, IsError: true}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Proxy configured on device %s (port %d).\nadb reverse tunnel created and HTTP proxy settings applied.", deviceId, port))},
	}, nil
}

func (s *MCPServer) handleProxyDeviceCleanup(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceId, _ := args["device_id"].(string)
	if deviceId == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")}, IsError: true}, nil
	}
	port := 8080
	if p, ok := args["port"].(float64); ok {
		port = int(p)
	}

	if err := s.app.CleanupProxyForDevice(deviceId, port); err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error cleaning up proxy: %v", err))}, IsError: true}, nil
	}
	s.app.SetProxyDevice("")

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Proxy removed from device %s. Reverse tunnel and HTTP proxy settings cleared.", deviceId))},
	}, nil
}

func (s *MCPServer) handleProxyCertInstall(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceId, _ := args["device_id"].(string)
	if deviceId == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")}, IsError: true}, nil
	}

	result, err := s.app.InstallProxyCert(deviceId)
	if err != nil {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error pushing certificate: %v", err))}, IsError: true}, nil
	}

	msg := fmt.Sprintf("CA certificate pushed to device %s.\nPath: /sdcard/Download/Gaze-CA.crt\n%s\n\nNext steps: Install the certificate on the device via Settings > Security > Install from storage.", deviceId, result)
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(msg)},
	}, nil
}

func (s *MCPServer) handleProxyCertTrustCheck(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceId, _ := args["device_id"].(string)
	if deviceId == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{mcp.NewTextContent("Error: device_id is required")}, IsError: true}, nil
	}

	status := s.app.CheckCertTrust(deviceId)
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Certificate trust status for %s: %s", deviceId, status))},
	}, nil
}
