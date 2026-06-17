package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========================================
// Event Assertion System - 事件断言系统
// 验证设备/应用行为是否符合预期
// ========================================

// AssertionType 断言类型
type AssertionType string

const (
	AssertExists    AssertionType = "exists"     // 事件存在
	AssertNotExists AssertionType = "not_exists" // 事件不存在
	AssertSequence  AssertionType = "sequence"   // 事件顺序
	AssertTiming    AssertionType = "timing"     // 事件时间间隔
	AssertCount     AssertionType = "count"      // 事件数量
	AssertCondition AssertionType = "condition"  // 条件匹配
)

// Assertion 断言定义
type Assertion struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Type        AssertionType          `json:"type"`
	SessionID   string                 `json:"sessionId,omitempty"`
	DeviceID    string                 `json:"deviceId,omitempty"`
	TimeRange   *TimeRange             `json:"timeRange,omitempty"`
	Criteria    EventCriteria          `json:"criteria"`
	Expected    AssertionExpected      `json:"expected"`
	Timeout     int64                  `json:"timeout,omitempty"` // ms, for real-time assertions
	CreatedAt   int64                  `json:"createdAt"`
	Tags        []string               `json:"tags,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TimeRange 时间范围
type TimeRange struct {
	Start int64 `json:"start"` // 相对时间 (ms)
	End   int64 `json:"end"`
}

// EventCriteria 事件匹配条件
type EventCriteria struct {
	Sources    []EventSource   `json:"sources,omitempty"`
	Categories []EventCategory `json:"categories,omitempty"`
	Types      []string        `json:"types,omitempty"`
	Levels     []EventLevel    `json:"levels,omitempty"`
	TitleMatch string          `json:"titleMatch,omitempty"` // 正则表达式
	DataMatch  []DataMatcher   `json:"dataMatch,omitempty"`  // 数据字段匹配
}

// DataMatcher 数据字段匹配器
type DataMatcher struct {
	Path     string      `json:"path"`     // JSON path (如 "statusCode", "packageName")
	Operator string      `json:"operator"` // eq, ne, gt, lt, gte, lte, contains, regex, exists
	Value    interface{} `json:"value"`
}

// AssertionExpected 期望值
type AssertionExpected struct {
	// For exists/not_exists
	Exists bool `json:"exists,omitempty"`

	// For count
	Count    *int `json:"count,omitempty"`
	MinCount *int `json:"minCount,omitempty"`
	MaxCount *int `json:"maxCount,omitempty"`

	// For sequence
	Sequence []EventCriteria `json:"sequence,omitempty"`
	Ordered  bool            `json:"ordered,omitempty"` // true=严格顺序, false=只要都出现

	// For timing
	MinInterval int64 `json:"minInterval,omitempty"` // ms
	MaxInterval int64 `json:"maxInterval,omitempty"` // ms

	// For condition - custom expression
	Expression string `json:"expression,omitempty"`
}

// AssertionResult 断言结果
type AssertionResult struct {
	ID            string                 `json:"id"`
	AssertionID   string                 `json:"assertionId"`
	AssertionName string                 `json:"assertionName"`
	SessionID     string                 `json:"sessionId"`
	Passed        bool                   `json:"passed"`
	Message       string                 `json:"message"`
	MatchedEvents []string               `json:"matchedEvents,omitempty"` // 匹配的事件 ID
	ActualValue   interface{}            `json:"actualValue,omitempty"`
	ExpectedValue interface{}            `json:"expectedValue,omitempty"`
	ExecutedAt    int64                  `json:"executedAt"`
	Duration      int64                  `json:"duration"` // ms
	Details       map[string]interface{} `json:"details,omitempty"`
}

// ========================================
// AssertionEngine - 断言执行引擎
// ========================================

type AssertionEngine struct {
	app      *App
	store    *EventStore
	pipeline *EventPipeline

	// 实时断言监控
	liveAssertions map[string]*LiveAssertion
	liveMu         sync.RWMutex

	// 断言结果缓存
	results   map[string]*AssertionResult
	resultsMu sync.RWMutex
}

// LiveAssertion 实时断言 (等待事件触发)
type LiveAssertion struct {
	Assertion   *Assertion
	StartTime   int64
	MatchBuffer []UnifiedEvent
	ResultChan  chan *AssertionResult
	Cancel      chan struct{}
}

// NewAssertionEngine 创建断言引擎
func NewAssertionEngine(app *App, store *EventStore, pipeline *EventPipeline) *AssertionEngine {
	return &AssertionEngine{
		app:            app,
		store:          store,
		pipeline:       pipeline,
		liveAssertions: make(map[string]*LiveAssertion),
		results:        make(map[string]*AssertionResult),
	}
}

// ========================================
// 断言执行
// ========================================

// ExecuteAssertion 执行断言 (基于已有事件)
func (e *AssertionEngine) ExecuteAssertion(assertion *Assertion) (*AssertionResult, error) {
	startTime := time.Now()

	result := &AssertionResult{
		ID:            uuid.New().String(),
		AssertionID:   assertion.ID,
		AssertionName: assertion.Name,
		SessionID:     assertion.SessionID,
		ExecutedAt:    startTime.UnixMilli(),
	}

	// 查询匹配的事件
	events, err := e.queryMatchingEvents(assertion)
	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Query failed: %v", err)
		result.Duration = time.Since(startTime).Milliseconds()
		return result, err
	}

	// 根据断言类型执行验证
	switch assertion.Type {
	case AssertExists:
		e.evaluateExists(assertion, events, result)
	case AssertNotExists:
		e.evaluateNotExists(assertion, events, result)
	case AssertCount:
		e.evaluateCount(assertion, events, result)
	case AssertSequence:
		e.evaluateSequence(assertion, events, result)
	case AssertTiming:
		e.evaluateTiming(assertion, events, result)
	case AssertCondition:
		e.evaluateCondition(assertion, events, result)
	default:
		result.Passed = false
		result.Message = fmt.Sprintf("Unknown assertion type: %s", assertion.Type)
	}

	result.Duration = time.Since(startTime).Milliseconds()

	// 缓存结果
	e.resultsMu.Lock()
	e.results[result.ID] = result
	e.resultsMu.Unlock()

	// 持久化结果到数据库
	e.persistResult(result)

	// 发送断言结果事件
	e.emitAssertionResult(assertion, result)

	return result, nil
}

// persistResult 持久化断言结果到数据库
func (e *AssertionEngine) persistResult(result *AssertionResult) {
	if e.store == nil {
		return
	}

	actualValueJSON, err := json.Marshal(result.ActualValue)
	if err != nil {
		LogWarn("event_assertion").Err(err).Str("resultId", result.ID).Msg("Failed to marshal actual value")
		actualValueJSON = []byte("null")
	}
	expectedValueJSON, err := json.Marshal(result.ExpectedValue)
	if err != nil {
		LogWarn("event_assertion").Err(err).Str("resultId", result.ID).Msg("Failed to marshal expected value")
		expectedValueJSON = []byte("null")
	}
	detailsJSON, err := json.Marshal(result.Details)
	if err != nil {
		LogWarn("event_assertion").Err(err).Str("resultId", result.ID).Msg("Failed to marshal details")
		detailsJSON = []byte("{}")
	}

	storedResult := &StoredAssertionResult{
		ID:            result.ID,
		AssertionID:   result.AssertionID,
		AssertionName: result.AssertionName,
		SessionID:     result.SessionID,
		Passed:        result.Passed,
		Message:       result.Message,
		MatchedEvents: result.MatchedEvents,
		ActualValue:   actualValueJSON,
		ExpectedValue: expectedValueJSON,
		ExecutedAt:    result.ExecutedAt,
		Duration:      result.Duration,
		Details:       detailsJSON,
	}

	if err := e.store.SaveAssertionResult(storedResult); err != nil {
		LogDebug("assertion").Err(err).Msg("Failed to persist assertion result")
	}
}

// queryMatchingEvents 查询匹配条件的事件
func (e *AssertionEngine) queryMatchingEvents(assertion *Assertion) ([]UnifiedEvent, error) {
	query := EventQuery{
		SessionID: assertion.SessionID,
		DeviceID:  assertion.DeviceID,
	}

	if len(assertion.Criteria.Sources) > 0 {
		query.Sources = assertion.Criteria.Sources
	}
	if len(assertion.Criteria.Categories) > 0 {
		query.Categories = assertion.Criteria.Categories
	}
	if len(assertion.Criteria.Types) > 0 {
		query.Types = expandAggregatedTypes(assertion.Criteria.Types)
	}
	if len(assertion.Criteria.Levels) > 0 {
		query.Levels = assertion.Criteria.Levels
	}
	if assertion.TimeRange != nil {
		query.StartTime = assertion.TimeRange.Start
		query.EndTime = assertion.TimeRange.End
	}

	queryResult, err := e.store.QueryEvents(query)
	if err != nil {
		return nil, err
	}
	events := queryResult.Events

	// 应用额外过滤
	filtered := e.filterEvents(events, assertion.Criteria)
	return filtered, nil
}

// filterEvents 过滤事件
func (e *AssertionEngine) filterEvents(events []UnifiedEvent, criteria EventCriteria) []UnifiedEvent {
	result := make([]UnifiedEvent, 0)

	var titleRegex *regexp.Regexp
	if criteria.TitleMatch != "" {
		titleRegex, _ = regexp.Compile(criteria.TitleMatch)
	}

	for _, event := range events {
		// 标题匹配
		if titleRegex != nil && !titleRegex.MatchString(event.Title) {
			continue
		}

		// 数据字段匹配
		if len(criteria.DataMatch) > 0 {
			if !e.matchEventData(event, criteria.DataMatch) {
				continue
			}
		}

		result = append(result, event)
	}

	return result
}

// matchEventData 匹配事件数据字段
func (e *AssertionEngine) matchEventData(event UnifiedEvent, matchers []DataMatcher) bool {
	if len(event.Data) == 0 {
		return false
	}

	var data map[string]interface{}
	if err := json.Unmarshal(event.Data, &data); err != nil {
		return false
	}

	for _, matcher := range matchers {
		value := getJSONPath(data, matcher.Path)
		if !matchValue(value, matcher.Operator, matcher.Value) {
			return false
		}
	}

	return true
}

// ========================================
// 各类型断言评估
// ========================================

func (e *AssertionEngine) evaluateExists(assertion *Assertion, events []UnifiedEvent, result *AssertionResult) {
	exists := len(events) > 0
	result.Passed = exists == assertion.Expected.Exists
	result.ActualValue = len(events)
	result.ExpectedValue = "exists"

	if result.Passed {
		result.Message = fmt.Sprintf("Found %d matching events", len(events))
	} else {
		result.Message = "No matching events found"
	}

	// 记录匹配的事件
	for _, ev := range events {
		result.MatchedEvents = append(result.MatchedEvents, ev.ID)
	}
}

func (e *AssertionEngine) evaluateNotExists(assertion *Assertion, events []UnifiedEvent, result *AssertionResult) {
	exists := len(events) > 0
	result.Passed = !exists
	result.ActualValue = len(events)
	result.ExpectedValue = "not exists"

	if result.Passed {
		result.Message = "No matching events found (as expected)"
	} else {
		result.Message = fmt.Sprintf("Found %d unexpected events", len(events))
		for _, ev := range events {
			result.MatchedEvents = append(result.MatchedEvents, ev.ID)
		}
	}
}

func (e *AssertionEngine) evaluateCount(assertion *Assertion, events []UnifiedEvent, result *AssertionResult) {
	count := len(events)
	result.ActualValue = count

	passed := true
	var messages []string

	if assertion.Expected.Count != nil {
		expected := *assertion.Expected.Count
		result.ExpectedValue = expected
		if count != expected {
			passed = false
			messages = append(messages, fmt.Sprintf("expected %d, got %d", expected, count))
		}
	}

	if assertion.Expected.MinCount != nil {
		minCount := *assertion.Expected.MinCount
		if count < minCount {
			passed = false
			messages = append(messages, fmt.Sprintf("expected at least %d, got %d", minCount, count))
		}
	}

	if assertion.Expected.MaxCount != nil {
		maxCount := *assertion.Expected.MaxCount
		if count > maxCount {
			passed = false
			messages = append(messages, fmt.Sprintf("expected at most %d, got %d", maxCount, count))
		}
	}

	result.Passed = passed
	if passed {
		result.Message = fmt.Sprintf("Count check passed: %d events", count)
	} else {
		result.Message = strings.Join(messages, "; ")
	}

	for _, ev := range events {
		result.MatchedEvents = append(result.MatchedEvents, ev.ID)
	}
}

func (e *AssertionEngine) evaluateSequence(assertion *Assertion, events []UnifiedEvent, result *AssertionResult) {
	if len(assertion.Expected.Sequence) == 0 {
		result.Passed = true
		result.Message = "No sequence to check"
		return
	}

	// 按时间排序
	sortedEvents := make([]UnifiedEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	// 查找序列
	seqIndex := 0
	matchedIDs := make([]string, 0)

	if assertion.Expected.Ordered {
		// 严格顺序匹配: 每个 criteria 必须在前一个之后出现
		for _, event := range sortedEvents {
			if seqIndex >= len(assertion.Expected.Sequence) {
				break
			}

			criteria := assertion.Expected.Sequence[seqIndex]
			if e.eventMatchesCriteria(event, criteria) {
				matchedIDs = append(matchedIDs, event.ID)
				seqIndex++
			}
		}
	} else {
		// 乱序匹配: 只要求每个 criteria 都能找到事件，不要求出现顺序
		// 已匹配的事件 ID 去重，避免同一事件满足多个 criteria。
		// 已知局限（有意保留的简化实现）：按 criteria 顺序贪心 first-fit，
		// 当多个 criteria 的匹配集重叠时，宽泛 criteria 先抢走唯一事件
		// 可能导致存在合法分配却判失败；完整解需要二分图匹配，当前场景
		// 不值得该复杂度
		used := make(map[string]bool, len(assertion.Expected.Sequence))
		for _, criteria := range assertion.Expected.Sequence {
			for _, event := range sortedEvents {
				if used[event.ID] {
					continue
				}
				if e.eventMatchesCriteria(event, criteria) {
					used[event.ID] = true
					matchedIDs = append(matchedIDs, event.ID)
					seqIndex++
					break
				}
			}
		}
	}

	result.Passed = seqIndex == len(assertion.Expected.Sequence)
	result.MatchedEvents = matchedIDs
	result.ActualValue = seqIndex
	result.ExpectedValue = len(assertion.Expected.Sequence)

	if result.Passed {
		result.Message = fmt.Sprintf("Sequence matched: %d/%d steps", seqIndex, len(assertion.Expected.Sequence))
	} else {
		result.Message = fmt.Sprintf("Sequence incomplete: matched %d/%d steps", seqIndex, len(assertion.Expected.Sequence))
	}
}

func (e *AssertionEngine) evaluateTiming(assertion *Assertion, events []UnifiedEvent, result *AssertionResult) {
	if len(events) < 2 {
		result.Passed = false
		result.Message = "Need at least 2 events for timing check"
		return
	}

	// 按时间排序
	sortedEvents := make([]UnifiedEvent, len(events))
	copy(sortedEvents, events)
	sort.Slice(sortedEvents, func(i, j int) bool {
		return sortedEvents[i].Timestamp < sortedEvents[j].Timestamp
	})

	// 计算相邻事件间隔
	intervals := make([]int64, 0)
	for i := 1; i < len(sortedEvents); i++ {
		interval := sortedEvents[i].Timestamp - sortedEvents[i-1].Timestamp
		intervals = append(intervals, interval)
	}

	passed := true
	var failedIntervals []int

	for i, interval := range intervals {
		if assertion.Expected.MinInterval > 0 && interval < assertion.Expected.MinInterval {
			passed = false
			failedIntervals = append(failedIntervals, i)
		}
		if assertion.Expected.MaxInterval > 0 && interval > assertion.Expected.MaxInterval {
			passed = false
			failedIntervals = append(failedIntervals, i)
		}
	}

	result.Passed = passed
	result.ActualValue = intervals
	result.ExpectedValue = map[string]int64{
		"minInterval": assertion.Expected.MinInterval,
		"maxInterval": assertion.Expected.MaxInterval,
	}
	result.Details = map[string]interface{}{
		"intervals":       intervals,
		"failedIntervals": failedIntervals,
	}

	if passed {
		result.Message = fmt.Sprintf("Timing check passed: %d intervals", len(intervals))
	} else {
		result.Message = fmt.Sprintf("Timing check failed: %d intervals out of range", len(failedIntervals))
	}
}

func (e *AssertionEngine) evaluateCondition(assertion *Assertion, events []UnifiedEvent, result *AssertionResult) {
	// 简单条件评估 - 可扩展为完整表达式解析
	expression := assertion.Expected.Expression
	if expression == "" {
		result.Passed = true
		result.Message = "No condition to evaluate"
		return
	}

	// 目前支持简单条件: "count > N", "any.field == value"
	result.Passed = false
	result.Message = "Condition evaluation: " + expression

	// 解析 count 条件
	if strings.HasPrefix(expression, "count") {
		var op string
		var num int
		if n, _ := fmt.Sscanf(expression, "count > %d", &num); n == 1 {
			op = ">"
		} else if n, _ := fmt.Sscanf(expression, "count >= %d", &num); n == 1 {
			op = ">="
		} else if n, _ := fmt.Sscanf(expression, "count < %d", &num); n == 1 {
			op = "<"
		} else if n, _ := fmt.Sscanf(expression, "count <= %d", &num); n == 1 {
			op = "<="
		} else if n, _ := fmt.Sscanf(expression, "count == %d", &num); n == 1 {
			op = "=="
		}

		count := len(events)
		switch op {
		case ">":
			result.Passed = count > num
		case ">=":
			result.Passed = count >= num
		case "<":
			result.Passed = count < num
		case "<=":
			result.Passed = count <= num
		case "==":
			result.Passed = count == num
		}

		result.ActualValue = count
		result.ExpectedValue = fmt.Sprintf("%s %d", op, num)
	}
}

// ========================================
// 辅助函数
// ========================================

// typeMatchesCriteria reports whether an event type satisfies a criteria type.
// "logcat" also matches "logcat_aggregated": bucketed aggregation (logcat.go)
// merges log bursts into aggregated events, and per-type filters written as
// ["logcat"] would otherwise silently miss most log lines.
func typeMatchesCriteria(criteriaType, eventType string) bool {
	if criteriaType == eventType {
		return true
	}
	return criteriaType == "logcat" && eventType == "logcat_aggregated"
}

// expandAggregatedTypes widens a type filter list for SQL-level filtering so
// aggregated variants are not excluded before typeMatchesCriteria runs.
func expandAggregatedTypes(types []string) []string {
	out := append([]string(nil), types...)
	for _, t := range types {
		if t == "logcat" {
			out = append(out, "logcat_aggregated")
			break
		}
	}
	return out
}

func (e *AssertionEngine) eventMatchesCriteria(event UnifiedEvent, criteria EventCriteria) bool {
	if len(criteria.Sources) > 0 {
		found := false
		for _, s := range criteria.Sources {
			if event.Source == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(criteria.Categories) > 0 {
		found := false
		for _, c := range criteria.Categories {
			if event.Category == c {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(criteria.Types) > 0 {
		found := false
		for _, t := range criteria.Types {
			if typeMatchesCriteria(t, event.Type) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if criteria.TitleMatch != "" {
		if re, err := regexp.Compile(criteria.TitleMatch); err == nil {
			if !re.MatchString(event.Title) {
				return false
			}
		}
	}

	return true
}

func (e *AssertionEngine) emitAssertionResult(assertion *Assertion, result *AssertionResult) {
	if e.pipeline == nil {
		return
	}

	level := LevelInfo
	if !result.Passed {
		level = LevelError
	}

	title := fmt.Sprintf("Assertion %s: %s", assertion.Name, map[bool]string{true: "PASSED", false: "FAILED"}[result.Passed])

	data, err := json.Marshal(result)
	if err != nil {
		LogWarn("event_assertion").Err(err).Str("assertionName", assertion.Name).Msg("Failed to marshal assertion result")
		data = []byte("{}")
	}

	e.pipeline.Emit(UnifiedEvent{
		DeviceID:  assertion.DeviceID,
		SessionID: assertion.SessionID,
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceAssertion,
		Category:  CategoryDiagnostic,
		Type:      "assertion_result",
		Level:     level,
		Title:     title,
		Data:      data,
	})
}

// getJSONPath 获取 JSON 路径值
func getJSONPath(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

// matchValue 匹配值
func matchValue(actual interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "eq", "==":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	case "ne", "!=":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected)
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", actual), fmt.Sprintf("%v", expected))
	case "regex":
		if re, err := regexp.Compile(fmt.Sprintf("%v", expected)); err == nil {
			return re.MatchString(fmt.Sprintf("%v", actual))
		}
		return false
	case "exists":
		return actual != nil
	case "gt", ">":
		return toFloat64(actual) > toFloat64(expected)
	case "gte", ">=":
		return toFloat64(actual) >= toFloat64(expected)
	case "lt", "<":
		return toFloat64(actual) < toFloat64(expected)
	case "lte", "<=":
		return toFloat64(actual) <= toFloat64(expected)
	default:
		return false
	}
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}

// ========================================
// Assertion Set Types - 断言集类型定义
// ========================================

// AssertionSet 断言集定义
type AssertionSet struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Assertions  []string `json:"assertions"` // 断言模板 ID 数组
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
}

// AssertionSetSummary 断言集执行摘要
type AssertionSetSummary struct {
	Total    int     `json:"total"`
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Error    int     `json:"error"`
	PassRate float64 `json:"passRate"`
}

// AssertionSetResult 断言集执行结果
type AssertionSetResult struct {
	ID          string              `json:"id"`
	SetID       string              `json:"setId"`
	SetName     string              `json:"setName"`
	SessionID   string              `json:"sessionId"`
	DeviceID    string              `json:"deviceId"`
	ExecutionID string              `json:"executionId"`
	StartTime   int64               `json:"startTime"`
	EndTime     int64               `json:"endTime"`
	Duration    int64               `json:"duration"` // ms
	Status      string              `json:"status"`   // "passed", "failed", "partial", "error"
	Summary     AssertionSetSummary `json:"summary"`
	Results     []AssertionResult   `json:"results"`
	ExecutedAt  int64               `json:"executedAt"`
}

// ========================================
// Assertion Set Engine - 断言集执行引擎
// ========================================

// ExecuteAssertionSet 执行断言集 (并行执行所有断言)
func (e *AssertionEngine) ExecuteAssertionSet(set *AssertionSet, sessionID, deviceID string) (*AssertionSetResult, error) {
	startTime := time.Now()
	executionID := uuid.New().String()

	setResult := &AssertionSetResult{
		ID:          uuid.New().String(),
		SetID:       set.ID,
		SetName:     set.Name,
		SessionID:   sessionID,
		DeviceID:    deviceID,
		ExecutionID: executionID,
		StartTime:   startTime.UnixMilli(),
		Status:      "running",
		ExecutedAt:  startTime.UnixMilli(),
	}

	if len(set.Assertions) == 0 {
		setResult.EndTime = time.Now().UnixMilli()
		setResult.Duration = setResult.EndTime - setResult.StartTime
		setResult.Status = "passed"
		setResult.Summary = AssertionSetSummary{Total: 0, PassRate: 100}
		return setResult, nil
	}

	// 加载所有断言模板
	assertions := make([]*Assertion, 0, len(set.Assertions))
	for _, assertionID := range set.Assertions {
		stored, err := e.GetStoredAssertion(assertionID)
		if err != nil || stored == nil {
			LogWarn("assertion_set").Str("assertionId", assertionID).Msg("Assertion not found, skipping")
			continue
		}
		// 转换为运行时断言
		assertion, err := e.storedToRuntime(stored, sessionID, deviceID)
		if err != nil {
			LogWarn("assertion_set").Err(err).Str("assertionId", assertionID).Msg("Failed to convert assertion")
			continue
		}
		assertions = append(assertions, assertion)
	}

	// 并行执行所有断言
	type resultEntry struct {
		index  int
		result *AssertionResult
		err    error
	}

	resultChan := make(chan resultEntry, len(assertions))
	for i, assertion := range assertions {
		go func(idx int, a *Assertion) {
			r, err := e.ExecuteAssertion(a)
			resultChan <- resultEntry{index: idx, result: r, err: err}
		}(i, assertion)
	}

	// 收集结果
	results := make([]AssertionResult, len(assertions))
	summary := AssertionSetSummary{Total: len(assertions)}

	for range assertions {
		entry := <-resultChan
		if entry.err != nil {
			summary.Error++
			results[entry.index] = AssertionResult{
				ID:            uuid.New().String(),
				AssertionID:   assertions[entry.index].ID,
				AssertionName: assertions[entry.index].Name,
				SessionID:     sessionID,
				Passed:        false,
				Message:       fmt.Sprintf("Execution error: %v", entry.err),
				ExecutedAt:    time.Now().UnixMilli(),
			}
		} else if entry.result != nil {
			results[entry.index] = *entry.result
			if entry.result.Passed {
				summary.Passed++
			} else {
				summary.Failed++
			}
		}
	}

	// 计算摘要
	if summary.Total > 0 {
		summary.PassRate = float64(summary.Passed) / float64(summary.Total) * 100
	}

	// 确定状态
	endTime := time.Now()
	setResult.EndTime = endTime.UnixMilli()
	setResult.Duration = endTime.Sub(startTime).Milliseconds()
	setResult.Results = results
	setResult.Summary = summary

	if summary.Failed == 0 && summary.Error == 0 {
		setResult.Status = "passed"
	} else if summary.Passed == 0 {
		setResult.Status = "failed"
	} else {
		setResult.Status = "partial"
	}

	// 发送断言集执行完成事件
	e.emitAssertionSetResult(setResult)

	return setResult, nil
}

// storedToRuntime 将存储的断言转换为运行时断言
func (e *AssertionEngine) storedToRuntime(stored *StoredAssertion, sessionID, deviceID string) (*Assertion, error) {
	var criteria EventCriteria
	if err := json.Unmarshal(stored.Criteria, &criteria); err != nil {
		return nil, fmt.Errorf("invalid criteria: %w", err)
	}
	var expected AssertionExpected
	if err := json.Unmarshal(stored.Expected, &expected); err != nil {
		return nil, fmt.Errorf("invalid expected: %w", err)
	}
	var metadata map[string]interface{}
	if len(stored.Metadata) > 0 {
		json.Unmarshal(stored.Metadata, &metadata)
	}

	effectiveSessionID := sessionID
	if effectiveSessionID == "" {
		effectiveSessionID = stored.SessionID
	}
	effectiveDeviceID := deviceID
	if effectiveDeviceID == "" {
		effectiveDeviceID = stored.DeviceID
	}

	assertion := &Assertion{
		ID:          stored.ID,
		Name:        stored.Name,
		Description: stored.Description,
		Type:        AssertionType(stored.Type),
		SessionID:   effectiveSessionID,
		DeviceID:    effectiveDeviceID,
		Criteria:    criteria,
		Expected:    expected,
		Timeout:     stored.Timeout,
		Tags:        stored.Tags,
		Metadata:    metadata,
		CreatedAt:   stored.CreatedAt,
	}

	if stored.TimeRange != nil {
		assertion.TimeRange = &TimeRange{
			Start: stored.TimeRange.Start,
			End:   stored.TimeRange.End,
		}
	}

	return assertion, nil
}

// emitAssertionSetResult 发送断言集执行结果事件
func (e *AssertionEngine) emitAssertionSetResult(setResult *AssertionSetResult) {
	if e.pipeline == nil {
		return
	}

	level := LevelInfo
	if setResult.Status == "failed" {
		level = LevelError
	} else if setResult.Status == "partial" {
		level = LevelWarn
	}

	title := fmt.Sprintf("Assertion Set %s: %s (%d/%d passed)",
		setResult.SetName,
		strings.ToUpper(setResult.Status),
		setResult.Summary.Passed,
		setResult.Summary.Total,
	)

	data, err := json.Marshal(map[string]interface{}{
		"setId":       setResult.SetID,
		"setName":     setResult.SetName,
		"executionId": setResult.ExecutionID,
		"status":      setResult.Status,
		"summary":     setResult.Summary,
		"duration":    setResult.Duration,
	})
	if err != nil {
		data = []byte("{}")
	}

	e.pipeline.Emit(UnifiedEvent{
		DeviceID:  setResult.DeviceID,
		SessionID: setResult.SessionID,
		Timestamp: time.Now().UnixMilli(),
		Source:    SourceAssertion,
		Category:  CategoryDiagnostic,
		Type:      "assertion_set_complete",
		Level:     level,
		Title:     title,
		Data:      data,
	})
}

// ========================================
// Assertion Set CRUD Methods
// ========================================

// CreateAssertionSet 创建断言集
func (e *AssertionEngine) CreateAssertionSet(name, description string, assertionIDs []string) (string, error) {
	if e.store == nil {
		return "", fmt.Errorf("store not initialized")
	}

	id := uuid.New().String()
	now := time.Now().UnixMilli()

	set := &AssertionSet{
		ID:          id,
		Name:        name,
		Description: description,
		Assertions:  assertionIDs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := e.store.CreateAssertionSet(set); err != nil {
		return "", err
	}

	return id, nil
}

// UpdateAssertionSet 更新断言集
func (e *AssertionEngine) UpdateAssertionSet(id, name, description string, assertionIDs []string) error {
	if e.store == nil {
		return fmt.Errorf("store not initialized")
	}

	set := &AssertionSet{
		ID:          id,
		Name:        name,
		Description: description,
		Assertions:  assertionIDs,
		UpdatedAt:   time.Now().UnixMilli(),
	}

	return e.store.UpdateAssertionSet(set)
}

// DeleteAssertionSet 删除断言集
func (e *AssertionEngine) DeleteAssertionSet(id string) error {
	if e.store == nil {
		return fmt.Errorf("store not initialized")
	}
	return e.store.DeleteAssertionSet(id)
}

// GetAssertionSet 获取断言集
func (e *AssertionEngine) GetAssertionSet(id string) (*AssertionSet, error) {
	if e.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return e.store.GetAssertionSet(id)
}

// ListAssertionSets 列出所有断言集
func (e *AssertionEngine) ListAssertionSets() ([]AssertionSet, error) {
	if e.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return e.store.ListAssertionSets()
}

// SaveAssertionSetResult 保存断言集执行结果
func (e *AssertionEngine) SaveAssertionSetResult(result *AssertionSetResult) error {
	if e.store == nil {
		return fmt.Errorf("store not initialized")
	}
	return e.store.SaveAssertionSetResult(result)
}

// ListAssertionSetResults 获取断言集执行历史
func (e *AssertionEngine) ListAssertionSetResults(setID string, limit int) ([]AssertionSetResult, error) {
	if e.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return e.store.ListAssertionSetResults(setID, limit)
}

// GetAssertionSetResult 获取单个执行结果
func (e *AssertionEngine) GetAssertionSetResult(executionID string) (*AssertionSetResult, error) {
	if e.store == nil {
		return nil, fmt.Errorf("store not initialized")
	}
	return e.store.GetAssertionSetResult(executionID)
}

// ========================================
// API 方法
// ========================================

// CreateAssertion 创建断言并持久化
func (e *AssertionEngine) CreateAssertion(assertion *Assertion, saveAsTemplate bool) error {
	if assertion.ID == "" {
		assertion.ID = uuid.New().String()
	}
	now := time.Now().UnixMilli()
	if assertion.CreatedAt == 0 {
		assertion.CreatedAt = now
	}

	if e.store == nil {
		return nil
	}

	// 序列化 criteria 和 expected
	criteriaJSON, err := json.Marshal(assertion.Criteria)
	if err != nil {
		return fmt.Errorf("marshal criteria: %w", err)
	}
	expectedJSON, err := json.Marshal(assertion.Expected)
	if err != nil {
		return fmt.Errorf("marshal expected: %w", err)
	}
	metadataJSON, err := json.Marshal(assertion.Metadata)
	if err != nil {
		LogWarn("event_assertion").Err(err).Str("assertionId", assertion.ID).Msg("Failed to marshal metadata")
		metadataJSON = []byte("{}")
	}

	stored := &StoredAssertion{
		ID:          assertion.ID,
		Name:        assertion.Name,
		Description: assertion.Description,
		Type:        string(assertion.Type),
		SessionID:   assertion.SessionID,
		DeviceID:    assertion.DeviceID,
		Criteria:    criteriaJSON,
		Expected:    expectedJSON,
		Timeout:     assertion.Timeout,
		Tags:        assertion.Tags,
		Metadata:    metadataJSON,
		IsTemplate:  saveAsTemplate,
		CreatedAt:   assertion.CreatedAt,
		UpdatedAt:   now,
	}

	if assertion.TimeRange != nil {
		stored.TimeRange = &struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		}{Start: assertion.TimeRange.Start, End: assertion.TimeRange.End}
	}

	return e.store.CreateAssertion(stored)
}

// GetStoredAssertion 获取已存储的断言
func (e *AssertionEngine) GetStoredAssertion(id string) (*StoredAssertion, error) {
	if e.store == nil {
		return nil, nil
	}
	return e.store.GetAssertion(id)
}

// UpdateAssertion 更新已存储的断言
func (e *AssertionEngine) UpdateAssertion(assertion *Assertion) error {
	if e.store == nil {
		return nil
	}

	now := time.Now().UnixMilli()

	// 序列化 criteria 和 expected
	criteriaJSON, err := json.Marshal(assertion.Criteria)
	if err != nil {
		return fmt.Errorf("marshal criteria: %w", err)
	}
	expectedJSON, err := json.Marshal(assertion.Expected)
	if err != nil {
		return fmt.Errorf("marshal expected: %w", err)
	}
	metadataJSON, err := json.Marshal(assertion.Metadata)
	if err != nil {
		LogWarn("event_assertion").Err(err).Str("assertionId", assertion.ID).Msg("Failed to marshal metadata in update")
		metadataJSON = []byte("{}")
	}

	stored := &StoredAssertion{
		ID:          assertion.ID,
		Name:        assertion.Name,
		Description: assertion.Description,
		Type:        string(assertion.Type),
		SessionID:   assertion.SessionID,
		DeviceID:    assertion.DeviceID,
		Criteria:    criteriaJSON,
		Expected:    expectedJSON,
		Timeout:     assertion.Timeout,
		Tags:        assertion.Tags,
		Metadata:    metadataJSON,
		CreatedAt:   assertion.CreatedAt,
		UpdatedAt:   now,
	}

	if assertion.TimeRange != nil {
		stored.TimeRange = &struct {
			Start int64 `json:"start"`
			End   int64 `json:"end"`
		}{Start: assertion.TimeRange.Start, End: assertion.TimeRange.End}
	}

	return e.store.UpdateAssertion(stored)
}

// ListStoredAssertions 列出已存储的断言
func (e *AssertionEngine) ListStoredAssertions(sessionID, deviceID string, templatesOnly bool, limit int) ([]StoredAssertion, error) {
	if e.store == nil {
		return nil, nil
	}
	return e.store.ListAssertions(sessionID, deviceID, templatesOnly, limit)
}

// DeleteStoredAssertion 删除已存储的断言
func (e *AssertionEngine) DeleteStoredAssertion(id string) error {
	if e.store == nil {
		return nil
	}
	return e.store.DeleteAssertion(id)
}

// ListStoredResults 从数据库列出断言结果
func (e *AssertionEngine) ListStoredResults(sessionID string, limit int) ([]StoredAssertionResult, error) {
	if e.store == nil {
		return nil, nil
	}
	return e.store.ListAssertionResults(sessionID, "", limit)
}

// GetResult 获取断言结果
func (e *AssertionEngine) GetResult(resultID string) *AssertionResult {
	e.resultsMu.RLock()
	defer e.resultsMu.RUnlock()
	return e.results[resultID]
}

// ListResults 列出断言结果
func (e *AssertionEngine) ListResults(sessionID string, limit int) []*AssertionResult {
	e.resultsMu.RLock()
	defer e.resultsMu.RUnlock()

	results := make([]*AssertionResult, 0)
	for _, r := range e.results {
		if sessionID == "" || r.SessionID == sessionID {
			results = append(results, r)
		}
	}

	// 按时间倒序
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].ExecutedAt > results[i].ExecutedAt {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}
