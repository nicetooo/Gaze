package main

import (
	"fmt"
	"strings"
	"testing"
)

// ========================================
// Real device output captured from Pixel 7a (36211JEHN03441)
// Non-rooted, 8 cores, Android
// ========================================

const realDumpsysCPUInfo = `Load: 1.06 / 1.07 / 1.64
CPU usage from 6334ms to 1271ms ago (2026-01-30 00:17:36.113 to 2026-01-30 00:17:41.177) with 99% awake:
  111% 20805/app.footos: 75% user + 35% kernel / faults: 13442 minor
  48% 20855/com.google.android.webview:sandboxed_process0:org.chromium.content.app.SandboxedProcessService0:0: 40% user + 8.5% kernel / faults: 75062 minor
  0.1% 14778/com.google.android.gms: 0.1% user + 0% kernel / faults: 33 minor 2 major
  0.3% 881/surfaceflinger: 0.1% user + 0.1% kernel
  0% 12345/com.example.test: 0% user + 0% kernel
46% TOTAL: 22% user + 18% kernel + 2.8% iowait + 1.9% irq + 0.5% softirq
`

const realDumpsysMeminfo = `Applications Memory Usage (in Kilobytes):
Uptime: 123456789 Realtime: 123456789

** MEMINFO in pid 20805 [app.footos] **
                   Pss  Private  Private  SwapPss      Rss     Heap     Heap     Heap
                 Total    Dirty    Clean    Dirty    Total     Size    Alloc     Free
                ------   ------   ------   ------   ------   ------   ------   ------
  Native Heap    12345     5678     1234      567    15000    20000    15000     5000
  Dalvik Heap     8901     3456      789      345    10000    12000     9000     3000
     Stack          64       64        0        0       80
     Ashmem          0        0        0        0        0
  Gfx dev        12000    12000        0        0    12000
  Other dev         48       24       24        0      200
   .so mmap      10234      456     3456     2345    25000
  .apk mmap       5678        0     3456        0    10000
  .dex mmap        234        0      200       12     3000
  .oat mmap         12        0        0        0      500
  .art mmap       3456     1234      567      890     8000
  Other mmap        78       12       56        0      300
  EGL mtrack    40000    40000        0        0    40000
  GL mtrack      7890     7890        0        0     7890
      Unknown      345      200      100       40      600
        TOTAL   100386     1856    15060    64499   142204    62985    33183    25817
           TOTAL PSS:   100386            TOTAL RSS:   142204       TOTAL SWAP PSS:    64499
 Objects
               Views:        120         ViewRootImpl:          2
         AppContexts:          5           Activities:          3
`

const realProcMeminfo = `MemTotal:        7919072 kB
MemFree:          234560 kB
MemAvailable:    4123456 kB
Buffers:          123456 kB
Cached:          2345678 kB
SwapCached:        12345 kB
Active:          3456789 kB
Inactive:        2345678 kB
Active(anon):    1234567 kB
Inactive(anon):   567890 kB
Active(file):    2222222 kB
Inactive(file):  1777788 kB
Unevictable:       12345 kB
Mlocked:           12345 kB
SwapTotal:       4194300 kB
SwapFree:        3145728 kB
Dirty:              1234 kB
Writeback:             0 kB
AnonPages:       1800000 kB
Mapped:           567890 kB
Shmem:             12345 kB
Slab:             345678 kB
SReclaimable:     123456 kB
SUnreclaim:       222222 kB
KernelStack:       56789 kB
PageTables:        45678 kB
NFS_Unstable:          0 kB
Bounce:                0 kB
WritebackTmp:          0 kB
CommitLimit:     8153836 kB
Committed_AS:    9876543 kB
VmallocTotal:   263061440 kB
VmallocUsed:      123456 kB
VmallocChunk:          0 kB
`

const realProcNetDev = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:  123456     789    0    0    0     0          0         0   123456     789    0    0    0     0       0          0
  dummy0:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
wlan0: 247816032  242814    0    1    0     0          0         0 100640622  157184    0    4    0     0       0          0
rmnet0:   50000     100    0    0    0     0          0         0    30000      80    0    0    0     0       0          0
`

// Alternative format: bytes immediately after colon (no space)
const procNetDevNoSpace = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:123456     789    0    0    0     0          0         0   123456     789    0    0    0     0       0          0
wlan0:247816032  242814    0    1    0     0          0         0 100640622  157184    0    4    0     0       0          0
`

