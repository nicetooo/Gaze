package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// setupTestDB 创建内存数据库并初始化 PluginStore schema
// pruneVersions 在 SavePlugin 事务内通过 tx 执行，内存库与生产 WAL 库行为一致
func setupTestDB(t *testing.T) (*sql.DB, *PluginStore) {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=ON&_busy_timeout=100")
	if err != nil {
		t.Fatalf("Failed to open in-memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store := NewPluginStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatalf("Failed to init schema: %v", err)
	}
	return db, store
}

// newTestPlugin 创建一个最小化的测试用 Plugin
func newTestPlugin(id, name string) *Plugin {
	return &Plugin{
		Metadata: PluginMetadata{
			ID:          id,
			Name:        name,
			Version:     "1.0.0",
			Author:      "test-author",
			Description: "test plugin",
			Enabled:     true,
			Filters: PluginFilters{
				Sources: []string{"network"},
			},
			Config: map[string]interface{}{"key": "value"},
		},
		SourceCode:   `const plugin = { onEvent: (e, ctx) => null };`,
		Language:     "typescript",
		CompiledCode: `const plugin = { onEvent: (e, ctx) => null };`,
		State:        make(map[string]interface{}),
	}
}

// ========== InitSchema 测试 ==========

func TestPluginStore_InitSchema(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	store := NewPluginStore(db)

	// 第一次初始化
	if err := store.InitSchema(); err != nil {
		t.Fatalf("First InitSchema failed: %v", err)
	}

	// 验证表存在
	tables := []string{"plugins", "plugin_versions"}
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to check table %s: %v", table, err)
		}
		if count != 1 {
			t.Errorf("Table %s not found", table)
		}
	}

	// 第二次初始化应该是幂等的 (CREATE IF NOT EXISTS)
	if err := store.InitSchema(); err != nil {
		t.Fatalf("Second InitSchema (idempotent) failed: %v", err)
	}
}

// ========== SavePlugin 测试 ==========

func TestPluginStore_SavePlugin_Insert(t *testing.T) {
	_, store := setupTestDB(t)

	plugin := newTestPlugin("test-1", "Test Plugin 1")
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin (insert) failed: %v", err)
	}

	// 验证插入成功
	got, err := store.GetPlugin("test-1")
	if err != nil {
		t.Fatalf("GetPlugin failed: %v", err)
	}

	if got.Metadata.ID != "test-1" {
		t.Errorf("ID = %q, want %q", got.Metadata.ID, "test-1")
	}
	if got.Metadata.Name != "Test Plugin 1" {
		t.Errorf("Name = %q, want %q", got.Metadata.Name, "Test Plugin 1")
	}
	if got.Metadata.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", got.Metadata.Version, "1.0.0")
	}
	if got.Metadata.Author != "test-author" {
		t.Errorf("Author = %q, want %q", got.Metadata.Author, "test-author")
	}
	if !got.Metadata.Enabled {
		t.Error("Enabled = false, want true")
	}
	if len(got.Metadata.Filters.Sources) != 1 || got.Metadata.Filters.Sources[0] != "network" {
		t.Errorf("Filters.Sources = %v, want [network]", got.Metadata.Filters.Sources)
	}
	if got.Metadata.Config["key"] != "value" {
		t.Errorf("Config[key] = %v, want 'value'", got.Metadata.Config["key"])
	}
	if got.SourceCode != plugin.SourceCode {
		t.Errorf("SourceCode mismatch")
	}
	if got.CompiledCode != plugin.CompiledCode {
		t.Errorf("CompiledCode mismatch")
	}
	if got.Language != "typescript" {
		t.Errorf("Language = %q, want %q", got.Language, "typescript")
	}
}

func TestPluginStore_SavePlugin_Update(t *testing.T) {
	_, store := setupTestDB(t)

	// 先插入
	plugin := newTestPlugin("test-update", "Original Name")
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin (insert) failed: %v", err)
	}

	// 再更新
	plugin.Metadata.Name = "Updated Name"
	plugin.Metadata.Version = "2.0.0"
	plugin.CompiledCode = `const plugin = { onEvent: (e, ctx) => ({ derivedEvents: [] }) };`
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin (update) failed: %v", err)
	}

	// 验证更新后的数据
	got, err := store.GetPlugin("test-update")
	if err != nil {
		t.Fatalf("GetPlugin failed: %v", err)
	}
	if got.Metadata.Name != "Updated Name" {
		t.Errorf("Name = %q, want %q", got.Metadata.Name, "Updated Name")
	}
	if got.Metadata.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", got.Metadata.Version, "2.0.0")
	}
}

