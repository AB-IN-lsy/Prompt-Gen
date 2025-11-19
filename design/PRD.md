# 产品需求文档 (PRD)

> 版本：v1.1（2025-10-07）
>
> 负责人：产品 AB-IN
>

## 1. 背景与目标

### 1.1 背景

- 面试准备、专项训练和日常工作中，用户需要快速生成高质量 Prompt 与关键词组合，但缺乏系统化工具。
- 现有 Prompt 管理工具要么偏在线、缺乏离线能力，要么难以自定义模板与关键词治理。
- 我们已有 Electron + Go 技术栈样例，可快速搭建跨平台桌面应用，并复用现有 AI 模型接入经验。

### 1.2 项目目标

- 提供一款桌面端 Prompt 生产力工具，以本地离线存储为核心，帮助用户在 5 分钟内从需求到可用 Prompt。
- 支持围绕面试等高频场景的模板化生成与关键词治理，构建中心化的 Prompt 资产库。
- 打好扩展基础，未来可逐步扩展更多模型，实现多模型选择与协同。

### 1.3 成功指标（首版上线后 30 天内）

- DAU ≥ 300，注册到活跃转化率 ≥ 40%。
- 单次生成 Prompt 平均耗时 ≤ 20 秒（含关键词生成）。
- 平均每位活跃用户保存 ≥ 3 条 Prompt。
- 本地备份成功率 ≥ 98%，数据一致性投诉为 0。

### 1.4 目标用户与使用场景

| 角色 | 典型场景 | 关键诉求 |
| --- | --- | --- |
| 面试候选人 | 准备技术/行为面试 | 快速获得结构化提问、模拟对话 Prompt |
| 企业 HR / 面试官 | 设计面试问题、对话脚本 | 复用模板、批量调整关键词 |
| 内容创作者 | 生成主题内容或社媒脚本 | 按场景组合正/负面关键词，保存模板 |
| Prompt 工程师 | 管理 Prompt 库 | 管理版本、分类、共享 |

### 1.5 价值主张

- **提效**：从需求输入到 Prompt 出结果全链路自动化，覆盖关键词、模板生成、版本管理。
- **稳定**：本地 SQLite 提供离线访问，可通过手动备份保障数据不丢失。
- **扩展**：支持 DeepSeek 等模型与自定义模板，满足不同领域。

## 2. 范围定义

### 2.1 MVP 范围（v1.0）

1. 桌面应用（Windows/macOS）登录 / 本地模式切换。
2. 主题检索（本地）与关键词分类（正面/负面）。
3. Prompt 自动生成、编辑、保存、删除功能。
4. 本地 SQLite 存储，提供导出/导入能力确保数据安全。
5. AI 模型接入 DeepSeek（开启即用，可通过设置页管理）。
6. 设置页：API Key 管理、模型选择、数据导出、本地数据库路径。
7. 日志记录与简单的操作审计（供后续排查）。

### 2.2 非目标（首版不实现）

- 模板社区/分享、多人协作与权限管理。
- 移动/网页端客户端。
- 复杂权限、团队版计费。

### 2.3 关键假设与依赖

- 用户具备可用的 AI API Key（如 DeepSeek）；若无需登录则提供公用 Key 限额。
- 桌面端需安装 Go 运行环境或打包后端二进制。
- （可选）云端 MySQL/对象存储用于后续扩展，当前版本仍以本地 SQLite 为核心。
- 法务与安全：模型凭据等敏感字段需继续使用 AES-256-GCM（由 `MODEL_CREDENTIAL_MASTER_KEY` 提供密钥）进行加密存储，本地 Prompt 数据保持最小化持久化，并在扩展远程能力前完成合规评估。

## 3. 用户旅程与故事

### 3.1 主要旅程（面试准备）

1. 用户首次打开应用 → 选择本地模式 → 完成 API Key 填写。
2. 在首页输入“前端工程师面试” → 系统返回推荐主题与历史记录。
3. 用户选择“React 前端面试”，查看已有关键词；如不满意，点击“重新生成”。
4. 按需调整正/负面关键词 → 生成 Prompt → 预览并保存。
5. 添加备注、标签，保存至本地，并导出备份。
6. 后续进入“我的 Prompt”查看、编辑或复制导出。

### 3.2 核心用户故事

| 编号 | 角色 | 场景 | 验收标准 |
| --- | --- | --- | --- |
| US-01 | 面试候选人 | 需要快速生成模拟问答 Prompt | 输入主题后 20 秒内获得可用 Prompt，生成失败需给出可操作反馈 |
| US-02 | HR | 希望复用公司统一模板 | 能从模板库选择模板，关键词一键替换，保存新版本 |
| US-03 | Prompt 工程师 | 管理本地 Prompt | 本地列表支持搜索/过滤；可随时导出备份 |
| US-04 | 所有用户 | 选择生成模型（DeepSeek 等） | 设置页切换模型后，下次生成默认使用最新模型，并在结果中展示来源标识 |

## 4. 功能需求

### 4.1 首页与搜索