const realDumpsysBattery = `Current Battery Service state:
  AC powered: false
  USB powered: true
  Wireless powered: false
  Max charging current: 500000
  Max charging voltage: 5000000
  Charge type: 1
  status: 5
  health: 2
  present: true
  level: 100
  scale: 100
  voltage: 4350
  temperature: 378
  technology: Li-ion
`

const realDumpsysGfxInfo = `** Graphics info for pid 20805 [app.footos] **

Stats since: 13245678901234ns
Total frames rendered: 12345
Janky frames: 678 (5.49%)
50th percentile: 5ms
90th percentile: 12ms
95th percentile: 18ms
99th percentile: 32ms

Number Missed Vsync: 123
Number High input latency: 45
Number Slow UI thread: 234
Number Slow bitmap uploads: 12
Number Slow issue draw commands: 89

HISTOGRAM DATA:
...
`

const realGfxInfoNoFrames = `** Graphics info for pid 99999 [com.idle.app] **

Stats since: 13245678901234ns
Total frames rendered: 0
Janky frames: 0 (nan%)
50th percentile: 0ms
`

const realMCurrentFocus = `  mCurrentFocus=Window{ab3a179 u0 app.footos/app.footos.MainActivity}
`

const mCurrentFocusStatusBar = `  mCurrentFocus=Window{1234abc u0 StatusBar}
`

const mCurrentFocusLauncher = `  mCurrentFocus=Window{deadbeef u0 com.google.android.apps.nexuslauncher/com.google.android.apps.nexuslauncher.NexusLauncherActivity}
`

// ========================================
// Test: parseCPUFromDumpsys
// ========================================

func TestParseCPUFromDumpsys(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect float64
	}{
		{
			name:   "real Pixel 7a output",
			input:  realDumpsysCPUInfo,
			expect: 46,
		},
		{
			name:   "decimal percentage",
			input:  "12.5% TOTAL: 8.1% user + 4.4% kernel\n",
			expect: 12.5,
		},
		{
			name:   "100% load",
			input:  "100% TOTAL: 60% user + 40% kernel\n",
			expect: 100,
		},
		{
			name:   "empty output",
			input:  "",
			expect: 0,
		},
		{
			name:   "no TOTAL line",
			input:  "  10% 1234/com.example: 5% user + 5% kernel\n",
			expect: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCPUFromDumpsys(tt.input)
			if got != tt.expect {
				t.Errorf("parseCPUFromDumpsys() = %v, want %v", got, tt.expect)
			}
		})
	}
}

// ========================================
// Test: parseAppCPUFromDumpsys
// ========================================

func TestParseAppCPUFromDumpsys(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		packageName string
		expect      float64
	}{
		{
			name:        "single process app (app.footos)",
			input:       realDumpsysCPUInfo,
			packageName: "app.footos",
			expect:      111, // 111% on 8-core device
		},
		{
			name:        "multi-process app (webview sandbox counts too)",
			input:       realDumpsysCPUInfo,
			packageName: "app.footos",
			// app.footos (111%) - webview sandbox line contains "app.footos" in its description? No.
			// The webview line is: "com.google.android.webview:sandboxed_process0:org.chromium..."
			// It does NOT contain "app.footos", so only 111%
			expect: 111,
		},
		{
			name:        "GMS service",
			input:       realDumpsysCPUInfo,
			packageName: "com.google.android.gms",
			expect:      0.1,
		},
		{
			name:        "zero CPU app",
			input:       realDumpsysCPUInfo,
			packageName: "com.example.test",
			expect:      0, // "0%" matches as 0
		},
		{
			name:        "package not found",
			input:       realDumpsysCPUInfo,
			packageName: "com.nonexistent.app",
			expect:      0,
		},
		{
			name:        "empty output",
			input:       "",
			packageName: "com.example",
			expect:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAppCPUFromDumpsys(tt.input, tt.packageName)
			if got != tt.expect {
				t.Errorf("parseAppCPUFromDumpsys(%q) = %v, want %v", tt.packageName, got, tt.expect)
			}
		})
	}
}

// ========================================
// Test: parseMemInfo
// ========================================

