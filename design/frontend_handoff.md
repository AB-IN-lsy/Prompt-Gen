# PromptGen 前端设计交接说明

> 版本：v1.0（2025-10-09）  
> 作者：GitHub Copilot  
> 协作者：AB-IN

---

## 1. 背景与目标

- 明确 PromptGen 项目前端的交互设计、组件规划与 API 契约，为我们内部协作（后端开发 + 前端实现）提供统一基线。
- 在进入视觉稿 / 前端开发之前，对信息架构、交互流程、数据接口、职责分工达成一致，降低返工成本。

---

## 2. 全局 UX 与视觉基调

- **设计原则**：高效、可扩展、低学习成本；关键任务≤3 步，信息一眼可得。
- **色彩体系**：主色 `#3B5BDB`，辅色 `#5F3DC4`，背景 `#F8F9FA`，卡片 `#FFFFFF`，正文 `#1F2933`，分隔线 `#E5E7EB`，状态色（成功 `#12B886` / 警告 `#F08C00` / 错误 `#E03131`）。
- **排版**：主字体 `Inter`（中文 fallback `PingFang SC`）；标题 28/24/20px，正文 16px，说明文字 14px。
- **组件风格**：圆角 12px 卡片 + 轻阴影；按钮圆角 8px。采用 shadcn UI + TailwindCSS 统一主题。

### 2.1 高级视觉语言（Modern & Sleek）

- **背景层次**：默认浅色模式中引入柔和渐变背景（如 `linear-gradient(135deg, #F8F9FA 0%, #E9EDFF 100%)`），卡片周围使用微弱投影（`0 18px 40px rgba(56, 56, 122, 0.12)`）营造悬浮感。
- **玻璃质感点缀**：导航栏与关键浮层（快捷搜索、版本抽屉）采用半透明毛玻璃效果（背景透明度 70%、背板色 #FFFFFF、Blur 20px），既保持现代感，又能凸显层级。
- **色彩闪点**：主行动按钮与高优先级标签允许添加轻微渐变（如 `#3B5BDB → #5F3DC4`），Hover 时伴随 4px 内发光（`box-shadow: 0 0 0 4px rgba(59, 91, 219, 0.15)`）强化可点击性。
- **微交互动效**：
  - 导航与按钮使用 150ms ease-out 过渡，点击后提供 90ms scale-in（0.98 → 1）反馈。
  - 列表卡片 Hover 时微微抬升（translateY(-4px) + shadow 加强），配合淡入阴影提升高级感。
  - 关键词拖拽时展示磁吸效果：拖动元素附带 8px 阴影 + 背景色变亮，放置区域出现描边高亮。
- **数据可视化**：Dashboard 中的指标卡片使用柔和渐变背景与细线图（细至 1px，颜色带透明度 40%）呈现趋势，强调“炫酷但不喧宾”。
- **图标与插画**：统一使用 `lucide-react` 线性图标，必要时配合定制渐变描边；空态图和引导图可选用简洁抽象插画（如 Undraw 线性风格），色调控制在主辅色系内。
- **暗色模式预留**：在 Tailwind 主题中为暗色模式预设反转色（背景 #0F172A、卡片 #111C3A、主色提亮 15%），确保后续拓展易于实现。

---

## 3. 页面线框描述

### 3.1 应用框架

- 顶部导航（高度 ~64px）：Logo、全局搜索、同步状态、最近活动、用户菜单。
- 侧边导航（宽 240px，可折叠）：Dashboard、我的 Prompt、Prompt 工作台、设置、帮助与日志；底部主题切换与版本信息。
- 主内容区域：浅灰背景 + 白色卡片承载子页面。

### 3.2 Dashboard

- 顶部双卡片：快捷搜索入口、云端同步状态提示。
- 中部：左侧最近 Prompt 列表（快捷继续编辑），右侧草稿提醒 + “新建 Prompt” 按钮。
- 底部：模型额度/API Key 状态、公告与操作指南。

### 3.3 Prompt 工作台

- 左栏关键词面板（宽 ~380px）：正/负面 Tab、AI 生成/手动添加按钮、关键词拖拽卡片（来源标识 + 权重调整）。
- 右栏编辑区：步骤条（关键词 → Prompt → 发布）、文本编辑 + Markdown 预览、模型切换、再次生成/保存草稿/发布按钮、自动草稿提示。

### 3.4 我的 Prompt

- 顶部筛选栏：搜索、状态、模型、标签、批量操作开关。
- 默认表格视图：显示标题、状态、标签、模型、更新时间、快捷操作（编辑、发布/归档、复制、删除）。
- 卡片视图：可视化展示 Prompt 摘要与关键词。
- 右侧版本抽屉：查看历史版本、差异、回滚操作。

### 3.5 设置中心

