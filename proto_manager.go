package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"Gaze/pkg/fileutil"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// protoHTTPClient is a shared HTTP client for proto file downloads with proper timeout.
var protoHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// Global proto registry and decoder, shared across the app.
var (
	protoRegistry *ProtoRegistry
	protoDecoder  *ProtobufDecoder
	protoOnce     sync.Once

	// Set when the corresponding config file on disk failed to parse; saves are
	// refused until restart so a half-loaded state can't overwrite user data.
	protoFilesLoadFailed    bool
	protoMappingsLoadFailed bool
	protoLoadFailedMu       sync.Mutex
)

func getProtoRegistry() *ProtoRegistry {
	protoOnce.Do(func() {
		protoRegistry = NewProtoRegistry()
		protoDecoder = NewProtobufDecoder(protoRegistry)
	})
	return protoRegistry
}

func getProtoDecoder() *ProtobufDecoder {
	getProtoRegistry() // ensure initialized
	return protoDecoder
}

// --- Persistence paths ---

func getProtoDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".adbGUI", "proto")
}

func getProtoFilesPath() string {
	return filepath.Join(getProtoDir(), "files.json")
}

func getProtoMappingsPath() string {
	return filepath.Join(getProtoDir(), "mappings.json")
}

// --- App methods (Wails bindings) ---

// LoadProtoConfig loads proto files and mappings from disk. Called on app startup.
func (a *App) LoadProtoConfig() error {
	reg := getProtoRegistry()

	dir := getProtoDir()
	_ = os.MkdirAll(dir, 0755)

	// Load files
	if data, err := os.ReadFile(getProtoFilesPath()); err == nil {
		var files []*ProtoFileEntry
		if err := json.Unmarshal(data, &files); err == nil {
			reg.mu.Lock()
			for _, f := range files {
				reg.files[f.ID] = f
			}
			reg.mu.Unlock()
		} else {
			protoLoadFailedMu.Lock()
			protoFilesLoadFailed = true
			protoLoadFailedMu.Unlock()
			quarantineCorruptConfig("proto_files", getProtoFilesPath(), err)
		}
	}

	// Load mappings
	if data, err := os.ReadFile(getProtoMappingsPath()); err == nil {
		var mappings []*ProtoMapping
		if err := json.Unmarshal(data, &mappings); err == nil {
			reg.mu.Lock()
			for _, m := range mappings {
				reg.mappings[m.ID] = m
			}
			reg.mu.Unlock()
		} else {
			protoLoadFailedMu.Lock()
			protoMappingsLoadFailed = true
			protoLoadFailedMu.Unlock()
			quarantineCorruptConfig("proto_mappings", getProtoMappingsPath(), err)
		}
	}

	// Compile all loaded files
	if err := reg.recompile(); err != nil {
		log.Printf("[Proto] Compile warning: %v", err)
		// Don't return error — partial compilation is acceptable
	}

	getProtoDecoder().ClearAutoCache()
	return nil
}

// AddProtoFile adds a .proto file definition. Content is the raw .proto text.
func (a *App) AddProtoFile(name, content string) (string, error) {
	if name == "" || content == "" {
		return "", fmt.Errorf("name and content are required")
	}

	entry := &ProtoFileEntry{
		ID:       uuid.New().String(),
		Name:     name,
		Content:  content,
		LoadedAt: time.Now().UnixMilli(),
	}

	reg := getProtoRegistry()
	if err := reg.AddFile(entry); err != nil {
		return "", fmt.Errorf("compile error: %w", err)
	}

	getProtoDecoder().ClearAutoCache()
	return entry.ID, saveProtoFiles(reg)
}

// UpdateProtoFile updates an existing .proto file's content.
func (a *App) UpdateProtoFile(id, name, content string) error {
	reg := getProtoRegistry()

	reg.mu.RLock()
	existing, ok := reg.files[id]
	reg.mu.RUnlock()
	if !ok {
		return fmt.Errorf("proto file not found: %s", id)
	}

	entry := &ProtoFileEntry{
		ID:       id,
		Name:     name,
		Content:  content,
		LoadedAt: existing.LoadedAt,
	}

	if err := reg.AddFile(entry); err != nil {
		return fmt.Errorf("compile error: %w", err)
	}

	getProtoDecoder().ClearAutoCache()
	return saveProtoFiles(reg)
}

