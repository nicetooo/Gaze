package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// logcatLinePattern matches Android logcat time format: "01-04 12:34:56.789 D/Tag( 1234): message"
var logcatLinePattern = regexp.MustCompile(`^\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}\.\d{3}\s+([VDIWEF])/([^(]+)\(\s*\d+\):\s*(.*)`)

// parseLogcatLine extracts level, tag, and message from a logcat line
func parseLogcatLine(line string) (level, tag, message string, ok bool) {
	matches := logcatLinePattern.FindStringSubmatch(line)
	if len(matches) < 4 {
		return "", "", "", false
	}
	return matches[1], strings.TrimSpace(matches[2]), matches[3], true
}

// logcatLevelToSessionLevel converts Android log level to session level
func logcatLevelToSessionLevel(androidLevel string) string {
	switch androidLevel {
	case "E", "F": // Error, Fatal
		return "error"
	case "W":
		return "warn"
	case "I":
		return "info"
	case "D":
		return "debug"
	default: // V (Verbose)
		return "verbose"
	}
}

// StartLogcat starts the logcat stream for a device
func (a *App) StartLogcat(deviceId, packageName, preFilter string, preUseRegex bool, excludeFilter string, excludeUseRegex bool) error {
	// 验证 deviceId 格式
	if err := ValidateDeviceID(deviceId); err != nil {
		return err
	}

	a.updateLastActive(deviceId)

	if a.logcatCmd != nil {
		a.StopLogcat()
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.logcatCancel = cancel

	var cmd *exec.Cmd
	shellCmd := "logcat -v time"

	if preFilter != "" {
		grepCmd := "grep -i"
		if preUseRegex {
			grepCmd += "E"
		}
		safeFilter := strings.ReplaceAll(preFilter, "'", "'\\''")
		shellCmd += fmt.Sprintf(" | %s '%s'", grepCmd, safeFilter)
	}

	if excludeFilter != "" {
		grepCmd := "grep -iv"
		if excludeUseRegex {
			grepCmd += "E"
		}
		safeExclude := strings.ReplaceAll(excludeFilter, "'", "'\\''")
		shellCmd += fmt.Sprintf(" | %s '%s'", grepCmd, safeExclude)
	}

	if preFilter != "" || excludeFilter != "" {
		cmd = a.newAdbCommand(ctx, "-s", deviceId, "shell", shellCmd)
	} else {
		cmd = a.newAdbCommand(ctx, "-s", deviceId, "logcat", "-v", "time")
	}
	a.logcatCmd = cmd

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		a.logcatCmd = nil
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		a.logcatCmd = nil
		return fmt.Errorf("failed to start logcat: %w", err)
	}

	var currentPids []string
	var currentUid string
	var pidMutex sync.RWMutex

	if packageName != "" {
		uidCmd := a.newAdbCommand(nil, "-s", deviceId, "shell", "pm list packages -U "+packageName)
		uidOut, _ := uidCmd.Output()
		uidStr := string(uidOut)
		if strings.Contains(uidStr, "uid:") {
			parts := strings.Split(uidStr, "uid:")
			if len(parts) > 1 {
				currentUid = strings.TrimSpace(strings.Fields(parts[1])[0])
			}
		}
	}

	if packageName != "" {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			checkPid := func() {
				// Use a timeout context to prevent PID check commands from piling up
				pidCtx, pidCancel := context.WithTimeout(ctx, 5*time.Second)
				defer pidCancel()

				c := a.newAdbCommand(pidCtx, "-s", deviceId, "shell", "pgrep -f", packageName)
				out, _ := c.Output()
				raw := strings.TrimSpace(string(out))

				if raw == "" {
					c2 := a.newAdbCommand(pidCtx, "-s", deviceId, "shell", "pidof", packageName)
					out2, _ := c2.Output()
					raw = strings.TrimSpace(string(out2))
				}

				if raw == "" {
					c3 := a.newAdbCommand(pidCtx, "-s", deviceId, "shell", "ps -A")
					out3, _ := c3.Output()
					lines := strings.Split(string(out3), "\n")
					var matchedPids []string
					for _, line := range lines {
						if strings.Contains(line, packageName) {
							fields := strings.Fields(line)
							if len(fields) > 1 {
								matchedPids = append(matchedPids, fields[1])
							}
						}
					}
					raw = strings.Join(matchedPids, " ")
				}

				pids := strings.Fields(raw)

				pidMutex.Lock()
				changed := len(pids) != len(currentPids)
				if !changed {
					for i, p := range pids {
						if p != currentPids[i] {
							changed = true
							break
						}
					}
				}

				if changed {
					currentPids = pids
					if len(pids) > 0 {
						status := fmt.Sprintf("--- Monitoring %s (UID: %s, PIDs: %s) ---", packageName, currentUid, strings.Join(pids, ", "))
						if !a.mcpMode {
							wailsRuntime.EventsEmit(a.ctx, "logcat-data", status)
						}
					} else {
						if !a.mcpMode {
							wailsRuntime.EventsEmit(a.ctx, "logcat-data", fmt.Sprintf("--- Waiting for %s processes... ---", packageName))
						}
					}
				}
				pidMutex.Unlock()
			}

			checkPid()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					checkPid()
				}
			}
		}()
	}

	// Channel for parsed log events
	logEvtChan := make(chan map[string]interface{}, 1000)

	// Reader & Filter Routine
	go func() {
		reader := bufio.NewReader(stdout)
		defer close(logEvtChan)

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				break
			}

			if packageName != "" {
				pidMutex.RLock()
				pids := currentPids
				uid := currentUid
				pidMutex.RUnlock()

				if len(pids) > 0 {
					found := false
					for _, pid := range pids {
						if strings.Contains(line, "("+pid+")") ||
							strings.Contains(line, "( "+pid+")") ||
							strings.Contains(line, "("+pid+" )") ||
							strings.Contains(line, "["+pid+"]") ||
							strings.Contains(line, "[ "+pid+"]") ||
							strings.Contains(line, " "+pid+":") ||
							strings.Contains(line, "/"+pid+"(") ||
							strings.Contains(line, " "+pid+" ") ||
							strings.Contains(line, " "+pid+"):") ||
							strings.Contains(line, " "+pid+":") {
							found = true
							break
						}
					}

					if !found && uid != "" && strings.Contains(line, " "+uid+" ") {
						found = true
					}

					if !found {
						continue
					}
				} else {
					continue
				}
			}

			if level, tag, message, ok := parseLogcatLine(line); ok {
				logEvtChan <- map[string]interface{}{
					"tag":         tag,
					"message":     message,
					"level":       level,
					"packageName": packageName,
					"raw":         strings.TrimSpace(line),
				}
			}
		}
	}()

	// Aggregator & Emitter Routine
	// Lines are bucketed by (tag, level): real devices interleave many tags,
	// so flushing on every tag/level switch (old behavior) produced mostly
	// single-line events. Buckets keep insertion order; flushes emit buckets
	// ordered by first-entry time, one event per bucket.
	go func() {
		type bucketKey struct {
			tag   string
			level string
		}
		type logBucket struct {
			entries      []map[string]interface{}
			firstTime    time.Time
			lastActivity time.Time
		}

		const bucketEntryLimit = 500
		// A continuously-active bucket must still flush periodically, or a
		// high-frequency single tag would stall the live log view until it
		// hits bucketEntryLimit
		const bucketMaxAge = time.Second

		buckets := make(map[bucketKey]*logBucket)

		flushTicker := time.NewTicker(300 * time.Millisecond)
		defer flushTicker.Stop()

		emitBucket := func(key bucketKey, b *logBucket) {
			sessionLevel := logcatLevelToSessionLevel(key.level)
			eventType := "logcat"
			title := fmt.Sprintf("[%s] %s", key.tag, b.entries[0]["message"])
			if len(b.entries) > 1 {
				eventType = "logcat_aggregated"
				title = fmt.Sprintf("Logcat Output (%d entries) - %s", len(b.entries), key.tag)
			}

			// Data stays the same JSON array of entries the frontend expects
			dataBytes, err := json.Marshal(b.entries)
			if err != nil {
				LogWarn("logcat").Err(err).Msg("Failed to marshal logcat entries")
				dataBytes = []byte("[]")
			}

			a.eventPipeline.Emit(UnifiedEvent{
				ID:             uuid.New().String(),
				DeviceID:       deviceId,
				Timestamp:      b.firstTime.UnixMilli(),
				Source:         SourceLogcat,
				Category:       CategoryLog,
				Type:           eventType,
				Level:          ParseEventLevel(sessionLevel),
				Title:          title,
				Data:           dataBytes,
				AggregateCount: len(b.entries),
				AggregateFirst: b.firstTime.UnixMilli(),
				AggregateLast:  b.lastActivity.UnixMilli(),
			})
		}

		// flushKeys emits the given buckets ordered by first-entry time and
		// removes them from the map.
		flushKeys := func(keys []bucketKey) {
			sort.Slice(keys, func(i, j int) bool {
				return buckets[keys[i]].firstTime.Before(buckets[keys[j]].firstTime)
			})
			for _, k := range keys {
				emitBucket(k, buckets[k])
				delete(buckets, k)
			}
		}

		flushAll := func() {
			if len(buckets) == 0 {
				return
			}
			keys := make([]bucketKey, 0, len(buckets))
			for k := range buckets {
				keys = append(keys, k)
			}
			flushKeys(keys)
		}

		for {
			select {
			case evt, ok := <-logEvtChan:
				if !ok {
					flushAll()
					return
				}

				key := bucketKey{tag: evt["tag"].(string), level: evt["level"].(string)}
				now := time.Now()

				b := buckets[key]
				if b == nil {
					b = &logBucket{firstTime: now}
					buckets[key] = b
				}
				b.entries = append(b.entries, evt)
				b.lastActivity = now

				// Safety valve: a runaway bucket (e.g. infinite loop of the
				// same log) flushes everything, preserving cross-bucket
				// ordering by first-entry time.
				if len(b.entries) >= bucketEntryLimit {
					flushAll()
				}

			case <-flushTicker.C:
				// Ticker fires every 300ms. Burst-merge semantics per bucket:
				// a bucket that received a line <100ms ago is mid-burst, keep
				// it — unless it has been held for bucketMaxAge already; flush
				// idle/expired buckets, oldest first.
				if len(buckets) == 0 {
					continue
				}
				due := make([]bucketKey, 0, len(buckets))
				for k, b := range buckets {
					if time.Since(b.lastActivity) < 100*time.Millisecond &&
						time.Since(b.firstTime) < bucketMaxAge {
						continue
					}
					due = append(due, k)
				}
				flushKeys(due)

			case <-ctx.Done():
				flushAll()
				return
			}
		}
	}()

	return nil
}

// StopLogcat stops the logcat stream
func (a *App) StopLogcat() {
	if a.logcatCancel != nil {
		a.logcatCancel()
	}
	if a.logcatCmd != nil && a.logcatCmd.Process != nil {
		_ = a.logcatCmd.Process.Kill()
	}
	a.logcatCmd = nil
	a.logcatCancel = nil
}
