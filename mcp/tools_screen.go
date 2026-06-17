package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
)

// registerScreenTools registers screen control tools
func (s *MCPServer) registerScreenTools() {
	// screen_screenshot - Take a screenshot
	s.server.AddTool(
		mcp.NewTool("screen_screenshot",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription(`Take a screenshot of the device screen and return as base64 image.
Optionally includes UI hierarchy XML for element analysis.
Returns: base64 image (JPEG by default) + optional UI hierarchy JSON

SIZE & FORMAT:
- Images are downscaled (bilinear, aspect ratio preserved) if either dimension
  exceeds max_dimension (default: 1568, which matches typical vision model input limits)
- format=jpeg (default) is 5-10x smaller than PNG for UI screenshots; use format=png
  for lossless pixel inspection
- save_path always receives the original full-resolution PNG`),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithBoolean("include_ui",
				mcp.Description("Include UI hierarchy in response (default: false)"),
			),
			mcp.WithString("save_path",
				mcp.Description("Also save the original full-resolution PNG to this path (optional)"),
			),
			mcp.WithString("format",
				mcp.Description("Returned image format: 'jpeg' (default, smaller) or 'png' (lossless)"),
			),
			mcp.WithNumber("quality",
				mcp.Description("JPEG quality 1-100 (default: 80, ignored for png)"),
			),
			mcp.WithNumber("max_dimension",
				mcp.Description("Downscale if width or height exceeds this many pixels (default: 1568)"),
			),
		),
		s.handleScreenshot,
	)

	// screen_record_start - Start recording
	s.server.AddTool(
		mcp.NewTool("screen_record_start",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Start recording the device screen"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
			mcp.WithNumber("max_size",
				mcp.Description("Maximum video dimension (default: 1280)"),
			),
			mcp.WithNumber("bit_rate",
				mcp.Description("Video bit rate in Mbps (default: 8)"),
			),
		),
		s.handleRecordStart,
	)

	// screen_record_stop - Stop recording
	s.server.AddTool(
		mcp.NewTool("screen_record_stop",
			mcp.WithDestructiveHintAnnotation(false),
			mcp.WithDescription("Stop recording the device screen"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
		),
		s.handleRecordStop,
	)

	// screen_recording_status - Check recording status
	s.server.AddTool(
		mcp.NewTool("screen_recording_status",
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
			mcp.WithDescription("Check if device screen is being recorded"),
			mcp.WithString("device_id",
				mcp.Required(),
				mcp.Description("Device ID"),
			),
		),
		s.handleRecordingStatus,
	)
}

// Tool handlers

func (s *MCPServer) handleScreenshot(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	includeUI := false
	if v, ok := args["include_ui"].(bool); ok {
		includeUI = v
	}

	// Generate temp path for screenshot
	tempDir := os.TempDir()
	filename := fmt.Sprintf("screenshot_%s_%s.png", deviceID, time.Now().Format("20060102_150405"))
	tempPath := filepath.Join(tempDir, filename)

	// Take screenshot
	path, err := s.app.TakeScreenshot(deviceID, tempPath)
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	// Ensure temp file is always cleaned up immediately after we're done
	defer os.Remove(path)

	// Read screenshot file (original full-resolution PNG)
	imageData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read screenshot: %w", err)
	}

	// Save the ORIGINAL PNG to user-specified path before any downscaling/transcoding
	savedPath := ""
	if savePath, ok := args["save_path"].(string); ok && savePath != "" {
		if err := os.WriteFile(savePath, imageData, 0644); err == nil {
			savedPath = savePath
		}
	}

	format := "jpeg"
	if f, ok := args["format"].(string); ok && f != "" {
		switch f {
		case "png", "jpeg":
			format = f
		case "jpg":
			format = "jpeg"
		default:
			return nil, fmt.Errorf("format must be 'png' or 'jpeg', got '%s'", f)
		}
	}
	quality := 80
	if q, ok := args["quality"].(float64); ok && q >= 1 && q <= 100 {
		quality = int(q)
	}
	maxDimension := 1568
	if md, ok := args["max_dimension"].(float64); ok && md > 0 {
		maxDimension = int(md)
	}

	mimeType := "image/png"
	if img, _, decErr := image.Decode(bytes.NewReader(imageData)); decErr == nil {
		bounds := img.Bounds()
		w, h := bounds.Dx(), bounds.Dy()
		needResize := w > maxDimension || h > maxDimension
		if needResize {
			var newW, newH int
			if w >= h {
				newW = maxDimension
				newH = int(float64(h) * float64(maxDimension) / float64(w))
			} else {
				newH = maxDimension
				newW = int(float64(w) * float64(maxDimension) / float64(h))
			}
			img = resizeBilinear(img, newW, newH)
		}
		if format == "png" && !needResize {
			// Source is already PNG at target size — skip a pointless re-encode
		} else {
			var buf bytes.Buffer
			var encErr error
			if format == "jpeg" {
				encErr = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
				mimeType = "image/jpeg"
			} else {
				encErr = png.Encode(&buf, img)
			}
			if encErr == nil {
				imageData = buf.Bytes()
			} else {
				mimeType = "image/png" // fall back to the original PNG bytes
			}
		}
	}

	base64Image := base64.StdEncoding.EncodeToString(imageData)

	// Build response content
	contents := []mcp.Content{}

	// Add image content
	contents = append(contents, mcp.NewImageContent(base64Image, mimeType))

	// Build text description
	textInfo := fmt.Sprintf("Screenshot captured for device %s", deviceID)
	if savedPath != "" {
		textInfo += fmt.Sprintf("\nSaved to: %s", savedPath)
	}

	// Include UI hierarchy if requested
	if includeUI {
		hierarchy, err := s.app.GetUIHierarchy(deviceID)
		if err == nil {
			jsonData, err := json.Marshal(hierarchy)
			if err == nil {
				textInfo += fmt.Sprintf("\n\nUI Hierarchy:\n```json\n%s\n```", string(jsonData))
			}
		} else {
			textInfo += fmt.Sprintf("\n\nUI Hierarchy: failed to get (%v)", err)
		}
	}

	contents = append(contents, mcp.NewTextContent(textInfo))

	return &mcp.CallToolResult{
		Content: contents,
	}, nil
}