// RemoveProtoFile removes a .proto file.
func (a *App) RemoveProtoFile(id string) error {
	reg := getProtoRegistry()
	if err := reg.RemoveFile(id); err != nil {
		return err
	}
	getProtoDecoder().ClearAutoCache()
	return saveProtoFiles(reg)
}

// GetProtoFiles returns all loaded .proto files.
func (a *App) GetProtoFiles() []*ProtoFileEntry {
	reg := getProtoRegistry()
	files := reg.GetFiles()
	sort.Slice(files, func(i, j int) bool {
		return files[i].LoadedAt < files[j].LoadedAt
	})
	return files
}

// AddProtoMapping adds a URL→message type mapping.
func (a *App) AddProtoMapping(urlPattern, messageType, direction, description string) (string, error) {
	if urlPattern == "" || messageType == "" {
		return "", fmt.Errorf("urlPattern and messageType are required")
	}
	if direction == "" {
		direction = "response"
	}

	m := &ProtoMapping{
		ID:          uuid.New().String(),
		URLPattern:  urlPattern,
		MessageType: messageType,
		Direction:   direction,
		Description: description,
	}

	reg := getProtoRegistry()
	reg.AddMapping(m)

	getProtoDecoder().ClearAutoCache()
	return m.ID, saveProtoMappings(reg)
}

// UpdateProtoMapping updates an existing mapping.
func (a *App) UpdateProtoMapping(id, urlPattern, messageType, direction, description string) error {
	reg := getProtoRegistry()

	reg.mu.RLock()
	_, ok := reg.mappings[id]
	reg.mu.RUnlock()
	if !ok {
		return fmt.Errorf("mapping not found: %s", id)
	}

	m := &ProtoMapping{
		ID:          id,
		URLPattern:  urlPattern,
		MessageType: messageType,
		Direction:   direction,
		Description: description,
	}

	reg.AddMapping(m)
	getProtoDecoder().ClearAutoCache()
	return saveProtoMappings(reg)
}

// RemoveProtoMapping removes a URL mapping.
func (a *App) RemoveProtoMapping(id string) error {
	reg := getProtoRegistry()
	reg.RemoveMapping(id)
	getProtoDecoder().ClearAutoCache()
	return saveProtoMappings(reg)
}

// GetProtoMappings returns all URL→message mappings.
func (a *App) GetProtoMappings() []*ProtoMapping {
	reg := getProtoRegistry()
	return reg.GetMappings()
}

// GetProtoMessageTypes returns all available message type names from compiled protos.
func (a *App) GetProtoMessageTypes() []string {
	reg := getProtoRegistry()
	types := reg.GetAvailableMessageTypes()
	sort.Strings(types)
	return types
}

