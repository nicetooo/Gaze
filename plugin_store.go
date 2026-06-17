package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"
)

// ErrPluginNotFound 标识插件不存在（区别于数据库瞬时错误），供调用方 errors.Is 判定
var ErrPluginNotFound = errors.New("plugin not found")

// PluginStore 插件数据库存储
type PluginStore struct {
	db *sql.DB
}

// NewPluginStore 创建插件存储
func NewPluginStore(db *sql.DB) *PluginStore {
	return &PluginStore{db: db}
}

// InitSchema 初始化数据库表
func (ps *PluginStore) InitSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS plugins (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		version TEXT NOT NULL,
		author TEXT,
		description TEXT,
		
		-- 源代码
		source_code TEXT NOT NULL,
		source_language TEXT DEFAULT 'typescript',
		compiled_code TEXT NOT NULL,
		
		-- 配置
		filters TEXT,
		config TEXT,
		
		-- 状态
		enabled INTEGER DEFAULT 1,
		
		-- 时间戳
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL
	);

	CREATE TABLE IF NOT EXISTS plugin_versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		plugin_id TEXT NOT NULL,
		version TEXT NOT NULL,
		name TEXT NOT NULL,
		author TEXT,
		description TEXT,
		source_code TEXT NOT NULL,
		source_language TEXT DEFAULT 'typescript',
		compiled_code TEXT NOT NULL,
		filters TEXT,
		config TEXT,
		saved_at INTEGER NOT NULL,
		FOREIGN KEY (plugin_id) REFERENCES plugins(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_plugin_versions_plugin_id ON plugin_versions(plugin_id);
	CREATE INDEX IF NOT EXISTS idx_plugin_versions_saved_at ON plugin_versions(saved_at);

	-- plugins 表索引
	CREATE INDEX IF NOT EXISTS idx_plugins_enabled ON plugins(enabled);
	CREATE INDEX IF NOT EXISTS idx_plugins_created_at ON plugins(created_at);
	CREATE INDEX IF NOT EXISTS idx_plugins_updated_at ON plugins(updated_at);
	`

	_, err := ps.db.Exec(schema)
	return err
}

// SavePlugin 保存插件
func (ps *PluginStore) SavePlugin(plugin *Plugin) error {
	filtersJSON, err := json.Marshal(plugin.Metadata.Filters)
	if err != nil {
		return fmt.Errorf("序列化 filters 失败: %w", err)
	}

	configJSON, err := json.Marshal(plugin.Metadata.Config)
	if err != nil {
		return fmt.Errorf("序列化 config 失败: %w", err)
	}

	now := time.Now().Unix()
	enabled := 0
	if plugin.Metadata.Enabled {
		enabled = 1
	}

	// 检查是否已存在
	var exists int
	err = ps.db.QueryRow("SELECT COUNT(*) FROM plugins WHERE id = ?", plugin.Metadata.ID).Scan(&exists)
	if err != nil {
		return err
	}

	if exists > 0 {
		// 使用事务保护更新操作（版本历史 + 更新当前记录）
		tx, txErr := ps.db.Begin()
		if txErr != nil {
			return fmt.Errorf("开始事务失败: %w", txErr)
		}
		defer func() {
			if txErr != nil {
				tx.Rollback()
			}
		}()

		// 更新前先保存旧版本到历史表
		var oldPlugin Plugin
		var oldFiltersJSON, oldConfigJSON string
		var oldEnabled int
		err = tx.QueryRow(`
			SELECT name, version, author, description,
				   source_code, source_language, compiled_code,
				   filters, config, enabled
			FROM plugins WHERE id = ?
		`, plugin.Metadata.ID).Scan(
			&oldPlugin.Metadata.Name,
			&oldPlugin.Metadata.Version,
			&oldPlugin.Metadata.Author,
			&oldPlugin.Metadata.Description,
			&oldPlugin.SourceCode,
			&oldPlugin.Language,
			&oldPlugin.CompiledCode,
			&oldFiltersJSON,
			&oldConfigJSON,
			&oldEnabled,
		)

		if err == nil {
			// 保存旧版本到历史表
			_, err = tx.Exec(`
				INSERT INTO plugin_versions (
					plugin_id, version, name, author, description,
					source_code, source_language, compiled_code,
					filters, config, saved_at
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`,
				plugin.Metadata.ID,
				oldPlugin.Metadata.Version,
				oldPlugin.Metadata.Name,
				oldPlugin.Metadata.Author,
				oldPlugin.Metadata.Description,
				oldPlugin.SourceCode,
				oldPlugin.Language,
				oldPlugin.CompiledCode,
				oldFiltersJSON,
				oldConfigJSON,
				now,
			)
			if err != nil {
				log.Printf("[PluginStore] Failed to save old version: %v", err)
				// 继续执行更新，不因历史保存失败而阻止更新
			}

			// 自动清理旧版本：保留最近 maxVersions 个
			ps.pruneVersions(tx, plugin.Metadata.ID, 20)
		}

		// 更新
		_, txErr = tx.Exec(`
			UPDATE plugins SET
				name = ?,
				version = ?,
				author = ?,
				description = ?,
				source_code = ?,
				source_language = ?,
				compiled_code = ?,
				filters = ?,
				config = ?,
				enabled = ?,
				updated_at = ?
			WHERE id = ?
		`,
			plugin.Metadata.Name,
			plugin.Metadata.Version,
			plugin.Metadata.Author,
			plugin.Metadata.Description,
			plugin.SourceCode,
			plugin.Language,
			plugin.CompiledCode,
			string(filtersJSON),
			string(configJSON),
			enabled,
			now,
			plugin.Metadata.ID,
		)
		if txErr != nil {
			return txErr
		}

		return tx.Commit()
	} else {
		// 插入
		_, err = ps.db.Exec(`
			INSERT INTO plugins (
				id, name, version, author, description,
				source_code, source_language, compiled_code,
				filters, config, enabled,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			plugin.Metadata.ID,
			plugin.Metadata.Name,
			plugin.Metadata.Version,
			plugin.Metadata.Author,
			plugin.Metadata.Description,
			plugin.SourceCode,
			plugin.Language,
			plugin.CompiledCode,
			string(filtersJSON),
			string(configJSON),
			enabled,
			now,
			now,
		)
	}

	return err
}

