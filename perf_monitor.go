package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ========================================
// Performance Monitor - 性能数据采集
// 采集 CPU、内存、FPS、网络流量等指标
// ========================================

// ProcessPerfData 单个进程的性能数据 (任务管理器视图)
type ProcessPerfData struct {
	PID       int     `json:"pid"`
	Name      string  `json:"name"`      // 包名或进程名, e.g. "com.example.app"
	CPU       float64 `json:"cpu"`       // CPU 使用率 %
	MemoryKB  int     `json:"memoryKB"`  // RSS 内存 (KB)
	User      float64 `json:"user"`      // user CPU %
	Kernel    float64 `json:"kernel"`    // kernel CPU %
	LinuxUser string  `json:"linuxUser"` // Linux 用户, e.g. "u0_a123"
	PPID      int     `json:"ppid"`      // 父进程 PID
	VSZKB     int     `json:"vszKB"`     // 虚拟内存 (KB)
	State     string  `json:"state"`     // 进程状态: R=running, S=sleeping, D=disk sleep, Z=zombie, T=stopped
}

// PerfSampleData 性能采样数据
type PerfSampleData struct {
	// CPU
	CPUUsage   float64 `json:"cpuUsage"`   // 总 CPU 使用率 (0-100)
	CPUApp     float64 `json:"cpuApp"`     // 目标应用 CPU 使用率
	CPUCores   int     `json:"cpuCores"`   // CPU 核心数
	CPUFreqMHz int     `json:"cpuFreqMHz"` // 当前 CPU 频率 (MHz)
	CPUTempC   float64 `json:"cpuTempC"`   // CPU 温度 (°C)

	// Memory
	MemTotalMB int     `json:"memTotalMB"` // 总内存 (MB)
	MemUsedMB  int     `json:"memUsedMB"`  // 已用内存 (MB)
	MemFreeMB  int     `json:"memFreeMB"`  // 空闲内存 (MB)
	MemUsage   float64 `json:"memUsage"`   // 内存使用率 (0-100)
	MemAppMB   int     `json:"memAppMB"`   // 目标应用内存 (MB)

	// FPS (via SurfaceFlinger)
	FPS       float64 `json:"fps"`       // 当前帧率
	JankCount int     `json:"jankCount"` // 卡顿帧数

	// Network I/O
	NetRxKBps    float64 `json:"netRxKBps"`    // 接收速率 (KB/s)
	NetTxKBps    float64 `json:"netTxKBps"`    // 发送速率 (KB/s)
	NetRxTotalMB float64 `json:"netRxTotalMB"` // 总接收 (MB)
	NetTxTotalMB float64 `json:"netTxTotalMB"` // 总发送 (MB)

	// Battery
	BatteryLevel int     `json:"batteryLevel"` // 电池电量 (0-100)
	BatteryTemp  float64 `json:"batteryTemp"`  // 电池温度 (°C)

	// Target package (if monitoring specific app)
	PackageName string `json:"packageName,omitempty"`

	// Per-process data (任务管理器视图)
	Processes []ProcessPerfData `json:"processes,omitempty"`
}

// PerfMonitorConfig 性能监控配置
type PerfMonitorConfig struct {
	PackageName   string `json:"packageName,omitempty"` // 目标应用包名 (空表示全局)
	IntervalMs    int    `json:"intervalMs"`            // 采样间隔 (毫秒)
	EnableCPU     bool   `json:"enableCPU"`             // 启用 CPU 采集
	EnableMemory  bool   `json:"enableMemory"`          // 启用内存采集
	EnableFPS     bool   `json:"enableFPS"`             // 启用 FPS 采集
	EnableNetwork bool   `json:"enableNetwork"`         // 启用网络采集
	EnableBattery bool   `json:"enableBattery"`         // 启用电池采集
}

// DefaultPerfConfig 默认性能监控配置
func DefaultPerfConfig() PerfMonitorConfig {
	return PerfMonitorConfig{
		IntervalMs:    2000, // 2秒采样
		EnableCPU:     true,
		EnableMemory:  true,
		EnableFPS:     true,
		EnableNetwork: true,
		EnableBattery: true,
	}
}

// PerfMonitor 性能监控器
type PerfMonitor struct {
	app      *App
	deviceID string
	config   PerfMonitorConfig
	ctx      context.Context
	cancel   context.CancelFunc

	// 上一次采样的网络数据 (用于计算速率)
	lastNetRxBytes int64
	lastNetTxBytes int64
	lastNetTime    time.Time

	// FPS tracking
	lastFrameCount int64
	lastFrameTime  time.Time

	// CPU 核心数是常量, 首次采集成功后缓存, 后续 tick 不再重查
	cpuCores int
}

// ========================================
// PerfMonitor 管理器
// ========================================

var (
	perfMonitors   = make(map[string]*PerfMonitor) // deviceID -> PerfMonitor
	perfMonitorsMu sync.Mutex
)

// StartPerfMonitor 启动性能监控
func (a *App) StartPerfMonitor(deviceID string, config PerfMonitorConfig) string {
	perfMonitorsMu.Lock()
	defer perfMonitorsMu.Unlock()

	// 停止已有的
	if m, ok := perfMonitors[deviceID]; ok {
		m.Stop()
	}

	// 设置默认间隔
	if config.IntervalMs <= 0 {
		config.IntervalMs = 2000
	}
	if config.IntervalMs < 500 {
		config.IntervalMs = 500 // 最小500ms
	}

	ctx, cancel := context.WithCancel(a.ctx)
	monitor := &PerfMonitor{
		app:      a,
		deviceID: deviceID,
		config:   config,
		ctx:      ctx,
		cancel:   cancel,
	}

	perfMonitors[deviceID] = monitor
	go monitor.Start()

	LogInfo("perf_monitor").Str("device", deviceID).Str("package", config.PackageName).Msg("Performance monitor started")
	return "started"
}