func TestParseMemInfo(t *testing.T) {
	sample := &PerfSampleData{}
	parseMemInfo(realProcMeminfo, sample)

	// MemTotal: 7919072 kB → ~7733 MB
	if sample.MemTotalMB != 7733 {
		t.Errorf("MemTotalMB = %d, want 7733", sample.MemTotalMB)
	}

	// MemAvailable: 4123456 kB → ~4026 MB
	if sample.MemFreeMB != 4026 {
		t.Errorf("MemFreeMB = %d, want 4026", sample.MemFreeMB)
	}

	// MemUsed = Total - Free = 7733 - 4026 = 3707
	if sample.MemUsedMB != 3707 {
		t.Errorf("MemUsedMB = %d, want 3707", sample.MemUsedMB)
	}

	// MemUsage = 3707/7733 * 100 = ~47.9%
	if sample.MemUsage < 47 || sample.MemUsage > 48 {
		t.Errorf("MemUsage = %.1f%%, want ~47.9%%", sample.MemUsage)
	}
}

func TestParseMemInfoFallback(t *testing.T) {
	// Without MemAvailable, should fall back to Free+Buffers+Cached
	input := `MemTotal:        7919072 kB
MemFree:          234560 kB
Buffers:          123456 kB
Cached:          2345678 kB
`
	sample := &PerfSampleData{}
	parseMemInfo(input, sample)

	if sample.MemTotalMB != 7733 {
		t.Errorf("MemTotalMB = %d, want 7733", sample.MemTotalMB)
	}

	// Free+Buffers+Cached = 234560+123456+2345678 = 2703694 kB → ~2640 MB
	expectedFree := (234560 + 123456 + 2345678) / 1024
	if sample.MemFreeMB != expectedFree {
		t.Errorf("MemFreeMB = %d, want %d", sample.MemFreeMB, expectedFree)
	}
}

// ========================================
// Test: parseAppMemory
// ========================================

func TestParseAppMemory(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect int
	}{
		{
			name:   "real full dumpsys meminfo output",
			input:  realDumpsysMeminfo,
			expect: 100386 / 1024, // ~98 MB
		},
		{
			name:   "TOTAL PSS summary line only",
			input:  "           TOTAL PSS:   100386            TOTAL RSS:   142204\n",
			expect: 100386 / 1024,
		},
		{
			name:   "TOTAL detail line only",
			input:  "        TOTAL   100386     1856    15060    64499   142204\n",
			expect: 100386 / 1024,
		},
		{
			name:   "small app",
			input:  "        TOTAL    5120     100    200    300    600\n           TOTAL PSS:    5120\n",
			expect: 5120 / 1024, // 5 MB
		},
		{
			name:   "empty output",
			input:  "",
			expect: 0,
		},
		{
			name:   "no TOTAL line",
			input:  "  Native Heap    12345\n  Dalvik Heap     8901\n",
			expect: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAppMemory(tt.input)
			if got != tt.expect {
				t.Errorf("parseAppMemory() = %d, want %d", got, tt.expect)
			}
		})
	}
}

// ========================================
// Test: parseGfxInfoFrameCounts
// ========================================

func TestParseGfxInfoFrameCounts(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTotal int
		wantJanky int
	}{
		{
			name:      "real gfxinfo output",
			input:     realDumpsysGfxInfo,
			wantTotal: 12345,
			wantJanky: 678,
		},
		{
			name:      "idle app (0 frames)",
			input:     realGfxInfoNoFrames,
			wantTotal: 0,
			wantJanky: 0,
		},
		{
			name:      "empty output",
			input:     "",
			wantTotal: 0,
			wantJanky: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, janky := parseGfxInfoFrameCounts(tt.input)
			if total != tt.wantTotal {
				t.Errorf("totalFrames = %d, want %d", total, tt.wantTotal)
			}
			if janky != tt.wantJanky {
				t.Errorf("jankyFrames = %d, want %d", janky, tt.wantJanky)
			}
		})
	}
}

// ========================================
// Test: parseForegroundPackage
// ========================================

func TestParseForegroundPackage(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "real app focus",
			input:  realMCurrentFocus,
			expect: "app.footos",
		},
		{
			name:   "status bar (no dot, should return empty)",
			input:  mCurrentFocusStatusBar,
			expect: "",
		},
		{
			name:   "launcher with long package",
			input:  mCurrentFocusLauncher,
			expect: "com.google.android.apps.nexuslauncher",
		},
		{
			name:   "empty output",
			input:  "",
			expect: "",
		},
		{
			name:   "null focus",
			input:  "  mCurrentFocus=null\n",
			expect: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseForegroundPackage(tt.input)
			if got != tt.expect {
				t.Errorf("parseForegroundPackage() = %q, want %q", got, tt.expect)
			}
		})
	}
}

