package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerPluginTools registers plugin management tools
func (s *MCPServer) registerPluginTools() {
	// plugin_list - List all plugins
	s.server.AddTool(
		mcp.NewTool("plugin_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all saved plugins.

Returns an array of plugin metadata including ID, name, version, author, description, 
enabled status, filters, and timestamps.

EXAMPLE:
  # List all plugins
  plugin_list

RETURNS: Array of plugin metadata objects`),
		),
		s.handlePluginList,
	)

	// plugin_get - Get plugin details
	s.server.AddTool(
		mcp.NewTool("plugin_get",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Get detailed information about a specific plugin including source code.

Returns complete plugin data:
- Metadata (name, version, filters, config)
- Source code (TypeScript or JavaScript)
- Compiled code (JavaScript for execution)
- Language type

EXAMPLE:
  plugin_get plugin_id="tracking-validator"

RETURNS: Full plugin object with source code`),
			mcp.WithString("plugin_id",
				mcp.Required(),
				mcp.Description("Plugin ID to retrieve"),
			),
		),
		s.handlePluginGet,
	)

	// plugin_create - Create a new plugin
	s.server.AddTool(
		mcp.NewTool("plugin_create",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Create a new plugin from source code.

PLUGIN API SPECIFICATION:

Your plugin code must export a 'plugin' object with TypeScript type annotations:

  const plugin: Plugin = {
    metadata: {
      name: "string",        // Display name (optional, overridden by 'name' parameter)
      version: "string",     // Semantic version (optional)
      description: "string"  // What this plugin does (optional)
    },
    
    onInit: (context: PluginContext): void => {
      // Optional: Called once when plugin is loaded
      // Use context.state to initialize persistent state across events
      // Example: context.state.counter = 0;
    },
    
    onEvent: (event: UnifiedEvent, context: PluginContext): PluginResult | null => {
      // Required: Called for each matching event
      // Return a PluginResult object or null/undefined
      // Example:
      //   return {
      //     derivedEvents: [{ source: "plugin", type: "alert", level: "warn", title: "Issue found" }]
      //   };
    },
    
    onDestroy: (context: PluginContext): void => {
      // Optional: Called when plugin is unloaded
      // Use for cleanup if needed
    }
  };

EVENT OBJECT (passed to onEvent):
  {
    id: "string",               // Unique event ID
    deviceId: "string",         // Device ID
    sessionId: "string",        // Session ID
    timestamp: number,          // Unix milliseconds
    relativeTime: number,       // Milliseconds from session start
    source: "string",           // Event source: "network", "logcat", "app", "device", etc.
    category: "string",         // Category: "network", "log", "state", etc.
    type: "string",             // Event type: "network_request", "logcat", "app_crash", etc.
    level: "string",            // Level: "info", "warn", "error", "debug", "verbose", "fatal"
    title: "string",            // Short description
    content?: "string",         // Optional detailed content
    data?: any                  // Event-specific data as JSON object
  }

CONTEXT OBJECT (passed to onEvent and onInit):
  {
    config: any,                // User configuration from config_json parameter
    state: any,                 // Persistent state object (survives across events)
    
    // Helper functions:
    log(message: string): void,              // Write debug log
    emit(type, title, data): void,           // Emit a derived event (shortcut)
    jsonPath(json, path): any,               // Extract JSON value by path (e.g., "user.name")
    matchURL(url, pattern): boolean          // URL wildcard matching (* supported)
  }

PLUGIN RESULT (return value from onEvent):
  {
    derivedEvents?: [           // Array of new events to generate
      {
        source: "string",       // Event source (typically "plugin")
        type: "string",         // Event type (e.g., "validation_error", "alert")
        level: "string",        // "info", "warn", or "error"
        title: "string",        // Short description
        content?: "string",     // Optional detailed content
        data?: any              // Optional data object
      }
    ],
    tags?: ["string"],          // Tags to add to the original event
    metadata?: { key: value }   // Metadata to attach to the original event
  }

COMPLETE EXAMPLE - API Tracking Validator (TypeScript):

  const plugin: Plugin = {
    metadata: {
      name: "API Tracking Validator",
      version: "1.0.0",
      description: "Validates tracking API calls have required parameters"
    },
    
    onEvent: (event: UnifiedEvent, context: PluginContext): PluginResult | null => {
      // Extract URL from event data
      const data = event.data || {};
      const url = data.url || "";
      
      if (!url) return null;
      
      try {
        const urlObj = new URL(url);
        const params = urlObj.searchParams;
        
        // Get required parameters from config
        const required: string[] = context.config.requiredParams || ["user_id", "event_name"];
        const missing: string[] = [];
        
        for (const param of required) {
          if (!params.has(param)) {
            missing.push(param);
          }
        }
        
        // If parameters are missing, emit a warning event
        if (missing.length > 0) {
          return {
            derivedEvents: [{
              source: "plugin",
              type: "validation_error",
              level: "warn",
              title: "Missing tracking parameters: " + missing.join(", "),
              data: {
                url: url,
                missingParams: missing,
                foundParams: Array.from(params.keys())
              }
            }]
          };
        }
      } catch (err: any) {
        context.log("Failed to parse URL: " + err.message);
      }
      
      return null;
    }
  };

PARAMETERS:
  id: Unique plugin ID (lowercase, numbers, hyphens only, e.g., "tracking-validator")
  name: Display name (e.g., "Tracking Parameter Validator")
  version: Semantic version (default: "1.0.0")
  author: Plugin author name (optional)
  description: Plugin description (optional)
  source_code: TypeScript source code with type annotations
  language: Ignored (system always uses TypeScript)
  compiled_code: JavaScript code transpiled from source_code
  
FILTERS (JSON string):
  All filters use AND logic (event must match all specified filters).
  Empty arrays match all values for that field. Multiple values use OR logic within the field.
  
  {
    "sources": ["network", "logcat"],   // Event sources (OR logic)
    "types": ["network_request"],       // Event types (OR logic)
    "levels": ["error", "warn"],        // Event levels (OR logic)
    "urlPattern": "*/api/track*",       // URL wildcard (only for network events, * = wildcard)
    "titleMatch": "ActivityManager.*"   // Title regex pattern (optional)
  }
  
CONFIG (JSON string):
  User-defined configuration passed to context.config in plugin code.
  Example: {"requiredParams": ["user_id", "event_name"], "timeout": 5000}

USAGE EXAMPLE:
  # 1. Create the plugin (TypeScript source code with types)
  plugin_create \
    id="tracking-validator" \
    name="API Tracking Validator" \
    version="1.0.0" \
    source_code='const plugin: Plugin = { ... (see complete example above) ... }' \
    compiled_code='const plugin = { ... (JavaScript transpiled from TypeScript) ... }' \
    filters_json='{"sources": ["network"], "urlPattern": "*/api/track*"}' \
    config_json='{"requiredParams": ["user_id", "event_name"]}'

  # 2. Test against a real event before deploying
  plugin_test \
    script='const plugin: Plugin = { ... }' \
    event_id="abc123-network-event-id"

  # 3. Enable the plugin
  plugin_toggle plugin_id="tracking-validator" enabled=true

RETURNS: Success message with plugin ID`),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Unique plugin ID (e.g., 'tracking-validator')"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Plugin display name"),
			),
			mcp.WithString("version",
				mcp.Description("Plugin version (default: '1.0.0')"),
			),
			mcp.WithString("author",
				mcp.Description("Plugin author name (optional)"),
			),
			mcp.WithString("description",
				mcp.Description("Plugin description (optional)"),
			),
			mcp.WithString("source_code",
				mcp.Required(),
				mcp.Description("TypeScript source code (shown in editor with type checking)"),
			),
			mcp.WithString("language",
				mcp.Description("Ignored (system always uses TypeScript, kept for compatibility)"),
			),
			mcp.WithString("compiled_code",
				mcp.Required(),
				mcp.Description("JavaScript code transpiled from source_code"),
			),
			mcp.WithString("filters_json",
				mcp.Description("JSON string of event filters (e.g., '{\"sources\": [\"network\"]}')"),
			),
			mcp.WithString("config_json",
				mcp.Description("JSON string of plugin configuration (e.g., '{\"timeout\": 5000}')"),
			),
			mcp.WithBoolean("enabled",
				mcp.Description("Whether the plugin starts enabled (default: true)"),
			),
		),
		s.handlePluginCreate,
	)

	// plugin_update - Update an existing plugin
	s.server.AddTool(
		mcp.NewTool("plugin_update",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Update an existing plugin. All fields are replaced with new values.

WORKFLOW:
  1. Get current plugin: plugin_get plugin_id="xxx"
  2. Modify the fields you want to change
  3. Call plugin_update with ALL required fields (id, name, source_code, compiled_code)
  4. Optional fields will use defaults if not provided

IMPORTANT: This is a full replacement, not a merge. All fields must be provided.

PARAMETERS EXPLAINED:
  
  id (required):
    Plugin ID to update. Must match an existing plugin.
  
  name (required):
    Display name shown in UI (e.g., "API Tracking Validator v2")
  
  source_code (required):
    Plugin source code in TypeScript with type annotations.
    This is what users see and edit in the plugin editor.
    Must use TypeScript syntax: 'const plugin: Plugin = { onEvent: (e, ctx) => { ... } }'
  
  compiled_code (required):
    Executable JavaScript code transpiled from source_code.
    Use TypeScript compiler to convert source_code to JavaScript.
    This separation allows showing TypeScript in editor while running JS.
    
  language (optional, ignored):
    This parameter is ignored. System always uses TypeScript.
    Kept for backward compatibility only.
  
  version (optional):
    Semantic version (e.g., "2.0.0")
    Default: "1.0.0"
  
  author (optional):
    Plugin author name (shown in plugin list)
  
  description (optional):
    What this plugin does (shown as tooltip in UI)
  
  filters_json (optional):
    JSON string defining which events this plugin processes.
    Structure: {
      "sources": ["network", "logcat"],    // Event sources (OR logic)
      "types": ["network_request"],        // Event types (OR logic)
      "levels": ["error", "warn"],         // Event levels (OR logic)
      "urlPattern": "*/api/track*",        // URL wildcard (network events only)
      "titleMatch": "ActivityManager.*"    // Title regex pattern
    }
    All filters use AND logic (event must match all specified filters).
    Empty arrays match all values. Multiple values within a field use OR logic.
  
  config_json (optional):
    JSON object passed to plugin as context.config
    Use this for user-configurable parameters.
    Example: {"requiredParams": ["user_id"], "timeout": 5000}
    Access in plugin: context.config.requiredParams

  enabled (optional):
    Enable or disable the plugin. If omitted, the current enabled
    state is preserved (a disabled plugin stays disabled after update).

EXAMPLE:
  # Get current plugin
  plugin_get plugin_id="tracking-validator"
  
  # Update with new version and code
  plugin_update \
    id="tracking-validator" \
    name="Tracking Validator v2" \
    version="2.0.0" \
    source_code='const plugin: Plugin = { ... TypeScript code ... }' \
    compiled_code='const plugin = { ... JavaScript compiled code ... }' \
    filters_json='{"sources": ["network"], "urlPattern": "*/api/*"}' \
    config_json='{"requiredParams": ["user_id", "session_id"]}'

RETURNS: Success message with plugin ID`),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("Plugin ID to update (must exist)"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Display name for UI (e.g., 'API Validator v2')"),
			),
			mcp.WithString("version",
				mcp.Description("Semantic version (default: '1.0.0')"),
			),
			mcp.WithString("author",
				mcp.Description("Author name (optional)"),
			),
			mcp.WithString("description",
				mcp.Description("What this plugin does (shown as tooltip)"),
			),
			mcp.WithString("source_code",
				mcp.Required(),
				mcp.Description("TypeScript source code (shown in editor with type checking)"),
			),
			mcp.WithString("language",
				mcp.Description("Ignored (system always uses TypeScript, kept for compatibility)"),
			),
			mcp.WithString("compiled_code",
				mcp.Required(),
				mcp.Description("JavaScript code transpiled from source_code"),
			),
			mcp.WithString("filters_json",
				mcp.Description("Event filters JSON: {\"sources\": [...], \"types\": [...], \"urlPattern\": \"*...*\"}"),
			),
			mcp.WithString("config_json",
				mcp.Description("Plugin config JSON (passed to context.config): {\"key\": \"value\"}"),
			),
			mcp.WithBoolean("enabled",
				mcp.Description("Enable/disable the plugin. Omit to keep the current enabled state"),
			),
		),
		s.handlePluginUpdate,
	)

	// plugin_delete - Delete a plugin
	s.server.AddTool(
		mcp.NewTool("plugin_delete",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription(`Delete a plugin permanently.

The plugin will be unloaded from memory and removed from the database.
This action cannot be undone.

EXAMPLE:
  plugin_delete plugin_id="old-plugin"

RETURNS: Success message`),
			mcp.WithString("plugin_id",
				mcp.Required(),
				mcp.Description("Plugin ID to delete"),
			),
		),
		s.handlePluginDelete,
	)

	// plugin_toggle - Enable/disable a plugin
	s.server.AddTool(
		mcp.NewTool("plugin_toggle",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Enable or disable a plugin without deleting it.

Disabled plugins will not process any events but remain in the database.

EXAMPLE:
  plugin_toggle plugin_id="tracking-validator" enabled=true
  plugin_toggle plugin_id="old-plugin" enabled=false

RETURNS: Success message`),
			mcp.WithString("plugin_id",
				mcp.Required(),
				mcp.Description("Plugin ID to toggle"),
			),
			mcp.WithBoolean("enabled",
				mcp.Required(),
				mcp.Description("Enable (true) or disable (false) the plugin"),
			),
		),
		s.handlePluginToggle,
	)

	// plugin_test_detailed - Test plugin and return detailed results
	s.server.AddTool(
		mcp.NewTool("plugin_test_detailed",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Test plugin code against a specific event and return detailed results.

Similar to plugin_test but returns comprehensive information including:
- Execution status (success/failure)
- Generated derived events
- Execution time in milliseconds
- Captured logs from context.log() calls
- Error messages with stack traces
- Whether the event matched the plugin's filters
- Snapshot of the test event

This is useful for debugging and understanding exactly what your plugin does.

EXAMPLE:
  plugin_test_detailed \
    script='const plugin: Plugin = { ... }' \
    event_id="abc123-event-id"

RETURNS:
  {
    "success": true,
    "derivedEvents": [...],
    "executionTimeMs": 15,
    "logs": ["[info] Processing network request", "[info] URL matched pattern"],
    "matchedFilters": true,
    "eventSnapshot": {...}
  }`),
			mcp.WithString("script",
				mcp.Required(),
				mcp.Description("Plugin JavaScript code to test"),
			),
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("Event ID to test against (from session_events)"),
			),
		),
		s.handlePluginTestDetailed,
	)

	// plugin_test_custom - Test plugin with custom event data
	s.server.AddTool(
		mcp.NewTool("plugin_test_custom",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Test plugin code against custom event data without needing a real event in the database.

This is useful for:
- Testing plugins before you have real event data
- Creating reproducible test cases
- Testing edge cases and error handling
- Rapid prototyping without device connection

You can provide a JSON object representing a UnifiedEvent. The system will auto-fill
missing required fields (id, timestamp, deviceId, sessionId).

MINIMAL EVENT DATA:
  {
    "source": "network",
    "type": "network_request",
    "level": "info",
    "title": "GET /api/users",
    "data": {
      "url": "https://api.example.com/users?page=1",
      "method": "GET",
      "statusCode": 200
    }
  }

FULL EVENT DATA EXAMPLE:
  {
    "id": "test-event-1",
    "deviceId": "test-device",
    "sessionId": "test-session",
    "timestamp": 1704067200000,
    "relativeTime": 1000,
    "source": "network",
    "category": "network",
    "type": "network_request",
    "level": "info",
    "title": "POST /api/tracking",
    "data": {
      "url": "https://api.example.com/tracking?event=click&user_id=123",
      "method": "POST",
      "statusCode": 200,
      "requestHeaders": {"Content-Type": "application/json"},
      "requestBody": "{\"action\":\"click\"}",
      "responseBody": "{\"success\":true}"
    }
  }

EXAMPLE:
  plugin_test_custom \
    script='const plugin: Plugin = { onEvent: (e, ctx) => { ... } }' \
    event_data='{"source":"network","type":"network_request","title":"Test","data":{"url":"https://example.com"}}'

RETURNS: Same detailed result as plugin_test_detailed`),
			mcp.WithString("script",
				mcp.Required(),
				mcp.Description("Plugin JavaScript code to test"),
			),
			mcp.WithString("event_data",
				mcp.Required(),
				mcp.Description("JSON string of event data (UnifiedEvent structure)"),
			),
		),
		s.handlePluginTestCustom,
	)

	// plugin_test_batch - Test plugin against multiple events
	s.server.AddTool(
		mcp.NewTool("plugin_test_batch",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Test plugin code against multiple events at once.

This is useful for:
- Validating plugin behavior across different event types
- Ensuring robustness (no errors on unexpected data)
- Performance testing (finding slow operations)
- Batch validation before deploying to production

The tool will test the plugin against each event and return an array of results.
Maximum 50 events can be tested at once.

EXAMPLE:
  # First, find some events to test
  session_events session_id="session-abc" sources="network" limit=10
  
  # Extract event IDs and test
  plugin_test_batch \
    script='const plugin: Plugin = { ... }' \
    event_ids='["event-id-1","event-id-2","event-id-3"]'

RETURNS:
  [
    {
      "success": true,
      "derivedEvents": [...],
      "executionTimeMs": 12,
      "logs": [...],
      "eventSnapshot": {...}
    },
    {
      "success": false,
      "error": "...",
      "executionTimeMs": 5,
      "logs": [...],
      "eventSnapshot": {...}
    }
  ]

TIP: Look for patterns in failures - are certain event types causing errors?`),
			mcp.WithString("script",
				mcp.Required(),
				mcp.Description("Plugin JavaScript code to test"),
			),
			mcp.WithString("event_ids",
				mcp.Required(),
				mcp.Description("JSON array of event IDs to test (max 50)"),
			),
		),
		s.handlePluginTestBatch,
	)

	// plugin_sample_events - Get sample events for testing
	s.server.AddTool(
		mcp.NewTool("plugin_sample_events",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Get sample events from a session for plugin testing.

This tool helps you find suitable events to test your plugin against.
You can filter by source, type, and limit the number of results.

COMMON USE CASE:
  1. Get sample network events: plugin_sample_events session_id="xxx" sources='["network"]' limit=10
  2. Pick an event ID from the results
  3. Test your plugin: plugin_test_detailed script='...' event_id="..."

FILTERS:
  - sources: Array of event sources (e.g., ["network", "logcat", "app"])
  - types: Array of event types (e.g., ["network_request", "websocket_message"])
  - limit: Number of events to return (default: 20, max: 100)

EXAMPLE:
  # Get network events with errors
  plugin_sample_events \
    session_id="session-abc" \
    sources='["network"]' \
    types='["network_request"]' \
    limit=5

RETURNS:
  Array of UnifiedEvent objects (same structure as session_events)`),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID to query events from"),
			),
			mcp.WithString("sources",
				mcp.Description("JSON array of event sources (e.g., [\"network\",\"logcat\"])"),
			),
			mcp.WithString("types",
				mcp.Description("JSON array of event types (e.g., [\"network_request\"])"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Number of events to return (default: 20, max: 100)"),
			),
		),
		s.handlePluginSampleEvents,
	)

	// plugin_test - Test a plugin against a specific event
	s.server.AddTool(
		mcp.NewTool("plugin_test",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Test plugin code against a specific event without saving to database.

This is the primary debugging tool for plugin development. It executes your plugin
code against a real event from your session and shows exactly what derived events
would be generated.

DEBUGGING WORKFLOW:

1. Find a matching event:
   session_events session_id="<id>" sources="network" limit=10
   
2. Copy an event ID from the results

3. Write your plugin code (see plugin_create for API specification)

4. Test it:
   plugin_test script='const plugin: Plugin = { ... }' event_id="<event-id>"

5. Check the output:
   - If empty array: plugin didn't generate any derived events (check your logic)
   - If has events: review the derived event structure
   - If error: fix the JavaScript error and test again

6. Iterate until working correctly

7. Create the plugin:
   plugin_create id="my-plugin" name="My Plugin" source_code='...' compiled_code='...'

ERROR HANDLING:

If your plugin code throws an error, you'll see:
  Error: <JavaScript error message with line number>

Common errors:
- "undefined is not a function": Check function names (onEvent, not onevent)
- "Cannot read property 'x' of undefined": Add null checks for event.data
- "URL is not a constructor": Wrap URL parsing in try-catch

DEBUGGING TIPS:

1. Use context.log() to output debug messages:
   context.log("URL: " + event.data.url);
   
2. Check event.data structure first:
   context.log("Event data: " + JSON.stringify(event.data));
   
3. Always validate data before processing:
   if (!event.data || !event.data.url) return null;

4. Test with multiple events to ensure robustness:
   - Find events with different data structures
   - Test edge cases (missing fields, null values, etc.)

EXAMPLE:
  # Get an event ID from a session
  session_events session_id="session-abc" sources="network" limit=1
  
  # Test plugin against that event (use TypeScript syntax)
  plugin_test \
    script='const plugin: Plugin = {
      onEvent: (event: UnifiedEvent, context: PluginContext): PluginResult | null => {
        context.log("Processing event: " + event.type);
        if (event.source === "network") {
          return {
            derivedEvents: [{
              source: "plugin",
              type: "network_detected",
              level: "info",
              title: "Network event processed"
            }]
          };
        }
        return null;
      }
    }' \
    event_id="abc123-event-id"

RETURNS: 
  On success: JSON array of derived events that would be generated
  On error: Error message with details
  
NOTES:
  - The event must exist in the database (use session_events to find IDs)
  - context.state is available but not persisted (testing is stateless)
  - context.log() output is not returned (only visible in server logs)`),
			mcp.WithString("script",
				mcp.Required(),
				mcp.Description("Plugin JavaScript code to test"),
			),
			mcp.WithString("event_id",
				mcp.Required(),
				mcp.Description("Event ID to test against (from session_events)"),
			),
		),
		s.handlePluginTest,
	)
}