- **功能点**：主题搜索、历史记录、快速入口。
- **描述**：用户输入主题后，系统基于本地 SQLite 数据进行匹配，展示相关 Prompt 与关键词；若未命中，提供“生成新主题”按钮与关键词建议。
- **验收标准**：
  1. 搜索响应时间 ≤ 1 秒（本地查询）。
  2. 未找到时提供“生成新主题”按钮及关键词建议。
  3. 最近 5 条历史搜索展示，可手动删除。

### 4.2 自然语言描述解析

- **触发入口**：用户在首页或工作台输入一段自然语言意图描述（如“生成一个聚焦 React 的前端面试 Prompt，不要涉及老旧 HTML”）。
- **流程**：
  1. 前端将 `raw_description`、可选的目标模型（默认 DeepSeek）封装后调用 `POST /api/prompts/interpret`。
  2. 后端统一调用大模型，对描述进行结构化解析，返回 `topic`、`positive_keywords[]`、`negative_keywords[]`、解析可信度等元数据。
  3. 解析结果写入工作台左侧 Topic/关键词面板，显示来源=`model` 标签。
  4. 解析 API 默认仅在首次进入工作台或用户显式点击“重新解析描述”时触发，避免无谓重复消耗；需设置冷却时间（建议 60 秒）并给出限流提示。
  5. 服务端默认采用 1 分钟内最多 8 次的解析限流（可通过环境变量微调）。
- **验收标准**：
  - 初次解析需在 8 秒内返回结构化结果，失败时提示“请稍后重试或手动填写”。
  - 解析结果至少包含 1 个正面关键词；若模型给出的关键词为空，需提示用户补充。
  - 重新解析后保留用户已手动标记的关键词（来源=manual）不被覆盖。

### 4.3 关键词生成与管理

- **流程**：
  1. 调用本地 DB 检索（按 topic、正向/负向关键词权重排序）。
  2. 并行调用 AI 模型生成补充关键词（默认 DeepSeek），并与解析结果合并去重复；该调用在用户点击“向 AI 补充关键词”或解析结果明显不足时触发，返回内容标记来源=`api`。
  3. 接收用户输入的自然语言或关键词文本，解析后直接新增到列表，来源默认标记为 `manual`，支持与 AI 结果共同去重。
  4. 展示为可拖拽标签，支持改名、删除、标记正/负。
- **接口**：
  - `POST /api/prompts/keywords/augment`：基于当前 topic 与用户提供的上下文提示，返回补充关键词列表。
  - `POST /api/prompts/keywords/manual`：接收用户输入的关键词或短语，写入工作台并同步到本地数据库。
- **验收标准**：
  - 冲突处理：同名词汇合并显示来源标签（local/API）。
  - 用户编辑后需落库并更新“来源 = manual”。
  - 至少保留一个正面关键词才能生成 Prompt。
  - 支持手动补充关键词。
  - 关键词入库后会关联到 Prompt，可在详情中反查被哪些 Prompt 使用。

  - 每个关键词记录 `weight`（0~5），默认 5；前端支持通过 ± 调整、拖拽跨列，最新排序仅在权重或拖拽修改后同步至 `/api/prompts/keywords/sync`（附“按权重排序”按钮供手动整理）。

### 4.4 Prompt 生成与版本管理

- **功能点**：模板应用、生成、预览、保存、历史版本。
- **描述**：基于解析/编辑后的 Topic 与关键词，调用用户已配置凭据的模型生成主 Prompt + 可选的系统/示例消息；模型选择面板实时读取“模型凭据列表”中状态为启用的项，若无可用模型需引导用户前往设置页补全 API。
- **验收标准**：
  - 生成 API `POST /api/prompts/generate` 响应内包含：主题、正面关键词、负面关键词、使用模型、生成的 Prompt 正文、耗时；UI 中需同步展示。
  - 提供“再次生成”按钮，保留用户调整的关键词；默认采用 1 分钟内最多 5 次的限流阈值（可通过环境变量覆盖），超限时提示“请稍后再试”。
  - 保存时写入 SQLite（`prompts` 表），并记录 `user_id` / 时间戳。
  - 默认保存为 `draft` 状态，自动本地草稿保存（停顿 10 秒后触发，可通过 `VITE_PROMPT_AUTOSAVE_DELAY_MS` 调整），用户显式点击“发布”后才变为 `published` 并记录新的版本快照。
  - 发布操作需逐项校验主题、正文、补充要求、模型、正/负向关键词以及标签是否齐全，缺失项目需以 UI 提示阻断发布；保存草稿时上述字段可为空，支持分步骤完善内容。
  - 发布时需读取已存在的最大版本号并在其基础上递增，确保与历史 `prompt_versions` 记录一致，避免重复版本插入导致的唯一键冲突。
  - 历史版本默认保留最近 5 版，可回滚，回滚后生成新的 `draft` 版本记录（阈值可通过 `PROMPT_VERSION_KEEP_LIMIT` 配置）。
  - 版本号策略：首次仅保存草稿时版本号保持为 `0`；首次发布写入版本 `1`；发布后的再次保存草稿不会改变版本号，下一次发布会在历史最大版本基础上顺延 +1，保证序号连续。
  - `positive_keywords` / `negative_keywords` 仍保存为 JSON 数组，元素结构扩展为 `{ "keyword_id": number, "word": string, "weight": number, "source": string }`；SQLite 可直接使用 JSON 类型并默认 `[]`，MySQL 5.7 使用 `TEXT` 存储序列化结果，服务端需保证空数组写为 `'[]'`。

