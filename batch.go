package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// batchOperationTimeout 返回批量操作的单设备超时:
// install/push 涉及大文件传输给 5 分钟, 其余操作 60 秒。
// 无超时时一台无响应设备会让整批操作永久挂起。
func batchOperationTimeout(opType string) time.Duration {
	switch opType {
	case "install", "push":
		return 5 * time.Minute
	default:
		return 60 * time.Second
	}
}

// emitBatchEvent 将单设备的批量操作结果发到该设备的 Session 时间线
func (a *App) emitBatchEvent(opType string, br BatchResult) {
	if a.eventPipeline == nil {
		return
	}
	level := LevelInfo
	status := "success"
	if !br.Success {
		level = LevelError
		status = "failed"
	}
	a.eventPipeline.EmitRaw(br.DeviceID, SourceSystem, "batch_operation", level,
		fmt.Sprintf("Batch %s: %s", opType, status), br)
}

// ExecuteBatchOperation executes an operation on multiple devices in parallel
func (a *App) ExecuteBatchOperation(op BatchOperation) BatchOperationResult {
	result := BatchOperationResult{
		TotalDevices: len(op.DeviceIDs),
		Results:      make([]BatchResult, 0, len(op.DeviceIDs)),
	}

	if len(op.DeviceIDs) == 0 {
		return result
	}

	var wg sync.WaitGroup
	resultsChan := make(chan BatchResult, len(op.DeviceIDs))

	for _, deviceID := range op.DeviceIDs {
		wg.Add(1)
		go func(devID string) {
			defer wg.Done()

			var br BatchResult
			br.DeviceID = devID

			if err := ValidateDeviceID(devID); err != nil {
				br.Error = fmt.Sprintf("invalid device ID: %v", err)
			} else {
				ctx, cancel := context.WithTimeout(a.ctx, batchOperationTimeout(op.Type))
				switch op.Type {
				case "install":
					br = a.batchInstall(ctx, devID, op.APKPath)
				case "uninstall":
					br = a.batchUninstall(ctx, devID, op.PackageName)
				case "clear":
					br = a.batchClearData(ctx, devID, op.PackageName)
				case "stop":
					br = a.batchForceStop(ctx, devID, op.PackageName)
				case "shell":
					br = a.batchShellCommand(ctx, devID, op.Command)
				case "push":
					br = a.batchPushFile(ctx, devID, op.LocalPath, op.RemotePath)
				case "reboot":
					br = a.batchReboot(ctx, devID)
				default:
					br.Error = fmt.Sprintf("unknown operation type: %s", op.Type)
				}
				cancel()
			}

			br.DeviceID = devID
			a.emitBatchEvent(op.Type, br)
			resultsChan <- br

			// Emit progress event
			if !a.mcpMode {
				wailsRuntime.EventsEmit(a.ctx, "batch-progress", br)
			}
		}(deviceID)
	}

	// Wait for all operations to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results (单 goroutine 消费, 无需加锁)
	for br := range resultsChan {
		result.Results = append(result.Results, br)
		if br.Success {
			result.SuccessCount++
		} else {
			result.FailureCount++
		}
	}

	return result
}

func (a *App) batchInstall(ctx context.Context, deviceID, apkPath string) BatchResult {
	br := BatchResult{DeviceID: deviceID}

	if apkPath == "" {
		br.Error = "no APK path specified"
		return br
	}

	cmd := a.newAdbCommand(ctx, "-s", deviceID, "install", "-r", apkPath)
	output, err := cmd.CombinedOutput()
	br.Output = string(output)

	if err != nil {
		br.Error = err.Error()
		return br
	}

	if strings.Contains(br.Output, "Success") {
		br.Success = true
	} else if strings.Contains(br.Output, "Failure") {
		br.Error = br.Output
	} else {
		br.Success = true
	}

	return br
}

func (a *App) batchUninstall(ctx context.Context, deviceID, packageName string) BatchResult {
	br := BatchResult{DeviceID: deviceID}

	if packageName == "" {
		br.Error = "no package name specified"
		return br
	}

	// Try standard uninstall first
	cmd := a.newAdbCommand(ctx, "-s", deviceID, "uninstall", packageName)
	output, err := cmd.CombinedOutput()
	br.Output = string(output)

	if err == nil && !strings.Contains(br.Output, "Failure") {
		br.Success = true
		return br
	}

	// Try pm uninstall for system apps
	cmd2 := a.newAdbCommand(ctx, "-s", deviceID, "shell", "pm", "uninstall", "-k", "--user", "0", packageName)
	output2, err2 := cmd2.CombinedOutput()
	br.Output = string(output2)

	if err2 != nil || strings.Contains(br.Output, "Failure") {
		br.Error = br.Output
		return br
	}

	br.Success = true
	return br
}

func (a *App) batchClearData(ctx context.Context, deviceID, packageName string) BatchResult {
	br := BatchResult{DeviceID: deviceID}

	if packageName == "" {
		br.Error = "no package name specified"
		return br
	}

	cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell", "pm", "clear", packageName)
	output, err := cmd.CombinedOutput()
	br.Output = string(output)

	if err != nil {
		br.Error = err.Error()
		return br
	}

	if strings.Contains(br.Output, "Success") {
		br.Success = true
	} else {
		br.Error = br.Output
	}

	return br
}

func (a *App) batchForceStop(ctx context.Context, deviceID, packageName string) BatchResult {
	br := BatchResult{DeviceID: deviceID}

	if packageName == "" {
		br.Error = "no package name specified"
		return br
	}

	cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell", "am", "force-stop", packageName)
	output, err := cmd.CombinedOutput()
	br.Output = string(output)

	if err != nil {
		br.Error = err.Error()
		return br
	}

	br.Success = true
	return br
}

func (a *App) batchShellCommand(ctx context.Context, deviceID, command string) BatchResult {
	br := BatchResult{DeviceID: deviceID}

	if command == "" {
		br.Error = "no command specified"
		return br
	}

	cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell", command)
	output, err := cmd.CombinedOutput()
	br.Output = string(output)

	if err != nil {
		br.Error = err.Error()
		return br
	}

	br.Success = true
	return br
}

func (a *App) batchPushFile(ctx context.Context, deviceID, localPath, remotePath string) BatchResult {
	br := BatchResult{DeviceID: deviceID}

	if localPath == "" || remotePath == "" {
		br.Error = "local path and remote path are required"
		return br
	}

	cmd := a.newAdbCommand(ctx, "-s", deviceID, "push", localPath, remotePath)
	output, err := cmd.CombinedOutput()
	br.Output = string(output)

	if err != nil {
		br.Error = err.Error()
		return br
	}

	br.Success = true
	return br
}

func (a *App) batchReboot(ctx context.Context, deviceID string) BatchResult {
	br := BatchResult{DeviceID: deviceID}

	cmd := a.newAdbCommand(ctx, "-s", deviceID, "reboot")
	output, err := cmd.CombinedOutput()
	br.Output = string(output)

	if err != nil {
		br.Error = err.Error()
		return br
	}

	br.Success = true
	return br
}

// SelectAPKForBatch opens a file dialog to select an APK file
func (a *App) SelectAPKForBatch() (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select APK",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Android Package (*.apk)", Pattern: "*.apk"},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// SelectFileForBatch opens a file dialog to select a file for pushing
func (a *App) SelectFileForBatch() (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select File to Push",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}
