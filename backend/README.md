# 后端开发指南

本项目提供 Electron 桌面端所需的 Go 后端服务，负责用户注册、登录、验证码获取以及用户信息维护。

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

### 可选：验证码与 Redis

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

在仓库根目录中复制 `.env.example` 为 `.env.local` 填入真实值，服务启动时会自动加载。

> **提示：** 刷新令牌默认存入 Redis；若未配置 Redis，则退化为进程内内存存储，适合开发环境，但服务重启后刷新令牌会全部失效。

```powershell
Copy-Item ..\..\.env.example ..\..\.env.local -Force
```

## 对外 API

| 方法 | 路径 | 描述 | 请求参数 |
| --- | --- | --- | --- |
| `GET` | `/api/auth/captcha` | 获取图形验证码 | 无；按客户端 IP 控制限流 |
| `POST` | `/api/auth/register` | 用户注册 | JSON：`username`、`email`、`password`、`captcha_id`、`captcha_code`（验证码开启时必填） |
| `POST` | `/api/auth/login` | 用户登录 | JSON：`email`、`password` |
| `POST` | `/api/auth/refresh` | 刷新访问令牌 | JSON：`refresh_token` |
| `POST` | `/api/auth/logout` | 撤销刷新令牌 | JSON：`refresh_token` |
| `GET` | `/api/users/me` | 获取当前登录用户信息 | 需附带 `Authorization: Bearer <token>` |
| `PUT` | `/api/users/me` | 更新当前用户信息 | JSON：`username`、`email`、`settings`；同样需要登录 |

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

如需运行带外部依赖的集成测试，请确保 Nacos / MySQL 就绪并补全对应环境变量。

## 项目结构

```text
backend/
├─ cmd/server/           # 程序入口，负责加载配置与启动 HTTP 服务
├─ internal/
│  ├─ app/               # 应用启动流程：读取 Nacos、连接 MySQL、提供资源管理
│  ├─ bootstrap/         # 组合业务依赖（仓储、服务、Handler、Router）
│  ├─ config/            # .env 文件加载、测试辅助
│  ├─ domain/            # 领域模型与实体定义
│  ├─ handler/           # Gin Handler，处理 HTTP 请求
│  ├─ infra/             # 基础设施层（Redis/Nacos/MySQL 客户端、验证码、日志、Token 等）
│  ├─ middleware/        # Gin 中间件（如鉴权）
│  ├─ repository/        # 数据访问层，封装 GORM 操作
│  ├─ server/            # Gin Router 构建与公共路由配置
│  └─ service/           # 业务服务层逻辑（Auth、User）
├─ tests/
│  ├─ unit/              # 单元测试，包含 Redis、验证码、服务层等测试用例
│  ├─ integration/       # 需外部依赖的集成测试
│  └─ e2e/               # 端到端测试脚本
└─ design/               # 需求与流程文档
```

需要新增模块时，推荐按照上述分层模式扩展，保持 Handler、Service、Repository 等职责清晰。