// ========== Handler functions ==========

func (s *MCPServer) handlePluginList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	plugins, err := s.app.ListPlugins()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	jsonData, _ := json.MarshalIndent(plugins, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
	}, nil
}

func (s *MCPServer) handlePluginGet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	pluginID, _ := args["plugin_id"].(string)

	if pluginID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: plugin_id is required")},
			IsError: true,
		}, nil
	}

	plugin, err := s.app.GetPlugin(pluginID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	jsonData, _ := json.MarshalIndent(plugin, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
	}, nil
}

func (s *MCPServer) handlePluginCreate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.handlePluginSave(ctx, request, false)
}

func (s *MCPServer) handlePluginUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return s.handlePluginSave(ctx, request, true)
}

func (s *MCPServer) handlePluginSave(ctx context.Context, request mcp.CallToolRequest, isUpdate bool) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	id, _ := args["id"].(string)
	name, _ := args["name"].(string)
	version, _ := args["version"].(string)
	author, _ := args["author"].(string)
	description, _ := args["description"].(string)
	sourceCode, _ := args["source_code"].(string)
	language, _ := args["language"].(string)
	compiledCode, _ := args["compiled_code"].(string)
	filtersJSON, _ := args["filters_json"].(string)
	configJSON, _ := args["config_json"].(string)

	// enabled 为可选参数：未传时保留现有启用状态（新建默认启用）
	var enabled *bool
	if v, ok := args["enabled"].(bool); ok {
		enabled = &v
	}

	// Validate required fields
	if id == "" || name == "" || sourceCode == "" || compiledCode == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: id, name, source_code, and compiled_code are required")},
			IsError: true,
		}, nil
	}

	// Set defaults
	if version == "" {
		version = "1.0.0"
	}
	if language == "" {
		language = "typescript" // System always uses TypeScript
	}

	// Parse filters
	var filters struct {
		Sources    []string `json:"sources"`
		Types      []string `json:"types"`
		Levels     []string `json:"levels"`
		URLPattern string   `json:"urlPattern"`
		TitleMatch string   `json:"titleMatch"`
	}
	if filtersJSON != "" {
		if err := json.Unmarshal([]byte(filtersJSON), &filters); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing filters_json: %v", err))},
				IsError: true,
			}, nil
		}
	}

	// Parse config
	var config map[string]interface{}
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing config_json: %v", err))},
				IsError: true,
			}, nil
		}
	} else {
		config = make(map[string]interface{})
	}

	// Import PluginSaveRequest and PluginFilters from main package
	// These types need to be accessible via the bridge
	req := struct {
		ID           string                 `json:"id"`
		Name         string                 `json:"name"`
		Version      string                 `json:"version"`
		Author       string                 `json:"author"`
		Description  string                 `json:"description"`
		SourceCode   string                 `json:"sourceCode"`
		Language     string                 `json:"language"`
		CompiledCode string                 `json:"compiledCode"`
		Filters      interface{}            `json:"filters"`
		Config       map[string]interface{} `json:"config"`
		Enabled      *bool                  `json:"enabled,omitempty"`
	}{
		ID:           id,
		Name:         name,
		Version:      version,
		Author:       author,
		Description:  description,
		SourceCode:   sourceCode,
		Language:     language,
		CompiledCode: compiledCode,
		Filters: map[string]interface{}{
			"sources":    filters.Sources,
			"types":      filters.Types,
			"levels":     filters.Levels,
			"urlPattern": filters.URLPattern,
			"titleMatch": filters.TitleMatch,
		},
		Config:  config,
		Enabled: enabled,
	}

	// Convert to JSON and back to get proper type
	reqJSON, _ := json.Marshal(req)
	var saveReq interface{}
	json.Unmarshal(reqJSON, &saveReq)

	// Call bridge method
	err := s.app.SavePlugin(saveReq)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	action := "created"
	if isUpdate {
		action = "updated"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Plugin %s: %s", action, id))},
	}, nil
}