func TestPluginStore_SavePlugin_UpdateCreatesVersionHistory(t *testing.T) {
	_, store := setupTestDB(t)

	// 插入 v1
	plugin := newTestPlugin("test-version", "Version Test")
	plugin.Metadata.Version = "1.0.0"
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin v1 failed: %v", err)
	}

	// 更新到 v2
	plugin.Metadata.Version = "2.0.0"
	plugin.Metadata.Name = "Version Test v2"
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin v2 failed: %v", err)
	}

	// 验证版本历史有 v1
	versions, err := store.GetPluginVersions("test-version", 10)
	if err != nil {
		t.Fatalf("GetPluginVersions failed: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("Expected 1 version in history, got %d", len(versions))
	}
	if versions[0].Version != "1.0.0" {
		t.Errorf("History version = %q, want %q", versions[0].Version, "1.0.0")
	}
	if versions[0].Name != "Version Test" {
		t.Errorf("History name = %q, want %q", versions[0].Name, "Version Test")
	}

	// 更新到 v3
	plugin.Metadata.Version = "3.0.0"
	plugin.Metadata.Name = "Version Test v3"
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin v3 failed: %v", err)
	}

	// 验证版本历史有 v1 和 v2
	versions, err = store.GetPluginVersions("test-version", 10)
	if err != nil {
		t.Fatalf("GetPluginVersions failed: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("Expected 2 versions in history, got %d", len(versions))
	}
	// 验证两个历史版本都在 (v1 和 v2)，不依赖顺序
	versionSet := map[string]bool{}
	for _, v := range versions {
		versionSet[v.Version] = true
	}
	if !versionSet["1.0.0"] {
		t.Error("Missing v1.0.0 in version history")
	}
	if !versionSet["2.0.0"] {
		t.Error("Missing v2.0.0 in version history")
	}
}

// ========== GetPlugin 测试 ==========

func TestPluginStore_GetPlugin_NotFound(t *testing.T) {
	_, store := setupTestDB(t)

	_, err := store.GetPlugin("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent plugin, got nil")
	}
}

func TestPluginStore_GetPlugin_DisabledPlugin(t *testing.T) {
	_, store := setupTestDB(t)

	plugin := newTestPlugin("disabled-1", "Disabled Plugin")
	plugin.Metadata.Enabled = false
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin failed: %v", err)
	}

	got, err := store.GetPlugin("disabled-1")
	if err != nil {
		t.Fatalf("GetPlugin failed: %v", err)
	}
	if got.Metadata.Enabled {
		t.Error("Enabled = true, want false")
	}
}

// ========== ListPlugins 测试 ==========

func TestPluginStore_ListPlugins_Empty(t *testing.T) {
	_, store := setupTestDB(t)

	plugins, err := store.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins failed: %v", err)
	}
	if len(plugins) != 0 {
		t.Errorf("Expected 0 plugins, got %d", len(plugins))
	}
}

func TestPluginStore_ListPlugins_Multiple(t *testing.T) {
	_, store := setupTestDB(t)

	for i := 0; i < 3; i++ {
		plugin := newTestPlugin(
			"plugin-"+string(rune('a'+i)),
			"Plugin "+string(rune('A'+i)),
		)
		if err := store.SavePlugin(plugin); err != nil {
			t.Fatalf("SavePlugin %d failed: %v", i, err)
		}
	}

	plugins, err := store.ListPlugins()
	if err != nil {
		t.Fatalf("ListPlugins failed: %v", err)
	}
	if len(plugins) != 3 {
		t.Errorf("Expected 3 plugins, got %d", len(plugins))
	}

	// 验证每个插件都有初始化的 State
	for _, p := range plugins {
		if p.State == nil {
			t.Errorf("Plugin %s State is nil, should be initialized", p.Metadata.ID)
		}
	}
}

// ========== DeletePlugin 测试 ==========

func TestPluginStore_DeletePlugin(t *testing.T) {
	_, store := setupTestDB(t)

	plugin := newTestPlugin("delete-me", "To Be Deleted")
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin failed: %v", err)
	}

	// 确认存在
	_, err := store.GetPlugin("delete-me")
	if err != nil {
		t.Fatalf("Plugin should exist before delete: %v", err)
	}

	// 删除
	if err := store.DeletePlugin("delete-me"); err != nil {
		t.Fatalf("DeletePlugin failed: %v", err)
	}

	// 确认已删除
	_, err = store.GetPlugin("delete-me")
	if err == nil {
		t.Fatal("Expected error after delete, got nil")
	}
}

