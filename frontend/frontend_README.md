# PromptGen 前端

基于 Vite + React + TypeScript 构建的 Electron 前端界面，提供仪表盘、提示词工作台、设置中心等功能，并整合 React Query、Zustand 与 i18next 完成数据管理与多语言支持。

## 近期前端调整

- **窗口外壳**：Electron 渲染进程默认启动在 1100×720 的固定比例窗口中，禁用边框拖拽；Windows / Linux 保留自定义的最小化、最大化与关闭按钮，macOS 则让位于系统左上角交通灯，仅保留置顶（Pin）按钮以避免与原生控件冲突。网页模式下会自动隐藏桌面专属按钮并显示 “Web Preview” 提示。
- **生成配置弹窗**：Prompt 工作台新增 “生成配置” 按钮，可在弹窗内调整逐步推理、采样温度、Top P 与最大输出 tokens，修改结果会同步到工作台与保存/生成流程；默认值读取 `VITE_PROMPT_GENERATE_*`，与后端 `PROMPT_GENERATE_*` 保持一致。
- **管理员指标面板**：左侧导航新增 “运营监控” ，调用 `GET /api/admin/metrics` 渲染日活、请求量、成功率等卡片，并通过 Recharts 绘制 7 天折线/柱状组合图；页面会每 5 分钟自动刷新，提供错误重试，非管理员将显示无权限提示。
- **管理员用户总览**：新增 “用户总览” 页面，调用 `GET /api/admin/users` 展示分页用户列表、在线状态、Prompt 数量分布与最近活动；支持输入用户名/邮箱模糊搜索，并在卡片中汇总总用户数、在线数和当前页 Prompt 统计。
- **全局 Spotlight 搜索**：应用顶部新增 Spotlight 风格的全局搜索栏，输入关键词后按下回车即可跳转到“优质 Prompt 库”并带着同样的查询文案，方便跨页面快速检索。
- **公共 Prompt 评论区**：精选库详情弹层内新增评论面板，支持楼中楼回复、分页与管理员审核，使用 `/api/prompts/:id/comments` 与 `/api/prompts/comments/:id/review` 接口，评论成功会根据审核状态提示立即发布或等待审核。
- **评论点赞体验**：公共 Prompt 评论区支持点赞与取消点赞，按钮会即时刷新点赞数与当前状态，未登录或离线模式会提示无法操作。
  - 列表渲染读取 `like_count` 与 `is_liked` 字段，调用 `POST/DELETE /api/prompts/comments/:id/like` 更新点赞态。
  - 未通过审核的评论不可点赞；离线模式、未登录用户点击会提示 `comments.likeLoginRequired`。
- **Prompt 分享串**：Prompt 详情页新增“分享”按钮，调用 `POST /api/prompts/:id/share` 生成 `PGSHARE-*` 文本并复制；“我的 Prompt” 页签提供“导入分享串”对话框，粘贴 `PGSHARE-` 即可通过 `/api/prompts/share/import` 在当前账号下创建草稿。
- **创作者主页**：新增 `/creators/:id` 页面以霓虹 Hero + 指标卡 + 精选作品 + 全量列表四段式呈现投稿者，依赖新接口 `GET /api/creators/:id`；公共库详情和评论头像均支持点击跳转，前端对 `author.headline`/`bio` 等字段自动回退，离线同样只读浏览。
- **自动解析 Prompt**：在“我的 Prompt”页新增“自动解析 Prompt”入口（位于“更多操作”菜单），可粘贴既有 Prompt 正文并调用 `/api/prompts/ingest` 自动拆解主题、关键词与标签，解析成功后会自动打开工作台并载入新草稿；原有“导入分享串”“备份与导入设置”按钮现收纳在同一菜单，顶部仅保留“打开工作台”主按钮，避免操作区拥挤。
- **关键词配置**：根目录 `.env(.local)` 支持 `VITE_KEYWORD_ROW_LIMIT`（默认 3）与 `VITE_DEFAULT_KEYWORD_WEIGHT`（默认 5）等字段，可快速调节前端关键词与权重默认值。
- **请求与生成等待**：
  - `VITE_API_REQUEST_TIMEOUT_MS`（默认 60000）用于控制 Axios 请求超时时间，AI 生成等耗时操作可按需放宽。
  - `VITE_AI_GENERATE_MIN_DURATION_MS`（默认 2000）可调节 Prompt 工作台在生成提示时的最短加载展示时间。
