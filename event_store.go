package main

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	sqlite3 "github.com/mattn/go-sqlite3"
)

// init registers a custom SQLite driver exposing gz_text(), which transparently
// decompresses gzip-compressed event_data so SQL LIKE can search inside it.
// sql.Register panics on duplicate names, so this must run exactly once.
func init() {
	sql.Register("sqlite3_gaze", &sqlite3.SQLiteDriver{
		ConnectHook: func(conn *sqlite3.SQLiteConn) error {
			// gz_text: decompress gzip data to text; non-gzip data passes through
			// unchanged (same semantics as decompressData)
			return conn.RegisterFunc("gz_text", func(data []byte) string {
				out, err := decompressData(data)
				if err != nil {
					return string(data)
				}
				return string(out)
			}, true)
		},
	})
}

// ========================================
// EventStore - SQLite 事件存储
// ========================================

type EventStore struct {
	db     *sql.DB
	dbPath string

	// 写入缓冲
	writeBuffer    []UnifiedEvent
	writeBufferMu  sync.Mutex
	flushInterval  time.Duration
	flushThreshold int
	flushTicker    *time.Ticker
	stopChan       chan struct{}
	wg             sync.WaitGroup // 用于等待后台 goroutine 完成

	// FTS 支持标志（缓存初始化时的检查结果）
	hasFTS bool

	// 预编译语句
	stmtInsertEvent        *sql.Stmt
	stmtInsertEventData    *sql.Stmt
	stmtInsertSession      *sql.Stmt
	stmtUpdateSession      *sql.Stmt
	stmtInsertAssertion    *sql.Stmt
	stmtUpdateAssertion    *sql.Stmt
	stmtInsertAssertResult *sql.Stmt
}

// SQL Schema
const schemaSQL = `
-- 启用 WAL 模式提升并发写入性能
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -64000;
PRAGMA temp_store = MEMORY;
PRAGMA mmap_size = 268435456;

-- ==================== Sessions 表 ====================
CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL,
    type TEXT NOT NULL,
    name TEXT NOT NULL,
    start_time INTEGER NOT NULL,
    end_time INTEGER DEFAULT 0,
    status TEXT DEFAULT 'active',
    event_count INTEGER DEFAULT 0,
    video_path TEXT,
    video_duration INTEGER,
    video_offset INTEGER DEFAULT 0,
    metadata TEXT DEFAULT '{}',
    created_at INTEGER DEFAULT (strftime('%s', 'now') * 1000),
    updated_at INTEGER DEFAULT (strftime('%s', 'now') * 1000)
);

CREATE INDEX IF NOT EXISTS idx_sessions_device ON sessions(device_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_time ON sessions(start_time DESC);

-- ==================== Events 表 ====================
CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    relative_time INTEGER NOT NULL,
    duration INTEGER DEFAULT 0,
    source TEXT NOT NULL,
    category TEXT NOT NULL,
    type TEXT NOT NULL,
    level TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT,
    parent_id TEXT,
    step_id TEXT,
    trace_id TEXT,
    aggregate_count INTEGER DEFAULT 0,
    aggregate_first INTEGER DEFAULT 0,
    aggregate_last INTEGER DEFAULT 0,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

-- 核心索引
CREATE INDEX IF NOT EXISTS idx_events_session_time ON events(session_id, relative_time);
CREATE INDEX IF NOT EXISTS idx_events_session_timestamp ON events(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_events_device_time ON events(device_id, timestamp);

-- 筛选索引
CREATE INDEX IF NOT EXISTS idx_events_source ON events(session_id, source);
CREATE INDEX IF NOT EXISTS idx_events_category ON events(session_id, category);
CREATE INDEX IF NOT EXISTS idx_events_level ON events(session_id, level);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(session_id, type);

-- 关联索引
CREATE INDEX IF NOT EXISTS idx_events_parent ON events(parent_id);
CREATE INDEX IF NOT EXISTS idx_events_step ON events(step_id);
CREATE INDEX IF NOT EXISTS idx_events_trace ON events(trace_id);

-- ==================== Event Data 表 (大数据分离存储) ====================
CREATE TABLE IF NOT EXISTS event_data (
    event_id TEXT PRIMARY KEY,
    data TEXT NOT NULL,
    data_size INTEGER,
    FOREIGN KEY (event_id) REFERENCES events(id) ON DELETE CASCADE
);

-- ==================== Bookmarks 表 ====================
CREATE TABLE IF NOT EXISTS bookmarks (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL,
    relative_time INTEGER NOT NULL,
    label TEXT NOT NULL,
    color TEXT,
    type TEXT DEFAULT 'user',
    created_at INTEGER DEFAULT (strftime('%s', 'now') * 1000),
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bookmarks_session ON bookmarks(session_id, relative_time);

-- ==================== Assertions 表 (断言定义) ====================
CREATE TABLE IF NOT EXISTS assertions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    type TEXT NOT NULL,
    session_id TEXT,
    device_id TEXT,
    time_range_start INTEGER,
    time_range_end INTEGER,
    criteria TEXT NOT NULL,
    expected TEXT NOT NULL,
    timeout INTEGER DEFAULT 0,
    tags TEXT,
    metadata TEXT,
    is_template INTEGER DEFAULT 0,
    created_at INTEGER DEFAULT (strftime('%s', 'now') * 1000),
    updated_at INTEGER DEFAULT (strftime('%s', 'now') * 1000)
);

CREATE INDEX IF NOT EXISTS idx_assertions_session ON assertions(session_id);
CREATE INDEX IF NOT EXISTS idx_assertions_device ON assertions(device_id);
CREATE INDEX IF NOT EXISTS idx_assertions_template ON assertions(is_template);

-- ==================== Assertion Results 表 (断言执行结果) ====================
CREATE TABLE IF NOT EXISTS assertion_results (
    id TEXT PRIMARY KEY,
    assertion_id TEXT NOT NULL,
    assertion_name TEXT NOT NULL,
    session_id TEXT,
    passed INTEGER NOT NULL,
    message TEXT,
    matched_events TEXT,
    actual_value TEXT,
    expected_value TEXT,
    executed_at INTEGER NOT NULL,
    duration INTEGER NOT NULL,
    details TEXT
);

CREATE INDEX IF NOT EXISTS idx_assertion_results_session ON assertion_results(session_id, executed_at DESC);
CREATE INDEX IF NOT EXISTS idx_assertion_results_assertion ON assertion_results(assertion_id);

-- ==================== Assertion Sets 表 (断言集定义) ====================
CREATE TABLE IF NOT EXISTS assertion_sets (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    assertions TEXT NOT NULL, -- JSON array of assertion IDs
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_assertion_sets_name ON assertion_sets(name);

-- ==================== Assertion Set Results 表 (断言集执行结果) ====================
CREATE TABLE IF NOT EXISTS assertion_set_results (
    id TEXT PRIMARY KEY,
    set_id TEXT NOT NULL,
    set_name TEXT NOT NULL,
    session_id TEXT,
    device_id TEXT,
    execution_id TEXT NOT NULL,
    start_time INTEGER NOT NULL,
    end_time INTEGER,
    duration INTEGER,
    status TEXT NOT NULL,
    summary TEXT, -- JSON of AssertionSetSummary
    results TEXT NOT NULL, -- JSON array of AssertionResult
    executed_at INTEGER NOT NULL,
    FOREIGN KEY (session_id) REFERENCES sessions(id),
    FOREIGN KEY (set_id) REFERENCES assertion_sets(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_assertion_set_results_set ON assertion_set_results(set_id);
CREATE INDEX IF NOT EXISTS idx_assertion_set_results_session ON assertion_set_results(session_id);
CREATE INDEX IF NOT EXISTS idx_assertion_set_results_device ON assertion_set_results(device_id);
CREATE INDEX IF NOT EXISTS idx_assertion_set_results_executed ON assertion_set_results(executed_at DESC);
`

// FTS Schema (单独创建，因为可能需要检查是否存在)
const ftsSchemaSQL = `
-- 全文搜索 (FTS5)
CREATE VIRTUAL TABLE IF NOT EXISTS events_fts USING fts5(
    id,
    title,
    summary,
    content='events',
    content_rowid='rowid'
);
`

const ftsTriggerSQL = `
-- FTS 触发器
CREATE TRIGGER IF NOT EXISTS events_ai AFTER INSERT ON events BEGIN
    INSERT INTO events_fts(rowid, id, title, summary)
    VALUES (new.rowid, new.id, new.title, new.summary);
END;

CREATE TRIGGER IF NOT EXISTS events_ad AFTER DELETE ON events BEGIN
    INSERT INTO events_fts(events_fts, rowid, id, title, summary)
    VALUES('delete', old.rowid, old.id, old.title, old.summary);
END;

CREATE TRIGGER IF NOT EXISTS events_au AFTER UPDATE ON events BEGIN
    INSERT INTO events_fts(events_fts, rowid, id, title, summary)
    VALUES('delete', old.rowid, old.id, old.title, old.summary);
    INSERT INTO events_fts(rowid, id, title, summary)
    VALUES (new.rowid, new.id, new.title, new.summary);
END;
`

// ========================================
// 数据压缩辅助函数
// ========================================

