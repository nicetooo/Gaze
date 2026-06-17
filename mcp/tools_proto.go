package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerProtoTools registers protobuf management tools
func (s *MCPServer) registerProtoTools() {
	// proto_file_list - List all loaded .proto files
	s.server.AddTool(
		mcp.NewTool("proto_file_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all loaded .proto schema files.

Returns an array of proto file entries with id, name, content, and loadedAt timestamp.
Use this to see which protobuf schemas are currently available for decoding network traffic.`),
		),
		s.handleProtoFileList,
	)

	// proto_file_add - Add a .proto file
	s.server.AddTool(
		mcp.NewTool("proto_file_add",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Add a .proto schema file for protobuf decoding.

The file will be compiled and its message types become available for decoding
intercepted network traffic. Supports standard proto3 syntax.

Example content:
  syntax = "proto3";
  package myapp;
  message UserResponse {
    int32 id = 1;
    string name = 2;
    repeated string tags = 3;
  }`),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("Filename for the proto definition (e.g. 'user.proto')"),
			),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("The .proto file content (proto3 syntax)"),
			),
		),
		s.handleProtoFileAdd,
	)

	// proto_file_update - Update an existing .proto file
	s.server.AddTool(
		mcp.NewTool("proto_file_update",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Update an existing .proto file's name and/or content. The file will be recompiled."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the proto file to update"),
			),
			mcp.WithString("name",
				mcp.Required(),
				mcp.Description("New filename"),
			),
			mcp.WithString("content",
				mcp.Required(),
				mcp.Description("New .proto file content"),
			),
		),
		s.handleProtoFileUpdate,
	)

	// proto_file_remove - Remove a .proto file
	s.server.AddTool(
		mcp.NewTool("proto_file_remove",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("Remove a .proto schema file. Its message types will no longer be available for decoding."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the proto file to remove"),
			),
		),
		s.handleProtoFileRemove,
	)

	// proto_mapping_list - List all URL→message mappings
	s.server.AddTool(
		mcp.NewTool("proto_mapping_list",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all URL-to-protobuf-message-type mappings.

Mappings tell the proxy which protobuf message type to use when decoding
request/response bodies for matching URLs. Without mappings, protobuf data
is decoded using raw field numbers only.`),
		),
		s.handleProtoMappingList,
	)

	// proto_mapping_add - Add a URL→message mapping
	s.server.AddTool(
		mcp.NewTool("proto_mapping_add",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Add a URL-to-protobuf-message-type mapping.

When the proxy intercepts a request matching the URL pattern, it will use
the specified message type to decode the protobuf body into named fields
instead of raw field numbers.

URL pattern supports wildcards: * matches any characters.
Examples:
  *api.example.com/v1/users*  → matches any request to this endpoint
  */grpc/UserService/*        → matches gRPC service calls

Direction controls which part of the request to decode:
  "response" (default) - decode response body only
  "request"  - decode request body only
  "both"     - decode both request and response bodies`),
			mcp.WithString("urlPattern",
				mcp.Required(),
				mcp.Description("URL wildcard pattern to match (e.g. '*api.example.com/v1/users*')"),
			),
			mcp.WithString("messageType",
				mcp.Required(),
				mcp.Description("Full protobuf message type name (e.g. 'myapp.UserResponse')"),
			),
			mcp.WithString("direction",
				mcp.Description("Which body to decode: 'request', 'response' (default), or 'both'"),
			),
			mcp.WithString("description",
				mcp.Description("Optional description of what this mapping does"),
			),
		),
		s.handleProtoMappingAdd,
	)

	// proto_mapping_update - Update an existing mapping
	s.server.AddTool(
		mcp.NewTool("proto_mapping_update",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Update an existing URL→message type mapping."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the mapping to update"),
			),
			mcp.WithString("urlPattern",
				mcp.Required(),
				mcp.Description("New URL wildcard pattern"),
			),
			mcp.WithString("messageType",
				mcp.Required(),
				mcp.Description("New protobuf message type name"),
			),
			mcp.WithString("direction",
				mcp.Description("Which body to decode: 'request', 'response', or 'both'"),
			),
			mcp.WithString("description",
				mcp.Description("Optional description"),
			),
		),
		s.handleProtoMappingUpdate,
	)

	// proto_mapping_remove - Remove a mapping
	s.server.AddTool(
		mcp.NewTool("proto_mapping_remove",
			mcp.WithDestructiveHintAnnotation(true),
			mcp.WithDescription("Remove a URL→message type mapping."),
			mcp.WithString("id",
				mcp.Required(),
				mcp.Description("ID of the mapping to remove"),
			),
		),
		s.handleProtoMappingRemove,
	)

	// proto_message_types - List available message types
	s.server.AddTool(
		mcp.NewTool("proto_message_types",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`List all available protobuf message types from compiled .proto files.

Returns an alphabetically sorted list of fully-qualified message type names
(e.g. 'myapp.UserResponse'). These names can be used when creating URL mappings.`),
		),
		s.handleProtoMessageTypes,
	)

	// proto_load_url - Load .proto file from URL with dependency resolution
	s.server.AddTool(
		mcp.NewTool("proto_load_url",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription(`Download and load a .proto file from a URL.

Automatically resolves and downloads all import dependencies recursively.
Well-known imports (google/protobuf/*) are handled by the built-in resolver.

Example URLs:
  https://raw.githubusercontent.com/user/repo/main/proto/service.proto
  https://example.com/api/v1/schema.proto

The file and all its dependencies will be compiled and their message types
become available for decoding and URL mapping.`),
			mcp.WithString("url",
				mcp.Required(),
				mcp.Description("URL to fetch the .proto file from"),
			),
		),
		s.handleProtoLoadURL,
	)
}