- **内容审核提示**：当后端返回 `CONTENT_REJECTED` 时，工作台会抓取 `error.details.reason` 并通过 `promptWorkbench.contentRejected`、`promptWorkbench.contentRejectedFallbackReason` 两个 i18n 键展示多语言 toast，指导用户修改不合规文案。
- **邮箱验证体验**：验证页会自动填入账号邮箱、提供更醒目的发送按钮，并给出“如未收到可再次发送”的提示。
- **AI 生成等待时间**：通过 `VITE_AI_GENERATE_MIN_DURATION_MS`（默认 2000ms）调整 Prompt 工作台触发 AI 生成时的最短加载时长，配合 UI 提示让离线/低速环境下的长耗时更易感知。
- **自动保存草稿**：Prompt 工作台新增自动草稿功能，会在正文、关键词或标签修改后 **10 秒** 内（可通过 `VITE_PROMPT_AUTOSAVE_DELAY_MS` 调整）自动调用 `/api/prompts` 保存草稿；若 Redis 工作区可用，会同步刷新 `workspace_token`，离线模式则直接写入本地 SQLite。
- **偏好模型展示**：个人资料页的“偏好模型”字段现为只读，用于展示在模型凭据或工作台里设定的偏好值；Prompt 工作台加载时会优先选中该模型作为默认生成模型。
- **发布前校验提示**：当用户点击“发布”时，工作台会逐项检查主题、正文、补充要求、模型、正/负向关键词与标签是否齐全，缺失项会分别以 toast 提醒；保存草稿则允许字段为空，便于逐步完善。
- **Prompt 工作台布局**：补充要求与标签输入区改为纵向堆叠，标签按钮加宽并与输入框同列，避免小屏下换行。
- **关键词拖拽修复**：Prompt 工作台回填草稿时的关键词拖拽现在使用会话级唯一 ID，并仅在提交时回传真实 `keywordId`，彻底解决“总是拖动第一个关键词”和跨列排序失败的问题。
  前端现在给每个关键词做了两层 ID：
  - id：前端组件用来绑定拖拽/排序的标识。每次把数据装进工作台（不论是新增、接口返回还是从“我的 Prompt”回填）都会用 nanoid() 临时生成一个字符串，只存
    在于浏览器内存，用来喂给 dnd-kit。它不会写回数据库，也不进 Redis。
  - keywordId：保持后端原有的数字 ID，来自接口字段 keyword_id。同步到后端（保存、生成、同步排序等接口）时，只要这个字段有值，就带着它；如果是全新创
    建、后端还没分配 ID 的关键词，就带 keywordId: undefined，让后端按现有逻辑生成。
