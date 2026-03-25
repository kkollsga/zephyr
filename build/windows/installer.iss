; Zephyr Windows Installer — Inno Setup Script

#ifndef APP_VERSION
  #define APP_VERSION "0.0.0"
#endif

[Setup]
AppId={{B8F3E2A1-7C4D-4E5F-9A1B-3D2E8F6C7A90}
AppName=Zephyr
AppVersion={#APP_VERSION}
AppVerName=Zephyr {#APP_VERSION}
AppPublisher=Kristian Kollsgard
AppPublisherURL=https://github.com/kkollsga/zephyr
DefaultDirName={autopf}\Zephyr
DefaultGroupName=Zephyr
DisableProgramGroupPage=yes
LicenseFile=..\..\LICENSE
OutputDir=..\..\
OutputBaseFilename=Zephyr-{#APP_VERSION}-setup
SetupIconFile=..\..\assets\icon.ico
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
UninstallDisplayIcon={app}\zephyr.exe

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked
Name: "addtopath"; Description: "Add to PATH"; GroupDescription: "Other:"

[Files]
Source: "..\..\zephyr.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\Zephyr"; Filename: "{app}\zephyr.exe"
Name: "{group}\{cm:UninstallProgram,Zephyr}"; Filename: "{uninstallexe}"
Name: "{autodesktop}\Zephyr"; Filename: "{app}\zephyr.exe"; Tasks: desktopicon

[Registry]
; "Open with Zephyr" context menu
Root: HKCR; Subkey: "*\shell\Open with Zephyr"; ValueType: string; ValueData: "Open with Zephyr"; Flags: uninsdeletekey
Root: HKCR; Subkey: "*\shell\Open with Zephyr\command"; ValueType: string; ValueData: """{app}\zephyr.exe"" ""%1"""; Flags: uninsdeletekey
; Add to PATH (optional)
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Tasks: addtopath; Check: NeedsAddPath(ExpandConstant('{app}'))

[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_CURRENT_USER,
    'Environment', 'Path', OrigPath) then
  begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;

[Run]
Filename: "{app}\zephyr.exe"; Description: "{cm:LaunchProgram,Zephyr}"; Flags: nowait postinstall skipifsilent