// resizeBilinear scales src to newW x newH using bilinear interpolation with direct
// pixel buffer access. The previous nearest-neighbor implementation made millions of
// At()/Set() interface calls per screenshot and produced jagged text edges.
func resizeBilinear(src image.Image, newW, newH int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()

	// Normalize to an *image.RGBA anchored at (0,0) for direct Pix indexing
	srcRGBA, ok := src.(*image.RGBA)
	if !ok || b.Min != (image.Point{}) {
		srcRGBA = image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(srcRGBA, srcRGBA.Bounds(), src, b.Min, draw.Src)
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	if w < 2 || h < 2 || newW < 1 || newH < 1 {
		draw.Draw(dst, dst.Bounds(), srcRGBA, image.Point{}, draw.Src)
		return dst
	}

	xRatio := float64(w) / float64(newW)
	yRatio := float64(h) / float64(newH)

	for y := 0; y < newH; y++ {
		sy := (float64(y)+0.5)*yRatio - 0.5
		if sy < 0 {
			sy = 0
		}
		y0 := int(sy)
		if y0 > h-2 {
			y0 = h - 2
		}
		fy := sy - float64(y0)
		row0 := y0 * srcRGBA.Stride
		row1 := (y0 + 1) * srcRGBA.Stride
		dstRow := y * dst.Stride

		for x := 0; x < newW; x++ {
			sx := (float64(x)+0.5)*xRatio - 0.5
			if sx < 0 {
				sx = 0
			}
			x0 := int(sx)
			if x0 > w-2 {
				x0 = w - 2
			}
			fx := sx - float64(x0)

			i00 := row0 + x0*4
			i10 := i00 + 4
			i01 := row1 + x0*4
			i11 := i01 + 4

			w00 := (1 - fx) * (1 - fy)
			w10 := fx * (1 - fy)
			w01 := (1 - fx) * fy
			w11 := fx * fy

			di := dstRow + x*4
			for c := 0; c < 4; c++ {
				v := w00*float64(srcRGBA.Pix[i00+c]) +
					w10*float64(srcRGBA.Pix[i10+c]) +
					w01*float64(srcRGBA.Pix[i01+c]) +
					w11*float64(srcRGBA.Pix[i11+c])
				dst.Pix[di+c] = uint8(v + 0.5)
			}
		}
	}
	return dst
}

func (s *MCPServer) handleRecordStart(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	config := ScrcpyConfig{
		MaxSize: 1280,
		BitRate: 8000000,
		MaxFps:  30,
	}

	if maxSize, ok := args["max_size"].(float64); ok {
		config.MaxSize = int(maxSize)
	}
	if bitRate, ok := args["bit_rate"].(float64); ok {
		config.BitRate = int(bitRate * 1000000) // Convert Mbps to bps
	}

	err := s.app.StartRecording(deviceID, config)
	if err != nil {
		return nil, fmt.Errorf("failed to start recording: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Started recording device %s", deviceID)),
		},
	}, nil
}

func (s *MCPServer) handleRecordStop(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	err := s.app.StopRecording(deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to stop recording: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Stopped recording device %s", deviceID)),
		},
	}, nil
}

func (s *MCPServer) handleRecordingStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := request.GetArguments()
	deviceID, ok := args["device_id"].(string)
	if !ok || deviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	isRecording := s.app.IsRecording(deviceID)
	status := "not recording"
	if isRecording {
		status = "recording"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf("Device %s is %s", deviceID, status)),
		},
	}, nil
}