// StopPerfMonitor 停止性能监控
func (a *App) StopPerfMonitor(deviceID string) string {
	perfMonitorsMu.Lock()
	defer perfMonitorsMu.Unlock()

	if m, ok := perfMonitors[deviceID]; ok {
		m.Stop()
		delete(perfMonitors, deviceID)
		LogInfo("perf_monitor").Str("device", deviceID).Msg("Performance monitor stopped")
		return "stopped"
	}
	return "not_running"
}

// IsPerfMonitorRunning 检查性能监控是否正在运行
func (a *App) IsPerfMonitorRunning(deviceID string) bool {
	perfMonitorsMu.Lock()
	defer perfMonitorsMu.Unlock()
	_, ok := perfMonitors[deviceID]
	return ok
}

// GetPerfMonitorConfig 获取当前性能监控配置
func (a *App) GetPerfMonitorConfig(deviceID string) *PerfMonitorConfig {
	perfMonitorsMu.Lock()
	defer perfMonitorsMu.Unlock()
	if m, ok := perfMonitors[deviceID]; ok {
		return &m.config
	}
	return nil
}

// GetPerfSnapshot 获取一次性能快照 (不需要启动持续监控)
func (a *App) GetPerfSnapshot(deviceID string, packageName string) (*PerfSampleData, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	sample := &PerfSampleData{
		PackageName: packageName,
	}

	// 并行采集所有指标
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string

	collectFn := func(name string, fn func(context.Context, *PerfSampleData)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					errs = append(errs, fmt.Sprintf("%s: panic: %v", name, r))
					mu.Unlock()
				}
			}()
			fn(ctx, sample)
		}()
	}

	collectFn("cpu", func(c context.Context, s *PerfSampleData) {
		a.collectCPU(c, deviceID, packageName, s)
		s.CPUCores = a.queryCPUCores(c, deviceID)
	})
	collectFn("memory", func(c context.Context, s *PerfSampleData) {
		a.collectMemory(c, deviceID, packageName, s)
	})
	collectFn("network", func(c context.Context, s *PerfSampleData) {
		a.collectNetworkStats(c, deviceID, s)
	})
	collectFn("battery", func(c context.Context, s *PerfSampleData) {
		a.collectBattery(c, deviceID, s)
	})

	wg.Wait()

	if len(errs) > 0 {
		LogWarn("perf_monitor").Strs("errors", errs).Msg("Some perf collectors had errors")
	}

	return sample, nil
}

// StopAllPerfMonitors 停止所有性能监控 (应用关闭时调用)
func (a *App) StopAllPerfMonitors() {
	perfMonitorsMu.Lock()
	defer perfMonitorsMu.Unlock()

	for id, m := range perfMonitors {
		m.Stop()
		LogInfo("perf_monitor").Str("device", id).Msg("Stopped perf monitor on shutdown")
	}
	perfMonitors = make(map[string]*PerfMonitor)
}

// ========================================
// PerfMonitor 实例方法
// ========================================

// Start 启动采集循环
func (m *PerfMonitor) Start() {
	interval := time.Duration(m.config.IntervalMs) * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 立即采集一次
	m.collect()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.collect()
		}
	}
}

// Stop 停止采集
func (m *PerfMonitor) Stop() {
	m.cancel()
}

// collect 执行一次数据采集
// 所有采集器并行执行，各自写入 sample 的不同字段。
// wg.Wait() 提供 happens-before 同步保证。
// 轻量命令 (meminfo / net dev / battery / ps -A) 合并为一条 adb shell 执行，
// 把每 tick 的 adb 子进程数从最多 9 个降到 4-5 个。
func (m *PerfMonitor) collect() {
	ctx, cancel := context.WithTimeout(m.ctx, time.Duration(m.config.IntervalMs)*time.Millisecond)
	defer cancel()

	sample := &PerfSampleData{
		PackageName: m.config.PackageName,
	}

	var wg sync.WaitGroup

	launch := func(name string, fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					LogWarn("perf_monitor").Str("collector", name).Msgf("Panic: %v", r)
				}
			}()
			fn()
		}()
	}

	// 用于进程列表合并的共享数据 (各 goroutine 写入不同变量, wg.Wait 后读取)
	var cpuinfoText string
	var psText string

	// CPU (同时捕获 cpuinfo 原始文本用于进程列表)
	if m.config.EnableCPU {
		launch("cpu", func() {
			cpuinfoText = m.app.collectCPU(ctx, m.deviceID, m.config.PackageName, sample)
		})
		// 核心数是常量, 只在首次采集时查询一次并缓存
		if m.cpuCores == 0 {
			launch("cpu_cores", func() {
				m.cpuCores = m.app.queryCPUCores(ctx, m.deviceID)
			})
		}
	}

	// 目标应用内存 (dumpsys meminfo <pkg> 较重, 单独执行)
	if m.config.EnableMemory && m.config.PackageName != "" {
		launch("app_memory", func() {
			m.app.collectAppMemory(ctx, m.deviceID, m.config.PackageName, sample)
		})
	}

	// FPS
	if m.config.EnableFPS {
		launch("fps", func() {
			m.collectFPS(ctx, sample)
		})
	}

	// 合并的轻量命令: 系统内存 + 网络 + 电池 + 进程列表 (ps -A)
	launch("combined", func() {
		psText = m.collectCombined(ctx, sample)
	})

	wg.Wait()

	if m.config.EnableCPU {
		sample.CPUCores = m.cpuCores
	}

	// 合并 CPU + RSS 数据构建进程列表
	sample.Processes = buildProcessList(cpuinfoText, psText)

	// 发送 perf_sample 事件
	m.emitPerfEvent(sample)
}

// perfSegmentSeparator 合并 shell 命令各段输出之间的分隔符
const perfSegmentSeparator = "---GAZE---"