- 模块卡片：账号信息、模型配置、API Key 管理（抽屉输入 + 测试）、同步策略（自动/手动 + 路径）、本地数据库路径（修改后重启生效）。
- 安全提示：API Key 加密策略与日志规范。

### 3.6 帮助与日志

- 左侧 FAQ 折叠菜单。
- 右侧日志表格（时间、事件、结果、trace_id、详情、导出）。
- 顶部 “反馈问题” 按钮：弹窗支持描述 + 上传日志包。

---

## 4. 核心交互流程

| 流程 | 步骤概述 | 备注 |
| --- | --- | --- |
| 关键词生成 | “AI 生成” → 选择模型 → Loading → 返回建议 → 勾选合并 → 写入列表 | 冲突词显示来源标签，支持拖拽排序与正/负互换 |
| Prompt 编辑 | 调整关键词 → 编辑文本/模板 → 实时预览 → 自动草稿保存 → “保存草稿” | 草稿仅本地保存，不同步 |
| Prompt 发布 | 点击 “发布” → 校验（关键词、内容、Key 状态）→ 确认弹窗 → 调用发布 API | 发布后进入同步队列，并提示成功 |
| Prompt 再生成 | 点击 “再次生成” → 调用后端生成 API → 更新预览 → 选择覆盖或另存草稿 | 保留用户自定义关键词 |
| 同步冲突处理 | 同步失败 → 红色提示 → 打开冲突弹窗 → 展示差异 → 选择策略 → 提交 | 策略：本地覆盖 / 云端覆盖 / 保留两份 |
| API Key 管理 | 设置页 → “管理 Key” → 输入 + 测试连通性 → 保存（加密） | 主进程负责调用系统安全组件或 AES 加密 |
| 日志反馈 | 帮助页 → 查看/筛选日志 → 导出或提交反馈 | 日志保留 30 天，可附加 trace_id |

---

## 5. 组件清单

| 组件 | 功能 | 关键状态 |
| --- | --- | --- |
| `AppShellLayout` | 顶部导航 + 侧边栏框架 | 折叠/展开、主题切换 |
| `GlobalSearch` | 全局搜索与历史建议 | 输入中、候选下拉、空态 |
| `SyncStatusIndicator` | 同步状态灯 + Tooltip + 操作按钮 | 成功/失败/冲突、禁用 |
| `PromptCard` | Dashboard / 我的 Prompt 列表项 | Hover、快捷操作、标签展示 |
| `KeywordTagCard` | 关键词标签卡片 | 拖拽、权重调整、来源标识、删除 |
| `AIKeywordModal` | AI 生成关键词弹窗 | Loading、勾选、冲突提示 |
| `PromptEditor` | 文本编辑 + Markdown 预览 | 自动保存、生成中、模型切换 |
| `VersionHistoryDrawer` | Prompt 历史版本抽屉 | 差异对比、回滚确认 |
| `FilterBar` | 列表筛选工具栏 | 多条件组合、重置 |
| `ApiKeyDrawer` | API Key 录入和测试 | 校验反馈、密文提示 |
| `ConflictResolutionModal` | 同步冲突处理页 | 策略选择、差异展示 |
| `LogTable` | 日志列表 | 分页、筛选、导出 |
| `Toast/Notification` | 全局提示 | 成功/失败/警告、队列管理 |

---

## 6. 首批 API 契约（草案）

### 6.1 关键词

- `GET /api/keywords?topic=&polarity=&source=` → `[{ id, word, polarity, source, weight, language, topic, updatedAt }]`
- `POST /api/keywords/generate`
  - Body：`{ topic, positiveSeeds, negativeSeeds, model }`
  - Response：`{ suggestions: [{ word, polarity, source, confidence }] }`
- `POST /api/keywords` → `{ word, polarity, source: 'manual', topic, weight, language }`
- `PATCH /api/keywords/:id`
- `DELETE /api/keywords/:id`

### 6.2 Prompt

- `GET /api/prompts?status=&tags=&model=&search=&page=&size=` → 分页响应 `{ items, total }`
- `GET /api/prompts/:id` → 包含 `versions: [{ versionId, status, createdAt, diffSummary }]`
- `POST /api/prompts`
  - Body：`{ topic, prompt, positiveKeywords: KeywordRef[], negativeKeywords: KeywordRef[], model, tags, status }`
  - `KeywordRef = { keywordId, word }`
- `PATCH /api/prompts/:id`
- `POST /api/prompts/:id/publish`
- `POST /api/prompts/:id/regenerate`

### 6.3 同步

- `GET /api/sync/status` → `{ lastSyncedAt, mode, pendingCount, lastResult, conflicts? }`
- `POST /api/sync/run`
- `POST /api/sync/resolve`
  - Body：`{ promptId, strategy: 'local'|'remote'|'duplicate', remoteVersion, localVersion }`

### 6.4 设置与安全

