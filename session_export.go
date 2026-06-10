package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ========================================
// Session Export/Import
// ========================================

// GazeExportManifest describes the export archive
type GazeExportManifest struct {
	FormatVersion int    `json:"formatVersion"` // 1
	AppVersion    string `json:"appVersion"`
	ExportTime    int64  `json:"exportTime"` // Unix ms
	SessionID     string `json:"sessionId"`
	SessionName   string `json:"sessionName"`
	EventCount    int    `json:"eventCount"`
	HasVideo      bool   `json:"hasVideo"`
	HasBookmarks  bool   `json:"hasBookmarks"`
}

// ExportSession shows a save dialog and exports a session to a .gaze file
func (a *App) ExportSession(sessionID string) (string, error) {
	// Get session metadata
	session, err := a.eventStore.GetSession(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	// Build default filename
	safeName := strings.ReplaceAll(session.Name, " ", "_")
	safeName = strings.ReplaceAll(safeName, "/", "_")
	if safeName == "" {
		safeName = "session"
	}
	ts := time.UnixMilli(session.StartTime).Format("2006-01-02")
	defaultFilename := fmt.Sprintf("%s_%s.gaze", safeName, ts)

	// Default save directory
	defaultDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(defaultDir, "Downloads")
	if _, err := os.Stat(downloadsDir); err == nil {
		defaultDir = downloadsDir
	}

	// Show save dialog (only for GUI mode)
	if a.ctx == nil || a.mcpMode {
		return "", fmt.Errorf("ExportSession requires GUI mode, use ExportSessionToPath for MCP")
	}

	savePath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultFilename: defaultFilename,
		Title:           "Export Session",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Gaze Session Archive (*.gaze)", Pattern: "*.gaze"},
		},
		DefaultDirectory: defaultDir,
	})
	if err != nil {
		return "", fmt.Errorf("failed to open save dialog: %w", err)
	}
	if savePath == "" {
		return "", nil // User cancelled
	}

	// Ensure .gaze extension
	if !strings.HasSuffix(savePath, ".gaze") {
		savePath += ".gaze"
	}

	return a.exportSessionToFile(session, savePath)
}

// ExportSessionToPath exports a session to a specific path (for MCP)
func (a *App) ExportSessionToPath(sessionID, outputPath string) (string, error) {
	session, err := a.eventStore.GetSession(sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return "", fmt.Errorf("session not found: %s", sessionID)
	}

	// Ensure .gaze extension
	if !strings.HasSuffix(outputPath, ".gaze") {
		outputPath += ".gaze"
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	return a.exportSessionToFile(session, outputPath)
}

// exportSessionToFile performs the actual export to a ZIP archive
func (a *App) exportSessionToFile(session *DeviceSession, outputPath string) (string, error) {
	LogInfo("session_export").Str("sessionId", session.ID).Str("path", outputPath).Msg("Starting session export")

	// Flush any buffered events
	a.eventStore.Flush()

	// Create the ZIP file
	zipFile, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	// 1. Export events with data
	events, err := a.eventStore.ExportSessionEvents(session.ID)
	if err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("failed to export events: %w", err)
	}

	eventsWriter, err := w.Create("events.jsonl")
	if err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("failed to create events entry: %w", err)
	}
	encoder := json.NewEncoder(eventsWriter)
	for _, event := range events {
		if err := encoder.Encode(event); err != nil {
			os.Remove(outputPath)
			return "", fmt.Errorf("failed to encode event: %w", err)
		}
	}

	// 2. Export bookmarks
	bookmarks, err := a.eventStore.GetBookmarks(session.ID)
	if err != nil {
		LogWarn("session_export").Err(err).Msg("Failed to get bookmarks, skipping")
		bookmarks = nil
	}

	if len(bookmarks) > 0 {
		bookmarksWriter, err := w.Create("bookmarks.json")
		if err != nil {
			os.Remove(outputPath)
			return "", fmt.Errorf("failed to create bookmarks entry: %w", err)
		}
		bookmarksJSON, _ := json.Marshal(bookmarks)
		bookmarksWriter.Write(bookmarksJSON)
	}

	// 3. Export session metadata
	sessionWriter, err := w.Create("session.json")
	if err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("failed to create session entry: %w", err)
	}
	sessionJSON, _ := json.MarshalIndent(session, "", "  ")
	sessionWriter.Write(sessionJSON)

	// 4. Include video recording if exists
	hasVideo := false
	if session.VideoPath != "" {
		if info, statErr := os.Stat(session.VideoPath); statErr == nil && info.Size() > 0 {
			videoExt := filepath.Ext(session.VideoPath)
			if videoExt == "" {
				videoExt = ".mp4"
			}
			videoEntry, err := w.Create("recording" + videoExt)
			if err != nil {
				LogWarn("session_export").Err(err).Msg("Failed to create video entry, skipping video")
			} else {
				videoFile, err := os.Open(session.VideoPath)
				if err != nil {
					LogWarn("session_export").Err(err).Msg("Failed to open video file, skipping video")
				} else {
					_, copyErr := io.Copy(videoEntry, videoFile)
					videoFile.Close()
					if copyErr != nil {
						LogWarn("session_export").Err(copyErr).Msg("Failed to copy video data, archive may be incomplete")
					} else {
						hasVideo = true
					}
				}
			}
		}
	}

	// 5. Write manifest (last, so we know final stats)
	manifest := GazeExportManifest{
		FormatVersion: 1,
		AppVersion:    a.GetAppVersion(),
		ExportTime:    time.Now().UnixMilli(),
		SessionID:     session.ID,
		SessionName:   session.Name,
		EventCount:    len(events),
		HasVideo:      hasVideo,
		HasBookmarks:  len(bookmarks) > 0,
	}
	manifestWriter, err := w.Create("manifest.json")
	if err != nil {
		os.Remove(outputPath)
		return "", fmt.Errorf("failed to create manifest entry: %w", err)
	}
	manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
	manifestWriter.Write(manifestJSON)

	LogInfo("session_export").
		Str("sessionId", session.ID).
		Int("eventCount", len(events)).
		Bool("hasVideo", hasVideo).
		Str("path", outputPath).
		Msg("Session exported successfully")

	return outputPath, nil
}

