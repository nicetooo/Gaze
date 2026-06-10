# Gaze (adbGUI) - Claude Code 项目指南

## 项目概述

Gaze 是一个跨平台的 Android 设备管理工具，使用 **Go + React** 构建，基于 **Wails** 框架。核心理念是通过统一的 **Session-Event** 架构，将设备的所有活动（日志、网络、触摸、应用状态等）关联到时间线上，实现完整的设备行为追踪和自动化测试。

## 最关键事项：Session 统一贯穿设备事件

### 核心架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Session (会话)                            │
│  - 每个设备在任意时刻只有一个活跃 Session                          │
│  - Session 是所有事件的容器和时间基准                              │
│  - 所有模块产生的事件都必须关联到 Session                          │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     EventPipeline (事件管道)                      │
│  - 统一的事件入口：所有事件通过 Emit() 进入管道                     │
│  - 自动关联 Session：根据 DeviceID 查找活跃 Session               │
│  - 计算 RelativeTime：事件时间相对于 Session 开始的偏移            │
│  - 背压控制：防止事件洪泛，保护关键事件                             │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     EventStore (事件存储)                         │
│  - SQLite 持久化：支持大量事件的高效存储和查询                      │
│  - 时间索引：按秒聚合，支持快速跳转                                 │
│  - 全文搜索：FTS5 支持事件内容搜索                                 │
└─────────────────────────────────────────────────────────────────┘
```

### 事件来源 (EventSource)

所有设备活动都统一为事件，通过 `Source` 字段区分来源：

| Source      | 描述         | 典型事件类型                                        |
| ----------- | ------------ | --------------------------------------------------- |
| `logcat`    | 设备日志     | logcat, logcat_aggregated                           |
| `network`   | 网络请求     | network_request, websocket_message                  |
| `device`    | 设备状态     | battery_change, network_change, screen_change       |
| `app`       | 应用生命周期 | app_start, app_stop, app_crash, app_anr             |
| `touch`     | 触摸事件     | touch, gesture                                      |
| `workflow`  | 自动化流程   | workflow*start, workflow_step*\*, workflow_complete |
| `ui`        | UI 状态      | element_found, element_click                        |
| `perf`      | 性能指标     | perf_sample                                         |
| `assertion` | 断言结果     | assertion_result                                    |
| `system`    | 系统事件     | session_start, session_end, recording_start         |

### 关键代码路径

1. **Session 创建**: `event_pipeline.go:StartSession()` / `getOrCreateSession()`
2. **事件发送**: `event_pipeline.go:Emit()` / `EmitRaw()`
3. **事件处理**: `event_pipeline.go:processEvent()` - 关联 Session、计算相对时间、写入存储
4. **事件存储**: `event_store.go:WriteEvent()` / `QueryEvents()`
5. **前端同步**: `event_pipeline.go:frontendEmitter()` - 批量推送到前端

### 发送事件的正确方式

```go
// 方式 1: 使用 EmitRaw (推荐，自动填充字段)
a.eventPipeline.EmitRaw(
    deviceID,           // 设备 ID
    SourceNetwork,      // 事件来源
    "network_request",  // 事件类型
    LevelInfo,          // 事件级别
    "GET /api/users",   // 标题
    networkRequestData, // 详细数据 (会被 JSON 序列化)
)

// 方式 2: 构造完整事件
a.eventPipeline.Emit(UnifiedEvent{
    ID:        uuid.New().String(),
    DeviceID:  deviceID,
    Timestamp: time.Now().UnixMilli(),
    Source:    SourceLogcat,
    Category:  CategoryLog,
    Type:      "logcat",
    Level:     LevelInfo,
    Title:     "ActivityManager: Starting activity",
    Data:      json.RawMessage(`{"tag":"ActivityManager","message":"..."}`),
})
```

### 添加新事件类型的步骤

1. **定义事件类型** (`event_types.go`):

   ```go
   // 在 EventRegistry 中注册
   "my_new_event": {
       Type: "my_new_event",
       Source: SourceApp,
       Category: CategoryState,
       Description: "My new event type",
   },

   // 如需要，定义数据结构
   type MyNewEventData struct {
       Field1 string `json:"field1"`
       Field2 int    `json:"field2"`
   }
   ```

2. **发送事件** (在相应模块中):

   ```go
   a.eventPipeline.EmitRaw(deviceID, SourceApp, "my_new_event", LevelInfo, "标题", MyNewEventData{...})
   ```

3. **前端处理** (`frontend/src/stores/eventStore.ts`):
   ```typescript
   // 事件会自动通过 "session-events-batch" 推送到前端
   // 在 eventStore 中按需添加处理逻辑
   ```

## 技术栈

### 后端 (Go)

- **Wails v2**: 桌面应用框架
- **SQLite**: 事件存储 (WAL 模式)
- **goproxy**: HTTP/HTTPS 代理

### 前端 (React/TypeScript)

- **React 18** + **Ant Design**: UI
- **Zustand**: 状态管理
- **XYFlow**: 工作流可视化
- **i18next**: 国际化

## 项目结构

```
adbGUI/
├── 核心后端文件
│   ├── main.go              # 应用入口
│   ├── app.go               # App 结构和初始化
│   ├── event_types.go       # ⭐ 事件类型定义
│   ├── event_store.go       # ⭐ SQLite 事件存储
│   ├── event_pipeline.go    # ⭐ 事件处理管道 (唯一的 Session 管理入口)
│   ├── event_assertion.go   # 断言引擎
│   ├── device.go            # 设备管理
│   ├── device_monitor.go    # 设备状态监控
│   ├── automation.go        # 自动化引擎
│   ├── proxy_bridge.go      # 代理事件桥接
│   ├── workflow.go          # 工作流执行
│   └── mcp_bridge.go        # MCP 桥接层
│
├── mcp/                     # ⭐ MCP 服务器模块
│   ├── server.go            # MCP 服务器入口和 GazeApp 接口
│   ├── tools_device.go      # 设备管理工具 (adb_execute, aapt_execute, ffmpeg_execute, ffprobe_execute)
│   ├── tools_apps.go        # 应用管理工具
│   ├── tools_automation.go  # UI 自动化工具
│   ├── tools_screen.go      # 屏幕控制工具
│   ├── tools_session.go     # 会话管理工具
│   ├── tools_workflow.go    # 工作流工具
│   ├── tools_proxy.go       # 代理工具
│   ├── tools_video.go       # 视频处理工具
│   └── resources.go         # MCP 资源定义
│
├── frontend/src/
│   ├── stores/
│   │   ├── eventStore.ts    # ⭐ 前端事件状态
│   │   ├── eventTypes.ts    # ⭐ 事件类型定义 + 工具函数 (formatDuration 等)
│   │   └── ...              # 其他状态管理
│   ├── components/
│   │   ├── EventTimeline.tsx  # 事件时间线组件
│   │   ├── SessionManager.tsx
│   │   ├── MCPInfoSection.tsx # ⭐ MCP 集成文档（主页展示）
│   │   └── ...
│   └── locales/             # 多语言 (en/zh/ja/ko/zh-TW)
│
└── proxy/                   # HTTP/HTTPS 代理模块
    ├── proxy.go
    └── cert.go