func TestPluginStore_DeletePlugin_NonExistent(t *testing.T) {
	_, store := setupTestDB(t)

	// 删除不存在的插件不应报错 (DELETE WHERE id = ? 影响 0 行)
	if err := store.DeletePlugin("nonexistent"); err != nil {
		t.Fatalf("DeletePlugin on nonexistent should not error, got: %v", err)
	}
}

// ========== GetPluginVersions 测试 ==========

func TestPluginStore_GetPluginVersions_NoVersions(t *testing.T) {
	_, store := setupTestDB(t)

	// 新建插件还没有更新过，不应有版本历史
	plugin := newTestPlugin("no-versions", "No Versions")
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin failed: %v", err)
	}

	versions, err := store.GetPluginVersions("no-versions", 10)
	if err != nil {
		t.Fatalf("GetPluginVersions failed: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("Expected 0 versions, got %d", len(versions))
	}
}

func TestPluginStore_GetPluginVersions_DefaultLimit(t *testing.T) {
	_, store := setupTestDB(t)

	// limit <= 0 时默认为 10
	versions, err := store.GetPluginVersions("any-id", 0)
	if err != nil {
		t.Fatalf("GetPluginVersions with limit=0 failed: %v", err)
	}
	if versions == nil {
		// 即使没有数据，也不应该返回 nil + error
		// 实际返回值取决于实现，空切片或 nil 都可接受
	}
	_ = versions // 只要不 panic 就行
}

// ========== RollbackPlugin 测试 ==========

func TestPluginStore_RollbackPlugin(t *testing.T) {
	_, store := setupTestDB(t)

	// 创建 v1
	plugin := newTestPlugin("rollback-test", "Rollback V1")
	plugin.Metadata.Version = "1.0.0"
	plugin.CompiledCode = `const plugin = { onEvent: () => null }; // v1`
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin v1 failed: %v", err)
	}

	// 更新到 v2 (产生一条版本历史)
	plugin.Metadata.Version = "2.0.0"
	plugin.Metadata.Name = "Rollback V2"
	plugin.CompiledCode = `const plugin = { onEvent: () => null }; // v2`
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin v2 failed: %v", err)
	}

	// 获取版本历史
	versions, err := store.GetPluginVersions("rollback-test", 10)
	if err != nil {
		t.Fatalf("GetPluginVersions failed: %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("Expected at least 1 version in history")
	}

	// 执行回滚到 v1
	if err := store.RollbackPlugin(versions[0].ID); err != nil {
		t.Fatalf("RollbackPlugin failed: %v", err)
	}

	// 验证当前版本已回滚
	got, err := store.GetPlugin("rollback-test")
	if err != nil {
		t.Fatalf("GetPlugin after rollback failed: %v", err)
	}
	if got.Metadata.Version != "1.0.0" {
		t.Errorf("Version after rollback = %q, want %q", got.Metadata.Version, "1.0.0")
	}
	if got.Metadata.Name != "Rollback V1" {
		t.Errorf("Name after rollback = %q, want %q", got.Metadata.Name, "Rollback V1")
	}
}

func TestPluginStore_RollbackPlugin_NonExistentVersion(t *testing.T) {
	_, store := setupTestDB(t)

	err := store.RollbackPlugin(99999)
	if err == nil {
		t.Fatal("Expected error for nonexistent version ID, got nil")
	}
}

// ========== PruneVersions 测试 ==========

func TestPluginStore_PruneVersions(t *testing.T) {
	db, store := setupTestDB(t)

	plugin := newTestPlugin("prune-test", "Prune Test")

	// 创建多个版本：初始 + 5 次更新 = 5 个版本历史
	for i := 0; i < 6; i++ {
		plugin.Metadata.Version = string(rune('0'+i)) + ".0.0"
		plugin.Metadata.Name = "Prune Test v" + string(rune('0'+i))
		if err := store.SavePlugin(plugin); err != nil {
			t.Fatalf("SavePlugin iteration %d failed: %v", i, err)
		}
	}

	// 验证有 5 个版本历史 (第一次是 insert，不产生版本历史)
	versions, err := store.GetPluginVersions("prune-test", 100)
	if err != nil {
		t.Fatalf("GetPluginVersions failed: %v", err)
	}
	if len(versions) != 5 {
		t.Errorf("Expected 5 versions before prune, got %d", len(versions))
	}

	// 手动在事务内调用 pruneVersions，保留 2 个
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin tx failed: %v", err)
	}
	store.pruneVersions(tx, "prune-test", 2)
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// 验证只剩 2 个
	versions, err = store.GetPluginVersions("prune-test", 100)
	if err != nil {
		t.Fatalf("GetPluginVersions after prune failed: %v", err)
	}
	if len(versions) != 2 {
		t.Errorf("Expected 2 versions after prune, got %d", len(versions))
	}
}

