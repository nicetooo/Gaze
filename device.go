package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetDevices returns a list of connected ADB devices
func (a *App) GetDevices(forceLog bool) ([]Device, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if a.adbPath == "" {
		return nil, fmt.Errorf("ADB path is not initialized")
	}

	// 1. Get raw output from adb devices -l
	cmd := a.newAdbCommand(ctx, "devices", "-l")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run adb devices (path: %s): %w, output: %s", a.adbPath, err, string(output))
	}

	// Load history to help with device identification and metadata preservation
	a.historyMu.Lock()
	historyDevices := a.loadHistoryInternal()
	a.historyMu.Unlock()

	historyByID := make(map[string]HistoryDevice)
	historyBySerial := make(map[string]HistoryDevice)
	for _, hd := range historyDevices {
		if hd.ID != "" {
			historyByID[hd.ID] = hd
		}
		if hd.Serial != "" {
			historyBySerial[hd.Serial] = hd
		}
	}

	// 3. Parse raw identifiers
	lines := strings.Split(string(output), "\n")
	type adbNode struct {
		id         string
		state      string
		isWireless bool
		isMDNS     bool
		hasUSB     bool
		model      string
		serial     string // resolved hardware serial
	}
	var nodes []*adbNode

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices attached") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			node := &adbNode{
				id:    parts[0],
				state: parts[1],
			}
			// Parse properties
			for _, p := range parts[2:] {
				if strings.Contains(p, ":") {
					kv := strings.SplitN(p, ":", 2)
					if kv[0] == "model" {
						node.model = kv[1]
					}
					if kv[0] == "usb" {
						node.hasUSB = true
					}
				}
			}
			node.isWireless = strings.Contains(node.id, ":") || strings.Contains(node.id, "._tcp") || strings.Contains(node.id, "._adb-tls-connect")
			if node.hasUSB {
				node.isWireless = false
			}
			node.isMDNS = strings.Contains(node.id, "._tcp") || strings.Contains(node.id, "._adb-tls-connect")
			nodes = append(nodes, node)

			// OPTIMIZATION: If a wireless device is offline, try to reconnect it
			if node.isWireless && node.state == "offline" {
				a.tryAutoReconnect(node.id)
			}
		}
	}

	// 3.5. Proactively reconnect to recently active wireless devices missing from the current list
	for _, hd := range historyDevices {
		if hd.WifiAddr != "" && time.Since(time.Unix(hd.LastSeen, 0)) < 15*time.Minute {
			found := false
			for _, n := range nodes {
				if n.id == hd.WifiAddr {
					found = true
					break
				}
			}
			if !found {
				a.tryAutoReconnect(hd.WifiAddr)
			}
		}
	}

	// Regex for mDNS serial extraction
	mdnsRe := regexp.MustCompile(`adb-([a-zA-Z0-9]+)-`)

	// 4. Phase 1: Resolve "True Serial" for every node
	var wg sync.WaitGroup
	for _, n := range nodes {
		wg.Add(1)
		go func(node *adbNode) {
			defer wg.Done()

			// A. If already authorised, ask the device
			if node.state == "device" {
				sCtx, sCancel := context.WithTimeout(ctx, 3*time.Second)
				defer sCancel()
				c := exec.CommandContext(sCtx, a.adbPath, "-s", node.id, "shell", "getprop ro.serialno")
				out, err := c.Output()
				if err == nil {
					s := strings.TrimSpace(string(out))
					if s != "" {
						node.serial = s
						return
					}
				}
			}

			// B. Extract from mDNS ID if possible (format: adb-SERIAL-...)
			if node.isMDNS {
				matches := mdnsRe.FindStringSubmatch(node.id)
				if len(matches) > 1 {
					node.serial = matches[1]
					return
				}
			}

			// C. Try History by current ID
			if h, ok := historyByID[node.id]; ok && h.Serial != "" {
				node.serial = h.Serial
				return
			}

			// D. Fallback: use ID as serial for non-wireless or unknown
			if !node.isWireless {
				node.serial = node.id
			}
		}(n)
	}
	wg.Wait()

	// 5. Phase 2: Grouping by resolved Serial
	deviceMap := make(map[string]*Device)
	var finalDevices []*Device

	// Sort nodes to ensure stable primary ID selection (prefer wired)
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].hasUSB != nodes[j].hasUSB {
			return nodes[i].hasUSB
		}
		if nodes[i].state != nodes[j].state {
			return nodes[i].state == "device"
		}
		return !nodes[i].isMDNS
	})

	for _, n := range nodes {
		serialKey := n.serial
		if serialKey == "" {
			serialKey = n.id
		}

		d, exists := deviceMap[serialKey]
		if !exists {
			d = &Device{
				ID:     n.id,
				Serial: serialKey,
				State:  n.state,
				IDs:    []string{n.id},
				Model:  strings.TrimSpace(strings.ReplaceAll(n.model, "_", " ")),
			}
			if n.isWireless {
				d.Type = "wireless"
				d.WifiAddr = n.id
			} else {
				d.Type = "wired"
			}
			deviceMap[serialKey] = d
			finalDevices = append(finalDevices, d)
		} else {
			d.IDs = append(d.IDs, n.id)
			if n.state == "device" {
				if d.State != "device" || n.hasUSB {
					d.State = "device"
					d.ID = n.id
				}
			}
			if n.isWireless {
				if !strings.Contains(d.WifiAddr, ":") || strings.Contains(n.id, ":") {
					d.WifiAddr = n.id
				}
				if d.Type == "wired" {
					d.Type = "both"
				} else if d.Type == "" {
					d.Type = "wireless"
				}
			} else if n.hasUSB {
				if d.Type == "wireless" {
					d.Type = "both"
				} else if d.Type == "" {
					d.Type = "wired"
				}
			}
		}
	}

	// 6. Phase 3: Final Polishing (Metadata & History)
	for i := range finalDevices {
		dev := finalDevices[i]

		dev.Model = strings.TrimSpace(strings.ReplaceAll(dev.Model, "_", " "))

		if (dev.Type == "wireless" || dev.Type == "both") && !strings.Contains(dev.WifiAddr, ":") {
			if hd, ok := historyBySerial[dev.Serial]; ok && time.Since(time.Unix(hd.LastSeen, 0)) < 2*time.Hour && strings.Contains(hd.WifiAddr, ":") {
				dev.WifiAddr = hd.WifiAddr
			}
		}

		if dev.Brand == "" || dev.Model == "" {
			if h, ok := historyBySerial[dev.Serial]; ok {
				if dev.Brand == "" {
					dev.Brand = h.Brand
				}
				if dev.Model == "" {
					dev.Model = h.Model
				}
			}
			if dev.Brand == "" || dev.Model == "" {
				for _, hid := range dev.IDs {
					if h, ok := historyByID[hid]; ok {
						if dev.Brand == "" {
							dev.Brand = h.Brand
						}
						if dev.Model == "" {
							dev.Model = h.Model
						}
					}
				}
			}
		}

		if dev.State == "device" {
			wg.Add(1)
			go func(d *Device) {
				defer wg.Done()
				pCtx, pCancel := context.WithTimeout(ctx, 5*time.Second)
				defer pCancel()
				cmd := exec.CommandContext(pCtx, a.adbPath, "-s", d.ID, "shell", "getprop ro.product.manufacturer; getprop ro.product.model")
				out, err := cmd.Output()
				if err == nil {
					parts := strings.Split(string(out), "\n")
					if len(parts) >= 1 && strings.TrimSpace(parts[0]) != "" {
						d.Brand = strings.TrimSpace(parts[0])
					}
					if len(parts) >= 2 && strings.TrimSpace(parts[1]) != "" {
						m := strings.TrimSpace(parts[1])
						d.Model = strings.ReplaceAll(m, "_", " ")
					}
				} else {
					a.Log("Failed to fetch props for %s: %v", d.ID, err)
				}
			}(dev)
		}
	}
	wg.Wait()

	// Sync to history and update ID mapping cache
	newIdToSerial := make(map[string]string)
	for _, d := range finalDevices {
		if d.State == "device" {
			deviceCopy := *d
			go a.addToHistory(deviceCopy)
		}
		newIdToSerial[d.ID] = d.Serial
		newIdToSerial[d.Serial] = d.Serial
		for _, id := range d.IDs {
			newIdToSerial[id] = d.Serial
		}
	}

	a.idToSerialMu.Lock()
	a.idToSerial = newIdToSerial
	a.idToSerialMu.Unlock()

	// 7. Populating Metadata and Sorting
	a.lastActiveMu.RLock()
	a.pinnedMu.RLock()
	for i := range finalDevices {
		d := finalDevices[i]
		if ts, ok := a.lastActive[d.Serial]; ok {
			d.LastActive = ts
		}
		if d.Serial == a.pinnedSerial {
			d.IsPinned = true
		}
	}
	a.pinnedMu.RUnlock()
	a.lastActiveMu.RUnlock()

	sort.SliceStable(finalDevices, func(i, j int) bool {
		if finalDevices[i].IsPinned != finalDevices[j].IsPinned {
			return finalDevices[i].IsPinned
		}
		return finalDevices[i].LastActive > finalDevices[j].LastActive
	})

	if forceLog || len(finalDevices) != a.lastDevCount {
		a.Log("GetDevices returning %d devices (prev: %d)", len(finalDevices), a.lastDevCount)
		a.lastDevCount = len(finalDevices)
	}

	result := make([]Device, len(finalDevices))
	for i, d := range finalDevices {
		result[i] = *d
	}
	return result, nil
}

