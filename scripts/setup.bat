@echo off
chcp 65001 >nul 2>&1
setlocal

rem One-time environment setup:
rem - python-service: create .venv and install requirements.txt
rem - frontend: install node_modules
rem Already prepared subprojects are skipped safely.

set "REPO_ROOT=%~dp0.."
pushd "%REPO_ROOT%" >nul
if errorlevel 1 exit /b 1
set "REPO_ROOT=%CD%"
popd >nul

rem ----- python-service -----
rem Default: python. To choose another interpreter:
rem   set PYTHON=C:\Python311\python.exe
rem   scripts\setup.bat
if "%PYTHON%"=="" set "PYTHON=python"

set "PY_DIR=%REPO_ROOT%\python-service"
set "VENV=%PY_DIR%\.venv"
set "VENV_PY=%VENV%\Scripts\python.exe"

rem Force UTF-8 inside Python/pip so UTF-8 requirements comments do not fail
rem on Windows systems whose default ANSI code page is GBK or another locale.
set "PYTHONUTF8=1"
set "PYTHONIOENCODING=utf-8"

"%PYTHON%" -c "import sys" >nul 2>&1
if errorlevel 1 (
    echo [setup] Python not found: %PYTHON%
    echo [setup] Skipping python-service setup.
    goto :frontend
)

for /f "tokens=2 delims= " %%V in ('call "%PYTHON%" --version 2^>^&1') do set "PY_VERSION_FULL=%%V"
for /f "tokens=1,2 delims=." %%A in ("%PY_VERSION_FULL%") do set "PY_VERSION=%%A.%%B"
echo [setup] Using Python %PY_VERSION% (%PYTHON%)

if "%PY_VERSION%"=="3.12" (
    echo [setup] Warning: Python %PY_VERSION% may not have a tree-sitter-languages wheel.
    echo [setup] Recommended: Python 3.11.x.
)
if "%PY_VERSION%"=="3.13" (
    echo [setup] Warning: Python %PY_VERSION% may not have a tree-sitter-languages wheel.
    echo [setup] Recommended: Python 3.11.x.
)

if not exist "%VENV%" (
    echo [setup] Creating python-service venv ...
    "%PYTHON%" -m venv "%VENV%"
    if errorlevel 1 exit /b 1
) else (
    echo [setup] python-service venv already exists; skipping creation.
    echo [setup] To change Python version, delete: %VENV%
)

if not exist "%VENV_PY%" (
    echo [setup] venv python not found: %VENV_PY%
    exit /b 1
)

echo [setup] Installing python-service dependencies ...
"%VENV_PY%" -m pip install --upgrade pip
if errorlevel 1 exit /b 1
"%VENV_PY%" -m pip install -r "%PY_DIR%\requirements.txt"
if errorlevel 1 exit /b 1
echo [setup] python-service is ready.

:frontend
rem ----- frontend -----
set "FE_DIR=%REPO_ROOT%\frontend"

where npm >nul 2>&1
if errorlevel 1 (
    echo [setup] npm not found.
    echo [setup] Skipping frontend setup.
    goto :done
)

if not exist "%FE_DIR%\node_modules" (
    echo [setup] Installing frontend dependencies ...
    pushd "%FE_DIR%" >nul
    if errorlevel 1 exit /b 1
    call npm install
    if errorlevel 1 (
        popd >nul
        exit /b 1
    )
    popd >nul
) else (
    echo [setup] frontend node_modules already exists; skipping install.
    echo [setup] To refresh dependencies, delete: %FE_DIR%\node_modules
)
echo [setup] frontend is ready.

:done
echo [setup] All done. Next steps:
echo   Terminal 1: scripts\dev-python.bat
echo   Terminal 2: scripts\dev-frontend.bat

endlocal
exit /b 0