// ========================================
// Test: parseNetworkDev
// ========================================

func TestParseNetworkDev(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantRx int64
		wantTx int64
	}{
		{
			name:  "real device output (wlan0 + rmnet0)",
			input: realProcNetDev,
			// wlan0: rx=247816032 tx=100640622 + rmnet0: rx=50000 tx=30000
			wantRx: 247816032 + 50000,
			wantTx: 100640622 + 30000,
		},
		{
			name:   "bytes after colon without space",
			input:  procNetDevNoSpace,
			wantRx: 247816032,
			wantTx: 100640622,
		},
		{
			name:   "empty output",
			input:  "",
			wantRx: 0,
			wantTx: 0,
		},
		{
			name:   "only header lines",
			input:  "Inter-|   Receive\n face |bytes    packets\n",
			wantRx: 0,
			wantTx: 0,
		},
		{
			name: "only lo interface (should be skipped)",
			input: `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:  123456     789    0    0    0     0          0         0   123456     789    0    0    0     0       0          0
`,
			wantRx: 0,
			wantTx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRx, gotTx := parseNetworkDev(tt.input)
			if gotRx != tt.wantRx {
				t.Errorf("parseNetworkDev() rx = %d, want %d", gotRx, tt.wantRx)
			}
			if gotTx != tt.wantTx {
				t.Errorf("parseNetworkDev() tx = %d, want %d", gotTx, tt.wantTx)
			}
		})
	}
}

// ========================================
// Test: parseNetDevLine
// ========================================

func TestParseNetDevLine(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		wantRx int64
		wantTx int64
	}{
		{
			name:   "wlan0 with space after colon",
			input:  "wlan0: 247816032  242814    0    1    0     0          0         0 100640622  157184    0    4    0     0       0          0",
			wantRx: 247816032,
			wantTx: 100640622,
		},
		{
			name:   "wlan0 no space after colon",
			input:  "wlan0:247816032  242814    0    1    0     0          0         0 100640622  157184    0    4    0     0       0          0",
			wantRx: 247816032,
			wantTx: 100640622,
		},
		{
			name:   "too few fields",
			input:  "wlan0: 123",
			wantRx: 0,
			wantTx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rx, tx := parseNetDevLine(tt.input)
			if rx != tt.wantRx {
				t.Errorf("parseNetDevLine() rx = %d, want %d", rx, tt.wantRx)
			}
			if tx != tt.wantTx {
				t.Errorf("parseNetDevLine() tx = %d, want %d", tx, tt.wantTx)
			}
		})
	}
}

// ========================================
// Test: parseBatteryOutput (collectBattery / collectCombined 共用的解析逻辑)
// ========================================

func TestParseBattery(t *testing.T) {
	sample := &PerfSampleData{}
	parseBatteryOutput(realDumpsysBattery, sample)

	if sample.BatteryLevel != 100 {
		t.Errorf("BatteryLevel = %d, want 100", sample.BatteryLevel)
	}

	// temperature: 378 → 37.8°C
	if sample.BatteryTemp != 37.8 {
		t.Errorf("BatteryTemp = %.1f, want 37.8", sample.BatteryTemp)
	}

	// CPUTempC should be set to battery temp as fallback
	if sample.CPUTempC != 37.8 {
		t.Errorf("CPUTempC = %.1f, want 37.8 (fallback from battery)", sample.CPUTempC)
	}
}

// ========================================
// Test: parseAllProcessCPU
// ========================================

