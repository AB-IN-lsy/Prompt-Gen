# PromptGen 前端

基于 Vite + React + TypeScript 构建的 Electron 前端界面，提供仪表盘、提示词工作台、设置中心等功能，并整合 React Query、Zustand 与 i18next 完成数据管理与多语言支持。

## 近期前端调整

- **窗口外壳**：Electron 渲染进程默认启动在 1100×720 的固定比例窗口中，禁用边框拖拽，仅保留最小化与最大化控制；网页模式下会自动隐藏桌面专属按钮并显示 “Web Preview” 提示。
- **关键词配置**：新增 `frontend/.env.example`，可通过 `VITE_KEYWORD_ROW_LIMIT`（默认 3）与 `VITE_DEFAULT_KEYWORD_WEIGHT`（默认 5）覆写前端关键词展示与权重初值。
- **Prompt 工作台布局**：补充要求与标签输入区改为纵向堆叠，标签按钮加宽并与输入框同列，避免小屏下换行。

## 已实现功能概览

- **用户注册/登录**：注册页集成图形验证码与头像上传，支持自动重试、行内校验与失败提示；当邮箱与用户名同时冲突时，会逐项标记冲突字段；登录页支持邮箱或用户名登录，提供“记住此账号”选项，并通过全局通知（Toast）反馈成功或错误状态。

### 记住账号实现

- 登录页勾选“记住此账号”时，会把当前输入的邮箱/用户名写入 `localStorage`（键名 `promptgen:last-identifier`）；进入页面时读取并回填，同时自动勾选复选框。
- 若用户未勾选或在提交前取消勾选，登录成功后会删除该键，不再自动填充账号；访问令牌仍通过 tokenStorage 管理，与账号记住功能无关。
- **邮箱验证提示**：登录页与个人信息页共用的验证状态面板会展示剩余重发次数并实时响应后端广播，保证用户在任意入口都能看到最新验证进度。
- **注册反馈增强**：注册成功后会自动触发邮箱验证邮件并弹出提示，指引用户前往收件箱继续完成流程。
- **仪表盘**：`Dashboard.tsx` 提供工作台概览，包括关键指标卡、近期活动、提示词表现表格、关键词洞察等模块，当前以 Mock 数据占位，便于后续接入真实接口。
- **Prompt 工作台（新版）**：新增“自然语言需求解析 → AI 补词 → 模型生成 → 草稿/发布”一站式流程；支持实时解析置信度、模型可用性联动、手动/自动关键词去重与来源标记；补充要求与标签编辑器上下排列，支持快速添加/移除并显示数量上限。
- **关键词权重与拖拽**：正、负向关键词支持 0~5 的相关度权重，点击 ± 调整；拖拽即可跨列移动，需要时再点击“按权重排序”按钮整理顺序。
- **关键词同步优化**：仅当权重或排序实际发生变化时才触发 `/api/prompts/keywords/sync`，并强化跨列拖拽的视觉反馈，减少多余请求同时提升操作感受。
- **交互细节**：需求解析下方直接提供手动添加与「AI 补充关键词」引导，中列仅保留正/负向关键字池，排序按钮固定在行尾；解析/补词/生成等长时操作会弹出 Toast 进度条。
- **模型凭据管理**：设置页新增模型列表与创建表单，可启用/禁用、删除模型并一键设为偏好，支持额外 JSON 配置持久化。
- **模型可用性联动**：Prompt 工作台智能感知模型启用状态，自动回退到可用模型并提示用户，避免引用失效配置。
- **设置中心**：支持头像上传/移除、用户名/邮箱/偏好模型/云同步开关的编辑，同时保留语言切换与状态持久化；内置主题偏好（亮色/暗色/跟随系统），会监听 `prefers-color-scheme` 自动切换；移除头像后点击保存会立即同步至服务器与本地状态。
- **应用外壳**：`AppShell` 搭建统一导航、顶部工具栏、账户信息与退出功能，展示后端返回的用户头像并在退出时提示成功。导航栏已分为“普通入口”和“管理后台”两段：普通用户只会看到仪表盘、Prompt 工作台、帮助等菜单；管理员登录后，在侧边栏下半段会出现“管理后台”分隔标题，包含 IP 防护与更新日志管理页面入口。桌面模式下的自定义标题栏直接调用 Electron 预加载桥暴露的窗口控制，浏览器预览则自动降级为静态提示。
- **帮助中心**：新增 `Help` 页面，按模块整理快速上手、认证、工作台、模型凭据和排障建议，并集成 CSDN / GitHub / 邮箱等外部资源链接。
- **更新日志**：`Logs` 页面优先从 `/api/changelog` 读取最新动态，面向所有用户展示我们已经发布的变更条目。
- **更新日志管理**：新增 `ChangelogAdmin` 管理页，管理员可在导航栏下半区进入，完成日志的创建、编辑、删除与翻译发布。页面展示的标签（如“身份验证”等）已统一更换为渐变色胶囊，和用户端展示保持一致。
- **IP 防护黑名单**：新增 `IpGuard` 页面，管理员可在界面上查询被限流封禁的 IP 并一键解除，普通用户仅展示权限受限提示。