func (s *MCPServer) handlePluginDelete(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	pluginID, _ := args["plugin_id"].(string)

	if pluginID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: plugin_id is required")},
			IsError: true,
		}, nil
	}

	err := s.app.DeletePlugin(pluginID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Plugin deleted: %s", pluginID))},
	}, nil
}

func (s *MCPServer) handlePluginToggle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	pluginID, _ := args["plugin_id"].(string)
	enabled, enabledOk := args["enabled"].(bool)

	if pluginID == "" || !enabledOk {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: plugin_id and enabled are required")},
			IsError: true,
		}, nil
	}

	err := s.app.TogglePlugin(pluginID, enabled)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Plugin %s: %s", action, pluginID))},
	}, nil
}

func (s *MCPServer) handlePluginTest(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	script, _ := args["script"].(string)
	eventID, _ := args["event_id"].(string)

	if script == "" || eventID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: script and event_id are required")},
			IsError: true,
		}, nil
	}

	derivedEvents, err := s.app.TestPlugin(script, eventID)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	jsonData, _ := json.MarshalIndent(derivedEvents, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
	}, nil
}

func (s *MCPServer) handlePluginTestDetailed(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	script, _ := args["script"].(string)
	eventID, _ := args["event_id"].(string)

	if script == "" || eventID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: script and event_id are required")},
			IsError: true,
		}, nil
	}

	result, err := s.app.TestPluginDetailed(script, eventID)
	if err != nil {
		// 即使有错误，如果 result 中已包含错误信息，也返回 result
		if resultInterface, ok := result.(interface{ GetError() string }); ok && resultInterface.GetError() != "" {
			jsonData, _ := json.MarshalIndent(result, "", "  ")
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
			}, nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
	}, nil
}