### 4.5 本地存储

- **数据表**：沿用原设计（`users` / `prompts` / `keywords`），补充索引用于搜索。
- **需求补充**：
  - 提供备份导出能力（JSON）。

### 4.6 数据备份与导入（更新）

- **模式**：提供一键导出（JSON）与导入能力，保障本地数据安全。
- **流程**：
  1. 用户在设置页点击“导出”即生成本地文件。
  2. 导入时校验文件结构，按需合并或覆盖现有 Prompt。
- **验收标准**：
  - 导出文件包含 Prompt、关键词、标签等核心字段。
  - 导入前提示备份当前数据，出现冲突时以“跳过/覆盖”方式处理。
  - 操作成功/失败均以 toast 提示，并在帮助页给出操作说明。

#### 离线预置数据同步（新增）

- **目标**：离线客户端首次启动时即可看到最新公共 Prompt 与 changelog，避免空白页。
- **流程**：
  1. 发布前执行 `go run ./backend/cmd/export-offline-data -output-dir backend/data/bootstrap`，从线上 MySQL 导出 `public_prompts.json`、`changelog_entries.json`。
  2. 将生成的 JSON 提交到版本库（目录固定为 `backend/data/bootstrap/`），CI 在打包阶段复制到 `resources/app/backend/data/bootstrap/`。
  3. Electron 后端子进程启动后，通过 `bootstrapdata.SeedLocalDatabase` 在 SQLite 表为空时导入上述数据；导入完成的条目会直接展示在 changelog 页面和公共 Prompt 列表。
- **验收标准**：
  - 打包后的安装目录可见两个 JSON，并在 changelog 页面看到一致的条目数。
  - 离线模式不暴露 `/admin/changelog`、`/admin/public-prompts` 等后台写操作，仅提供只读视图。
  - 更新 JSON 后重新打包即可覆盖旧数据，用户升级安装包后下次启动自动完成同步。

### 4.7 设置页（已实现）

- **配置项**：
  - AI 模型凭据：列表展示用户绑定的模型（如 DeepSeek、Moonshot 等），支持新增/编辑/停用；工作台仅展示启用且验证通过的模型（现已落地——使用抽屉表单录入凭据）。
  - API Key 管理：每个模型凭据单独存储 Key / Endpoint / 其他配置，可触发“测试连通性”，通过后标记为启用。前端在抽屉底部提供“测试连通性”按钮，成功/失败均以 toast + 字段校验反馈表现。
  - 数据导出：提供 JSON 导出/导入入口，并记录最近一次导出时间。
  - 界面偏好：提供「界面动效」开关，可控制 Aurora 背景与欢迎过场动画是否启用，并与用户设置联动。
  - 数据路径：显示当前 SQLite 文件路径，可修改（需重启后生效）。
- **验收标准**：修改后保存成功提示，错误需高亮字段并给出说明。当前实现中，提交表单时会针对空值/格式错误高亮字段并显示后端返回的错误文案。界面动效开关需在切换后即刻影响 Aurora 背景与 EntryTransition，并持久化到用户设置。
- **安全要求**：API Key 保存时走应用内 AES-256-GCM 加密，密钥通过 `MODEL_CREDENTIAL_MASTER_KEY` 提供；后端不会在日志或响应中输出明文。
- **接口**：
  - `GET /api/models`：返回当前用户启用/停用的模型凭据列表。
  - `POST /api/models`：新增模型凭据并触发连通性校验；成功后默认设为启用。
  - `PUT /api/models/{id}`：更新模型凭据信息或启用状态。
  - `DELETE /api/models/{id}`：软删除模型凭据（保留审计）。

#### API Key 管理策略

- **输入与校验**：设置页的抽屉表单要求填写模型标识、展示名、API Key、Base URL（可选）、附加参数（JSON）。提交前进行格式校验，并提供“测试连通性”按钮调用后端验证；错误会以 toast + 字段红框提示。
- **本地存储**：默认调用系统安全存储（Windows Credential Locker、macOS Keychain）。若因环境限制需应用内存储，则使用 AES-256-GCM 加密，密钥由用户主密码派生（PBKDF2），主密码不落盘；API Key 永不写入 SQLite 明文。
- **调用与缓存**：解密后的 Key 仅在内存中短时存在（最长 15 分钟或用户退出即时清除），调用结束立即清零内存；后端请求附带最小必要权限。
- **备份策略**：API Key 不随 Prompt 导出；如需在多设备使用，用户需在每台设备单独录入，可导出加密备份。
- **日志与审计**：所有相关日志只记录操作结果（成功/失败）与错误码，不包含 Key 内容；操作写入审计表以便排查。

### 4.8 日志与反馈（已实现）

- 操作日志：`/api/changelog` + 本地日志列表已提供，记录生成、导出、删除等关键行为，保留 30 天。UI 端在“帮助与日志”页面展示操作日志表格，支持过滤与导出。
- 系统反馈：统一 toast + 日志面板；严重错误可导出日志反馈给支持团队。目前前端在 `useRequest` 层统一调度成功/失败 toast，并提供导出日志按钮。

