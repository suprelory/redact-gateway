@echo off
echo Building Redact Gateway...
echo.

cd backend
go build -o ..\gateway.exe .\cmd\gateway

if %errorlevel% neq 0 (
    echo Build failed!
    exit /b %errorlevel%
)

echo Build complete: gateway.exe
echo.
echo Usage:
echo   set ADMIN_TOKEN=your-secure-token
echo   gateway.exe