- **数据备份（JSON）**：`Settings.tsx` 的“应用设置”标签页提供“数据备份与导入”卡片，支持 JSON 导出/导入（合并/覆盖两种模式），会调用 `POST /api/prompts/export` 与 `POST /api/prompts/import`，并记录最近一次导出/导入的路径、数量与错误详情，操作完成后自动刷新 `MyPrompts` 列表。
- **点赞数据预埋**：后端已提供 `POST/DELETE /api/prompts/:id/like` 以及 `like_count` 字段，前端暂不在个人列表展示按钮，待公共库或协作视图启用后即可直接复用。
- **内置模型体验**：设置页会展示“DeepSeek 免费体验”条目，自动标注每日免费额度并默认启用；该条目不可编辑、删除或禁用，引导用户添加自有模型凭据以解锁完整能力。
- **动效升级**：全局外壳引入基于 Reactbits 灵感的 `AuroraBackdrop` 背景（轻量级渐变+噪点），并在认证完成或离线直接进入时显示一次性的 `EntryTransition` 过场动画，营造更柔和的视觉层次。
- **Dashboard 粒子背景**：Spotlight Hero 区域新增 React Bits 风格的粒子场背景，默认开启轻量级渐变粒子，参数可通过以下前端环境变量调节（`.env.example` 已提供默认值）：
  - `VITE_DASHBOARD_PARTICLE_COUNT`：粒子数量与排布密度，建议保持在 8~18 之间保证性能。
  - `VITE_DASHBOARD_PARTICLE_RADIUS`：粒子绕中心旋转的半径（px），影响覆盖范围。
  - `VITE_DASHBOARD_PARTICLE_DURATION`：完成一圈旋转的时长（秒），数值越大动画越舒缓。
  - `VITE_DASHBOARD_PARTICLE_SIZE_BASE` / `VITE_DASHBOARD_PARTICLE_SIZE_VARIANCE`：控制粒子基础尺寸与波动幅度，范围（px）。
  - `VITE_DASHBOARD_PARTICLE_DELAY_STEP`：相邻粒子在时间轴上的错位间隔（秒），避免同时重叠。
  - `VITE_DASHBOARD_PARTICLE_WAVE_FREQ` / `VITE_DASHBOARD_PARTICLE_WAVE_AMPLITUDE`：调节粒子尺寸振幅的频率与强度，打造轻微呼吸感。
  - `VITE_DASHBOARD_PARTICLE_ROTATION_DEGREES`：单次旋转角度，默认 360°，可用于反向或半圈效果。
  - `VITE_CARD_ANIMATION_DURATION` / `VITE_CARD_ANIMATION_OFFSET`：控制日志、帮助中心等卡片淡入动效的时长（秒）与初始位移（px），手机与桌面端统一体验。
  - `VITE_CARD_ANIMATION_STAGGER` / `VITE_CARD_ANIMATION_EASE`：用于调节卡片出场的错位间隔与贝塞尔曲线（格式为 `x1,y1,x2,y2`），可按需放大节奏或替换动效曲线。
- **动效开关**：设置页“应用设置”新增「界面动效」开关，可随时开启/关闭欢迎过场与背景光效，偏好会同步保存到用户设置。
- **Dashboard Spotlight Hero**：仪表盘顶区改为 Spotlight Hero 模式，融合欢迎语、快捷按钮与核心指标卡片，叠加柔光/噪点背景。
- **Dashboard Metrics**：指标卡新增“优质 Prompt”统计，同时保留三项原有卡片。
- **Magnetic 按钮**：在仪表盘 CTA 及设置页数据导入导出按钮启用磁吸动效，增强指针跟随反馈。
- **Spotlight 搜索框**：抽象出 `SpotlightSearch` 组件，并在仪表盘、我的 Prompt、公共 Prompt 库复用，保持光影聚焦与指针跟随体验。
- **优质 Prompt 库**：新增 “优质 Prompt 库” 页面，支持按关键词、状态筛选公共 Prompt，并一键导入到个人库（离线模式保留浏览与下载能力，投稿在在线模式开放）；普通用户在筛选“待审核 / 已驳回”时只会看到自己的投稿记录。最新版本增加了综合评分排序，列表可按照评分、下载量、点赞数、浏览量或更新时间切换，卡片与详情页均展示质量评分数值；评分由后台任务定期刷新（默认 5 分钟），界面上存在轻微延迟属正常表现。
- **分页摘要优化**：公共库分页默认每页加载 9 条，`My Prompts` 默认 10 条，并在底部显示 `current_count` 提示“本页条目数”，便于确认栅格/表格填充情况（同字段已在接口 `meta.current_count` 返回；可通过 `VITE_PUBLIC_PROMPT_LIST_PAGE_SIZE`、`VITE_MY_PROMPTS_PAGE_SIZE` 调整默认条数）。
- **我的 Prompt 快捷发布**：列表操作区新增“立即发布”按钮，无需进入工作台即可触发发布流程，沿用工作台发布校验逻辑，确保主题、正文、补充要求、关键词及标签齐备。
- **更新日志焕新**：更新日志页面改用帮助中心同款的玻璃态卡片与高光背景，阅读体验与帮助文档保持一致。
- **帮助中心 Scroll Stack**：`Help.tsx` 采用滚动联动卡片布局，左侧章节索引与右侧卡片堆叠联动，逐步呈现操作指南。
- **版本历史**：Prompt 详情页左侧新增“版本历史”卡片，可浏览、预览历史版本并一键加载到工作台继续编辑。
- **Prompt 投稿按钮**：Prompt 详情页右上新增“投稿到公共库”入口，仅在 Prompt 状态为已发布时展示；支持基于当前或指定历史版本一键提交至公共库，并自定义摘要、标签与展示语言。
- **公共库审核面板**：管理后台追加“公共库审核”菜单，提供分页搜索、快速审批与驳回原因记录，并新增“删除公共 Prompt”按钮，支持在审核前后移除不合规条目；对应 `/admin/public-prompts` 页面完成投稿审核流。审核详情面板同步展示综合评分，便于筛选重点内容。

