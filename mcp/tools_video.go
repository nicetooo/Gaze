package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// stripDataURLPrefix removes the "data:image/jpeg;base64," prefix from a data URL
func stripDataURLPrefix(dataURL string) string {
	// Handle common data URL prefixes
	prefixes := []string{
		"data:image/jpeg;base64,",
		"data:image/png;base64,",
		"data:image/gif;base64,",
		"data:image/webp;base64,",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(dataURL, prefix) {
			return strings.TrimPrefix(dataURL, prefix)
		}
	}
	// If no known prefix, try generic pattern
	if idx := strings.Index(dataURL, ";base64,"); idx != -1 {
		return dataURL[idx+8:]
	}
	return dataURL
}

// registerVideoTools registers video-related tools
func (s *MCPServer) registerVideoTools() {
	// video_frame - Extract a single frame from a video
	s.server.AddTool(
		mcp.NewTool("video_frame",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Extract a single frame from a video at a specific time. Returns base64-encoded JPEG image."),
			mcp.WithString("video_path",
				mcp.Required(),
				mcp.Description("Path to the video file"),
			),
			mcp.WithNumber("time_ms",
				mcp.Required(),
				mcp.Description("Time position in milliseconds"),
			),
			mcp.WithNumber("width",
				mcp.Description("Output width in pixels (default: 720, preserves aspect ratio)"),
			),
		),
		s.handleVideoFrame,
	)

	// video_metadata - Get video metadata
	s.server.AddTool(
		mcp.NewTool("video_metadata",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Get metadata for a video file (duration, resolution, codec, etc.)"),
			mcp.WithString("video_path",
				mcp.Required(),
				mcp.Description("Path to the video file"),
			),
		),
		s.handleVideoMetadata,
	)

	// session_video_frame - Extract a frame from a session's recording
	s.server.AddTool(
		mcp.NewTool("session_video_frame",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Extract a frame from a session's recording at a specific event time. Automatically adjusts for video offset."),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID"),
			),
			mcp.WithNumber("event_time_ms",
				mcp.Required(),
				mcp.Description("Event time in milliseconds (relative to session start)"),
			),
			mcp.WithNumber("width",
				mcp.Description("Output width in pixels (default: 720)"),
			),
		),
		s.handleSessionVideoFrame,
	)

	// session_video_info - Get session video info
	s.server.AddTool(
		mcp.NewTool("session_video_info",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Get video information for a session (path, duration, metadata)"),
			mcp.WithString("session_id",
				mcp.Required(),
				mcp.Description("Session ID"),
			),
		),
		s.handleSessionVideoInfo,
	)
}

// Tool handlers

func (s *MCPServer) handleVideoFrame(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	videoPath, ok := args["video_path"].(string)
	if !ok || videoPath == "" {
		return nil, fmt.Errorf("video_path is required")
	}

	timeMs, ok := args["time_ms"].(float64)
	if !ok {
		return nil, fmt.Errorf("time_ms is required")
	}

	width := 720
	if w, ok := args["width"].(float64); ok && w > 0 {
		width = int(w)
	}

	base64DataURL, err := s.app.GetVideoFrame(videoPath, int64(timeMs), width)
	if err != nil {
		return nil, fmt.Errorf("failed to extract frame: %w", err)
	}

	// Strip data URL prefix - MCP expects pure base64 data
	base64Data := stripDataURLPrefix(base64DataURL)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Frame extracted at %dms (width: %d)\n\nBase64 data length: %d characters", int64(timeMs), width, len(base64Data))),
			mcp.NewImageContent(base64Data, "image/jpeg"),
		},
	}, nil
}

func (s *MCPServer) handleVideoMetadata(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	videoPath, ok := args["video_path"].(string)
	if !ok || videoPath == "" {
		return nil, fmt.Errorf("video_path is required")
	}

	meta, err := s.app.GetVideoMetadata(videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video metadata: %w", err)
	}

	result := fmt.Sprintf("Video: %s\n\n", videoPath)
	result += fmt.Sprintf("Duration: %.2fs (%dms)\n", meta.Duration, meta.DurationMs)
	result += fmt.Sprintf("Resolution: %dx%d\n", meta.Width, meta.Height)
	result += fmt.Sprintf("Frame Rate: %.2f fps\n", meta.FrameRate)
	result += fmt.Sprintf("Total Frames: %d\n", meta.TotalFrames)
	result += fmt.Sprintf("Codec: %s\n", meta.Codec)
	result += fmt.Sprintf("Bit Rate: %d bps\n", meta.BitRate)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}

