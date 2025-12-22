package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"time"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/icon.svg
var iconData []byte

//go:embed build/icon_recording.svg
var iconRecordingData []byte

//go:embed all:frontend/dist
var assets embed.FS

// version is the application version, set at build time via -ldflags
var version = "v0.0.0-dev"

func main() {
	// Create an instance of the app structure
	app := NewApp(version)

	// Handle system signals for graceful shutdown (like Dock Quit or Cmd+Q)
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		sig := <-sigChan
		os.Stderr.WriteString(fmt.Sprintf("\n[SIGNAL RECEIVED: %v]\n", sig))
		wailsRuntime.Quit(app.ctx)
		time.Sleep(200 * time.Millisecond)
		os.Exit(0)
	}()

	// Create application menu
	var applicationMenu *menu.Menu
	if runtime.GOOS == "darwin" {
		applicationMenu = menu.NewMenu()

		// Custom App Menu to intercept Quit
		customAppMenu := menu.NewMenu()
		customAppMenu.Append(menu.Text("About adbGUI", nil, func(_ *menu.CallbackData) {
			wailsRuntime.WindowShow(app.ctx)
		}))
		customAppMenu.Append(menu.Separator())
		customAppMenu.Append(menu.Text("Quit adbGUI", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
			os.Stderr.WriteString("\n[MENU QUIT CLICKED]\n")
			wailsRuntime.Quit(app.ctx)
		}))

		applicationMenu.Append(menu.SubMenu("adbGUI", customAppMenu))
		applicationMenu.Append(menu.EditMenu())
		applicationMenu.Append(menu.WindowMenu())
	}

	// Create application with options
	err := wails.Run(&options.App{
		Title:     "adbGUI",
		Width:     1280,
		Height:    720,
		MinWidth:  1280,
		MinHeight: 720,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		Menu:              applicationMenu,
		BackgroundColour:  &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		HideWindowOnClose: true,
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)

			// Initialize system tray
			if runtime.GOOS == "darwin" {
				start, _ := systray.RunWithExternalLoop(func() {
					systray.SetIcon(iconData)
					systray.SetTooltip("adbGUI")

					// Initial update
					updateTrayMenu(ctx, app)

					// Start ticker to update tray menu
					go func() {
						ticker := time.NewTicker(2 * time.Second)
						var lastDevices []Device
						lastRecordingStates := make(map[string]bool)

						for {
							select {
							case <-ctx.Done():
								return
							case <-ticker.C:
								currentDevices, _ := app.GetDevices()
								changed := false

								// Check devices
								if len(lastDevices) != len(currentDevices) {
									changed = true
								} else {
									for i, d := range currentDevices {
										if d.ID != lastDevices[i].ID || d.State != lastDevices[i].State {
											changed = true
											break
										}
									}
								}

								// Check recording states
								currentRecordingStates := make(map[string]bool)
								for _, d := range currentDevices {
									isRec := app.IsRecording(d.ID)
									currentRecordingStates[d.ID] = isRec
									if lastRecordingStates[d.ID] != isRec {
										changed = true
									}
								}

								if changed {
									lastDevices = currentDevices
									lastRecordingStates = currentRecordingStates
									systray.ResetMenu()
									updateTrayMenu(ctx, app)
								}
							}
						}
					}()
				}, func() {
					os.Stderr.WriteString("\n[SYSTRAY EXITING]\n")
					os.Exit(0)
				})
				start()
			}
		},
		OnShutdown: func(ctx context.Context) {
			app.Shutdown(ctx)
		},
		WindowStartState: options.Normal,
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  true,
				HideTitleBar:               false,
				FullSizeContent:            true,
				UseToolbar:                 false,
				HideToolbarSeparator:       true,
			},
			Appearance:           mac.NSAppearanceNameDarkAqua,
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			About: &mac.AboutInfo{
				Title:   "adbGUI",
				Message: "A modern ADB GUI tool",
			},
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// package-level variable to track if we should really quit

