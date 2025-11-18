# 后端开发指南

本项目提供 Electron 桌面端所需的 Go 后端服务，负责用户注册、登录、验证码获取以及用户信息维护。

## 最近进展

- 调整 Prompt 工作台：前端仅在检测到关键词顺序或权重实际变化时才调用 `POST /api/prompts/keywords/sync`，显著降低接口调用频率，后端无需额外改动即可受益。
- Prompt 生成参数可配置：`prompts` 表新增 `generation_profile` TEXT 字段，保存逐步推理、温度、Top P、最大输出 tokens 等配置，并在生成/保存时透传到模型调用链；相关默认值通过 `PROMPT_GENERATE_*` 环境变量控制。
- 新增 Prometheus 指标：后端会在 `/metrics` 暴露生成/保存相关指标，可配合 Grafana Cloud 进行性能与错误监控。
- 管理员指标缓存就绪：`internal/service/adminmetrics` 会基于生成/保存事件维护按日聚合的活跃用户、请求量、成功率、平均耗时与保存次数，刷新周期、保留窗口通过 `ADMIN_METRICS_REFRESH_INTERVAL`、`ADMIN_METRICS_RETENTION_DAYS` 配置；新增 `GET /api/admin/metrics` 供前端仪表盘直接消费。
- 管理员指标持久化：服务会将原始事件写入 `admin_metrics_events`，按日聚合结果写入 `admin_metrics_daily`，启动时自动回放最近 `ADMIN_METRICS_RETENTION_DAYS` 天的数据以恢复内存缓存；当快照刷新后会同步更新 MySQL 并清理过期事件。
- 管理员用户总览上线：新增 `internal/service/adminuser` 聚合用户、Prompt 统计与最近活动，配置项通过 `ADMIN_USER_*` 控制分页、近况条数与在线阈值；`GET /api/admin/users` 返回在线状态、Prompt 数量分布及最近更新的 Prompt 摘要，便于后台巡检。
- 解析 & 生成统一接入内容审核：新增 `Service.auditContent`，在解析前和生成后复用用户配置的模型执行违规检测，若命中策略会返回 `CONTENT_REJECTED` 错误码与可读提示，前端直接用于 toast。
- 新增 `POST /api/prompts/import`，可上传导出的 JSON 文件并选择“合并/覆盖”模式批量回灌 Prompt，导入批大小由 `PROMPT_IMPORT_BATCH_SIZE` 控制。
- “自动解析 Prompt” 上线：`POST /api/prompts/ingest` 支持粘贴完整 Prompt 正文并自动拆解主题、标签与关键词，随后立刻创建草稿；限流阈值由 `PROMPT_INGEST_LIMIT` / `PROMPT_INGEST_WINDOW` 控制。
- 本地离线模式下会自动关闭邮箱验证、Prompt 生成与公共库的限流器，避免开发或演示环境频繁操作触发限流提示。
- `/api/users/me` 响应新增 `runtime_mode` 字段，标记后端当前运行在本地（`local`）还是在线（`online`）模式，供前端自动切换离线特性。
- 评论互动增强：
  - 新增 `POST /api/prompts/comments/:id/like` 与 `DELETE /api/prompts/comments/:id/like`，用于点赞 / 取消点赞单条评论，接口响应包含最新的 `like_count` 与 `liked` 状态。
  - `GET /api/prompts/:id/comments` 增补 `like_count`、`is_liked` 字段，支持在同一结构中返回评论树及点赞态。
  - 新增表 `prompt_comment_likes` 存储点赞关系，点赞计数写入 `prompt_comments.like_count`，步长可通过 `PROMPT_COMMENT_LIKE_STEP` 调整（默认 1）。
  - 需要在 `.env(.local)` 中新增 `PROMPT_COMMENT_LIKE_STEP=<整数>` 控制单次点赞对计数的增量。
- 新增用户头像字段 `avatar_url`，支持上传后通过 `PUT /api/users/me` 保存。
- 新增 `POST /api/uploads/avatar` 接口，可接收 multipart 头像并返回可访问的静态地址（支持匿名访问，便于注册阶段上传头像）。
- 默认开放 `/static/**` 路由映射到 `backend/public` 目录，供头像等资源直接访问。
- 用户资料更新接口现会在保存前检查用户名、邮箱冲突，并返回更明确的错误。
- 注册接口在邮箱、用户名同时冲突时会通过 `error.details.fields` 返回完整字段列表，方便前端逐项提示。
- `PUT /api/users/me` 支持显式传入空字符串清空头像，便于用户撤销已上传的头像图片。
- 单元测试覆盖 `UserService.UpdateProfile`，确保资料更新逻辑稳定。
- 新增邮箱验证流程，注册后会发放一次性验证令牌；邮箱未验证前无法登录。
- 注册成功会立即调用邮件发送器发出验证邮件，并在响应中返回 `remaining_attempts`，前端可用以刷新 UI 提示。
- 统一邮箱验证频控与验证码限流，实现 Redis/内存双实现的 `ratelimit.Limiter`，支持 `EMAIL_VERIFICATION_LIMIT`、`EMAIL_VERIFICATION_WINDOW` 自定义阈值。
- 邮箱验证与图形验证码接口返回剩余尝试次数（`remaining_attempts`），被限流时附带冷却秒数（`retry_after_seconds`），便于前端展示剩余机会与等待时间。
- 邮件发送新增阿里云 DirectMail 发信器，优先使用 DirectMail，未配置时自动回退到 SMTP。
- 新增 Prompt 导出能力，调用 `POST /api/prompts/export` 会生成包含全部 Prompt 的 JSON 文件，并在响应中返回本地保存路径，目录通过 `PROMPT_EXPORT_DIR` 配置。
- 在线模式新增 DeepSeek 免费额度：未配置模型凭据的用户会自动使用内置密钥完成解析/生成，每日调用次数默认 10 次，可通过 `PROMPT_FREE_TIER_*` 环境变量调整，超过额度将返回 429 提示用户改用自有模型。
- Prompt 分享串上线：`POST /api/prompts/:id/share` 生成以 `PGSHARE-` 开头的分享文本，可复制到任意 IM/邮件；`POST /api/prompts/share/import` 粘贴分享串即可在当前账户下创建草稿，相关长度与前缀分别由 `PROMPT_SHARE_MAX_BYTES`、`PROMPT_SHARE_PREFIX` 控制。
- Prompt 历史版本接口可用：新增 `GET /api/prompts/:id/versions` 与 `GET /api/prompts/:id/versions/:version`，支持查看历史版本详情，保留数量由 `PROMPT_VERSION_KEEP_LIMIT` 控制。
- 启动流程拆分为 `internal/app.InitResources`（负责连接/迁移）与 `internal/bootstrap.BuildApplication`（负责装配依赖），提升职责清晰度。
- 新增 `/api/models` 系列接口，支持模型凭据的创建、查看、更新与删除，API Key 会在入库前加密。
- 引入 `MODEL_CREDENTIAL_MASTER_KEY` 环境变量，使用 AES-256-GCM 加解密用户提交的模型凭据。
- 新增 `POST /api/prompts/import`，可上传导出的 JSON 文件并选择“合并/覆盖”模式批量回灌 Prompt，导入批大小由 `PROMPT_IMPORT_BATCH_SIZE` 控制。
- “我的 Prompt” 支持收藏：新增 `PATCH /api/prompts/:id/favorite` 切换收藏状态，`GET /api/prompts` 返回 `is_favorited` 字段并支持 `favorited=true` 筛选列表。
- “我的 Prompt” 支持点赞：新增 `POST /api/prompts/:id/like` 与 `DELETE /api/prompts/:id/like`，响应透出 `like_count` 与 `is_liked`，方便前端展示点赞态与热度。
- Prompt 评论模块上线：新增 `prompt_comments` 表与 `/api/prompts/:id/comments`、`/api/prompts/comments/:id/review` 等接口，支持楼中楼回复、分页查询与自动审核；管理员可通过 `DELETE /api/prompts/comments/:id` 直接移除整条讨论线程。
- 公共 Prompt 库上线：新增 `/api/public-prompts` 列表、详情与下载接口；投稿仅在在线模式开放，离线模式默认只读，避免本地环境误提交。
  - 点赞串联逻辑：公共库的点赞只是代理到源 Prompt —— handler 会调用 service 的 `Like/Unlike`，再由 `PromptRepository.AddLike/RemoveLike` 写入 `prompt_likes` 表，并通过 `IncrementLikeCount` 同步 `prompts.like_count`，随后用 `LikeSnapshot` 返回最新的 `like_count` 与 `is_liked` 方便前端立即刷新。
    1. 前端点击心形按钮后，会向 `/api/public-prompts/:id/like`（点赞）或 `DELETE /api/public-prompts/:id/like`（取消）发起请求。
    2. `PublicPromptHandler.handleLike` 负责解析公共 Prompt ID 与登录用户 ID，随后调用 `Service.Like/Unlike`。
    3. Service 层先查公共 Prompt 绑定的源 Prompt 编号，确保点赞动作落在真实的 Prompt 数据上。
    4. 找到源 Prompt 后，调用 `PromptRepository.AddLike/RemoveLike` 对 `prompt_likes` 表增删记录，并用 `IncrementLikeCount` 更新 `prompts.like_count`。
    5. 最后执行 `LikeSnapshot`，一次性读取“总点赞数 + 当前用户是否点赞”，填回到公共 Prompt 的 `LikeCount/IsLiked` 字段，HTTP 响应即可直接携带最新状态返回给前端。
- 公共 Prompt 访问量统计改用 Redis 缓冲 + 后台刷库：详情页按用户 ID 设置 1 分钟去重窗口，访问增量先写入 `prompt:visit:buffer` Hash，后台协程依据 `PUBLIC_PROMPT_VISIT_*` 环境变量批量刷新 MySQL，接口会返回“已落库值 + 缓存值”，方便前端即时展示最新访问数。
- Prompt 版本号策略：首次仅保存草稿不会产生版本（`latest_version_no=0`），首次发布才会生成版本 1；发布后再保存草稿不改变版本号，下次发布时会遍历历史版本取最大值再 +1，保证序号连续递增。
- 数据库自动迁移包含 `user_model_credentials` 与 `changelog_entries` 表，服务启动即可创建所需数据结构。
- 模型凭据禁用或删除时，会自动清理用户偏好的 `preferred_model`，避免指向不可用的模型；`PUT /api/users/me` 也会验证偏好模型是否存在并已启用。
- 新增 `infra/model/deepseek` 模块与 `Service.InvokeChatCompletion`，可使用存量凭据直接向接入的大模型发起调用（当前支持 DeepSeek / 火山引擎）。
- 新增 `changelog_entries` 表和 `/api/changelog` 接口，允许管理员在线维护更新日志；普通用户可直接读取最新发布的条目。
- JWT 访问令牌新增 `is_admin` 字段，后端会在鉴权中间件里解析并注入上下文，前端可据此展示后台管理能力。
- 新增 `/api/ip-guard/bans` 黑名单管理接口，管理员可查询限流封禁的 IP 并调用 `DELETE /api/ip-guard/bans/:ip` 解除；默认从环境变量 `IP_GUARD_ADMIN_SCAN_COUNT`、`IP_GUARD_ADMIN_MAX_ENTRIES` 读取扫描批量与返回上限，避免硬编码“神秘数字”。

## 请求生命周期与并发模型
>
> 所有请求都走同一个 http.Server 和单例 Gin Router，但 net/http 会在后台为每个连接启动 goroutine，各自顺序处理该连接里的请求；整体并发、调度都由 Go 负责，无需你手动 go func()。

- 入口：`backend/cmd/server/main.go:55` 使用 `http.Server` + `ListenAndServe` 监听请求；这一步由 Go 标准库负责为每个新连接/请求开辟 goroutine，无需手动 `go func()`。
- 路由：`backend/internal/server/router.go:25` 初始化 Gin 引擎并注册中间件、路由；每条路由的 handler（例如 `backend/internal/handler/prompt_handler.go:117` 的 `GeneratePrompt`）都在独立 goroutine 中执行。
- 业务拆分：handler 将请求绑定/校验后转给 `internal/service`，例如 `PromptHandler.GeneratePrompt → prompt.Service.Generate`，在同一 goroutine 内同步调用数据库、Redis 或模型 API；阻塞时调度器会自动切换去执行其他请求的 goroutine。
- 优雅退出：当进程收到 `SIGTERM` 时，`main.go:32` 会触发 `server.Shutdown`，等待当前 goroutine 完成后再释放资源。

> 因此，一个用户发起“解析/生成 Prompt”请求时，整个链路会在同一个 goroutine 上完成 handler → service → repository → 外部调用，无需显式创建新 goroutine；Go 会在高并发场景下自动调度这些请求处理线程。

## 离线预置数据导出与校验

离线安装包会在首次启动时读取 `backend/data/bootstrap/` 下的 JSON，并通过 `bootstrapdata.SeedLocalDatabase` 向空数据库导入公共 Prompt 与更新日志。为了确保桌面端安装包内置最新内容，请在每次发布前按以下步骤更新并校验预置数据：

1. **准备环境变量**：保证 `.env.local` 中的 MySQL、Redis 等在线资源配置可用，`LOCAL_BOOTSTRAP_DATA_DIR` 若为空将默认写入 `backend/data/bootstrap`。
2. **导出最新数据**：

   ```bash
   go run ./backend/cmd/export-offline-data -output-dir backend/data/bootstrap
   ```

   命令会根据环境配置查询线上库，并生成 `public_prompts.json`、`changelog_entries.json`。由于 `.gitignore` 已放行该目录，生成后的文件会显示在 `git status` 中，注意一并提交。
3. **本地校验（可选）**：

   ```bash
   go run ./backend/cmd/offline-bootstrap -output ./release/assets/promptgen-offline.db
   ```

   该命令会加载刚刚导出的 JSON，将其写入临时 SQLite 并输出统计信息，便于确认条目数量是否符合预期。若未指定 `-output`，CLI 会写入 `LOCAL_SQLITE_PATH` 指向的位置。
4. **打包验证**：运行 `npm run dist:win` / `npm run dist:mac` 生成安装包后，解压安装目录确认 `resources/app/backend/data/bootstrap/*.json` 是否存在，避免遗漏。

> 以上流程仅更新 JSON 数据，不会影响 .env 或其他配置；请勿将 `.env.local`、真实凭据等敏感文件提交到仓库。

## 环境变量

### 运行必需