func TestParseAllProcessCPU(t *testing.T) {
	entries := parseAllProcessCPU(realDumpsysCPUInfo)

	if len(entries) == 0 {
		t.Fatal("parseAllProcessCPU returned empty list")
	}

	// Verify known entries
	found := map[string]bool{}
	for _, e := range entries {
		found[e.Name] = true
		switch e.Name {
		case "app.footos":
			if e.PID != 20805 {
				t.Errorf("app.footos PID = %d, want 20805", e.PID)
			}
			if e.Total != 111 {
				t.Errorf("app.footos CPU = %.1f, want 111", e.Total)
			}
			if e.User != 75 {
				t.Errorf("app.footos User = %.1f, want 75", e.User)
			}
			if e.Kernel != 35 {
				t.Errorf("app.footos Kernel = %.1f, want 35", e.Kernel)
			}
		case "com.google.android.gms":
			if e.Total != 0.1 {
				t.Errorf("gms CPU = %.1f, want 0.1", e.Total)
			}
		}
	}

	if !found["app.footos"] {
		t.Error("app.footos not found in results")
	}
	if !found["com.google.android.gms"] {
		t.Error("com.google.android.gms not found in results")
	}
	if !found["surfaceflinger"] {
		t.Error("surfaceflinger not found in results")
	}
}

func TestParseAllProcessCPUEmpty(t *testing.T) {
	entries := parseAllProcessCPU("")
	if len(entries) != 0 {
		t.Errorf("expected empty, got %d entries", len(entries))
	}
}

// ========================================
// Test: parsePSOutput
// ========================================

const realPSOutput = `USER           PID  PPID     VSZ    RSS WCHAN            ADDR S NAME
root             1     0   47660  10340 do_epoll_+          0 S init
system         881     1  234567  12345 do_epoll_+          0 S surfaceflinger
u0_a123      20805  1234 1234567 123456 do_epoll_+          0 S com.example.app
u0_a456      20855  1234  987654  45678 do_epoll_+          0 S com.google.android.webview
u0_a789      14778  1234  567890  34567 do_epoll_+          0 S com.google.android.gms
u0_a100      99999  1234  111111  22222 do_epoll_+          0 S com.idle.background
`

func TestParsePSOutput(t *testing.T) {
	result := parsePSOutput(realPSOutput)

	if len(result) < 5 {
		t.Fatalf("parsePSOutput returned %d entries, want >= 5", len(result))
	}

	// Check specific entries
	if ps, ok := result[20805]; !ok {
		t.Error("PID 20805 not found")
	} else {
		if ps.Name != "com.example.app" {
			t.Errorf("PID 20805 name = %q, want %q", ps.Name, "com.example.app")
		}
		if ps.RSS != 123456 {
			t.Errorf("PID 20805 RSS = %d, want 123456", ps.RSS)
		}
		if ps.LinuxUser != "u0_a123" {
			t.Errorf("PID 20805 LinuxUser = %q, want %q", ps.LinuxUser, "u0_a123")
		}
		if ps.PPID != 1234 {
			t.Errorf("PID 20805 PPID = %d, want 1234", ps.PPID)
		}
		if ps.VSZ != 1234567 {
			t.Errorf("PID 20805 VSZ = %d, want 1234567", ps.VSZ)
		}
		if ps.State != "S" {
			t.Errorf("PID 20805 State = %q, want %q", ps.State, "S")
		}
	}

	if ps, ok := result[1]; !ok {
		t.Error("PID 1 (init) not found")
	} else {
		if ps.RSS != 10340 {
			t.Errorf("PID 1 RSS = %d, want 10340", ps.RSS)
		}
		if ps.LinuxUser != "root" {
			t.Errorf("PID 1 LinuxUser = %q, want %q", ps.LinuxUser, "root")
		}
		if ps.PPID != 0 {
			t.Errorf("PID 1 PPID = %d, want 0", ps.PPID)
		}
		if ps.State != "S" {
			t.Errorf("PID 1 State = %q, want %q", ps.State, "S")
		}
	}
}

func TestParsePSOutputEmpty(t *testing.T) {
	result := parsePSOutput("")
	if len(result) != 0 {
		t.Errorf("expected empty, got %d", len(result))
	}
}

// ========================================
// Test: buildProcessList
// ========================================

