## Admin Metrics Service 流程总览

后台指标服务的职责只有三件事：

1. **记录事件**：Prompt 生成 / 保存结束后丢一条“活动记录”进内存，并同步写入事件表持久化。
2. **定时汇总**：按配置的周期把近几天的数据整理成快照，同时把每日聚合写入日表。
3. **对外提供快照**：管理员接口一次性返回汇总结果，服务重启时还能从数据库恢复最近 N 天的历史。

下图描述了数据流向（从左到右）：

```
生成/保存请求
   └─(RecordActivity)─> 内存日桶
            └─(持久化)─> admin_metrics_events
   └─(定时器/手动刷新)─> 汇总快照
            └─(写入)─> admin_metrics_daily
                          ↓
                     服务重启时回放
```

---

## 0. 数据落库设计

| 表名 | 作用 | 更新时机 |
| ---- | ---- | -------- |
| `admin_metrics_events` | 按事件逐条存储 “生成/保存” 记录，字段包括用户、类型、成功标记、耗时、发生时间等 | `RecordActivity` 把事件写入内存后，调用 `persistEvent → repo.AppendEvent` 追加到表中；重启时 `hydrateFromRepository → repo.LoadEventsRange` 会把最近 `RetentionDays` 的记录回放到内存；刷新后会调用 `cleanExpiredEvents → repo.DeleteEventsBefore` 清理过期行 |
| `admin_metrics_daily` | 保存每日聚合后的统计结果（活跃用户、请求数、成功率、平均耗时、保存数） | `rebuildSnapshotLocked` 生成新快照后返回最新 `DailyMetric` 列表，由 `persistDailyMetrics → repo.UpsertDaily` 写入或更新 |

> 说明：事件表提供“重放”能力，避免后端重启后指标清零；日表存放给 BI / 运营使用的最终统计，便于独立查询或做更长周期报表。

### Repository 接口与调用顺序

`Service` 通过 `Repository` 抽象数据库操作，主要方法如下：

- `LoadEventsRange(ctx, start, end)`：`Start` 阶段调用，读取最近 `RetentionDays` 的事件并逐条执行 `applyEventLocked` 恢复内存桶。
- `AppendEvent(ctx, event)`：`RecordActivity` 在写完内存后调用，持久化当前事件。
- `UpsertDaily(ctx, record)`：`refreshSnapshot` 或 `Snapshot()` 触发快照重建后调用，把每日统计落库（`INSERT ... ON DUPLICATE KEY UPDATE`）。
- `DeleteEventsBefore(ctx, boundary)`：刷新完快照后清理窗口之外的旧事件，控制事件表体量。

---

## 1. 事件写入：`RecordActivity`

触发点：`prompt.Service.GeneratePrompt` / `Save` 在 `defer` 中调用。

执行逻辑：

1. **快速校验**  
   - `UserID == 0` → 直接丢弃。  
   - `Timestamp.IsZero()` → 填充 `time.Now()`。
2. **定位桶**  
   - `truncateToDay` 将时间截断到当天 00:00。  
   - 使用 `YYYY-MM-DD` 作为哈希 key。
3. **写入数据（持写锁）**  
   - `s.mu.Lock()` 确保串行写入。  
   - 当天桶不存在则创建并缓存。  
   - `activeUsers[userID] = struct{}{}` 用 map 做天然去重。  
   - 根据事件类型更新 `generateTotal / generateSuccess / latency / saveTotal`。  
   - `s.dirty = true` 标记快照已过期。  
   - `defer s.mu.Unlock()`。
4. **写入数据库（非阻塞）**  
   - 锁释放后调用 `persistEvent → repo.AppendEvent` 把事件追加到 `admin_metrics_events`。  
   - 写库失败会打印告警日志，但不影响内存缓存；下次事件仍然照常运行。

> 若同一时间有并发写入，后来的协程会在 `Lock()` 阻塞，等待前一次写入完成后继续执行，保证数据安全不会丢。

---

## 2. 周期汇总：`Start` + `refreshSnapshot`

- `Start(ctx)` 是在应用启动阶段调用的，只允许执行一次（`CompareAndSwap(false, true)`）。
- 启动流程：
  1. 立即执行一次 `refreshSnapshot(time.Now())`，让快照不为空。
  2. 创建 `time.NewTicker(cfg.RefreshInterval)`，在 goroutine 中循环：
     - `ctx.Done()` → 退出，打印停止日志。
     - `<-ticker.C` → 到点重算 `refreshSnapshot(time.Now())`。

`refreshSnapshot` 会在持锁重建快照后返回最新的 `DailyMetric` 列表，再异步调用：

- `persistDailyMetrics`：将最新结果 Upsert 到 `admin_metrics_daily`；
- `cleanExpiredEvents`：删除超过保留窗口的旧事件，避免事件表无限增长。

---

## 3. 生成快照：`rebuildSnapshotLocked`

这个函数就像“打报表”，把缓存里的原始事件整理成对外易读的统计数据。调用它之前已经持有写锁，所以可以放心修改内部状态。

### 步骤拆解（配合代码理解）

