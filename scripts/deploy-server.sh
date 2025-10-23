#!/bin/bash
###
 # @Author: NEFU AB-IN
 # @Date: 2025-10-23 22:21:50
 # @FilePath: \electron-go-app\scripts\deploy-server.sh
 # @LastEditTime: 2025-10-23 22:40:05
### 
set -euo pipefail

REPO=/opt/promptgen/Prompt-Gen
TMP=/opt/promptgen/tmp
SERVICE=promptgen

log(){ echo "[$(date '+%F %T')] $*"; }

log "停止服务"
systemctl stop "$SERVICE" || true

log "替换后端"
install -m 0755 "$TMP/server" "$REPO/bin/server"

log "替换前端静态资源"
rm -rf "$REPO/frontend/dist"
mv "$TMP/dist" "$REPO/frontend/dist"

log "启动服务"
systemctl start "$SERVICE"
systemctl status "$SERVICE" --no-pager

log "部署完成"