// collectCombined 将多个轻量采集命令合并为一条 adb shell 执行,
// 按分隔符拆分输出后分发给现有解析器 (解析器签名不变)。
// 返回 ps -A 输出 (供 buildProcessList 使用)。
func (m *PerfMonitor) collectCombined(ctx context.Context, sample *PerfSampleData) string {
	type segment struct {
		name  string
		shell string
	}
	var segments []segment
	if m.config.EnableMemory {
		segments = append(segments, segment{"meminfo", "cat /proc/meminfo"})
	}
	if m.config.EnableNetwork {
		segments = append(segments, segment{"netdev", "cat /proc/net/dev"})
	}
	if m.config.EnableBattery {
		segments = append(segments, segment{"battery", "dumpsys battery"})
	}
	// 进程 RSS 内存 (ps -A, 快速 ~100ms, 用于任务管理器视图)
	segments = append(segments, segment{"ps", "ps -A"})

	cmds := make([]string, len(segments))
	for i, s := range segments {
		cmds[i] = s.shell
	}
	// 用 ; 串联, 单段失败不影响后续段和分隔符输出
	shellCmd := strings.Join(cmds, "; echo "+perfSegmentSeparator+"; ")

	cmd := m.app.newAdbCommand(ctx, "-s", m.deviceID, "shell", shellCmd)
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return ""
	}

	parts := strings.Split(string(output), perfSegmentSeparator)
	psText := ""
	for i, s := range segments {
		if i >= len(parts) {
			break
		}
		text := parts[i]
		switch s.name {
		case "meminfo":
			parseMemInfo(text, sample)
		case "netdev":
			totalRx, totalTx := parseNetworkDev(text)
			sample.NetRxTotalMB = float64(totalRx) / (1024 * 1024)
			sample.NetTxTotalMB = float64(totalTx) / (1024 * 1024)
			m.updateNetworkRates(sample)
		case "battery":
			parseBatteryOutput(text, sample)
		case "ps":
			psText = text
		}
	}
	return psText
}

// emitPerfEvent 发送性能采样事件
func (m *PerfMonitor) emitPerfEvent(sample *PerfSampleData) {
	if m.app.eventPipeline == nil {
		return
	}

	title := fmt.Sprintf("CPU: %.1f%% | Mem: %.1f%% | FPS: %.0f",
		sample.CPUUsage, sample.MemUsage, sample.FPS)

	if sample.PackageName != "" {
		title = fmt.Sprintf("[%s] CPU: %.1f%% | Mem: %dMB | FPS: %.0f",
			sample.PackageName, sample.CPUApp, sample.MemAppMB, sample.FPS)
	}

	data, _ := json.Marshal(sample)

	m.app.eventPipeline.Emit(UnifiedEvent{
		DeviceID:  m.deviceID,
		Timestamp: time.Now().UnixMilli(),
		Source:    SourcePerf,
		Category:  CategoryDiagnostic,
		Type:      "perf_sample",
		Level:     LevelDebug,
		Title:     title,
		Data:      data,
	})
}

// ========================================
// Data Collectors
// ========================================

// collectCPU 采集 CPU 使用率 (使用 dumpsys cpuinfo，更可靠)
// 返回 dumpsys cpuinfo 原始输出，供 buildProcessList 复用（避免重复执行命令）
func (a *App) collectCPU(ctx context.Context, deviceID, packageName string, sample *PerfSampleData) string {
	// dumpsys cpuinfo 输出格式:
	//   111% 20805/app.footos: 75% user + 35% kernel / faults: ...
	//   ...
	//   46% TOTAL: 22% user + 18% kernel + 2.8% iowait + ...
	cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell", "dumpsys cpuinfo")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	text := string(output)

	// 解析总 CPU 和 app CPU
	sample.CPUUsage = parseCPUFromDumpsys(text)
	if packageName != "" {
		sample.CPUApp = parseAppCPUFromDumpsys(text, packageName)
	}

	return text
}

// queryCPUCores 查询 CPU 核心数。该值是设备常量, 调用方应缓存结果避免每 tick 重查
func (a *App) queryCPUCores(ctx context.Context, deviceID string) int {
	cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell",
		"cat /proc/cpuinfo | grep processor | wc -l")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}
	cores, _ := strconv.Atoi(strings.TrimSpace(string(output)))
	return cores
}

// collectMemory 采集内存信息 (GetPerfSnapshot 单次快照用; PerfMonitor 周期采集走 collectCombined)
func (a *App) collectMemory(ctx context.Context, deviceID, packageName string, sample *PerfSampleData) {
	// 获取总内存信息
	cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell", "cat /proc/meminfo")
	output, err := cmd.Output()
	if err == nil {
		parseMemInfo(string(output), sample)
	}

	// 获取目标应用内存 (如果指定了包名)
	if packageName != "" {
		a.collectAppMemory(ctx, deviceID, packageName, sample)
	}
}

// collectAppMemory 采集目标应用内存 (dumpsys meminfo <pkg>)
// 注意: 不使用 shell grep 管道，因为 Android toybox grep 不支持 \| 交替匹配，
// 会导致 grep 返回 exit 1 → cmd.Output() 返回 error → 跳过解析 → memAppMB 永远是 0
// 改为: 直接获取完整 dumpsys meminfo 输出，在 Go 中解析
func (a *App) collectAppMemory(ctx context.Context, deviceID, packageName string, sample *PerfSampleData) {
	cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell",
		fmt.Sprintf("dumpsys meminfo %s", packageName))
	output, err := cmd.Output()
	if err == nil {
		sample.MemAppMB = parseAppMemory(string(output))
	}
}