| 变量 | 作用 |
| --- | --- |
| `SERVER_PORT` | HTTP 服务监听端口，默认 `9090` |
| `JWT_SECRET` | JWT 签名密钥，必填 |
| `JWT_ACCESS_TTL` | 访问令牌有效期，如 `15m` |
| `JWT_REFRESH_TTL` | 刷新令牌有效期，如 `168h` |
| `NACOS_ENDPOINT` | Nacos 地址，形如 `ip:port`（仅在继续使用配置中心时需要） |
| `NACOS_USERNAME` / `NACOS_PASSWORD` | Nacos 登录凭证 |
| `NACOS_GROUP` / `NACOS_NAMESPACE` | Nacos 配置读取时使用的分组与命名空间 |
| `MYSQL_HOST` / `MYSQL_PORT` | MySQL 主机与端口（默认 `3306`） |
| `MYSQL_USERNAME` / `MYSQL_PASSWORD` | MySQL 连接账号与密码 |
| `MYSQL_DATABASE` | 默认数据库名，未填时为 `prompt` |
| `MYSQL_PARAMS` | 追加在 DSN 末尾的参数，默认 `charset=utf8mb4&parseTime=true&loc=Local` |
| `MODEL_CREDENTIAL_MASTER_KEY` | 32 字节主密钥（需使用 Base64 编码后写入），用于加解密模型 API Key |

### 本地离线模式

- 设置 `APP_MODE=local` 即可启用离线模式，后端将自动跳过 MySQL、Redis、Nacos 连接并使用本地 SQLite 文件。
- SQLite 路径通过 `LOCAL_SQLITE_PATH` 控制，支持 `~` 前缀，默认值写入 `data/promptgen-local.db`；首次启动会自动创建目录与数据库。
- `LOCAL_USER_ID`、`LOCAL_USER_USERNAME`、`LOCAL_USER_EMAIL`、`LOCAL_USER_ADMIN` 控制离线模式下的默认账号信息；后端会在启动时写入/更新该账号，并在没有 JWT 的情况下直接注入该用户身份。
- 可通过 `LOCAL_BOOTSTRAP_DATA_DIR` 指定预置数据目录，默认读取 `backend/data/bootstrap`，表为空时会自动导入公共 Prompt 与更新日志示例。
- 离线模式仍然需要 `MODEL_CREDENTIAL_MASTER_KEY` 用于模型凭据加密；未配置 `JWT_SECRET` 时会自动回退到内置的本地密钥，避免开发时额外填写。

#### 离线模式快速上手

1. 在根目录复制 `.env.example` 为 `.env.local`，修改以下字段：
   - `APP_MODE=local`
   - `LOCAL_SQLITE_PATH=~/promptgen/promptgen-local.db`（按需调整路径）
   - 如需管理员权限，可将 `LOCAL_USER_ADMIN=true`
2. 启动后端：`go run ./backend/cmd/server`
3. 启动前端：`npm run dev:frontend`（Electron 启动流程不变）
4. 登录页点击“离线模式”按钮即可直接进入工作台；此时所有数据仅保存在上一步配置的 SQLite 文件中，邮箱验证等依赖在线服务的功能会自动禁用。
5. 若要恢复在线模式，将 `APP_MODE` 改回 `online` 并按照原有方式配置数据库/Redis/Nacos。
6. 需要在 CI 或本地生成离线数据库时，可执行 `go run ./backend/cmd/offline-bootstrap -output ./release/assets/promptgen-offline.db`，命令会依据 `LOCAL_BOOTSTRAP_DATA_DIR` 自动导入示例数据。

#### 离线数据更新

- 当需要同步线上 MySQL 中的最新公共 Prompt、更新日志时，运行 `go run ./backend/cmd/export-offline-data -output-dir backend/data/bootstrap`。命令会根据环境变量连接数据库，并写入同名 JSON 文件，后续打包可直接复用。
- 导出完成后再执行 `go run ./backend/cmd/offline-bootstrap -output ./release/assets/promptgen-offline.db` 生成最新的 SQLite，确保离线安装包携带最新数据。
- 客户端启动时会按 `badge+locale`、`author_user_id+topic` 的组合键增量合并 JSON 数据，不会覆盖已有本地条目，仅插入或更新新版本内容。

### 管理员指标缓存

| 变量 | 作用 |
| --- | --- |
| `ADMIN_METRICS_REFRESH_INTERVAL` | 后台指标快照的刷新周期（`time.Duration` 格式），默认 `5m` |
| `ADMIN_METRICS_RETENTION_DAYS` | 内存缓存按日保留的天数，默认 `7` |

> 若未配置上述变量，服务会使用默认值；将间隔缩短可提升实时性，但需评估数据库压力。

### 管理员用户总览

| 变量 | 作用 |
| --- | --- |
| `ADMIN_USER_PAGE_SIZE` | `/api/admin/users` 默认每页返回的用户数量，默认 `20` |
| `ADMIN_USER_PAGE_SIZE_MAX` | 单页允许的最大数量，超出请求会被截断，默认 `100` |
| `ADMIN_USER_RECENT_PROMPT_LIMIT` | 每名用户返回的“最近 Prompt”条数，默认 `3` |
| `ADMIN_USER_ONLINE_THRESHOLD` | 计算在线状态时允许的最近登录阈值，例如 `15m` |

接口 `GET /api/admin/users` 仅限管理员调用，支持 `page`、`page_size`、`query` 查询参数（模糊匹配用户名 / 邮箱）。返回结构示例：

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": 1,
        "username": "alice",
        "email": "alice@example.com",
        "avatar_url": "/static/avatars/xx.png",
        "is_admin": true,
        "last_login_at": "2025-11-05T12:00:00Z",
        "created_at": "2025-10-01T08:00:00Z",
        "updated_at": "2025-11-05T11:59:00Z",
        "is_online": true,
        "prompt_totals": {
          "total": 12,
          "draft": 4,
          "published": 6,
          "archived": 2
        },
        "latest_prompt_at": "2025-11-05T11:55:00Z",
        "recent_prompts": [
          {
            "id": 88,
            "topic": "产品发布稿",
            "status": "published",
            "updated_at": "2025-11-05T11:55:00Z",
            "created_at": "2025-11-03T09:30:00Z"
          }
        ]
      }
    ],
    "total": 36,
    "page": 1,
    "page_size": 20,
    "online_threshold_seconds": 900
  }
}
```

`is_online` 由 `last_login_at` 与 `ADMIN_USER_ONLINE_THRESHOLD` 比较得出；`recent_prompts` 会按照更新时间倒序返回最近活动，协助快速定位活跃用户的内容。

### 验证码与 Redis

| 变量 | 作用 |
| --- | --- |
| `REDIS_ENDPOINT` | Redis 地址，启用验证码时必填 |
| `REDIS_PASSWORD` / `REDIS_DB` | Redis 认证和 DB，下划线留空即可走默认值 |
| `CAPTCHA_ENABLED` | 设置为 `1` 时开启验证码能力 |
| `CAPTCHA_PREFIX` | Redis Key 前缀，默认 `captcha` |
| `CAPTCHA_TTL` | 验证码有效期，例如 `5m` |
| `CAPTCHA_LENGTH` | 验证码长度 |
| `CAPTCHA_WIDTH` / `CAPTCHA_HEIGHT` | 图片尺寸 |
| `CAPTCHA_MAX_SKEW` / `CAPTCHA_DOT_COUNT` | 图片扭曲、噪点参数 |
| `CAPTCHA_RATE_LIMIT_PER_MIN` | 每个 IP 在窗口内允许请求次数 |
| `CAPTCHA_RATE_LIMIT_WINDOW` | 限流窗口时长，例如 `1m` |

> 验证码配置全部从 `.env(.local)` 读取，取消了对 Nacos 的依赖；若需修改行为，请直接调整环境变量并重启服务即可生效。

### 邮箱验证与邮件发送

| 变量 | 作用 |
| --- | --- |
| `APP_PUBLIC_BASE_URL` | 前端公共地址，用于拼接验证链接，例如 `https://app.example.com` |
| `EMAIL_VERIFICATION_LIMIT` | 邮箱验证重发限频次数，默认 `5` |
| `EMAIL_VERIFICATION_WINDOW` | 邮箱验证限频窗口时长，默认 `1h` |

### Prompt 工作台限流（可选）

| 变量 | 作用 |
| --- | --- |
| `PROMPT_INTERPRET_LIMIT` | 自然语言解析接口限流次数，默认 `8` |
| `PROMPT_INTERPRET_WINDOW` | 自然语言解析限流窗口，如 `60s`，默认 `1m` |
| `PROMPT_GENERATE_LIMIT` | Prompt 生成接口限流次数，默认 `5` |
| `PROMPT_GENERATE_WINDOW` | Prompt 生成限流窗口，默认 `1m` |
| `PROMPT_SAVE_LIMIT` | 保存/发布接口限流次数，默认 `20` |
| `PROMPT_SAVE_WINDOW` | 保存/发布限流窗口，默认 `1m` |
| `PROMPT_PUBLISH_LIMIT` | 发布操作限流次数（仅在 `publish=true` 时计数），默认 `6` |
| `PROMPT_PUBLISH_WINDOW` | 发布限流窗口，默认 `10m` |
| `PROMPT_KEYWORD_LIMIT` | 正向/负向关键词的数量上限，默认 `10`，需与前端 `VITE_PROMPT_KEYWORD_LIMIT` 保持一致 |
| `PROMPT_KEYWORD_MAX_LENGTH` | 单个关键词允许的最大字符数（按 Unicode 码点计），默认 `32` |
| `PROMPT_TAG_LIMIT` | 标签数量上限，默认 `3`，需与前端 `VITE_PROMPT_TAG_LIMIT` 保持一致 |
| `PROMPT_TAG_MAX_LENGTH` | 单个标签允许的最大字符数，默认 `5` |
| `PROMPT_LIST_PAGE_SIZE` | “我的 Prompt”列表默认每页数量，默认 `10` |
| `PROMPT_LIST_MAX_PAGE_SIZE` | “我的 Prompt”列表单页最大数量，默认 `100` |
| `PROMPT_USE_FULLTEXT` | 设置为 `1` 时使用 FULLTEXT 检索（需提前创建 `ft_prompts_topic_tags` 索引） |
| `PROMPT_IMPORT_BATCH_SIZE` | 导入 Prompt 时单批处理的最大条数，默认 `20` |
| `PROMPT_AUDIT_ENABLED` | 是否启用内置的模型内容审核，在线模式默认开启 |
| `PROMPT_AUDIT_PROVIDER` | 审核模型提供方，当前支持 `deepseek` |
| `PROMPT_AUDIT_MODEL_KEY` | 调用审核时使用的模型标识，缺省时回退到 `deepseek-chat` |
| `PROMPT_AUDIT_API_KEY` | 审核模型所需的 API Key（在线模式必须配置） |
| `PROMPT_AUDIT_BASE_URL` | 审核模型的自定义接口地址，留空使用默认值 |
| `PROMPT_FREE_TIER_ENABLED` | 是否启用内置免费体验模型，默认 `1`（仅在线模式有效） |
| `PROMPT_FREE_TIER_PROVIDER` | 体验模型提供方标识，默认 `deepseek` |
| `PROMPT_FREE_TIER_MODEL_KEY` | 前端可见的模型键别名，默认 `deepseek` |
| `PROMPT_FREE_TIER_ACTUAL_MODEL` | 请求实际使用的模型标识，默认 `deepseek-chat` |
| `PROMPT_FREE_TIER_DISPLAY_NAME` | 前端展示名称，默认 “DeepSeek 免费体验” |
| `PROMPT_FREE_TIER_API_KEY` | 内置模型的 API Key，留空时回退到 `PROMPT_AUDIT_API_KEY` |
| `PROMPT_FREE_TIER_BASE_URL` | 内置模型的 Base URL，可选 |
| `PROMPT_FREE_TIER_DAILY_LIMIT` | 每位用户每日免费额度，默认 `10` 次 |
| `PROMPT_FREE_TIER_WINDOW` | 免费额度计数窗口，默认 `24h` |

> 在线模式下只要配置了 `PROMPT_AUDIT_API_KEY`，Prompt 服务会自动切换到内置 DeepSeek 审核器；本地模式始终跳过审核，便于开发调试。
> ❗ **排障提示**：如果日志中出现  
> `decode interpretation response: json: cannot unmarshal array into Go struct`，说明模型把 `instructions` 字段生成为数组。现有实现已兼容数组与字符串两种格式；若自定义提示词，请确保仍返回 JSON 对象，并将补充要求放在 `instructions` 字段（字符串或字符串数组均可）。

### DeepSeek 免费额度流程

1. **配置加载**：启动时 `loadPromptConfig` 读取 `PROMPT_FREE_TIER_*` 环境变量，并在 `promptsvc.NewServiceWithConfig` 中构建一个内置的 DeepSeek 客户端（`freeTier`）。
2. **调用链兜底**：Prompt Service 在 Interpret / Augment / Generate 三条链路里优先使用用户自配模型；若凭据缺失或被禁用，则透明回退到 `freeTier.invoke`，同时按照 Redis（在线）或内存限流（离线）扣减每日免费额度。
3. **额度耗尽反馈**：当免费额度被用完时，Service 会抛出 `FreeTierQuotaExceededError`，Handler 统一返回 `429`，并在 `details.retry_after_seconds` / `details.remaining` 告知前端剩余冷却时间与余量。
4. **前端展示**：后端不会为内置模型写入数据库，而是通过 `builtinModels` 将“DeepSeek 免费体验”作为只读条目拼接到 `/api/models` 响应中，前端设置页据此显示每日免费次数并禁用编辑/删除按钮。
5. **安全考虑**：内置模型使用的平台级 API Key 仅存在于服务端内存和调用路径中；任何列表或接口都不会回传密钥，前端只能看到基础描述信息。

> ★ 没有新增 API。现有 `/api/models`、`/api/prompts/interpret`、`/api/prompts/generate`、`/api/prompts/keywords/augment` 等接口保持不变，只是在后端增加了上述兜底逻辑与 429 错误码分支。

### 公共 Prompt 库限流（可选）

| 变量 | 作用 |
| --- | --- |
| `PUBLIC_PROMPT_LIST_PAGE_SIZE` | 公共库列表默认每页数量，默认 `9` |
| `PUBLIC_PROMPT_LIST_MAX_PAGE_SIZE` | 公共库列表单页最大数量，默认 `60` |
| `PUBLIC_PROMPT_SUBMIT_LIMIT` | 投稿接口限流次数，默认 `5` |
| `PUBLIC_PROMPT_SUBMIT_WINDOW` | 投稿限流窗口，默认 `30m` |
| `PUBLIC_PROMPT_DOWNLOAD_LIMIT` | 下载接口限流次数，默认 `30` |
| `PUBLIC_PROMPT_DOWNLOAD_WINDOW` | 下载限流窗口，默认 `1h` |