### 4.9 帮助中心（更新）

- 卡片分为六个主题：快速开始、仪表盘、我的 Prompt、Prompt 工作台、设置中心、排障手册，图标与文案分别对应上述主流程，帮助用户按页面快速定位操作指南。
- 每个面板提供 3 条以内的操作要点，覆盖新的自动保存延迟、分页配置与悬停反馈等行为；需保持中英双语同步更新。
- 资源区继续提供博客、源码、邮箱等外部链接，鼓励用户反馈问题并附带日志。

### 4.10 公共 Prompt 权限与治理

- **投稿前置条件**：仅允许状态为 `published` 的 Prompt 触发“投稿公共库”，草稿或归档状态需隐藏入口。后端在收到 `source_prompt_id` 时强制校验归属与发布状态，未满足条件直接返回错误。
- **列表可见性**：普通用户默认仅能浏览 `approved` 状态的条目；当筛选 `pending` / `rejected` 时，仅可获得本人投稿记录。管理员保留全量视图，可按作者与状态协同筛选。
- **详情权限**：公共 Prompt 详情在未通过审核前仅向作者与管理员开放，其余用户访问返回权限受限提示。
- **治理操作**：管理后台需提供“删除公共 Prompt”能力，可在审核前后移除不合规内容，同时刷新公共库列表与个人下载缓存。
- **驳回再投稿**：当投稿被驳回后，作者可在原主题下重新提交；系统复用原记录并把状态置回 `pending`、清空驳回原因，避免唯一索引冲突，并在界面明显展示驳回说明。
- **创作者主页（新增）**：
  - 为投稿人自动生成 `/creators/:id` 公开页面，展示头像、Headline、Bio、所在地 / 网站链接以及累计指标（投稿数、总下载、总点赞、总浏览），并突出最近 6 条精选作品。
  - 公共 Prompt 详情抽屉与评论头像点击即可跳转创作者主页；`GET /api/public-prompts` 与 `GET /api/public-prompts/:id` 同步返回 `author` 对象，供前端渲染投稿名片。
  - 用户资料新增 `profile_headline`、`profile_bio`、`profile_location`、`profile_website`、`profile_banner_url` 字段，通过 `PUT /api/users/me` 更新后即可反映在创作者主页及投稿名片中，默认使用“精选创作者”文案作为占位。
  - 新接口 `GET /api/creators/:id` 返回 `creator` 资料、`stats` 聚合指标与 `recent_prompts` 精选列表，供前端构建炫酷主页；未登录或 ID 不存在时返回 `401/404`。

### 4.11 管理员指标面板（新增）

- **功能点**：在桌面端直接呈现 DAU、生成请求量、生成成功率、平均耗时、保存次数等核心指标，并提供最近 7 天趋势图，替代手动登录 Grafana。
- **描述**：
  - 后端在应用启动时创建指标缓存服务，基于生成/保存事件写入内存桶，并以 `ADMIN_METRICS_REFRESH_INTERVAL` 周期重算快照；保留天数可通过 `ADMIN_METRICS_RETENTION_DAYS` 配置。
  - 新增接口 `GET /api/admin/metrics`（需管理员 Token），返回 `totals` 汇总与 `daily` 明细，字段包括活跃用户、请求量、成功数、成功率、平均耗时、保存次数。
  - 前端导航新增“运营监控”入口 `/admin/metrics`，加载成功后展示概览卡片、折线图与柱状图，自动每 5 分钟刷新并允许手动重试；非管理员访问时返回无权限提示。
- **验收标准**：
  1. 生成或保存操作后 1 分钟内能在面板中看到更新数据，刷新请求最长 5 秒返回。
  2. 环境变量缺失时默认使用 5 分钟刷新、7 天保留，服务运行期间不会重复注册指标或触发 panic。
  3. 图表与指标卡支持中英双语和响应式布局，加载失败时提供统一错误提示与重试按钮。

## 5. 数据结构设计

保留原表结构，并补充下列字段：

### 5.1 `users` 表

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| last_login_at | datetime | 最近登录时间 |
| settings | json / text | 用户自定义设置（模型、偏好项等），SQLite 使用 JSON 字段，MySQL 5.7 以 TEXT 存储 JSON 字符串 |

### 5.2 `prompts` 表

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| status | string | 状态：draft / published / archived |
| tags | json / text | 自定义标签，SQLite 使用 JSON 字段，MySQL 5.7 以 TEXT 存储 JSON 字符串 |
| model | string | 使用的 AI 模型，如 deepseek / gpt-5 |

### 5.3 `keywords` 表

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| weight | integer | 权重，用于排序 |
| language | string | 语言（zh / en 等） |

> SQL 建表语句参考附录 A，新增字段需同步更新。

### 5.4 `user_model_credentials` 表

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| id | integer / bigint | 主键，自增 |
| user_id | integer / bigint | 关联 `users.id` |
| provider | string | 模型供应商标识（如 `deepseek`、`openai`） |
| model_key | string | 模型唯一键（如 `deepseek-chat`、`gpt-5-large`） |
| display_name | string | UI 展示名称，可自定义 |
| base_url | string | 可选自定义 Endpoint |
| api_key_cipher | blob / text | 加密后的 API Key，明文不落盘 |
| extra_config | json / text | 其他配置（超时、代理等），默认 `{}` |
| status | string | `enabled` / `disabled` / `error` |
| last_verified_at | datetime | 最近一次连通性校验时间 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |

> API Key 采用 AES-256-GCM 加密，密钥来源于用户主密码或操作系统安全存储。

### 5.5 索引与约束策略

- `users`：`username`、`email` 全局唯一；为 `last_login_at` 创建 BTREE 索引以支撑最近登录排序；所有敏感字段加密存储或散列。
- `prompts`：对 `user_id, topic, status` 建复合索引用于列表过滤；为 `updated_at` 创建索引用于最近更新排序；`user_id + topic + created_at` 设唯一约束避免重复生成。
- `keywords`：对 `user_id + topic` 建复合索引用于主题检索；`user_id + word + topic` 唯一约束防止重复关键词；按 `polarity` 建部分索引便于正/负面筛选。
- `prompt_keywords`：设置 `(prompt_id, keyword_id)` 主键并对 `keyword_id` 建索引，支撑关键词反向查询；`relation` 字段与 Prompt 中 JSON 数组保持一致，定期校验一致性。
- `user_model_credentials`：对 `user_id + model_key` 设唯一约束，保证同一模型配置仅出现一次；为 `status`、`updated_at` 建索引便于筛选启用模型与定时校验任务。
- 所有外键默认 `ON UPDATE CASCADE ON DELETE CASCADE`（SQLite 通过触发器模拟），保证关联删除/更新一致；云端 MySQL 保留同样约束策略。

## 6. 信息架构与界面要点

### 6.1 页面结构

- **全局框架**：
  - 顶部固定跨页搜索条（支持主题、关键词模糊搜索）+ 全局操作区（“新建 Prompt”主按钮、最近活动）。
  - 左侧导航栏：`Dashboard`、`我的 Prompt`、`模板中心（预留）`、`设置`、`帮助与日志`；支持折叠。
- **主要页面 / 区域**：

| 区域 | 入口 | 主要模块 | 说明 |
| --- | --- | --- | --- |
| Dashboard 仪表盘 | 默认首页 | 快速搜索、最近访问 Prompt、草稿提醒、模型额度状态 | 作为登陆后的主视图，集中展示关键提醒和快捷入口（“继续编辑草稿”、“发布待办”）；最近访问列表提供悬停高亮和快捷打开。|
| Prompt 工作台 | 顶部“新建 Prompt”或“编辑”入口 | 左侧关键词面板（正/负面 Tag + 自定义输入）、右侧 Prompt 预览与元信息面板 | 关键词与 Prompt 强关联，使用双栏结构；顶部步骤条提示“关键词 → Prompt → 发布”。|
| 我的 Prompt | 左侧导航 | 列表、筛选、排序、批量操作、状态筛选（draft/published/archived） | 默认每页 10 条（`PROMPT_LIST_PAGE_SIZE` 可调），行悬停提供高亮与前移动效，支持标签筛选、批量发布/归档及快捷进入工作台。|
| 设置中心 | 左侧导航 | 账号信息、模型配置、数据导出、本地路径 | 同一页签内使用分组卡片，提供“连接测试”“重置路径”等操作。|
| 帮助与日志 | 左侧导航 | FAQ、错误日志、导出按钮、联系客服入口 | 整合系统日志查看与反馈渠道。|

- **分页提示**：公共 Prompt 列表统一 9 条/页，`meta.current_count` 用于驱动“本页条目数”提示；`My Prompts` 底部同样展示 `current_count`，帮助用户在离线安装包或导入数据后快速核对展示数量。默认值可通过环境变量 `PUBLIC_PROMPT_LIST_PAGE_SIZE` / `PROMPT_LIST_PAGE_SIZE`（后端）与 `VITE_PUBLIC_PROMPT_LIST_PAGE_SIZE` / `VITE_MY_PROMPTS_PAGE_SIZE`（前端）覆盖。
- **快捷发布**：`My Prompts` 列表提供“立即发布”按钮，直接复用工作台的发布校验逻辑（主题、正文、补充要求、模型、关键词、标签必填），满足快速上线需求。

### 6.2 关键 UX 原则

- 主要操作最多 3 步完成。
- 强反馈：生成 / 导出需展示 loading、进度。
- 新手引导：首次使用提供 3 步引导卡片或视频。

### 6.3 导航与流程

1. 用户在 Dashboard 通过顶部搜索或“新建 Prompt”进入 Prompt 工作台。
2. 工作台左侧维护关键词（支持拖拽排序、正负面切换、AI/手动混合）；右侧实时渲染 Prompt 草稿，提供版本信息与模型切换。
3. 点击“保存草稿”留在本地，状态为 `draft`，列表页面会出现草稿提醒；点击“发布”触发校验、调用后端生成最终 Prompt，并写入本地发布列表，状态切换为 `published`。
4. 发布后的 Prompt 可在“我的 Prompt”中再次编辑（生成新的草稿版本）或归档；归档后默认保存在本地备份中，不参与常规列表。
5. 所有页面保留左下角“帮助与日志”入口，便于问题定位与反馈。