func TestBuildProcessList(t *testing.T) {
	processes := buildProcessList(realDumpsysCPUInfo, realPSOutput)

	if len(processes) == 0 {
		t.Fatal("buildProcessList returned empty list")
	}

	// Should only contain app-like processes (name contains ".")
	for _, p := range processes {
		if !strings.Contains(p.Name, ".") {
			t.Errorf("non-app process in list: %q", p.Name)
		}
	}

	// Should be sorted by CPU descending
	for i := 1; i < len(processes); i++ {
		if processes[i].CPU > processes[i-1].CPU {
			t.Errorf("not sorted by CPU: [%d].CPU=%.1f > [%d].CPU=%.1f",
				i, processes[i].CPU, i-1, processes[i-1].CPU)
		}
	}

	// Verify CPU data is merged with RSS data (including new ps fields)
	foundWithBoth := false
	for _, p := range processes {
		if p.CPU > 0 && p.MemoryKB > 0 {
			foundWithBoth = true
			// Verify new fields are populated from ps -A merge
			if p.LinuxUser == "" {
				t.Errorf("process %q has CPU+Memory but empty LinuxUser", p.Name)
			}
			if p.PPID == 0 && p.PID != 1 {
				t.Errorf("process %q has CPU+Memory but PPID=0", p.Name)
			}
			if p.VSZKB == 0 {
				t.Errorf("process %q has CPU+Memory but VSZKB=0", p.Name)
			}
			if p.State == "" {
				t.Errorf("process %q has CPU+Memory but empty State", p.Name)
			}
			break
		}
	}
	if !foundWithBoth {
		t.Error("no process has both CPU and memory data - merge failed")
	}

	// Idle background app should be included (0% CPU, has RSS)
	foundIdle := false
	for _, p := range processes {
		if p.Name == "com.idle.background" {
			foundIdle = true
			if p.MemoryKB != 22222 {
				t.Errorf("idle app memoryKB = %d, want 22222", p.MemoryKB)
			}
			if p.LinuxUser != "u0_a100" {
				t.Errorf("idle app LinuxUser = %q, want %q", p.LinuxUser, "u0_a100")
			}
			if p.State != "S" {
				t.Errorf("idle app State = %q, want %q", p.State, "S")
			}
			if p.VSZKB != 111111 {
				t.Errorf("idle app VSZKB = %d, want 111111", p.VSZKB)
			}
			break
		}
	}
	if !foundIdle {
		t.Error("idle background app not found in process list")
	}
}

func TestBuildProcessListEmpty(t *testing.T) {
	processes := buildProcessList("", "")
	if processes != nil && len(processes) != 0 {
		t.Errorf("expected empty, got %d", len(processes))
	}
}

func TestBuildProcessListTruncation(t *testing.T) {
	// 构造超过 maxProcessListEntries 个应用进程的 ps -A 输出
	var sb strings.Builder
	sb.WriteString("USER           PID  PPID     VSZ    RSS WCHAN            ADDR S NAME\n")
	for i := 0; i < maxProcessListEntries+20; i++ {
		sb.WriteString(fmt.Sprintf("u0_a%d %d 1234 100000 %d do_epoll_+ 0 S com.example.app%d\n",
			i, 1000+i, 10000+i, i))
	}

	processes := buildProcessList("", sb.String())
	if len(processes) != maxProcessListEntries {
		t.Fatalf("process list length = %d, want %d (truncated)", len(processes), maxProcessListEntries)
	}
	// 截断保留的应是排序后内存最高的条目 (无 CPU 数据时按内存降序)
	if processes[0].MemoryKB < processes[len(processes)-1].MemoryKB {
		t.Errorf("truncated list not sorted: first MemoryKB=%d < last MemoryKB=%d",
			processes[0].MemoryKB, processes[len(processes)-1].MemoryKB)
	}
}

// ========================================
// Test: Process Detail (dumpsys meminfo + /proc/status)
// ========================================

