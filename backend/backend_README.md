# 后端开发指南

本项目提供 Electron 桌面端所需的 Go 后端服务，负责用户注册、登录、验证码获取以及用户信息维护。

## 最近进展

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
- 启动流程拆分为 `internal/app.InitResources`（负责连接/迁移）与 `internal/bootstrap.BuildApplication`（负责装配依赖），提升职责清晰度。
- 新增 `/api/models` 系列接口，支持模型凭据的创建、查看、更新与删除，API Key 会在入库前加密。
- 引入 `MODEL_CREDENTIAL_MASTER_KEY` 环境变量，使用 AES-256-GCM 加解密用户提交的模型凭据。
- 数据库自动迁移包含 `user_model_credentials` 与 `changelog_entries` 表，服务启动即可创建所需数据结构。
- 模型凭据禁用或删除时，会自动清理用户偏好的 `preferred_model`，避免指向不可用的模型；`PUT /api/users/me` 也会验证偏好模型是否存在并已启用。
- 新增 `infra/model/deepseek` 模块与 `Service.InvokeChatCompletion`，可使用存量凭据直接向接入的大模型发起调用（当前支持 DeepSeek / 火山引擎）。
- 新增 `changelog_entries` 表和 `/api/changelog` 接口，允许管理员在线维护更新日志；普通用户可直接读取最新发布的条目。
- JWT 访问令牌新增 `is_admin` 字段，后端会在鉴权中间件里解析并注入上下文，前端可据此展示后台管理能力。

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
| `PUT` | `/api/users/me` | 更新当前用户信息与偏好设置 | JSON：`username`、`email`、`avatar_url`、`preferred_model`、`sync_enabled` |
| `POST` | `/api/uploads/avatar` | 上传头像文件并返回静态地址 | 需登录；multipart 表单：`avatar` 文件字段 |
| `GET` | `/api/models` | 列出当前用户的模型凭据 | 需登录 |
| `POST` | `/api/models` | 新增模型凭据并加密存储 | JSON：`provider`、`label`、`api_key`、`metadata` |
| `PUT` | `/api/models/:id` | 更新模型凭据（可替换 API Key） | JSON：`label`、`api_key`、`metadata` |
| `DELETE` | `/api/models/:id` | 删除模型凭据 | 无 |
| `POST` | `/api/prompts/interpret` | 自然语言解析主题与关键词 | JSON：`description`、`model_key`、`language` |
| `POST` | `/api/prompts/keywords/augment` | 补充关键词并去重 | JSON：`topic`、`model_key`、`existing_positive[]`、`existing_negative[]` |
| `POST` | `/api/prompts/keywords/manual` | 手动新增关键词并落库 | JSON：`topic`、`word`、`polarity`、`prompt_id` |
| `POST` | `/api/prompts/generate` | 调模型生成 Prompt 正文 | JSON：`topic`、`model_key`、`positive_keywords[]`、`negative_keywords[]` |
| `POST` | `/api/prompts` | 保存草稿或发布 Prompt | JSON：`prompt_id`、`topic`、`body`、`status`、`publish` |
| `GET` | `/api/changelog` | 获取更新日志列表 | Query：`locale`（可选，默认 `en`） |
| `POST` | `/api/changelog` | 新增更新日志（管理员） | JSON：`locale`、`badge`、`title`、`summary`、`items[]`、`published_at` |
| `PUT` | `/api/changelog/:id` | 编辑指定日志（管理员） | 同 `POST` |
| `DELETE` | `/api/changelog/:id` | 删除指定日志（管理员） | 无 |

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
        "sync_enabled": true
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
    "sync_enabled": false,
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

- **用途**：解析用户输入的自然语言描述，生成主题与首批关键词建议。
- **请求体**

  ```json
  {
    "description": "帮我准备 React 前端工程师面试，聚焦 Hooks 和性能优化，排除 jQuery",
    "model_key": "deepseek-chat",
    "language": "中文"
  }
  ```

- **成功响应**：`200`，返回 `topic`、`positive_keywords[]`、`negative_keywords[]`、`confidence`。同一用户 60 秒内默认限 3 次，超限返回 `429` 并在 `error.details.retry_after_seconds` 给出冷却时间。
- **常见错误**：描述为空或模型未配置 → `400`。

#### POST /api/prompts/keywords/augment

