package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ListFiles returns a list of files in the specified directory on the device
func (a *App) ListFiles(deviceId, pathStr string) ([]FileInfo, error) {
	a.updateLastActive(deviceId)
	if err := ValidateDeviceID(deviceId); err != nil {
		return nil, err
	}

	pathStr = path.Clean("/" + pathStr)
	cmdPath := pathStr
	if cmdPath != "/" {
		cmdPath += "/"
	}

	cmd := a.newAdbCommand(nil, "-s", deviceId, "shell", "ls", "-la", "\""+cmdPath+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w (output: %s)", err, string(output))
	}

	dateTimeRegex := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2})|([A-Z][a-z]{2}\s+\d{1,2}\s+(\d{2}:\d{2}|\d{4}))`)

	lines := strings.Split(string(output), "\n")
	var files []FileInfo

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "total ") {
			continue
		}

		loc := dateTimeRegex.FindStringIndex(line)
		if loc == nil {
			continue
		}

		modTime := line[loc[0]:loc[1]]
		afterDateTime := strings.TrimSpace(line[loc[1]:])
		beforeDateTime := strings.TrimSpace(line[:loc[0]])
		beforeParts := strings.Fields(beforeDateTime)

		if len(beforeParts) < 1 {
			continue
		}

		mode := beforeParts[0]
		isDir := strings.HasPrefix(mode, "d")
		isLink := strings.HasPrefix(mode, "l")

		var size int64
		if len(beforeParts) >= 1 {
			fmt.Sscanf(beforeParts[len(beforeParts)-1], "%d", &size)
		}

		name := afterDateTime
		if isLink {
			arrowIdx := strings.Index(name, " -> ")
			if arrowIdx != -1 {
				name = name[:arrowIdx]
			}
			isDir = true
		}

		cleanName := strings.TrimSpace(name)
		if cleanName == "." || cleanName == ".." || cleanName == "" || cleanName == "?" {
			continue
		}

		if cleanName == path.Base(pathStr) || cleanName == pathStr {
			continue
		}

		files = append(files, FileInfo{
			Name:    cleanName,
			Size:    size,
			Mode:    mode,
			ModTime: modTime,
			IsDir:   isDir,
			Path:    path.Join(pathStr, cleanName),
		})
	}

	return files, nil
}

// DownloadFile pulls a file from the device to a user-selected local path
func (a *App) DownloadFile(deviceId, remotePath string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	fileName := path.Base(remotePath)
	defaultDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(defaultDir, "Downloads")
	if _, err := os.Stat(downloadsDir); err == nil {
		defaultDir = downloadsDir
	}

	savePath, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		DefaultFilename:  fileName,
		Title:            "Download File",
		DefaultDirectory: defaultDir,
	})

	if err != nil {
		return "", err
	}
	if savePath == "" {
		return "", nil
	}

	cmd := a.newAdbCommand(nil, "-s", deviceId, "pull", remotePath, savePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to download file: %w, output: %s", err, string(output))
	}

	return savePath, nil
}

// UploadFile pushes a local file to the device
func (a *App) UploadFile(deviceId, localPath, remotePath string) error {
	LogInfo("app").Str("deviceId", deviceId).Str("localPath", localPath).Str("remotePath", remotePath).Msg("UploadFile called")

	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}

	cmd := a.newAdbCommand(nil, "-s", deviceId, "push", localPath, remotePath)
	output, err := cmd.CombinedOutput()
	LogInfo("app").Str("output", string(output)).Err(err).Msg("UploadFile adb push result")
	if err != nil {
		return fmt.Errorf("failed to upload file: %w, output: %s", err, string(output))
	}

	return nil
}

// DeleteFile deletes a file or directory on the device
func (a *App) DeleteFile(deviceId, pathStr string) error {
	a.updateLastActive(deviceId)
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	pathStr = path.Clean("/" + pathStr)
	cmd := a.newAdbCommand(nil, "-s", deviceId, "shell", "rm", "-rf", "\""+pathStr+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// MoveFile moves or renames a file or directory on the device
func (a *App) MoveFile(deviceId, src, dest string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	src = path.Clean("/" + src)
	dest = path.Clean("/" + dest)
	cmd := a.newAdbCommand(nil, "-s", deviceId, "shell", "mv", "\""+src+"\"", "\""+dest+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// CopyFile copies a file or directory on the device
func (a *App) CopyFile(deviceId, src, dest string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	src = path.Clean("/" + src)
	dest = path.Clean("/" + dest)
	cmd := a.newAdbCommand(nil, "-s", deviceId, "shell", "cp", "-R", "\""+src+"\"", "\""+dest+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// Mkdir creates a new directory on the device
func (a *App) Mkdir(deviceId, pathStr string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}
	pathStr = path.Clean("/" + pathStr)
	cmd := a.newAdbCommand(nil, "-s", deviceId, "shell", "mkdir", "-p", "\""+pathStr+"\"")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, string(output))
	}
	return nil
}

// OpenFileOnHost pulls a file from the device to a temporary location and opens it
func (a *App) OpenFileOnHost(deviceId, remotePath string) error {
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}

	fileName := path.Base(remotePath)
	tmpDir := filepath.Join(os.TempDir(), "adb-gui-open")
	_ = os.MkdirAll(tmpDir, 0755)
	localPath := filepath.Join(tmpDir, fileName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := a.newAdbCommand(ctx, "-s", deviceId, "pull", remotePath, localPath)

	a.openFileMu.Lock()
	a.openFileCmds[remotePath] = cmd
	a.openFileMu.Unlock()

	defer func() {
		a.openFileMu.Lock()
		delete(a.openFileCmds, remotePath)
		a.openFileMu.Unlock()
	}()

	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("open cancelled")
		}
		return fmt.Errorf("failed to pull file: %w, output: %s", err, string(output))
	}

	var openCmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		openCmd = exec.Command("cmd", "/c", "start", "", localPath)
	case "darwin":
		openCmd = exec.Command("open", localPath)
	default:
		openCmd = exec.Command("xdg-open", localPath)
	}

	return openCmd.Start()
}

// CancelOpenFile cancels the pull process for a specific file
func (a *App) CancelOpenFile(remotePath string) {
	a.openFileMu.Lock()
	defer a.openFileMu.Unlock()
	if cmd, exists := a.openFileCmds[remotePath]; exists {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		delete(a.openFileCmds, remotePath)
	}
}

// GetThumbnail returns a base64 encoded thumbnail for an image or video file
func (a *App) GetThumbnail(deviceId, remotePath, modTime string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	ext := strings.ToLower(filepath.Ext(remotePath))
	isImage := ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".webp" || ext == ".gif"
	isVideo := ext == ".mp4" || ext == ".mkv" || ext == ".mov" || ext == ".avi"

	if !isImage && !isVideo {
		return "", fmt.Errorf("unsupported file type")
	}

	configDir, _ := os.UserConfigDir()
	thumbDir := filepath.Join(configDir, "Gaze", "thumbnails")
	_ = os.MkdirAll(thumbDir, 0755)

	cacheKey := fmt.Sprintf("%x", md5.Sum([]byte(deviceId+remotePath+modTime+"v2")))
	cachePath := filepath.Join(thumbDir, cacheKey+".jpg")

	if _, err := os.Stat(cachePath); err == nil {
		data, err := os.ReadFile(cachePath)
		if err == nil {
			return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data), nil
		}
	}

	tmpDir := filepath.Join(os.TempDir(), "adb-gui-thumb")
	_ = os.MkdirAll(tmpDir, 0755)
	localPath := filepath.Join(tmpDir, cacheKey+ext)
	defer os.Remove(localPath)

	pullCmd := a.newAdbCommand(nil, "-s", deviceId, "pull", remotePath, localPath)
	if err := pullCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to pull file: %w", err)
	}

	var thumbData []byte
	var err error

	if isImage {
		thumbData, err = a.generateImageThumbnail(localPath)
	} else if isVideo {
		thumbData, err = a.generateVideoThumbnail(localPath)
	}

	if err != nil {
		return "", err
	}

	_ = os.WriteFile(cachePath, thumbData, 0644)

	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(thumbData), nil
}

func (a *App) generateImageThumbnail(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	targetSize := 512
	scale := 1
	if width > targetSize || height > targetSize {
		if width > height {
			scale = width / targetSize
		} else {
			scale = height / targetSize
		}
	}

	if scale < 1 {
		scale = 1
	}

	newWidth := width / scale
	newHeight := height / scale
	thumb := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			thumb.Set(x, y, img.At(x*scale, y*scale))
		}
	}

	var buf bytes.Buffer
	err = jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 70})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (a *App) generateVideoThumbnail(localPath string) ([]byte, error) {
	ffmpegPath := a.ffmpegPath
	if ffmpegPath == "" {
		// Fall back to PATH lookup if embedded binary is unavailable
		ffmpegPath = "ffmpeg"
	}

	tmpThumb := localPath + ".jpg"
	defer os.Remove(tmpThumb)

	cmd := exec.Command(ffmpegPath, "-y", "-i", localPath, "-ss", "00:00:01", "-vframes", "1", "-s", "512x512", "-f", "image2", tmpThumb)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg not available or failed: %w", err)
	}

	return os.ReadFile(tmpThumb)
}