## 7. 技术架构

### 7.1 系统分层

- **前端**：Electron + React + TailwindCSS + shadcn UI；渲染进程采用 TypeScript，所有跨进程通信通过 preload + contextBridge 暴露白名单 API。
- **主进程**：负责窗口管理、后端进程控制、系统设置读写和更新检测，禁止直接操作 UI 业务逻辑。
- **后端（Go）**：
  - 基于 Gin + Wire 进行路由和依赖注入管理，gRPC 预留可扩展接口。
  - 提供健康检查 `/api/status`、关键词生成 `/api/generate_keywords`、Prompt 生成 `/api/generate_prompt`、模型凭据管理 `/api/models` 等核心 API。
  - 集成 DeepSeek 模型 SDK，支持并发请求、超时重试与熔断（go-resilience）。
- **数据库**：
  - 本地 SQLite（加密），通过 GORM 访问，统一迁移脚本管理。
  - 预留云端 MySQL（结构与本地一致，额外 `user_id` 约束与审计字段），供后续拓展使用。
- **运行日志**：异常日志写入本地文件，并可自助上传至支持端。

### 7.2 配置与环境管理

- **Nacos 注册中心**：
  - Server 列表通过代码静态声明：

    ```go
    serverConfig := []constant.ServerConfig{
        {
            IpAddr:      "8.140.250.217",
            Port:        8848,
            ContextPath: "/nacos",
        },
    }
    ```

  - Client 侧读取 `DEFAULT_GROUP` 下的配置以获取数据库、模型开关等动态参数。
  - 账号：`nacos`，密码：`AB-INlsy20010831`（仅供开发调试使用，正式发行版必须改为环境变量或系统凭据管理）。
- **MySQL 远程实例**：
  - 主库地址：`8.140.250.217:3306`，数据库名 `prompt`。
  - 账号：`root`，密码：`AB-INlsy20010831`（仅用于内部开发环境）。
  - 上述凭据由 Nacos 托管，不能写入仓库或构建产物，部署时统一从 Nacos 动态拉取。
  - Nacos 配置项：

    | Data ID | Group | 内容格式 | JSON 样例 |
    | --- | --- | --- | --- |
    | `mysql-config.properties` | `DEFAULT_GROUP` | JSON | `{ "mysql.host": "8.140.250.217", "mysql.port": 3306, "mysql.username": "root", "mysql.password": "AB-INlsy20010831" }` |

  - 服务启动逻辑：
    1. 读取 Nacos 的 `mysql-config.properties`。
    2. 使用 `mysql.host` 等字段拼装连接串（默认字符集 `utf8mb4`，开启超时重试）。
    3. 若 Nacos 不可用，则回退到本地安全存储的密文配置（不得以明文写入代码库）。
- **安全要求**：任何明文凭据仅可存在于 Nacos 或受控的运维系统；仓库中的示例配置必须指明“禁止明文提交”，CI 需检测防止泄漏。客户端发行版本需引导用户在首次启动时输入 Nacos 密码或从操作系统安全存储中解密获取，严禁硬编码到可执行文件中。

### 7.3 编码规范与工程约束

- **前后端协作**：遵循前后端分离，Electron 主进程只负责基础能力，业务数据通过 HTTP/gRPC 获取，禁止通过 `remote` 模块或直接共享对象。
- **语言与框架约束**：
  - 前端统一使用 TypeScript + React Function Component，组件目录遵循 `feature/ComponentName` 结构。
  - Go 服务使用 Gin，业务按“handler → service → repository → entity”分层，所有接口返回统一响应结构。
- **代码风格**：
  - 前端遵循 ESLint (airbnb-typescript) + Prettier；后端遵循 `golangci-lint` 默认规则，提交前必须通过 `make lint`。
  - 单文件长度前端不超过 300 行、后端不超过 400 行；超过需拆分模块或组件。
  - 组件/服务命名短小明确，禁止缩写不明词，如 `GeneratePromptService`、`KeywordList`。
- **测试与质量**：每个模块至少配备 1 条 Happy Path + 1 条边界单测；关键流程（解析、生成、导出）补充集成测试，前端使用 Playwright 进行冒烟测试。
- **提交规范**：采用 Conventional Commits（`feat`, `fix`, `chore` 等）；启用 Husky + lint-staged 做 pre-commit 校验。

## 8. 非功能需求

| 维度 | 指标 |
| --- | --- |
| 性能 | 单次生成接口响应 ≤ 8 秒；并发 50 QPS 下可用 |
| 可用性 | 服务可用性 ≥ 99%；桌面应用崩溃率 < 1% |
| 安全 | 本地数据 AES 加密；API Key 仅存加密形式；遵循《个人信息保护法》 |
| 兼容性 | Windows 10+、macOS 13+；后续评估 Linux |
| 可维护性 | 日志包含 `trace_id`，关键操作可追踪 |

## 9. 指标与运营

- **产品指标**：DAU、生成成功率、平均生成时长、关键词使用频次。
- **商业指标**：高级模型使用率，潜在订阅转化（预留）。
- **运营动作**：上线期通过引导模板包（热门面试岗位）提升活跃；提供内置示例库。
- **埋点需求**：页面曝光、按钮点击、生成结果状态、导出结果等。

