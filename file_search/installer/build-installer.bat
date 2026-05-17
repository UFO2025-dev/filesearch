@echo off
REM build-installer.bat — Run this on Windows to build FileSearch-Setup-v1.0.1.exe
REM Requirements: Inno Setup 6 installed at default location

SET ISCC="C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
if not exist %ISCC% SET ISCC="C:\Program Files\Inno Setup 6\ISCC.exe"

if not exist %ISCC% (
    echo ERROR: Inno Setup 6 not found.
    echo Download from: https://jrsoftware.org/isdl.php
    pause
    exit /b 1
)

echo Building FileSearch installer...
%ISCC% installer.iss

if %ERRORLEVEL% EQU 0 (
    echo.
    echo SUCCESS: FileSearch-Setup-v1.0.1.exe created!
) else (
    echo.
    echo ERROR: Build failed. Check output above.
)
pause
