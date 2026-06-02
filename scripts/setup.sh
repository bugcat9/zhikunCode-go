#!/usr/bin/env bash
# 一次性环境准备：python-service 的 venv + 依赖，frontend 的 node_modules
# 已经准备过的子项目会自动跳过，安全重跑。
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

log() { printf '\033[1;36m[setup]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[setup]\033[0m %s\n' "$*"; }

# ─── python-service ───
# 默认用 python3；如要指定版本，PYTHON=/opt/homebrew/bin/python3.11 bash scripts/setup.sh
PYTHON="${PYTHON:-python3}"
PY_DIR="$REPO_ROOT/python-service"
VENV="$PY_DIR/.venv"

if ! command -v "$PYTHON" >/dev/null 2>&1; then
  warn "未找到 $PYTHON，跳过 python-service 安装。"
else
  PY_VERSION="$("$PYTHON" -c 'import sys; print("%d.%d" % sys.version_info[:2])')"
  log "使用 Python $PY_VERSION ($PYTHON)"
  case "$PY_VERSION" in
    3.11) ;;  # 推荐版本
    3.12|3.13)
      warn "Python $PY_VERSION 上 tree-sitter-languages 没有 wheel，pip install 大概率失败。"
      warn "推荐：brew install python@3.11，然后 PYTHON=/opt/homebrew/bin/python3.11 bash scripts/setup.sh"
      ;;
    *)
      warn "Python $PY_VERSION 未在项目支持范围（要求 3.11-3.12），可能会有兼容性问题。"
      ;;
  esac

  if [ ! -d "$VENV" ]; then
    log "创建 python-service venv ..."
    "$PYTHON" -m venv "$VENV"
  else
    log "python-service venv 已存在，跳过创建。如要换 Python 版本，先 rm -rf $VENV"
  fi

  log "安装 python-service 依赖（requirements.txt，可能耗时几分钟）..."
  # shellcheck disable=SC1091
  source "$VENV/bin/activate"
  pip install --upgrade pip >/dev/null
  pip install -r "$PY_DIR/requirements.txt"
  deactivate
  log "python-service 准备完成。"
fi

# ─── frontend ───
FE_DIR="$REPO_ROOT/frontend"

if ! command -v npm >/dev/null 2>&1; then
  warn "未找到 npm，跳过 frontend 安装。"
else
  if [ ! -d "$FE_DIR/node_modules" ]; then
    log "安装 frontend 依赖 ..."
    (cd "$FE_DIR" && npm install)
  else
    log "frontend node_modules 已存在，跳过安装（如需更新，删掉后重跑）。"
  fi
  log "frontend 准备完成。"
fi

log "全部完成。下一步："
echo "  Terminal 1: ./scripts/dev-python.sh"
echo "  Terminal 2: ./scripts/dev-frontend.sh"