// gzipWriterPool 池化 gzip.Writer，高频事件写入时避免每次重建 deflate 状态 (约 64KB+)
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(io.Discard)
	},
}

// compressData 压缩数据 (使用 gzip)
// 小于 1KB 的数据不压缩，避免头部开销
func compressData(data []byte) ([]byte, error) {
	if len(data) < 1024 {
		return data, nil
	}

	var buf bytes.Buffer
	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(&buf)
	_, writeErr := gz.Write(data)
	closeErr := gz.Close()
	gzipWriterPool.Put(gz)
	if writeErr != nil {
		return nil, writeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}

	// 如果压缩后更大，保留原始数据
	if buf.Len() >= len(data) {
		return data, nil
	}
	return buf.Bytes(), nil
}

// decompressData 解压数据
// 自动检测是否为 gzip 格式，兼容未压缩的数据
func decompressData(data []byte) ([]byte, error) {
	// 检测 gzip 魔数
	if len(data) < 2 || data[0] != 0x1f || data[1] != 0x8b {
		return data, nil // 未压缩，直接返回
	}

	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	return io.ReadAll(gz)
}

// NewEventStore 创建事件存储
func NewEventStore(dataDir string) (*EventStore, error) {
	// 确保目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "events.db")

	// 打开数据库连接
	// WAL 模式支持并发读取，但写入仍需串行
	db, err := sql.Open("sqlite3_gaze", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_cache_size=-64000&_foreign_keys=ON&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 设置连接池
	// SQLite WAL 模式: 写入串行，但支持并发读取
	db.SetMaxOpenConns(4)                   // 允许多个读连接
	db.SetMaxIdleConns(2)                   // 保持 2 个空闲连接
	db.SetConnMaxLifetime(time.Hour)        // 连接最长存活 1 小时
	db.SetConnMaxIdleTime(10 * time.Minute) // 空闲连接 10 分钟后关闭

	// 验证数据库连接
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	store := &EventStore{
		db:             db,
		dbPath:         dbPath,
		writeBuffer:    make([]UnifiedEvent, 0, 1000),
		flushInterval:  500 * time.Millisecond,
		flushThreshold: 500,
		stopChan:       make(chan struct{}),
	}

	// 初始化 schema
	if err := store.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	// 预编译语句
	if err := store.prepareStatements(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to prepare statements: %w", err)
	}

	// 启动后台写入
	store.startBackgroundWriter()

	return store, nil
}

// initSchema 初始化数据库 schema
func (s *EventStore) initSchema() error {
	// 执行主 schema
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// 尝试创建 FTS 表 (可能失败如果 SQLite 没有 FTS5 支持)
	if _, err := s.db.Exec(ftsSchemaSQL); err != nil {
		// FTS5 不可用，跳过
		LogWarn("event_store").Err(err).Msg("FTS5 not available, full-text search disabled")
		s.hasFTS = false
	} else {
		// 创建 FTS 触发器
		if _, err := s.db.Exec(ftsTriggerSQL); err != nil {
			LogWarn("event_store").Err(err).Msg("Failed to create FTS triggers")
			s.hasFTS = false
		} else {
			s.hasFTS = true
		}
	}

	// 检查 FTS 表是否实际存在（验证缓存）
	var ftsExists int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='events_fts'").Scan(&ftsExists); err == nil {
		s.hasFTS = ftsExists > 0
	}

	if s.hasFTS {
		LogInfo("event_store").Msg("FTS5 full-text search enabled")
	} else {
		LogWarn("event_store").Msg("FTS5 not available, using LIKE fallback for search")
	}

	// 数据库迁移：添加插件相关字段
	if err := s.migratePluginFields(); err != nil {
		return fmt.Errorf("failed to migrate plugin fields: %w", err)
	}

	// 迁移：移除旧版 time_index 表（只写不读且计数错误，时间索引现由
	// GetTimeIndex 对 events 实时 GROUP BY 生成）
	s.db.Exec("DROP TABLE IF EXISTS time_index")

	return nil
}

// migratePluginFields 迁移插件相关字段
func (s *EventStore) migratePluginFields() error {
	// 添加 tags 字段 (JSON 数组)
	s.db.Exec("ALTER TABLE events ADD COLUMN tags TEXT")

	// 添加 metadata 字段 (JSON 对象)
	s.db.Exec("ALTER TABLE events ADD COLUMN metadata TEXT")

	// 添加 parent_event_id 字段
	s.db.Exec("ALTER TABLE events ADD COLUMN parent_event_id TEXT")

	// 添加 generated_by_plugin 字段
	s.db.Exec("ALTER TABLE events ADD COLUMN generated_by_plugin TEXT")

	// 创建索引 (忽略错误，可能已存在)
	s.db.Exec("CREATE INDEX IF NOT EXISTS idx_events_parent_event ON events(parent_event_id)")
	s.db.Exec("CREATE INDEX IF NOT EXISTS idx_events_plugin ON events(generated_by_plugin)")

	return nil
}

