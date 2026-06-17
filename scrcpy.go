package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// StartScrcpy starts scrcpy for the given device with custom configuration
func (a *App) StartScrcpy(deviceId string, config ScrcpyConfig) error {
	timer := StartOperation("scrcpy", "start_mirror").AddDetail("device_id", deviceId)

	a.updateLastActive(deviceId)
	if err := ValidateDeviceID(deviceId); err != nil {
		timer.EndWithError(err)
		return err
	}

	LogUserAction(ActionScrcpyStart, deviceId, map[string]interface{}{
		"video_source": config.VideoSource,
		"max_fps":      config.MaxFps,
		"bit_rate":     config.BitRate,
	})

	a.scrcpyMu.Lock()
	if cmd, exists := a.scrcpyCmds[deviceId]; exists && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	a.scrcpyMu.Unlock()

	args := []string{"-s", deviceId}

	if config.MaxSize > 0 {
		args = append(args, "--max-size", fmt.Sprintf("%d", config.MaxSize))
	}
	if config.BitRate > 0 {
		args = append(args, "--video-bit-rate", fmt.Sprintf("%d", config.BitRate))
	}
	if config.MaxFps > 0 {
		args = append(args, "--max-fps", fmt.Sprintf("%d", config.MaxFps))
	}
	isCamera := config.VideoSource == "camera"

	if config.StayAwake && !isCamera {
		args = append(args, "--stay-awake")
	}
	if config.TurnScreenOff && !isCamera {
		args = append(args, "--turn-screen-off")
	}
	if config.VideoCodec != "" {
		args = append(args, "--video-codec", config.VideoCodec)
	}

	if config.NoAudio {
		args = append(args, "--no-audio")
	} else if config.AudioCodec != "" {
		args = append(args, "--audio-codec", config.AudioCodec)
	}

	if config.AlwaysOnTop {
		args = append(args, "--always-on-top")
	}
	if config.ShowTouches && !isCamera {
		args = append(args, "--show-touches")
		go a.RunAdbCommand(deviceId, "shell settings put system show_touches 1")
	} else if !isCamera {
		go a.RunAdbCommand(deviceId, "shell settings put system show_touches 0")
	}
	if config.Fullscreen {
		args = append(args, "--fullscreen")
	}
	if config.ReadOnly {
		args = append(args, "--no-control")
	}
	if config.PowerOffOnClose && !isCamera {
		args = append(args, "--power-off-on-close")
	}
	if config.WindowBorderless {
		args = append(args, "--window-borderless")
	}

	if config.VideoSource == "camera" {
		args = append(args, "--video-source", "camera")
		if config.CameraId != "" {
			args = append(args, "--camera-id", config.CameraId)
		}
		if config.CameraSize != "" {
			args = append(args, "--camera-size", config.CameraSize)
		}
	} else {
		if config.VideoSource == "display" {
			args = append(args, "--video-source", "display")
		}
		if config.DisplayId > 0 {
			args = append(args, "--display-id", fmt.Sprintf("%d", config.DisplayId))
		}
	}
	if config.DisplayOrientation != "" && config.DisplayOrientation != "0" {
		args = append(args, "--display-orientation", config.DisplayOrientation)
	}
	if config.CaptureOrientation != "" && config.CaptureOrientation != "0" {
		args = append(args, "--capture-orientation", config.CaptureOrientation)
	}
	if config.KeyboardMode != "" && config.KeyboardMode != "sdk" {
		args = append(args, "--keyboard", config.KeyboardMode)
	}
	if config.MouseMode != "" && config.MouseMode != "sdk" {
		args = append(args, "--mouse", config.MouseMode)
	}
	if config.NoClipboardSync {
		args = append(args, "--no-clipboard-autosync")
	}
	if config.ShowFps {
		args = append(args, "--print-fps")
	}
	if config.NoPowerOn {
		args = append(args, "--no-power-on")
	}

	if config.VideoSource == "camera" {
		foundNoAudio := false
		for _, arg := range args {
			if arg == "--no-audio" {
				foundNoAudio = true
				break
			}
		}
		if !foundNoAudio {
			args = append(args, "--no-audio")
		}
	}

	args = append(args, "--window-title", "ADB GUI - "+deviceId)

	cmd := a.newScrcpyCommand(args...)

	var stderrBuf bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	a.Log("Starting scrcpy: %s %v", a.scrcpyPath, cmd.Args)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scrcpy: %w", err)
	}

	a.scrcpyMu.Lock()
	a.scrcpyCmds[deviceId] = cmd
	a.scrcpyMu.Unlock()

	timer.End()

	startTime := time.Now()

	if !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "scrcpy-started", map[string]interface{}{
			"deviceId":  deviceId,
			"startTime": startTime.Unix(),
		})
	}

	go func() {
		err := cmd.Wait()
		duration := time.Since(startTime)

		a.scrcpyMu.Lock()
		defer a.scrcpyMu.Unlock()

		if a.scrcpyCmds[deviceId] == cmd {
			delete(a.scrcpyCmds, deviceId)

			if err != nil && duration < 5*time.Second {
				errorMsg := stderrBuf.String()
				if errorMsg == "" {
					errorMsg = err.Error()
				}
				a.Log("Scrcpy failed quickly (%v): %s", duration, errorMsg)
				if !a.mcpMode {
					wailsRuntime.EventsEmit(a.ctx, "scrcpy-failed", map[string]interface{}{
						"deviceId": deviceId,
						"error":    errorMsg,
					})
				}
			} else {
				if !a.mcpMode {
					wailsRuntime.EventsEmit(a.ctx, "scrcpy-stopped", deviceId)
				}
			}
		}
	}()

	return nil
}

