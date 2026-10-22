@echo off
echo ========================================
echo    C2-DECT Lite Build System v2.0
echo ========================================
echo.

:: Create build directory
if not exist "build" mkdir build

echo [+] Building server...
cd server
go build -ldflags="-s -w" -o ..\build\server.exe .\cmd\main.go
if %errorlevel% neq 0 (
    echo [-] Server build failed!
    cd ..
    pause
    exit /b 1
)
cd ..
echo [+] Server built successfully

echo [+] Building agent...
cd agent
go build -ldflags="-s -w" -o ..\build\agent.exe .
if %errorlevel% neq 0 (
    echo [-] Agent build failed!
    cd ..
    pause
    exit /b 1
)
cd ..
echo [+] Agent built successfully

echo [+] Building GUI...
cd gui\C2Dect
dotnet build -c Release
if %errorlevel% neq 0 (
    echo [-] GUI build failed!
    cd ..\..\..
    pause
    exit /b 1
)
cd ..\..\..
echo [+] GUI built successfully

echo.
echo ========================================
echo    Build Complete!
echo ========================================
echo.
echo Binaries location: build\
echo   - server.exe
echo   - agent.exe
echo   - gui\C2Dect\bin\Release\net8.0-windows\C2Dect-Lite.exe
echo.
echo Quick start:
echo   1. set C2_JWT_SECRET and C2_BOOTSTRAP_USER / C2_BOOTSTRAP_PASSWORD
echo   2. Start server: build\server.exe
echo   3. Start agent:  build\agent.exe -s 127.0.0.1 -p 8443
echo   4. Open GUI:     gui\C2Dect\bin\Release\net8.0-windows\C2Dect-Lite.exe
echo.
echo There are no default credentials: the administrator is created on the
echo first start from C2_BOOTSTRAP_USER / C2_BOOTSTRAP_PASSWORD.
echo.
pause
