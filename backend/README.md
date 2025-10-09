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

```powershell
Copy-Item ..\..\.env.example ..\..\.env.local -Force
```

## 对外 API

| 方法 | 路径 | 描述 | 请求参数 |
| --- | --- | --- | --- |
| `GET` | `/api/auth/captcha` | 获取图形验证码 | 无；按客户端 IP 控制限流 |
| `POST` | `/api/auth/register` | 用户注册 | JSON：`username`、`email`、`password`、`captcha_id`、`captcha_code`（验证码开启时必填） |
| `POST` | `/api/auth/login` | 用户登录 | JSON：`email`、`password` |
| `GET` | `/api/users/me` | 获取当前登录用户信息 | 需附带 `Authorization: Bearer <token>` |
| `PUT` | `/api/users/me` | 更新当前用户信息 | JSON：`username`、`email`、`settings`；同样需要登录 |

接口返回统一结构：`success`、`data`、`error`、`meta`。错误码见 `internal/infra/common/response.go`。

## 启动与测试

```powershell
cd backend
go run ./cmd/server
```

```powershell
cd backend
go test ./...
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