// StopScrcpy stops scrcpy for the given device
func (a *App) StopScrcpy(deviceId string) error {
	a.scrcpyMu.Lock()
	defer a.scrcpyMu.Unlock()

	if cmd, exists := a.scrcpyCmds[deviceId]; exists && cmd.Process != nil {
		err := cmd.Process.Kill()
		if err != nil && (strings.Contains(err.Error(), "process already finished") || strings.Contains(err.Error(), "already finished")) {
			return nil
		}
		return err
	}
	return nil
}

// StartRecording starts a separate scrcpy process just for recording without a window
func (a *App) StartRecording(deviceId string, config ScrcpyConfig) error {
	a.updateLastActive(deviceId)
	if deviceId == "" {
		return fmt.Errorf("no device specified")
	}

	if config.RecordPath == "" {
		return fmt.Errorf("no record path specified")
	}

	// 优雅停止已有录制进程, 直接 Kill 会导致旧 mp4 缺少 moov atom 而损坏
	a.gracefulStopRecording(deviceId)

	args := []string{"-s", deviceId, "--no-window", "--no-audio-playback", "--record", config.RecordPath}

	if config.MaxSize > 0 {
		args = append(args, "--max-size", fmt.Sprintf("%d", config.MaxSize))
	}
	if config.BitRate > 0 {
		args = append(args, "--video-bit-rate", fmt.Sprintf("%d", config.BitRate))
	}
	if config.MaxFps > 0 {
		args = append(args, "--max-fps", fmt.Sprintf("%d", config.MaxFps))
	}
	if config.VideoCodec != "" {
		args = append(args, "--video-codec", config.VideoCodec)
	}

	if config.NoAudio {
		args = append(args, "--no-audio")
	} else if config.AudioCodec != "" {
		args = append(args, "--audio-codec", config.AudioCodec)
	}

	if config.VideoSource == "camera" {
		args = append(args, "--video-source", "camera")
		if config.CameraId != "" {
			args = append(args, "--camera-id", config.CameraId)
		}
		if config.CameraSize != "" {
			args = append(args, "--camera-size", config.CameraSize)
		}
	} else {
		if config.VideoSource == "display" {
			args = append(args, "--video-source", "display")
		}
		if config.DisplayId > 0 {
			args = append(args, "--display-id", fmt.Sprintf("%d", config.DisplayId))
		}
	}
	if config.DisplayOrientation != "" && config.DisplayOrientation != "0" {
		args = append(args, "--display-orientation", config.DisplayOrientation)
	}

	cmd := a.newScrcpyCommand(args...)

	a.Log("Starting recording process: %s %v", a.scrcpyPath, cmd.Args)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start recording: %w", err)
	}

	a.scrcpyMu.Lock()
	a.scrcpyRecordCmd[deviceId] = cmd
	a.scrcpyMu.Unlock()

	if !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "scrcpy-record-started", map[string]interface{}{
			"deviceId":   deviceId,
			"recordPath": config.RecordPath,
			"startTime":  time.Now().Unix(),
		})
	}

	go func() {
		_ = cmd.Wait()
		a.scrcpyMu.Lock()
		// 仅当注册表中仍是本进程时才清理, 避免误删替换后的新录制进程
		stillRegistered := a.scrcpyRecordCmd[deviceId] == cmd
		if stillRegistered {
			delete(a.scrcpyRecordCmd, deviceId)
		}
		a.scrcpyMu.Unlock()
		if stillRegistered && !a.mcpMode {
			wailsRuntime.EventsEmit(a.ctx, "scrcpy-record-stopped", deviceId)
		}
	}()

	return nil
}