### 公共 Prompt 访问统计（仅在线模式）

| 变量 | 作用 |
| --- | --- |
| `PUBLIC_PROMPT_VISIT_ENABLED` | 是否开启访问量统计（默认开启，在线模式才生效） |
| `PUBLIC_PROMPT_VISIT_BUFFER_KEY` | Redis Hash 缓冲键，默认 `prompt:visit:buffer`，按 Prompt ID 累加访问增量 |
| `PUBLIC_PROMPT_VISIT_GUARD_PREFIX` / `PUBLIC_PROMPT_VISIT_GUARD_TTL` | 去重键前缀与 TTL，默认按用户 ID 1 分钟内仅计一次访问 |
| `PUBLIC_PROMPT_VISIT_FLUSH_INTERVAL` | 后台协程刷新的间隔，默认 `1m` |
| `PUBLIC_PROMPT_VISIT_FLUSH_BATCH` | 单次刷库处理的最大 Prompt 数，默认 `128` |
| `PUBLIC_PROMPT_VISIT_FLUSH_LOCK_KEY` / `PUBLIC_PROMPT_VISIT_FLUSH_LOCK_TTL` | 分布式锁键名与有效期，防止多实例同时刷新 |

> 访问量刷新流程：详情接口命中后先设置去重键，再将增量写入 Redis 缓冲 Hash；后台协程每隔 `PUBLIC_PROMPT_VISIT_FLUSH_INTERVAL` 读取 Hash，将增量累加到 `prompts.visit_count` 并移除已处理字段。HTTP 响应会返回“落库值 + 缓存增量”，确保前端立即看到最新访问次数。

访问统计的整体节奏如下：

1. **读取访问量**：接口在返回公共 Prompt 数据之前，会先查询 MySQL 中的 `prompts.visit_count`，再从 Redis 缓冲 `prompt:visit:buffer` 读取尚未落库的增量，两个值相加后回传给前端。
2. **记录本次访问**：详情接口校验用户级去重键（默认 1 分钟），通过后优先调用 `HINCRBY prompt:visit:buffer <promptID> 1` 写入 Redis；若 Redis 不可用，则回退到直接更新 MySQL。
3. **批量落库**：后台协程依据 `PUBLIC_PROMPT_VISIT_FLUSH_INTERVAL` 定期触发 `flushVisitBuffer`，抢占分布式锁后批量读取缓冲增量，加到 `prompts.visit_count` 并 `HDEL` 清理已处理字段，从而保证 Redis 与 MySQL 数据一致。

> 这样设计的好处是：读取只做一次简单查询，写入则通过 Redis 做缓冲和去重，把大量的单次 `UPDATE` 合并成定时批量更新，既保证访问统计即时准确，又显著降低 MySQL 写压力。

### IP 防护（可选）

| 变量 | 作用 |
| --- | --- |
| `IP_GUARD_ENABLED` | 是否启用 IP 限流与黑名单（`1/true` 开启，`0/false` 关闭） |
| `IP_GUARD_MAX_REQUESTS` | 单个 IP 在同一个窗口内允许的最大请求数，超过即计一次“违规”，默认 `120` |
| `IP_GUARD_WINDOW` | 请求计数的窗口时长，默认 `30s` |
| `IP_GUARD_STRIKE_LIMIT` | 在 `IP_GUARD_STRIKE_WINDOW` 内累计多少次违规会触发封禁，默认 `5` |
| `IP_GUARD_STRIKE_WINDOW` | 统计违规次数的窗口时长，默认 `10m` |
| `IP_GUARD_BAN_TTL` | 被封禁后黑名单的持续时间，默认 `30m` |
| `IP_GUARD_HONEYPOT_PATH` | 蜜罐接口的相对路径（挂载在 `/api` 下，无需带前导 `/`），默认 `__internal__/trace` |

> 蜜罐接口会在前端以隐藏链接形式暴露，一旦被访问会立刻拉黑触发方；若需要解除封禁，可在 Redis 中删除 `ipguard:ban:<ip>`。
> 默认配置会注册 `/api/__internal__/trace`，如需调整只需改写 `IP_GUARD_HONEYPOT_PATH`（无需带前导 `/`）。

#### SMTP 发信（可选）

| 变量 | 作用 |
| --- | --- |
| `SMTP_HOST` / `SMTP_PORT` | SMTP 服务地址与端口 |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | SMTP 登录凭证（匿名时可留空用户名/密码） |
| `SMTP_FROM` | 发信人邮箱地址 |

#### 阿里云 DirectMail（可选）

| 变量 | 作用 |
| --- | --- |
| `ALIYUN_DM_ACCESS_KEY_ID` / `ALIYUN_DM_ACCESS_KEY_SECRET` | 阿里云访问密钥对 |
| `ALIYUN_DM_REGION_ID` | DirectMail 实例所在区域，例如 `cn-hangzhou` |
| `ALIYUN_DM_ENDPOINT` | DirectMail API 访问域名，默认 `dm.aliyuncs.com` |
| `ALIYUN_DM_ACCOUNT_NAME` | 管理控制台配置的发信地址（AccountName） |
| `ALIYUN_DM_FROM_ALIAS` | 发信人别名，可选 |
| `ALIYUN_DM_TAG_NAME` | 邮件标签，可选 |
| `ALIYUN_DM_REPLY_TO_ADDRESS` | 是否启用回信地址（`true` / `false`） |
| `ALIYUN_DM_ADDRESS_TYPE` | 发信地址类型（`0` 随机账号，`1` 独立账号） |

DirectMail 环境变量齐全时，`initEmailSender` 会优先实例化阿里云客户端；否则退回 SMTP 发信器，再不满足则仅写日志。

在仓库根目录中复制 `.env.example` 为 `.env.local` 填入真实值，服务启动时会自动加载。

> **提示：** 刷新令牌默认存入 Redis；若未配置 Redis，则退化为进程内内存存储，适合开发环境，但服务重启后刷新令牌会全部失效。

```powershell
Copy-Item ..\..\.env.example ..\..\.env.local -Force
```

## 邮件发送调试

阿里云 DirectMail 在本地开发时可以使用辅助命令快速发信确认配置：

```powershell
go run ./backend/cmd/sendmail -to you@example.com -name "测试账号"
```

命令会加载 `.env.local`，构造一封验证邮件发送至指定地址，并在终端打印生成的 token。可配合前端验证页面进行手动测试。

## 跨域访问（CORS）

### 为什么需要它？

浏览器有一道“同源策略”安全限制：页面只能访问和自己协议、域名、端口完全相同的接口。我们的场景里存在多个“来源”：

1. `npm run dev` 启动的 Vite 页面对外地址是 `http://localhost:5173`。
2. `npm run preview` 会在 `http://localhost:4173` 启动模拟生产的静态服务器。
3. Electron 开发模式下，主窗口直接载入 Vite 地址；打包后则会以 `file://...` 形式打开本地 `dist/index.html`。此时浏览器发送的 `Origin` 会变成 `null`。

后端监听在 `http://localhost:9090`，上述来源在访问 `/api/...` 时都属于“跨域”。如果没有额外配置，浏览器会拒绝这些请求，表现为前端收到 `CORS error` 或直接失败。

### 项目中的处理方式