```

## 常用命令

```bash
# 开发模式（推荐使用 Makefile）
make dev
# 或直接使用 wails
wails dev -tags fts5

# 构建
make build
# 或直接使用 wails
wails build -tags fts5

# 运行测试
make test
# 或直接使用 go
go test -tags fts5 ./...
```

**⚠️ 重要**：必须使用 `-tags fts5` 启用 SQLite FTS5 全文搜索，否则搜索功能会降级为 LIKE 查询。

## 开发注意事项

### ⭐⭐ 文档记录规范：所有过程文档必须持久化到 md 文件

**这是最高优先级的开发纪律。** 所有的规划、方案、Bug 记录、测试记录、决策日志等过程文档，都必须记录在项目根目录下的 markdown 文件中，并在上下文中始终引用该文件，确保在 context compact（上下文压缩）时不会丢失对这些文件的引用。

#### 核心原则

1. **所有过程文档必须写入 md 文件**: 包括但不限于：
   - 功能规划与设计方案 → `docs/plans/`
   - Bug 记录与修复日志 → `docs/bugs/`
   - 测试记录与结果 → `docs/tests/`
   - 技术决策记录 → `docs/decisions/`
   - 开发进度追踪 → `docs/progress/`
   - 任何需要跨对话保留的上下文信息

2. **上下文中必须引用文档文件**: 每次创建或更新文档后，必须在回复中明确引用文件路径（如 `docs/bugs/2026-01-30-proxy-crash.md`），确保该路径出现在对话上下文中

3. **禁止纯对话记录**: 不允许将重要的规划、Bug 分析、方案讨论仅以对话形式存在。如果内容有跨轮次引用价值，必须写入文件

4. **Compact 安全**: 在 context compact 发生时，文件路径引用会被保留在压缩后的摘要中。因此：
   - 创建文件后，必须在回复中说明 "已记录到 `docs/xxx/yyy.md`"
   - 后续引用时，必须先 Read 该文件获取最新内容，而非依赖记忆
   - 关键文档路径应在 Todo List 中也有记录，作为双重保障

5. **⭐ 完成即更新：Bug 修复和方案实现后必须立即更新文档状态**
   - Bug 修复后，**立即**将对应 `docs/bugs/xxx.md` 的状态从 "分析中" 更新为 "已修复"，并补充修复方案和验证结果
   - 功能方案中的实现步骤完成一项，**立即**在 `docs/plans/xxx.md` 中勾选或标记为已完成
   - 禁止"事后批量更新"——每完成一个小步骤就更新一次文档，确保文档始终反映最新进度
   - 如果验证通过，将状态进一步更新为 "已验证"
   - **所有状态变更必须带时间戳**，格式：`[YYYY-MM-DD HH:mm] 状态变更说明`

   **⚠️ 这是一条阻断性规则：完成任何一个步骤后，必须先更新对应文档，才能开始下一个步骤。**
   **执行顺序：修改代码 → 立即 Edit 更新文档状态 → 才能继续下一项工作。不允许连续完成多个步骤后再回头更新。**

   触发更新的时机（每一个都必须立即触发文档 Edit）：
   - 修复了 1 个 Bug → **立即** Edit `docs/bugs/xxx.md` 状态改为"已修复"，写入修复方案
   - 方案中完成了 1 个步骤 → **立即** Edit `docs/plans/xxx.md` 勾选该步骤
   - 验证通过 1 项 → **立即** Edit 文档状态改为"已验证"
   - 发现了新问题 → **立即** Edit 文档补充发现
   - 修改了方案 → **立即** Edit 文档更新方案内容

   ```
   ❌ 错误: 修复了 3 个 Bug，全部改完后才去更新文档，中途 compact 导致忘记更新
   ✅ 正确: 每修复 1 个 Bug，立即更新该 Bug 文档的状态、修复方案、验证结果

   ❌ 错误: 方案中有 5 个步骤，全部做完才更新文档，中途 compact 后不知道做到哪了
   ✅ 正确: 每完成 1 个步骤，立即在方案文档中标记完成，记录实际改动

   ❌ 错误: Bug 已修复但文档状态仍然是"发现"，下次读取文档时以为还没处理
   ✅ 正确: 修复 → 更新为"已修复" → 验证通过 → 更新为"已验证"，每步都即时更新
   ```

6. **⭐ 文档禁止提交 Git，禁止自行删除**
   - `docs/` 目录已加入 `.gitignore`，**严禁**将任何过程文档提交到 Git 仓库
   - 文档仅在本地保留，供开发过程中持续引用
   - **禁止自行删除文档文件**：即使任务全部完成、Bug 全部验证通过，也不得主动删除 `docs/` 下的任何文件
   - 只有**用户明确要求删除**时，才可以执行删除操作
   - 理由：用户可能需要回溯历史记录、复查修复方案、或在未来会话中参考旧文档

   ```
   ❌ 错误: Bug 已验证通过，觉得文档没用了，主动删除 docs/bugs/xxx.md
   ✅ 正确: Bug 验证通过后更新状态为"已验证"，文件保留，等待用户决定是否清理

   ❌ 错误: 功能开发完成，主动 git add docs/ 提交文档到仓库
   ✅ 正确: docs/ 已在 .gitignore 中，永远不会被提交，仅本地保留

   ❌ 错误: 开始新任务前，主动清理旧的 docs/sessions/ 文件腾空间
   ✅ 正确: 旧文档保留不动，新任务创建新文件，用户说"清理"时才删除
   ```

#### 文档目录结构

```
docs/
├── plans/          # 功能规划、设计方案、架构讨论
├── bugs/           # Bug 记录、复现步骤、修复方案
├── tests/          # 测试计划、测试结果、回归记录
├── decisions/      # 技术决策记录 (ADR)
├── progress/       # 开发进度、里程碑追踪
└── sessions/       # 每次开发会话的工作记录
```

> **注意**: `docs/` 已加入 `.gitignore`，不会被 Git 追踪。文件仅存在于本地，用户要求删除时才可清理。

#### 文档文件命名规范

```
# 日期前缀 + 简短描述
docs/bugs/2026-01-30-proxy-websocket-crash.md
docs/plans/2026-01-30-workflow-v2-design.md
docs/tests/2026-01-30-drag-install-regression.md
docs/sessions/2026-01-30-session-log.md
```

#### 文档模板

**Bug 记录**:
```markdown
# Bug: [简短描述]
- **日期**: YYYY-MM-DD
- **状态**: 发现 / 分析中 / 已修复 / 已验证
- **严重程度**: P0/P1/P2/P3