// prepareStatements 预编译 SQL 语句
func (s *EventStore) prepareStatements() error {
	var err error

	s.stmtInsertEvent, err = s.db.Prepare(`
		INSERT INTO events (
			id, session_id, device_id, timestamp, relative_time, duration,
			source, category, type, level, title, summary,
			parent_id, step_id, trace_id,
			aggregate_count, aggregate_first, aggregate_last,
			tags, metadata, parent_event_id, generated_by_plugin
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert event: %w", err)
	}

	s.stmtInsertEventData, err = s.db.Prepare(`
		INSERT OR REPLACE INTO event_data (event_id, data, data_size) VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert event data: %w", err)
	}

	s.stmtInsertSession, err = s.db.Prepare(`
		INSERT INTO sessions (
			id, device_id, type, name, start_time, end_time, status,
			event_count, video_path, video_duration, video_offset, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert session: %w", err)
	}

	s.stmtUpdateSession, err = s.db.Prepare(`
		UPDATE sessions SET
			end_time = ?, status = ?, event_count = ?,
			video_path = ?, video_duration = ?, video_offset = ?,
			metadata = ?, updated_at = ?
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare update session: %w", err)
	}

	s.stmtInsertAssertion, err = s.db.Prepare(`
		INSERT INTO assertions (
			id, name, description, type, session_id, device_id,
			time_range_start, time_range_end, criteria, expected,
			timeout, tags, metadata, is_template, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert assertion: %w", err)
	}

	s.stmtUpdateAssertion, err = s.db.Prepare(`
		UPDATE assertions SET
			name = ?, description = ?, type = ?, session_id = ?, device_id = ?,
			time_range_start = ?, time_range_end = ?, criteria = ?, expected = ?,
			timeout = ?, tags = ?, metadata = ?, is_template = ?, updated_at = ?
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("prepare update assertion: %w", err)
	}

	s.stmtInsertAssertResult, err = s.db.Prepare(`
		INSERT INTO assertion_results (
			id, assertion_id, assertion_name, session_id, passed, message,
			matched_events, actual_value, expected_value, executed_at, duration, details
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert assertion result: %w", err)
	}

	return nil
}

// startBackgroundWriter 启动后台写入
func (s *EventStore) startBackgroundWriter() {
	s.flushTicker = time.NewTicker(s.flushInterval)
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		for {
			select {
			case <-s.flushTicker.C:
				s.Flush()
			case <-s.stopChan:
				s.flushTicker.Stop()
				s.Flush() // 最后一次刷新
				return
			}
		}
	}()
}

// Close 关闭存储
func (s *EventStore) Close() error {
	close(s.stopChan)

	// 等待后台写入 goroutine 完成 (包括最后一次 Flush)
	s.wg.Wait()

	// 关闭预编译语句
	if s.stmtInsertEvent != nil {
		s.stmtInsertEvent.Close()
	}
	if s.stmtInsertEventData != nil {
		s.stmtInsertEventData.Close()
	}
	if s.stmtInsertSession != nil {
		s.stmtInsertSession.Close()
	}
	if s.stmtUpdateSession != nil {
		s.stmtUpdateSession.Close()
	}
	if s.stmtInsertAssertion != nil {
		s.stmtInsertAssertion.Close()
	}
	if s.stmtUpdateAssertion != nil {
		s.stmtUpdateAssertion.Close()
	}
	if s.stmtInsertAssertResult != nil {
		s.stmtInsertAssertResult.Close()
	}

	return s.db.Close()
}

// ========================================
// Event 写入
// ========================================

// WriteEvent 写入单个事件 (缓冲)
func (s *EventStore) WriteEvent(event UnifiedEvent) {
	s.writeBufferMu.Lock()
	s.writeBuffer = append(s.writeBuffer, event)
	shouldFlush := len(s.writeBuffer) >= s.flushThreshold
	s.writeBufferMu.Unlock()

	if shouldFlush {
		go s.Flush()
	}
}

// WriteEventDirect 直接写入事件 (不缓冲)
func (s *EventStore) WriteEventDirect(event UnifiedEvent) error {
	return s.writeEventsBatch([]UnifiedEvent{event})
}

// Flush 刷新缓冲区
func (s *EventStore) Flush() {
	s.writeBufferMu.Lock()
	if len(s.writeBuffer) == 0 {
		s.writeBufferMu.Unlock()
		return
	}

	events := s.writeBuffer
	s.writeBuffer = make([]UnifiedEvent, 0, 1000)
	s.writeBufferMu.Unlock()

	if err := s.writeEventsBatch(events); err != nil {
		LogError("event_store").Err(err).Int("event_count", len(events)).Msg("Failed to flush events")
	}
}

// writeEventsBatch 批量写入事件
func (s *EventStore) writeEventsBatch(events []UnifiedEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmtEvent := tx.Stmt(s.stmtInsertEvent)
	stmtData := tx.Stmt(s.stmtInsertEventData)

	for _, event := range events {
		// 序列化 tags 和 metadata
		var tagsJSON, metadataJSON string
		if len(event.Tags) > 0 {
			if b, err := json.Marshal(event.Tags); err == nil {
				tagsJSON = string(b)
			}
		}
		if len(event.Metadata) > 0 {
			if b, err := json.Marshal(event.Metadata); err == nil {
				metadataJSON = string(b)
			}
		}

		// 插入基础事件
		_, err := stmtEvent.Exec(
			event.ID, event.SessionID, event.DeviceID,
			event.Timestamp, event.RelativeTime, event.Duration,
			string(event.Source), string(event.Category), event.Type, string(event.Level),
			event.Title, nullString(event.Summary),
			nullString(event.ParentID), nullString(event.StepID), nullString(event.TraceID),
			event.AggregateCount, event.AggregateFirst, event.AggregateLast,
			nullString(tagsJSON), nullString(metadataJSON),
			nullString(event.ParentEventID), nullString(event.GeneratedByPlugin),
		)
		if err != nil {
			return fmt.Errorf("insert event %s: %w", event.ID, err)
		}

		// 插入扩展数据
		if len(event.Data) > 0 {
			// 压缩数据
			compressedData, err := compressData([]byte(event.Data))
			if err != nil {
				// 压缩失败，使用原始数据
				LogWarn("event_store").Err(err).Str("eventId", event.ID).Msg("Failed to compress event data, using raw data")
				compressedData = []byte(event.Data)
			}

			_, err = stmtData.Exec(event.ID, compressedData, len(event.Data))
			if err != nil {
				return fmt.Errorf("insert event data %s: %w", event.ID, err)
			}
		}
	}

	return tx.Commit()
}

// ========================================
// Session 操作
// ========================================

// CreateSession 创建 Session
func (s *EventStore) CreateSession(session *DeviceSession) error {
	metadata, err := json.Marshal(session.Metadata)
	if err != nil {
		LogWarn("event_store").Err(err).Str("sessionId", session.ID).Msg("Failed to marshal session metadata")
		metadata = []byte("{}")
	}
	_, err = s.stmtInsertSession.Exec(
		session.ID, session.DeviceID, session.Type, session.Name,
		session.StartTime, session.EndTime, session.Status, session.EventCount,
		nullString(session.VideoPath), session.VideoDuration, session.VideoOffset,
		string(metadata),
	)
	return err
}

// UpdateSession 更新 Session
func (s *EventStore) UpdateSession(session *DeviceSession) error {
	metadata, err := json.Marshal(session.Metadata)
	if err != nil {
		LogWarn("event_store").Err(err).Str("sessionId", session.ID).Msg("Failed to marshal session metadata")
		metadata = []byte("{}")
	}
	_, err = s.stmtUpdateSession.Exec(
		session.EndTime, session.Status, session.EventCount,
		nullString(session.VideoPath), session.VideoDuration, session.VideoOffset,
		string(metadata), time.Now().UnixMilli(),
		session.ID,
	)
	return err
}

// RenameSession 重命名 Session
func (s *EventStore) RenameSession(id, newName string) error {
	_, err := s.db.Exec(`UPDATE sessions SET name = ?, updated_at = ? WHERE id = ?`,
		newName, time.Now().UnixMilli(), id)
	return err
}

// GetSession 获取 Session
func (s *EventStore) GetSession(id string) (*DeviceSession, error) {
	row := s.db.QueryRow(`
		SELECT id, device_id, type, name, start_time, end_time, status, event_count,
			video_path, video_duration, video_offset, metadata
		FROM sessions WHERE id = ?
	`, id)
	return s.scanSession(row)
}

// ListSessions 列出 Sessions
func (s *EventStore) ListSessions(deviceID string, limit int) ([]DeviceSession, error) {
	query := `
		SELECT id, device_id, type, name, start_time, end_time, status, event_count,
			video_path, video_duration, video_offset, metadata
		FROM sessions
	`
	var args []interface{}

	if deviceID != "" {
		query += ` WHERE device_id = ?`
		args = append(args, deviceID)
	}
	query += ` ORDER BY start_time DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []DeviceSession
	for rows.Next() {
		session, err := s.scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *session)
	}
	return sessions, rows.Err()
}

// DeleteSession 删除 Session
func (s *EventStore) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// scanSession 扫描单行 Session
func (s *EventStore) scanSession(row *sql.Row) (*DeviceSession, error) {
	var session DeviceSession
	var videoPath, metadata sql.NullString
	var videoDuration, videoOffset sql.NullInt64

	err := row.Scan(
		&session.ID, &session.DeviceID, &session.Type, &session.Name,
		&session.StartTime, &session.EndTime, &session.Status, &session.EventCount,
		&videoPath, &videoDuration, &videoOffset, &metadata,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	session.VideoPath = videoPath.String
	session.VideoDuration = videoDuration.Int64
	session.VideoOffset = videoOffset.Int64

	if metadata.Valid {
		if err := json.Unmarshal([]byte(metadata.String), &session.Metadata); err != nil {
			LogWarn("event_store").Err(err).Str("sessionId", session.ID).Msg("Failed to unmarshal session metadata")
		}
	}

	return &session, nil
}

// scanSessionRow 扫描 Session 行
func (s *EventStore) scanSessionRow(rows *sql.Rows) (*DeviceSession, error) {
	var session DeviceSession
	var videoPath, metadata sql.NullString
	var videoDuration, videoOffset sql.NullInt64

	err := rows.Scan(
		&session.ID, &session.DeviceID, &session.Type, &session.Name,
		&session.StartTime, &session.EndTime, &session.Status, &session.EventCount,
		&videoPath, &videoDuration, &videoOffset, &metadata,
	)
	if err != nil {
		return nil, err
	}

	session.VideoPath = videoPath.String
	session.VideoDuration = videoDuration.Int64
	session.VideoOffset = videoOffset.Int64

	if metadata.Valid {
		if err := json.Unmarshal([]byte(metadata.String), &session.Metadata); err != nil {
			LogWarn("event_store").Err(err).Str("sessionId", session.ID).Msg("Failed to unmarshal session metadata")
		}
	}

	return &session, nil
}

// ========================================
// Event 查询
// ========================================

// EventQuery 事件查询参数
type EventQuery struct {
	SessionID   string          `json:"sessionId,omitempty"`
	DeviceID    string          `json:"deviceId,omitempty"`
	Sources     []EventSource   `json:"sources,omitempty"`
	Categories  []EventCategory `json:"categories,omitempty"`
	Types       []string        `json:"types,omitempty"`
	Levels      []EventLevel    `json:"levels,omitempty"`
	StartTime   int64           `json:"startTime,omitempty"` // 相对时间 (ms)
	EndTime     int64           `json:"endTime,omitempty"`
	SearchText  string          `json:"searchText,omitempty"`
	ParentID    string          `json:"parentId,omitempty"`
	StepID      string          `json:"stepId,omitempty"`
	TraceID     string          `json:"traceId,omitempty"`
	Limit       int             `json:"limit,omitempty"`
	Offset      int             `json:"offset,omitempty"`
	OrderDesc   bool            `json:"orderDesc,omitempty"`   // true = 时间倒序
	IncludeData bool            `json:"includeData,omitempty"` // true = 加载完整 event_data
}

// EventQueryResult 查询结果
type EventQueryResult struct {
	Events  []UnifiedEvent `json:"events"`
	Total   int            `json:"total"`
	HasMore bool           `json:"hasMore"`
}

// escapeFTS5Query 将用户输入转换为安全的 FTS5 MATCH 查询：
// 按空白拆分 token，每个 token 用双引号包裹为短语（内部双引号翻倍），
// token 间以空格连接（FTS5 隐式 AND）；末尾 * 保留为前缀匹配（"tok"* 语法）。
// 防止 URL 冒号、AND/OR、不配对引号等输入触发 FTS5 语法错误。
func escapeFTS5Query(input string) string {
	fields := strings.Fields(input)
	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		prefix := ""
		if strings.HasSuffix(f, "*") && len(f) > 1 {
			f = strings.TrimRight(f, "*")
			prefix = "*"
		}
		if f == "" {
			continue
		}
		tokens = append(tokens, `"`+strings.ReplaceAll(f, `"`, `""`)+`"`+prefix)
	}
	return strings.Join(tokens, " ")
}