## 快速开始

```powershell
# 安装依赖
npm install

# 开发模式（默认使用 Vite Dev Server）
npm run dev

# 构建生产包
npm run build

# 本地预览构建结果
npm run preview
```

### 开发端口说明

- `5173`：Vite 开发服务器默认端口。执行 `npm run dev` 时，浏览器或 Electron 渲染进程会从该端口加载最新的热更新页面。
- `4173`：`npm run preview` 使用的预览端口，用于本地验证生产构建的效果。

前端的 API 客户端（`src/lib/http.ts`）默认直接指向 `http://localhost:9090/api`（Go 后端监听端口）。若需要接入远程环境，可以在 `frontend/.env.local` 中设置 `VITE_API_BASE_URL` 覆盖默认地址，构建与运行都会读取该变量。上传成功的头像会返回 `/static/avatars/<文件名>` 路径，由后端静态资源路由托管。

关键词与标签的数量与长度上限均可通过环境变量调节：在 `frontend/.env.local` 中设置 `VITE_PROMPT_KEYWORD_LIMIT=10`、`VITE_PROMPT_KEYWORD_MAX_LENGTH=32`、`VITE_PROMPT_TAG_LIMIT=5`、`VITE_PROMPT_TAG_MAX_LENGTH=5`（需与后端的 `PROMPT_*` 对应项保持一致），即可同步约束提示词与标签的数量和字符长度，避免前后端配置漂移。

## 接口请求流程一图梳理

```text
┌───────────────┐         开发环境 (npm run dev)
│  浏览器 /     │  HTTP   ┌──────────────────────────────┐
│  Electron UI  ├────────▶│ Vite Dev Server (localhost:5173)
└───────────────┘         └──────────────────────────────┘
          │                                │
          │ 直接发起 API 请求              │ 反向代理（旧方案，已弃用）
          ▼                                ▼
┌────────────────────────────────────────────────────────┐
│          Go Backend (localhost:9090/api/...)           │
└────────────────────────────────────────────────────────┘

打包执行 / npm run preview：页面静态托管于 4173 或 file://，仍然按上图直接访问 9090
```

### 为什么改成“直接请求后端”？

1. **统一代码路径**：无论在浏览器、Electron 还是生产环境，前端都直接请求后端地址；不再依赖 Vite 的代理规则，避免“本地好用、Electron 失败”的情况。
2. **CORS 兜底**：由于后端已经允许来自 `localhost`/`127.0.0.1`/`null` 的跨域访问，直接请求不会被浏览器拦截。
3. **易于切换环境**：把接口根路径放在 `VITE_API_BASE_URL`，想连远程服务器时，只需在 `.env` 中改一个变量。