// LoadProtoFromURL fetches a .proto file from a remote URL and automatically
// resolves and downloads all `import` dependencies recursively.
// Well-known imports (google/protobuf/*) are skipped (handled by protocompile).
//
// Import resolution strategy:
//  1. Infer the proto root URL from the file's `package` declaration and URL path
//  2. Fall back to detecting path overlap between import path and URL
//  3. Fall back to resolving relative to the parent file's directory
//
// Returns (ids, error) for all added files.
func (a *App) LoadProtoFromURL(rawURL string) ([]string, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("URL is required")
	}

	reg := getProtoRegistry()
	existingNames := make(map[string]bool)
	reg.mu.RLock()
	for _, f := range reg.files {
		existingNames[f.Name] = true
	}
	reg.mu.RUnlock()

	// Collected files: importPath -> content
	collected := make(map[string]string)
	visited := make(map[string]bool) // importPath -> already processed

	// protoRoot is inferred from the first file's package declaration + URL.
	// Once set, all imports resolve relative to this root.
	var protoRoot string

	// fetchAndCollect downloads a proto file and recursively resolves its imports.
	// candidateURLs are tried in order; the first successful one is used.
	var fetchAndCollect func(importPath string, candidateURLs []string) error
	fetchAndCollect = func(importPath string, candidateURLs []string) error {
		if visited[importPath] {
			return nil
		}
		visited[importPath] = true

		// Try each candidate URL until one succeeds
		var content string
		var lastErr error
		var successURL string
		for _, url := range candidateURLs {
			var err error
			content, err = fetchProtoURL(url)
			if err == nil {
				successURL = url
				break
			}
			lastErr = err
			log.Printf("[Proto] Candidate URL failed for %s: %s (%v)", importPath, url, err)
		}
		if successURL == "" {
			return fmt.Errorf("failed to fetch %q (tried %d URLs): %w", importPath, len(candidateURLs), lastErr)
		}

		collected[importPath] = content

		// Infer proto root from package declaration (only needs to succeed once)
		if protoRoot == "" {
			if root := inferProtoRoot(successURL, content); root != "" {
				protoRoot = root
				log.Printf("[Proto] Inferred proto root: %s", protoRoot)
			}
		}

		// Recursively resolve imports
		for _, imp := range parseProtoImports(content) {
			// Skip well-known types (protocompile has built-in support)
			if strings.HasPrefix(imp, "google/protobuf/") {
				continue
			}
			// Skip if already in registry or collected
			if existingNames[imp] || collected[imp] != "" || visited[imp] {
				continue
			}

			// Build candidate URLs for this import (multiple strategies)
			candidates := buildImportURLCandidates(protoRoot, successURL, imp)
			if err := fetchAndCollect(imp, candidates); err != nil {
				return err
			}
		}

		return nil
	}

	// Start with the root file
	rootName := extractProtoFileName(rawURL)
	if err := fetchAndCollect(rootName, []string{rawURL}); err != nil {
		return nil, err
	}

	// Re-key the root file with its proper import path (package-based).
	// e.g., "check_error.proto" → "google/api/servicecontrol/v1/check_error.proto"
	// This ensures the proto compiler can find it by its canonical import path.
	if protoRoot != "" {
		suffix := strings.TrimPrefix(rawURL, protoRoot+"/")
		if suffix != rawURL && suffix != rootName {
			if content, ok := collected[rootName]; ok {
				delete(collected, rootName)
				collected[suffix] = content
				log.Printf("[Proto] Re-keyed root file: %s → %s", rootName, suffix)
			}
		}
	}

	// Add all collected files in dependency order.
	// Use a retry loop: each pass adds files whose dependencies are already resolved.
	// This handles arbitrary dependency graphs without explicit topological sort.
	remaining := make(map[string]string, len(collected))
	for name, content := range collected {
		remaining[name] = content
	}

	var ids []string
	var lastErrors []string // track last compile errors for user-facing messages
	for retry := 0; retry <= len(collected) && len(remaining) > 0; retry++ {
		progress := false
		lastErrors = nil // reset each pass
		for name, content := range remaining {
			id, err := a.AddProtoFile(name, content)
			if err != nil {
				lastErrors = append(lastErrors, fmt.Sprintf("%s: %v", name, err))
				continue
			}
			ids = append(ids, id)
			delete(remaining, name)
			progress = true
		}
		if !progress {
			break
		}
	}

	if len(ids) == 0 {
		errDetail := strings.Join(lastErrors, "; ")
		return nil, fmt.Errorf("all files failed: %s", errDetail)
	}
	return ids, nil
}

// inferProtoRoot infers the proto root URL from a file's URL and its package declaration.
//
// For example:
//
//	URL:     "https://raw.githubusercontent.com/googleapis/googleapis/master/google/api/annotations.proto"
//	package: "google.api"
//	→ canonical path = "google/api/annotations.proto"
//	→ root = "https://raw.githubusercontent.com/googleapis/googleapis/master"
func inferProtoRoot(fileURL, content string) string {
	pkg := parseProtoPackage(content)
	if pkg == "" {
		return ""
	}

	// Convert package to path: "google.api" → "google/api"
	pkgPath := strings.ReplaceAll(pkg, ".", "/")

	// Build canonical suffix: "/google/api/annotations.proto"
	filename := extractProtoFileName(fileURL)
	canonicalSuffix := "/" + pkgPath + "/" + filename

	if strings.HasSuffix(fileURL, canonicalSuffix) {
		return strings.TrimSuffix(fileURL, canonicalSuffix)
	}
	return ""
}