## 现象
[描述 Bug 表现]

## 复现步骤
1. ...

## 根因分析
[分析过程和结论]

## 修复方案
[代码改动说明]

## 验证结果
[修复后的测试结果]
```

**功能规划**:
```markdown
# 功能: [功能名称]
- **日期**: YYYY-MM-DD
- **状态**: 规划 / 设计中 / 开发中 / 已完成

## 需求背景
[为什么做这个功能]

## 设计方案
[技术方案详述]

## 实现步骤
1. ...

## 影响范围
[涉及的文件和模块]
```

#### 操作流程

```
1. 用户提出需求/报告 Bug
   ↓
2. 创建对应的 md 文件，记录初始信息
   ↓
3. 在回复中明确引用: "已记录到 docs/xxx/yyy.md"
   ↓
4. 开发过程中持续更新该文件（进度、发现、决策）
   ↓
5. 每次引用时先 Read 文件获取最新内容
   ↓
6. 完成后更新文件状态为"已完成"/"已验证"
```

#### 典型错误 vs 正确做法

```
❌ 错误: 用户报告了一个 Bug，只在对话中分析，compact 后分析过程全部丢失
✅ 正确: 立即创建 docs/bugs/xxx.md，记录现象和分析，后续持续更新

❌ 错误: 讨论了一个复杂的设计方案，结论只存在于对话上下文中
✅ 正确: 创建 docs/plans/xxx.md，把方案写入文件，对话中引用文件路径

❌ 错误: 做了一系列测试，结果只用文字回复给用户
✅ 正确: 创建 docs/tests/xxx.md，记录测试步骤和结果，回复中引用文件

❌ 错误: compact 后忘记了之前的工作进度，重复做已完成的工作
✅ 正确: 每次开始工作先 Read docs/sessions/ 或 docs/progress/ 中的文件，恢复上下文
```

### ⭐ 开发测试规范：必须在浏览器中验证，禁止盲改

**这是最重要的开发纪律。** 所有前端变更都必须通过浏览器实际验证，严禁凭猜测修改代码。

#### ⚠️ 前端测试与 MCP 测试严格分离

**前端功能 → 只用 Playwright 验证，禁止用 MCP 工具测试前端。**
**MCP 功能 → 只用 MCP 工具验证，禁止用 Playwright/浏览器测试 MCP。**

两套测试工具链完全隔离，不得混用：

| 改动类型 | 验证方式 | 禁止使用 |
|---------|---------|---------|
| 前端 UI / 组件 / 样式 / 交互 | Playwright (snapshot/screenshot/click) | MCP 工具 (gaze_*) |
| MCP 工具 / 后端接口 / 设备操作 | MCP 工具 (gaze_*) 直接调用 | Playwright 浏览器 |

```
❌ 错误: 修改了前端组件，用 MCP gaze_screen_screenshot 去设备上看效果
✅ 正确: 修改了前端组件，用 Playwright browser_snapshot 在浏览器中验证

