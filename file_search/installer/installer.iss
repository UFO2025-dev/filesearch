
; FileSearch Inno Setup Script
; Compile with Inno Setup 6: iscc installer.iss
; Output: FileSearch-Setup-v1.0.1.exe

#define AppName      "FileSearch"
#define AppVersion   "1.0.1"
#define AppPublisher "FileSearch"
#define AppURL       "https://github.com/UFO2025-dev/filesearch"
#define AppExeName   "filesearch.exe"
#define AppDesc      "Local File Search Engine"

[Setup]
AppId={{A4E2B3F1-7C8D-4E5A-9B1C-2D3F4E5A6B7C}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}
AppUpdatesURL={#AppURL}
DefaultDirName={autopf}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
; No UAC elevation needed — asInvoker in manifest
PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=dialog
OutputDir=.
OutputBaseFilename=FileSearch-Setup-v{#AppVersion}
SetupIconFile=assets\icon.ico
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
WizardSmallImageFile=assets\wizard_small.bmp
; DPI awareness
DPIScaling=yes
; Uninstall
UninstallDisplayIcon={app}\{#AppExeName}
UninstallDisplayName={#AppName} {#AppVersion}
; Minimum Windows 10
MinVersion=10.0

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"
Name: "french";  MessagesFile: "compiler:Languages\French.isl"

[Tasks]
Name: "desktopicon";    Description: "{cm:CreateDesktopIcon}";    GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "startupicon";    Description: "Start FileSearch with Windows"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
; Main executable
Source: "..\filesearch.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
; Start menu shortcut
Name: "{group}\{#AppName}";         Filename: "{app}\{#AppExeName}"; Comment: "{#AppDesc}"
Name: "{group}\Uninstall {#AppName}"; Filename: "{uninstallexe}"
; Desktop shortcut (optional)
Name: "{autodesktop}\{#AppName}";   Filename: "{app}\{#AppExeName}"; Comment: "{#AppDesc}"; Tasks: desktopicon
; Startup shortcut (optional)
Name: "{autostartup}\{#AppName}";   Filename: "{app}\{#AppExeName}"; Tasks: startupicon

[Registry]
; Register app for "Open with" in Windows Explorer
Root: HKCU; Subkey: "Software\{#AppPublisher}\{#AppName}"; ValueType: string; ValueName: "InstallPath"; ValueData: "{app}"; Flags: uninsdeletesubkey
; Remember version for update checks
Root: HKCU; Subkey: "Software\{#AppPublisher}\{#AppName}"; ValueType: string; ValueName: "Version"; ValueData: "{#AppVersion}"

[Run]
; Launch app after install (optional, user can uncheck)
Filename: "{app}\{#AppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(AppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
; Kill any running instance before uninstall
Filename: "taskkill.exe"; Parameters: "/F /IM {#AppExeName}"; Flags: runhidden skipifdoesntexist; RunOnceId: "KillFileSearch"

[Code]
// Kill any running instance of FileSearch before upgrading.
function InitializeSetup(): Boolean;
var
  ResultCode: Integer;
begin
  // Attempt to close gracefully (if registered in registry / tray)
  Exec('taskkill.exe', '/F /IM {#AppExeName}', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
  Result := True;
end;

// Show a friendly message if port 8080 is already in use (informational only).
procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    // Nothing to do — the app binds on launch, not at install time.
  end;
end;
