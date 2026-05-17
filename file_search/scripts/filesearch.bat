@echo off
setlocal

:: ─── FileSearch Launcher ─────────────────────────────────────────
:: Lance le serveur FileSearch dans WSL et ouvre l'interface web.
:: Usage: filesearch.bat [--kill]
:: ─────────────────────────────────────────────────────────────────

set PORT=8080
set WSL_DISTRO=Ubuntu
set BIN_PATH=~/surveillance/gatewatch_mvp/file_search
set DATA_PATH=%BIN_PATH%/data

:: -- Mode kill uniquement --
if "%1"=="--kill" (
    echo Arret du serveur FileSearch...
    wsl -d %WSL_DISTRO% -e bash -lc "pkill -f filesearch-server 2>/dev/null; echo done"
    exit /b 0
)

:: -- Vérifier si déjà en cours --
wsl -d %WSL_DISTRO% -e bash -lc "pgrep -f filesearch-server > /dev/null 2>&1" >nul 2>&1
if %errorlevel%==0 (
    echo FileSearch deja en cours d execution.
    echo Ouverture du navigateur...
    start "" "http://localhost:%PORT%"
    exit /b 0
)

:: -- S'assurer que le dossier data existe --
wsl -d %WSL_DISTRO% -e bash -lc "mkdir -p %DATA_PATH%"

:: -- Démarrer le serveur en arrière-plan --
echo Demarrage de FileSearch...
wsl -d %WSL_DISTRO% -e bash -lc "cd %BIN_PATH% && nohup ./filesearch-server -config data/config.json >> data/server.log 2>&1 &"

:: -- Attendre que le serveur réponde --
set /a tries=0
:wait_loop
timeout /t 1 /nobreak >nul
set /a tries+=1
curl -s -o nul -w "%%{http_code}" "http://localhost:%PORT%/health" 2>nul | findstr /r "^200" >nul 2>&1
if %errorlevel%==0 goto ready
if %tries% lss 15 goto wait_loop
echo AVERTISSEMENT: Le serveur n a pas repondu dans les delais.

:ready
echo FileSearch pret sur http://localhost:%PORT%
start "" "http://localhost:%PORT%"
exit /b 0