## 10. 项目里程碑（建议）

| 时间 | 里程碑 | 交付 |
| --- | --- | --- |
| T+0 | 需求评审完成 | PRD 冻结、设计启动 |
| T+2 周 | 原型评审 | 高保真交互稿、设计规范 |
| T+6 周 | 功能开发完成 | MVP 功能验收、单元测试 |
| T+8 周 | Beta 内测 | 30 名种子用户、收集反馈 |
| T+10 周 | 正式发布 | Windows/macOS 版本上线、基础运营物料 |

## 11. 风险与对策

| 风险 | 影响 | 对策 |
| --- | --- | --- |
| AI 模型费用高 | 成本超支 | 设定每日调用限额，区分免费/付费额度 |
| 模型服务不稳定 | 影响生成体验 | 默认使用 DeepSeek，必要时提示当前模型状态并允许切换 |
| Electron 包体大 | 下载体验差 | 分离后端二进制，考虑差分更新 |
| 数据安全合规 | 法务风险 | 引入隐私协议弹窗、加密策略审计 |

## 12. 未来扩展方向

- 模板中心：用户上传/分享/评分，形成社区型资产库。
- 团队协作：权限管理、多人编辑、版本 diff。
- 智能分析：根据历史使用情况推荐关键词和 Prompt。
- 移动端 / Web 端轻量版本。
- 插件生态：面试平台或 ATS 系统接入。

---

### 附录 A：原始数据表建模 SQL（节选）

> 说明：本地默认使用 SQLite，若后续启用云端能力，可参考 MySQL 版本 DDL。以下给出双版本 DDL，便于直接复制执行。

#### SQLite 版本（本地离线）

```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL,
  settings TEXT,
  last_login_at DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT uq_users_username UNIQUE (username),
  CONSTRAINT uq_users_email UNIQUE (email)
);

CREATE INDEX idx_users_last_login ON users (last_login_at DESC);

CREATE TABLE prompts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  topic VARCHAR(255) NOT NULL DEFAULT '',
  prompt TEXT NOT NULL,
  positive_keywords JSON DEFAULT '[]',
  negative_keywords JSON DEFAULT '[]',
  model VARCHAR(64) NOT NULL DEFAULT '',
  status VARCHAR(32) NOT NULL DEFAULT 'draft',
  tags JSON DEFAULT '[]',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_prompts_user FOREIGN KEY (user_id)
    REFERENCES users (id)
    ON UPDATE CASCADE
    ON DELETE CASCADE,
  CONSTRAINT uq_prompts_user_topic_created UNIQUE (user_id, topic, created_at)
);

CREATE INDEX idx_prompts_user_topic_status ON prompts (user_id, topic, status);
CREATE INDEX idx_prompts_updated_at ON prompts (updated_at DESC);

CREATE TABLE keywords (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  word VARCHAR(255) NOT NULL DEFAULT '',
  polarity VARCHAR(16) NOT NULL DEFAULT 'positive' CHECK (polarity IN ('positive', 'negative')),
  topic VARCHAR(255) NOT NULL DEFAULT '',
  source VARCHAR(64) NOT NULL DEFAULT 'local',
  weight INTEGER DEFAULT 0,
  language VARCHAR(16) NOT NULL DEFAULT 'zh',
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_keywords_user FOREIGN KEY (user_id)
    REFERENCES users (id)
    ON UPDATE CASCADE
    ON DELETE CASCADE,
  CONSTRAINT uq_keywords_user_word_topic UNIQUE (user_id, word, topic)
);

CREATE INDEX idx_keywords_user_topic ON keywords (user_id, topic);
CREATE INDEX idx_keywords_user_polarity ON keywords (user_id, polarity);

CREATE TABLE prompt_keywords (
  prompt_id INTEGER NOT NULL,
  keyword_id INTEGER NOT NULL,
  relation VARCHAR(16) NOT NULL DEFAULT 'positive' CHECK (relation IN ('positive', 'negative')),
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (prompt_id, keyword_id),
  CONSTRAINT fk_prompt_keywords_prompt FOREIGN KEY (prompt_id)
    REFERENCES prompts (id)
    ON UPDATE CASCADE
    ON DELETE CASCADE,
  CONSTRAINT fk_prompt_keywords_keyword FOREIGN KEY (keyword_id)
    REFERENCES keywords (id)
    ON UPDATE CASCADE
    ON DELETE CASCADE
);

CREATE INDEX idx_prompt_keywords_prompt ON prompt_keywords (prompt_id);
CREATE INDEX idx_prompt_keywords_keyword ON prompt_keywords (keyword_id);

CREATE TABLE user_model_credentials (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL,
  provider VARCHAR(64) NOT NULL,
  model_key VARCHAR(128) NOT NULL,
  display_name VARCHAR(128) NOT NULL,
  base_url TEXT,
  api_key_cipher BLOB NOT NULL,
  extra_config JSON DEFAULT '{}',
  status VARCHAR(16) NOT NULL DEFAULT 'enabled',
  last_verified_at DATETIME,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  CONSTRAINT fk_model_credentials_user FOREIGN KEY (user_id)
    REFERENCES users (id)
    ON UPDATE CASCADE
    ON DELETE CASCADE,
  CONSTRAINT uq_model_credentials_user_model UNIQUE (user_id, model_key)
);

CREATE INDEX idx_model_credentials_status ON user_model_credentials (status);
CREATE INDEX idx_model_credentials_updated_at ON user_model_credentials (updated_at DESC);
```

