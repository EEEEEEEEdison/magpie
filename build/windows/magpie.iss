; Build with Inno Setup 6:
;   ISCC.exe /DMyAppVersion=0.11.2 build\windows\magpie.iss
#ifndef MyAppVersion
  #define MyAppVersion "0.0.0"
#endif

#define MyAppName "magpie"
#define MyAppPublisher "magpie"
#define MyAppURL "https://github.com/EEEEEEEEdison/magpie"
#define MyAppExeName "magpie-windows-amd64.exe"

[Setup]
AppId={{C93A9EDD-97E6-49FE-95FE-63BD3160BBBE}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppVerName={#MyAppName} {#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}/releases
DefaultDirName={localappdata}\Programs\magpie
DefaultGroupName=magpie
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
OutputDir=..\..\dist
OutputBaseFilename=magpie-setup-windows-amd64
SetupIconFile=..\..\internal\gui\icon-windows.ico
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
CloseApplications=yes
RestartApplications=no
UninstallDisplayIcon={app}\{#MyAppExeName}
UninstallDisplayName={#MyAppName}
VersionInfoVersion={#MyAppVersion}.0
VersionInfoDescription={#MyAppName} Setup
VersionInfoProductName={#MyAppName}
VersionInfoCompany={#MyAppName}
LicenseFile=..\..\LICENSE

[Languages]
Name: "chinesesimplified"; MessagesFile: "ChineseSimplified.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Files]
Source: "..\..\dist\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{userprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Comment: "magpie"
Name: "{userdesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; WorkingDir: "{app}"; Comment: "magpie"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "启动 magpie"; Flags: nowait postinstall skipifsilent

[Code]
function FindWindow(lpClassName: String; lpWindowName: String): THandle;
external 'FindWindowW@user32.dll stdcall';

function InitializeSetup(): Boolean;
var
  H: THandle;
begin
  H := FindWindow('', 'magpie');
  if H <> 0 then
  begin
    MsgBox('magpie 正在运行，请先退出 magpie 后再安装。', mbError, MB_OK);
    Result := False;
  end
  else
    Result := True;
end;