// GetDeviceInfo returns detailed information about a device
func (a *App) GetDeviceInfo(deviceId string) (DeviceInfo, error) {
	var info DeviceInfo
	info.Props = make(map[string]string)

	if deviceId == "" {
		return info, fmt.Errorf("no device specified")
	}

	runQuickCmd := func(args ...string) string {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := a.newAdbCommand(ctx, append([]string{"-s", deviceId, "shell"}, args...)...)
		out, _ := cmd.Output()
		return strings.TrimSpace(string(out))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := a.newAdbCommand(ctx, "-s", deviceId, "shell", "getprop")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "]: [", 2)
			if len(parts) == 2 {
				key := strings.TrimPrefix(parts[0], "[")
				val := strings.TrimSuffix(parts[1], "]")
				info.Props[key] = val

				switch key {
				case "ro.product.model":
					info.Model = val
				case "ro.product.brand":
					info.Brand = val
				case "ro.product.manufacturer":
					info.Manufacturer = val
				case "ro.build.version.release":
					info.AndroidVer = val
				case "ro.build.version.sdk":
					info.SDK = val
				case "ro.product.cpu.abi":
					info.ABI = val
				case "ro.serialno":
					info.Serial = val
				}
			}
		}
	}

	info.Resolution = strings.TrimPrefix(runQuickCmd("wm", "size"), "Physical size: ")
	info.Density = strings.TrimPrefix(runQuickCmd("wm", "density"), "Physical density: ")

	cpu := runQuickCmd("cat /proc/cpuinfo | grep 'Hardware' | head -1")
	if cpu != "" {
		info.CPU = strings.TrimSpace(strings.TrimPrefix(cpu, "Hardware\t: "))
	}
	if info.CPU == "" {
		cores := runQuickCmd("cat /proc/cpuinfo | grep 'processor' | wc -l")
		if cores != "" {
			info.CPU = fmt.Sprintf("%s Core(s)", cores)
		}
	}

	mem := runQuickCmd("cat /proc/meminfo | grep 'MemTotal'")
	if mem != "" {
		info.Memory = strings.TrimSpace(strings.TrimPrefix(mem, "MemTotal:"))
	}

	return info, nil
}