- **用途**：基于已有关键词调用模型补充新词，同时写入关键词字典，重复词会被自动过滤。
- **请求体**

  ```json
  {
    "topic": "React 前端面试",
    "model_key": "deepseek-chat",
    "existing_positive": [{"word": "React"}],
    "existing_negative": [{"word": "过时框架"}]
  }
  ```

- **成功响应**：`200`，返回新增的 `positive[]`、`negative[]`。
- **常见错误**：缺少主题或模型 → `400`。

#### POST /api/prompts/keywords/manual

- **用途**：手动录入关键词，系统会在用户词典中落库并可选关联到当前 Prompt。
- **请求体**

  ```json
  {
    "topic": "React 前端面试",
    "word": "组件设计",
    "polarity": "positive",
    "prompt_id": 18
  }
  ```

- **成功响应**：`201`，返回 `keyword_id`、`word`、`polarity`、`source`。
- **常见错误**：缺少词语或主题 → `400`。

#### POST /api/prompts/generate

- **用途**：根据主题与关键词调用用户配置的大模型生成 Prompt 正文。
- **请求体**

  ```json
  {
    "topic": "React 前端面试",
    "model_key": "deepseek-chat",
    "positive_keywords": [{"word": "React"}, {"word": "Hooks"}],
    "negative_keywords": [{"word": "陈旧框架"}],
    "instructions": "使用 STAR 框架组织问题",
    "temperature": 0.7
  }
  ```

- **成功响应**：`200`，返回 `prompt`、`model`、`duration_ms`、`usage`、关键词快照。同一用户 60 秒内默认限 3 次。
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
    "positive_keywords": [{"word": "React"}],
    "negative_keywords": []
  }
  ```

- **成功响应**：`200`，返回 `prompt_id`、`status`、`version`。当 `publish=true` 时生成历史版本并更新 `published_at`。
- **常见错误**：缺少 body/topic → `400`；目标 Prompt 不存在 → `404`。

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
│  ├─ server/
│  │  └─ main.go                 # 后端入口，负责加载配置与启动 HTTP Server
│  └─ sendmail/
│     └─ main.go                 # 阿里云 DirectMail 调试命令
├─ internal/
│  ├─ app/
│  │  └─ app.go                  # 资源生命周期管理（数据库、缓存等）
│  ├─ bootstrap/
│  │  └─ bootstrap.go            # 组装仓储、服务、Handler、Router
│  ├─ config/
│  │  └─ env_loader.go           # 加载 .env / 环境变量
│  ├─ domain/
│  │  ├─ prompt/
│  │  │  └─ entity.go            # Prompt / Keyword / 版本实体定义
│  │  └─ user/
│  │     └─ entity.go            # User 实体与 Settings 结构
│  ├─ handler/
│  │  ├─ auth_handler.go         # /api/auth/* 接口
│  │  ├─ user_handler.go         # /api/users/me 查询与更新
│  │  ├─ prompt_handler.go       # /api/prompts/* 工作台接口
│  │  └─ upload_handler.go       # /api/uploads/avatar 上传
│  ├─ infra/
│  │  ├─ captcha/
│  │  │  ├─ config.go            # 验证码配置读取
│  │  │  └─ manager.go           # 验证码生成、验证、限流
│  │  ├─ client/
│  │  │  ├─ mysql_client.go      # MySQL 连接初始化
│  │  │  ├─ nacos_client.go      # Nacos 客户端
│  │  │  └─ redis_client.go      # Redis 客户端
│  │  ├─ common/response.go      # 统一响应体封装
│  │  ├─ logger/logger.go        # Zap 日志初始化
│  │  ├─ security/cipher.go      # AES-256-GCM 加解密工具
│  │  └─ token/
│  │     ├─ jwt_manager.go       # Access Token 签发
│  │     └─ refresh_store.go     # 刷新令牌存储（Redis/内存）
│  ├─ middleware/
│  │  └─ auth_middleware.go      # Bearer Token 鉴权
│  ├─ repository/
│  │  ├─ user_repository.go      # User GORM 操作与唯一性检查
│  │  ├─ model_credential_repository.go # 模型凭据持久化
│  │  └─ prompt_repository.go    # Prompt / Keyword / 版本存储
│  ├─ server/
│  │  └─ router.go               # Gin 路由、CORS、静态资源配置
│  └─ service/
│     ├─ auth/service.go         # 注册、登录、刷新、登出逻辑
│     ├─ user/service.go         # 用户资料、设置更新
│     ├─ model/service.go        # 模型凭据加密与业务逻辑
│     └─ prompt/service.go       # Prompt 工作台核心流程
├─ tests/
│  ├─ unit/
│  │  ├─ auth_service_test.go
│  │  ├─ user_service_test.go
│  │  ├─ captcha_manager_test.go
│  │  └─ response_helper_test.go
│  ├─ integration/
│  │  ├─ auth_flow_integration_test.go
│  │  ├─ mysql_integration_test.go
│  │  ├─ nacos_integration_test.go
│  │  └─ redis_integration_test.go
│  └─ e2e/
│     └─ auth_remote_e2e_test.go
├─ public/
│  └─ avatars/
│     └─ .gitkeep                # 占位文件，运行时头像保存在此目录
└─ design/                       # PRD、流程图等文档
```