如果未来再次需要本地反向代理（例如解决 HTTPS 或复杂路径问题），可以在 `vite.config.ts` 中重新开启 `server.proxy`，但记得同时评估 Electron 或其他运行时是否也能复用这条链路。

## 邮箱验证码与图形验证码的限流提示

- `POST /api/auth/request-email-verification` 成功或限流时都会返回 `remaining_attempts` 字段，前端可据此显示“剩余发送次数”；被限流还会返回 `retry_after_seconds`，用于告知用户冷却时间。
- `GET /api/auth/captcha` 同样返回 `remaining_attempts`，便于在 UI 中提示还可刷新几次验证码。

## 目录结构概览

```text
frontend/
├─ package.json
├─ postcss.config.js
├─ tailwind.config.ts
├─ tsconfig*.json
├─ vite.config.*
└─ src/
   ├─ main.tsx                     # React 应用入口，挂载 QueryClient 与浏览器路由
   ├─ App.tsx                      # 根路由配置，统一注入布局组件
   ├─ index.css                    # 全局样式（Tailwind + 自定义变量）
   ├─ components/
   │  ├─ layout/
   │  │  ├─ AppShell.tsx           # 顶部工具栏 + 侧边导航的应用外壳
   │  │  └─ AuthLayout.tsx         # 登录/注册页的独立布局
   │  └─ ui/                       # 轻量 UI 组件（Button、Input、GlassCard 等）
   ├─ pages/
   │  ├─ Dashboard.tsx             # 仪表盘页面，展示指标卡、动态与提示词表现
   │  ├─ Login.tsx                 # 登录表单，内置行内校验与 Toast 成功/失败提示
   │  ├─ Register.tsx              # 注册表单，内置验证码刷新、自动重试与行内校验提示
   │  ├─ PromptWorkbench.tsx       # 提示词工作台，整合关键词管理与草稿编辑
   │  ├─ Settings.tsx              # 设置页，管理账户资料、模型偏好与主题
   │  ├─ Help.tsx                  # 帮助中心，汇总使用指南与支持渠道
   │  ├─ Logs.tsx                  # 更新日志，展示并（管理员）维护 changelog
   │  ├─ IpGuard.tsx               # IP 防护黑名单管理页，管理员可查看/解封
   │  └─ ChangelogAdmin.tsx        # 更新日志管理页，管理员在此维护内容
   ├─ hooks/
   │  ├─ useAppSettings.ts         # 管理语言等全局设置的 Zustand Store
   │  ├─ useAuth.ts                # 认证状态 Store，负责初始化、登录、登出
   │  └─ usePromptWorkbench.ts     # 提示词工作台的业务状态 Store
   ├─ lib/
   │  ├─ api.ts                    # 针对后端业务接口的封装（关键词、草稿、生成等）
   │  ├─ http.ts                   # Axios 实例与 Token 刷新、错误归一化逻辑
   │  ├─ errors.ts                 # 统一的 ApiError 定义
   │  ├─ tokenStorage.ts           # Token 持久化与过期判断
   │  └─ utils.ts                  # 常用工具（className 合并等）
   ├─ i18n/
   │  ├─ index.ts                  # i18next 初始化、语言探测与持久化
   │  └─ locales/                  # 多语言文案资源（中文、英文）
   └─ lib/types 等其它业务文件
```

## 核心模块说明