我们在 `internal/server/router.go` 中启用了 [CORS（Cross-Origin Resource Sharing）](https://developer.mozilla.org/zh-CN/docs/Web/HTTP/CORS) 中间件，并允许以下来源访问：

- `http://localhost:<任何端口>`（覆盖 5173、4173 及其他本地调试端口）
- `http://127.0.0.1:<任何端口>`（同上，只是访问方式不同）
- `null`（Electron 以 `file://` 打开页面时浏览器带上的特殊值）

中间件会自动添加 `Access-Control-Allow-Origin`、`Access-Control-Allow-Methods` 等响应头，从而告诉浏览器“这类跨域请求是允许的”。如需联调更多域名，只需扩展 `AllowOriginFunc` 或改为读取环境变量即可。

## 对外 API

> 后端 HTTP 服务默认监听 `SERVER_PORT`（缺省值 `9090`）。下表列出的路径均需拼接在 `http://<host>:<SERVER_PORT>` 之后，例如本地调试常用的 `http://localhost:9090/api/...`。

| 方法 | 路径 | 描述 | 请求参数 |
| --- | --- | --- | --- |
| `GET` | `/api/auth/captcha` | 获取图形验证码 | 无；按客户端 IP 控制限流 |
| `POST` | `/api/auth/register` | 用户注册 | JSON：`username`、`email`、`password`、`avatar_url`、`captcha_id`、`captcha_code`（验证码开启时必填） |
| `POST` | `/api/auth/login` | 用户登录 | JSON：`identifier`（邮箱或用户名）、`password` |
| `POST` | `/api/auth/verify-email/request` | 重新发送邮箱验证令牌 | JSON：`email` |
| `POST` | `/api/auth/verify-email/confirm` | 使用 token 完成邮箱验证 | JSON：`token` |
| `POST` | `/api/auth/refresh` | 刷新访问令牌 | JSON：`refresh_token` |
| `POST` | `/api/auth/logout` | 撤销刷新令牌 | JSON：`refresh_token` |
| `GET` | `/api/users/me` | 获取当前登录用户信息 | 需附带 `Authorization: Bearer <token>` |
| `PUT` | `/api/users/me` | 更新当前用户信息与偏好设置 | JSON：`username`、`email`、`avatar_url`、`preferred_model`、`enable_animations` |
| `POST` | `/api/uploads/avatar` | 上传头像文件并返回静态地址 | 无需登录；multipart 表单：`avatar` 文件字段 |
| `GET` | `/api/models` | 列出当前用户的模型凭据 | 需登录 |
| `POST` | `/api/models` | 新增模型凭据并加密存储 | JSON：`provider`、`label`、`api_key`、`metadata` |
| `PUT` | `/api/models/:id` | 更新模型凭据（可替换 API Key） | JSON：`label`、`api_key`、`metadata` |
| `DELETE` | `/api/models/:id` | 删除模型凭据 | 无 |
| `POST` | `/api/prompts/interpret` | 自然语言解析主题与关键词 | JSON：`description`、`model_key`、`language` |
| `POST` | `/api/prompts/ingest` | 粘贴成品 Prompt 并生成草稿 | JSON：`body`、`model_key`（可选）、`language`（可选） |
| `POST` | `/api/prompts/keywords/augment` | 补充关键词并去重 | JSON：`topic`、`model_key`、`existing_positive[]`、`existing_negative[]`、`workspace_token`（可选） |
| `POST` | `/api/prompts/keywords/manual` | 手动新增关键词并落库 | JSON：`topic`、`word`、`polarity`、`weight`（可选，默认 5）、`prompt_id`（可选）、`workspace_token`（可选） |
| `POST` | `/api/prompts/keywords/remove` | 从工作区移除关键词 | JSON：`word`、`polarity`、`workspace_token` |
| `POST` | `/api/prompts/keywords/sync` | 同步排序与权重到工作区 | JSON：`workspace_token`、`positive_keywords[]`、`negative_keywords[]`（元素含 `word`、`polarity`、`weight`） |
| `GET` | `/api/prompts` | 获取当前用户的 Prompt 列表 | Query：`status`（可选，draft/published）、`q`（模糊搜索 topic/tags）、`page`、`page_size`、`favorited`（可选，true/1 表示仅展示收藏项） |
| `PATCH` | `/api/prompts/:id/favorite` | 收藏或取消收藏 Prompt | JSON：`favorited`（布尔值） |
| `POST` | `/api/prompts/:id/like` | 点赞 Prompt 并返回最新计数 | 无 |
| `DELETE` | `/api/prompts/:id/like` | 取消点赞 Prompt | 无 |
| `POST` | `/api/prompts/export` | 导出当前用户的 Prompt 并返回本地保存路径 | 无 |
| `POST` | `/api/prompts/import` | 导入导出的 Prompt JSON（支持合并/覆盖模式） | multipart：`file`（JSON 文件）、`mode`（可选，merge/overwrite）；或直接提交 JSON 正文 |
| `POST` | `/api/prompts/:id/share` | 生成 `PGSHARE-` 分享串 | 路径参数 `id`；无需请求体 |
| `POST` | `/api/prompts/share/import` | 粘贴分享串并创建草稿 | JSON：`payload`（`PGSHARE-` 文本） |
| `GET` | `/api/prompts/:id` | 获取单条 Prompt 详情并返回最新工作区 token | 无 |
| `GET` | `/api/prompts/:id/versions` | 列出指定 Prompt 的历史版本 | Query：`limit`（可选，默认保留配置中的数量） |
| `GET` | `/api/prompts/:id/versions/:version` | 获取指定版本的完整内容 | 无 |
| `POST` | `/api/prompts/generate` | 调模型生成 Prompt 正文 | JSON：`topic`、`model_key`、`positive_keywords[]`、`negative_keywords[]`、`workspace_token`（可选） |
| `POST` | `/api/prompts` | 保存草稿或发布 Prompt | JSON：`prompt_id`、`topic`、`body`、`status`、`publish`、`positive_keywords[]`、`negative_keywords[]`、`workspace_token`（可选） |
| `DELETE` | `/api/prompts/:id` | 删除指定 Prompt 及其历史版本/关键词关联 | 无 |
| `GET` | `/api/prompts/:id/comments` | 查询 Prompt 评论（含楼中楼） | Query：`page`、`page_size`、`status`（管理员可选 `all/pending/rejected`），需登录；响应项含 `like_count`、`is_liked` |
| `POST` | `/api/prompts/:id/comments` | 新增评论或回复 | JSON：`body`、`parent_id`（可选），需登录；写库前会执行内容审核 |
| `POST` | `/api/prompts/comments/:id/like` | 点赞指定评论 | 无额外参数，需登录；仅允许对已审核通过的评论点赞 |
| `DELETE` | `/api/prompts/comments/:id/like` | 取消点赞指定评论 | 无额外参数，需登录 |
| `POST` | `/api/prompts/comments/:id/review` | 审核评论（管理员） | JSON：`status`（`approved/rejected/pending`）、`note`（可选），需管理员权限 |
| `DELETE` | `/api/prompts/comments/:id` | 删除评论（管理员） | 级联移除评论及其子回复，需管理员权限 |
| `GET` | `/api/admin/metrics` | 管理员指标快照 | 无额外参数；需管理员权限，返回按 `ADMIN_METRICS_*` 配置生成的缓存 |
| `GET` | `/api/admin/users` | 管理员用户总览 | Query：`page`、`page_size`、`query`（可选），需管理员权限 |
| `GET` | `/api/changelog` | 获取更新日志列表 | Query：`locale`（可选，默认 `en`） |
| `POST` | `/api/changelog` | 新增更新日志（管理员） | JSON：`locale`、`badge`、`title`、`summary`、`items[]`、`published_at` |
| `PUT` | `/api/changelog/:id` | 编辑指定日志（管理员） | 同 `POST` |
| `DELETE` | `/api/changelog/:id` | 删除指定日志（管理员） | 无 |
| `POST` | `/api/public-prompts/:id/like` | 为公共 Prompt 点赞 | 无 |
| `DELETE` | `/api/public-prompts/:id/like` | 取消公共 Prompt 点赞 | 无 |
| `GET` | `/api/ip-guard/bans` | 查询仍在封禁期内的 IP 列表（管理员） | 无；需登录且具备 `is_admin=true` |
| `DELETE` | `/api/ip-guard/bans/:ip` | 解除指定 IP 的封禁记录（管理员） | 路径参数 `ip`，需登录且具备 `is_admin=true` |

> 注：出于本地开发便利，来自 127.0.0.1 / ::1 的回环地址会跳过 IP Guard 限制，不计入限流与封禁。

### Prompt 表索引优化

`GET /api/prompts` 默认按照 `user_id`、`status` 过滤并以 `updated_at DESC` 排序，同时支持主题/标签模糊搜索。推荐在 MySQL 中追加以下索引以提升查询效率：

```sql
-- 用户+状态+更新时间的复合索引，可覆盖分页排序场景。
ALTER TABLE prompts
  ADD INDEX idx_prompts_user_status_updated (user_id, status, updated_at DESC);

-- 若启用 MATCH ... AGAINST 搜索，可额外创建全文索引。
ALTER TABLE prompts
  ADD FULLTEXT INDEX ft_prompts_topic_tags (topic, tags);
```

> 当前仓储实现仍使用 `LIKE` 模糊查询；如需利用 FULLTEXT，请将查询改为 `MATCH(topic, tags) AGAINST (? IN BOOLEAN MODE)` 或建立专门的检索服务。

### 更新日志存储与管理

- `changelog_entries` 用于记录前端展示的 Release Notes，关键字段包括 `locale`（语言）、`badge`（标签）、`items`（高亮信息 JSON 数组）、`published_at`（发布日期）。  
- 接口会根据 `locale` 排序返回最新条目，若指定语言暂未录入数据，会自动回退使用英文内容。  
- 管理员身份通过用户表中的 `is_admin` 字段判定；JWT 会携带该布尔值，鉴权中间件会将其写入 `Gin Context` 供 Handler 使用。  
- 创建日志时可传入 `translate_to`（字符串数组）与 `translation_model_key` 字段，后端会调用管理员名下的大模型凭据完成自动翻译，并为每个目标语言生成额外的 changelog 记录。支持 DeepSeek（如 `deepseek-chat`）以及火山引擎方舟模型（如 `doubao-1-5-thinking-pro-250415`）。  
- 如需手动初始化数据，可执行以下 SQL 新建表并将目标账号标记为管理员：

> 现阶段仍保留 i18n 中的静态 changelog 文案，用作空库时的兜底展示；待生产环境数据稳定后，可将这些静态条目迁移成初始 SQL 种子，再移除冗余文案。

### 模型凭据字段说明与 DeepSeek 示例

- **provider**：服务商代号，保持小写，目前限定为 `deepseek` 或 `volcengine`。  
- **model_key**：用户自定义的模型标识，需在账号范围内唯一，后端也会用它作为默认的 `model` 字段传给大模型，例如 `deepseek-chat`。  
- **display_name**：前端展示名称，可写成 `DeepSeek Chat（团队密钥）`。  
- **base_url**：可选，覆盖默认的 `https://api.deepseek.com/v1`，当你使用代理或企业网关时填写。  
- **api_key**：实际的访问密钥，后端入库前会用 `MODEL_CREDENTIAL_MASTER_KEY` 加密。  
- **extra_config**：可选 JSON，对应 DeepSeek `chat/completions` 的可选参数（如 `max_tokens`、`temperature`、`response_format` 等），未在请求体里显式提供时会自动回填。

> 当前后端已支持 DeepSeek 与火山引擎（Volcengine）；其余提供方将在后续迭代中逐步接入。

例如将 DeepSeek 官方 Demo 注册进系统，可在“新增模型”表单输入：

```json
 {
  "provider": "deepseek",
  "model_key": "deepseek-chat",
  "display_name": "DeepSeek Chat（主力）",
  "base_url": "https://api.deepseek.com/v1",
  "api_key": "sk-...替换成自己的密钥...",
  "extra_config": {
    "max_tokens": 4096,
    "temperature": 1,
    "response_format": {
      "type": "text"
    }
  }
}
```

如果需要接入火山引擎（方舟 Doubao），可以参考以下示例：

```json
{
  "provider": "volcengine",
  "model_key": "doubao-1-5-thinking-pro-250415",
  "display_name": "Volcengine Doubao",
  "base_url": "https://ark.cn-beijing.volces.com/api/v3",
  "api_key": "<替换为你的方舟 API Key>",
  "extra_config": {
    "max_tokens": 4096,
    "temperature": 1
  }
}
```

前端在工作台或设置页触发模型调用时，会通过 `InvokeChatCompletion` 流程完成以下动作：

1. 读取并校验模型凭据，确认状态为 `enabled`。  
2. 使用 AES-256-GCM 解密 API Key，并根据 `base_url` 创建 DeepSeek 客户端。  
3. 合并 `extra_config` 与本次调用显式传入的参数，将缺失字段（如 `max_tokens`）补齐。  
4. 发送 `POST {base_url}/chat/completions` 请求并返回标准结构。  
5. 若 DeepSeek 返回 `4xx/5xx`，会封装为 `deepseek.APIError`，包含 `status_code` / `type` / `code` 信息，方便上层定位问题。
6. 设置页新增了“测试连通性”操作，会调用 `POST /api/models/{id}/test`，成功后更新 `last_verified_at` 以便追踪最近一次验证时间。

响应示例：

```json
{
  "id": "7bce4a57-c144-4162-a3f9-1bde7b7f195f",
  "object": "chat.completion",
  "created": 1760181608,
  "model": "deepseek-chat",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Hello! How can I assist you today? 😊"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 11,
    "total_tokens": 21
  }
}
```

### 火山引擎（Volcengine）适配说明

- **双向兼容**：模型管理的新增 / 更新 / 连通性测试接口同时支持 `deepseek` 与 `volcengine`，`POST /api/models/:id/test` 会自动根据 provider 调用对应客户端。
- **默认配置兜底**：未显式填写 `model` 时会使用凭据里的 `model_key`，DeepSeek 默认 `deepseek-chat`，火山引擎默认 `doubao-1-5-thinking-pro-250415`；Base URL 不填写则分别落到 `https://api.deepseek.com/v1` 与 `https://ark.cn-beijing.volces.com/api/v3`。
- **响应字段映射**：方舟返回的 `service_tier` 会映射到 `system_fingerprint`，`reasoning_content` 以 `choices[*].logprobs.reasoning_content` 形式透出，其余 token 统计、原始 JSON 全量保存在 `usage` 与 `raw` 中，前端可统一渲染。
- **示例**：新增火山引擎模型时可参考上文 JSON 示例，将 `provider` 改为 `volcengine`、`api_key` 替换为实际凭据即可。
- **前端联动**：设置页模型卡片会根据 provider 自动填充常用默认值，并支持 DeepSeek/Volcengine 的“测试连通性”按钮，方便在界面上直接验证凭据是否可用。

### 静态资源与上传目录

- 服务器启动时会将 `/static/**` 映射到项目内的 `backend/public` 目录，头像上传默认写入 `backend/public/avatars`；本地/离线模式下会改写到 SQLite 同级目录的 `avatars/` 中，覆盖安装也不会丢失。
- 可以根据需要挂载到对象存储或 CDN，只需替换 `UploadHandler` 的写入逻辑并调整返回的 URL。
- 若运行在容器环境，请确保挂载该目录或改为外部存储，以免应用重启后上传内容丢失。

接口返回统一结构：`success`、`data`、`error`、`meta`。错误码见 `internal/infra/common/response.go`。

### 认证流程概览

- **注册**：首次创建账号。成功后立即返回一对 `access_token`（短期使用）和 `refresh_token`（长期续期用）。
- **登录**：提交邮箱 + 密码，返回新的 TokenPair（会覆盖旧的刷新令牌）。
- **自动续期**：当访问接口提示 401（access token 过期）时，前端调用 `/api/auth/refresh`，带上手里的 refresh token。
  - 刷新成功后会同时换发新的 access/refresh，并使旧的刷新令牌失效。
  - 如果刷新令牌已过期或被注销，接口返回 401，前端需要引导用户重新登录。
- **登出**：调用 `/api/auth/logout`，服务端删除刷新令牌记录，阻止后续续期；前端清理本地缓存。

### 接口详情

> 本节示例默认通过 `http://localhost:9090` 访问后端服务；如果修改了 `SERVER_PORT`，请将示例中的端口替换为实际值。

#### POST /api/auth/register

- **请求体**

  ```json
  {
    "username": "alice",
    "email": "alice@example.com",
    "password": "Passw0rd!",
    "captcha_id": "d2c1",
    "captcha_code": "7bz4"
  }
  ```

- **成功响应**：`201`，返回用户信息与 TokenPair。
- **常见错误**：邮箱/用户名重复（409）、验证码缺失/错误（400）。

#### GET /api/auth/captcha

- **用途**：返回图形验证码以及验证码 ID，供注册等场景校验。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "captcha_id": "...",
      "image": "data:image/png;base64,...",
      "remaining_attempts": 4
    }
  }
  ```

- **限制说明**：按客户端 IP 限流，超过阈值会返回 `429 Too Many Requests`，必要时调整 `CAPTCHA_RATE_LIMIT_PER_MIN` 与 `CAPTCHA_RATE_LIMIT_WINDOW`。
- **提示信息**：`remaining_attempts` 表示当前窗口内剩余刷新次数，被限流时响应体会在 `error.details` 中附带 `retry_after_seconds` 与 `remaining_attempts: 0`。

#### POST /api/auth/login

- **请求体**

  ```json
  {
    "email": "alice@example.com",
    "password": "Passw0rd!"
  }
  ```

- **成功响应**：`200`，返回用户信息与新的 TokenPair。
- **常见错误**：账号不存在或密码错误 → `401` + `INVALID_CREDENTIALS`。
- **邮箱未验证**：返回 `403` + `EMAIL_NOT_VERIFIED`，需先完成邮箱验证。
  - 响应还会在 `error.details.email` 中附带账号邮箱，便于前端自动填充。

#### POST /api/auth/verify-email/request

- **用途**：重新发送邮箱验证邮件/令牌，默认 24 小时内有效。
- **请求体**

  ```json
  {
    "email": "alice@example.com"
  }
  ```

- **成功响应**：`200`。开发环境会在 `data.token` 返回一次性 Token，便于测试；`data.remaining_attempts` 提示当前窗口剩余可用次数，被限流时 `error.details.retry_after_seconds` 给出冷却时间。
- **常见错误**：邮箱已通过验证 → `409` + `EMAIL_ALREADY_VERIFIED`。

#### POST /api/auth/verify-email/confirm

- **用途**：提交邮件中的一次性 token，完成邮箱验证。
- **请求体**

  ```json
  {
    "token": "0c1e1fa5-1c6d-4a6a-8c5d-8f3e7b5c2ee1"
  }
  ```

- **成功响应**：`204`。
- **常见错误**：token 过期、已使用或不存在 → `400` + `VERIFICATION_TOKEN_INVALID`。

#### POST /api/auth/refresh

- **适用场景**：access token 已过期，但 refresh token 仍在有效期内。
- **请求体**

  ```json
  {
    "refresh_token": "<旧的刷新令牌>"
  }
  ```

- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "tokens": {
        "access_token": "...",
        "refresh_token": "...",
        "expires_in": 900
      }
    }
  }
  ```

  刷新后旧的刷新令牌立即失效。

- **常见错误**：请求缺少 `refresh_token`（400）、刷新令牌过期/已登出/被吊销（401）。

#### POST /api/auth/logout

- **用途**：用户主动退出登录或后台强制失效刷新令牌。
- **请求体**

  ```json
  {
    "refresh_token": "<当前刷新令牌>"
  }
  ```

- **成功响应**：`204`（无内容）。此后该刷新令牌无法再换取新的 access token。
- **常见错误**：请求缺少或提供了无效的刷新令牌 → `400 / 401`。

#### GET /api/users/me

- **用途**：返回当前登录用户的基础资料和偏好设置，前端初始化与刷新页面时会调用。
- **请求头**：`Authorization: Bearer <access_token>`。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "user": {
        "id": 1,
        "username": "alice",
        "email": "alice@example.com",
        "avatar_url": "/static/avatars/9be0d0c6.png",
        "last_login_at": "2025-10-10T00:11:22Z"
      },
      "settings": {
        "preferred_model": "gpt-4o-mini",
        "enable_animations": true
      },
      "runtime_mode": "online"
    }
  }
  ```

- **常见错误**：访问令牌缺失或过期 → `401`。

#### PUT /api/users/me

- **用途**：更新用户名、邮箱、头像或模型偏好；请求体只需携带需要修改的字段。
- **请求体示例**：

  ```json
  {
    "username": "alice-dev",
    "email": "alice.dev@example.com",
    "preferred_model": "deepseek",
    "enable_animations": false,
    "avatar_url": ""
  }
  ```

  > 当 `avatar_url` 传入空字符串时，后端会清除数据库中的头像地址，实现“移除头像”。

- **成功响应**：`200`，返回更新后的 `user` 与 `settings`。
- **常见错误**：
  - 邮箱或用户名被其他账号占用 → `409`，`error.details.fields` 会列出冲突字段。
  - 请求体为空或字段格式不正确 → `400`。

#### POST /api/uploads/avatar

- **用途**：把前端选中的头像上传到 `public/avatars`，返回可以立即使用的静态 URL。
- **请求体**（multipart/form-data）：`avatar` 文件字段，支持 PNG/JPG/WEBP，大小 ≤ 5 MB。
- **鉴权**：公开接口，无需登录即可上传头像，注册表单可直接调用。
- **成功响应**：`201`

  ```json
  {
    "success": true,
    "data": {
      "avatar_url": "/static/avatars/6b94de26.png"
    }
  }
  ```

- **常见错误**：
  - 未上传文件或大小为 0 → `400`。
  - 文件超出大小限制或 MIME 类型不被允许 → `400`。
  - 存储目录创建失败或磁盘不可写 → `500`。

#### GET /static/avatars/<filename>

- **用途**：直接访问用户上传的头像资源；无需鉴权。
- **静态托管**：由 `router.go` 的 `r.Static("/static", "./public")` 提供服务，可按需改为指向对象存储或 CDN。

> **提示**：当某个模型凭据被删除或禁用时，若用户当前的 `preferred_model` 指向该模型，服务会自动回退到默认值（`deepseek`）。更新偏好时如果请求的模型不存在或处于禁用状态，会返回 `400 Bad Request` 并在 `error.details.field` 中标出 `preferred_model`。

#### GET /api/models

- **用途**：返回当前登录用户已配置的所有模型凭据，便于前端渲染模型列表。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "models": [
        {
          "id": 1,
          "provider": "openai",
          "label": "GPT-4o",
          "metadata": {
            "default_model": "gpt-4o-mini"
          }
        }
      ]
    }
  }
  ```