// buildImportURLCandidates generates candidate URLs for an import path using multiple strategies.
// Returns URLs in priority order (most likely correct first).
//
// Strategies:
//  1. Proto root (from package declaration inference) — most reliable
//  2. Path overlap detection — find the import path's first directory segment in the parent URL
//  3. Relative to parent directory — original fallback behavior
func buildImportURLCandidates(protoRoot, parentFileURL, importPath string) []string {
	var candidates []string
	seen := make(map[string]bool)
	add := func(url string) {
		if url != "" && !seen[url] && strings.Contains(url, "://") {
			seen[url] = true
			candidates = append(candidates, url)
		}
	}

	// Strategy 1: Use inferred proto root (most reliable when available)
	if protoRoot != "" {
		add(protoRoot + "/" + importPath)
	}

	// Strategy 2: Detect path overlap between import path and parent URL.
	// If the import is "google/api/http.proto" and parent URL contains "/google/api/...",
	// we find that overlap to compute the root without doubling the path.
	parentDir := parentFileURL
	if idx := strings.LastIndex(parentDir, "/"); idx > 0 {
		parentDir = parentDir[:idx]
	}

	importFirstSeg := importPath
	if idx := strings.IndexByte(importPath, '/'); idx > 0 {
		importFirstSeg = importPath[:idx]
	}
	// Search from the right to find the best match
	searchNeedle := "/" + importFirstSeg + "/"
	if idx := strings.LastIndex(parentDir, searchNeedle); idx >= 0 {
		root := parentDir[:idx]
		add(root + "/" + importPath)
	}
	// Also check if parentDir ends with the first segment (no trailing slash)
	searchNeedleEnd := "/" + importFirstSeg
	if strings.HasSuffix(parentDir, searchNeedleEnd) {
		root := strings.TrimSuffix(parentDir, searchNeedleEnd)
		add(root + "/" + importPath)
	}

	// Strategy 3: Relative to parent file's directory (original behavior, acts as fallback).
	// Only add if the import's directory prefix is NOT already present in parentDir,
	// otherwise we'd create a doubled path like ".../google/api/google/api/http.proto".
	relativeURL := parentDir + "/" + importPath
	if importDirIdx := strings.LastIndex(importPath, "/"); importDirIdx > 0 {
		importDir := importPath[:importDirIdx] // e.g., "google/api"
		// Check if parentDir already ends with any prefix of importDir's segments.
		// If the first segment of the import (e.g., "google") already appears in parentDir,
		// then relative resolution will likely double the path.
		if !strings.Contains(parentDir, "/"+importFirstSeg+"/") &&
			!strings.HasSuffix(parentDir, "/"+importDir) {
			add(relativeURL)
		}
	} else {
		// Simple filename import (no directory), relative resolution is always safe
		add(relativeURL)
	}

	return candidates
}

// parseProtoPackage extracts the package name from .proto file content.
// Returns empty string if no package declaration is found.
func parseProtoPackage(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			// Extract: package google.api;
			line = strings.TrimPrefix(line, "package ")
			line = strings.TrimSuffix(strings.TrimSpace(line), ";")
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// fetchProtoURL downloads content from a URL with proper timeout, headers, and size limit.
func fetchProtoURL(rawURL string) (string, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", "Gaze/1.0 (proto-loader)")
	req.Header.Set("Accept", "text/plain, application/octet-stream, */*")

	resp, err := protoHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request to %s failed: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
	}

	// Limit body to 10MB to prevent memory issues
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", rawURL, err)
	}

	content := string(body)
	if len(strings.TrimSpace(content)) == 0 {
		return "", fmt.Errorf("empty content from %s", rawURL)
	}
	return content, nil
}

// parseProtoImports extracts import paths from .proto file content.
// Matches: import "path/to/file.proto"; and import public "path/to/file.proto";
func parseProtoImports(content string) []string {
	var imports []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		// Match: import "..." or import public "..."
		if !strings.HasPrefix(line, "import ") {
			continue
		}
		// Extract the quoted path
		q1 := strings.IndexByte(line, '"')
		if q1 < 0 {
			continue
		}
		q2 := strings.IndexByte(line[q1+1:], '"')
		if q2 < 0 {
			continue
		}
		imp := line[q1+1 : q1+1+q2]
		if imp != "" {
			imports = append(imports, imp)
		}
	}
	return imports
}