需要新增模块时，推荐按照上述分层模式扩展，保持 Handler、Service、Repository 等职责清晰，并优先在 `tests/` 中补充对应的单元/集成测试。

## Prompt 工作台流程

> 新方案：解析阶段不再直接写 MySQL，而是把 Prompt 工作区缓存在 Redis，降低接口耗时并支持高频编辑；最终保存/发布时再异步落库，确保主题与关键词仍旧持久化在 MySQL。

1. **自然语言解析**：`POST /api/prompts/interpret` 调用模型返回 `topic` 及首批正/负关键词。服务端生成 `workspace_token`，将解析结果写入 Redis（详情见下文），并把 token、topic、keywords 返回给前端。  
2. **关键词补足/手动维护**：`POST /api/prompts/keywords/augment` 和 `POST /api/prompts/keywords/manual` 统一操作 Redis 中的工作区数据：  
   - 模型补词直接 `ZADD` 到对应关键词集合（按 `word` 去重，`score` 记录权重或插入时间）。  
   - 手动增删改在 Redis 完成，响应即时返回。  
3. **生成 Prompt**：`POST /api/prompts/generate` 读取 Redis 中最新的 topic 与关键词，调用模型生成正文；生成结果同样写回工作区缓存，便于之后继续修改。  
4. **保存草稿/发布**：`POST /api/prompts` 会从 Redis 工作区拉取快照，立即将 Prompt 正文、关键词字典及 `prompt_keywords` 关联一并写入 MySQL，并在发布时生成最新的 `prompt_versions`。若工作区或请求体中携带已有 `prompt_id`，则执行更新；否则创建新的草稿记录并将 `prompt_id` 与最新状态写回工作区，供后续发布复用。  
5. **关键词回收与回放**：后台 worker 在 MySQL 完成入库后，同步更新 Redis 补全缓存（若仍在有效期内），确保后续 interpret/augment 可以复用历史词条；用户重新打开工作台时，优先用 Redis 中的工作区数据，若不存在再回源查询 MySQL。

> **说明**：仍保留 `PersistenceTask` 队列能力，用于后续扩展批量或重型任务；任务幂等关键字段为 `(user_id, prompt_id, workspace_token)`。

### Redis 工作区模型

- **工作区标识**：`prompt:workspace:{userID}:{workspaceToken}`（Hash），存储 `topic`、`language`、`model_key`、`draft_body`、`updated_at` 等元数据。  
- **正/负关键词集合**：`prompt:workspace:{userID}:{workspaceToken}:positive` / `:negative`（ZSET），`member` 为小写 `word`，`score` 可存储权重或 `unix timestamp`。实际关键词内容存放在 Hash `prompt:workspace:{userID}:{workspaceToken}:keywords` 中，字段为 `polarity|word`，值为 JSON（包含 `source`、`weight`、`display_word` 等）。  
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

## 待办与安全增强排期

- **邮箱校验与验证流程**：新增邮箱格式校验、中间层错误提示，并提供验证邮件链路，防止伪造账号。
- **密码重置**：设计申请重置、邮件验证码、设置新密码的完整流程，覆盖服务层与 Handler 单元测试。
- **登录防护**：补充基于 IP 与账号的速率限制，以及可选的登录失败黑名单策略。
- **审计日志**：记录敏感操作（重置密码、修改邮箱、登出所有设备等），并提供查询接口以便追踪。

以上条目后续会进入迭代计划，优先实现鉴权流程的完整闭环。