// Tool handlers

func (s *MCPServer) handleProtoFileList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	files := s.app.GetProtoFiles()

	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Proto files (%d):\n%s", len(files), string(data))),
		},
	}, nil
}

func (s *MCPServer) handleProtoFileAdd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	name, _ := args["name"].(string)
	content, _ := args["content"].(string)

	id, err := s.app.AddProtoFile(name, content)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error adding proto file: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Proto file '%s' added successfully (id: %s)", name, id)),
		},
	}, nil
}

func (s *MCPServer) handleProtoFileUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)
	name, _ := args["name"].(string)
	content, _ := args["content"].(string)

	err := s.app.UpdateProtoFile(id, name, content)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error updating proto file: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Proto file '%s' updated successfully", name)),
		},
	}, nil
}

func (s *MCPServer) handleProtoFileRemove(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)

	err := s.app.RemoveProtoFile(id)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error removing proto file: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent("Proto file removed successfully"),
		},
	}, nil
}

func (s *MCPServer) handleProtoMappingList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mappings := s.app.GetProtoMappings()

	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Proto mappings (%d):\n%s", len(mappings), string(data))),
		},
	}, nil
}

func (s *MCPServer) handleProtoMappingAdd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	urlPattern, _ := args["urlPattern"].(string)
	messageType, _ := args["messageType"].(string)
	direction, _ := args["direction"].(string)
	description, _ := args["description"].(string)

	id, err := s.app.AddProtoMapping(urlPattern, messageType, direction, description)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error adding mapping: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Proto mapping added (id: %s): %s → %s", id, urlPattern, messageType)),
		},
	}, nil
}

func (s *MCPServer) handleProtoMappingUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)
	urlPattern, _ := args["urlPattern"].(string)
	messageType, _ := args["messageType"].(string)
	direction, _ := args["direction"].(string)
	description, _ := args["description"].(string)

	err := s.app.UpdateProtoMapping(id, urlPattern, messageType, direction, description)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error updating mapping: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Proto mapping updated: %s → %s", urlPattern, messageType)),
		},
	}, nil
}

func (s *MCPServer) handleProtoMappingRemove(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	id, _ := args["id"].(string)

	err := s.app.RemoveProtoMapping(id)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error removing mapping: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent("Proto mapping removed successfully"),
		},
	}, nil
}

func (s *MCPServer) handleProtoMessageTypes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	types := s.app.GetProtoMessageTypes()

	if len(types) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent("No message types available. Load .proto files first using proto_file_add or proto_load_url."),
			},
		}, nil
	}

	data, err := json.MarshalIndent(types, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Available message types (%d):\n%s", len(types), string(data))),
		},
	}, nil
}

func (s *MCPServer) handleProtoLoadURL(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	url, _ := args["url"].(string)

	ids, err := s.app.LoadProtoFromURL(url)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error loading proto from URL: %v", err))},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Successfully loaded %d proto file(s) from URL.\nFile IDs: %v", len(ids), ids)),
		},
	}, nil
}