// TestPluginStore_SavePlugin_WALPruneRegression 回归测试 (PL-1):
// pruneVersions 曾在 SavePlugin 未提交事务期间用连接池另一条连接执行 DELETE，
// 在生产 WAL + _busy_timeout=5000 DSN 下每次更新固定阻塞 ~5s 且清理永不生效。
// 此测试用与生产一致的 DSN 断言：更新不再阻塞 busy_timeout，且版本数被裁剪到上限 (20)。
func TestPluginStore_SavePlugin_WALPruneRegression(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "plugins.db")
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("Failed to open WAL db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store := NewPluginStore(db)
	if err := store.InitSchema(); err != nil {
		t.Fatalf("InitSchema failed: %v", err)
	}

	plugin := newTestPlugin("wal-prune", "WAL Prune")
	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin (insert) failed: %v", err)
	}

	// 25 次更新产生 25 条版本历史，超出 20 的保留上限
	start := time.Now()
	for i := 0; i < 25; i++ {
		plugin.Metadata.Version = fmt.Sprintf("1.0.%d", i+1)
		if err := store.SavePlugin(plugin); err != nil {
			t.Fatalf("SavePlugin (update %d) failed: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// 修复前单次更新即阻塞 ~5.2s（25 次 ≈ 130s）；修复后整组更新应远低于此
	if elapsed > 10*time.Second {
		t.Errorf("25 updates took %v, prune DELETE is likely blocking on busy_timeout again", elapsed)
	}

	versions, err := store.GetPluginVersions("wal-prune", 100)
	if err != nil {
		t.Fatalf("GetPluginVersions failed: %v", err)
	}
	if len(versions) != 20 {
		t.Errorf("Expected versions pruned to 20, got %d", len(versions))
	}
}

// ========== SavePlugin 完整性测试 ==========

func TestPluginStore_SavePlugin_FiltersAndConfig(t *testing.T) {
	_, store := setupTestDB(t)

	plugin := newTestPlugin("complex-filters", "Complex Filters")
	plugin.Metadata.Filters = PluginFilters{
		Sources:    []string{"network", "logcat"},
		Types:      []string{"http_request", "websocket_message"},
		Levels:     []string{"error", "warn"},
		URLPattern: "*/api/v1/*",
		TitleMatch: "^GET .+",
	}
	plugin.Metadata.Config = map[string]interface{}{
		"timeout":     5000.0,
		"retries":     3.0,
		"enableDebug": true,
		"tags":        []interface{}{"a", "b"},
	}

	if err := store.SavePlugin(plugin); err != nil {
		t.Fatalf("SavePlugin failed: %v", err)
	}

	got, err := store.GetPlugin("complex-filters")
	if err != nil {
		t.Fatalf("GetPlugin failed: %v", err)
	}

	// 验证 Filters
	f := got.Metadata.Filters
	if len(f.Sources) != 2 || f.Sources[0] != "network" || f.Sources[1] != "logcat" {
		t.Errorf("Filters.Sources = %v", f.Sources)
	}
	if len(f.Types) != 2 {
		t.Errorf("Filters.Types = %v", f.Types)
	}
	if len(f.Levels) != 2 {
		t.Errorf("Filters.Levels = %v", f.Levels)
	}
	if f.URLPattern != "*/api/v1/*" {
		t.Errorf("Filters.URLPattern = %q", f.URLPattern)
	}
	if f.TitleMatch != "^GET .+" {
		t.Errorf("Filters.TitleMatch = %q", f.TitleMatch)
	}

	// 验证 Config
	c := got.Metadata.Config
	if c["timeout"] != 5000.0 {
		t.Errorf("Config.timeout = %v", c["timeout"])
	}
	if c["enableDebug"] != true {
		t.Errorf("Config.enableDebug = %v", c["enableDebug"])
	}
}
