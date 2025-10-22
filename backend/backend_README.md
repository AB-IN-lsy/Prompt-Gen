# 后端开发指南

本项目提供 Electron 桌面端所需的 Go 后端服务，负责用户注册、登录、验证码获取以及用户信息维护。

## 最近进展

- 调整 Prompt 工作台：前端仅在检测到关键词顺序或权重实际变化时才调用 `POST /api/prompts/keywords/sync`，显著降低接口调用频率，后端无需额外改动即可受益。
- 新增用户头像字段 `avatar_url`，支持上传后通过 `PUT /api/users/me` 保存。
- 新增 `POST /api/uploads/avatar` 接口，可接收 multipart 头像并返回可访问的静态地址。
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
- Prompt 历史版本接口可用：新增 `GET /api/prompts/:id/versions` 与 `GET /api/prompts/:id/versions/:version`，支持查看历史版本详情，保留数量由 `PROMPT_VERSION_KEEP_LIMIT` 控制。
- 启动流程拆分为 `internal/app.InitResources`（负责连接/迁移）与 `internal/bootstrap.BuildApplication`（负责装配依赖），提升职责清晰度。
- 新增 `/api/models` 系列接口，支持模型凭据的创建、查看、更新与删除，API Key 会在入库前加密。
- 引入 `MODEL_CREDENTIAL_MASTER_KEY` 环境变量，使用 AES-256-GCM 加解密用户提交的模型凭据。
- 新增 `POST /api/prompts/import`，可上传导出的 JSON 文件并选择“合并/覆盖”模式批量回灌 Prompt，导入批大小由 `PROMPT_IMPORT_BATCH_SIZE` 控制。
- Prompt 版本号策略：首次仅保存草稿不会产生版本（`latest_version_no=0`），首次发布才会生成版本 1；发布后再保存草稿不改变版本号，下次发布时会遍历历史版本取最大值再 +1，保证序号连续递增。
- 数据库自动迁移包含 `user_model_credentials` 与 `changelog_entries` 表，服务启动即可创建所需数据结构。
- 模型凭据禁用或删除时，会自动清理用户偏好的 `preferred_model`，避免指向不可用的模型；`PUT /api/users/me` 也会验证偏好模型是否存在并已启用。
- 新增 `infra/model/deepseek` 模块与 `Service.InvokeChatCompletion`，可使用存量凭据直接向接入的大模型发起调用（当前支持 DeepSeek / 火山引擎）。
- 新增 `changelog_entries` 表和 `/api/changelog` 接口，允许管理员在线维护更新日志；普通用户可直接读取最新发布的条目。
- JWT 访问令牌新增 `is_admin` 字段，后端会在鉴权中间件里解析并注入上下文，前端可据此展示后台管理能力。
- 新增 `/api/ip-guard/bans` 黑名单管理接口，管理员可查询限流封禁的 IP 并调用 `DELETE /api/ip-guard/bans/:ip` 解除；默认从环境变量 `IP_GUARD_ADMIN_SCAN_COUNT`、`IP_GUARD_ADMIN_MAX_ENTRIES` 读取扫描批量与返回上限，避免硬编码“神秘数字”。

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
- 离线模式仍然需要 `MODEL_CREDENTIAL_MASTER_KEY` 用于模型凭据加密；未配置 `JWT_SECRET` 时会自动回退到内置的本地密钥，避免开发时额外填写。

#### 离线模式快速上手

1. 在根目录复制 `.env.example` 为 `.env.local`，修改以下字段：
   - `APP_MODE=local`
   - `LOCAL_SQLITE_PATH=~/promptgen/promptgen-local.db`（按需调整路径）
   - 如需管理员权限，可将 `LOCAL_USER_ADMIN=true`
2. 启动后端：`go run ./backend/cmd/server`
3. 启动前端：`npm run dev:frontend`（Electron 启动流程不变）
4. 登录页点击“离线模式”按钮即可直接进入工作台；此时所有数据仅保存在上一步配置的 SQLite 文件中，云同步、邮箱验证等依赖在线服务的功能会自动禁用。
5. 若要恢复在线模式，将 `APP_MODE` 改回 `online` 并按照原有方式配置数据库/Redis/Nacos。

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
| `CAPTCHA_CONFIG_DATA_ID` / `CAPTCHA_CONFIG_GROUP` | （可选）若填则从 Nacos 拉取验证码配置 JSON，未填时使用环境变量 |
| `CAPTCHA_CONFIG_POLL_INTERVAL` | 拉取 Nacos 配置的轮询间隔，默认 `30s` |

> 当设置 `CAPTCHA_CONFIG_DATA_ID` 时，服务启动后会按 `CAPTCHA_CONFIG_POLL_INTERVAL` 轮询 Nacos，实时应用修改；配置 JSON 中的 `enabled` 字段用于动态开关验证码功能。

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
| `PROMPT_KEYWORD_LIMIT` | 正向/负向关键词的数量上限，默认 `10`，需与前端 `VITE_PROMPT_KEYWORD_LIMIT` 保持一致 |
| `PROMPT_KEYWORD_MAX_LENGTH` | 单个关键词允许的最大字符数（按 Unicode 码点计），默认 `32` |
| `PROMPT_TAG_LIMIT` | 标签数量上限，默认 `3`，需与前端 `VITE_PROMPT_TAG_LIMIT` 保持一致 |
| `PROMPT_TAG_MAX_LENGTH` | 单个标签允许的最大字符数，默认 `5` |
| `PROMPT_LIST_PAGE_SIZE` | “我的 Prompt”列表默认每页数量，默认 `20` |
| `PROMPT_LIST_MAX_PAGE_SIZE` | “我的 Prompt”列表单页最大数量，默认 `100` |
| `PROMPT_USE_FULLTEXT` | 设置为 `1` 时使用 FULLTEXT 检索（需提前创建 `ft_prompts_topic_tags` 索引） |
| `PROMPT_IMPORT_BATCH_SIZE` | 导入 Prompt 时单批处理的最大条数，默认 `20` |