## 已实现功能概览

- **用户注册/登录**：注册页集成图形验证码与头像上传，支持自动重试、行内校验与失败提示；当邮箱与用户名同时冲突时，会逐项标记冲突字段；登录页支持邮箱或用户名登录，提供“记住此账号”选项，并通过全局通知（Toast）反馈成功或错误状态。

### 记住账号实现

- 登录页勾选“记住此账号”时，会把当前输入的邮箱/用户名写入 `localStorage`（键名 `promptgen:last-identifier`）；进入页面时读取并回填，同时自动勾选复选框。
- 若用户未勾选或在提交前取消勾选，登录成功后会删除该键，不再自动填充账号；访问令牌仍通过 tokenStorage 管理，与账号记住功能无关。
- **邮箱验证提示**：登录页与个人信息页共用的验证状态面板会展示剩余重发次数并实时响应后端广播，保证用户在任意入口都能看到最新验证进度。
- **注册反馈增强**：注册成功后会自动触发邮箱验证邮件并弹出提示，指引用户前往收件箱继续完成流程。
- **仪表盘**：`Dashboard.tsx` 提供工作台概览，包括关键指标卡、近期活动、提示词表现表格、关键词洞察等模块，现已直接从后端拉取实时数据并在加载阶段显示骨架态。
- **Prompt 工作台（新版）**：新增“自然语言需求解析 → AI 补词 → 模型生成 → 草稿/发布”一站式流程；支持实时解析置信度、模型可用性联动、手动/自动关键词去重与来源标记；补充要求与标签编辑器上下排列，支持快速添加/移除并显示数量上限。
- **关键词权重与拖拽**：正、负向关键词支持 0~5 的相关度权重，点击 ± 调整；拖拽即可跨列移动，需要时再点击“按权重排序”按钮整理顺序。
- **关键词同步优化**：仅当权重或排序实际发生变化时才触发 `/api/prompts/keywords/sync`，并强化跨列拖拽的视觉反馈，减少多余请求同时提升操作感受。
- **交互细节**：需求解析下方直接提供手动添加与「AI 补充关键词」引导，中列仅保留正/负向关键字池，排序按钮固定在行尾；解析/补词/生成等长时操作会弹出 Toast 进度条。
- **模型凭据管理**：设置页新增模型列表与创建表单，可启用/禁用、删除模型并一键设为偏好，支持额外 JSON 配置持久化。
- **模型可用性联动**：Prompt 工作台智能感知模型启用状态，自动回退到可用模型并提示用户，避免引用失效配置。
- **设置中心**：支持头像上传/移除、用户名/邮箱/偏好模型的编辑，同时保留语言与主题偏好；可在此导出本地数据或调整数据库路径。
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

# 构建生产包（默认面向 Web 部署）
npm run build

# 构建 Electron 桌面壳专用包（保留相对资源路径）
npm run build:electron

# 本地预览构建结果
npm run preview
```

### 开发端口说明

> 提示：若需与桌面壳同步调试，请在仓库根目录执行 `npm start`，脚本会通过 `scripts/start-electron.cjs` 调用本地安装的 Electron，避免 Windows 环境出现 `'electron' 不是内部或外部命令'` 的报错。

- `5173`：Vite 开发服务器默认端口。执行 `npm run dev` 时，浏览器或 Electron 渲染进程会从该端口加载最新的热更新页面。
- `4173`：`npm run preview` 使用的预览端口，用于本地验证生产构建的效果。

若要部署到独立的 Web 服务器，请确保构建时 `VITE_PUBLIC_BASE_PATH=/`，以便静态资源在任意路径下都能正确加载；Electron 打包则沿用默认的 `./` 即可兼容 `file://` 协议。

前端的 API 客户端（`src/lib/http.ts`）默认直接指向 `http://localhost:9090/api`（Go 后端监听端口）。若需要接入远程环境，可以在根目录 `.env.local` 中设置 `VITE_API_BASE_URL` 覆盖默认地址，构建与运行都会读取该变量。上传成功的头像会返回 `/static/avatars/<文件名>` 路径，由后端静态资源路由托管。

