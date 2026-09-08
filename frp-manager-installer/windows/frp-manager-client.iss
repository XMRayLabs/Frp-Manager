#define MyAppName "frp-manager Client"
#define MyAppVersion "1.0.0"
#define MyAppPublisher "Sakurame1"
#define MyAppURL "https://github.com/XMRayLabs/Frp-Manager"

[Setup]
AppId={{7F790E88-AE90-4C5E-A853-4D02D7F409A8}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\frp-manager
DisableDirPage=no
DefaultGroupName=frp-manager
OutputDir=output
OutputBaseFilename=frp-manager-client-1.0.0-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64
UninstallDisplayIcon={app}\frpp.exe

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Files]
Source: "files\frpp.exe"; DestDir: "{app}"; DestName: "frpp.exe"; Flags: ignoreversion
Source: "frp-manager-client-manager.ps1"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\frp-manager Client Manager"; Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File ""{app}\frp-manager-client-manager.ps1"""; WorkingDir: "{app}"
Name: "{group}\Uninstall frp-manager Client"; Filename: "{uninstallexe}"
Name: "{commondesktop}\frp-manager Client Manager"; Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File ""{app}\frp-manager-client-manager.ps1"""; WorkingDir: "{app}"; Tasks: desktopicon

[Tasks]
Name: "desktopicon"; Description: "Create a desktop shortcut"; GroupDescription: "Additional shortcuts:"; Flags: unchecked

[Run]
Filename: "powershell.exe"; Parameters: "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File ""{app}\frp-manager-client-manager.ps1"""; Description: "Open frp-manager Client Manager"; Flags: postinstall nowait skipifsilent

[UninstallRun]
Filename: "{app}\frpp.exe"; Parameters: "stop"; Flags: runhidden waituntilterminated skipifdoesntexist; RunOnceId: "StopFrppService"
Filename: "{app}\frpp.exe"; Parameters: "uninstall"; Flags: runhidden waituntilterminated skipifdoesntexist; RunOnceId: "UninstallFrppService"

[InstallDelete]
Type: filesandordirs; Name: "C:\frpp"

[Code]
procedure StopAndRemoveLegacyClient();
var
  ResultCode: Integer;
begin
  if FileExists('C:\frpp\frpp.exe') then
  begin
    Exec('C:\frpp\frpp.exe', 'stop', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
    Exec('C:\frpp\frpp.exe', 'uninstall', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
    Sleep(3000);
    DeleteFile('C:\frpp\frpp.exe');
    DelTree('C:\frpp', True, True, True);
  end;
end;

procedure StopExistingInstalledClient();
var
  ResultCode: Integer;
  ExePath: String;
begin
  ExePath := ExpandConstant('{autopf}\frp-manager\frpp.exe');
  if FileExists(ExePath) then
  begin
    Exec(ExePath, 'stop', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
    Exec(ExePath, 'uninstall', '', SW_HIDE, ewWaitUntilTerminated, ResultCode);
    Sleep(1000);
  end;
end;

function InitializeSetup(): Boolean;
begin
  StopAndRemoveLegacyClient();
  StopExistingInstalledClient();
  Result := True;
end;
