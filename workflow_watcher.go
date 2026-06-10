package main

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// WorkflowWatcher monitors the workflows directory for changes from external processes (e.g., MCP)
type WorkflowWatcher struct {
	app     *App
	watcher *fsnotify.Watcher
	stopCh  chan struct{}
	mu      sync.Mutex

	selfWriteMu sync.Mutex
	selfWrites  map[string]time.Time // safeName -> time of our own write/delete
}

// MarkSelfWrite records a file write/delete initiated by this process, so the
// watcher can skip the resulting fsnotify events (prevent self-triggering).
// workflowId must be the sanitized file name (safeName), matching what the
// watcher extracts from the file path.
func (w *WorkflowWatcher) MarkSelfWrite(workflowId string) {
	w.selfWriteMu.Lock()
	defer w.selfWriteMu.Unlock()
	if w.selfWrites == nil {
		w.selfWrites = make(map[string]time.Time)
	}
	w.selfWrites[workflowId] = time.Now()
}

func (w *WorkflowWatcher) isSelfWrite(workflowId string) bool {
	w.selfWriteMu.Lock()
	defer w.selfWriteMu.Unlock()
	t, ok := w.selfWrites[workflowId]
	if !ok {
		return false
	}
	if time.Since(t) < time.Second { // debounce delay (300ms) + margin
		return true
	}
	delete(w.selfWrites, workflowId) // lazily evict expired entries
	return false
}

// NewWorkflowWatcher creates a new workflow directory watcher
func NewWorkflowWatcher(app *App) *WorkflowWatcher {
	return &WorkflowWatcher{
		app:    app,
		stopCh: make(chan struct{}),
	}
}

// Start begins watching the workflows directory
func (w *WorkflowWatcher) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Don't start watcher in MCP mode (no GUI to notify)
	if w.app.mcpMode {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	w.watcher = watcher

	workflowsPath := w.app.getWorkflowsPath()
	if err := watcher.Add(workflowsPath); err != nil {
		watcher.Close()
		return err
	}

	LogInfo("workflow_watcher").Str("path", workflowsPath).Msg("Started watching workflows directory")

	go w.watch()
	return nil
}

// Stop stops watching the workflows directory
func (w *WorkflowWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.watcher != nil {
		close(w.stopCh)
		w.watcher.Close()
		w.watcher = nil
		LogInfo("workflow_watcher").Msg("Stopped watching workflows directory")
	}
}

// watch is the main watch loop
func (w *WorkflowWatcher) watch() {
	// Debounce: wait for events to settle before notifying
	var debounceTimer *time.Timer
	debounceDelay := 300 * time.Millisecond

	notifyChange := func(action, workflowId string) {
		if !w.app.mcpMode && w.app.ctx != nil {
			wailsRuntime.EventsEmit(w.app.ctx, "workflow-list-changed", map[string]interface{}{
				"action":     action,
				"workflowId": workflowId,
				"external":   true, // Mark as external change (from MCP or file system)
			})
			LogDebug("workflow_watcher").
				Str("action", action).
				Str("workflowId", workflowId).
				Msg("Emitted workflow-list-changed event")
		}
	}

	for {
		select {
		case <-w.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only care about JSON files
			if !strings.HasSuffix(event.Name, ".json") {
				continue
			}

			// Extract workflow ID from filename
			workflowId := strings.TrimSuffix(filepath.Base(event.Name), ".json")

			// Skip events caused by our own SaveWorkflow/DeleteWorkflow before
			// touching the shared debounce timer, so a self-write can't swallow
			// a pending notification for an external change to another workflow
			if w.isSelfWrite(workflowId) {
				continue
			}

			// Debounce: reset timer on each event
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			action := ""
			switch {
			case event.Op&fsnotify.Create == fsnotify.Create:
				action = "create"
			case event.Op&fsnotify.Write == fsnotify.Write:
				action = "save"
			case event.Op&fsnotify.Remove == fsnotify.Remove:
				action = "delete"
			case event.Op&fsnotify.Rename == fsnotify.Rename:
				action = "delete" // Rename is often used for atomic writes
			}

			if action != "" {
				debounceTimer = time.AfterFunc(debounceDelay, func() {
					notifyChange(action, workflowId)
				})
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			LogError("workflow_watcher").Err(err).Msg("Watcher error")
		}
	}
}