关键词与标签的数量与长度上限均可通过环境变量调节：在根目录 `.env.local` 中设置 `VITE_PROMPT_KEYWORD_LIMIT=10`、`VITE_PROMPT_KEYWORD_MAX_LENGTH=32`、`VITE_PROMPT_TAG_LIMIT=5`、`VITE_PROMPT_TAG_MAX_LENGTH=5`（需与后端的 `PROMPT_*` 对应项保持一致），即可同步约束提示词与标签的数量和字符长度，避免前后端配置漂移。

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
   │  └─ ui/
   │     ├─ badge.tsx              # Capsule 风格徽章
   │     ├─ button.tsx             # 多 Variant 按钮
   │     ├─ glass-card.tsx         # 毛玻璃卡片容器，支持透传原生事件
   │     ├─ magnetic-button.tsx    # 指针跟随磁吸按钮
   │     └─ spotlight-search.tsx   # Spotlight 风格搜索框，带渐变光晕与指针跟随
   ├─ pages/
   │  ├─ Dashboard.tsx             # 仪表盘页面，展示指标卡、动态与提示词表现
   │  ├─ Login.tsx                 # 登录表单，内置行内校验与 Toast 成功/失败提示
   │  ├─ Register.tsx              # 注册表单，内置验证码刷新、自动重试与行内校验提示
   │  ├─ PromptWorkbench.tsx       # 提示词工作台，整合关键词管理与草稿编辑
   │  ├─ PublicPrompts.tsx         # 优质 Prompt 列表，支持搜索、筛选与下载/导入
   │  ├─ AdminPublicPrompts.tsx    # 公共库审核页，管理员审批/驳回投稿
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

### 公共 Prompt 审核流程

- Prompt 详情页的“投稿到公共库”按钮会基于当前或选中的历史版本生成投稿载荷，支持自定义摘要、标签与展示语言并同步限流配置（`PUBLIC_PROMPT_SUBMIT_*`）。相关表单默认行为可通过以下变量微调：`VITE_PUBLIC_PROMPT_SUMMARY_PARAGRAPHS`、`VITE_PUBLIC_PROMPT_SUMMARY_ROWS`、`VITE_PUBLIC_PROMPT_DEFAULT_LANGUAGE`。
- 侧边栏新增“公共库审核”入口（仅管理员可见），对应 `/admin/public-prompts` 页面，提供搜索、状态筛选、分页浏览与审批/驳回操作。页面分页与驳回输入框行数分别受 `VITE_PUBLIC_PROMPT_REVIEW_PAGE_SIZE` 与 `VITE_PUBLIC_PROMPT_REVIEW_REASON_ROWS` 控制。
- 审核通过后会刷新公共库与个人库缓存；驳回时可选填原因（写入 `review_reason` 字段），空值代表静默驳回。所有操作即时反馈到 `react-query` 缓存，并提示操作结果。

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

## 离线预置数据联调

- 客户端打包时会把 `backend/data/bootstrap/*.json` 一并带入安装目录，渲染进程通过 `useAppSettings` 识别离线模式后，直接向本地后端请求 `/api/changelog`、`/api/public-prompts`，从 SQLite 中读取刚刚导入的离线条目。
- 若需在开发环境验证离线表现，先按后端文档运行：

  ```bash
  go run ./backend/cmd/export-offline-data -output-dir backend/data/bootstrap
  ```

  再执行：

  ```bash
  go run ./backend/cmd/offline-bootstrap -output ./release/assets/promptgen-offline.db
  ```

  然后将 `LOCAL_SQLITE_PATH` 指向生成的数据库或清空本地数据库后重启 Electron，即可看到 changelog 页面和公共 Prompt 列表在离线模式下的完整数据。
- 发布前请至少执行一次 `npm run dist:win`（或 `dist:mac`）验证安装包，确认 `resources/app/backend/data/bootstrap/` 中的 JSON 已随包分发；若缺失，请重新运行导出命令并打包。
- 所有删除类操作统一使用 `ConfirmDialog` 玻璃态弹窗组件，提供键盘 Esc 关闭与加载态提示，避免浏览器原生弹窗带来的风格割裂。
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