❌ 错误: 修改了 MCP 工具逻辑，打开浏览器去 UI 上点击测试
✅ 正确: 修改了 MCP 工具逻辑，直接调用对应的 MCP 工具函数验证返回结果

❌ 错误: 测试代理功能，一会儿用 Playwright 点 UI，一会儿用 MCP 调接口，混在一起
✅ 正确: 前端代理页面的 UI 渲染 → Playwright；MCP proxy_* 工具的接口行为 → MCP 调用
```

#### 核心原则

1. **先看再改**: 修改任何 UI 之前，必须先在浏览器中打开应用，用 Playwright snapshot/screenshot 观察当前实际状态
2. **改后必验**: 每次修改后，必须在浏览器中重新验证效果，确认改动符合预期
3. **禁止盲改**: 不允许在没有打开浏览器查看的情况下，仅凭代码阅读就进行 UI 修改。代码逻辑正确不代表渲染结果正确
4. **报错必查**: 遇到问题时，先用 Playwright 截图/snapshot 查看实际界面状态和控制台报错，再决定修复方案

#### 开发测试流程

**第 1 步：启动开发服务器**

使用 Bash 工具在后台启动 Wails 开发服务器：

```bash
# ⚠️ 启动前必须先关闭已有的 wails dev 进程，否则端口冲突导致启动失败
# 步骤 1: 杀掉所有已有的 wails dev 相关进程
pkill -f "wails dev" 2>/dev/null; lsof -ti:34115 | xargs kill -9 2>/dev/null; sleep 1

# 步骤 2: 确认端口已释放后再启动
wails dev &
```

> **⭐ 重要**: 每次需要重启 `wails dev` 之前，**必须先关闭所有正在运行的 dev 服务**。
> 直接执行 `wails dev` 而不先清理旧进程，会导致端口 34115 被占用、新进程启动失败或前后端不同步等问题。
> **正确流程**: `pkill` 杀进程 → 等待 1 秒 → 确认端口释放 → 再启动新的 `wails dev`。

启动后会同时运行：
- Go 后端服务（Wails 运行时、API、代理等）
- 前端 Vite dev server，监听在 **http://localhost:34115**

> **注意**: 首次启动需要等待 Go 编译和 Vite 构建完成，可能需要 10-30 秒。
> 如果端口 34115 被占用，检查是否已有 `wails dev` 进程在运行。

**第 2 步：用 Playwright 打开浏览器**

使用 Playwright `browser_navigate` 工具打开应用页面：

```
→ browser_navigate 到 http://localhost:34115
```

页面加载后，立即用 snapshot 或 screenshot 确认页面状态：

```
→ browser_snapshot    （查看页面完整 DOM 结构，推荐首选）
→ browser_take_screenshot  （查看视觉渲染效果）
```

**第 3 步：修改代码**

修改前端代码后，Vite HMR 会自动热更新，无需刷新页面。
修改 Go 后端代码后，Wails 会自动重新编译并重启。

**第 4 步：再次用 Playwright 验证修改效果**

```
→ browser_snapshot / browser_take_screenshot 确认改动生效且无异常
→ browser_console_messages (level: "error") 检查是否有新增报错
```

**完整操作序列示例**:
```
1. Bash: wails dev &                           ← 启动开发服务器
2. 等待启动完成（约 10-30 秒）
3. browser_navigate: http://localhost:34115     ← 打开应用
4. browser_snapshot                             ← 查看当前状态
5. 用 Edit 工具修改代码                          ← HMR 自动生效
6. browser_snapshot                             ← 验证修改结果
7. browser_console_messages (level: "error")    ← 检查报错
```

#### Playwright 常用验证操作

```
# 查看页面完整结构（推荐，比截图更快更准确）
→ browser_snapshot

# 截图查看视觉效果
→ browser_take_screenshot

# 检查控制台错误
→ browser_console_messages (level: "error")

# 交互测试
→ browser_click / browser_type / browser_select_option

# 等待异步操作完成
→ browser_wait_for (text: "某个预期出现的文本")
```

#### 需要设备连接时：请求用户协助

**唯一允许请求用户协助的场景：需要真实 Android 设备连接。** 设备连接无法通过 Playwright 模拟，必须由用户物理操作。

当调试的页面依赖已连接的设备（如设备详情、应用列表、文件管理、屏幕镜像等），应主动向用户提问：

```
例: "我需要验证设备详情页的渲染效果，请帮我连接一台 Android 设备，连接后告诉我"
```

用户确认设备已连接后，用 Playwright snapshot/screenshot 接手调试。

**除此之外的所有操作（页面导航、打开弹窗、启动代理、创建规则、填写表单等）都必须通过 Playwright 自主完成，禁止要求用户代劳。**

#### 典型错误示例

```
❌ 错误: 看到代码里用了 useState，直接全部改成 zustand store，不验证页面是否正常
✅ 正确: 先 snapshot 看当前页面状态 → 修改代码 → 再 snapshot 确认功能正常

❌ 错误: 觉得某个样式"应该"有问题，直接改 CSS
✅ 正确: 先 screenshot 看实际渲染效果 → 确认确实有问题 → 修改 → 再 screenshot 验证

❌ 错误: 编译报错后，根据错误信息猜测原因，连续改多个文件
✅ 正确: 先看编译错误 → 定位根因 → 最小化修改 → 编译通过 → 浏览器验证功能正常

❌ 错误: 需要验证设备连接后的页面效果，但没有设备，就直接跳过验证改代码
✅ 正确: 询问用户 "请帮我连接设备" → 用户就位后 snapshot 接手调试

