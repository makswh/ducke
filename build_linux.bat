@echo off
setlocal enabledelayedexpansion

echo ========================================================
echo   Ducke Linux ^& Flatpak Builder for Steam Deck / SteamOS
echo ========================================================
echo.

set "SCRIPT_DIR=%~dp0"
if "%SCRIPT_DIR:~-1%"=="\" set "SCRIPT_DIR=%SCRIPT_DIR:~0,-1%"

cd /d "%SCRIPT_DIR%\frontend"
echo [1/3] Building frontend (Vite/Svelte)...
call npm run build
if %ERRORLEVEL% neq 0 (
    echo.
    echo [-] Frontend build failed!
    exit /b %ERRORLEVEL%
)

cd /d "%SCRIPT_DIR%"
echo [2/3] Resolving WSL environment...
set "FORWARD_DIR=%SCRIPT_DIR:\=/%"
for /f "usebackq tokens=*" %%i in (`wsl.exe -d Ubuntu-22.04 wslpath -u "!FORWARD_DIR!"`) do set "WSL_PROJECT_DIR=%%i"

if not defined WSL_PROJECT_DIR (
    echo.
    echo [-] Error: Failed to resolve WSL path for "%SCRIPT_DIR%".
    echo     Please verify that WSL is installed and operational.
    exit /b 1
)

echo Project directory in WSL: %WSL_PROJECT_DIR%
echo.
echo [3/3] Compiling Linux binary ^& packaging Flatpak bundle in WSL...
wsl.exe -d Ubuntu-22.04 -u root -e bash "%WSL_PROJECT_DIR%/build_linux.sh" "%WSL_PROJECT_DIR%"
if %ERRORLEVEL% neq 0 (
    echo.
    echo [-] WSL build failed!
    exit /b %ERRORLEVEL%
)

echo.
echo ========================================================
echo   BUILD FINISHED SUCCESSFULLY!
echo ========================================================
echo.
echo Artifacts created:
echo   - Native Binary:     build\bin\Ducke
echo   - Flatpak Bundle:    build\bin\Ducke.flatpak
echo   - 1-Click Installer: build\bin\install_steamdeck.sh
echo.
echo To install on Steam Deck:
echo   1. Copy build\bin\Ducke.flatpak and install_steamdeck.sh to your Steam Deck.
echo   2. In Desktop Mode, double-click install_steamdeck.sh (or Ducke.flatpak).
echo.