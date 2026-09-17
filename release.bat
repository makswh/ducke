@echo off
setlocal enabledelayedexpansion

set "SCRIPT_DIR=%~dp0"
if "%SCRIPT_DIR:~-1%"=="\" set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%SCRIPT_DIR%\scripts\release.ps1" %*
if %ERRORLEVEL% neq 0 (
    echo.
    echo [-] Release process failed with error code %ERRORLEVEL%.
    exit /b %ERRORLEVEL%
)

exit /b 0
