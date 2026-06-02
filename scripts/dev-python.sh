#!/usr/bin/env bash
# 启动 python-service（FastAPI + uvicorn，热重载）于 http://localhost:8000
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PY_DIR="$REPO_ROOT/python-service"
VENV="$PY_DIR/.venv"

if [ ! -d "$VENV" ]; then
  echo "[dev-python] 未找到 venv，请先执行：./scripts/setup.sh" >&2
  exit 1
fi

# shellcheck disable=SC1091
source "$VENV/bin/activate"

# main.py 里写的是 from capabilities import ...（不带 src. 前缀），
# 所以要在 src/ 目录里跑，让 python 把 src/ 加进 sys.path。
cd "$PY_DIR/src"
exec python main.py