- `GET /api/settings` → `{ defaultModel, autoSync, databasePath, lastSyncAt, featureFlags }`
- `PATCH /api/settings`
- `POST /api/settings/api-key`
- `POST /api/settings/api-key/test`

### 6.5 日志

- `GET /api/logs?eventType=&result=&rangeStart=&rangeEnd=&page=`
- `POST /api/logs/export`

> 说明：认证相关接口（登录 / 刷新 / 登出）已完成；其余接口按本表逐步实现并补测试。

---

## 7. 职责分工与协作方式

| 模块 | 负责人 | 备注 |
| --- | --- | --- |
| 前端架构与实现 | GitHub Copilot | Electron + React + Tailwind + shadcn + Zustand + React Query；按页面优先级交付 |
| 后端接口扩展 | AB-IN | 实现关键词、Prompt、同步、设置、日志等 API；补单元/集成测试 |
| UI / 视觉校验 | 双方协作 | 联合审查线框与视觉稿，无需额外设计师 |
| 数据合同维护 | 双方协作 | 字段调整需先更新本文档再开发 |
| 发布与测试 | 双方协作 | 前端提供打包方案，后端提供测试环境与数据 |

- 协作机制：
  - 本文档作为单一事实源，后续变更及时更新版本号。
  - 交互或 API 变更先在此记录，再安排开发排期。
  - 视觉稿确认后再进入实现阶段，避免重复返工。

---

## 8. 近期里程碑建议

1. **第 1 周**：完成 Figma 线框与视觉稿；细化 API 字段与错误码；初始化 Electron + React 脚手架。
2. **第 2-3 周**：实现 Prompt 工作台核心流程（关键词管理、Prompt 编辑、草稿/发布）。
3. **第 4 周**：落地 “我的 Prompt” 列表、版本历史、Dashboard 页面。
4. **第 5 周**：完善设置中心、API Key 管理、日志与反馈；与后端联调同步 API。
5. **第 6 周**：打通同步与冲突处理全链路，完成端到端冒烟测试，准备 Beta。

---

## 9. 动效与交互反馈规范

| 场景 | 动效参数 | 说明 |
| --- | --- | --- |
| 页面切换 | 200ms fade + 10px slide-up，`ease-out` | 保持轻盈流畅，减少眩晕感 |
| 卡片浮现 | 150ms scale-in (0.96 → 1) + 阴影渐显 | Dashboard、Prompt 卡片统一效果 |
| Toast 通知 | 180ms slide-from-bottom + 12px 模糊消散 | 保持现代感，同时不遮挡核心内容 |
| 模态弹窗 | 220ms `ease-in-out`，背景蒙层透明度 0 → 70% | 支持 ESC 与蒙层点击关闭；出现时主色描边闪动一次 |
| Loading 骨架 | 使用 shimmer（渐变亮带 1200ms 循环） | 统一骨架宽高，颜色 `#E2E8F0 → #F8FAFC` |
| 拖拽排序 | 元素跟随 60fps，释放时 180ms 回弹 `spring(0.5, 0.8)` | 保证顺滑、倍率与实际位置同步 |

- 所有动效遵循 “Fast-in, Smooth-out” 原则，避免冗长动画影响效率。
- 动效常量集中管理（如 `motion.config.ts`），便于统一调优与暗色模式兼容。
- 输入校验反馈采用 120ms color transition + icon shake（偏移 2px，2 次往返），保证专业又醒目。

---

如需扩展动效、状态图或错误码清单，可在本文件追加章节并同步到排期。

---

## 10. 前端 API 客户端约定

- **基础实例**：`frontend/src/lib/http.ts`
  - 从 `VITE_API_BASE_URL`（默认 `/api`）读取后端地址。
  - 自动注入 `Authorization: Bearer <access_token>`。
  - 解包统一响应结构 `{ success, data, error, meta }`；失败时抛出 `ApiError`。
  - 对 401 响应执行刷新流程，避免在多个并发请求中重复刷新。
- **错误模型**：`frontend/src/lib/errors.ts` 定义 `ApiError`，包含 `status`、`code`、`details`，方便 UI 精细处理。
- **Token 存储**：`frontend/src/lib/tokenStorage.ts` 提供 `setTokenPair` / `getTokenPair` / `clearTokenPair`，并在 access token 过期前 5 秒尝试预刷新。
- **业务 API**：`frontend/src/lib/api.ts`
  - 暴露关键词、Prompt 列表/保存/发布等函数，均抛出 `ApiError`。
  - 返回值包含强类型接口（如 `Keyword`、`PromptSummary`），分页接口会返回 `{ items, total, meta }`。
- **鉴权流程**：
  - 登录成功后调用 `setAuthTokens(access, refresh, expires_in)`；
  - `http` 实例负责后续请求头注入与刷新；
  - 退出登录调用 `clearAuthTokens()`，确保本地凭证清理。
