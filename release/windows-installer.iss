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
UninstallDisplayIcon={app}\tcforge-gui.exe
WizardStyle=modern

[Files]
Source: "{#SourceDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\tcforge"; Filename: "{app}\tcforge-gui.exe"
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

function RemovePathEntry(Path: string; Dir: string): string;
var
  Pieces: TArrayOfString;
  I: Integer;
  CleanPath: string;
begin
  CleanPath := '';
  StringChangeEx(Path, ';;', ';', True);
  Pieces := StringSplit(Path, [';'], stExcludeEmpty);
  for I := 0 to GetArrayLength(Pieces) - 1 do
  begin
    if UpperCase(Trim(Pieces[I])) <> UpperCase(Trim(Dir)) then
    begin
      if CleanPath <> '' then
        CleanPath := CleanPath + ';';
      CleanPath := CleanPath + Pieces[I];
    end;
  end;
  Result := CleanPath;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Path: string;
  UpdatedPath: string;
  Dir: string;
begin
  if CurUninstallStep <> usUninstall then
    exit;

  Dir := ExpandConstant('{app}');
  if not RegQueryStringValue(HKCU, 'Environment', 'Path', Path) then
    exit;

  UpdatedPath := RemovePathEntry(Path, Dir);
  if UpdatedPath <> Path then
    RegWriteExpandStringValue(HKCU, 'Environment', 'Path', UpdatedPath);
end;
