#define AppName "tcforge"
#ifndef AppVersion
#define AppVersion "dev"
#endif
#ifndef SourceDir
#define SourceDir "..\dist\tcforge-windows-x64"
#endif
#ifndef OutputDir
#define OutputDir "..\dist"
#endif

[Setup]
AppId={{D3B52AC1-6E6D-4D8F-B9B2-3B79E4A39E4B}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher=tcforge
DefaultDirName={autopf}\tcforge
DefaultGroupName=tcforge
DisableProgramGroupPage=yes
OutputDir={#OutputDir}
OutputBaseFilename=tcforge-windows-x64-setup
Compression=lzma2
SolidCompression=yes
ArchitecturesAllowed=x64
ArchitecturesInstallIn64BitMode=x64
PrivilegesRequired=lowest
UninstallDisplayIcon={app}\tcforge.exe
WizardStyle=modern

[Files]
Source: "{#SourceDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\tcforge PowerShell"; Filename: "powershell.exe"; Parameters: "-NoExit -Command ""Set-Location '{app}'; .\tcforge.exe --version"""

[Registry]
Root: HKCU; Subkey: "Environment"; ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; Check: NeedsAddPath(ExpandConstant('{app}'))

[Code]
function NeedsAddPath(Dir: string): Boolean;
var
  Path: string;
begin
  if not RegQueryStringValue(HKCU, 'Environment', 'Path', Path) then
  begin
    Result := True;
    exit;
  end;
  Result := Pos(';' + UpperCase(Dir) + ';', ';' + UpperCase(Path) + ';') = 0;
end;