- **常见错误**：尚未登录 → `401`；数据库不可用 → `500`。

#### POST /api/models

- **用途**：创建新的模型凭据。服务端会使用 `MODEL_CREDENTIAL_MASTER_KEY` 对 `api_key` 进行 AES-256-GCM 加密后入库。
- **请求体示例**：

  ```json
  {
    "provider": "openai",
    "label": "主账号",
    "api_key": "sk-***",
    "metadata": {
      "default_model": "gpt-4o-mini"
    }
  }
  ```

- **成功响应**：`201`，返回新建记录的 ID。
- **常见错误**：缺少必填字段 → `400`；主密钥未配置 → `500`。

#### PUT /api/models/:id

- **用途**：更新模型标签、元数据或替换 API Key。若传入新的 `api_key`，会重新加密替换旧值；如果字段为空则保持现值。
- **成功响应**：`200`，返回更新后的模型信息。
- **常见错误**：记录不存在 → `404`；主密钥未配置 → `500`。

#### DELETE /api/models/:id

- **用途**：删除指定模型凭据，常用于清理无效或过期的密钥。
- **成功响应**：`204`。
- **常见错误**：记录不存在 → `404`。

#### POST /api/prompts/interpret

- **用途**：解析自然语言描述，生成主题、关键词及补充要求，并初始化工作区缓存。
- **请求体**

  ```json
  {
    "description": "帮我准备 React 前端工程师面试，聚焦 Hooks 和性能优化，排除 jQuery",
    "model_key": "deepseek-chat",
    "language": "中文"
  }
  ```

- **成功响应**：`200`，返回 `topic`、`positive_keywords[]`、`negative_keywords[]`（每项包含 `word`、`weight`、`source`）、`confidence`、`instructions` 以及 `workspace_token`。`weight` 为 0~5 的整数，数值越大表示与主题越相关，前端会优先展示权重高的关键词。
- **常见错误**：描述为空或模型未配置 → `400`。
- **限流**：默认 `8 req/min`，可通过 `PROMPT_INTERPRET_LIMIT` 与 `PROMPT_INTERPRET_WINDOW` 调整。

#### POST /api/prompts/ingest

- **用途**：粘贴已经写好的 Prompt 正文，自动拆解主题、补充要求、正/负关键词与标签，并立即创建一条草稿（状态 `draft`）。常见于“我的 Prompt → 自动解析 Prompt”入口。
- **请求体**

  ```json
  {
    "body": "完整的 Prompt 正文",
    "model_key": "deepseek-chat",
    "language": "zh-CN"
  }
  ```

- **成功响应**：`200`，返回与 `GET /api/prompts/:id` 相同的结构（含 `id`、`topic`、`body`、`instructions`、`tags`、`positive_keywords[]`、`negative_keywords[]`、`workspace_token`、`generation_profile` 等），前端可直接填充工作台。
- **常见错误**：正文为空或模型未配置 → `400`；触发内容审核 → `400`（`CONTENT_REJECTED`）。
- **容错说明**：若初次解析未返回正向关键词，服务会自动使用 Interpret 流程再尝试一次，并回填缺失的主题、标签与补充要求；两次都解析不到关键词时，接口会提示“模型未能从 Prompt 中提取关键词，请补充更多上下文后再试”。
- **限流**：默认 `5 req/min`，可通过 `PROMPT_INGEST_LIMIT` 与 `PROMPT_INGEST_WINDOW` 配置。调用会复用 Interpret/Generate 相同的模型选择逻辑：若请求未显式指定 `model_key`，将使用免费额度别名（`PROMPT_FREE_TIER_*`），因此也会计入模型配额。

#### POST /api/prompts/keywords/augment

- **用途**：在现有基础上补充高关联度关键词。若携带 `workspace_token`，新增词条会直接写入 Redis 工作区。
- **请求体**

  ```json
  {
    "topic": "React 前端面试",
    "model_key": "deepseek-chat",
    "existing_positive": [{"word": "React", "weight": 5}],
    "existing_negative": [{"word": "过时框架", "weight": 2}],
    "workspace_token": "c9f0d7..."
  }
  ```

- **成功响应**：`200`，返回新增的 `positive[]`、`negative[]`，每个元素同样包含 `word`、`weight`、`source` 字段。
- **常见错误**：缺少主题或模型 → `400`；关键词数量已达上限 → `429`。

#### POST /api/prompts/keywords/manual

- **用途**：手动录入关键词，支持指定权重并可选写入当前 Prompt 或工作区。
- **请求体**

  ```json
  {
    "topic": "React 前端面试",
    "word": "组件设计",
    "polarity": "positive",
    "weight": 5,
    "prompt_id": 18,
    "workspace_token": "c9f0d7..."
  }
  ```

- **成功响应**：`201`，返回 `keyword_id`、`word`、`polarity`、`source`、`weight`。
- **常见错误**：缺少词语或主题 → `400`；超出正/负向上限 → `429`。

#### POST /api/prompts/keywords/sync

- **用途**：在拖拽排序或调整权重后同步前端状态到 Redis 工作区，保持后续生成/保存一致。
- **请求体**

  ```json
  {
    "workspace_token": "c9f0d7...",
    "positive_keywords": [{"word": "React", "weight": 5}],
    "negative_keywords": [{"word": "陈旧框架", "weight": 1}]
  }
  ```

- **成功响应**：`204`。
- **常见错误**：工作区不存在或 token 过期 → `400/404`。

#### GET /api/prompts

- **用途**：分页获取当前登录用户保存的 Prompt 列表，包含关键词摘要与标签等基础信息。
- **查询参数**

  | 参数 | 说明 |
  | --- | --- |
  | `status` | 可选，按 `draft` / `published` 过滤，默认不过滤 |
  | `q` | 可选，对 `topic` 与 `tags` 做模糊搜索 |
  | `favorited` | 可选，设置为 `true` / `1` 时仅返回已收藏 Prompt |
  | `page` / `page_size` | 可选，分页参数，默认 `page=1`、`page_size=10`，单页上限 100 |

- **成功响应**：`200`，`data.items` 为 Prompt 列表，每项包含 `id`、`topic`、`model`、`status`、`tags`、`positive_keywords`、`negative_keywords`、`is_favorited`、`is_liked`、`like_count`、`updated_at`、`published_at`；`meta` 返回 `page`、`page_size`、`current_count`、`total_items`、`total_pages`。

#### GET /api/prompts/:id

- **用途**：获取单条 Prompt 详情，并返回一个新的 `workspace_token` 方便在工作台继续编辑。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "id": 42,
      "topic": "React 前端面试",
      "body": "Prompt 正文……",
      "model": "deepseek-chat",
      "status": "draft",
      "tags": ["前端", "面试"],
      "is_favorited": true,
      "is_liked": true,
      "like_count": 12,
      "positive_keywords": [{"word": "React", "weight": 5}],
      "negative_keywords": [{"word": "陈旧框架", "weight": 1}],
      "workspace_token": "c9f0d7...",
      "created_at": "2025-10-10T12:00:00Z",
      "updated_at": "2025-10-12T08:15:00Z",
      "published_at": null
    }
  }
  ```

- **常见错误**：目标 Prompt 不存在或归属不同用户 → `404`。

#### GET /api/prompts/:id/versions

- **用途**：返回 Prompt 的历史版本列表，默认按照版本号倒序排列。
- **查询参数**：`limit`（可选）控制返回数量，未指定时采用 `PROMPT_VERSION_KEEP_LIMIT` 配置值。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "versions": [
        { "version_no": 3, "model": "deepseek-chat", "created_at": "2025-10-12T08:30:45Z" },
        { "version_no": 2, "model": "deepseek-chat", "created_at": "2025-10-10T10:22:11Z" }
      ]
    }
  }
  ```

- **常见错误**：Prompt 不存在或无权访问 → `404`。

#### GET /api/prompts/:id/versions/:version

- **用途**：获取指定历史版本的完整内容，包括正文、补充说明及关键词快照。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "version_no": 3,
      "model": "deepseek-chat",
      "body": "历史正文",
      "instructions": "历史补充说明",
      "positive_keywords": [{"word": "示例", "weight": 4}],
      "negative_keywords": [],
      "created_at": "2025-10-12T08:30:45Z"
    }
  }
  ```

- **常见错误**：指定版本不存在 → `404`。

#### POST /api/prompts/export

- **用途**：将当前用户的全部 Prompt 导出为本地 JSON 文件，并返回导出文件路径及导出时间。
- **请求体**：无。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "file_path": "C:/Users/alice/AppData/Roaming/promptgen/exports/prompt-export-20251012-163045-1.json",
      "prompt_count": 12,
      "generated_at": "2025-10-12T08:30:45Z"
    }
  }
  ```

- **备注**：导出目录通过 `PROMPT_EXPORT_DIR` 配置，默认写入 `data/exports`，首次导出会自动创建目录。

#### POST /api/prompts/import

- **用途**：将 `POST /api/prompts/export` 生成的 JSON 文件导入当前账号，可选择“合并”（默认）或“覆盖”已有 Prompt。
- **请求方式**：
  - `multipart/form-data`：`file`（必填，导出的 JSON 文件）、`mode`（可选，`merge` / `overwrite`）。
  - 或直接以 `application/json` 提交导出文件的原始内容，同时通过查询字符串 `?mode=merge` 指定模式。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "imported_count": 8,
      "skipped_count": 1,
      "errors": [
        { "topic": "React 面试", "reason": "topic is required" }
      ]
    }
  }
  ```

- **说明**：
  - 覆盖模式会在导入前调用 `DeleteByUser` 清理该用户的所有 Prompt、关联关键词与历史版本；关键词字典按需增量更新。
  - 正/负关键词、标签、正文等字段会走与 `POST /api/prompts` 相同的校验逻辑；不符合要求的条目会加入 `errors` 列表并跳过。
  - 导入批处理大小由 `PROMPT_IMPORT_BATCH_SIZE` 控制，默认每批 20 条，并会在日志中输出进度。
  - 历史版本无法完整恢复时，服务会创建一条最新版本快照，版本号沿用导出文件中的 `latest_version_no`。

#### POST /api/prompts/:id/share

- **用途**：生成以 `PGSHARE-` 开头的分享串，可粘贴到 IM/邮件中，由另一台离线客户端导入。
- **请求体**：无，路径参数携带 Prompt ID。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "payload": "PGSHARE-QUJDREVGR0hJSktMTU4",
      "topic": "React 面试官自检",
      "payload_size": 64,
      "generated_at": "2025-10-12T08:30:45Z"
    }
  }
  ```

- **说明**：分享串长度由 `PROMPT_SHARE_MAX_BYTES` 控制，超过阈值会返回 `400` 并提示“分享内容超过允许长度”。

#### POST /api/prompts/share/import

- **用途**：粘贴 `PGSHARE-` 分享串并为当前账号创建一份草稿副本。
- **请求体**：`{ "payload": "PGSHARE-..." }`
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "prompt_id": 128,
      "topic": "React 面试官自检",
      "status": "draft",
      "imported_at": "2025-10-12T08:31:12Z"
    }
  }
  ```

- **常见错误**：分享串缺失或内容被修改 → `400`（`分享串格式不正确`）；分享文本超过 `PROMPT_SHARE_MAX_BYTES` → `400`（`分享串超出长度限制`）。

### Prompt 分享串（PGSHARE）

> 离线客户端无需服务器即可交换 Prompt：生成 `PGSHARE-` 文本 → 通过 IM/邮件传递 → 对方粘贴导入。

- **数据结构**：分享串使用 `PGSHARE-` + Base64URL 的形式封装 `promptShareEnvelope`，内含 `version`、`generated_at` 与单条 Prompt 的正文、关键词、标签、生成配置等字段，不包含用户隐私字段或点赞/评论等派生数据。
- **配置项**：
  - `PROMPT_SHARE_PREFIX`：自定义分享前缀，默认 `PGSHARE`，允许企业独立区分不同产品线。
  - `PROMPT_SHARE_MAX_BYTES`：控制 Base64 部分的最大长度（默认 16384），防止剪贴板写入超大文本；若提示“分享串超出长度限制”，可提示用户精简 Prompt 或调大配置。
- **接口配合**：
  - 生成：`POST /api/prompts/:id/share`。
  - 导入：`POST /api/prompts/share/import`（总是创建草稿）。
- **安全校验**：导入端会验证 Topic/Body/Model 是否为空、关键词数量是否超限、JSON 格式是否合法；若遭篡改会返回 `ErrSharePayloadInvalid`，同时避免恢复历史点赞/公共库状态。
- **客户端建议**：Electron 端可在 Prompt 详情提供“分享”按钮并写入剪贴板，也可在“我的 Prompt”页监听剪贴板或提供“导入分享串”输入框，增强离线协作体验。

#### DELETE /api/prompts/:id

- **用途**：删除指定 Prompt 及其关联的关键词关系、历史版本。
- **成功响应**：`204`。
- **常见错误**：Prompt 不存在或无访问权限 → `404`。

#### PATCH /api/prompts/:id/favorite

- **用途**：收藏或取消收藏指定 Prompt，便于在“我的 Prompt”中快速筛选。
- **请求体**：`favorited`（布尔值），传 `true` 表示收藏，`false` 取消收藏。
- **成功响应**：`200`，返回 `{ "favorited": <bool> }`。
- **常见错误**：Prompt 不存在或归属不同用户 → `404`。

#### POST /api/prompts/:id/like

- **用途**：为指定 Prompt 点赞，累计热度并回传最新计数。
- **成功响应**：`200`，返回 `{ "liked": true, "like_count": <number> }`。
- **常见错误**：Prompt 不存在或无访问权限 → `404`。

#### DELETE /api/prompts/:id/like

- **用途**：取消对指定 Prompt 的点赞。
- **成功响应**：`200`，返回 `{ "liked": false, "like_count": <number> }`。
- **常见错误**：Prompt 不存在或无访问权限 → `404`。

#### GET /api/prompts/:id/comments

- **用途**：分页获取指定 Prompt 的评论线程（顶层评论 + 楼中楼回复）。
- **查询参数**：
  - `page` / `page_size`：分页参数，默认分别为 `1` 与 `PROMPT_COMMENT_PAGE_SIZE`。
  - `status`：普通用户默认仅返回已通过评论；管理员可传 `approved` / `pending` / `rejected` 或 `all` 查看所有记录。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "items": [
        {
          "id": 12,
          "prompt_id": 8,
          "user_id": 3,
      "body": "这条 Prompt 很好用！",
      "status": "approved",
      "like_count": 5,
      "is_liked": true,
      "reply_count": 1,
          "author": { "id": 3, "username": "alice", "avatar_url": "https://..." },
          "created_at": "2025-10-25T08:30:00Z",
          "updated_at": "2025-10-25T08:30:00Z",
          "replies": [
            {
              "id": 13,
              "parent_id": 12,
              "body": "感谢反馈～",
              "status": "approved",
              "like_count": 2,
              "is_liked": false,
              "author": { "id": 1, "username": "admin" },
              "created_at": "2025-10-25T09:05:00Z",
              "updated_at": "2025-10-25T09:05:00Z"
            }
          ]
        }
      ]
    },
    "meta": {
      "page": 1,
      "page_size": 10,
      "total_items": 4,
      "total_pages": 1,
      "current_count": 1
    }
  }

  ```