// gracefulStopRecording 优雅终止设备的 scrcpy 录制进程:
// 先发送 Interrupt 让 scrcpy 写完 mp4 的 moov atom, 最多等 3 秒, 超时再 Kill。
// 进程退出由 StartRecording 的 Wait goroutine 感知并从注册表删除条目,
// 这里以条目消失作为退出信号轮询。调用时不得持有 scrcpyMu。
func (a *App) gracefulStopRecording(deviceId string) {
	a.scrcpyMu.Lock()
	cmd, exists := a.scrcpyRecordCmd[deviceId]
	a.scrcpyMu.Unlock()
	if !exists || cmd.Process == nil {
		return
	}

	if runtime.GOOS == "windows" {
		// Windows 不支持向子进程发送 Interrupt, 只能 Kill
		_ = cmd.Process.Kill()
		return
	}

	_ = cmd.Process.Signal(os.Interrupt)

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		a.scrcpyMu.Lock()
		current, still := a.scrcpyRecordCmd[deviceId]
		a.scrcpyMu.Unlock()
		if !still || current != cmd {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
}

// StopRecording stops the recording process for the given device
func (a *App) StopRecording(deviceId string) error {
	a.scrcpyMu.Lock()
	defer a.scrcpyMu.Unlock()

	if cmd, exists := a.scrcpyRecordCmd[deviceId]; exists && cmd.Process != nil {
		var err error
		if runtime.GOOS != "windows" {
			err = cmd.Process.Signal(os.Interrupt)
		} else {
			err = cmd.Process.Kill()
		}

		if err != nil && (strings.Contains(err.Error(), "process already finished") || strings.Contains(err.Error(), "already finished")) {
			return nil
		}
		return err
	}
	return nil
}

// IsRecording checks if a recording process is running for the device
func (a *App) IsRecording(deviceId string) bool {
	a.scrcpyMu.Lock()
	defer a.scrcpyMu.Unlock()
	_, exists := a.scrcpyRecordCmd[deviceId]
	return exists
}

// ListCameras returns a list of available cameras for the given device
func (a *App) ListCameras(deviceId string) ([]string, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("no device specified")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := a.newScrcpyCommandContext(ctx, "-s", deviceId, "--list-cameras")
	output, err := cmd.CombinedOutput()
	a.Log("ListCameras for %s: err=%v, output=%s", deviceId, err, string(output))

	lines := strings.Split(string(output), "\n")
	var cameras []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "--camera-id=") {
			cameras = append(cameras, line)
		}
	}
	return cameras, nil
}

// ListDisplays returns a list of available displays for the given device
func (a *App) ListDisplays(deviceId string) ([]string, error) {
	if deviceId == "" {
		return nil, fmt.Errorf("no device specified")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := a.newScrcpyCommandContext(ctx, "-s", deviceId, "--list-displays")
	output, err := cmd.CombinedOutput()
	a.Log("ListDisplays for %s: err=%v, output=%s", deviceId, err, string(output))

	lines := strings.Split(string(output), "\n")
	var displays []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "--display-id=") {
			displays = append(displays, line)
		}
	}
	return displays, nil
}

// SelectRecordPath returns a default recording path in the Downloads folder
func (a *App) SelectRecordPath(deviceModel string) (string, error) {
	defaultDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(defaultDir, "Downloads")
	if _, err := os.Stat(downloadsDir); err == nil {
		defaultDir = downloadsDir
	}

	cleanModel := "Device"
	if deviceModel != "" {
		cleanModel = strings.ReplaceAll(deviceModel, " ", "_")
		reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
		cleanModel = reg.ReplaceAllString(cleanModel, "")
	}

	filename := fmt.Sprintf("Gaze_%s_%s.mp4", cleanModel, time.Now().Format("20060102_150405"))
	fullPath := filepath.Join(defaultDir, filename)
	return fullPath, nil
}