func (s *MCPServer) handleSessionVideoFrame(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	sessionID, ok := args["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	eventTimeMs, ok := args["event_time_ms"].(float64)
	if !ok {
		return nil, fmt.Errorf("event_time_ms is required")
	}

	width := 720
	if w, ok := args["width"].(float64); ok && w > 0 {
		width = int(w)
	}

	// Get session video info
	videoInfo, err := s.app.GetSessionVideoInfo(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session video info: %w", err)
	}

	hasVideo, _ := videoInfo["hasVideo"].(bool)
	if !hasVideo {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Session %s has no video recording", sessionID)),
			},
		}, nil
	}

	videoPath, _ := videoInfo["videoPath"].(string)
	if videoPath == "" {
		return nil, fmt.Errorf("video path not found for session")
	}

	// Calculate video time (adjust for video offset)
	videoOffset := int64(0)
	if offset, ok := videoInfo["videoOffset"].(int64); ok {
		videoOffset = offset
	} else if offset, ok := videoInfo["videoOffset"].(float64); ok {
		videoOffset = int64(offset)
	}

	videoTimeMs := int64(eventTimeMs) - videoOffset
	if videoTimeMs < 0 {
		videoTimeMs = 0
	}

	base64DataURL, err := s.app.GetVideoFrame(videoPath, videoTimeMs, width)
	if err != nil {
		return nil, fmt.Errorf("failed to extract frame: %w", err)
	}

	// Strip data URL prefix - MCP expects pure base64 data
	base64Data := stripDataURLPrefix(base64DataURL)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Frame from session %s at event time %dms (video time: %dms)\n\nBase64 data length: %d characters",
				sessionID, int64(eventTimeMs), videoTimeMs, len(base64Data))),
			mcp.NewImageContent(base64Data, "image/jpeg"),
		},
	}, nil
}

func (s *MCPServer) handleSessionVideoInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	sessionID, ok := args["session_id"].(string)
	if !ok || sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}

	videoInfo, err := s.app.GetSessionVideoInfo(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session video info: %w", err)
	}

	hasVideo, _ := videoInfo["hasVideo"].(bool)
	if !hasVideo {
		errMsg := ""
		if e, ok := videoInfo["error"].(string); ok {
			errMsg = fmt.Sprintf(" (%s)", e)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				mcp.NewTextContent(fmt.Sprintf("Session %s has no video recording%s", sessionID, errMsg)),
			},
		}, nil
	}

	result := fmt.Sprintf("Session %s Video Info:\n\n", sessionID)

	if path, ok := videoInfo["videoPath"].(string); ok {
		result += fmt.Sprintf("Path: %s\n", path)
	}

	if offset, ok := videoInfo["videoOffset"].(int64); ok && offset > 0 {
		result += fmt.Sprintf("Video Offset: %dms\n", offset)
	} else if offset, ok := videoInfo["videoOffset"].(float64); ok && offset > 0 {
		result += fmt.Sprintf("Video Offset: %dms\n", int64(offset))
	}

	if meta, ok := videoInfo["metadata"].(map[string]interface{}); ok {
		result += "\nMetadata:\n"
		if d, ok := meta["durationMs"].(float64); ok {
			result += fmt.Sprintf("  Duration: %.2fs\n", d/1000)
		}
		if w, ok := meta["width"].(float64); ok {
			if h, ok := meta["height"].(float64); ok {
				result += fmt.Sprintf("  Resolution: %dx%d\n", int(w), int(h))
			}
		}
		if fps, ok := meta["frameRate"].(float64); ok {
			result += fmt.Sprintf("  Frame Rate: %.2f fps\n", fps)
		}
		if codec, ok := meta["codec"].(string); ok {
			result += fmt.Sprintf("  Codec: %s\n", codec)
		}
	} else if meta, ok := videoInfo["metadata"].(*VideoMetadata); ok {
		result += "\nMetadata:\n"
		result += fmt.Sprintf("  Duration: %.2fs\n", meta.Duration)
		result += fmt.Sprintf("  Resolution: %dx%d\n", meta.Width, meta.Height)
		result += fmt.Sprintf("  Frame Rate: %.2f fps\n", meta.FrameRate)
		result += fmt.Sprintf("  Codec: %s\n", meta.Codec)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(result),
		},
	}, nil
}