// extractProtoFileName extracts a clean filename from a URL.
// Uses path.Base (not filepath.Base) for correct URL path handling on all platforms.
func extractProtoFileName(rawURL string) string {
	// Strip query string first, then extract base name
	clean := rawURL
	if idx := strings.IndexByte(clean, '?'); idx != -1 {
		clean = clean[:idx]
	}
	name := path.Base(clean)
	if !strings.HasSuffix(name, ".proto") {
		name += ".proto"
	}
	return name
}

// LoadProtoFromDisk opens a file dialog to select local .proto files and adds them.
// Automatically resolves `import` dependencies from the same directory tree.
// Returns ([]id, error) for all successfully added files.
func (a *App) LoadProtoFromDisk() ([]string, error) {
	paths, err := wailsRuntime.OpenMultipleFilesDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Proto Files",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Proto Files (*.proto)", Pattern: "*.proto"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, nil // user cancelled
	}

	reg := getProtoRegistry()
	existingNames := make(map[string]bool)
	reg.mu.RLock()
	for _, f := range reg.files {
		existingNames[f.Name] = true
	}
	reg.mu.RUnlock()

	// Collect all files: importPath -> content
	collected := make(map[string]string)
	visited := make(map[string]bool) // absolute paths already read

	// Determine a common base directory from selected files for resolving imports
	// Use the directory of the first selected file as base
	baseDir := filepath.Dir(paths[0])

	var resolveLocal func(absPath, importPath string)
	resolveLocal = func(absPath, importPath string) {
		if visited[absPath] {
			return
		}
		visited[absPath] = true

		data, err := os.ReadFile(absPath)
		if err != nil {
			log.Printf("[Proto] Warning: cannot read %s: %v", importPath, err)
			return
		}
		content := string(data)
		if len(strings.TrimSpace(content)) == 0 {
			return
		}
		collected[importPath] = content

		// Resolve imports relative to the base directory
		dir := filepath.Dir(absPath)
		for _, imp := range parseProtoImports(content) {
			if strings.HasPrefix(imp, "google/protobuf/") {
				continue
			}
			if existingNames[imp] || collected[imp] != "" {
				continue
			}
			// Try resolving relative to current file's directory first, then base dir
			impAbs := filepath.Join(dir, imp)
			if _, err := os.Stat(impAbs); err != nil {
				impAbs = filepath.Join(baseDir, imp)
				if _, err := os.Stat(impAbs); err != nil {
					log.Printf("[Proto] Warning: import %q not found locally", imp)
					continue
				}
			}
			resolveLocal(impAbs, imp)
		}
	}

	// Process each selected file and resolve its dependencies
	for _, p := range paths {
		name := filepath.Base(p)
		resolveLocal(p, name)
	}

	// Add all collected files
	var ids []string
	var addErrors []string
	for name, content := range collected {
		id, err := a.AddProtoFile(name, content)
		if err != nil {
			addErrors = append(addErrors, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		ids = append(ids, id)
	}

	if len(addErrors) > 0 && len(ids) == 0 {
		return nil, fmt.Errorf("all files failed: %s", strings.Join(addErrors, "; "))
	}
	return ids, nil
}

// --- Persistence helpers ---

func saveProtoFiles(reg *ProtoRegistry) error {
	protoLoadFailedMu.Lock()
	failed := protoFilesLoadFailed
	protoLoadFailedMu.Unlock()
	if failed {
		return fmt.Errorf("proto files config failed to load at startup; refusing to overwrite (restart to retry)")
	}

	files := reg.GetFiles()
	sort.Slice(files, func(i, j int) bool {
		return files[i].LoadedAt < files[j].LoadedAt
	})

	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}

	dir := getProtoDir()
	_ = os.MkdirAll(dir, 0755)
	return fileutil.WriteFileAtomic(getProtoFilesPath(), data, 0644)
}

func saveProtoMappings(reg *ProtoRegistry) error {
	protoLoadFailedMu.Lock()
	failed := protoMappingsLoadFailed
	protoLoadFailedMu.Unlock()
	if failed {
		return fmt.Errorf("proto mappings config failed to load at startup; refusing to overwrite (restart to retry)")
	}

	mappings := reg.GetMappings()

	data, err := json.MarshalIndent(mappings, "", "  ")
	if err != nil {
		return err
	}

	dir := getProtoDir()
	_ = os.MkdirAll(dir, 0755)
	return fileutil.WriteFileAtomic(getProtoMappingsPath(), data, 0644)
}