- **数据获取与缓存**：`lib/http.ts` 负责统一的 Axios 设置、响应解包和 401 刷新；业务接口以 `lib/api.ts` 为入口，配合 React Query 在页面中使用。
- **状态管理**：`hooks/usePromptWorkbench.ts`、`hooks/useAppSettings.ts` 使用 Zustand 管理跨组件状态并提供严格的更新函数。
- **界面布局**：`components/layout/AppShell.tsx` 定义全局导航壳，所有页面走 `App.tsx` 进入；页面内 UI 统一复用 `components/ui/` 中的基础控件。
- **认证流程**：`hooks/useAuth.ts` 负责在启动时校验 token、刷新用户资料、执行登出；`Login.tsx`/`Register.tsx` 通过 `useAuth.authenticate` 串联登录/注册后的资料拉取；`App.tsx` 根据登录态切换路由。
- **国际化**：`i18n/index.ts` 初始化翻译实例并读取 `locales/` 下的文案；`Settings.tsx` 通过 store 调用 `setLanguage` 完成切换与持久化。

## 更新日志编辑（管理员专属）

- `Logs.tsx` 首次渲染时会调用 `GET /api/changelog?locale=<当前语言>`，并使用 React Query 缓存 5 分钟；若接口暂未返回数据，则使用 i18n 中的静态条目兜底。
- 当登录用户的 `profile.user.is_admin` 为 `true` 时，会展示一个可编辑的表单，可创建或更新 changelog；操作成功后自动刷新列表。删除操作会二次确认并调用 `DELETE /api/changelog/:id`。
- 表单支持在「亮点」文本域中逐行输入内容，保存前会自动拆分为字符串数组，再提交给后端。
- 若需要快速体验后台管理，可在数据库中将目标用户的 `is_admin` 字段置为 `1`，刷新登录态后即可看到管理面板。
- 勾选“自动翻译”可在提交时让后端调用已配置的模型（DeepSeek 或火山引擎等）生成目标语言版本；需在表单中提供 `translation_model_key`，例如 `deepseek-chat` 或 `doubao-1-5-thinking-pro-250415`。
- 后续若你将原有 i18n 文案迁移至数据库，记得清理 `logsPage.entries` 的静态内容以避免重复。

## 身份认证与路由

1. 首次打开页面时，`App.tsx` 会调用 `useAuth.initialize()` 并在加载阶段展示全屏进度，避免在资料尚未拉取完成前误把用户重定向到登录页；若本地存在 token 会自动请求 `/users/me`，成功后进入受保护页面，否则跳转登录页。
2. 登录或注册成功后，`useAuth.authenticate()` 会保存令牌并重新请求 `/users/me` 以刷新头像、用户名和个性化设置，然后导航到 Prompt 工作台。
3. `AppShell` 顶部展示当前用户名并提供退出按钮；退出会调用 `/auth/logout`、清理本地 token，并重定向到 `/login`。
4. 所有受保护路由在未登录状态下都会重定向到 `/login`，防止用户绕过权限访问。

## 开发约定

- 使用 TypeScript，建议在新增文件时同步补全类型定义。
- 所有对后端的请求都应通过 `lib/api.ts` 暴露的函数完成，避免散落的 Axios 调用。
- 新增页面时，把路由注册在 `App.tsx` 并复用 `AppShell` 外壳，保持导航一致。
- 文案统一维护在 `i18n/locales/*`，新增 key 请同步提供中英双语翻译。
- 若扩展 UI 组件，请优先考虑 `components/ui/` 目录，保持复用性。

## 后续改进建议

- **统一移除 JS 旧实现**：`src/pages` 与 `src/hooks` 仍保留 `.js` 版本，确认无依赖后可删除，避免与 TS 文件产生漂移。
- **表单体验增强**：为登录/注册表单补充即时校验、密码可见切换、全局 toast 等手势反馈。
- **验证码与安全**：根据实际接口返回码细化错误提示，并在失败次数过多时加入额外保护措施。
- 引入拖拽库（如 dnd-kit）实现关键词排序与权重调整的可视化操作。
- 在 `PromptWorkbench` 中补充请求/保存的 Toast 反馈，增强错误提示体验。
- 接入 Dashboard / My Prompts 实时数据接口，替换 mock 数据并增加骨架屏。
- 对国际化文案进行分模块拆分，减少单一 JSON 文件的体积与维护难度。