// AdbPair pairs a device using the given address and code
func (a *App) AdbPair(address string, code string) (string, error) {
	if address == "" || code == "" {
		return "", fmt.Errorf("address and pairing code are required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, a.adbPath, "pair", address, code)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("pairing failed: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// AdbConnect connects to a device using the given address
func (a *App) AdbConnect(address string) (string, error) {
	if address == "" {
		return "", fmt.Errorf("address is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	disconnectCmd := exec.CommandContext(ctx, a.adbPath, "disconnect", address)
	_ = disconnectCmd.Run()

	cmd := exec.CommandContext(ctx, a.adbPath, "connect", address)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("connection failed: %w, output: %s", err, string(output))
	}
	return string(output), nil
}

// AdbDisconnect disconnects from a wireless device
func (a *App) AdbDisconnect(address string) (string, error) {
	if address == "" {
		return "", fmt.Errorf("address is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addresses := strings.Split(address, ",")
	var lastOut string
	var lastErr error

	for _, addr := range addresses {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, a.adbPath, "disconnect", addr)
		output, err := cmd.CombinedOutput()
		lastOut = string(output)
		if err != nil && !strings.Contains(string(output), "no such device") {
			lastErr = err
		}
	}

	if lastErr != nil {
		return lastOut, fmt.Errorf("disconnection failed: %w, output: %s", lastErr, lastOut)
	}
	return "disconnected", nil
}

// GetDeviceIP gets the local IP address of the device
func (a *App) GetDeviceIP(deviceId string) (string, error) {
	if deviceId == "" {
		return "", fmt.Errorf("no device specified")
	}

	cmd := exec.Command(a.adbPath, "-s", deviceId, "shell", "ip addr show wlan0 | grep 'inet ' | awk '{print $2}' | cut -d/ -f1")
	output, err := cmd.CombinedOutput()
	ip := strings.TrimSpace(string(output))

	if err != nil || ip == "" {
		cmd = exec.Command(a.adbPath, "-s", deviceId, "shell", "getprop dhcp.wlan0.ipaddress")
		output, _ = cmd.CombinedOutput()
		ip = strings.TrimSpace(string(output))
	}

	if ip == "" {
		return "", fmt.Errorf("could not find device IP (ensure Wi-Fi is on)")
	}
	return ip, nil
}

// SwitchToWireless enables TCP/IP mode on the device and connects to it
func (a *App) SwitchToWireless(deviceId string) (string, error) {
	ip, err := a.GetDeviceIP(deviceId)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, a.adbPath, "-s", deviceId, "tcpip", "5555")
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), fmt.Errorf("failed to enable tcpip mode: %w", err)
	}

	time.Sleep(1 * time.Second)

	return a.AdbConnect(ip + ":5555")
}

// RestartAdbServer kills and restarts the ADB server
func (a *App) RestartAdbServer() (string, error) {
	a.Log("Restarting ADB server...")

	a.scrcpyMu.Lock()
	for id, cmd := range a.scrcpyCmds {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		delete(a.scrcpyCmds, id)
	}
	a.scrcpyMu.Unlock()

	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/F", "/IM", "adb.exe", "/T").Run()
	} else {
		_ = exec.Command("killall", "adb").Run()
		_ = exec.Command(a.adbPath, "kill-server").Run()
	}
	time.Sleep(500 * time.Millisecond)

	startCmd := exec.Command(a.adbPath, "start-server")
	output, err := startCmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to start adb server: %w", err)
	}

	return "ADB server restarted successfully", nil
}

