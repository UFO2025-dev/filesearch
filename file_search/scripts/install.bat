@echo off
setlocal EnableDelayedExpansion

:: ════════════════════════════════════════════════════════
::  FileSearch — Installateur Windows
::  Usage: install.bat [--uninstall]
:: ════════════════════════════════════════════════════════

set APP_NAME=FileSearch
set INSTALL_DIR=%USERPROFILE%\FileSearch
set WSL_DISTRO=Ubuntu
set WSL_SOURCE=~/surveillance/gatewatch_mvp/file_search
set DESKTOP=%USERPROFILE%\Desktop
set STARTUP=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup

echo.
echo  ╔══════════════════════════════════════╗
echo  ║   FileSearch — Installateur v1.0    ║
echo  ╚══════════════════════════════════════╝
echo.

:: ── Mode désinstallation ──────────────────────────────
if "%1"=="--uninstall" goto uninstall

:: ── Vérifier WSL disponible ──────────────────────────
wsl -d %WSL_DISTRO% -e bash -lc "echo ok" >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERREUR] WSL Ubuntu introuvable.
    echo Installez WSL2 Ubuntu depuis le Microsoft Store.
    pause
    exit /b 1
)

:: ── Vérifier que le binaire existe ───────────────────
wsl -d %WSL_DISTRO% -e bash -lc "test -f %WSL_SOURCE%/filesearch-server" >nul 2>&1
if %errorlevel% neq 0 (
    echo [INFO] Binaire absent - compilation en cours...
    wsl -d %WSL_DISTRO% -e bash -lc "cd %WSL_SOURCE% && CGO_ENABLED=0 go build -ldflags='-s -w' -o filesearch-server ./cmd/server"
    if %errorlevel% neq 0 (
        echo [ERREUR] Compilation echouee. Verifiez que Go est installe dans WSL.
        pause
        exit /b 1
    )
    echo [OK] Binaire compile.
)

:: ── Créer dossier d'installation ─────────────────────
echo [1/4] Creation du dossier %INSTALL_DIR% ...
if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if not exist "%INSTALL_DIR%\data" mkdir "%INSTALL_DIR%\data"

:: ── Copier le launcher .bat ───────────────────────────
echo [2/4] Copie du launcher...
copy /y "%~dp0filesearch.bat" "%INSTALL_DIR%\filesearch.bat" >nul
if %errorlevel% neq 0 (
    echo [ERREUR] Impossible de copier filesearch.bat
    echo Verifiez que vous lancez install.bat depuis le dossier scripts\
    pause
    exit /b 1
)

:: ── Créer raccourci Bureau ────────────────────────────
echo [3/4] Creation du raccourci bureau...
powershell -NoProfile -Command ^
  "$ws = New-Object -ComObject WScript.Shell; ^
   $s = $ws.CreateShortcut('%DESKTOP%\FileSearch.lnk'); ^
   $s.TargetPath = '%INSTALL_DIR%\filesearch.bat'; ^
   $s.WorkingDirectory = '%INSTALL_DIR%'; ^
   $s.Description = 'FileSearch - Moteur de recherche local'; ^
   $s.IconLocation = 'imageres.dll,109'; ^
   $s.Save()"
echo [OK] Raccourci cree sur le bureau.

:: ── Ajouter au démarrage Windows (optionnel) ─────────
echo.
set /p STARTUP_CHOICE=Lancer FileSearch automatiquement au demarrage Windows ? (o/n) : 
if /i "%STARTUP_CHOICE%"=="o" (
    copy /y "%INSTALL_DIR%\filesearch.bat" "%STARTUP%\FileSearch.bat" >nul
    echo [OK] Demarrage automatique active.
) else (
    echo [OK] Demarrage automatique ignore.
)

:: ── Fin ──────────────────────────────────────────────
echo.
echo [4/4] Installation terminee !
echo.
echo  Dossier : %INSTALL_DIR%
echo  Raccourci bureau : FileSearch.lnk
echo.
echo  Double-cliquez sur "FileSearch" sur votre bureau pour demarrer.
echo  Ou lancez : %INSTALL_DIR%\filesearch.bat
echo.
echo  Pour desinstaller : install.bat --uninstall
echo.
pause
exit /b 0

:: ── Désinstallation ──────────────────────────────────
:uninstall
echo Desinstallation de FileSearch...

:: Tuer le serveur s'il tourne
wsl -d %WSL_DISTRO% -e bash -lc "pkill -f filesearch-server 2>/dev/null; echo ok" >nul 2>&1

:: Supprimer raccourci bureau
if exist "%DESKTOP%\FileSearch.lnk" (
    del /q "%DESKTOP%\FileSearch.lnk"
    echo [OK] Raccourci bureau supprime.
)

:: Supprimer du démarrage
if exist "%STARTUP%\FileSearch.bat" (
    del /q "%STARTUP%\FileSearch.bat"
    echo [OK] Demarrage automatique supprime.
)

:: Supprimer le dossier d'installation
if exist "%INSTALL_DIR%" (
    set /p CONFIRM=Supprimer %INSTALL_DIR% et toutes les donnees indexees ? (o/n) : 
    if /i "!CONFIRM!"=="o" (
        rmdir /s /q "%INSTALL_DIR%"
        echo [OK] Dossier supprime.
    ) else (
        echo [OK] Donnees conservees dans %INSTALL_DIR%
    )
)

echo Desinstallation terminee.
pause
exit /b 0
