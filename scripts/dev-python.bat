@echo off
chcp 65001 >nul 2>&1
setlocal

rem Start python-service (FastAPI service) at http://localhost:8000.

set "REPO_ROOT=%~dp0.."
pushd "%REPO_ROOT%" >nul
if errorlevel 1 exit /b 1
set "REPO_ROOT=%CD%"
popd >nul

set "PY_DIR=%REPO_ROOT%\python-service"
set "VENV=%PY_DIR%\.venv"
set "VENV_PY=%VENV%\Scripts\python.exe"

set "PYTHONUTF8=1"
set "PYTHONIOENCODING=utf-8"

if not exist "%VENV_PY%" (
    echo [dev-python] venv not found. Run scripts\setup.bat first. >&2
    exit /b 1
)

cd /d "%PY_DIR%\src"
if errorlevel 1 exit /b 1

echo [dev-python] Starting python-service at http://localhost:8000 ...
"%VENV_PY%" main.py
set "EXIT_CODE=%ERRORLEVEL%"

endlocal & exit /b %EXIT_CODE%
