# Prompt Gen Desktop

> 一套集成 Electron + Go 的 AI Prompt 工作台，覆盖需求解析、关键词治理、模型管理和运维后台。

## 🌟 项目简介

Prompt Gen Desktop 将 Go 写的 API 服务与 Vite/React 前端打包成 Electron 桌面应用，帮助运营 / 研发团队维护高质量的提示词。系统内置自然语言解析、AI 补词、正负关键词治理、模型凭据管理、更新日志与 IP 防护等模块，可在离线桌面端与云端后台之间无缝协同。

## ✨ 核心特性

- **Prompt 工作台**：输入需求即可触发 AI 解析、关键词补全与权重治理，正负关键词支持拖拽、排序与权重调整。
- **模型凭据 & 多通道发送**：在后台保存 DeepSeek / 火山引擎等模型凭据，支持 AES-256-GCM 加密存储并可一键测试。
- **丰富的后台能力**：内置更新日志管理、IP Guard 黑名单解封、邮箱验证提醒、图形验证码限流等管理页面。
- **桌面端体验优化**：自定义无框标题栏、窗口固定比例、始终置顶开关、Electron 预加载桥，网页模式自动隐藏无效按钮。
- **环境驱动的可配置性**：前后端关键词/标签限制、默认权重、关键业务阈值均可通过 `.env` 与 `frontend/.env` 覆盖。
- **一致的 UI 体验**：基于 Tailwind 与 shadcn/ui 抽象的组件库，确保桌面端和网页端使用统一视觉语言。

## 🧱 技术栈

| 模块 | 技术 |
| --- | --- |
| 桌面壳 | Electron 38 / Node 18 |
| 前端 | Vite · React 18 · TypeScript · Tailwind CSS · shadcn/ui · React Query · Zustand · i18next |
| 后端 | Go 1.24 · Gin · Gorm · MySQL · Redis · JWT · Nacos |
| 测试 | go test（unit / integration / e2e）、前端 lint & build |

## 📁 目录总览

```css
.
├── backend/                  # Go 服务端：cmd、internal、tests 等
├── frontend/                 # Vite/React 前端源码与构建脚本
├── main.js / preload.js      # Electron 主进程 & 预加载桥
├── AGENTS.md                 # 贡献者指南
├── README.md                 # 当前文件
└── .env.example              # 后端环境变量模板
```

## 🚀 快速开始

### 前置依赖

- Node.js ≥ 18
- Go ≥ 1.24
- MySQL 5.7 / MariaDB（持久化 Prompt 与账号）
- Redis（验证码、限流、工作台会话缓存）
- Nacos（统一读取生产配置）

### 1. 克隆代码

```bash
git clone https://github.com/your-org/prompt-gen-desktop.git
cd prompt-gen-desktop
```

### 2. 配置环境变量

根目录复制 `.env.example` → `.env.local`，Electron 主进程、Go 后端 与 Vite 前端都会自动加载。建议至少填写：

- `MYSQL_HOST` / `MYSQL_USERNAME` / `MYSQL_PASSWORD` / `MYSQL_DATABASE`
- `JWT_SECRET`
- `MODEL_CREDENTIAL_MASTER_KEY`（32 字节 Base64）
- `REDIS_ENDPOINT` / `REDIS_PASSWORD`（如启用）

前端无须单独的 `frontend/.env.local`。同一个 `.env.local` 中可直接加入前端相关字段，Vite 会通过 `envDir` 自动读取：

### 3. 安装依赖

```bash
npm install                       # Electron 主程 & 脚手架
npm --prefix frontend install     # 前端依赖
go mod tidy                       # 后端依赖
```

### 4. 启动开发环境

```bash
# 1) 启动后端 API（默认 9090）
go run ./backend/cmd/server

# 2) 启动前端 Vite Dev Server（默认 5173）
npm run dev:frontend

# 3) 启动 Electron 桌面壳（会加载 Vite Dev URL）
npm start
```

生产构建可运行：

```bash
npm run build:frontend   # 构建前端产物到 frontend/dist
npm run build            # 同上（package.json 透传）
```

## 🧪 测试 & 质量

```bash
# 前端
npm --prefix frontend run lint
npm --prefix frontend run build   # 也会执行 TS 检查

# 后端
go test ./...                     # 单元测试
go test -tags=integration ./backend/tests/integration # 集成测试
go test -tags=e2e ./backend/tests/e2e # E2E 测试
```

> 集成 / E2E 测试需要联通的 MySQL、Redis、Nacos，并依赖 `.env.local` 或测试专用环境变量。

## 🔧 实用脚本（根目录）

| 命令 | 说明 |
| --- | --- |
| `npm run dev:frontend` | 启动 Vite 开发服务 |
| `npm run build:frontend` | 构建前端生产静态文件 |
| `npm start` | 启动 Electron，开发模式会指向 Vite URL |
| `go run ./backend/cmd/server` | 启动后端 HTTP 服务 |
| `go test ./...` | 运行全部 Go 测试 |

## 🧭 后续规划

- CI/CD 集成（lint / test / 构建自动化）
- 支持更多模型通道与模板库
- 桌面端差分更新与自动升级

## 📄 许可证

项目遵循仓库中的 [LICENSE](LICENSE) 约束。欢迎提交 Issue 或 PR，一起完善 Prompt Gen Desktop。