- **说明**：
  - `like_count` 表示评论累计点赞数量，`is_liked` 反映当前登录用户是否已点赞（未登录默认 `false`）。
  - `reply_count` 代表该楼层下已通过的回复数；管理员额外能读取 `review_note` 与 `reviewer_user_id` 字段。

#### POST /api/prompts/:id/comments

- **用途**：发表新的评论或在指定楼层下回复。
- **请求体**

  ```json
  {
    "body": "写得很详细，点个赞！",
    "parent_id": 12
  }
  ```

- **成功响应**：`201`，返回新建评论对象。
- **内容审核**：若环境变量开启 `PROMPT_AUDIT_ENABLED=1` 且配置了可用的审核模型，后端会在写库前调用模型审查。违规时返回 `400`，错误码为 `CONTENT_REJECTED`，并在 `error.details.reason` 中附带模型判定的中文描述。
- **常见错误**：评论内容为空或超长 → `400`；父级评论不存在或尚未通过审核 → `400`；目标 Prompt 不存在 → `404`。

#### POST /api/prompts/comments/:id/like

- **用途**：为指定评论点赞（包括楼主与子回复）。
- **前置条件**：评论状态需为 `approved`；请求必须携带登录态。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "like_count": 6,
      "liked": true
    }
  }
  ```

- **常见错误**：评论不存在 → `404`；评论尚未通过审核 → `400`，错误码为 `COMMENT_NOT_APPROVED`。

#### DELETE /api/prompts/comments/:id/like

- **用途**：取消对评论的点赞关系。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "like_count": 5,
      "liked": false
    }
  }
  ```

- **常见错误**：评论不存在 → `404`；重复取消不会报错，`like_count` 维持当前值。

#### POST /api/prompts/comments/:id/review

- **用途**：管理员审阅评论，支持改回 `pending`、通过或驳回并附加备注。
- **请求体**

  ```json
  {
    "status": "approved",
    "note": "人工审核通过"
  }
  ```

- **成功响应**：`200`，返回更新后的评论对象（附带审核人信息与备注）。
- **常见错误**：评论不存在 → `404`；非法状态值 → `400`。

#### DELETE /api/prompts/comments/:id

- **用途**：管理员删除指定评论及其所有子回复（级联删除）。
- **成功响应**：`200`，返回 `{ "id": <被删除的评论 ID> }`。
- **常见错误**：评论不存在或已被删除 → `404`。
- **提示**：删除操作不可恢复，如需临时隐藏评论可优先使用审核接口将其标记为 `rejected`。

#### GET /api/public-prompts

- **用途**：获取公共 Prompt 列表，用于优质 Prompt 浏览。离线模式下接口保持可用但仅支持只读，投稿入口会被前端隐藏。
- **查询参数**：`q`（关键词模糊搜索标题、主题、标签）、`status`（管理员可传 `pending`/`rejected` 查看待审条目；普通用户在传入 `pending`/`rejected` 时仅返回自己的投稿，默认返回全量 `approved` 数据）、`page`、`page_size`（默认 `9`，上限 `60`）、`sort_by`（可选，支持 `score`/`downloads`/`likes`/`visits`/`updated_at`/`created_at`，默认 `score`）、`sort_order`（可选，`asc` 或 `desc`，默认 `desc`）。
- **返回字段**：列表项附带 `like_count` / `is_liked`（点赞总数与当前用户点赞态）、`visit_count`（实时访问量，等于数据库值加上 Redis 缓冲增量）以及 `quality_score`（综合评分，越高代表热度/质量越优）。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "items": [
        {
          "id": 18,
          "title": "React 面试宝典",
          "topic": "React 面试",
          "summary": "整理常见行为与技术面试题",
          "model": "deepseek-chat",
          "language": "zh-CN",
          "status": "approved",
          "tags": ["面试", "前端"],
          "download_count": 32,
          "like_count": 12,
          "visit_count": 87,
          "quality_score": 7.6,
          "is_liked": true,
          "created_at": "2025-10-18T02:30:00Z",
          "updated_at": "2025-10-19T09:15:00Z",
          "author_user_id": 3,
          "reviewer_user_id": 1
        }
      ]
    },
    "meta": {
      "page": 1,
      "page_size": 9,
      "current_count": 9,
      "total_items": 24,
      "total_pages": 3
    }
  }
  ```

- **访问控制**：非管理员用户传入 `status=pending` 或 `status=rejected` 时，仅返回其本人投稿的记录；默认或 `status=approved` 场景下返回全量已通过条目。`meta.current_count` 会返回当前页实际条目数，方便前端在栅格布局中展示分页摘要。
  质量评分字段基于点赞、下载、访问与最近更新时间的指数衰减组合，由后台定时任务按 `PUBLIC_PROMPT_SCORE_REFRESH_INTERVAL` 配置（默认 5 分钟）批量重算，单批处理数量由 `PUBLIC_PROMPT_SCORE_REFRESH_BATCH` 控制；权重调节参考 `PUBLIC_PROMPT_SCORE_*` 环境变量，可根据运营策略微调排序。

#### GET /api/public-prompts/:id

- **用途**：查询公共 Prompt 详情，返回正文、说明及关键词快照。普通用户访问未审核（`pending`/`rejected`）条目会得到 `403`。
- **返回字段**：除基础信息外，响应包含 `visit_count`、`like_count` 与 `quality_score`，方便前端展示实时人气指标。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "id": 18,
      "title": "React 面试宝典",
      "topic": "React 面试",
      "summary": "整理常见行为与技术面试题",
      "body": "你是资深面试官...",
      "instructions": "使用 STAR 框架组织问题",
      "model": "deepseek-chat",
      "language": "zh-CN",
      "status": "approved",
      "positive_keywords": [
        { "word": "React", "weight": 5 },
        { "word": "Hooks", "weight": 4 }
      ],
      "negative_keywords": [
        { "word": "jQuery", "weight": 1 }
      ],
      "tags": ["面试", "前端"],
      "download_count": 32,
      "quality_score": 7.6,
      "like_count": 12,
      "visit_count": 87,
      "is_liked": true,
      "created_at": "2025-10-18T02:30:00Z",
      "updated_at": "2025-10-19T09:15:00Z"
    }
  }
  ```

- **访问控制**：仅条目作者或管理员可以查看待审核或已驳回的详情，其余用户访问将返回 `403 Forbidden`；响应体在作者或管理员访问时会包含 `review_reason` 字段。

#### POST /api/public-prompts/:id/like

- **用途**：为公共 Prompt 点赞，底层会同步更新原始私有 Prompt 的点赞数。
- **成功响应**：`200`，返回 `{ "liked": true, "like_count": <number> }`。
- **常见错误**：公共条目不存在 → `404`；条目缺少原始 Prompt（早期录入数据）→ `400`。

#### DELETE /api/public-prompts/:id/like

- **用途**：取消公共 Prompt 的点赞。
- **成功响应**：`200`，返回 `{ "liked": false, "like_count": <number> }`。
- **常见错误**：公共条目不存在 → `404`；条目缺少原始 Prompt → `400`。

#### POST /api/public-prompts/:id/download

- **用途**：将公共 Prompt 复制到当前用户的个人 Prompt 列表，同时自增公共库的下载次数。
- **成功响应**：`200`，返回新建 Prompt 的 ID 及状态，例如 `{ "prompt_id": 42, "status": "draft" }`。
- **限流说明**：受 `PUBLIC_PROMPT_DOWNLOAD_LIMIT` / `PUBLIC_PROMPT_DOWNLOAD_WINDOW` 约束；离线模式使用内存版限流器，仍会限制高频操作。

#### POST /api/public-prompts

- **用途**：提交公共 Prompt 供管理员审核，默认状态为 `pending`。离线模式下返回 `403 Forbidden`。
- **请求体**：`title`、`topic`、`summary`、`body`、`instructions`、`model`、`language`、`tags[]` 以及 `positive_keywords`/`negative_keywords`（支持字符串数组或对象数组）；可选 `source_prompt_id` 指向原始私有 Prompt。
- **校验规则**：当携带 `source_prompt_id` 时，必须引用当前用户名下且状态为 `published` 的 Prompt，否则返回 `400`。
- **重投逻辑**：若作者此前的同主题投稿处于 `pending`/`rejected` 状态，将复用原记录并重置为 `pending`，同时清空驳回信息，避免唯一约束冲突；已通过的条目仍视为只读。
- **成功响应**：`201`，返回新建公共 Prompt 的 `id` 与 `status`。
- **限流说明**：受 `PUBLIC_PROMPT_SUBMIT_LIMIT` / `PUBLIC_PROMPT_SUBMIT_WINDOW` 约束，默认 30 分钟内最多 5 次投稿。

#### POST /api/public-prompts/:id/review

- **用途**：管理员审核公共 Prompt，支持通过（`approved`）或驳回（`rejected`）；可选 `reason` 字段会记录驳回原因，便于前端展示。
- **成功响应**：`204`。
- **常见错误**：记录不存在 → `404`；状态非法 → `400`。

#### DELETE /api/public-prompts/:id

- **用途**：管理员删除指定公共 Prompt 及其统计信息，用于移除不合规内容。
- **成功响应**：`204`。
- **常见错误**：记录不存在或已被删除 → `404`。

#### POST /api/prompts/generate

- **用途**：根据主题与关键词调用模型生成 Prompt，权重会作为提示语的参考信息。
- **请求体**

  ```json
  {
    "topic": "React 前端面试",
    "model_key": "deepseek-chat",
    "positive_keywords": [{"word": "React", "weight": 5}, {"word": "Hooks", "weight": 4}],
    "negative_keywords": [{"word": "陈旧框架", "weight": 1}],
    "instructions": "使用 STAR 框架组织问题",
    "temperature": 0.7,
    "workspace_token": "c9f0d7..."
  }
  ```

- **成功响应**：`200`，返回 `prompt`、`model`、`duration_ms`、`usage`、关键词快照，并回传最终使用的关键词（含权重）。同一用户 60 秒内默认限 3 次。
- **常见错误**：正向关键词为空 → `400`；模型调用失败 → `502`。

#### POST /api/prompts

- **用途**：保存 Prompt 草稿或发布版本；传入 `prompt_id` 表示更新，否则创建新草稿。
- **请求体**

  ```json
  {
    "prompt_id": 18,
    "topic": "React 前端面试",
    "body": "Prompt 正文……",
    "model": "deepseek-chat",
    "status": "draft",
    "publish": false,
    "positive_keywords": [{"word": "React", "weight": 5}],
    "negative_keywords": [{"word": "陈旧框架", "weight": 1}],
    "workspace_token": "c9f0d7..."
  }
  ```

- **成功响应**：`200`，返回 `prompt_id`、`status`、`version`。当 `publish=true` 时生成历史版本并更新 `published_at`；关键字权重会一起落库，便于后续回滚与再生成。
- **校验规则**：当 `publish=true` 或 `status=published` 时，必须同时提供主题、正文、补充要求、模型、至少 1 个正向关键词、1 个负向关键词以及至少 1 个标签；保存草稿时上述字段允许为空。
- **常见错误**：发布时缺少必填字段 → `400`（错误信息形如“发布失败：缺少主题、标签”）；目标 Prompt 不存在 → `404`。

#### GET /api/changelog

- **用途**：向前端公开最近发布的更新日志，普通用户亦可访问。
- **查询参数**：`locale`（可选，默认 `en`）。当指定语言暂无数据时自动回退到英文条目。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "items": [
        {
          "id": 12,
          "locale": "zh-CN",
          "badge": "身份验证",
          "title": "新增邮箱验证码限流提示",
          "summary": "登录与注册的验证码流程新增剩余次数提醒。",
          "items": [
            "超限后返回 retry_after_seconds，前端自动倒计时。",
            "邮件验证请求返回 remaining_attempts 字段。"
          ],
          "published_at": "2025-10-12",
          "created_at": "2025-10-12T03:00:00Z",
          "updated_at": "2025-10-12T03:15:00Z"
        }
      ]
    }
  }
  ```

- **常见错误**：无。

#### POST /api/changelog

- **用途**：新增一条更新日志，需管理员权限。
- **请求头**：`Authorization: Bearer <access_token>`，且该用户 `is_admin=true`。
- **请求体示例**：

  ```json
  {
    "locale": "zh-CN",
    "badge": "身份验证",
    "title": "邮箱验证流程 UX 优化",
    "summary": "验证码限流提示与倒计时已上线。",
    "items": [
      "剩余尝试次数随接口返回实时更新。",
      "限流后返回 retry_after_seconds，前端展示倒计时。"
    ],
    "published_at": "2025-10-12",
    "translate_to": ["en"],
    "translation_model_key": "deepseek-chat"
  }
  ```

- **成功响应**：`201`，返回新建条目与（可选的）翻译条目。
- **常见错误**：
  - 非管理员调用 → `403`。
  - 未提供 `translation_model_key` 却携带 `translate_to` → `400`。
  - 自动翻译时模型调用失败 → `502`（主记录仍会创建，失败的翻译条目会忽略并写入日志）。

#### PUT /api/changelog/:id

- **用途**：更新指定日志条目（标签、标题、摘要、要点列表、发布日期），需管理员权限。
- **成功响应**：`200`，返回更新后的 `entry`。
- **常见错误**：记录不存在 → `404`；非管理员 → `403`。

#### DELETE /api/changelog/:id

- **用途**：删除指定日志条目（硬删除），需管理员权限。
- **成功响应**：`204`。
- **常见错误**：记录不存在 → `404`；非管理员 → `403`。

#### GET /api/ip-guard/bans

- **用途**：管理员查询当前仍在封禁期内的恶意 IP，用于后台可视化。
- **请求头**：`Authorization: Bearer <access_token>`，且 `is_admin=true`。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "items": [
        {
          "ip": "203.0.113.15",
          "ttl_seconds": 1420,
          "expires_at": "2025-10-14T03:32:15Z"
        }
      ]
    }
  }
  ```

  - `ttl_seconds` 会向上取整到秒，`expires_at` 为 UTC 时间，便于前端直接展示。