func updateTrayMenu(ctx context.Context, app *App) {

	// 1. Get all devices
	connectedDevices, _ := app.GetDevices()
	historyDevices := app.GetHistoryDevices()

	// Check for any active recording
	anyRecording := false
	for _, dev := range connectedDevices {
		if app.IsRecording(dev.ID) {
			anyRecording = true
			break
		}
	}

	if anyRecording {
		systray.SetIcon(iconRecordingData)
	} else {
		systray.SetIcon(iconData)
	}

	hasDevices := false
	seenSerials := make(map[string]bool)

	// A. Promote Primary Device Section to Top Level
	if len(connectedDevices) > 0 {
		d := connectedDevices[0]
		name := d.Model
		if name == "" {
			name = d.ID
		}
		if app.IsRecording(d.ID) {
			name = "⏺️ " + name
		}
		// Section Header (Device Name)
		mHeader := systray.AddMenuItem(name+":", "")
		mHeader.Disable()

		// 1. Mirror
		mMirrorTop := systray.AddMenuItem("  Screen Mirror", "")
		mMirrorTop.Click(func() {
			go func() {
				config := ScrcpyConfig{BitRate: 8, MaxFps: 60, StayAwake: true, VideoCodec: "h264", AudioCodec: "opus"}
				app.StartScrcpy(d.ID, config)
			}()
		})

		// 2. Screenshot
		mScreenshotTop := systray.AddMenuItem("  Take Screenshot", "")
		mScreenshotTop.Click(func() {
			go func() {
				savePath, err := app.SelectScreenshotPath(d.Model)
				if err == nil && savePath != "" {
					_, _ = app.TakeScreenshot(d.ID, savePath)
				}
			}()
		})

		// 3. Recording
		if app.IsRecording(d.ID) {
			mRecordTop := systray.AddMenuItem("  Stop Recording", "")
			mRecordTop.Click(func() { go app.StopRecording(d.ID) })
		} else {
			mRecordTop := systray.AddMenuItem("  Start Recording", "")
			mRecordTop.Click(func() {
				go func() {
					home, _ := os.UserHomeDir()
					saveDir := filepath.Join(home, "Downloads")
					if _, err := os.Stat(saveDir); err != nil {
						saveDir = home
					}
					filename := fmt.Sprintf("adbGUI_record_%s_%s.mp4", strings.ReplaceAll(d.Model, " ", "_"), time.Now().Format("20060102_150405"))
					savePath := filepath.Join(saveDir, filename)
					config := ScrcpyConfig{RecordPath: savePath, MaxSize: 0, BitRate: 8, MaxFps: 60, VideoCodec: "h264", NoAudio: false}
					app.StartRecording(d.ID, config)
				}()
			})
		}

		// 4. Logcat
		mLogcatTop := systray.AddMenuItem("  Logcat", "")
		mLogcatTop.Click(func() {
			go func() {
				wailsRuntime.WindowShow(ctx)
				wailsRuntime.EventsEmit(ctx, "tray:navigate", map[string]string{"view": "logcat", "deviceId": d.ID})
			}()
		})

		// 5. Shell
		mShellTop := systray.AddMenuItem("  Shell", "")
		mShellTop.Click(func() {
			go func() {
				wailsRuntime.WindowShow(ctx)
				wailsRuntime.EventsEmit(ctx, "tray:navigate", map[string]string{"view": "shell", "deviceId": d.ID})
			}()
		})

		// 6. Files
		mFilesTop := systray.AddMenuItem("  Files", "")
		mFilesTop.Click(func() {
			go func() {
				wailsRuntime.WindowShow(ctx)
				wailsRuntime.EventsEmit(ctx, "tray:navigate", map[string]string{"view": "files", "deviceId": d.ID})
			}()
		})

		systray.AddSeparator()
	}

	systray.AddMenuItem("Devices:", "").Disable()

	// Process Connected Devices
	for _, dev := range connectedDevices {
		hasDevices = true
		// Track seen devices
		key := dev.Serial
		if key == "" {
			key = dev.ID
		}
		seenSerials[key] = true

		name := dev.Model
		if name == "" {
			name = dev.ID
		}
		// Truncate if too long
		if len(name) > 30 {
			name = name[:27] + "..."
		}

		if app.IsRecording(dev.ID) {
			name = "⏺️ " + name
		}

		devItem := systray.AddMenuItem(name, "")

		d := dev // Capture loop variable

		// Screenshot
		mScreenshot := devItem.AddSubMenuItem("Take Screenshot", "")
		mScreenshot.Click(func() {
			go func() {
				// Use app functions to generate a path and take screenshot
				savePath, err := app.SelectScreenshotPath(d.Model)
				if err != nil {
					wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
						Type:    wailsRuntime.ErrorDialog,
						Title:   "Screenshot Failed",
						Message: fmt.Sprintf("Failed to select path: %v", err),
					})
					return
				}
				if savePath == "" {
					return // Cancelled by user
				}

				finalPath, err := app.TakeScreenshot(d.ID, savePath)
				if err != nil {
					return
				}
				_ = finalPath
			}()
		})

		// Submenus for connected devices
		mMirror := devItem.AddSubMenuItem("Screen Mirror", "")
		mMirror.Click(func() {
			go func() {
				// Default config for tray launch
				config := ScrcpyConfig{
					BitRate:    8,
					MaxFps:     60,
					StayAwake:  true,
					VideoCodec: "h264",
					AudioCodec: "opus",
				}
				app.StartScrcpy(d.ID, config)
			}()
		})

		// Recording
		if app.IsRecording(d.ID) {
			mRecord := devItem.AddSubMenuItem("Stop Recording", "")
			mRecord.Click(func() {
				go func() {
					app.StopRecording(d.ID)
				}()
			})
		} else {
			mRecord := devItem.AddSubMenuItem("Start Recording", "")
			mRecord.Click(func() {
				go func() {
					home, _ := os.UserHomeDir()
					// Default to Downloads
					saveDir := filepath.Join(home, "Downloads")
					if _, err := os.Stat(saveDir); err != nil {
						saveDir = home // Fallback
					}

					filename := fmt.Sprintf("adbGUI_record_%s_%s.mp4",
						strings.ReplaceAll(d.Model, " ", "_"),
						time.Now().Format("20060102_150405"))
					savePath := filepath.Join(saveDir, filename)

					// Use default nice settings
					config := ScrcpyConfig{
						RecordPath: savePath,
						MaxSize:    0, // nominal
						BitRate:    8, // 8Mbps
						MaxFps:     60,
						VideoCodec: "h264",
						NoAudio:    false,
					}
					app.StartRecording(d.ID, config)
				}()
			})
		}

		mLogcat := devItem.AddSubMenuItem("Logcat", "")
		mLogcat.Click(func() {
			go func() {
				wailsRuntime.WindowShow(ctx)
				wailsRuntime.EventsEmit(ctx, "tray:navigate", map[string]string{
					"view":     "logcat",
					"deviceId": d.ID,
				})
			}()
		})

		mShell := devItem.AddSubMenuItem("Shell", "")
		mShell.Click(func() {
			go func() {
				wailsRuntime.WindowShow(ctx)
				wailsRuntime.EventsEmit(ctx, "tray:navigate", map[string]string{
					"view":     "shell",
					"deviceId": d.ID,
				})
			}()
		})

		mFiles := devItem.AddSubMenuItem("Files", "")
		mFiles.Click(func() {
			go func() {
				wailsRuntime.WindowShow(ctx)
				wailsRuntime.EventsEmit(ctx, "tray:navigate", map[string]string{
					"view":     "files",
					"deviceId": d.ID,
				})
			}()
		})
	}

	// Process History (Connectable) Devices
	for _, hDev := range historyDevices {
		// Skip if no wireless IP
		if hDev.WifiAddr == "" {
			continue
		}

		// Skip if already connected (check Serial and WifiAddr/ID)
		if seenSerials[hDev.Serial] {
			continue
		}

		// Additional check: is this IP already connected?
		alreadyConnected := false
		for _, cDev := range connectedDevices {
			if cDev.ID == hDev.WifiAddr {
				alreadyConnected = true
				break
			}
		}
		if alreadyConnected {
			continue
		}

		hasDevices = true

		name := hDev.Model
		if name == "" {
			name = hDev.ID
		}
		name = fmt.Sprintf("%s (Offline)", name)

		if len(name) > 30 {
			name = name[:27] + "..."
		}

		devItem := systray.AddMenuItem(name, "")

		// Submenu for connectable device
		mConnect := devItem.AddSubMenuItem("Wireless Connect", "")
		ip := hDev.WifiAddr
		mConnect.Click(func() {
			go func() {
				_, _ = app.AdbConnect(ip)
				// The ticker will pick up the new connection automatically
			}()
		})
	}

	if !hasDevices {
		systray.AddMenuItem("No devices available", "").Disable()
	}

	systray.AddSeparator()

	mOpen := systray.AddMenuItem("Open adbGUI", "")
	mOpen.Click(func() {
		wailsRuntime.WindowShow(ctx)
	})

	mQuit := systray.AddMenuItem("Quit", "")
	mQuit.Click(func() {
		systray.Quit()
		wailsRuntime.Quit(ctx)
	})
}