❌ 错误: 需要打开代理页面或 Mock 编辑弹窗，要求用户帮忙点击导航
✅ 正确: 自己用 Playwright browser_click 点击导航菜单和按钮，自主到达目标页面
```

### 前端状态管理规范

**所有应用状态必须在 Zustand Store 中管理**，禁止在组件中使用独立的 `useState` 管理业务状态。

```
frontend/src/stores/
├── eventStore.ts       # 事件和时间线状态
├── eventTypes.ts       # 事件类型定义 + 格式化工具函数
├── deviceStore.ts      # 设备列表和选中状态
├── workflowStore.ts    # 工作流编辑和执行状态
├── automationStore.ts  # 触摸录制和任务状态
├── elementStore.ts     # UI 元素类型 (UINode, ElementSelector 的唯一定义)
├── proxyStore.ts       # 代理和网络请求状态
├── uiStore.ts          # 全局 UI 状态（导航、模态框）
└── ...                 # 其他领域状态
```

**规范要点**:

1. **禁止使用 `useState`**: 组件中不允许使用 `useState` 管理任何状态
2. **所有状态 → Store**: 包括 UI 状态、表单状态、临时状态都必须放在对应的 Store
3. **后端事件 → Store**: 所有 Wails 事件订阅的数据更新必须写入 Store
4. **新增状态**: 如需新状态，在现有 Store 中添加或创建新的 Store 文件

**正确示例**:
```typescript
// ✅ 在 Store 中管理业务状态
const useDeviceStore = create<DeviceState>((set) => ({
  devices: [],
  selectedDevice: null,
  setSelectedDevice: (device) => set({ selectedDevice: device }),
}));