// collectFPS 采集 FPS (通过 dumpsys gfxinfo)
// SurfaceFlinger 1013 在非 root 设备上返回 "Operation not permitted"，不使用
// 改用 dumpsys gfxinfo 的 framestats，通过连续两次采样间的帧数差计算实时 FPS
func (m *PerfMonitor) collectFPS(ctx context.Context, sample *PerfSampleData) {
	// 确定目标包名: 优先使用用户指定的包名，否则检测前台应用
	target := m.config.PackageName
	if target == "" {
		target = m.detectForegroundPackage(ctx)
	}
	if target == "" {
		return // 无法确定目标，跳过 FPS 采集
	}

	// 使用 dumpsys gfxinfo 获取帧信息 (不用 grep，直接在 Go 解析)
	cmd := m.app.newAdbCommand(ctx, "-s", m.deviceID, "shell",
		fmt.Sprintf("dumpsys gfxinfo %s", target))
	output, err := cmd.Output()
	if err != nil {
		return
	}

	text := string(output)
	totalFrames, jankyFrames := parseGfxInfoFrameCounts(text)

	now := time.Now()
	sample.JankCount = jankyFrames

	// 通过两次采样间的帧数差计算实时 FPS
	if m.lastFrameTime.IsZero() || m.lastFrameCount == 0 {
		m.lastFrameCount = int64(totalFrames)
		m.lastFrameTime = now
		return
	}

	elapsed := now.Sub(m.lastFrameTime).Seconds()
	if elapsed > 0 {
		frameDiff := int64(totalFrames) - m.lastFrameCount
		if frameDiff > 0 {
			sample.FPS = float64(frameDiff) / elapsed
			// 合理范围过滤: 0~120 fps
			if sample.FPS > 120 {
				sample.FPS = 0
			}
		}
		// frameDiff == 0 表示没有新帧渲染 (app idle), FPS = 0 是合理的
		// frameDiff < 0 表示 gfxinfo 被重置 (如 app 重启), 跳过此样本
	}

	m.lastFrameCount = int64(totalFrames)
	m.lastFrameTime = now
}

// detectForegroundPackage 检测前台应用包名
// 通过 dumpsys window 的 mCurrentFocus 获取当前焦点窗口
// 输出格式: mCurrentFocus=Window{ab3a179 u0 app.footos/app.footos.MainActivity}
func (m *PerfMonitor) detectForegroundPackage(ctx context.Context) string {
	cmd := m.app.newAdbCommand(ctx, "-s", m.deviceID, "shell",
		"dumpsys window displays | grep mCurrentFocus")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return parseForegroundPackage(string(output))
}

// parseForegroundPackage 从 mCurrentFocus 输出解析包名
// 输入: "  mCurrentFocus=Window{ab3a179 u0 app.footos/app.footos.MainActivity}"
// 输出: "app.footos"
func parseForegroundPackage(output string) string {
	// 匹配 mCurrentFocus=Window{... <package>/<activity>}
	re := regexp.MustCompile(`mCurrentFocus=Window\{[^}]*\s+(\S+)/\S+\}`)
	m := re.FindStringSubmatch(output)
	if len(m) >= 2 {
		return m[1]
	}
	// Fallback: 有些情况下格式可能是 mCurrentFocus=Window{... <package>}
	re2 := regexp.MustCompile(`mCurrentFocus=Window\{[^}]*\s+(\S+)\}`)
	m2 := re2.FindStringSubmatch(output)
	if len(m2) >= 2 {
		pkg := m2[1]
		// 过滤掉非包名的情况 (如 "StatusBar" 等)
		if strings.Contains(pkg, ".") {
			return pkg
		}
	}
	return ""
}

// updateNetworkRates 基于上次采样计算网络速率 (需先填充 sample 的 NetRx/TxTotalMB)
// 注意: lastNetRxBytes / lastNetTxBytes 存储的是原始字节数,
// sample.NetRxTotalMB / NetTxTotalMB 是 MB 为单位.
// 速率计算需要统一使用字节数才能得到准确的 KB/s.
func (m *PerfMonitor) updateNetworkRates(sample *PerfSampleData) {
	// 从 MB 还原为字节数以计算精确速率
	currentRxBytes := int64(sample.NetRxTotalMB * 1024 * 1024)
	currentTxBytes := int64(sample.NetTxTotalMB * 1024 * 1024)

	now := time.Now()

	// 计算速率 (KB/s)
	if !m.lastNetTime.IsZero() {
		elapsed := now.Sub(m.lastNetTime).Seconds()
		if elapsed > 0 {
			rxDiffBytes := currentRxBytes - m.lastNetRxBytes
			txDiffBytes := currentTxBytes - m.lastNetTxBytes
			if rxDiffBytes >= 0 {
				sample.NetRxKBps = float64(rxDiffBytes) / 1024 / elapsed
			}
			if txDiffBytes >= 0 {
				sample.NetTxKBps = float64(txDiffBytes) / 1024 / elapsed
			}
		}
	}

	m.lastNetRxBytes = currentRxBytes
	m.lastNetTxBytes = currentTxBytes
	m.lastNetTime = now
}

// collectNetworkStats 采集网络流量统计 (无速率)
func (a *App) collectNetworkStats(ctx context.Context, deviceID string, sample *PerfSampleData) {
	// 读取 /proc/net/dev 获取网络流量
	cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell",
		"cat /proc/net/dev")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	totalRx, totalTx := parseNetworkDev(string(output))
	sample.NetRxTotalMB = float64(totalRx) / (1024 * 1024)
	sample.NetTxTotalMB = float64(totalTx) / (1024 * 1024)
}

// collectBattery 采集电池信息 (GetPerfSnapshot 单次快照用; PerfMonitor 周期采集走 collectCombined)
func (a *App) collectBattery(ctx context.Context, deviceID string, sample *PerfSampleData) {
	cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell", "dumpsys battery")
	output, err := cmd.Output()
	if err != nil {
		return
	}
	parseBatteryOutput(string(output), sample)
}

// parseBatteryOutput 解析 dumpsys battery 输出，同时用电池温度作为设备温度近似值
func parseBatteryOutput(output string, sample *PerfSampleData) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "level:") {
			fmt.Sscanf(line, "level: %d", &sample.BatteryLevel)
		} else if strings.HasPrefix(line, "temperature:") {
			var temp int
			fmt.Sscanf(line, "temperature: %d", &temp)
			sample.BatteryTemp = float64(temp) / 10.0
			// 如果还没有 CPU 温度，用电池温度作为近似值
			// (电池温度接近 SoC 温度，通常差距在 2-5°C)
			if sample.CPUTempC == 0 {
				sample.CPUTempC = sample.BatteryTemp
			}
		}
	}
}