- **常见错误**：
  - Redis 未配置或 IP Guard 未启用 → `503`。
  - 非管理员访问 → `403`。

#### DELETE /api/ip-guard/bans/:ip

- **用途**：手动解除指定 IP 的封禁，解封后该 IP 后续请求将重新走限流计数。
- **路径参数**：`ip` 支持 IPv4/IPv6 字符串，按原样编码。
- **成功响应**：`204`。
- **常见错误**：Redis 不可用或 IP Guard 未启用 → `503`；非管理员 → `403`。

#### GET /api/admin/metrics

- **用途**：返回管理员仪表盘所需的运营指标快照，读自内存缓存，无需直接扫描数据库。
- **请求头**：`Authorization: Bearer <access_token>`，且 `is_admin=true`。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "refreshed_at": "2025-11-07T01:30:00Z",
      "range_days": 7,
      "totals": {
        "active_users": 42,
        "generate_requests": 128,
        "generate_success": 118,
        "generate_success_rate": 0.9219,
        "average_latency_ms": 832.5,
        "save_requests": 56
      },
      "daily": [
        {
          "date": "2025-11-06",
          "active_users": 18,
          "generate_requests": 56,
          "generate_success": 50,
          "generate_success_rate": 0.8929,
          "average_latency_ms": 912.0,
          "save_requests": 21
        }
      ]
    }
  }
  ```

- **常见错误**：服务尚未完成指标初始化 → `503`；非管理员访问 → `403`。
- **调优提示**：刷新周期与保留天数由 `ADMIN_METRICS_REFRESH_INTERVAL`、`ADMIN_METRICS_RETENTION_DAYS` 控制。

#### GET /api/admin/users

- **用途**：展示管理员后台的用户列表、在线状态、Prompt 统计与最近活动。
- **请求头**：`Authorization: Bearer <access_token>`，且 `is_admin=true`。
- **查询参数**：
  - `page`（可选，默认 1）：分页页码。
  - `page_size`（可选，默认 `ADMIN_USER_PAGE_SIZE`）：单页大小，受 `ADMIN_USER_PAGE_SIZE_MAX` 限制。
  - `query`（可选）：匹配用户名或邮箱的模糊搜索。
- **成功响应**：`200`

  ```json
  {
    "success": true,
    "data": {
      "items": [
        {
          "id": 9,
          "username": "ab-in",
          "email": "ab-in-liusy@outlook.com",
          "avatar_url": null,
          "is_admin": true,
          "last_login_at": "2025-11-07T01:34:00Z",
          "created_at": "2025-10-01T08:00:00Z",
          "updated_at": "2025-11-07T01:34:00Z",
          "is_online": true,
          "prompt_totals": {
            "total": 17,
            "draft": 1,
            "published": 16,
            "archived": 0
          },
          "latest_prompt_at": "2025-11-05T19:32:00Z",
          "recent_prompts": [
            {
              "id": 128,
              "topic": "桌面端错误排查指南",
              "status": "published",
              "updated_at": "2025-11-05T19:32:00Z",
              "created_at": "2025-11-03T09:30:00Z"
            }
          ]
        }
      ],
      "total": 36,
      "page": 1,
      "page_size": 20,
      "online_threshold_seconds": 900
    }
  }
  ```

- **常见错误**：
  - 非管理员访问 → `403`。
  - `page`/`page_size` 小于等于 0 → `400`。
  - 数据库连接异常 → `500`。
- **判定规则**：
  - `is_online` 通过 `last_login_at` 与 `ADMIN_USER_ONLINE_THRESHOLD`（默认 15 分钟）比较得出，阈值内登录即判为在线。
  - `recent_prompts` 默认返回每位用户最近 `ADMIN_USER_RECENT_PROMPT_LIMIT` 条，兼容 MySQL 5.6+。

## 启动与测试

```powershell
cd backend
go run ./cmd/server
```

```powershell
cd backend
go test ./...
```

```powershell
cd backend
go test -tags integration ./tests/integration
```

> **说明：** 单元测试会使用内存版 SQLite 驱动，避免依赖外部数据库，同时也用于验证自动迁移能正确创建 `user_model_credentials` 等表结构。
> **提示：** 若运行环境禁止绑定本地端口，依赖 `httptest`/`miniredis` 的测试会自动标记为 Skip；如遇默认缓存目录不可写，可改用 `GOCACHE=$(pwd)/.gocache go test ./...`。

如需运行带外部依赖的集成测试，请确保 Nacos / MySQL 就绪并补全对应环境变量。

## 项目结构

```text
backend/
├─ cmd/
│  ├─ server/            # 运行入口：加载配置、初始化依赖并启动 HTTP 服务
│  └─ sendmail/          # 邮件调试脚本，可快速验证 DirectMail / SMTP 配置
├─ internal/
│  ├─ app/               # 数据库/缓存等资源的初始化与回收
│  ├─ bootstrap/         # 注入 Repository / Service / Handler / Router
│  ├─ config/            # 环境变量加载、配置解析工具
│  ├─ domain/            # 领域模型（User、Prompt、Keyword、Settings…）
│  ├─ handler/           # HTTP Handler，负责参数绑定、日志、统一响应
│  ├─ infra/             # 基础设施封装（数据库、Redis、Nacos、验证码、加密、邮箱、限流、日志等）
│  ├─ middleware/        # Gin 中间件（JWT 鉴权、离线模式注入、IP Guard …）
│  ├─ repository/        # 数据访问层，对 GORM 做集中封装
│  ├─ server/            # Gin Engine 构建、CORS、静态资源托管
│  └─ service/           # 业务编排：Auth / User / Model / Prompt / Changelog 等
├─ public/               # 静态资源目录（头像等）
├─ tests/                # 单元、集成、E2E 测试用例
├─ logs/                 # 默认日志输出目录
├─ data/                 # 离线模式 SQLite、Prompt 导出文件等
├─ scripts/              # 构建、打包、签名、CI 辅助脚本
└─ design/               # 需求文档、流程/原型等设计资料
```

### 模块职责与协作关系

| 模块 | 职责 | 备注 |
| --- | --- | --- |
| `cmd/server` | 唯一运行入口。读取 `.env`，调用 `internal/app.InitResources` 初始化数据库/Redis/SQLite，再交给 `bootstrap.BuildApplication` 组装依赖并启动 HTTP Server。 | 打包后对应 `server.exe`，Electron 启动时也会调用它。 |
| `internal/app` | 统一管理资源生命周期：连接 MySQL 或 SQLite、自动迁移模型、创建离线模式默认用户、初始化 Redis。 | 离线模式 (`APP_MODE=local`) 会自动创建 `promptgen-local.db` 并 Skip MySQL。 |
| `internal/bootstrap` | 把 Repository、Service、限流器、验证码、邮件、日志等依赖注入 Handler 和 Router。根据 `APP_MODE` 决定使用 `AuthMiddleware`（JWT）还是 `OfflineAuthMiddleware`。 | 这里也是切换在线/离线鉴权的核心。 |
| `internal/domain` | 领域实体与常量，如 `User`、`Prompt`、`Keyword`、`PromptVersion`、`Settings`、各种错误定义等。保持 ORM 模型与业务模型一致，便于 Repository/Service 复用。 | |
| `internal/repository` | 对 GORM 操作做集中封装，提供 User/Prompt/Keyword/ModelCredential 等仓储接口；Service 层只通过接口交互，不直接操控 ORM。 | |
| `internal/service` | 业务规则执行者。负责注册登录、邮箱验证、Prompt 工作台工作流、模型凭据安全存储、更新日志等核心逻辑，返回领域对象或语义化错误。 承担业务规则，尽量保持无状态（stateless），从上下文中读取信息，只依赖接口型组件（仓储、限流器、邮件发送器等）。Service 返回领域对象和语义化错误，不直接写日志，方便被 handler、任务或测试复用。 | Service 不写日志，方便测试与复用。 |
| `internal/handler` | HTTP 适配层。负责请求绑定、校验、访问日志、异常捕获，并调用 Service。统一使用 `infra/common/response` 输出标准响应体。 | Handler 是唯一触达日志的地方。 |
| `internal/infra` | 基础设施适配：MySQL/Nacos/Redis 客户端，加密/解密、验证码、令牌、限流、邮件、日志等。Service/Repository 通过接口依赖这些组件，便于替换与 Mock。 | |
| `internal/middleware` | 提供 Gin 中间件：JWT 鉴权、离线模式注入、IP Guard、防刷等。在 `router.go` 中按需挂载。 | |
| `internal/server` | 负责构建 Gin Engine：配置 CORS、日志格式、静态目录 `/static`、路由分组等。所有 Handler 与中间件的装配都在这里完成。 | |
| `tests` | 覆盖单元/集成/E2E：<br>• `tests/unit` 使用内存 SQLite/假实现，验证业务规则；<br>• `tests/integration` 依赖真实 MySQL/Redis/Nacos，需通过 build tag `integration` 启动；<br>• `tests/e2e` 用于模拟真实部署流程。 | |
| `scripts` | 构建、打包、签名、CI 辅助脚本。Electron 打包命令（`npm run prepare:offline`、`npm run dist:win` 等）会调用这里的工具完成前后端编译。 | |

### 分层原则与建模建议

后端按照“**请求适配 → 业务编排 → 数据访问 → 基础设施**”单向依赖构建，保持各层职责清晰：

- Handler 只做 HTTP 交互与日志；Service 聚焦业务规则；Repository 处理数据库读写；Infra 提供外部能力封装。
- 新增功能时，建议从领域模型 (`internal/domain`) 入手，扩展 Repository，再在 Service 编排逻辑、最后在 Handler 暴露接口，并在 `tests/` 中补充对应测试用例。
- 离线模式、鉴权策略、限流、验证码等横切逻辑，都集中在 `internal/bootstrap` 和 `internal/middleware`，便于替换或按环境启停。

遵循以上结构，可以在排障时快速定位：接口 4xx/5xx 先看 Handler/Service 日志，数据差异看 Repository，连通性问题查 Infra，依赖装配问题查 Bootstrap；测试失败则根据 build tag 判断是单测还是集成测试，逐层回溯。

## Prompt 工作台流程

> 新方案：解析阶段不再直接写 MySQL，而是把 Prompt 工作区缓存在 Redis，降低接口耗时并支持高频编辑；最终保存/发布时再异步落库，确保主题与关键词仍旧持久化在 MySQL。

1. **自然语言解析**：`POST /api/prompts/interpret` 调用模型返回 `topic` 及首批正/负关键词。服务端生成 `workspace_token`，将解析结果写入 Redis（详情见下文），并把 token、topic、keywords 返回给前端。  
2. **关键词补足/手动维护**：`POST /api/prompts/keywords/augment` 与 `POST /api/prompts/keywords/manual` 统一写入 Redis 工作区：  
   - 模型补词直接 `ZADD` 到正/负关键词集合（按 `word` 去重，`score` 记录权重或插入时间）。  
   - 手动录入根据是否带 `workspace_token` 分两种路径：  

     | 调用场景 | 必需参数 | 后端行为 | 响应中的 `keyword_id` |
     | --- | --- | --- | --- |
     | 工作台实时编辑 | `workspace_token`（`prompt_id` 可选） | 仅写入 Redis 快照并刷新 TTL | `0`（表示仍是临时词条） |
     | 离线维护 / 后台批量 | 不传 `workspace_token`，可选 `prompt_id` | 直接 `Upsert` 到 MySQL `keywords` 表；若附带 `prompt_id` 会同步写入关联表 | 返回真实的数据库 ID |

     > **提示**：工作台里的“新关键词”在保存 Prompt 前都是临时态，只有在保存/发布时才会获得正式的 `keyword_id`。
3. **生成 Prompt**：`POST /api/prompts/generate` 读取 Redis 中最新的 topic 与关键词，调用模型生成正文；生成结果同样写回工作区缓存，便于之后继续修改。  
4. **保存草稿/发布**：`POST /api/prompts` 会合并 Redis 快照与请求体，随后调用 `upsertPromptKeywords`：先对正/负关键词逐条 `Upsert`（已有 `keyword_id` 的更新记录；缺省的自动新建并生成 ID），再重建 `prompt_keywords` 关联表，最后把最新快照写回 Prompt 主表与 Redis。若携带 `prompt_id` 则走更新流程，否则创建新的草稿并把新 ID 回写到工作区，供后续继续编辑。  
5. **关键词回收与回放**：后台 worker 在 MySQL 完成入库后，同步更新 Redis 补全缓存（若仍在有效期内），确保后续 interpret/augment 可以复用历史词条；用户重新打开工作台时，优先用 Redis 中的工作区数据，若不存在再回源查询 MySQL。

> **说明**：仍保留 `PersistenceTask` 队列能力，用于后续扩展批量或重型任务；任务幂等关键字段为 `(user_id, prompt_id, workspace_token)`。

### 页面跳转与数据回填

1. **我的 Prompt 列表**：前端通过 `GET /api/prompts` 渲染 “我的 Prompt” 页面，接口会携带精简版的 `positive_keywords[]`、`negative_keywords[]`（仅包含 `word`、`weight`、`source`）。
2. **进入编辑页**：点击“编辑”后调用 `GET /api/prompts/:id`。该接口返回完整的 Prompt 正文、标签以及一个新的 `workspace_token`。前端在 `frontend/src/pages/MyPrompts.tsx` 的 `populateWorkbench` 中，将返回的数据依次写入 `usePromptWorkbench` store：
   - `setTopic` / `setPrompt` / `setModel` / `setPromptId` / `setWorkspaceToken`；
   - `setCollections` 会把 `positive_keywords`、`negative_keywords` 传给工作台状态，并交给 `normaliseKeyword` 统一裁剪长度、去重、同步权重。
3. **工作台渲染**：`PromptWorkbench` 组件读取 store 中的 `positiveKeywords`、`negativeKeywords`，实时展示在拖拽面板中；若 Redis 中仍保留同一个 `workspace_token`，后续的 interpret/augment/manual/sync 都会命中同一份快照，实现原子更新。

### 关键词 ID 与前端拖拽

- 前端为了配合 dnd-kit 的拖拽机制，每次加载关键词都会生成一个仅在当前会话有效的 `id`（见 `frontend/src/pages/PromptWorkbench.tsx` 与 `frontend/src/pages/MyPrompts.tsx` 中对 `nanoid()` 的调用）。这个 `id` 只用于 React 渲染与拖拽定位，不会回传给后端。
- 后端真正识别的字段是 `keyword_id`：
  - interpret / augment / manual（无 `workspace_token`）场景下会返回真实 `keyword_id`；
  - manual（带 `workspace_token`）则返回 `0`，表示词条尚未持久化。
- 当前端调用 `POST /api/prompts` 保存时，会把每个关键词的 `keywordId`（真实 ID 或 `undefined`）随 payload 一起提交。Service 层的 `keywords.Upsert` 会：
  - 若收到合法的 `keyword_id`，更新对应记录的权重与极性；
  - 若缺省，则创建新记录并生成新的 `keyword_id`，随后写入 `prompt_keywords` 关联表。生成后的 ID 也会被序列化到 Prompt 主表，下一次打开工作台时即可复用。

> **小结**：前端 `id` ≠ 后端 `keyword_id`。前者服务于拖拽交互，后者才是数据库主键；在工作台临时阶段只产生前者，真正保存时才会生成或更新后者。

### 关键词权重与删除

#### 已发布 Prompt 再编辑：改权重 / 删除关键词时的流程

- 页面上只是改了某个关键词的权重，然后点击“发布”：
      1. 前端把最新的正向/负向关键词数组（含各自 keywordId）提交到 POST /api/prompts。
      2. 后端在 updatePromptRecord → upsertPromptKeywords 中逐条执行 keywords.Upsert：
          - 如果该词带着合法的 keyword_id，就更新 keywords 表里那条记录的权重、极性、来源；
          - 如果缺少 keyword_id，会新建一条关键词记录并生成新的自增 ID。
      3. 随后调用 ReplacePromptKeywords：它会先把 prompt_keywords 表里属于该 Prompt 的所有关联删掉，再用当前提交的数组重建关联。因此，如果你在工作台
         删掉了某个词，它不会再写回关系表，相当于从这个 Prompt 的“正/负关键词列表”里彻底移除。关键词主表里的那条记录还保留（方便以后再次使用），但和
         这条 Prompt 就不再有关联。
      4. 若这次操作是“再次发布”，LatestVersionNo 会 +1，并把最新正文/关键词快照写入 prompt_versions，同时最多保留最近 3 个版本。于是新权重即刻生效，
         历史版本也能回溯。
- 删除关键词的动作本质上就是让前端不要把它包含在最终的数组里。ReplacePromptKeywords 清空旧关联后只插入当前数组，因此被删除的词会从关联表中消失；如
    果后续完全没人引用，它只是留在 keywords 字典里，可以被其他 Prompt 复用。

#### 已发布 Prompt 再新增关键词并发布

- 新增的词在工作台阶段没有 keyword_id（前端带的是 undefined，Redis 中也是临时态）。
- 工作台阶段的 keyword 的“唯一性”在 Redis 里靠的是 polarity + lowercase(word) 这对组合，既保证同一个词不会重复写入，又方便后续通过 polarity|word 直接定位和删除。等到你真正保存 Prompt 时，才会给这些词补上数据库里的 keyword_id。
- 当你再次点击“发布”：
      1. upsertPromptKeywords 发现该词缺少 keyword_id，调用 keywords.Upsert 后会新增一条记录，拿到新的自增 ID。
      2. ReplacePromptKeywords 把新生成的 ID 与 Prompt 建立关联，原有词仍按当前顺序保留。
      3. 若之前状态已是 published，LatestVersionNo 会递增，同时写入 prompt_versions，旧版本会被保留在历史记录里。
      4. Redis 工作区与 Prompt 主表都会同步最新的关键词快照，下次打开工作台就能看到带着真实 keyword_id 的新词。

### 标签管理

- `SavePrompt` 接口新增 `tags` 字段，Handler 会统一捕获错误并按模块日志输出，Service 负责去重、裁剪空白并校验数量，只返回语义化错误对象。
- 标签上限由 `PROMPT_TAG_LIMIT` 控制（默认 3 个），同时暴露给 Handler 以便生成统一错误提示；持久化时始终写入去重后的 JSON 数组，返回给前端时亦会自动裁剪历史超限数据。
- 单元测试覆盖超限报错与去重行为，避免回归；若需放宽上限，只需修改环境变量并重启服务即可生效。

### Redis 工作区模型

- **工作区标识**：`prompt:workspace:{userID}:{workspaceToken}`（Hash），存储 `topic`、`language`、`model_key`、`draft_body`、`updated_at` 等元数据。  
- **正/负关键词集合**：`prompt:workspace:{userID}:{workspaceToken}:positive` / `:negative`（ZSET），`member` 为小写 `word`，`score` 用于记录当前排序（拖拽后按提交顺序递增）。实际关键词内容存放在 Hash `prompt:workspace:{userID}:{workspaceToken}:keywords` 中，字段为 `polarity|word`，值为 JSON（包含 `source`、`weight`、`display_word` 等）。  
- **关键词同步**：`MergeKeywords`/`replaceKeywords` 会通过 `TxPipeline()` 打开 `MULTI/EXEC`，先写 Hash（来源、权重）再写 ZSET 顺序，任一步失败都会 `Discard`，确保拖拽或调权时 Hash 与 ZSET 不会一半成功一半失败。当前 score 简单使用数组下标递增，纯粹用于保持排序。  
- **Prompt 元信息**：Hash 额外缓存 `prompt_id` 与 `status`（draft/published），创建或更新成功后立即写回，保证前端后续的保存/发布可以基于同一条记录继续编辑。  
- **TTL 策略**：工作区默认保留 30~60 分钟未操作即自动过期；每次写操作需刷新 TTL，避免活跃编辑被提前清理。  
- **并发控制**：使用 `WATCH`/`MULTI` 或 Lua 保证批量写入的原子性；对于生成/保存操作，在将任务入队前记录 `version` 字段，worker 按版本校验，防止旧任务覆盖新内容。  
- **降级策略**：若 Redis 不可用，Handler 会回退到旧流程直接写 MySQL（伴随较长延迟），并在响应头返回 `X-Cache-Bypass: 1` 供前端提示用户稍后重试。  

> **说明**：`LONGTEXT` 字段继续用于 MySQL 中保存最终的 JSON（正/负关键词、标签等），Redis 中的结构旨在加速编辑期的高频读写，而最终数据模型保持兼容。

### 模型调用超时与上下文拆分

- interpret / augment / generate / 保存发布等需要访问大模型的能力，统一通过 `modelInvocationContext` 创建调用上下文：基于 `context.WithoutCancel` 拆离 Gin 的请求生命周期，外层再包裹 35 秒的安全超时。
  1. 为什么不用原始 request.Context()？
     Gin 的 Context 底层包的是一个 request.Context()。一旦 HTTP 响应写回，Gin 会调用 cancel()，把整个请求链路的 context 标记为 done。
     如果我们直接把这个 context 传给模型 SDK，Gin 一返回 200 就会取消 context，模型那边还没回包就被强制中止，常见错误就是 context canceled。
  2. context.WithoutCancel(parent) 做了什么？
     它会复制一个新的 context，并保留原 context 的 Value / Deadline / Err 等信息，但是不会继承 cancel 信号。
     这就相当于“把 request.Context() 里的附件（比如 TraceID、用户 ID）拷贝出来，但切断 cancel 链条”，从而避免 Gin 的 cancel 影响模型请求。
  3. 为什么还要 context.WithTimeout(..., 35s)？
     如果完全不设置超时，请求可能一直卡住。35 秒是一个兜底值，表示“即便外层没有限制，也最多等 35 秒”。
     同时，如果外层本身就带着一个更短的 Deadline（例如调用方自己设置了 10 秒），代码里会优先沿用原来的截止时间：

     if deadline, ok := parent.Deadline(); ok && remaining <= 35s {
         return parent, noopCancel
     }
     也就是说，不会把 10 秒的 Deadline 硬生生拉长成 35 秒。
  4. 返回值为什么要 cancel？
     外层 context.WithTimeout 会返回 (ctx, cancel)。模型调用结束后记得 defer cancel()，及时释放定时器资源。如果走的是上面“沿用已有 Deadline”的分支，就返回一
     个空的 cancel，外层调用 defer cancel() 也不会出错。

- context.WithoutCancel 用来复制一个不会被 Gin 取消的上下文。
- 再给它套一个 35 秒的 timeout，避免长时间阻塞。
- 如果外层有更严格的 Deadline，就尊重原有限制。
- 这样既保留 Request 链上的 Value / Trace，又不会被 Gin 提前 cancel。

### Prompt 评论实现概览

#### 1. 评论表结构掌握两条“线”

| 字段 | 作用 | 举例 |
| --- | --- | --- |
| `parent_id` | 这条评论直接回复了谁，没有回复就为 `NULL` | 回复了 3 号评论，则 `parent_id = 3` |
| `root_id` | 顶层楼层编号，方便一次性把整串楼中楼取出来 | 楼中楼的所有回复都指向同一个首层 ID |
| `status` | `pending/approved/rejected` 三种状态 | 结合配置决定是否需要人工审核 |
| `review_note` / `reviewer_user_id` | 记录审核人和备注 | 管理后台展示用 |

简单记：**parent 表示“楼上是谁”，root 表示“整栋楼是哪栋”**。

#### 2. 从发评论到落库的完整流程

1. 前端请求 POST /api/prompts/:id/comments，携带正文和可选的 parent_id。
2. 后端先做几项检查：内容是否为空、是否超过最大长度、目标 Prompt 是否存在、父评论是否属于同一个 Prompt。
3. 如果环境变量里开启了审核（PROMPT_AUDIT_ENABLED=1 且有模型的 API Key），正文会先送到审核模型（例如 DeepSeek）跑一遍，命中违规会直接返回
   CONTENT_REJECTED 错误，前端可以把 error.details.reason 展示给用户。
4. 审核通过后写入数据库：
    - 顶层评论插进去时，先写一条记录，随后把它自己的 id 回填到 root_id，表示“这条楼的楼主”。
    - 回复评论则在插入时直接写好 parent_id 和 root_id，其中 root_id 等于它所回复的第一条顶层评论的 id。
5. 写库成功后，服务会补上作者的基本信息返回给前端，方便立刻渲染头像、昵称。

- root_id 有什么用？
  - 它让我们能快速把“同一栋楼”的所有回复取出来。列表接口的做法是：
    1. 先取一页顶层评论。
    2. 使用这些顶层评论的 id 作为 root_id 去查询所有子回复。
    3. 通过 root_id 分组，把回复挂到各自的楼主下面，前端拿到的就是一颗颗完整的楼中楼树状结构。
  - 如果没有 root_id，我们只能一条一条地查出每一层的回复，接口次数会暴增，性能也很差。

#### 3. 评论自动审核怎么触发？

- 在 `bootstrap` 里，我们把 Prompt 模块现成的 `AuditCommentContent` 注入进评论服务。
- 只要 `.env` 配置了：
  - `PROMPT_AUDIT_ENABLED=1`
  - `PROMPT_AUDIT_API_KEY`（比如 DeepSeek 的 Key）
  - 可选的 `PROMPT_AUDIT_MODEL_KEY`
- Service 在创建评论时就会把正文丢给模型：
  - 如果模型返回“违规”，接口直接报错，错误码是 `CONTENT_REJECTED`，并在 `error.details.reason` 写清原因；
  - 如果模型返回“通过”，再根据 `PROMPT_COMMENT_REQUIRE_APPROVAL` 决定是直接显示（`approved`）还是进入待审核（`pending`）。
- 离线模式或没配审核 Key 时，审核函数为空，评论会直接落库。

#### 4. 评论列表为什么能带楼中楼？

- `GET /api/prompts/:id/comments` 的 Service 会按照以下步骤执行：
  1. 先查询这一页的顶层评论（`parent_id IS NULL`）。
  2. 把这些顶层 ID 一次性取出来，到数据库里把它们的所有子回复拉回来。
  3. 用 `root_id` 把每个回复归到对应的楼主下面，同时统计每栋楼的回复数量填到 `reply_count`。
  4. 批量附带作者信息，最后组装成 `items = [{ root, replies[] }]` 的结构返回给前端。
- 这种“先批量拉主楼，再批量拉子楼”的方式避免了 N+1 查询，性能比较稳。

#### 5. 管理员能做什么？

- **审核**：
  - 接口：`POST /api/prompts/comments/:id/review`
  - 可把状态改成 `approved` / `rejected` / `pending`，并写审核备注。
- **删除**：
  - 接口：`DELETE /api/prompts/comments/:id`
  - 调用仓储的 `DeleteCascade`，一次性删除该 ID 以及所有子回复（避免遗留悬挂数据）。
  - 删除后返回被操作的 ID，让前端可以及时刷新列表。

#### 6. 相关配置快速对照

| 变量 | 默认值 | 含义 |
| --- | --- | --- |
| `PROMPT_COMMENT_PAGE_SIZE` | 10 | 评论列表默认每页条数 |
| `PROMPT_COMMENT_MAX_PAGE_SIZE` | 60 | 评论单页最大条数（防止一次性抓太多） |
| `PROMPT_COMMENT_MAX_LENGTH` | 1000 | 评论最大字符数 |
| `PROMPT_COMMENT_REQUIRE_APPROVAL` | 0 | 是否需要管理员审核后才展示 |
| `PROMPT_COMMENT_RATE_LIMIT` | 12 | 单个用户窗口期内允许的评论次数 |
| `PROMPT_COMMENT_RATE_WINDOW` | `1m` | 上述限流的时间窗口 |

#### 7. 测试覆盖

- `backend/tests/unit/prompt_comment_service_test.go`：
  - 顶层/回复创建；
  - 审核函数被调用 & 拒绝时直接报错；
  - 级联删除后表内数据归零。
- `backend/tests/unit/prompt_comment_handler_test.go`：
  - 非管理员无法删除评论；
  - 管理员删除后接口返回 200，并确认数据库里的评论被清干净。

掌握这几个点之后，基本可以从“发评论 → 自动审核 → 列表展示 → 审核/删除治理”的全链路自主排查问题。