// GetPlugin 获取插件
func (ps *PluginStore) GetPlugin(id string) (*Plugin, error) {
	var (
		name         string
		version      string
		author       string
		description  string
		sourceCode   string
		language     string
		compiledCode string
		filtersJSON  string
		configJSON   string
		enabled      int
		createdAt    int64
		updatedAt    int64
	)

	err := ps.db.QueryRow(`
		SELECT name, version, author, description,
			   source_code, source_language, compiled_code,
			   filters, config, enabled,
			   created_at, updated_at
		FROM plugins
		WHERE id = ?
	`, id).Scan(
		&name, &version, &author, &description,
		&sourceCode, &language, &compiledCode,
		&filtersJSON, &configJSON, &enabled,
		&createdAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("插件不存在: %s: %w", id, ErrPluginNotFound)
	}
	if err != nil {
		return nil, err
	}

	var filters PluginFilters
	if err := json.Unmarshal([]byte(filtersJSON), &filters); err != nil {
		return nil, fmt.Errorf("解析 filters 失败: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return nil, fmt.Errorf("解析 config 失败: %w", err)
	}

	plugin := &Plugin{
		Metadata: PluginMetadata{
			ID:          id,
			Name:        name,
			Version:     version,
			Author:      author,
			Description: description,
			Enabled:     enabled == 1,
			Filters:     filters,
			Config:      config,
			CreatedAt:   time.Unix(createdAt, 0),
			UpdatedAt:   time.Unix(updatedAt, 0),
		},
		SourceCode:   sourceCode,
		Language:     language,
		CompiledCode: compiledCode,
		State:        make(map[string]interface{}),
	}

	return plugin, nil
}

// ListPlugins 列出所有插件
func (ps *PluginStore) ListPlugins() ([]*Plugin, error) {
	rows, err := ps.db.Query(`
		SELECT id, name, version, author, description,
			   source_code, source_language, compiled_code,
			   filters, config, enabled,
			   created_at, updated_at
		FROM plugins
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []*Plugin

	for rows.Next() {
		var (
			id           string
			name         string
			version      string
			author       string
			description  string
			sourceCode   string
			language     string
			compiledCode string
			filtersJSON  string
			configJSON   string
			enabled      int
			createdAt    int64
			updatedAt    int64
		)

		err := rows.Scan(
			&id, &name, &version, &author, &description,
			&sourceCode, &language, &compiledCode,
			&filtersJSON, &configJSON, &enabled,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		var filters PluginFilters
		if err := json.Unmarshal([]byte(filtersJSON), &filters); err != nil {
			log.Printf("[PluginStore] Skipping plugin %s: invalid filters JSON: %v", id, err)
			continue
		}

		var config map[string]interface{}
		if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
			log.Printf("[PluginStore] Skipping plugin %s: invalid config JSON: %v", id, err)
			continue
		}

		plugin := &Plugin{
			Metadata: PluginMetadata{
				ID:          id,
				Name:        name,
				Version:     version,
				Author:      author,
				Description: description,
				Enabled:     enabled == 1,
				Filters:     filters,
				Config:      config,
				CreatedAt:   time.Unix(createdAt, 0),
				UpdatedAt:   time.Unix(updatedAt, 0),
			},
			SourceCode:   sourceCode,
			Language:     language,
			CompiledCode: compiledCode,
			State:        make(map[string]interface{}),
		}

		plugins = append(plugins, plugin)
	}

	return plugins, rows.Err()
}

// DeletePlugin 删除插件
func (ps *PluginStore) DeletePlugin(id string) error {
	_, err := ps.db.Exec("DELETE FROM plugins WHERE id = ?", id)
	return err
}

// pruneVersions 清理旧版本，保留最近 maxVersions 个。
// 必须在调用方的写事务内执行：SQLite 单写者模型下，若改用连接池中
// 另一条连接执行 DELETE，会被未提交事务持有的写锁阻塞到 busy_timeout 才失败。
func (ps *PluginStore) pruneVersions(tx *sql.Tx, pluginID string, maxVersions int) {
	_, err := tx.Exec(`
		DELETE FROM plugin_versions
		WHERE plugin_id = ? AND id NOT IN (
			SELECT id FROM plugin_versions
			WHERE plugin_id = ?
			ORDER BY saved_at DESC
			LIMIT ?
		)
	`, pluginID, pluginID, maxVersions)
	if err != nil {
		log.Printf("[PluginStore] Failed to prune old versions for %s: %v", pluginID, err)
	}
}

// PluginVersion 插件历史版本
type PluginVersion struct {
	ID          int       `json:"id"`
	PluginID    string    `json:"pluginId"`
	Version     string    `json:"version"`
	Name        string    `json:"name"`
	Author      string    `json:"author"`
	Description string    `json:"description"`
	SavedAt     time.Time `json:"savedAt"`
}

// GetPluginVersions 获取插件的历史版本列表
func (ps *PluginStore) GetPluginVersions(pluginID string, limit int) ([]PluginVersion, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := ps.db.Query(`
		SELECT id, plugin_id, version, name, author, description, saved_at
		FROM plugin_versions
		WHERE plugin_id = ?
		ORDER BY saved_at DESC
		LIMIT ?
	`, pluginID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []PluginVersion
	for rows.Next() {
		var v PluginVersion
		var savedAt int64
		err := rows.Scan(&v.ID, &v.PluginID, &v.Version, &v.Name, &v.Author, &v.Description, &savedAt)
		if err != nil {
			continue
		}
		v.SavedAt = time.Unix(savedAt, 0)
		versions = append(versions, v)
	}

	return versions, rows.Err()
}

// RollbackPlugin 回滚到指定版本
// "备份当前版本 + 更新为历史版本" 在同一事务内执行，避免只完成一半
func (ps *PluginStore) RollbackPlugin(versionID int) error {
	tx, err := ps.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback() // Commit 成功后为 no-op

	// 获取历史版本数据
	var (
		pluginID     string
		version      string
		name         string
		author       string
		description  string
		sourceCode   string
		language     string
		compiledCode string
		filtersJSON  string
		configJSON   string
	)

	err = tx.QueryRow(`
		SELECT plugin_id, version, name, author, description,
			   source_code, source_language, compiled_code,
			   filters, config
		FROM plugin_versions
		WHERE id = ?
	`, versionID).Scan(
		&pluginID, &version, &name, &author, &description,
		&sourceCode, &language, &compiledCode,
		&filtersJSON, &configJSON,
	)
	if err != nil {
		return fmt.Errorf("历史版本不存在: %w", err)
	}

	// 先保存当前版本到历史（回滚前备份）
	now := time.Now().Unix()
	var currentPlugin Plugin
	var currentFiltersJSON, currentConfigJSON string
	var currentEnabled int

	err = tx.QueryRow(`
		SELECT name, version, author, description,
			   source_code, source_language, compiled_code,
			   filters, config, enabled
		FROM plugins WHERE id = ?
	`, pluginID).Scan(
		&currentPlugin.Metadata.Name,
		&currentPlugin.Metadata.Version,
		&currentPlugin.Metadata.Author,
		&currentPlugin.Metadata.Description,
		&currentPlugin.SourceCode,
		&currentPlugin.Language,
		&currentPlugin.CompiledCode,
		&currentFiltersJSON,
		&currentConfigJSON,
		&currentEnabled,
	)

	if err == nil {
		// 保存当前版本到历史
		_, _ = tx.Exec(`
			INSERT INTO plugin_versions (
				plugin_id, version, name, author, description,
				source_code, source_language, compiled_code,
				filters, config, saved_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			pluginID,
			currentPlugin.Metadata.Version,
			currentPlugin.Metadata.Name,
			currentPlugin.Metadata.Author,
			currentPlugin.Metadata.Description,
			currentPlugin.SourceCode,
			currentPlugin.Language,
			currentPlugin.CompiledCode,
			currentFiltersJSON,
			currentConfigJSON,
			now,
		)
	}

	// 更新插件为历史版本
	_, err = tx.Exec(`
		UPDATE plugins SET
			name = ?,
			version = ?,
			author = ?,
			description = ?,
			source_code = ?,
			source_language = ?,
			compiled_code = ?,
			filters = ?,
			config = ?,
			updated_at = ?
		WHERE id = ?
	`,
		name, version, author, description,
		sourceCode, language, compiledCode,
		filtersJSON, configJSON,
		now, pluginID,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}