func (s *MCPServer) handlePluginTestCustom(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	script, _ := args["script"].(string)
	eventData, _ := args["event_data"].(string)

	if script == "" || eventData == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: script and event_data are required")},
			IsError: true,
		}, nil
	}

	result, err := s.app.TestPluginWithEventData(script, eventData)
	if err != nil {
		// 即使有错误，如果 result 中已包含错误信息，也返回 result
		jsonData, _ := json.MarshalIndent(result, "", "  ")
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
		}, nil
	}

	jsonData, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
	}, nil
}

func (s *MCPServer) handlePluginTestBatch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	script, _ := args["script"].(string)
	eventIDsJSON, _ := args["event_ids"].(string)

	if script == "" || eventIDsJSON == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: script and event_ids are required")},
			IsError: true,
		}, nil
	}

	// 解析事件 ID 数组
	var eventIDs []string
	if err := json.Unmarshal([]byte(eventIDsJSON), &eventIDs); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing event_ids: %v", err))},
			IsError: true,
		}, nil
	}

	results, err := s.app.TestPluginBatch(script, eventIDs)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	jsonData, _ := json.MarshalIndent(results, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
	}, nil
}

func (s *MCPServer) handlePluginSampleEvents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	sessionID, _ := args["session_id"].(string)
	sourcesJSON, _ := args["sources"].(string)
	typesJSON, _ := args["types"].(string)
	limit, _ := args["limit"].(float64)

	if sessionID == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent("Error: session_id is required")},
			IsError: true,
		}, nil
	}

	// 解析 sources 数组
	var sources []string
	if sourcesJSON != "" {
		if err := json.Unmarshal([]byte(sourcesJSON), &sources); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing sources: %v", err))},
				IsError: true,
			}, nil
		}
	}

	// 解析 types 数组
	var types []string
	if typesJSON != "" {
		if err := json.Unmarshal([]byte(typesJSON), &types); err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error parsing types: %v", err))},
				IsError: true,
			}, nil
		}
	}

	limitInt := int(limit)
	if limitInt == 0 {
		limitInt = 20
	}

	events, err := s.app.GetSampleEvents(sessionID, sources, types, limitInt)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	jsonData, _ := json.MarshalIndent(events, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.NewTextContent(string(jsonData))},
	}, nil
}