// ✅ 组件中使用 Store
function DeviceList() {
  const { devices, selectedDevice, setSelectedDevice } = useDeviceStore();
  return <List data={devices} onSelect={setSelectedDevice} />;
}
```

**错误示例**:
```typescript
// ❌ 在组件中用 useState 管理业务状态
function DeviceList() {
  const [devices, setDevices] = useState([]);  // 错误！应该放在 Store
  const [selected, setSelected] = useState(null);  // 错误！应该放在 Store
}
```

### Session 生命周期

1. **自动创建**: 当设备产生事件但无活跃 Session 时，自动创建 "auto" 类型 Session
2. **手动创建**: 用户开始录制/工作流时，创建带配置的 Session
3. **结束 Session**: 调用 `EndSession(sessionID, status)` 或新 Session 替换旧 Session
4. **不会自动重建**: 用户结束 Session 后，`GetDevices()` 轮询不会自动重建 Session

### 后端架构要点

- **单一 Session 系统**: 所有 Session 管理通过 `EventPipeline` 进行
- **初始化/关闭共享逻辑**: `app.go` 中 `initCore()` / `shutdownCore()` 是 GUI 模式和 MCP 模式共用的核心初始化/清理逻辑。`startup()` 和 `Shutdown()` 供 Wails GUI 调用；`InitializeWithoutGUI()` 和 `ShutdownWithoutGUI()` 供 MCP 模式调用，两者都委托到共享方法
- **URL 模式匹配**: `proxy.MatchPattern()` 是唯一的 URL 通配符匹配实现（`proxy/proxy.go`），`proxy_bridge.go` 通过导入使用，禁止创建本地副本
- **触摸事件去重**: `device_monitor.go` 的 `emitTouchEvent()` 内置了 `IsTouchRecordingActive()` 守卫，当 `automation.go` 正在进行触摸录制时自动跳过，防止同一个触摸产生两条事件

### 前端类型唯一来源

- **`UINode`**: 唯一定义在 `elementStore.ts`，其他文件（如 `automationStore.ts`）通过 `import type` 引入并 re-export
- **`ElementSelector`**: `elementStore.ts`（前端 UI 用）和 `types/workflow.ts`（匹配后端 Go 类型）各有一份，type union 不同，属于有意区分
- **`formatDuration` 系列**: 统一在 `eventTypes.ts` 中定义三个版本：
  - `formatDuration(ms)` — 紧凑格式（`381ms`, `5.0s`, `2m 30s`）
  - `formatDurationHMS(ms)` — 人类可读（`3s`, `2m 3s`, `1h 2m 3s`）
  - `formatDurationMMSS(seconds)` — 时钟格式（`02:30`）
  - 组件中禁止再创建本地 `formatDuration`，应从 `eventTypes.ts` 导入

### 事件级别 (EventLevel)

- `fatal`: 致命错误，永不丢弃
- `error`: 错误，永不丢弃
- `warn`: 警告
- `info`: 普通信息
- `debug`: 调试信息，高负载时可能被采样
- `verbose`: 详细日志，高负载时可能被丢弃

### 背压控制

`BackpressureController` 在高负载时自动：

1. 保护关键事件 (error/fatal/network/workflow)
2. 采样 verbose/debug 级别事件
3. 防止事件队列溢出

### 数据库 Schema

核心表:

- `sessions`: 会话信息
- `events`: 事件主表 (不含大数据)
- `event_data`: 事件详细数据 (分离存储)
- `time_index`: 时间索引 (按秒聚合)
- `bookmarks`: 用户书签
- `assertions`: 断言定义
- `assertion_results`: 断言结果

## 调试技巧

优先使用go后段日志直接进行后台运行日志监控

### 查看事件流

```go
// 在 event_pipeline.go:processEvent() 添加日志
log.Printf("[Event] %s: %s - %s", event.Source, event.Type, event.Title)
```

### 检查 Session 状态

```go
// 获取设备的活跃 Session
session := a.eventPipeline.GetActiveSession(deviceID)
log.Printf("Active session: %v", session)
```

### 前端调试

```typescript
// 在浏览器控制台监听事件
window.runtime.EventsOn("session-events-batch", (events) => {
  console.log("Events received:", events);
});
```

## 扩展指南

### 添加新的事件源模块

1. 确定事件来源 (SourceXxx) 和事件类型
2. 在 `event_types.go` 注册事件类型
3. 在模块代码中通过 `eventPipeline.EmitRaw()` 发送事件
4. 前端在 `eventStore` 中处理新事件类型

### 添加新的断言类型

1. 在 `event_assertion.go` 的 `AssertionEvaluator` 中添加评估逻辑
2. 在前端 `AssertionsPanel.tsx` 添加 UI 配置

## 性能考虑

- 事件写入使用缓冲批量提交 (500ms / 500条)
- 前端同步使用批量推送 (500ms)
- 时间索引使用内存缓存 + 定期持久化 (5s)
- 大数据 (请求/响应体) 分离存储，列表查询不加载

## 拖拽安装功能 (APK/XAPK/AAB)

### 功能概述

应用管理页面支持通过拖拽文件直接安装 Android 应用包，支持三种格式：
- **APK**: 标准 Android 应用包，使用 `adb install -r`
- **XAPK**: 包含多个 split APK 和 OBB 文件的 ZIP 压缩包，使用 `adb install-multiple`
- **AAB**: Android App Bundle，需要 bundletool 转换后安装

### 关键文件和代码路径

**后端 (`apps.go`)**:
- `InstallAPK(deviceId, path)`: 安装标准 APK
- `InstallXAPK(deviceId, path)`: 解压并安装 XAPK（处理 split APKs + OBB 文件）
- `InstallAAB(deviceId, path)`: 使用 bundletool 安装 AAB
- `InstallPackage(deviceId, path)`: 统一入口，根据扩展名自动选择安装方法
- `findBundletool()`: 查找 bundletool 可执行文件

**前端 (`AppsView.tsx`)**:
- 使用 Wails 的 `OnFileDrop` API 监听文件拖放事件
- 容器需要 CSS 属性 `--wails-drop-target: "drop"` 才能接收拖放
- 拖拽状态通过 `appsStore` 管理 (`isDraggingOver`, `isInstalling`, `installingFileName`)
- 安装成功后延迟 1 秒刷新应用列表（等待 Android 注册新应用）

**状态管理 (`appsStore.ts`)**:
```typescript
// 拖拽安装相关状态
isDraggingOver: boolean;      // 是否有文件拖拽到区域上
isInstalling: boolean;        // 是否正在安装
installingFileName: string;   // 正在安装的文件名
```

### 实现要点

1. **Wails 文件拖放配置** (`main.go`):
   ```go
   DragAndDrop: &options.DragAndDrop{
       EnableFileDrop:     true,
       DisableWebViewDrop: true,  // 禁用 WebView 原生拖放，使用 Wails 事件
   },
   ```

2. **前端拖放目标** (`AppsView.tsx`):
   ```tsx
   <div style={{ "--wails-drop-target": "drop" } as React.CSSProperties}>
   ```

3. **XAPK 安装流程**:
   - 创建临时目录解压 XAPK (ZIP 格式)
   - 提取所有 `.apk` 文件，确保 base.apk 排在最前
   - 使用 `adb install-multiple -r -d` 安装
   - 如遇签名冲突，尝试先卸载再安装
   - 提取 OBB 文件并推送到 `/sdcard/Android/obb/<package>/`

4. **AAB 安装流程**:
   - 查找 bundletool（PATH 或 `~/Library/Application Support/Gaze/bin/bundletool.jar`）
   - 获取设备规格 `bundletool get-device-spec`
   - 构建优化 APKs `bundletool build-apks --device-spec=...`
   - 安装 `bundletool install-apks`

### 注意事项

- **签名冲突**: 系统预装应用（如 Google Play Services）无法被第三方签名的包覆盖安装
- **AAB 依赖**: 需要用户安装 bundletool，否则显示友好错误提示
- **刷新延迟**: 安装成功后需等待 1 秒再刷新列表，否则新应用可能不显示
- **多语言**: 相关翻译键在 `apps.` 命名空间下（如 `apps.drop_package_here`）

### 修改此功能时的检查清单

- [ ] 确保 `--wails-drop-target` CSS 属性存在于容器元素
- [ ] 确保 `OnFileDrop` 回调正确注册和清理
- [ ] 确保 `appsStore` 中的拖拽状态正确更新
- [ ] 确保安装成功后有延迟刷新逻辑
- [ ] 测试 APK、XAPK、AAB 三种格式
- [ ] 测试签名冲突场景的错误提示

## 文件管理拖拽上传功能

### 功能概述

文件管理页面支持通过拖拽文件直接上传到 Android 设备，支持以下特性：
- **拖拽上传**: 将本地文件拖拽到文件列表区域即可上传
- **批量上传**: 支持同时拖拽多个文件
- **权限显示**: 文件列表显示文件权限（mode）列，如 `drwxr-xr-x`
- **智能重定向**: 当在只读根目录 `/` 时，自动重定向到 `/sdcard` 并提示用户

### 关键文件和代码路径

**后端 (`files.go`)**:
- `UploadFile(deviceId, localPath, remotePath)`: 使用 `adb push` 上传文件到设备
- `ListFiles(deviceId, path)`: 列出目录内容，返回包含 `mode` 权限字段的文件列表

**前端 (`FilesView.tsx`)**:
- 使用 Wails 的 `OnFileDrop`/`OnFileDropOff` API 监听文件拖放事件
- 容器需要 CSS 属性 `--wails-drop-target: "drop"` 才能接收拖放
- 使用 `App.useApp()` 获取 `modal` 和 `message` 实例（确保主题适配）
- 拖拽状态通过 `filesStore` 管理
- 根目录上传时自动重定向到 `/sdcard` 并显示提示

**状态管理 (`filesStore.ts`)**:
```typescript
// 拖拽上传相关状态
isDraggingOver: boolean;      // 是否有文件拖拽到区域上
isUploading: boolean;         // 是否正在上传
uploadingFileName: string;    // 正在上传的文件名
uploadProgress: { current: number; total: number } | null;  // 上传进度
```

**MCP 工具 (`mcp/tools_device.go`)**:
- `file_upload`: 上传文件到设备
- `file_list`: 列出设备目录内容

### 实现要点

1. **拖放事件注册** (`FilesView.tsx`):
   ```tsx
   useEffect(() => {
     const handleFileDrop = async (_x: number, _y: number, paths: string[]) => {
       // 处理上传逻辑
     };
     OnFileDrop(handleFileDrop, true);
     return () => OnFileDropOff();
   }, [selectedDevice, currentPath, ...]);
   ```

2. **根目录重定向**:
   ```typescript
   // 根目录是只读的，自动重定向到 /sdcard
   const isRootRedirect = !currentPath || currentPath === "/";
   const uploadDir = isRootRedirect ? "/sdcard" : currentPath;
   if (isRootRedirect) {
     message.info(t("files.upload_redirect_sdcard"));
   }
   ```

3. **主题适配** (`ThemeContext.tsx` + `FilesView.tsx`):
   ```tsx
   // ThemeContext.tsx - 包裹 AntApp 组件
   <ConfigProvider theme={themeConfig}>
     <AntApp>
       {children}
     </AntApp>
   </ConfigProvider>
   
   // FilesView.tsx - 使用 App.useApp() 获取实例
   const { modal, message } = App.useApp();
   modal.confirm({ ... });  // 而不是 Modal.confirm
   ```

4. **权限列显示**:
   ```typescript
   {
     title: t("files.mode"),
     dataIndex: "mode",
     key: "mode",
     width: 110,
     render: (mode: string) => (
       <span style={{ fontFamily: "monospace", fontSize: 12 }}>{mode || "-"}</span>
     ),
   }
   ```

### 注意事项

- **根目录只读**: Android 设备的根目录 `/` 是只读的，必须上传到 `/sdcard` 等可写目录
- **闭包问题**: `OnFileDrop` 回调中使用的变量需要在 `useEffect` 依赖数组中正确声明
- **主题适配**: `Modal.confirm` 等静态方法不会自动继承主题，必须使用 `App.useApp()` 获取实例
- **多语言**: 相关翻译键在 `files.` 命名空间下

### 修改此功能时的检查清单

- [ ] 确保 `--wails-drop-target` CSS 属性存在于 FilesView 容器元素
- [ ] 确保 `OnFileDrop` 回调正确注册和清理（检查 useEffect 依赖数组）
- [ ] 确保 `filesStore` 中的拖拽状态正确更新
- [ ] 确保根目录上传时有重定向提示
- [ ] 确保上传成功后正确刷新文件列表（如重定向则导航到目标目录）
- [ ] 确保使用 `App.useApp()` 获取 modal/message 实例（主题适配）
- [ ] 测试暗色/亮色主题下的弹窗显示
- [ ] 测试批量上传多个文件
- [ ] 确保 MCP 工具 `file_upload` 和 `file_list` 正常工作

## MCP (Model Context Protocol) 模块

### AI 架构原则

**重要**: 应用内不包含任何 AI 功能实现。所有 AI 能力必须通过 MCP 协议由外部 AI 客户端（如 Claude Desktop）提供。

```
正确的架构:
用户 → MCP 工具 → 外部 AI (Claude Desktop) → MCP 工具 → 设备操作