// RunAdbCommand executes an arbitrary ADB command
func (a *App) RunAdbCommand(deviceId string, fullCmd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fullCmd = strings.TrimSpace(fullCmd)
	if fullCmd == "" {
		return "", nil
	}

	var args []string
	args = append(args, "-s", deviceId)

	if strings.HasPrefix(fullCmd, "shell ") {
		shellArgs := strings.TrimPrefix(fullCmd, "shell ")
		args = append(args, "shell", shellArgs)
	} else {
		args = append(args, strings.Fields(fullCmd)...)
	}

	cmd := a.newAdbCommand(ctx, args...)
	output, err := cmd.CombinedOutput()
	res := string(output)
	if err != nil {
		return res, fmt.Errorf("command failed: %w, output: %s", err, res)
	}
	return strings.TrimSpace(res), nil
}

// GetLocalIP returns the first non-loopback local IPv4 address
func (a *App) GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// StartWirelessServer starts a temporary http server for QR code connection
func (a *App) StartWirelessServer() (string, error) {
	if a.httpServer != nil {
		return a.localAddr, nil
	}

	ip := a.GetLocalIP()
	if ip == "" {
		return "", fmt.Errorf("could not find local IP")
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return "", err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	a.localAddr = fmt.Sprintf("http://%s:%d", ip, port)

	mux := http.NewServeMux()
	mux.HandleFunc("/c", func(w http.ResponseWriter, r *http.Request) {
		remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		a.Log("Wireless connect request from: %s", remoteIP)

		output, err := a.AdbConnect(remoteIP + ":5555")
		success := err == nil && strings.Contains(output, "connected to")

		if success {
			wailsRuntime.EventsEmit(a.ctx, "wireless-connected", remoteIP)
		} else {
			wailsRuntime.EventsEmit(a.ctx, "wireless-connect-failed", map[string]string{
				"ip":    remoteIP,
				"error": output,
			})
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		var title, statusClass, message, hint, nextSteps string
		if success {
			title = "连接成功"
			statusClass = "success"
			message = "设备已成功连接到电脑"
			hint = "现在您可以关闭此页面并在电脑上操作了"
			nextSteps = ""
		} else {
			title = "连接失败"
			statusClass = "error"
			message = "无法连接到 ADB 服务"
			hint = "错误信息: " + strings.ReplaceAll(output, "\n", " ")
			nextSteps = `
				<div class="next-steps">
					<h3>后续操作建议：</h3>
					<ul>
						<li>检查手机 <b>无线调试</b> 是否已开启</li>
						<li>确保手机和电脑在 <b>同一个局域网</b></li>
						<li>如果手机使用了 <b>随机端口</b> (非 5555)，请在电脑上使用"无线配对"功能</li>
						<li>尝试重新扫码</li>
					</ul>
				</div>
			`
		}

		fmt.Fprintf(w, `
			<!DOCTYPE html>
			<html>
			<head>
				<meta charset="utf-8">
				<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
				<style>
					body {
						display: flex;
						flex-direction: column;
						align-items: center;
						justify-content: center;
						min-height: 100vh;
						margin: 0;
						font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
						background-color: #f5f5f5;
						color: #333;
					}
					.card {
						background: white;
						padding: 2rem;
						border-radius: 12px;
						box-shadow: 0 4px 6px rgba(0,0,0,0.1);
						text-align: center;
						width: 85%%;
						max-width: 400px;
					}
					h1 { margin-bottom: 1rem; font-size: 1.5rem; }
					.success h1 { color: #52c41a; }
					.error h1 { color: #ff4d4f; }
					p { font-size: 1.1rem; line-height: 1.5; margin: 0.5rem 0; }
					.ip-badge {
						display: inline-block;
						background: #e6f4ff;
						color: #0958d9;
						padding: 0.2rem 0.6rem;
						border-radius: 4px;
						font-family: monospace;
						font-weight: bold;
					}
					.hint { font-size: 0.9rem; color: #666; margin-top: 1rem; padding: 10px; background: #fafafa; border-radius: 4px; }
					.next-steps { text-align: left; margin-top: 1.5rem; border-top: 1px solid #eee; padding-top: 1rem; }
					.next-steps h3 { font-size: 1rem; margin-bottom: 0.5rem; }
					.next-steps ul { padding-left: 1.2rem; font-size: 0.9rem; color: #555; }
					.next-steps li { margin-bottom: 0.5rem; }
				</style>
			</head>
			<body class="%s">
				<div class="card">
					<h1>%s</h1>
					<p>手机 IP: <span class="ip-badge">%s</span></p>
					<p>%s</p>
					<div class="hint">%s</div>
					%s
				</div>
			</body>
			</html>
		`, statusClass, title, remoteIP, message, hint, nextSteps)
	})

	a.httpServer = &http.Server{Handler: mux}
	go a.httpServer.Serve(listener)

	return a.localAddr, nil
}

// tryAutoReconnect attempts to reconnect to a wireless device if it's offline
func (a *App) tryAutoReconnect(address string) {
	if address == "" || (!strings.Contains(address, ":") && !strings.Contains(address, "._tcp")) {
		return
	}

	a.reconnectMu.Lock()
	last, ok := a.reconnectCooldown[address]
	if ok && time.Since(last) < 30*time.Second {
		a.reconnectMu.Unlock()
		return
	}
	a.reconnectCooldown[address] = time.Now()
	a.reconnectMu.Unlock()

	go func() {
		a.Log("Auto-reconnecting to wireless device: %s", address)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := a.newAdbCommand(ctx, "connect", address)
		_ = cmd.Run()
	}()
}

// History management functions

func (a *App) loadHistoryInternal() []HistoryDevice {
	var history []HistoryDevice
	if a.historyPath == "" {
		return history
	}
	data, err := os.ReadFile(a.historyPath)
	if err != nil {
		return history
	}
	if err := json.Unmarshal(data, &history); err != nil {
		a.Log("Error unmarshaling history: %v", err)
		return []HistoryDevice{}
	}
	return history
}

func (a *App) saveHistory(history []HistoryDevice) error {
	data, err := json.Marshal(history)
	if err != nil {
		a.Log("Failed to marshal history: %v", err)
		return err
	}
	err = os.WriteFile(a.historyPath, data, 0644)
	if err != nil {
		a.Log("Failed to write history to %s: %v", a.historyPath, err)
		return err
	}
	return nil
}

func (a *App) addToHistory(device Device) {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()

	history := a.loadHistoryInternal()
	found := false
	for i, d := range history {
		if (device.Serial != "" && d.Serial == device.Serial) || d.ID == device.ID {
			history[i].LastSeen = time.Now().Unix()
			history[i].Model = device.Model
			history[i].Brand = device.Brand
			history[i].Type = device.Type
			history[i].Serial = device.Serial
			history[i].WifiAddr = device.WifiAddr
			history[i].ID = device.ID
			found = true
			break
		}
	}

	if !found {
		history = append(history, HistoryDevice{
			ID:       device.ID,
			Serial:   device.Serial,
			Model:    device.Model,
			Brand:    device.Brand,
			Type:     device.Type,
			WifiAddr: device.WifiAddr,
			LastSeen: time.Now().Unix(),
		})
	}

	if len(history) > 20 {
		history = history[len(history)-20:]
	}

	if err := a.saveHistory(history); err != nil {
		a.Log("Failed to save history in addToHistory: %v", err)
	}
}

// GetHistoryDevices returns all devices in history
func (a *App) GetHistoryDevices() []HistoryDevice {
	a.historyMu.Lock()
	defer a.historyMu.Unlock()
	return a.loadHistoryInternal()
}

// RemoveHistoryDevice removes a device from history
func (a *App) RemoveHistoryDevice(deviceId string) error {
	_, _ = a.AdbDisconnect(deviceId)

	a.historyMu.Lock()
	defer a.historyMu.Unlock()

	history := a.loadHistoryInternal()
	var newHistory []HistoryDevice
	for _, d := range history {
		if d.ID != deviceId && d.Serial != deviceId {
			newHistory = append(newHistory, d)
		}
	}
	return a.saveHistory(newHistory)
}

// TogglePinDevice pins/unpins a device by its serial
func (a *App) TogglePinDevice(serial string) {
	a.pinnedMu.Lock()
	if a.pinnedSerial == serial {
		a.pinnedSerial = ""
	} else {
		a.pinnedSerial = serial
	}
	a.pinnedMu.Unlock()

	go a.saveSettings()
}

// StartDeviceMonitor starts monitoring device connections using adb track-devices
// It emits "devices-changed" events when devices connect/disconnect
func (a *App) StartDeviceMonitor() {
	a.deviceMonitorMu.Lock()
	defer a.deviceMonitorMu.Unlock()

	// Stop existing monitor if running
	if a.deviceMonitorCancel != nil {
		a.deviceMonitorCancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.deviceMonitorCancel = cancel

	go a.runDeviceMonitor(ctx)
}

// StopDeviceMonitor stops the device monitor
func (a *App) StopDeviceMonitor() {
	a.deviceMonitorMu.Lock()
	defer a.deviceMonitorMu.Unlock()

	if a.deviceMonitorCancel != nil {
		a.deviceMonitorCancel()
		a.deviceMonitorCancel = nil
	}
}

// runDeviceMonitor runs the device monitoring loop
func (a *App) runDeviceMonitor(ctx context.Context) {
	// Debounce timer to avoid rapid-fire events
	var debounceTimer *time.Timer
	var debounceMu sync.Mutex

	emitDevicesChanged := func() {
		debounceMu.Lock()
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(300*time.Millisecond, func() {
			devices, err := a.GetDevices(false)
			if err != nil {
				a.Log("Device monitor: failed to get devices: %v", err)
				return
			}
			wailsRuntime.EventsEmit(a.ctx, "devices-changed", devices)
		})
		debounceMu.Unlock()
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Start adb track-devices
		cmd := exec.CommandContext(ctx, a.adbPath, "track-devices")
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			a.Log("Device monitor: failed to create pipe: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		if err := cmd.Start(); err != nil {
			a.Log("Device monitor: failed to start: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		a.Log("Device monitor started")

		// Read the track-devices output
		// Format: 4 hex chars (length) followed by device list
		buf := make([]byte, 4)
		for {
			select {
			case <-ctx.Done():
				cmd.Process.Kill()
				return
			default:
			}

			// Read length prefix (4 hex chars)
			_, err := stdout.Read(buf)
			if err != nil {
				break
			}

			var length int
			fmt.Sscanf(string(buf), "%04x", &length)

			if length > 0 {
				// Read device data
				data := make([]byte, length)
				_, err := stdout.Read(data)
				if err != nil {
					break
				}
			}

			// Emit event (debounced)
			emitDevicesChanged()
		}

		cmd.Wait()
		a.Log("Device monitor disconnected, restarting...")
		time.Sleep(1 * time.Second)
	}
}