```go
func (s *Service) rebuildSnapshotLocked(now time.Time) {
    // 1. 计算保留窗口：例如保留 7 天，就把窗口起点设为今日往前推 6 天。
    windowStart := truncateToDay(now).AddDate(0, 0, -(s.cfg.RetentionDays - 1))

    // 2. 把窗口之前的桶都删掉，防止数据无限增长。
    for key, bucket := range s.buckets {
        if bucket.date.Before(windowStart) {
            delete(s.buckets, key)
        }
    }

    // 3. 把剩余的日期（key）做个副本并排序，这样生成的日报有稳定顺序。
    keys := make([]string, 0, len(s.buckets))
    for key := range s.buckets {
        keys = append(keys, key)
    }
    sort.Strings(keys)

    // 4. 为输出准备容器：daily 用来放每天的指标，totalUsers 用来统计整个区间的活跃用户数。
    daily := make([]DailyMetric, 0, len(keys))
    totalUsers := make(map[uint]struct{})
    totalGenerate, totalSuccess, totalSave := 0, 0, 0
    totalLatency := time.Duration(0)
    totalSamples := 0

    // 5. 逐日累加
    for _, key := range keys {
        bucket := s.buckets[key]

        // 5.1 把这一天的活跃用户合并到 totalUsers，map 会自动去重。
        for userID := range bucket.activeUsers {
            totalUsers[userID] = struct{}{}
        }

        // 5.2 计算当天的成功率和平均耗时。
        successRate := computeRate(bucket.generateSuccess, bucket.generateTotal)
        avgLatency := computeAverageMillis(bucket.latencySum, bucket.latencySamples)

        // 5.3 填写 DailyMetric（对外展示用的结构体）。
        daily = append(daily, DailyMetric{
            Date:                 key,
            ActiveUsers:          len(bucket.activeUsers),
            GenerateRequests:     bucket.generateTotal,
            GenerateSuccess:      bucket.generateSuccess,
            GenerateSuccessRate:  successRate,
            AverageLatencyMillis: avgLatency,
            SaveRequests:         bucket.saveTotal,
        })

        // 5.4 汇总区间统计，用于 totals。
        totalGenerate += bucket.generateTotal
        totalSuccess += bucket.generateSuccess
        totalSave += bucket.saveTotal
        totalLatency += bucket.latencySum
        totalSamples += bucket.latencySamples
    }

    // 6. 生成新的快照对象：包含每日数组 + 区间 totals。
    s.snapshot = Snapshot{
        RefreshedAt: now,
        RangeDays:   s.cfg.RetentionDays,
        Daily:       daily,
        Totals: Totals{
            ActiveUsers:          len(totalUsers),
            GenerateRequests:     totalGenerate,
            GenerateSuccess:      totalSuccess,
            GenerateSuccessRate:  computeRate(totalSuccess, totalGenerate),
            AverageLatencyMillis: computeAverageMillis(totalLatency, totalSamples),
            SaveRequests:         totalSave,
        },
    }

    // 7. 报表刷新完毕，把 dirty 标记复位，方便外部知道快照已最新。
    s.dirty = false
}
```

### 通俗类比

可以把 `rebuildSnapshotLocked` 想象成“每日运营报表生成器”：

1. 先把过期的旧报表扔掉（只保留最近 7 天）。
2. 把剩余的每日原始数据按日期排序。
3. 遍历每一天：
   - 数有多少不同的人来过（去重）。
   - 算当天的请求次数、成功率、平均耗时。
   - 把这些值填进一行日报。
   - 顺便把全周期的总数累积起来。
4. 输出最终报表对象（`Snapshot`），其中既有明细（`daily`），也有总览（`totals`）。
5. 最后把“需要刷新”标记清零，表示账本已整理完毕。

这样做的好处是，读接口只要直接返回 `Snapshot`，前端就能一次性获得每日折线图所需的数据以及顶部总览卡片的数字。`rebuildSnapshotLocked` 最后会把新的 `daily` 列表返回给调用者，供 `persistDailyMetrics` 写入日表。

辅助函数：

| 函数                   | 作用                              |
| ---------------------- | --------------------------------- |
| `cloneSnapshot`        | 深拷贝快照，避免外部修改内部数据 |
| `truncateToDay`        | 时间截断到自然日                 |
| `computeRate`          | 计算成功率，避免除零             |
| `computeAverageMillis` | 根据耗时总和求平均值（毫秒）     |

---

## 4. 读取快照：`Snapshot()`

对外 API 调用顺序：

1. `Snapshot()` 先加写锁（因为可能要刷新）。  
2. 若 `dirty == true` 或距离上次刷新已超过 `RefreshInterval`，调用 `rebuildSnapshotLocked` 并拿到最新 `daily`。  
3. 使用 `cloneSnapshot` 生成副本。  
4. 解锁后，如本次确实重建过快照，则调用 `persistDailyMetrics / cleanExpiredEvents`，保证即使是手动读取触发的刷新也会同步到数据库。  
5. 返回快照副本。

> 由于返回的是拷贝，调用方修改数据不会影响内部状态。

---

## 5. 管理员接口：`GET /api/admin/metrics`

1. Handler 校验管理员权限（`isAdmin`）。  
2. 调用 `service.Snapshot()` 获取快照。  
3. 统一响应包装后返回 JSON。

环境变量 (可选)：

| 变量名                         | 默认 | 说明                     |
| ------------------------------ | ---- | ------------------------ |
| `ADMIN_METRICS_REFRESH_INTERVAL` | 5m   | 快照刷新周期             |
| `ADMIN_METRICS_RETENTION_DAYS`   | 7    | 统计保留天数（含今天）   |

---

## 6. 并发与锁策略总结

- **写入事件** 与 **重算快照** 共用同一把写锁 `s.mu`，因此写入、刷新、读取（当触发刷新时）都是串行执行的。
- 如果刷新耗时较长，新的写入会等待完成后继续，但不会丢失。
- 读取者拿到的是快照副本，外部操作无法污染内部缓存。

---

## 7. 调试建议

- 观察日志：  
  - `admin metrics scheduler started` / `stopped` 用于确认定时器状态。  
- 验证接口：  
  - 请求 `GET /api/admin/metrics`，确保管理员身份，确认 `daily` / `totals` 字段随生成/保存动作变化。
- 调节实时性：  
  - 若需更实时，可缩短刷新周期；若写入频繁导致锁竞争，可适当放大刷新周期或减少展示维度。