// SelectScreenshotPath returns a default screenshot path in the Downloads folder
func (a *App) SelectScreenshotPath(deviceModel string) (string, error) {
	defaultDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(defaultDir, "Downloads")
	if _, err := os.Stat(downloadsDir); err == nil {
		defaultDir = downloadsDir
	}

	cleanModel := "Device"
	if deviceModel != "" {
		cleanModel = strings.ReplaceAll(deviceModel, " ", "_")
		reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
		cleanModel = reg.ReplaceAllString(cleanModel, "")
	}

	filename := fmt.Sprintf("Screenshot_%s_%s.png", cleanModel, time.Now().Format("20060102_150405"))
	fullPath := filepath.Join(defaultDir, filename)
	return fullPath, nil
}

// TakeScreenshot captures a screenshot of the device and saves it to the host
func (a *App) TakeScreenshot(deviceId, savePath string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	if savePath == "" {
		return "", fmt.Errorf("no save path specified")
	}

	a.updateLastActive(deviceId)

	checkCmd := a.newAdbCommand(nil, "-s", deviceId, "shell", "dumpsys power | grep -iE 'state=|wakefulness=' ; dumpsys window | grep -iE 'keyguardShowing|showingLockscreen'")
	out, _ := checkCmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))

	reOn := regexp.MustCompile(`(?i)wakefulness=Awake|state=ON|mDisplayState=ON`)
	isOn := reOn.MatchString(outStr)
	isOff := !isOn

	reLocked := regexp.MustCompile(`(?i)(keyguardShowing|showingLockscreen).*true`)
	isLocked := reLocked.MatchString(outStr)

	if isOff || isLocked {
		if !a.mcpMode {
			wailsRuntime.EventsEmit(a.ctx, "screenshot-progress", "screenshot_off")
		}
		return "", fmt.Errorf("SCREEN_OFF")
	}

	if !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "screenshot-progress", "screenshot_capturing")
	}
	// Use unique remote path to avoid race conditions with concurrent/rapid calls
	remotePath := fmt.Sprintf("/sdcard/screenshot_%d.png", time.Now().UnixNano())
	capCmd := a.newAdbCommand(nil, "-s", deviceId, "shell", "screencap", "-p", remotePath)
	if out, err := capCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to capture screenshot on device: %w, output: %s", err, string(out))
	}
	defer a.newAdbCommand(nil, "-s", deviceId, "shell", "rm", remotePath).Run()

	// Force filesystem sync to ensure screenshot is fully written before pulling
	a.newAdbCommand(nil, "-s", deviceId, "shell", "sync").Run()

	if !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "screenshot-progress", "screenshot_pulling")
	}
	pullCmd := a.newAdbCommand(nil, "-s", deviceId, "pull", remotePath, savePath)
	if out, err := pullCmd.CombinedOutput(); err != nil {
		if !a.mcpMode {
			wailsRuntime.EventsEmit(a.ctx, "screenshot-progress", "screenshot_error", err.Error())
		}
		return "", fmt.Errorf("failed to pull screenshot: %w, output: %s", err, string(out))
	}

	if !a.mcpMode {
		wailsRuntime.EventsEmit(a.ctx, "screenshot-progress", "screenshot_success", savePath)
	}
	return savePath, nil
}

// OpenPath opens a file or directory in the default system browser
func (a *App) OpenPath(path string) error {
	if path == "::recordings::" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, "Downloads")
	}

	info, err := os.Stat(path)
	isDir := err == nil && info.IsDir()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		if isDir {
			cmd = exec.Command("explorer", filepath.Clean(path))
		} else {
			cmd = exec.Command("explorer", "/select,", filepath.Clean(path))
		}
	case "darwin":
		if isDir {
			cmd = exec.Command("open", path)
		} else {
			cmd = exec.Command("open", "-R", path)
		}
	default:
		if isDir {
			cmd = exec.Command("xdg-open", path)
		} else {
			cmd = exec.Command("xdg-open", filepath.Dir(path))
		}
	}
	return cmd.Start()
}

// OpenFile opens a file with the default system application (e.g., video player for videos)
func (a *App) OpenFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", path)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", filepath.Clean(path))
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}
