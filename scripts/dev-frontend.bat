@echo off
chcp 65001 >nul 2>&1
setlocal

rem Start frontend Vite dev server at http://localhost:5173.

set "REPO_ROOT=%~dp0.."
pushd "%REPO_ROOT%" >nul
if errorlevel 1 exit /b 1
set "REPO_ROOT=%CD%"
popd >nul

set "FE_DIR=%REPO_ROOT%\frontend"
set "ENV_FILE=%FE_DIR%\.env.development"
set "VITE_API_URL=http://localhost:8080"
set "VITE_PYTHON_URL=http://localhost:8000"

where npm >nul 2>&1
if errorlevel 1 (
    echo [dev-frontend] npm not found. Install Node.js first. >&2
    exit /b 1
)

if not exist "%FE_DIR%\node_modules" (
    echo [dev-frontend] node_modules not found. Run scripts\setup.bat first. >&2
    exit /b 1
)

if exist "%ENV_FILE%" (
    for /f "usebackq tokens=1,* delims==" %%A in ("%ENV_FILE%") do (
        if "%%A"=="VITE_API_URL" set "VITE_API_URL=%%B"
        if "%%A"=="VITE_PYTHON_URL" set "VITE_PYTHON_URL=%%B"
    )
)

call :check_url "%VITE_API_URL%"
if errorlevel 1 (
    echo [dev-frontend] Warning: main backend is not reachable: %VITE_API_URL%
    echo [dev-frontend] Requests like /api/models, /api/config, /api/sessions and /ws will fail until it is started.
)

call :check_url "%VITE_PYTHON_URL%"
if errorlevel 1 (
    echo [dev-frontend] Warning: python-service is not reachable: %VITE_PYTHON_URL%
    echo [dev-frontend] Routes like /api/files, /api/git, /api/code-quality and /api/analysis will fail until it is started.
)

pushd "%FE_DIR%" >nul
if errorlevel 1 exit /b 1

echo [dev-frontend] Starting frontend at http://localhost:5173 ...
call npm run dev
set "EXIT_CODE=%ERRORLEVEL%"

popd >nul
endlocal & exit /b %EXIT_CODE%

:check_url
powershell -NoProfile -ExecutionPolicy Bypass -Command "$u=[Uri]('%~1'); $p=if($u.IsDefaultPort){if($u.Scheme -eq 'https'){443}else{80}}else{$u.Port}; $c=New-Object Net.Sockets.TcpClient; try { $ar=$c.BeginConnect($u.Host,$p,$null,$null); if(-not $ar.AsyncWaitHandle.WaitOne(1000,$false)){exit 1}; $c.EndConnect($ar); exit 0 } catch { exit 1 } finally { $c.Close() }" >nul 2>&1
exit /b %ERRORLEVEL%