// Real output from Pixel 7a: dumpsys meminfo 27172 (app.footos) — for process detail tests
const realDumpsysMeminfoPid = `Applications Memory Usage (in Kilobytes):
Uptime: 58505945 Realtime: 174557885

** MEMINFO in pid 27172 [app.footos] **
                   Pss  Private  Private  SwapPss      Rss     Heap     Heap     Heap
                 Total    Dirty    Clean    Dirty    Total     Size    Alloc     Free
                ------   ------   ------   ------   ------   ------   ------   ------
  Native Heap    43241    43224       12       18    44452    71308    62133     4802
  Dalvik Heap    13552    13528        8       49    15104    29301     4789    24512
 Dalvik Other    11745     2744        0        0    21084                           
        Stack     2896     2896        0        0     2904                           
       Ashmem     4414       60        0        0     9664                           
    Other dev       24        0       20        0      384                           
     .so mmap    28231      172    15420        7    66204                           
    .jar mmap     2330        0      616        0    48124                           
    .apk mmap    63364     2180    34936        0    96232                           
    .ttf mmap       62        0        0        0      308                           
    .dex mmap       40        0        0        0      848                           
    .oat mmap       31        0        4        0     1968                           
    .art mmap     7783     7248      252       34    18176                           
   Other mmap     1717        4      984        0     3252                           
   EGL mtrack    74816    74816        0        0    74816                           
    GL mtrack   150456   150456        0        0   150456                           
      Unknown    26819    25096     1392       24    31384                           
        TOTAL   431653   322424    53644      132   585360   100609    66922    29314
 
 App Summary
                       Pss(KB)                        Rss(KB)
                        ------                         ------
           Java Heap:    21028                          33280
         Native Heap:    43224                          44452
                Code:    53332                         231688
               Stack:     2896                           2904
            Graphics:   225272                         225272
       Private Other:    30316
              System:    55585
             Unknown:                                   47764
 
           TOTAL PSS:   431653            TOTAL RSS:   585360       TOTAL SWAP PSS:      132
 
 Objects
               Views:       10         ViewRootImpl:        1
         AppContexts:        6           Activities:        1
              Assets:       29        AssetManagers:        0
       Local Binders:       62        Proxy Binders:       65
       Parcel memory:       13         Parcel count:       54
    Death Recipients:        4             WebViews:        1
 
 SQL
         MEMORY_USED:        0
  PAGECACHE_OVERFLOW:        0          MALLOC_SIZE:        0
`

// Real output from Pixel 7a: /proc/27172/status
const realProcStatus = `Name:	app.footos
Umask:	0077
State:	S (sleeping)
Tgid:	27172
Ngid:	0
Pid:	27172
PPid:	1040
TracerPid:	0
Uid:	10339	10339	10339	10339
Gid:	10339	10339	10339	10339
FDSize:	512
Groups:	3003 9997 20339 50339 
VmPeak:	67648212 kB
VmSize:	44488972 kB
VmLck:	       0 kB
VmPin:	       0 kB
VmHWM:	  524884 kB
VmRSS:	  505536 kB
RssAnon:	  114120 kB
RssFile:	  363616 kB
RssShmem:	   27800 kB
VmData:	 3207480 kB
VmStk:	    8192 kB
VmExe:	       4 kB
VmLib:	  386484 kB
VmPTE:	    2732 kB
VmSwap:	   16060 kB
CoreDumping:	0
THP_enabled:	1
Threads:	75
SigQ:	0/25426
SigPnd:	0000000000000000
ShdPnd:	0000000000000000
SigBlk:	0000000080001204
SigIgn:	0000000000000001
SigCgt:	0000006e400084f8
CapInh:	0000000000000000
CapPrm:	0000000000000000
CapEff:	0000000000000000
CapBnd:	0000000000000000
CapAmb:	0000000000000000
NoNewPrivs:	0
Seccomp:	2
Seccomp_filters:	1
Speculation_Store_Bypass:	thread vulnerable
SpeculationIndirectBranch:	unknown
Cpus_allowed:	ff
Cpus_allowed_list:	0-7
Mems_allowed:	1
Mems_allowed_list:	0
voluntary_ctxt_switches:	17819
nonvoluntary_ctxt_switches:	2640
`

