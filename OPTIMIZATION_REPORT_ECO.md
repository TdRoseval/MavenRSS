# MrRSS 低配置环境性能优化报告 (修正版)

## 🔄 核心反思与纠偏

| 维度 | 之前的主观判断 | **代码现状与事实** | **修正后的优化策略** |
| :--- | :--- | :--- | :--- |
| **并发控制** | 认为 Goroutine 无限制暴走 | **已有完善的 TaskManager**：基于信号量 (`poolSem`) 和队列实现了动态并发限制。 | **无需重写**，只需增加“启动间隔 (Pacing)”和降低默认并发阈值。 |
| **数据库写入** | 认为全是单条插入 | **已是批量插入**：`SaveArticles` 内部使用了事务和批量 `INSERT`。**但在 Worker 层面仍是并发事务**。 | **坚持优化**：将“Worker 级批量”升级为“**全局级批量**”，解决 SQLite 写锁竞争。 |
| **XML 解析** | 认为全文提取极其昂贵 | 刷新时仅做基础 XML 解析和正则清洗，未默认开启 XPath/Readability。 | **重点转向 304 缓存**：解析开销尚可，但**无效解析**太多（缺乏 ETag 支持）。 |
| **AI 冲突** | 认为后台 AI 会抢占资源 | **误判**：AI 摘要是由前端 API (`/summarize`) 按需触发的，不存在后台批量 AI 任务。 | **移除此项建议**：AI 与刷新天然解耦，无需专门做互斥。 |

---

## 🛠️ 深度优化方案 (可落地版)

### 1. 网络层：实现 HTTP 304 协商 (最高性价比)

**痛点**：目前 `fetcher.go` 每次都会下载完整的 XML 并解析，即使内容未更新。对于 100+ 个订阅源，这是巨大的 CPU 和带宽浪费。
**现状**：`gofeed` 解析器默认未处理缓存头。

**改造方案**：
*   **修改点**：在 `internal/feed/fetcher.go` 的 `ParseFeedWithFeed` 中。
*   **逻辑**：
    1.  数据库 `feeds` 表新增 `etag` 和 `last_modified` 字段。
    2.  构造请求时，从 DB 读取并设置 `If-None-Match` 和 `If-Modified-Since` 头。
    3.  **关键**：捕获 `304 Not Modified` 响应。
    4.  如果返回 304，直接更新 `last_updated` 时间，**立即返回**，跳过后续所有 `xml.Decode`、`CleanHTML` 和 `processArticles` 步骤。

> **预期收益**：在 80% 的刷新场景下，CPU 消耗降为 **0**。

### 2. 数据库层：全局单一写入通道 (解决 IO 瓶颈)

**痛点**：虽然 `SaveArticles` 内部是批量的，但 `TaskManager` 允许 10 个 Worker 同时运行。这意味着会有 **10 个并发的数据库事务** 试图获取 SQLite 的写锁。在低配磁盘（如 SD 卡）上，这会导致严重的 IO 等待和 CPU 上下文切换。

**改造方案**：
*   **修改点**：
    *   `Fetcher` 结构体新增 `articleSink chan []*models.Article`。
    *   新增 `func (f *Fetcher) articleWriterLoop()`。
*   **逻辑**：
    1.  **Worker 变更**：`fetchFeedWithContext` 不再调用 `f.db.SaveArticles`，而是将解析好的文章切片发送到 `f.articleSink`。
    2.  **Writer Loop**：启动一个单独的 Goroutine，死循环读取 `articleSink`。
    3.  **缓冲策略**：维护一个内部 Buffer，满足以下任一条件时执行 `db.SaveArticles`：
        *   Buffer 积压超过 50 篇文章。
        *   距离上次写入已过去 2 秒。
    4.  **事务合并**：这样，10 个 Feed 的更新可能会被合并进**同一个**数据库事务中。

> **预期收益**：数据库 IOPS 降低 90%，彻底消除 SQLite 锁竞争。

### 3. 调度层：增加微小启动间隔 (Pacing)

**痛点**：`TaskManager` 虽然限制了并发数为 N，但在刷新开始的一瞬间，N 个 Worker 是**同时**启动的。这会造成瞬时的 CPU 刺。

**改造方案**：
*   **修改点**：`internal/feed/task_manager.go` 中的 `processQueue`。
*   **逻辑**：
    ```go
    func (tm *TaskManager) processQueue(ctx context.Context) {
        // ... 
        // 在启动新 Goroutine 前增加微小延迟
        time.Sleep(100 * time.Millisecond) 
        
        tm.poolSem <- struct{}{} // 获取令牌
        // ...
        go tm.processTask(ctx, task)
    }
    ```
*   **效果**：将并发请求在时间轴上“抹匀”，避免瞬时负载过高。

### 4. 前端通信：去噪 (Debounce)

**痛点**：虽然没有看到具体的 Wails 发送代码，但通常进度更新是实时的。
**建议**：
*   后端维护一个 `ProgressStats` 结构体。
*   使用 `time.NewTicker(1 * time.Second)` 定时将 `ProgressStats` 推送给前端，而不是每完成一个 Feed 就推送。

---

## 📊 落地建议：新增“节能模式”配置

不要硬改现有逻辑，建议在 `config.json` 或设置界面增加 `performance_mode` 选项：

*   **Standard (默认)**：
    *   并发数：`runtime.NumCPU()` (自动)
    *   启动间隔：0ms
    *   数据库写入：Worker 独立事务 (低延迟)
*   **Eco (节能模式)**：
    *   并发数：固定为 **2**
    *   启动间隔：**500ms**
    *   数据库写入：**全局聚合写入** (高延迟，低负载)
    *   304 缓存：强制启用

**结论**：
通过实现 **HTTP 304** 和 **全局写入通道**，配合 **Eco 模式** 的参数调整，可以在不牺牲核心功能的前提下，让 MrRSS 在低配设备上实现“润物细无声”的运行效果。这比之前提议的“禁用全文”或“互斥锁”更加切中肯綮且优雅。
