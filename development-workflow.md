# 开发与发布流程指引

本文记录从本地开发到线上部署的标准流程，便于团队成员快速对齐操作步骤。请结合项目根目录 `README.md`、`backend/backend_README.md` 与 `frontend/frontend_README.md` 一起阅读。

---

## 0. 开始前确认

- 已在本地完成开发环境初始化（依赖安装、环境变量配置等）。
- 完成本次需求的功能编码，并通过自测。
- 若涉及前端文案与说明，请同步更新多语言文案及相关 README。

---

## 1. 同步公共库离线数据

离线版本及桌面端默认会携带公共 Prompt、Changelog 等预置数据。在发布前，请务必同步线上库数据：

```bash
go run ./backend/cmd/export-offline-data -output-dir backend/data/bootstrap
```

执行完后确认 `backend/data/bootstrap/public_prompts.json`、`changelog_entries.json` 已更新，并纳入后续提交。

> 如需在本地验证导入效果，可运行 `go run ./backend/cmd/offline-bootstrap -output ./release/assets/promptgen-offline.db`，再将 `LOCAL_SQLITE_PATH` 指向该文件后启动桌面端。

---

## 2. 运行必要的构建 / 测试

按照改动范围，至少完成以下校验：

- **后端**：`go test ./...`（如涉及 integration / e2e，请根据 README 说明补充运行）。
- **前端**：`npm --prefix frontend run lint`、`npm --prefix frontend run build`。
- **桌面端离线包**（如有 UI/数据更新）：视情况执行 `npm run prepare:offline` 或 `npm run dist:win|dist:mac` 验证。

---

## 3. 提交代码

1. 检查工作区：`git status`，确认仅有本次改动。
2. 将文件加入暂存区：`git add ...`。
3. 使用中文提交信息（遵循现有 Conventional Commit 习惯，例如 `feat: 增加公共库删除弹窗`）。
4. 如涉及版本发布，使用 `npm version patch|minor|major` 调整版本号（默认 patch），确保生成 tag 前完成。

---

## 4. 推送与标签

```bash
git push origin <branch>
git push origin <tag>    # 如果通过 npm version 生成了标签
```

GitHub Actions 会依据 tag 触发构建与 Release（具体流程可参考 `.github/workflows/` 目录）。提交前请再次确认版本号、变更说明与离线数据成果均已在仓库中。

---

## 5. 同步到线上服务器（在线模式）

如需更新线上服务器，请在本地运行：

```bash
scripts/deploy-online.sh
```

脚本说明：

- 默认使用 SSH 别名 `AB-IN` 连接服务器，请确保 `~/.ssh/config` 已配置该别名。
- 会在本地重新编译 Linux/amd64 后端二进制，并上传到服务器临时目录。
- 远端自动调用 `/opt/promptgen/Prompt-Gen/scripts/deploy-server.sh`：
  - 停止 `systemctl` 中的 `promptgen` 服务。
  - 覆盖后端二进制，按需构建/替换前端产物。
  - 重启并输出服务状态。

> 如需携带手动构建的前端 `dist/`，请提前将产物放入服务器的 `/opt/promptgen/tmp/dist`，脚本会优先复用该目录。

---

## 6. Release 验证

- 在 GitHub Release 页面确认最新 Tag 已生成对应的安装包 / 资源。
- 验证线上站点（或桌面端包）功能是否符合预期，重点检查：
  - 公共 Prompt、Changelog 等离线数据是否最新。
  - 新增的删除弹窗（ConfirmDialog）是否生效。
  - 新功能及修复点工作正常。

如发现问题，请及时回滚或追加修复。

---

## 7. 其它注意事项

- 文档变更：如涉及前后端同时改动，请同步更新两侧 README，并在根 README 添加必要说明。
- 版本回退：若需回滚部署，记得保留上一版编译产物或 Tag，以便快速恢复。
- 敏感参数：服务器部署依赖 `.env` / `.env.local`，请勿将真实凭据写入仓库。

---

如流程有变动或需启用自动化脚本，请在本文件同步更新，保持团队对齐。