> ❗ **排障提示**：如果日志中出现  
> `decode interpretation response: json: cannot unmarshal array into Go struct`，说明模型把 `instructions` 字段生成为数组。现有实现已兼容数组与字符串两种格式；若自定义提示词，请确保仍返回 JSON 对象，并将补充要求放在 `instructions` 字段（字符串或字符串数组均可）。

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
| `POST` | `/api/uploads/avatar` | 上传头像文件并返回静态地址 | 需登录；multipart 表单：`avatar` 文件字段 |
| `GET` | `/api/models` | 列出当前用户的模型凭据 | 需登录 |
| `POST` | `/api/models` | 新增模型凭据并加密存储 | JSON：`provider`、`label`、`api_key`、`metadata` |
| `PUT` | `/api/models/:id` | 更新模型凭据（可替换 API Key） | JSON：`label`、`api_key`、`metadata` |
| `DELETE` | `/api/models/:id` | 删除模型凭据 | 无 |
| `POST` | `/api/prompts/interpret` | 自然语言解析主题与关键词 | JSON：`description`、`model_key`、`language` |
| `POST` | `/api/prompts/keywords/augment` | 补充关键词并去重 | JSON：`topic`、`model_key`、`existing_positive[]`、`existing_negative[]`、`workspace_token`（可选） |
| `POST` | `/api/prompts/keywords/manual` | 手动新增关键词并落库 | JSON：`topic`、`word`、`polarity`、`weight`（可选，默认 5）、`prompt_id`（可选）、`workspace_token`（可选） |
| `POST` | `/api/prompts/keywords/remove` | 从工作区移除关键词 | JSON：`word`、`polarity`、`workspace_token` |
| `POST` | `/api/prompts/keywords/sync` | 同步排序与权重到工作区 | JSON：`workspace_token`、`positive_keywords[]`、`negative_keywords[]`（元素含 `word`、`polarity`、`weight`） |
| `GET` | `/api/prompts` | 获取当前用户的 Prompt 列表 | Query：`status`（可选，draft/published）、`q`（模糊搜索 topic/tags）、`page`、`page_size` |
| `POST` | `/api/prompts/export` | 导出当前用户的 Prompt 并返回本地保存路径 | 无 |
| `POST` | `/api/prompts/import` | 导入导出的 Prompt JSON（支持合并/覆盖模式） | multipart：`file`（JSON 文件）、`mode`（可选，merge/overwrite）；或直接提交 JSON 正文 |
| `GET` | `/api/prompts/:id` | 获取单条 Prompt 详情并返回最新工作区 token | 无 |
| `GET` | `/api/prompts/:id/versions` | 列出指定 Prompt 的历史版本 | Query：`limit`（可选，默认保留配置中的数量） |
| `GET` | `/api/prompts/:id/versions/:version` | 获取指定版本的完整内容 | 无 |
| `POST` | `/api/prompts/generate` | 调模型生成 Prompt 正文 | JSON：`topic`、`model_key`、`positive_keywords[]`、`negative_keywords[]`、`workspace_token`（可选） |
| `POST` | `/api/prompts` | 保存草稿或发布 Prompt | JSON：`prompt_id`、`topic`、`body`、`status`、`publish`、`positive_keywords[]`、`negative_keywords[]`、`workspace_token`（可选） |
| `DELETE` | `/api/prompts/:id` | 删除指定 Prompt 及其历史版本/关键词关联 | 无 |
| `GET` | `/api/changelog` | 获取更新日志列表 | Query：`locale`（可选，默认 `en`） |
| `POST` | `/api/changelog` | 新增更新日志（管理员） | JSON：`locale`、`badge`、`title`、`summary`、`items[]`、`published_at` |
| `PUT` | `/api/changelog/:id` | 编辑指定日志（管理员） | 同 `POST` |
| `DELETE` | `/api/changelog/:id` | 删除指定日志（管理员） | 无 |
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

- 服务器启动时会将 `/static/**` 映射到项目内的 `backend/public` 目录，头像上传默认写入 `backend/public/avatars`。
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
- **常见错误**：账号不存在或密码错误 → `401` + `ErrInvalidLogin`。
- **邮箱未验证**：返回 `403` + `EMAIL_NOT_VERIFIED`，需先完成邮箱验证。

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
      }
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
  | `page` / `page_size` | 可选，分页参数，默认 `page=1`、`page_size=20`，单页上限 100 |

- **成功响应**：`200`，`data.items` 为 Prompt 列表，每项包含 `id`、`topic`、`model`、`status`、`tags`、`positive_keywords`、`negative_keywords`、`updated_at`、`published_at`；`meta` 返回 `page`、`page_size`、`total_items`、`total_pages`。

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

#### DELETE /api/prompts/:id

- **用途**：删除指定 Prompt 及其关联的关键词关系、历史版本。
- **成功响应**：`204`。
- **常见错误**：Prompt 不存在或无访问权限 → `404`。

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
- **常见错误**：缺少 body/topic → `400`；目标 Prompt 不存在 → `404`。

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