// ImportSession shows an open dialog and imports a .gaze file
func (a *App) ImportSession() (string, error) {
	if a.ctx == nil || a.mcpMode {
		return "", fmt.Errorf("ImportSession requires GUI mode, use ImportSessionFromPath for MCP")
	}

	defaultDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(defaultDir, "Downloads")
	if _, err := os.Stat(downloadsDir); err == nil {
		defaultDir = downloadsDir
	}

	openPath, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Import Session",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Gaze Session Archive (*.gaze)", Pattern: "*.gaze"},
		},
		DefaultDirectory: defaultDir,
	})
	if err != nil {
		return "", fmt.Errorf("failed to open file dialog: %w", err)
	}
	if openPath == "" {
		return "", nil // User cancelled
	}

	return a.importSessionFromFile(openPath)
}

// ImportSessionFromPath imports from a specific path (for MCP)
func (a *App) ImportSessionFromPath(inputPath string) (string, error) {
	if _, err := os.Stat(inputPath); err != nil {
		return "", fmt.Errorf("file not found: %s", inputPath)
	}
	return a.importSessionFromFile(inputPath)
}

// importSessionFromFile reads a .gaze ZIP and imports all data
func (a *App) importSessionFromFile(inputPath string) (string, error) {
	LogInfo("session_import").Str("path", inputPath).Msg("Starting session import")

	r, err := zip.OpenReader(inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to open archive: %w", err)
	}
	defer r.Close()

	// Read all files from ZIP
	var (
		manifestData []byte
		sessionData  []byte
		eventsData   []byte
		bookmarkData []byte
		videoFile    *zip.File
	)

	for _, f := range r.File {
		switch {
		case f.Name == "manifest.json":
			manifestData, err = readZipFile(f)
			if err != nil {
				return "", fmt.Errorf("failed to read manifest: %w", err)
			}
		case f.Name == "session.json":
			sessionData, err = readZipFile(f)
			if err != nil {
				return "", fmt.Errorf("failed to read session: %w", err)
			}
		case f.Name == "events.jsonl":
			eventsData, err = readZipFile(f)
			if err != nil {
				return "", fmt.Errorf("failed to read events: %w", err)
			}
		case f.Name == "bookmarks.json":
			bookmarkData, err = readZipFile(f)
			if err != nil {
				return "", fmt.Errorf("failed to read bookmarks: %w", err)
			}
		case strings.HasPrefix(f.Name, "recording"):
			videoFile = f
		}
	}

	// Validate required data
	if sessionData == nil {
		return "", fmt.Errorf("invalid .gaze archive: missing session.json")
	}

	// Parse manifest (optional, for validation)
	if manifestData != nil {
		var manifest GazeExportManifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			LogWarn("session_import").Err(err).Msg("Failed to parse manifest, continuing anyway")
		} else {
			LogInfo("session_import").
				Int("formatVersion", manifest.FormatVersion).
				Str("appVersion", manifest.AppVersion).
				Int("eventCount", manifest.EventCount).
				Msg("Archive manifest")
		}
	}

	// Parse session
	var session DeviceSession
	if err := json.Unmarshal(sessionData, &session); err != nil {
		return "", fmt.Errorf("failed to parse session: %w", err)
	}

	// Generate new session ID to avoid conflicts
	oldID := session.ID
	session.ID = uuid.New().String()[:8]
	session.Status = "completed" // Imported sessions are always completed
	session.Name = session.Name + " (imported)"

	// Parse events
	var events []UnifiedEvent
	if eventsData != nil {
		lines := strings.Split(strings.TrimSpace(string(eventsData)), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var event UnifiedEvent
			if err := json.Unmarshal([]byte(line), &event); err != nil {
				LogWarn("session_import").Err(err).Str("line", line[:min(100, len(line))]).Msg("Failed to parse event line, skipping")
				continue
			}
			// Generate new event ID and update session ID to avoid conflicts
			event.ID = uuid.New().String()
			event.SessionID = session.ID
			events = append(events, event)
		}
	}

	// Parse bookmarks
	var bookmarks []Bookmark
	if bookmarkData != nil {
		if err := json.Unmarshal(bookmarkData, &bookmarks); err != nil {
			LogWarn("session_import").Err(err).Msg("Failed to parse bookmarks, skipping")
		} else {
			// Generate new bookmark IDs and update session ID references
			for i := range bookmarks {
				bookmarks[i].ID = uuid.New().String()
				bookmarks[i].SessionID = session.ID
			}
		}
	}

	// Handle video file
	if videoFile != nil {
		videoExt := filepath.Ext(videoFile.Name)
		if videoExt == "" {
			videoExt = ".mp4"
		}

		// Save to recordings directory
		recordDir := a.GetRecordingsDir()
		os.MkdirAll(recordDir, 0755)

		timestamp := time.Now().Format("2006-01-02_15-04-05")
		videoPath := filepath.Join(recordDir, fmt.Sprintf("imported_%s_%s%s", timestamp, session.ID[:8], videoExt))

		if err := extractZipFile(videoFile, videoPath); err != nil {
			LogWarn("session_import").Err(err).Msg("Failed to extract video, skipping")
		} else {
			session.VideoPath = videoPath
			LogInfo("session_import").Str("videoPath", videoPath).Msg("Video extracted")
		}
	}

	// Import into database
	if err := a.eventStore.ImportSession(&session, events, bookmarks); err != nil {
		return "", fmt.Errorf("failed to import session: %w", err)
	}

	LogInfo("session_import").
		Str("oldId", oldID).
		Str("newId", session.ID).
		Int("eventCount", len(events)).
		Int("bookmarkCount", len(bookmarks)).
		Msg("Session imported successfully")

	// Emit frontend event to refresh session list
	if a.ctx != nil && !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "session-imported", map[string]interface{}{
			"sessionId":  session.ID,
			"name":       session.Name,
			"eventCount": len(events),
		})
	}

	return session.ID, nil
}

// readZipFile reads the entire content of a zip entry
func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// extractZipFile extracts a zip entry to a file on disk
func extractZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, rc)
	return err
}

// ShowInFolder opens the system file manager and highlights the given file
func (a *App) ShowInFolder(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("empty file path")
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", "-R", filePath).Start()
	case "windows":
		return exec.Command("explorer", "/select,", filePath).Start()
	case "linux":
		// Try xdg-open on the parent directory
		return exec.Command("xdg-open", filepath.Dir(filePath)).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