#### MySQL 5.7 版本（云端扩展）

```sql
-- MySQL 5.7-compatible schema for prompt-gen
CREATE TABLE
    users (
        id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
        username VARCHAR(64) NOT NULL DEFAULT '',
        email VARCHAR(255) NOT NULL DEFAULT '',
        password_hash VARCHAR(255) NOT NULL,
        settings TEXT,
        last_login_at DATETIME,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        PRIMARY KEY (id),
        CONSTRAINT uq_users_username UNIQUE (username),
        CONSTRAINT uq_users_email UNIQUE (email)
    ) ENGINE = InnoDB;

CREATE INDEX idx_users_last_login ON users (last_login_at DESC);

CREATE TABLE
    prompts (
        id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
        user_id BIGINT UNSIGNED NOT NULL,
        topic VARCHAR(255) NOT NULL DEFAULT '',
        prompt TEXT NOT NULL,
        positive_keywords TEXT NOT NULL COMMENT 'JSON array with {"keyword_id":number,"word":string}',
        negative_keywords TEXT NOT NULL COMMENT 'JSON array with {"keyword_id":number,"word":string}',
        model VARCHAR(64) NOT NULL DEFAULT '',
        status VARCHAR(32) NOT NULL DEFAULT 'draft',
        tags TEXT NOT NULL COMMENT 'JSON array of labels',
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        PRIMARY KEY (id),
        CONSTRAINT fk_prompts_user FOREIGN KEY (user_id) REFERENCES users (id) ON UPDATE CASCADE ON DELETE CASCADE,
        CONSTRAINT uq_prompts_user_topic_created UNIQUE (user_id, topic, created_at)
    ) ENGINE = InnoDB;

CREATE INDEX idx_prompts_user_topic_status ON prompts (user_id, topic, status);

CREATE INDEX idx_prompts_updated_at ON prompts (updated_at DESC);

CREATE TABLE
    keywords (
        id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
        user_id BIGINT UNSIGNED NOT NULL,
        word VARCHAR(255) NOT NULL DEFAULT '',
        polarity VARCHAR(16) NOT NULL DEFAULT 'positive',
        topic VARCHAR(255) NOT NULL DEFAULT '',
        source VARCHAR(64) NOT NULL DEFAULT 'local',
        weight INT DEFAULT 0,
        language VARCHAR(16) NOT NULL DEFAULT 'zh',
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        PRIMARY KEY (id),
        CONSTRAINT chk_keywords_polarity CHECK (polarity IN ('positive', 'negative')),
        CONSTRAINT fk_keywords_user FOREIGN KEY (user_id) REFERENCES users (id) ON UPDATE CASCADE ON DELETE CASCADE,
        CONSTRAINT uq_keywords_user_word_topic UNIQUE (user_id, word, topic)
    ) ENGINE = InnoDB;

CREATE INDEX idx_keywords_user_topic ON keywords (user_id, topic);

CREATE INDEX idx_keywords_user_polarity ON keywords (user_id, polarity);

CREATE TABLE
    prompt_keywords (
        prompt_id BIGINT UNSIGNED NOT NULL,
        keyword_id BIGINT UNSIGNED NOT NULL,
        relation VARCHAR(16) NOT NULL DEFAULT 'positive',
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
        PRIMARY KEY (prompt_id, keyword_id),
        CONSTRAINT chk_prompt_keywords_relation CHECK (relation IN ('positive', 'negative')),
        CONSTRAINT fk_prompt_keywords_prompt FOREIGN KEY (prompt_id) REFERENCES prompts (id) ON UPDATE CASCADE ON DELETE CASCADE,
        CONSTRAINT fk_prompt_keywords_keyword FOREIGN KEY (keyword_id) REFERENCES keywords (id) ON UPDATE CASCADE ON DELETE CASCADE
    ) ENGINE = InnoDB;

CREATE INDEX idx_prompt_keywords_prompt ON prompt_keywords (prompt_id);

CREATE INDEX idx_prompt_keywords_keyword ON prompt_keywords (keyword_id);

CREATE TABLE
  user_model_credentials (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    provider VARCHAR(64) NOT NULL,
    model_key VARCHAR(128) NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    base_url TEXT,
    api_key_cipher VARBINARY(1024) NOT NULL,
    extra_config JSON,
    status ENUM('enabled','disabled','error') NOT NULL DEFAULT 'enabled',
    last_verified_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    CONSTRAINT fk_model_credentials_user FOREIGN KEY (user_id)
      REFERENCES users (id)
      ON UPDATE CASCADE
      ON DELETE CASCADE,
    CONSTRAINT uq_model_credentials_user_model UNIQUE (user_id, model_key)
  ) ENGINE = InnoDB;

CREATE INDEX idx_model_credentials_status ON user_model_credentials (status);
CREATE INDEX idx_model_credentials_updated_at ON user_model_credentials (updated_at DESC);
```