// QueryEvents 查询事件 (优化版：不加载 event_data，只加载列表需要的字段)
func (s *EventStore) QueryEvents(q EventQuery) (*EventQueryResult, error) {
	// 构建查询条件
	var conditions []string
	var args []interface{}

	if q.SessionID != "" {
		conditions = append(conditions, "session_id = ?")
		args = append(args, q.SessionID)
	}
	if q.DeviceID != "" {
		conditions = append(conditions, "device_id = ?")
		args = append(args, q.DeviceID)
	}
	if len(q.Sources) > 0 {
		placeholders := make([]string, len(q.Sources))
		for i, src := range q.Sources {
			placeholders[i] = "?"
			args = append(args, string(src))
		}
		conditions = append(conditions, fmt.Sprintf("source IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(q.Categories) > 0 {
		placeholders := make([]string, len(q.Categories))
		for i, cat := range q.Categories {
			placeholders[i] = "?"
			args = append(args, string(cat))
		}
		conditions = append(conditions, fmt.Sprintf("category IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(q.Types) > 0 {
		placeholders := make([]string, len(q.Types))
		for i, t := range q.Types {
			placeholders[i] = "?"
			args = append(args, t)
		}
		conditions = append(conditions, fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ",")))
	}
	if len(q.Levels) > 0 {
		placeholders := make([]string, len(q.Levels))
		for i, lvl := range q.Levels {
			placeholders[i] = "?"
			args = append(args, string(lvl))
		}
		conditions = append(conditions, fmt.Sprintf("level IN (%s)", strings.Join(placeholders, ",")))
	}
	if q.StartTime > 0 {
		conditions = append(conditions, "relative_time >= ?")
		args = append(args, q.StartTime)
	}
	if q.EndTime > 0 {
		conditions = append(conditions, "relative_time <= ?")
		args = append(args, q.EndTime)
	}
	if q.ParentID != "" {
		conditions = append(conditions, "parent_id = ?")
		args = append(args, q.ParentID)
	}
	if q.StepID != "" {
		conditions = append(conditions, "step_id = ?")
		args = append(args, q.StepID)
	}
	if q.TraceID != "" {
		conditions = append(conditions, "trace_id = ?")
		args = append(args, q.TraceID)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// 全文搜索标志和条件
	hasSearch := q.SearchText != ""
	var searchCondition string
	var searchArgs []interface{}

	// 全文搜索 (深度搜索: title + summary + event_data.data)
	// event_data.data 可能是 gzip 压缩存储的（见 compressData），LIKE 前必须经
	// gz_text() 解压，否则 ≥1KB 的网络请求体等大数据永远搜不到
	if hasSearch {
		searchPattern := "%" + q.SearchText + "%"
		ftsQuery := escapeFTS5Query(q.SearchText)

		if s.hasFTS && ftsQuery != "" {
			// FTS5 搜索 title/summary + LIKE 搜索 event_data 详情内容
			// 使用 LEFT JOIN，避免子查询性能问题
			searchCondition = "(e.id IN (SELECT id FROM events_fts WHERE events_fts MATCH ?) OR gz_text(COALESCE(ed.data, '')) LIKE ?)"
			searchArgs = []interface{}{ftsQuery, searchPattern}
		} else {
			// 降级: LIKE 搜索 title/summary + event_data 详情内容
			// （FTS 不可用，或输入转义后为空——如仅含空白/星号）
			searchCondition = "(e.title LIKE ? OR COALESCE(e.summary, '') LIKE ? OR gz_text(COALESCE(ed.data, '')) LIKE ?)"
			searchArgs = []interface{}{searchPattern, searchPattern, searchPattern}
		}
	}

	// 获取真实总数
	// 如果有搜索，需要 JOIN event_data 表来搜索 data 字段
	// limit==0 全量加载时跳过独立 COUNT 扫描，total 在主查询后取结果行数
	var total int
	if q.Limit > 0 {
		if hasSearch {
			// COUNT 查询需要 LEFT JOIN event_data
			countQuery := "SELECT COUNT(DISTINCT e.id) FROM events e LEFT JOIN event_data ed ON e.id = ed.event_id"
			if whereClause != "" {
				countQuery += whereClause + " AND " + searchCondition
			} else {
				countQuery += " WHERE " + searchCondition
			}
			// 合并参数：先是基础条件参数，再是搜索参数
			countArgs := append(append([]interface{}{}, args...), searchArgs...)
			if err := s.db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
				return nil, fmt.Errorf("count query: %w", err)
			}
		} else {
			// 无搜索时，只查 events 表
			countQuery := "SELECT COUNT(*) FROM events " + whereClause
			if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
				return nil, fmt.Errorf("count query: %w", err)
			}
		}
	}

	// 构建最终查询
	order := "ASC"
	if q.OrderDesc {
		order = "DESC"
	}

	var query string
	var queryArgs []interface{}

	// 如果有搜索或需要完整数据，必须 JOIN event_data
	needJoin := q.IncludeData || hasSearch

	if needJoin {
		// JOIN event_data 获取完整数据 或 执行深度搜索
		// 使用 DISTINCT 避免 LEFT JOIN 产生重复行
		selectClause := `SELECT DISTINCT e.id, e.session_id, e.device_id, e.timestamp, e.relative_time, e.duration,
			e.source, e.category, e.type, e.level, e.title, e.summary,
			e.parent_id, e.step_id, e.trace_id,
			e.aggregate_count, e.aggregate_first, e.aggregate_last,
			e.tags, e.metadata, e.parent_event_id, e.generated_by_plugin,
			ed.data`

		fromClause := `FROM events e LEFT JOIN event_data ed ON e.id = ed.event_id`

		if hasSearch {
			// 有搜索时，WHERE 条件需要包含基础条件 + 搜索条件
			if whereClause != "" {
				query = fmt.Sprintf("%s %s %s AND %s ORDER BY e.relative_time %s",
					selectClause, fromClause, whereClause, searchCondition, order)
			} else {
				query = fmt.Sprintf("%s %s WHERE %s ORDER BY e.relative_time %s",
					selectClause, fromClause, searchCondition, order)
			}
			// 合并参数
			queryArgs = append(append([]interface{}{}, args...), searchArgs...)
		} else {
			// 无搜索，只是需要加载 event_data
			query = fmt.Sprintf("%s %s %s ORDER BY e.relative_time %s",
				selectClause, fromClause, whereClause, order)
			queryArgs = args
		}
	} else {
		// 不 JOIN event_data，列表不需要完整数据，也无搜索
		query = fmt.Sprintf(`
			SELECT id, session_id, device_id, timestamp, relative_time, duration,
				source, category, type, level, title, summary,
				parent_id, step_id, trace_id,
				aggregate_count, aggregate_first, aggregate_last,
				tags, metadata, parent_event_id, generated_by_plugin
			FROM events
			%s
			ORDER BY relative_time %s
		`, whereClause, order)
		queryArgs = args
	}

	if q.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", q.Limit)
	}
	if q.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", q.Offset)
	}

	rows, err := s.db.Query(query, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []UnifiedEvent
	for rows.Next() {
		var event *UnifiedEvent
		var err error
		if needJoin {
			// 查询包含 ed.data 字段
			event, err = s.scanEventRow(rows)
		} else {
			// 查询不包含 ed.data 字段
			event, err = s.scanEventRowWithoutData(rows)
		}
		if err != nil {
			return nil, err
		}
		events = append(events, *event)
	}

	// limit==0 全量加载：结果行数即总数，HasMore 必为 false
	if q.Limit <= 0 {
		total = len(events)
	}

	hasMore := false
	if q.Limit > 0 {
		hasMore = q.Offset+len(events) < total
	}

	return &EventQueryResult{
		Events:  events,
		Total:   total,
		HasMore: hasMore,
	}, nil
}

// GetEvent 获取单个事件
func (s *EventStore) GetEvent(id string) (*UnifiedEvent, error) {
	row := s.db.QueryRow(`
		SELECT e.id, e.session_id, e.device_id, e.timestamp, e.relative_time, e.duration,
			e.source, e.category, e.type, e.level, e.title, e.summary,
			e.parent_id, e.step_id, e.trace_id,
			e.aggregate_count, e.aggregate_first, e.aggregate_last,
			e.tags, e.metadata, e.parent_event_id, e.generated_by_plugin,
			ed.data
		FROM events e
		LEFT JOIN event_data ed ON e.id = ed.event_id
		WHERE e.id = ?
	`, id)

	return s.scanEventSingle(row)
}

// applyPluginFields 反序列化插件相关列 (tags/metadata/parent_event_id/generated_by_plugin) 到事件
func applyPluginFields(event *UnifiedEvent, tags, metadata, parentEventID, generatedByPlugin sql.NullString) {
	if tags.Valid && tags.String != "" {
		if err := json.Unmarshal([]byte(tags.String), &event.Tags); err != nil {
			LogWarn("event_store").Err(err).Str("eventId", event.ID).Msg("Failed to unmarshal event tags")
		}
	}
	if metadata.Valid && metadata.String != "" {
		if err := json.Unmarshal([]byte(metadata.String), &event.Metadata); err != nil {
			LogWarn("event_store").Err(err).Str("eventId", event.ID).Msg("Failed to unmarshal event metadata")
		}
	}
	event.ParentEventID = parentEventID.String
	event.GeneratedByPlugin = generatedByPlugin.String
}

