#!/bin/bash
###
 # @Author: NEFU AB-IN
 # @Date: 2025-10-23 22:21:50
 # @FilePath: \electron-go-app\scripts\deploy-server.sh
 # @LastEditTime: 2025-10-23 23:25:54
### 
set -euo pipefail

REPO=/opt/promptgen/Prompt-Gen
TMP=/opt/promptgen/tmp
SERVICE=promptgen
FRONT_DIR="${REPO}/frontend"
DIST_DIR="${FRONT_DIR}/dist"

log(){ echo "[$(date '+%F %T')] $*"; }

log "停止服务"
systemctl stop "$SERVICE" || true

log "替换后端"
mkdir -p "${REPO}/bin"
install -m 0755 "$TMP/server" "$REPO/bin/server"

if [[ -f "${DIST_DIR}/.user.ini" ]]; then
  log "移除 dist 目录下的 .user.ini"
  chattr -i "${DIST_DIR}/.user.ini" 2>/dev/null || true
  rm -f "${DIST_DIR}/.user.ini" || true
fi

if [[ -d "$TMP/dist" ]]; then
  log "检测到上传的前端产物，直接覆盖"
  rm -rf "${DIST_DIR}"
  mv "$TMP/dist" "${DIST_DIR}"
else
  log "未检测到上传的前端产物，服务器本地重新构建"
  rm -rf "${DIST_DIR}"
  log "安装前端依赖（如已安装可忽略输出）"
  npm --prefix "${FRONT_DIR}" ci
  log "执行前端构建"
  npm --prefix "${FRONT_DIR}" run build
fi

log "启动服务"
systemctl start "$SERVICE"
systemctl status "$SERVICE" --no-pager

log "部署完成"