// ========================================
// Parsers
// ========================================

// parseCPUFromDumpsys 从 dumpsys cpuinfo 解析总 CPU 使用率
// 匹配最后一行: "46% TOTAL: 22% user + 18% kernel + ..."
func parseCPUFromDumpsys(output string) float64 {
	re := regexp.MustCompile(`(?m)^\s*(\d+(?:\.\d+)?)%\s+TOTAL:`)
	m := re.FindStringSubmatch(output)
	if len(m) >= 2 {
		val, _ := strconv.ParseFloat(m[1], 64)
		return val
	}
	return 0
}

// parseAppCPUFromDumpsys 从 dumpsys cpuinfo 解析特定应用的 CPU 使用率
// 匹配: "  10% 14778/com.google.android.gms: 5.8% user + 5% kernel"
func parseAppCPUFromDumpsys(output, packageName string) float64 {
	// 匹配包含指定包名的行
	lines := strings.Split(output, "\n")
	var total float64
	re := regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)%\s+\d+/`)

	for _, line := range lines {
		if !strings.Contains(line, packageName) {
			continue
		}
		m := re.FindStringSubmatch(line)
		if len(m) >= 2 {
			val, _ := strconv.ParseFloat(m[1], 64)
			total += val // 累加同一个包的多个进程 (如 :sandboxed_process)
		}
	}
	return total
}

// parseMemInfo 从 /proc/meminfo 解析内存信息
func parseMemInfo(output string, sample *PerfSampleData) {
	lines := strings.Split(output, "\n")
	var total, free, buffers, cached, available int64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		val, _ := strconv.ParseInt(parts[1], 10, 64) // kB

		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			total = val
		case strings.HasPrefix(line, "MemFree:"):
			free = val
		case strings.HasPrefix(line, "Buffers:"):
			buffers = val
		case strings.HasPrefix(line, "Cached:"):
			cached = val
		case strings.HasPrefix(line, "MemAvailable:"):
			available = val
		}
	}

	sample.MemTotalMB = int(total / 1024)
	if available > 0 {
		sample.MemFreeMB = int(available / 1024)
	} else {
		sample.MemFreeMB = int((free + buffers + cached) / 1024)
	}
	sample.MemUsedMB = sample.MemTotalMB - sample.MemFreeMB
	if sample.MemTotalMB > 0 {
		sample.MemUsage = float64(sample.MemUsedMB) / float64(sample.MemTotalMB) * 100
	}
}

// parseAppMemory 从 dumpsys meminfo 完整输出解析应用内存 (PSS, KB)
// 匹配以下两种格式 (出现在 dumpsys meminfo <package> 输出中):
//
//	"TOTAL   100386     1856    15060    64499   142204 ..."
//	"           TOTAL PSS:   100386            TOTAL RSS: ..."
//
// 优先匹配 "TOTAL PSS:" 格式 (更精确), 回退到 "TOTAL  <number>" 格式
func parseAppMemory(output string) int {
	// 优先: "TOTAL PSS:   100386" (summary 行)
	rePSS := regexp.MustCompile(`TOTAL\s+PSS:\s+(\d+)`)
	mPSS := rePSS.FindStringSubmatch(output)
	if len(mPSS) >= 2 {
		kb, _ := strconv.Atoi(mPSS[1])
		return kb / 1024 // 转 MB
	}

	// 回退: "TOTAL   100386   ..." (详情表的 TOTAL 行, 第一个数字是 PSS)
	// 需要排除 "TOTAL PSS:" / "TOTAL RSS:" / "TOTAL SWAP" 等 summary 行
	reTotal := regexp.MustCompile(`(?m)^\s*TOTAL\s+(\d+)\s`)
	mTotal := reTotal.FindStringSubmatch(output)
	if len(mTotal) >= 2 {
		kb, _ := strconv.Atoi(mTotal[1])
		return kb / 1024
	}
	return 0
}

// parseGfxInfoFrameCounts 从 dumpsys gfxinfo 完整输出解析帧计数
// 返回 (totalFrames, jankyFrames)
// 输出格式:
//
//	Total frames rendered: 12345
//	Janky frames: 678 (5.49%)
func parseGfxInfoFrameCounts(output string) (int, int) {
	var totalFrames, jankyFrames int
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Total frames rendered:") {
			fmt.Sscanf(line, "Total frames rendered: %d", &totalFrames)
		} else if strings.HasPrefix(line, "Janky frames:") {
			fmt.Sscanf(line, "Janky frames: %d", &jankyFrames)
		}
	}
	return totalFrames, jankyFrames
}

// parseNetworkDev 从 /proc/net/dev 输出解析网络流量 (字节数)
// 返回 (totalRxBytes, totalTxBytes), 仅计入 wlan/rmnet/eth/ccmni 接口
func parseNetworkDev(output string) (int64, int64) {
	var totalRx, totalTx int64
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// 跳过 lo 回环接口和非数据行
		if strings.HasPrefix(line, "lo:") || !strings.Contains(line, ":") {
			continue
		}
		// 只关心 wlan0、rmnet (移动数据)、eth、ccmni 接口
		if strings.HasPrefix(line, "wlan") || strings.HasPrefix(line, "rmnet") ||
			strings.HasPrefix(line, "eth") || strings.HasPrefix(line, "ccmni") {
			rx, tx := parseNetDevLine(line)
			totalRx += rx
			totalTx += tx
		}
	}
	return totalRx, totalTx
}

// parseNetDevLine 解析 /proc/net/dev 单行
// 格式: "wlan0: 247816032  242814  0  1  0  0  0  0 100640622  157184 ..."
// rx_bytes 在 : 后第1个字段, tx_bytes 在第9个字段 (0-indexed from interface:)
func parseNetDevLine(line string) (rxBytes, txBytes int64) {
	parts := strings.Fields(line)
	if len(parts) < 10 {
		return 0, 0
	}
	// 处理 "wlan0:247816032" (冒号紧跟数字) vs "wlan0: 247816032" (冒号后有空格)
	colonIdx := strings.Index(parts[0], ":")
	if colonIdx < 0 {
		return 0, 0
	}
	var rxStr, txStr string
	if colonIdx < len(parts[0])-1 {
		// bytes 紧跟在 : 后面, 如 "wlan0:247816032"
		rxStr = parts[0][colonIdx+1:]
		if len(parts) >= 9 {
			txStr = parts[8]
		}
	} else {
		// : 后是空格, 如 "wlan0:" 然后 "247816032" 是 parts[1]
		if len(parts) >= 10 {
			rxStr = parts[1]
			txStr = parts[9]
		}
	}
	rxBytes, _ = strconv.ParseInt(rxStr, 10, 64)
	txBytes, _ = strconv.ParseInt(txStr, 10, 64)
	return
}

// ========================================
// Process List (任务管理器)
// ========================================

// cpuProcessEntry dumpsys cpuinfo 解析后的进程条目 (内部使用)
type cpuProcessEntry struct {
	PID    int
	Name   string
	Total  float64
	User   float64
	Kernel float64
}

// parseAllProcessCPU 从 dumpsys cpuinfo 解析所有进程的 CPU 使用率
// 输入格式:
//
//	111% 20805/app.footos: 75% user + 35% kernel / faults: 13442 minor
//	48% 20855/com.google.android.webview:sandboxed_process0:...: 40% user + 8.5% kernel
//	0.1% 14778/com.google.android.gms: 0.1% user + 0% kernel
//	0.3% 881/surfaceflinger: 0.1% user + 0.1% kernel
func parseAllProcessCPU(output string) []cpuProcessEntry {
	var result []cpuProcessEntry
	re := regexp.MustCompile(`^\s*(\d+(?:\.\d+)?)%\s+(\d+)/(\S+?):\s+(\d+(?:\.\d+)?)%\s+user\s+\+\s+(\d+(?:\.\d+)?)%\s+kernel`)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		m := re.FindStringSubmatch(line)
		if len(m) < 6 {
			continue
		}
		total, _ := strconv.ParseFloat(m[1], 64)
		pid, _ := strconv.Atoi(m[2])
		name := m[3]
		user, _ := strconv.ParseFloat(m[4], 64)
		kernel, _ := strconv.ParseFloat(m[5], 64)

		if pid > 0 {
			result = append(result, cpuProcessEntry{
				PID:    pid,
				Name:   name,
				Total:  total,
				User:   user,
				Kernel: kernel,
			})
		}
	}
	return result
}

// psEntry ps -A 解析后的进程条目 (内部使用)
type psEntry struct {
	PID       int
	RSS       int // KB
	Name      string
	LinuxUser string // Linux 用户, e.g. "u0_a123"
	PPID      int    // 父进程 PID
	VSZ       int    // 虚拟内存 KB
	State     string // R/S/D/Z/T
}

// parsePSOutput 解析 ps -A 输出, 返回 PID → psEntry 映射
// 输入格式 (Android toybox):
//
//	USER           PID  PPID     VSZ    RSS WCHAN            ADDR S NAME
//	root             1     0   47660  10340 do_epoll_+          0 S init
//	u0_a123      12345  1234 1234567 123456 do_epoll_+          0 S com.example.app
//
// PID=field[1], RSS=field[4], NAME=最后一个 field
func parsePSOutput(output string) map[int]psEntry {
	result := make(map[int]psEntry)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "USER") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || pid <= 0 {
			continue
		}
		ppid, _ := strconv.Atoi(fields[2])
		vsz, _ := strconv.Atoi(fields[3])
		rss, _ := strconv.Atoi(fields[4])
		state := fields[7] // S=sleeping, R=running, D=disk sleep, Z=zombie, T=stopped
		name := fields[len(fields)-1]

		result[pid] = psEntry{
			PID:       pid,
			RSS:       rss,
			Name:      name,
			LinuxUser: fields[0],
			PPID:      ppid,
			VSZ:       vsz,
			State:     state,
		}
	}
	return result
}

// buildProcessList 合并 CPU 和 RSS 数据，生成进程性能列表
// 只返回 "应用级" 进程 (包名含 "." 的)，排除原生进程 (init, surfaceflinger 等)
func buildProcessList(cpuinfoText, psText string) []ProcessPerfData {
	cpuEntries := parseAllProcessCPU(cpuinfoText)
	rssMap := parsePSOutput(psText)

	pidSeen := make(map[int]bool)
	var result []ProcessPerfData

	// 1. 从 CPU 数据构建进程列表 (这些是最近使用了 CPU 的进程)
	for _, e := range cpuEntries {
		if !isAppProcess(e.Name) {
			continue
		}
		p := ProcessPerfData{
			PID:    e.PID,
			Name:   e.Name,
			CPU:    e.Total,
			User:   e.User,
			Kernel: e.Kernel,
		}
		if ps, ok := rssMap[e.PID]; ok {
			p.MemoryKB = ps.RSS
			p.LinuxUser = ps.LinuxUser
			p.PPID = ps.PPID
			p.VSZKB = ps.VSZ
			p.State = ps.State
		}
		result = append(result, p)
		pidSeen[e.PID] = true
	}

	// 2. 添加有内存占用但 0% CPU 的应用进程 (空闲但在后台)
	for _, ps := range rssMap {
		if pidSeen[ps.PID] || !isAppProcess(ps.Name) || ps.RSS == 0 {
			continue
		}
		result = append(result, ProcessPerfData{
			PID:       ps.PID,
			Name:      ps.Name,
			MemoryKB:  ps.RSS,
			LinuxUser: ps.LinuxUser,
			PPID:      ps.PPID,
			VSZKB:     ps.VSZ,
			State:     ps.State,
		})
	}

	// 按 CPU 降序排序 (CPU 相同则按内存降序)
	sortProcessList(result)
	// 截断为 top N, 降低 perf_sample 事件每 2 秒写入 SQLite 的 JSON 体积
	if len(result) > maxProcessListEntries {
		result = result[:maxProcessListEntries]
	}
	return result
}

// maxProcessListEntries perf_sample 进程列表的最大条目数 (已按 CPU/内存降序排列)
const maxProcessListEntries = 50

// isAppProcess 判断进程名是否为应用进程 (包含 "." 的 Java 包名)
func isAppProcess(name string) bool {
	return strings.Contains(name, ".")
}

// sortProcessList 按 CPU 降序排序，CPU 相同按内存降序
func sortProcessList(list []ProcessPerfData) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0; j-- {
			if list[j].CPU > list[j-1].CPU ||
				(list[j].CPU == list[j-1].CPU && list[j].MemoryKB > list[j-1].MemoryKB) {
				list[j], list[j-1] = list[j-1], list[j]
			} else {
				break
			}
		}
	}
}

// ========================================
// Process Detail (按需加载)
// ========================================

// ProcessMemoryCategory 内存分类条目 (来自 dumpsys meminfo App Summary)
type ProcessMemoryCategory struct {
	Name  string `json:"name"`  // 分类名: Java Heap, Native Heap, Code, Stack, Graphics, Private Other, System
	PssKB int    `json:"pssKB"` // PSS (Proportional Set Size) KB
	RssKB int    `json:"rssKB"` // RSS KB (部分分类可能为 0)
}

// ProcessObjects Android 对象统计 (来自 dumpsys meminfo Objects 段)
type ProcessObjects struct {
	Views           int `json:"views"`
	ViewRootImpl    int `json:"viewRootImpl"`
	AppContexts     int `json:"appContexts"`
	Activities      int `json:"activities"`
	Assets          int `json:"assets"`
	AssetManagers   int `json:"assetManagers"`
	LocalBinders    int `json:"localBinders"`
	ProxyBinders    int `json:"proxyBinders"`
	DeathRecipients int `json:"deathRecipients"`
	WebViews        int `json:"webViews"`
}

// ProcessDetail 进程详细信息 (按需获取，合并 dumpsys meminfo + /proc/status)
type ProcessDetail struct {
	PID         int    `json:"pid"`
	PackageName string `json:"packageName"`
	// Memory summary
	TotalPSSKB int `json:"totalPssKB"`
	TotalRSSKB int `json:"totalRssKB"`
	SwapPSSKB  int `json:"swapPssKB"`
	// Memory breakdown
	Memory []ProcessMemoryCategory `json:"memory"`
	// Heap detail
	JavaHeapSizeKB    int `json:"javaHeapSizeKB"`
	JavaHeapAllocKB   int `json:"javaHeapAllocKB"`
	JavaHeapFreeKB    int `json:"javaHeapFreeKB"`
	NativeHeapSizeKB  int `json:"nativeHeapSizeKB"`
	NativeHeapAllocKB int `json:"nativeHeapAllocKB"`
	NativeHeapFreeKB  int `json:"nativeHeapFreeKB"`
	// Objects
	Objects ProcessObjects `json:"objects"`
	// Process info (from /proc/<pid>/status)
	Threads     int `json:"threads"`
	FDSize      int `json:"fdSize"`
	VmSwapKB    int `json:"vmSwapKB"`
	OomScoreAdj int `json:"oomScoreAdj"`
	UID         int `json:"uid"`
}

// GetProcessDetail 按需获取进程详细信息 (dumpsys meminfo + /proc/status)
// 注意: dumpsys meminfo 较慢 (2-3秒)，仅在用户点击时调用
func (a *App) GetProcessDetail(deviceID string, pid int) (*ProcessDetail, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()

	detail := &ProcessDetail{PID: pid}

	var wg sync.WaitGroup
	var meminfoOut, statusOut string

	// 并行获取三个数据源
	wg.Add(3)
	go func() {
		defer wg.Done()
		cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell", fmt.Sprintf("dumpsys meminfo %d", pid))
		out, err := cmd.Output()
		if err == nil {
			meminfoOut = string(out)
		}
	}()
	go func() {
		defer wg.Done()
		cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell", fmt.Sprintf("cat /proc/%d/status", pid))
		out, err := cmd.Output()
		if err == nil {
			statusOut = string(out)
		}
	}()
	go func() {
		defer wg.Done()
		cmd := a.newAdbCommand(ctx, "-s", deviceID, "shell", fmt.Sprintf("cat /proc/%d/oom_score_adj", pid))
		out, err := cmd.Output()
		if err == nil {
			detail.OomScoreAdj, _ = strconv.Atoi(strings.TrimSpace(string(out)))
		}
	}()

	wg.Wait()

	// 解析 dumpsys meminfo
	parseMeminfoDump(meminfoOut, detail)

	// 解析 /proc/status
	parseProcStatus(statusOut, detail)

	return detail, nil
}

// parseMeminfoDump 解析 dumpsys meminfo <pid> 输出
// 提取 App Summary 段的内存分类 + TOTAL 行 + Heap 行 + Objects 段
func parseMeminfoDump(output string, detail *ProcessDetail) {
	lines := strings.Split(output, "\n")

	inAppSummary := false
	inObjects := false
	inMainTable := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// App Summary 段
		if strings.HasPrefix(trimmed, "App Summary") {
			inAppSummary = true
			inObjects = false
			inMainTable = false
			continue
		}
		// Objects 段
		if strings.HasPrefix(trimmed, "Objects") {
			inObjects = true
			inAppSummary = false
			inMainTable = false
			continue
		}
		// Main table header (MEMINFO in pid)
		if strings.Contains(trimmed, "MEMINFO in pid") {
			inMainTable = true
			// 提取包名: ** MEMINFO in pid 27172 [app.footos] **
			if idx := strings.Index(trimmed, "["); idx >= 0 {
				if end := strings.Index(trimmed[idx:], "]"); end >= 0 {
					detail.PackageName = trimmed[idx+1 : idx+end]
				}
			}
			continue
		}

		// 解析 App Summary 段
		if inAppSummary {
			parseAppSummaryLine(trimmed, detail)
		}

		// 解析 Objects 段
		if inObjects {
			parseObjectsLine(trimmed, detail)
		}

		// 解析主表的 Heap 信息 + TOTAL 行
		if inMainTable {
			parseMainTableLine(trimmed, detail)
		}
	}
}

// parseAppSummaryLine 解析 App Summary 中的单行
// 格式: "Java Heap:    21028                          33280"
//
//	"TOTAL PSS:   431653            TOTAL RSS:   585360       TOTAL SWAP PSS:      132"
func parseAppSummaryLine(line string, detail *ProcessDetail) {
	// TOTAL 行
	if strings.HasPrefix(line, "TOTAL PSS:") {
		re := regexp.MustCompile(`TOTAL PSS:\s+(\d+)`)
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			detail.TotalPSSKB, _ = strconv.Atoi(m[1])
		}
		re2 := regexp.MustCompile(`TOTAL RSS:\s+(\d+)`)
		if m := re2.FindStringSubmatch(line); len(m) >= 2 {
			detail.TotalRSSKB, _ = strconv.Atoi(m[1])
		}
		re3 := regexp.MustCompile(`TOTAL SWAP PSS:\s+(\d+)`)
		if m := re3.FindStringSubmatch(line); len(m) >= 2 {
			detail.SwapPSSKB, _ = strconv.Atoi(m[1])
		}
		return
	}

	// 普通分类行: "Java Heap:    21028                          33280"
	// 或无 RSS: "Private Other:    30316"
	re := regexp.MustCompile(`^(.+?):\s+(\d+)\s+(\d+)?`)
	m := re.FindStringSubmatch(line)
	if len(m) < 3 {
		return
	}
	name := strings.TrimSpace(m[1])
	pss, _ := strconv.Atoi(m[2])

	// 跳过非分类行 (表头 Pss(KB), 分隔线 ------)
	if name == "Pss(KB)" || strings.HasPrefix(name, "---") {
		return
	}

	cat := ProcessMemoryCategory{Name: name, PssKB: pss}
	if len(m) >= 4 && m[3] != "" {
		cat.RssKB, _ = strconv.Atoi(m[3])
	}
	detail.Memory = append(detail.Memory, cat)
}

// parseObjectsLine 解析 Objects 段的单行
// 格式: "Views:       10         ViewRootImpl:        1"
func parseObjectsLine(line string, detail *ProcessDetail) {
	re := regexp.MustCompile(`(\w[\w\s]*?):\s+(\d+)`)
	matches := re.FindAllStringSubmatch(line, -1)
	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		val, _ := strconv.Atoi(m[2])
		switch name {
		case "Views":
			detail.Objects.Views = val
		case "ViewRootImpl":
			detail.Objects.ViewRootImpl = val
		case "AppContexts":
			detail.Objects.AppContexts = val
		case "Activities":
			detail.Objects.Activities = val
		case "Assets":
			detail.Objects.Assets = val
		case "AssetManagers":
			detail.Objects.AssetManagers = val
		case "Local Binders":
			detail.Objects.LocalBinders = val
		case "Proxy Binders":
			detail.Objects.ProxyBinders = val
		case "Death Recipients":
			detail.Objects.DeathRecipients = val
		case "WebViews":
			detail.Objects.WebViews = val
		}
	}
}

// parseMainTableLine 解析主表中的 Heap 行和 TOTAL 行
// 格式: "  Native Heap    43241    43224       12       18    44452    71308    62133     4802"
//
//	"  Dalvik Heap    13552    13528        8       49    15104    29301     4789    24512"
//	"        TOTAL   431653   322424    53644      132   585360   100609    66922    29314"
func parseMainTableLine(line string, detail *ProcessDetail) {
	trimmed := strings.TrimSpace(line)
	fields := strings.Fields(trimmed)

	// Native Heap 行: name="Native" "Heap" + 8 数字
	if len(fields) >= 10 && fields[0] == "Native" && fields[1] == "Heap" {
		detail.NativeHeapSizeKB, _ = strconv.Atoi(fields[7])
		detail.NativeHeapAllocKB, _ = strconv.Atoi(fields[8])
		detail.NativeHeapFreeKB, _ = strconv.Atoi(fields[9])
	}

	// Dalvik Heap 行
	if len(fields) >= 10 && fields[0] == "Dalvik" && fields[1] == "Heap" {
		detail.JavaHeapSizeKB, _ = strconv.Atoi(fields[7])
		detail.JavaHeapAllocKB, _ = strconv.Atoi(fields[8])
		detail.JavaHeapFreeKB, _ = strconv.Atoi(fields[9])
	}
}

// parseProcStatus 解析 /proc/<pid>/status 输出
func parseProcStatus(output string, detail *ProcessDetail) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Threads:") {
			fmt.Sscanf(line, "Threads:\t%d", &detail.Threads)
		} else if strings.HasPrefix(line, "FDSize:") {
			fmt.Sscanf(line, "FDSize:\t%d", &detail.FDSize)
		} else if strings.HasPrefix(line, "VmSwap:") {
			// "VmSwap:	   16060 kB"
			re := regexp.MustCompile(`VmSwap:\s+(\d+)`)
			if m := re.FindStringSubmatch(line); len(m) >= 2 {
				detail.VmSwapKB, _ = strconv.Atoi(m[1])
			}
		} else if strings.HasPrefix(line, "Uid:") {
			// "Uid:	10339	10339	10339	10339"
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				detail.UID, _ = strconv.Atoi(fields[1])
			}
		}
	}
}

// ========================================
// Wails 前端事件
// ========================================

// emitPerfStatus 发送性能监控状态到前端
func (a *App) emitPerfStatus(deviceID string, running bool, config *PerfMonitorConfig) {
	if a.mcpMode || a.ctx == nil {
		return
	}

	status := map[string]interface{}{
		"deviceId": deviceID,
		"running":  running,
	}
	if config != nil {
		status["config"] = config
	}

	wailsRuntime.EventsEmit(a.ctx, "perf-status", status)
}
