@echo off
echo Building Ducke Linux/SteamOS binary with Docker...
docker build -f build/linux/Dockerfile --output type=local,dest=build/bin .
if %ERRORLEVEL% equ 0 (
    echo [SUCCESS] Linux binary compiled to build\bin\Ducke
) else (
    echo [ERROR] Docker build failed.
)