// scanEventRow 扫描事件行 (包含 data)
func (s *EventStore) scanEventRow(rows *sql.Rows) (*UnifiedEvent, error) {
	var event UnifiedEvent
	var summary, parentID, stepID, traceID, data sql.NullString
	var tags, metadata, parentEventID, generatedByPlugin sql.NullString
	var source, category, level string

	err := rows.Scan(
		&event.ID, &event.SessionID, &event.DeviceID,
		&event.Timestamp, &event.RelativeTime, &event.Duration,
		&source, &category, &event.Type, &level,
		&event.Title, &summary,
		&parentID, &stepID, &traceID,
		&event.AggregateCount, &event.AggregateFirst, &event.AggregateLast,
		&tags, &metadata, &parentEventID, &generatedByPlugin,
		&data,
	)
	if err != nil {
		return nil, err
	}

	event.Source = EventSource(source)
	event.Category = EventCategory(category)
	event.Level = EventLevel(level)
	event.Summary = summary.String
	event.ParentID = parentID.String
	event.StepID = stepID.String
	event.TraceID = traceID.String
	applyPluginFields(&event, tags, metadata, parentEventID, generatedByPlugin)

	if data.Valid && data.String != "" {
		// 解压数据（自动检测是否压缩）
		decompressedData, err := decompressData([]byte(data.String))
		if err != nil {
			LogWarn("event_store").Err(err).Str("eventId", event.ID).Msg("Failed to decompress event data, using raw data")
			event.Data = json.RawMessage(data.String)
		} else {
			event.Data = json.RawMessage(decompressedData)
		}
	}

	return &event, nil
}

// scanEventRowWithoutData 扫描事件行 (不包含 data，用于列表查询优化)
func (s *EventStore) scanEventRowWithoutData(rows *sql.Rows) (*UnifiedEvent, error) {
	var event UnifiedEvent
	var summary, parentID, stepID, traceID sql.NullString
	var tags, metadata, parentEventID, generatedByPlugin sql.NullString
	var source, category, level string

	err := rows.Scan(
		&event.ID, &event.SessionID, &event.DeviceID,
		&event.Timestamp, &event.RelativeTime, &event.Duration,
		&source, &category, &event.Type, &level,
		&event.Title, &summary,
		&parentID, &stepID, &traceID,
		&event.AggregateCount, &event.AggregateFirst, &event.AggregateLast,
		&tags, &metadata, &parentEventID, &generatedByPlugin,
	)
	if err != nil {
		return nil, err
	}

	event.Source = EventSource(source)
	event.Category = EventCategory(category)
	event.Level = EventLevel(level)
	event.Summary = summary.String
	event.ParentID = parentID.String
	event.StepID = stepID.String
	event.TraceID = traceID.String
	applyPluginFields(&event, tags, metadata, parentEventID, generatedByPlugin)

	return &event, nil
}

