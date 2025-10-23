#!/bin/bash
###
 # @Author: NEFU AB-IN
 # @Date: 2025-10-23 22:21:37
 # @FilePath: \electron-go-app\scripts\deploy-online.sh
 # @LastEditTime: 2025-10-23 22:57:58
### 
set -euo pipefail

TARGET=AB-IN                             # 服务器 SSH 别名
TMP_DIR=/opt/promptgen/tmp               # 服务器暂存目录
REMOTE_SCRIPT=/opt/promptgen/Prompt-Gen/scripts/deploy-server.sh

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$ROOT/deploy"                 # 临时输出目录
DIST_DIR="$ROOT/frontend/dist"

log(){ echo "[$(date '+%F %T')] $*"; }

log "清理临时产物目录"
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

log "构建后端二进制 (Linux/amd64)"
go env -w GOPROXY=https://proxy.golang.com.cn,direct
go mod download
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$BUILD_DIR/server" ./backend/cmd/server

log "仅安装前端依赖并构建"
npm --prefix frontend ci
npm --prefix frontend run build

log "上传后端"
scp "$BUILD_DIR/server" "$TARGET:$TMP_DIR/server"

log "上传前端 dist"
rsync -av --delete "$DIST_DIR/" "$TARGET:$TMP_DIR/dist/"

log "赋权并执行远端部署脚本"
ssh "$TARGET" "chmod +x $TMP_DIR/server && bash $REMOTE_SCRIPT"

log "部署完成"
