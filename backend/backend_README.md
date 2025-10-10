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
- 数据库自动迁移包含 `user_model_credentials` 表，服务启动即可创建所需数据结构。
- 模型凭据禁用或删除时，会自动清理用户偏好的 `preferred_model`，避免指向不可用的模型；`PUT /api/users/me` 也会验证偏好模型是否存在并已启用。

## 环境变量

### 运行必需

| 变量 | 作用 |
| --- | --- |
| `SERVER_PORT` | HTTP 服务监听端口，默认 `9090` |
| `JWT_SECRET` | JWT 签名密钥，必填 |
| `JWT_ACCESS_TTL` | 访问令牌有效期，如 `15m` |
| `JWT_REFRESH_TTL` | 刷新令牌有效期，如 `168h` |
| `NACOS_ENDPOINT` | Nacos 地址，形如 `ip:port` |
| `NACOS_USERNAME` / `NACOS_PASSWORD` | Nacos 登录凭证 |
| `NACOS_GROUP` / `NACOS_NAMESPACE` | Nacos 读取 MySQL 配置所用的分组与命名空间 |
| `MYSQL_CONFIG_DATA_ID` / `MYSQL_CONFIG_GROUP` | Nacos 中 MySQL 配置的 DataId 与 Group |
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

### 邮箱验证与邮件发送

| 变量 | 作用 |
| --- | --- |
| `APP_PUBLIC_BASE_URL` | 前端公共地址，用于拼接验证链接，例如 `https://app.example.com` |
| `EMAIL_VERIFICATION_LIMIT` | 邮箱验证重发限频次数，默认 `5` |
| `EMAIL_VERIFICATION_WINDOW` | 邮箱验证限频窗口时长，默认 `1h` |

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
| `POST` | `/api/auth/login` | 用户登录 | JSON：`email`、`password` |
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
│  │  └─ user/
│  │     └─ entity.go            # User 实体与 Settings 结构
│  ├─ handler/
│  │  ├─ auth_handler.go         # /api/auth/* 接口
│  │  ├─ user_handler.go         # /api/users/me 查询与更新
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
│  │  └─ model_credential_repository.go # 模型凭据持久化
│  ├─ server/
│  │  └─ router.go               # Gin 路由、CORS、静态资源配置
│  └─ service/
│     ├─ auth/service.go         # 注册、登录、刷新、登出逻辑
│     ├─ user/service.go         # 用户资料、设置更新
│     └─ model/service.go        # 模型凭据加密与业务逻辑
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

## 待办与安全增强排期

- **邮箱校验与验证流程**：新增邮箱格式校验、中间层错误提示，并提供验证邮件链路，防止伪造账号。
- **密码重置**：设计申请重置、邮件验证码、设置新密码的完整流程，覆盖服务层与 Handler 单元测试。
- **登录防护**：补充基于 IP 与账号的速率限制，以及可选的登录失败黑名单策略。
- **审计日志**：记录敏感操作（重置密码、修改邮箱、登出所有设备等），并提供查询接口以便追踪。

以上条目后续会进入迭代计划，优先实现鉴权流程的完整闭环。