// scanEventSingle 扫描单个事件
func (s *EventStore) scanEventSingle(row *sql.Row) (*UnifiedEvent, error) {
	var event UnifiedEvent
	var summary, parentID, stepID, traceID, data sql.NullString
	var tags, metadata, parentEventID, generatedByPlugin sql.NullString
	var source, category, level string

	err := row.Scan(
		&event.ID, &event.SessionID, &event.DeviceID,
		&event.Timestamp, &event.RelativeTime, &event.Duration,
		&source, &category, &event.Type, &level,
		&event.Title, &summary,
		&parentID, &stepID, &traceID,
		&event.AggregateCount, &event.AggregateFirst, &event.AggregateLast,
		&tags, &metadata, &parentEventID, &generatedByPlugin,
		&data,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	event.Source = EventSource(source)
	event.Category = EventCategory(category)
	event.Level = EventLevel(level)
	event.Summary = summary.String
	event.ParentID = parentID.String
	event.StepID = stepID.String
	event.TraceID = traceID.String
	applyPluginFields(&event, tags, metadata, parentEventID, generatedByPlugin)

	if data.Valid && data.String != "" {
		// 解压数据（自动检测是否压缩）
		decompressedData, err := decompressData([]byte(data.String))
		if err != nil {
			LogWarn("event_store").Err(err).Str("eventId", event.ID).Msg("Failed to decompress event data, using raw data")
			event.Data = json.RawMessage(data.String)
		} else {
			event.Data = json.RawMessage(decompressedData)
		}
	}

	return &event, nil
}

// ========================================
// Time Index 操作
// ========================================

// GetTimeIndex 直接从事件数据生成时间索引（更可靠）
func (s *EventStore) GetTimeIndex(sessionID string) ([]TimeIndexEntry, error) {
	// 先检查事件的时间分布
	var minTime, maxTime int64
	var totalCount int
	err := s.db.QueryRow(`
		SELECT MIN(relative_time), MAX(relative_time), COUNT(*)
		FROM events WHERE session_id = ?
	`, sessionID).Scan(&minTime, &maxTime, &totalCount)
	if err != nil {
		LogDebug("event_store").Err(err).Str("session_id", sessionID).Msg("GetTimeIndex: failed to get time range")
	} else {
		LogDebug("event_store").
			Str("session_id", sessionID).
			Int64("min_time", minTime).
			Int64("max_time", maxTime).
			Int("total_events", totalCount).
			Msg("GetTimeIndex: time range")
	}

	// 直接从 events 表聚合生成，确保数据准确
	rows, err := s.db.Query(`
		SELECT
			relative_time / 1000 as second,
			COUNT(*) as event_count,
			MIN(id) as first_event_id,
			MAX(CASE WHEN level IN ('error', 'fatal') THEN 1 ELSE 0 END) as has_error
		FROM events
		WHERE session_id = ?
		GROUP BY relative_time / 1000
		ORDER BY second
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []TimeIndexEntry
	for rows.Next() {
		var e TimeIndexEntry
		var hasError int
		if err := rows.Scan(&e.Second, &e.EventCount, &e.FirstEventID, &hasError); err != nil {
			return nil, err
		}
		e.HasError = hasError != 0
		entries = append(entries, e)
	}

	LogDebug("event_store").
		Str("session_id", sessionID).
		Int("entry_count", len(entries)).
		Msg("GetTimeIndex: completed")
	if len(entries) > 0 {
		LogDebug("event_store").
			Int("first_second", entries[0].Second).
			Int("first_count", entries[0].EventCount).
			Int("last_second", entries[len(entries)-1].Second).
			Int("last_count", entries[len(entries)-1].EventCount).
			Msg("GetTimeIndex: range details")
	}

	return entries, rows.Err()
}

// ========================================
// Bookmark 操作
// ========================================

// CreateBookmark 创建书签
func (s *EventStore) CreateBookmark(bookmark *Bookmark) error {
	_, err := s.db.Exec(`
		INSERT INTO bookmarks (id, session_id, relative_time, label, color, type, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, bookmark.ID, bookmark.SessionID, bookmark.RelativeTime,
		bookmark.Label, nullString(bookmark.Color), bookmark.Type, bookmark.CreatedAt)
	return err
}

// GetBookmarks 获取 Session 的书签
func (s *EventStore) GetBookmarks(sessionID string) ([]Bookmark, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, relative_time, label, color, type, created_at
		FROM bookmarks
		WHERE session_id = ?
		ORDER BY relative_time
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []Bookmark
	for rows.Next() {
		var b Bookmark
		var color sql.NullString
		if err := rows.Scan(&b.ID, &b.SessionID, &b.RelativeTime, &b.Label, &color, &b.Type, &b.CreatedAt); err != nil {
			return nil, err
		}
		b.Color = color.String
		bookmarks = append(bookmarks, b)
	}
	return bookmarks, rows.Err()
}

// DeleteBookmark 删除书签
func (s *EventStore) DeleteBookmark(id string) error {
	_, err := s.db.Exec(`DELETE FROM bookmarks WHERE id = ?`, id)
	return err
}

// ========================================
// 统计和维护
// ========================================

// GetSessionStats 获取 Session 统计信息
func (s *EventStore) GetSessionStats(sessionID string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// 事件总数
	var totalEvents int
	s.db.QueryRow(`SELECT COUNT(*) FROM events WHERE session_id = ?`, sessionID).Scan(&totalEvents)
	stats["totalEvents"] = totalEvents

	// 按来源统计
	rows, err := s.db.Query(`
		SELECT source, COUNT(*) as count
		FROM events WHERE session_id = ?
		GROUP BY source
	`, sessionID)
	if err == nil {
		sourceStats := make(map[string]int)
		for rows.Next() {
			var source string
			var count int
			rows.Scan(&source, &count)
			sourceStats[source] = count
		}
		rows.Close()
		stats["bySource"] = sourceStats
	}

	// 按级别统计
	rows, err = s.db.Query(`
		SELECT level, COUNT(*) as count
		FROM events WHERE session_id = ?
		GROUP BY level
	`, sessionID)
	if err == nil {
		levelStats := make(map[string]int)
		for rows.Next() {
			var level string
			var count int
			rows.Scan(&level, &count)
			levelStats[level] = count
		}
		rows.Close()
		stats["byLevel"] = levelStats
	}

	// 错误数量
	var errorCount int
	s.db.QueryRow(`
		SELECT COUNT(*) FROM events
		WHERE session_id = ? AND level IN ('error', 'fatal')
	`, sessionID).Scan(&errorCount)
	stats["errorCount"] = errorCount

	return stats, nil
}

// GetStoreStats returns database statistics
func (s *EventStore) GetStoreStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Total events count
	var totalEvents int64
	s.db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&totalEvents)
	stats["totalEvents"] = totalEvents

	// Total sessions count
	var totalSessions int64
	s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&totalSessions)
	stats["totalSessions"] = totalSessions

	// Database file size
	if info, err := os.Stat(s.dbPath); err == nil {
		stats["dbSizeBytes"] = info.Size()
	}

	// Write buffer status
	s.writeBufferMu.Lock()
	stats["writeBufferLen"] = len(s.writeBuffer)
	stats["writeBufferCap"] = cap(s.writeBuffer)
	s.writeBufferMu.Unlock()

	// Active sessions (not ended)
	var activeSessions int64
	s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE status = 'active'`).Scan(&activeSessions)
	stats["activeSessions"] = activeSessions

	return stats
}

// GetEventTypes 获取 Session 中所有事件类型
func (s *EventStore) GetEventTypes(sessionID string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT type FROM events
		WHERE session_id = ? AND type != ''
		ORDER BY type
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil && t != "" {
			types = append(types, t)
		}
	}
	return types, nil
}

// GetEventSources 获取 Session 中所有事件来源
func (s *EventStore) GetEventSources(sessionID string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT source FROM events
		WHERE session_id = ?
		ORDER BY source
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil && s != "" {
			sources = append(sources, s)
		}
	}
	return sources, nil
}

// GetEventLevels 获取 Session 中所有事件级别
func (s *EventStore) GetEventLevels(sessionID string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT level FROM events
		WHERE session_id = ?
		ORDER BY level
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var levels []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err == nil && l != "" {
			levels = append(levels, l)
		}
	}
	return levels, nil
}

// PreviewAssertionMatch 预览断言匹配的事件数量
func (s *EventStore) PreviewAssertionMatch(sessionID string, types []string, titleMatch string) (int, error) {
	query := `SELECT COUNT(*) FROM events WHERE session_id = ?`
	args := []interface{}{sessionID}

	if len(types) > 0 {
		placeholders := make([]string, len(types))
		for i := range types {
			placeholders[i] = "?"
			args = append(args, types[i])
		}
		query += ` AND type IN (` + strings.Join(placeholders, ",") + `)`
	}

	if titleMatch != "" {
		query += ` AND title REGEXP ?`
		args = append(args, titleMatch)
	}

	var count int
	err := s.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// CleanupOldSessions 清理旧 Session
func (s *EventStore) CleanupOldSessions(maxAge time.Duration) (int, error) {
	cutoff := time.Now().Add(-maxAge).UnixMilli()
	result, err := s.db.Exec(`
		DELETE FROM sessions
		WHERE end_time > 0 AND end_time < ?
	`, cutoff)
	if err != nil {
		return 0, err
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// VacuumDatabase 压缩数据库
func (s *EventStore) VacuumDatabase() error {
	_, err := s.db.Exec("VACUUM")
	return err
}

// ========================================
// Session Export/Import
// ========================================

// ExportSessionEvents exports all events (with data) for a session
func (s *EventStore) ExportSessionEvents(sessionID string) ([]UnifiedEvent, error) {
	rows, err := s.db.Query(`
		SELECT e.id, e.session_id, e.device_id, e.timestamp, e.relative_time, e.duration,
			e.source, e.category, e.type, e.level, e.title, e.summary,
			e.parent_id, e.step_id, e.trace_id,
			e.aggregate_count, e.aggregate_first, e.aggregate_last,
			e.tags, e.metadata, e.parent_event_id, e.generated_by_plugin,
			ed.data
		FROM events e
		LEFT JOIN event_data ed ON e.id = ed.event_id
		WHERE e.session_id = ?
		ORDER BY e.relative_time ASC
	`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("query events for export: %w", err)
	}
	defer rows.Close()

	var events []UnifiedEvent
	for rows.Next() {
		event, err := s.scanEventRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan event for export: %w", err)
		}
		events = append(events, *event)
	}
	return events, rows.Err()
}

// ImportSession imports a session with all its events and bookmarks
func (s *EventStore) ImportSession(session *DeviceSession, events []UnifiedEvent, bookmarks []Bookmark) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Insert session
	metadata, err := json.Marshal(session.Metadata)
	if err != nil {
		metadata = []byte("{}")
	}
	_, err = tx.Exec(`
		INSERT INTO sessions (
			id, device_id, type, name, start_time, end_time, status,
			event_count, video_path, video_duration, video_offset, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		session.ID, session.DeviceID, session.Type, session.Name,
		session.StartTime, session.EndTime, session.Status, len(events),
		nullString(session.VideoPath), session.VideoDuration, session.VideoOffset,
		string(metadata),
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}

	// 2. Batch insert events
	stmtEvent, err := tx.Prepare(`
		INSERT INTO events (
			id, session_id, device_id, timestamp, relative_time, duration,
			source, category, type, level, title, summary,
			parent_id, step_id, trace_id,
			aggregate_count, aggregate_first, aggregate_last,
			tags, metadata, parent_event_id, generated_by_plugin
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert event: %w", err)
	}
	defer stmtEvent.Close()

	stmtData, err := tx.Prepare(`
		INSERT OR REPLACE INTO event_data (event_id, data, data_size) VALUES (?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("prepare insert event data: %w", err)
	}
	defer stmtData.Close()

	for _, event := range events {
		var tagsJSON, metadataJSON string
		if len(event.Tags) > 0 {
			if b, err := json.Marshal(event.Tags); err == nil {
				tagsJSON = string(b)
			}
		}
		if len(event.Metadata) > 0 {
			if b, err := json.Marshal(event.Metadata); err == nil {
				metadataJSON = string(b)
			}
		}

		_, err = stmtEvent.Exec(
			event.ID, event.SessionID, event.DeviceID,
			event.Timestamp, event.RelativeTime, event.Duration,
			string(event.Source), string(event.Category), event.Type, string(event.Level),
			event.Title, nullString(event.Summary),
			nullString(event.ParentID), nullString(event.StepID), nullString(event.TraceID),
			event.AggregateCount, event.AggregateFirst, event.AggregateLast,
			nullString(tagsJSON), nullString(metadataJSON),
			nullString(event.ParentEventID), nullString(event.GeneratedByPlugin),
		)
		if err != nil {
			return fmt.Errorf("insert event %s: %w", event.ID, err)
		}

		if len(event.Data) > 0 {
			compressedData, cerr := compressData([]byte(event.Data))
			if cerr != nil {
				LogWarn("event_store").Err(cerr).Str("eventId", event.ID).Msg("Failed to compress event data, using raw data")
				compressedData = []byte(event.Data)
			}
			_, err = stmtData.Exec(event.ID, compressedData, len(event.Data))
			if err != nil {
				return fmt.Errorf("insert event data %s: %w", event.ID, err)
			}
		}
	}

	// 3. Insert bookmarks
	if len(bookmarks) > 0 {
		stmtBookmark, err := tx.Prepare(`
			INSERT INTO bookmarks (id, session_id, relative_time, label, color, type, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("prepare insert bookmark: %w", err)
		}
		defer stmtBookmark.Close()

		for _, b := range bookmarks {
			_, err = stmtBookmark.Exec(
				b.ID, b.SessionID, b.RelativeTime,
				b.Label, nullString(b.Color), b.Type, b.CreatedAt,
			)
			if err != nil {
				return fmt.Errorf("insert bookmark %s: %w", b.ID, err)
			}
		}
	}

	return tx.Commit()
}

// ========================================
// 辅助函数
// ========================================

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// ========================================
// Assertion 操作
// ========================================

// StoredAssertion 数据库中存储的断言结构
type StoredAssertion struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	SessionID   string `json:"sessionId,omitempty"`
	DeviceID    string `json:"deviceId,omitempty"`
	TimeRange   *struct {
		Start int64 `json:"start"`
		End   int64 `json:"end"`
	} `json:"timeRange,omitempty"`
	Criteria   json.RawMessage `json:"criteria"`
	Expected   json.RawMessage `json:"expected"`
	Timeout    int64           `json:"timeout,omitempty"`
	Tags       []string        `json:"tags,omitempty"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	IsTemplate bool            `json:"isTemplate"`
	CreatedAt  int64           `json:"createdAt"`
	UpdatedAt  int64           `json:"updatedAt"`
}

// CreateAssertion 创建断言
func (s *EventStore) CreateAssertion(assertion *StoredAssertion) error {
	var timeRangeStart, timeRangeEnd sql.NullInt64
	if assertion.TimeRange != nil {
		timeRangeStart = sql.NullInt64{Int64: assertion.TimeRange.Start, Valid: true}
		timeRangeEnd = sql.NullInt64{Int64: assertion.TimeRange.End, Valid: true}
	}

	tagsJSON, err := json.Marshal(assertion.Tags)
	if err != nil {
		LogWarn("event_store").Err(err).Str("assertionId", assertion.ID).Msg("Failed to marshal assertion tags")
		tagsJSON = []byte("[]")
	}
	isTemplate := 0
	if assertion.IsTemplate {
		isTemplate = 1
	}

	_, err = s.stmtInsertAssertion.Exec(
		assertion.ID, assertion.Name, nullString(assertion.Description),
		assertion.Type, nullString(assertion.SessionID), nullString(assertion.DeviceID),
		timeRangeStart, timeRangeEnd,
		string(assertion.Criteria), string(assertion.Expected),
		assertion.Timeout, string(tagsJSON), string(assertion.Metadata),
		isTemplate, assertion.CreatedAt, assertion.UpdatedAt,
	)
	return err
}

// UpdateAssertion 更新断言
func (s *EventStore) UpdateAssertion(assertion *StoredAssertion) error {
	var timeRangeStart, timeRangeEnd sql.NullInt64
	if assertion.TimeRange != nil {
		timeRangeStart = sql.NullInt64{Int64: assertion.TimeRange.Start, Valid: true}
		timeRangeEnd = sql.NullInt64{Int64: assertion.TimeRange.End, Valid: true}
	}

	tagsJSON, err := json.Marshal(assertion.Tags)
	if err != nil {
		LogWarn("event_store").Err(err).Str("assertionId", assertion.ID).Msg("Failed to marshal assertion tags")
		tagsJSON = []byte("[]")
	}
	isTemplate := 0
	if assertion.IsTemplate {
		isTemplate = 1
	}

	_, err = s.stmtUpdateAssertion.Exec(
		assertion.Name, nullString(assertion.Description),
		assertion.Type, nullString(assertion.SessionID), nullString(assertion.DeviceID),
		timeRangeStart, timeRangeEnd,
		string(assertion.Criteria), string(assertion.Expected),
		assertion.Timeout, string(tagsJSON), string(assertion.Metadata),
		isTemplate, time.Now().UnixMilli(),
		assertion.ID,
	)
	return err
}

// GetAssertion 获取单个断言
func (s *EventStore) GetAssertion(id string) (*StoredAssertion, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, type, session_id, device_id,
			time_range_start, time_range_end, criteria, expected,
			timeout, tags, metadata, is_template, created_at, updated_at
		FROM assertions WHERE id = ?
	`, id)
	return s.scanAssertion(row)
}

// ListAssertions 列出断言
func (s *EventStore) ListAssertions(sessionID string, deviceID string, templatesOnly bool, limit int) ([]StoredAssertion, error) {
	query := `
		SELECT id, name, description, type, session_id, device_id,
			time_range_start, time_range_end, criteria, expected,
			timeout, tags, metadata, is_template, created_at, updated_at
		FROM assertions WHERE 1=1
	`
	var args []interface{}

	if sessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, sessionID)
	}
	if deviceID != "" {
		query += ` AND device_id = ?`
		args = append(args, deviceID)
	}
	if templatesOnly {
		query += ` AND is_template = 1`
	}
	query += ` ORDER BY created_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var assertions []StoredAssertion
	for rows.Next() {
		a, err := s.scanAssertionRow(rows)
		if err != nil {
			return nil, err
		}
		assertions = append(assertions, *a)
	}
	return assertions, rows.Err()
}

// DeleteAssertion 删除断言
func (s *EventStore) DeleteAssertion(id string) error {
	_, err := s.db.Exec(`DELETE FROM assertions WHERE id = ?`, id)
	return err
}

// scanAssertion 扫描单个断言
func (s *EventStore) scanAssertion(row *sql.Row) (*StoredAssertion, error) {
	var a StoredAssertion
	var description, sessionID, deviceID sql.NullString
	var timeRangeStart, timeRangeEnd sql.NullInt64
	var criteria, expected, tags, metadata string
	var isTemplate int

	err := row.Scan(
		&a.ID, &a.Name, &description, &a.Type, &sessionID, &deviceID,
		&timeRangeStart, &timeRangeEnd, &criteria, &expected,
		&a.Timeout, &tags, &metadata, &isTemplate, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	a.Description = description.String
	a.SessionID = sessionID.String
	a.DeviceID = deviceID.String
	a.Criteria = json.RawMessage(criteria)
	a.Expected = json.RawMessage(expected)
	a.IsTemplate = isTemplate != 0

	if timeRangeStart.Valid && timeRangeEnd.Valid {
		a.TimeRange = &struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		}{Start: timeRangeStart.Int64, End: timeRangeEnd.Int64}
	}

	if tags != "" && tags != "null" {
		if err := json.Unmarshal([]byte(tags), &a.Tags); err != nil {
			LogWarn("event_store").Err(err).Str("assertionId", a.ID).Msg("Failed to unmarshal assertion tags")
		}
	}
	if metadata != "" && metadata != "null" {
		a.Metadata = json.RawMessage(metadata)
	}

	return &a, nil
}

// scanAssertionRow 扫描断言行
func (s *EventStore) scanAssertionRow(rows *sql.Rows) (*StoredAssertion, error) {
	var a StoredAssertion
	var description, sessionID, deviceID sql.NullString
	var timeRangeStart, timeRangeEnd sql.NullInt64
	var criteria, expected, tags, metadata string
	var isTemplate int

	err := rows.Scan(
		&a.ID, &a.Name, &description, &a.Type, &sessionID, &deviceID,
		&timeRangeStart, &timeRangeEnd, &criteria, &expected,
		&a.Timeout, &tags, &metadata, &isTemplate, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	a.Description = description.String
	a.SessionID = sessionID.String
	a.DeviceID = deviceID.String
	a.Criteria = json.RawMessage(criteria)
	a.Expected = json.RawMessage(expected)
	a.IsTemplate = isTemplate != 0

	if timeRangeStart.Valid && timeRangeEnd.Valid {
		a.TimeRange = &struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		}{Start: timeRangeStart.Int64, End: timeRangeEnd.Int64}
	}

	if tags != "" && tags != "null" {
		if err := json.Unmarshal([]byte(tags), &a.Tags); err != nil {
			LogWarn("event_store").Err(err).Str("assertionId", a.ID).Msg("Failed to unmarshal assertion tags")
		}
	}
	if metadata != "" && metadata != "null" {
		a.Metadata = json.RawMessage(metadata)
	}

	return &a, nil
}

// ========================================
// Assertion Result 操作
// ========================================

// StoredAssertionResult 数据库中存储的断言结果
type StoredAssertionResult struct {
	ID            string          `json:"id"`
	AssertionID   string          `json:"assertionId"`
	AssertionName string          `json:"assertionName"`
	SessionID     string          `json:"sessionId"`
	Passed        bool            `json:"passed"`
	Message       string          `json:"message"`
	MatchedEvents []string        `json:"matchedEvents,omitempty"`
	ActualValue   json.RawMessage `json:"actualValue,omitempty"`
	ExpectedValue json.RawMessage `json:"expectedValue,omitempty"`
	ExecutedAt    int64           `json:"executedAt"`
	Duration      int64           `json:"duration"`
	Details       json.RawMessage `json:"details,omitempty"`
}

// SaveAssertionResult 保存断言结果
func (s *EventStore) SaveAssertionResult(result *StoredAssertionResult) error {
	matchedEventsJSON, err := json.Marshal(result.MatchedEvents)
	if err != nil {
		LogWarn("event_store").Err(err).Str("resultId", result.ID).Msg("Failed to marshal matched events")
		matchedEventsJSON = []byte("[]")
	}
	passed := 0
	if result.Passed {
		passed = 1
	}

	_, err = s.stmtInsertAssertResult.Exec(
		result.ID, result.AssertionID, result.AssertionName, nullString(result.SessionID),
		passed, nullString(result.Message),
		string(matchedEventsJSON), string(result.ActualValue), string(result.ExpectedValue),
		result.ExecutedAt, result.Duration, string(result.Details),
	)
	return err
}

// ListAssertionResults 列出断言结果
func (s *EventStore) ListAssertionResults(sessionID string, assertionID string, limit int) ([]StoredAssertionResult, error) {
	query := `
		SELECT id, assertion_id, assertion_name, session_id, passed, message,
			matched_events, actual_value, expected_value, executed_at, duration, details
		FROM assertion_results WHERE 1=1
	`
	var args []interface{}

	if sessionID != "" {
		query += ` AND session_id = ?`
		args = append(args, sessionID)
	}
	if assertionID != "" {
		query += ` AND assertion_id = ?`
		args = append(args, assertionID)
	}
	query += ` ORDER BY executed_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []StoredAssertionResult
	for rows.Next() {
		r, err := s.scanAssertionResultRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *r)
	}
	return results, rows.Err()
}

// DeleteAssertionResult 删除断言结果
func (s *EventStore) DeleteAssertionResult(id string) error {
	_, err := s.db.Exec(`DELETE FROM assertion_results WHERE id = ?`, id)
	return err
}

// DeleteAssertionResultsBySession 删除 Session 的所有断言结果
func (s *EventStore) DeleteAssertionResultsBySession(sessionID string) error {
	_, err := s.db.Exec(`DELETE FROM assertion_results WHERE session_id = ?`, sessionID)
	return err
}

// scanAssertionResult 扫描单个断言结果
func (s *EventStore) scanAssertionResult(row *sql.Row) (*StoredAssertionResult, error) {
	var r StoredAssertionResult
	var sessionID, message sql.NullString
	var matchedEvents, actualValue, expectedValue, details string
	var passed int

	err := row.Scan(
		&r.ID, &r.AssertionID, &r.AssertionName, &sessionID, &passed, &message,
		&matchedEvents, &actualValue, &expectedValue, &r.ExecutedAt, &r.Duration, &details,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	r.SessionID = sessionID.String
	r.Message = message.String
	r.Passed = passed != 0

	if matchedEvents != "" && matchedEvents != "null" {
		if err := json.Unmarshal([]byte(matchedEvents), &r.MatchedEvents); err != nil {
			LogWarn("event_store").Err(err).Str("resultId", r.ID).Msg("Failed to unmarshal matched events")
		}
	}
	if actualValue != "" && actualValue != "null" {
		r.ActualValue = json.RawMessage(actualValue)
	}
	if expectedValue != "" && expectedValue != "null" {
		r.ExpectedValue = json.RawMessage(expectedValue)
	}
	if details != "" && details != "null" {
		r.Details = json.RawMessage(details)
	}

	return &r, nil
}

// scanAssertionResultRow 扫描断言结果行
func (s *EventStore) scanAssertionResultRow(rows *sql.Rows) (*StoredAssertionResult, error) {
	var r StoredAssertionResult
	var sessionID, message sql.NullString
	var matchedEvents, actualValue, expectedValue, details string
	var passed int

	err := rows.Scan(
		&r.ID, &r.AssertionID, &r.AssertionName, &sessionID, &passed, &message,
		&matchedEvents, &actualValue, &expectedValue, &r.ExecutedAt, &r.Duration, &details,
	)
	if err != nil {
		return nil, err
	}

	r.SessionID = sessionID.String
	r.Message = message.String
	r.Passed = passed != 0

	if matchedEvents != "" && matchedEvents != "null" {
		if err := json.Unmarshal([]byte(matchedEvents), &r.MatchedEvents); err != nil {
			LogWarn("event_store").Err(err).Str("resultId", r.ID).Msg("Failed to unmarshal matched events")
		}
	}
	if actualValue != "" && actualValue != "null" {
		r.ActualValue = json.RawMessage(actualValue)
	}
	if expectedValue != "" && expectedValue != "null" {
		r.ExpectedValue = json.RawMessage(expectedValue)
	}
	if details != "" && details != "null" {
		r.Details = json.RawMessage(details)
	}

	return &r, nil
}

// ========================================
// Assertion Set 操作
// ========================================

// CreateAssertionSet 创建断言集
func (s *EventStore) CreateAssertionSet(set *AssertionSet) error {
	assertionsJSON, err := json.Marshal(set.Assertions)
	if err != nil {
		return fmt.Errorf("marshal assertions: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO assertion_sets (id, name, description, assertions, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, set.ID, set.Name, nullString(set.Description), string(assertionsJSON),
		set.CreatedAt, set.UpdatedAt)
	return err
}

// UpdateAssertionSet 更新断言集
func (s *EventStore) UpdateAssertionSet(set *AssertionSet) error {
	assertionsJSON, err := json.Marshal(set.Assertions)
	if err != nil {
		return fmt.Errorf("marshal assertions: %w", err)
	}

	_, err = s.db.Exec(`
		UPDATE assertion_sets SET name = ?, description = ?, assertions = ?, updated_at = ?
		WHERE id = ?
	`, set.Name, nullString(set.Description), string(assertionsJSON),
		set.UpdatedAt, set.ID)
	return err
}

// DeleteAssertionSet 删除断言集
func (s *EventStore) DeleteAssertionSet(id string) error {
	_, err := s.db.Exec(`DELETE FROM assertion_sets WHERE id = ?`, id)
	return err
}

// GetAssertionSet 获取断言集
func (s *EventStore) GetAssertionSet(id string) (*AssertionSet, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, assertions, created_at, updated_at
		FROM assertion_sets WHERE id = ?
	`, id)

	var set AssertionSet
	var description sql.NullString
	var assertionsJSON string

	err := row.Scan(&set.ID, &set.Name, &description, &assertionsJSON,
		&set.CreatedAt, &set.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	set.Description = description.String
	if err := json.Unmarshal([]byte(assertionsJSON), &set.Assertions); err != nil {
		LogWarn("event_store").Err(err).Str("setId", set.ID).Msg("Failed to unmarshal assertion set assertions")
		set.Assertions = []string{}
	}

	return &set, nil
}

// ListAssertionSets 列出所有断言集
func (s *EventStore) ListAssertionSets() ([]AssertionSet, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, assertions, created_at, updated_at
		FROM assertion_sets ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sets []AssertionSet
	for rows.Next() {
		var set AssertionSet
		var description sql.NullString
		var assertionsJSON string

		err := rows.Scan(&set.ID, &set.Name, &description, &assertionsJSON,
			&set.CreatedAt, &set.UpdatedAt)
		if err != nil {
			return nil, err
		}

		set.Description = description.String
		if err := json.Unmarshal([]byte(assertionsJSON), &set.Assertions); err != nil {
			LogWarn("event_store").Err(err).Str("setId", set.ID).Msg("Failed to unmarshal assertion set assertions")
			set.Assertions = []string{}
		}

		sets = append(sets, set)
	}
	return sets, rows.Err()
}

// SaveAssertionSetResult 保存断言集执行结果
func (s *EventStore) SaveAssertionSetResult(result *AssertionSetResult) error {
	summaryJSON, err := json.Marshal(result.Summary)
	if err != nil {
		return fmt.Errorf("marshal summary: %w", err)
	}
	resultsJSON, err := json.Marshal(result.Results)
	if err != nil {
		return fmt.Errorf("marshal results: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO assertion_set_results (
			id, set_id, set_name, session_id, device_id, execution_id,
			start_time, end_time, duration, status, summary, results, executed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, result.ID, result.SetID, result.SetName,
		nullString(result.SessionID), nullString(result.DeviceID),
		result.ExecutionID,
		result.StartTime, result.EndTime, result.Duration,
		result.Status, string(summaryJSON), string(resultsJSON),
		result.ExecutedAt)
	return err
}

// ListAssertionSetResults 列出断言集执行结果
func (s *EventStore) ListAssertionSetResults(setID string, limit int) ([]AssertionSetResult, error) {
	query := `
		SELECT id, set_id, set_name, session_id, device_id, execution_id,
			start_time, end_time, duration, status, summary, results, executed_at
		FROM assertion_set_results
	`
	var args []interface{}

	if setID != "" {
		query += ` WHERE set_id = ?`
		args = append(args, setID)
	}
	query += ` ORDER BY executed_at DESC`
	if limit > 0 {
		query += fmt.Sprintf(` LIMIT %d`, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AssertionSetResult
	for rows.Next() {
		r, err := s.scanAssertionSetResultRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, *r)
	}
	return results, rows.Err()
}

// GetAssertionSetResult 获取单个断言集执行结果
func (s *EventStore) GetAssertionSetResult(executionID string) (*AssertionSetResult, error) {
	row := s.db.QueryRow(`
		SELECT id, set_id, set_name, session_id, device_id, execution_id,
			start_time, end_time, duration, status, summary, results, executed_at
		FROM assertion_set_results WHERE execution_id = ?
	`, executionID)

	var r AssertionSetResult
	var sessionID, deviceID sql.NullString
	var endTime sql.NullInt64
	var summaryJSON, resultsJSON string

	err := row.Scan(&r.ID, &r.SetID, &r.SetName, &sessionID, &deviceID,
		&r.ExecutionID, &r.StartTime, &endTime, &r.Duration,
		&r.Status, &summaryJSON, &resultsJSON, &r.ExecutedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	r.SessionID = sessionID.String
	r.DeviceID = deviceID.String
	if endTime.Valid {
		r.EndTime = endTime.Int64
	}
	if summaryJSON != "" {
		json.Unmarshal([]byte(summaryJSON), &r.Summary)
	}
	if resultsJSON != "" {
		json.Unmarshal([]byte(resultsJSON), &r.Results)
	}

	return &r, nil
}

// scanAssertionSetResultRow 扫描断言集结果行
func (s *EventStore) scanAssertionSetResultRow(rows *sql.Rows) (*AssertionSetResult, error) {
	var r AssertionSetResult
	var sessionID, deviceID sql.NullString
	var endTime sql.NullInt64
	var summaryJSON, resultsJSON string

	err := rows.Scan(&r.ID, &r.SetID, &r.SetName, &sessionID, &deviceID,
		&r.ExecutionID, &r.StartTime, &endTime, &r.Duration,
		&r.Status, &summaryJSON, &resultsJSON, &r.ExecutedAt)
	if err != nil {
		return nil, err
	}

	r.SessionID = sessionID.String
	r.DeviceID = deviceID.String
	if endTime.Valid {
		r.EndTime = endTime.Int64
	}
	if summaryJSON != "" {
		json.Unmarshal([]byte(summaryJSON), &r.Summary)
	}
	if resultsJSON != "" {
		json.Unmarshal([]byte(resultsJSON), &r.Results)
	}

	return &r, nil
}