错误的架构 (已移除):
用户 → 应用内 AI → 设备操作  ❌
```

这意味着：
- **禁止** 在应用中集成 OpenAI/Anthropic 等 AI SDK
- **禁止** 创建 AI 相关的 Store（如 aiStore）
- **禁止** 实现 AI 驱动的自动化功能（如 AI 生成工作流、AI 崩溃分析）
- **允许** 通过 MCP 工具暴露数据和操作，让外部 AI 客户端调用

### ⭐ 新增功能必须同步暴露 MCP 工具

**所有新增的后端功能都必须通过 MCP 暴露给外部 AI 客户端。** 这是核心开发规范之一。

当你添加任何新功能时，必须同步完成以下工作：

1. **后端实现** → 在 `app.go` 或相关文件中实现功能方法
2. **MCP 接口暴露** → 在 `mcp/server.go` 的 `GazeApp` 接口添加方法签名
3. **MCP 桥接** → 在 `mcp_bridge.go` 中添加桥接方法
4. **MCP 工具注册** → 在 `mcp/tools_*.go` 中注册工具并编写详细描述
5. **前端 MCP 文档更新** → 更新 `MCPInfoSection.tsx` 组件中的工具列表和分类
6. **多语言翻译更新** → 在所有 locale 文件 (`en.json`, `zh.json`, `ja.json`, `ko.json`, `zh-TW.json`) 的 `mcp.tool.*` 下添加新工具的翻译

**不允许** 只在前端 UI 中添加功能而不暴露 MCP 工具。AI 客户端必须能通过 MCP 完成用户在 GUI 中能做的所有操作。

### MCP 工具文档位置

**重要**: MCP 工具的接口文档直接写在 Go 代码的 `mcp.WithDescription()` 中，这些描述会通过 MCP 协议传递给 AI 客户端。

```
mcp/
├── tools_device.go      # adb_execute, aapt_execute, ffmpeg_execute, ffprobe_execute
├── tools_apps.go        # app_list, app_info, app_start, app_stop, app_install...
├── tools_automation.go  # ui_hierarchy, ui_tap, ui_swipe, ui_input, ui_search...
├── tools_screen.go      # screen_screenshot, screen_record_start/stop...
├── tools_session.go     # session_create, session_end, session_events...
├── tools_workflow.go    # workflow_create, workflow_run, workflow_execute_step...
├── tools_proxy.go       # proxy_start, proxy_stop, proxy_status
├── tools_plugins.go     # ⭐ plugin_list, plugin_create, plugin_test (含完整 API 规范)
└── tools_video.go       # video_frame, video_metadata, session_video_frame...
```

**⭐ 插件开发文档**: `tools_plugins.go` 中的 `plugin_create` 工具包含了完整的插件 JavaScript API 规范：
- `plugin` 对象结构 (`metadata`, `onInit`, `onEvent`, `onDestroy`)
- `event` 对象字段定义（传递给 `onEvent` 的参数）
- `context` 对象 API（`config`, `state`, `log()`, `emit()`, `jsonPath()`, `matchURL()`）
- `PluginResult` 返回值格式
- 完整的可运行示例（API Tracking Validator）
- 调试工作流和错误处理指南（`plugin_test` 工具）

### 内置 CLI 工具

项目内嵌了以下命令行工具，全部通过 MCP 暴露：

| 工具 | MCP 工具名 | 用途 |
|------|-----------|------|
| adb | `adb_execute` | Android 调试桥，设备交互 |
| aapt | `aapt_execute` | APK 分析 (包信息、权限、资源) |
| ffmpeg | `ffmpeg_execute` | 视频/音频处理 |
| ffprobe | `ffprobe_execute` | 媒体文件分析 |
| scrcpy | 内部使用 | 屏幕镜像和录制 |

### 添加新 MCP 工具的步骤

1. **在 `mcp/server.go` 的 `GazeApp` 接口中添加方法签名**:
   ```go
   type GazeApp interface {
       // ...existing methods...
       MyNewMethod(param string) (string, error)
   }
   ```

2. **在主应用中实现方法** (如 `device.go`, `app.go` 等):
   ```go
   func (a *App) MyNewMethod(param string) (string, error) {
       // 实现逻辑
   }
   ```

3. **在 `mcp_bridge.go` 中添加桥接方法**:
   ```go
   func (b *MCPBridge) MyNewMethod(param string) (string, error) {
       return b.app.MyNewMethod(param)
   }
   ```

4. **在对应的 `mcp/tools_*.go` 中注册工具**:
   ```go
   s.server.AddTool(
       mcp.NewTool("my_new_tool",
           mcp.WithDescription(`详细的工具描述，包括：
   
   用途说明...
   
   常用命令/参数:
   - 命令1: 说明
   - 命令2: 说明
   
   示例:
     example1
     example2
   
   注意事项...`),
           mcp.WithString("param",
               mcp.Required(),
               mcp.Description("参数说明"),
           ),
       ),
       s.handleMyNewTool,
   )
   ```

5. **实现 handler 函数**:
   ```go
   func (s *MCPServer) handleMyNewTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
       args := request.GetArguments()
       param, _ := args["param"].(string)
       
       result, err := s.app.MyNewMethod(param)
       if err != nil {
           return &mcp.CallToolResult{
               Content: []mcp.Content{mcp.NewTextContent(fmt.Sprintf("Error: %v", err))},
               IsError: true,
           }, nil
       }
       
       return &mcp.CallToolResult{
           Content: []mcp.Content{mcp.NewTextContent(result)},
       }, nil
   }
   ```

6. **更新前端 MCP 文档组件** (`frontend/src/components/MCPInfoSection.tsx`):
   - 在 `TOOL_CATEGORIES` 数组对应分类的 `tools` 中添加新工具名
   - 如果是全新分类，在 `TOOL_CATEGORIES` 中新增分类项

7. **更新多语言翻译** (所有 locale 文件: `en.json`, `zh.json`, `ja.json`, `ko.json`, `zh-TW.json`):
   - 在 `mcp.tool.*` 下添加新工具的描述翻译
   - 如果新增了分类，同步添加 `mcp.category.*` 和 `mcp.category.*_desc` 翻译

### 前端 MCP 文档组件

应用主页（DevicesView）底部包含一个可折叠的 **MCP 集成文档区域**，由 `MCPInfoSection.tsx` 组件实现，内容包括：

- MCP 功能概述
- 各 AI 客户端配置方式（Claude Desktop / Claude Code / Cursor）
- 全部 MCP 工具列表（按分类组织，含描述）
- 可用资源列表

**关键文件**:
- 组件: `frontend/src/components/MCPInfoSection.tsx`
- 翻译: 各 locale 文件的 `mcp` 字段

每次新增/修改 MCP 工具后，**必须同步更新此组件和对应翻译**。

### MCP 工具描述规范

工具描述应该详细且结构化，参考 `tools_workflow.go` 和 `tools_device.go` 的风格：

- 简短的功能概述
- 分类列出常用命令/操作
- 提供具体的使用示例
- 说明参数格式和限制
- 添加必要的注意事项
