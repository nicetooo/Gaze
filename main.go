package main

import (
	"context"
	"embed"
	"runtime"

	"time"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/icon.svg
var iconData []byte

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()
	var shouldQuit bool

	// Create application menu
	var applicationMenu *menu.Menu
	if runtime.GOOS == "darwin" {
		applicationMenu = menu.NewMenu()
		applicationMenu.Append(menu.AppMenu())
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
		Menu:             applicationMenu,
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			// Initialize system tray
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
						for {
							select {
							case <-ctx.Done():
								return
							case <-ticker.C:
								currentDevices, _ := app.GetDevices()
								// Simple check if devices changed (count or IDs)
								changed := false
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

								if changed {
									lastDevices = currentDevices
									systray.ResetMenu()
									updateTrayMenu(ctx, app)
								}
							}
						}
					}()
				}, func() {})
				start()
			}
		},
		WindowStartState: options.Normal,
		OnBeforeClose: func(ctx context.Context) (prevent bool) {
			if runtime.GOOS == "darwin" && !shouldQuit {
				wailsRuntime.WindowHide(ctx)
				return true // Prevent closing
			}
			return false
		},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: true,
				HideTitle:                  false,
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

var shouldQuit bool

func updateTrayMenu(ctx context.Context, app *App) {
	devices, _ := app.GetDevices()

	if len(devices) > 0 {
		systray.AddMenuItem("Connected Devices:", "").Disable()
		for _, dev := range devices {
			name := dev.Model
			if name == "" {
				name = dev.ID
			}
			// Truncate if too long
			if len(name) > 30 {
				name = name[:27] + "..."
			}

			devItem := systray.AddMenuItem(name, "")

			// Submenus
			mMirror := devItem.AddSubMenuItem("Screen Mirror", "")
			d := dev // Capture loop variable
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
	} else {
		systray.AddMenuItem("No devices connected", "").Disable()
	}

	systray.AddSeparator()

	mOpen := systray.AddMenuItem("Open adbGUI", "")
	mOpen.Click(func() {
		wailsRuntime.WindowShow(ctx)
	})

	mQuit := systray.AddMenuItem("Quit", "")
	mQuit.Click(func() {
		shouldQuit = true
		systray.Quit()
		wailsRuntime.Quit(ctx)
	})
}