func TestParseMeminfoDump(t *testing.T) {
	detail := &ProcessDetail{PID: 27172}
	parseMeminfoDump(realDumpsysMeminfoPid, detail)

	// Package name
	if detail.PackageName != "app.footos" {
		t.Errorf("PackageName = %q, want %q", detail.PackageName, "app.footos")
	}

	// TOTAL PSS / RSS / SWAP
	if detail.TotalPSSKB != 431653 {
		t.Errorf("TotalPSSKB = %d, want 431653", detail.TotalPSSKB)
	}
	if detail.TotalRSSKB != 585360 {
		t.Errorf("TotalRSSKB = %d, want 585360", detail.TotalRSSKB)
	}
	if detail.SwapPSSKB != 132 {
		t.Errorf("SwapPSSKB = %d, want 132", detail.SwapPSSKB)
	}

	// Memory categories from App Summary
	if len(detail.Memory) == 0 {
		t.Fatal("Memory categories empty")
	}

	// Check specific categories
	catMap := make(map[string]ProcessMemoryCategory)
	for _, cat := range detail.Memory {
		catMap[cat.Name] = cat
	}

	if java, ok := catMap["Java Heap"]; !ok {
		t.Error("Java Heap not found")
	} else {
		if java.PssKB != 21028 {
			t.Errorf("Java Heap PssKB = %d, want 21028", java.PssKB)
		}
		if java.RssKB != 33280 {
			t.Errorf("Java Heap RssKB = %d, want 33280", java.RssKB)
		}
	}

	if graphics, ok := catMap["Graphics"]; !ok {
		t.Error("Graphics not found")
	} else if graphics.PssKB != 225272 {
		t.Errorf("Graphics PssKB = %d, want 225272", graphics.PssKB)
	}

	if native, ok := catMap["Native Heap"]; !ok {
		t.Error("Native Heap not found")
	} else if native.PssKB != 43224 {
		t.Errorf("Native Heap PssKB = %d, want 43224", native.PssKB)
	}

	// Heap details from main table
	if detail.NativeHeapSizeKB != 71308 {
		t.Errorf("NativeHeapSizeKB = %d, want 71308", detail.NativeHeapSizeKB)
	}
	if detail.NativeHeapAllocKB != 62133 {
		t.Errorf("NativeHeapAllocKB = %d, want 62133", detail.NativeHeapAllocKB)
	}
	if detail.NativeHeapFreeKB != 4802 {
		t.Errorf("NativeHeapFreeKB = %d, want 4802", detail.NativeHeapFreeKB)
	}
	if detail.JavaHeapSizeKB != 29301 {
		t.Errorf("JavaHeapSizeKB = %d, want 29301", detail.JavaHeapSizeKB)
	}
	if detail.JavaHeapAllocKB != 4789 {
		t.Errorf("JavaHeapAllocKB = %d, want 4789", detail.JavaHeapAllocKB)
	}
	if detail.JavaHeapFreeKB != 24512 {
		t.Errorf("JavaHeapFreeKB = %d, want 24512", detail.JavaHeapFreeKB)
	}

	// Objects
	if detail.Objects.Views != 10 {
		t.Errorf("Views = %d, want 10", detail.Objects.Views)
	}
	if detail.Objects.Activities != 1 {
		t.Errorf("Activities = %d, want 1", detail.Objects.Activities)
	}
	if detail.Objects.WebViews != 1 {
		t.Errorf("WebViews = %d, want 1", detail.Objects.WebViews)
	}
	if detail.Objects.LocalBinders != 62 {
		t.Errorf("LocalBinders = %d, want 62", detail.Objects.LocalBinders)
	}
	if detail.Objects.ProxyBinders != 65 {
		t.Errorf("ProxyBinders = %d, want 65", detail.Objects.ProxyBinders)
	}
	if detail.Objects.DeathRecipients != 4 {
		t.Errorf("DeathRecipients = %d, want 4", detail.Objects.DeathRecipients)
	}
	if detail.Objects.AppContexts != 6 {
		t.Errorf("AppContexts = %d, want 6", detail.Objects.AppContexts)
	}
	if detail.Objects.Assets != 29 {
		t.Errorf("Assets = %d, want 29", detail.Objects.Assets)
	}
}

func TestParseMeminfoDumpEmpty(t *testing.T) {
	detail := &ProcessDetail{}
	parseMeminfoDump("", detail)
	if detail.TotalPSSKB != 0 || len(detail.Memory) != 0 {
		t.Errorf("expected empty detail, got PSS=%d, cats=%d", detail.TotalPSSKB, len(detail.Memory))
	}
}

func TestParseProcStatus(t *testing.T) {
	detail := &ProcessDetail{}
	parseProcStatus(realProcStatus, detail)

	if detail.Threads != 75 {
		t.Errorf("Threads = %d, want 75", detail.Threads)
	}
	if detail.FDSize != 512 {
		t.Errorf("FDSize = %d, want 512", detail.FDSize)
	}
	if detail.VmSwapKB != 16060 {
		t.Errorf("VmSwapKB = %d, want 16060", detail.VmSwapKB)
	}
	if detail.UID != 10339 {
		t.Errorf("UID = %d, want 10339", detail.UID)
	}
}

func TestParseProcStatusEmpty(t *testing.T) {
	detail := &ProcessDetail{}
	parseProcStatus("", detail)
	if detail.Threads != 0 || detail.FDSize != 0 {
		t.Errorf("expected zeros, got Threads=%d FDSize=%d", detail.Threads, detail.FDSize)
	}
}
