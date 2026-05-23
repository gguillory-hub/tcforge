param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("windows-x64", "macos-arm64")]
    [string] $Platform,

    [string] $Version = "dev",
    [string] $Commit = "",
    [string] $Date = "",
    [string] $DependencyDir = "",
    [string] $OutputRoot = "dist",
    [switch] $BuildInstaller
)

$ErrorActionPreference = "Stop"

function Resolve-RepoRoot {
    return (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
}

function Find-CommandPath {
    param([string] $Name)
    $cmd = Get-Command $Name -ErrorAction SilentlyContinue
    if ($null -eq $cmd) {
        return $null
    }
    return $cmd.Source
}

function Copy-Tool {
    param(
        [string] $Name,
        [string] $FromDir,
        [string] $ToDir
    )

    $names = @($Name)
	if (-not $Name.EndsWith(".exe")) {
		$names += "$Name.exe"
	}

	if ($FromDir -ne "") {
		foreach ($candidateName in $names) {
			$candidate = Join-Path $FromDir $candidateName
			if (Test-Path -LiteralPath $candidate) {
				Copy-Item -LiteralPath $candidate -Destination (Join-Path $ToDir $candidateName) -Force
				return
			}
		}
	}

	foreach ($candidateName in $names) {
		$pathCandidate = Find-CommandPath $candidateName
		if ($null -ne $pathCandidate) {
			Copy-Item -LiteralPath $pathCandidate -Destination (Join-Path $ToDir $candidateName) -Force
			return
        }
    }

    throw "Could not find required tool '$Name'. Pass -DependencyDir containing ffmpeg, ffprobe, and ltcdump."
}

function New-Sha256Manifest {
	param(
		[string] $Root,
		[string] $OutputPath
	)

    $files = Get-ChildItem -LiteralPath $Root -Recurse -File |
        Where-Object { $_.FullName -ne (Resolve-Path -LiteralPath $OutputPath -ErrorAction SilentlyContinue) } |
        Sort-Object FullName

	$lines = foreach ($file in $files) {
		$rootUri = New-Object System.Uri(($Root.TrimEnd("\", "/") + [System.IO.Path]::DirectorySeparatorChar))
		$fileUri = New-Object System.Uri($file.FullName)
		$relative = [System.Uri]::UnescapeDataString($rootUri.MakeRelativeUri($fileUri).ToString())
		$hash = (Get-FileHash -Algorithm SHA256 -LiteralPath $file.FullName).Hash.ToLowerInvariant()
		"$hash  $relative"
	}
    Set-Content -LiteralPath $OutputPath -Value $lines -Encoding utf8
}

$repo = Resolve-RepoRoot
$dist = Join-Path $repo $OutputRoot
$stageName = "tcforge-$Platform"
$stage = Join-Path $dist $stageName
$tools = Join-Path $stage "tools"
$binaryName = "tcforge"
$goos = "darwin"
$goarch = "arm64"

if ($Platform -eq "windows-x64") {
	$binaryName = "tcforge.exe"
	$goos = "windows"
	$goarch = "amd64"
}

if (Test-Path -LiteralPath $stage) {
	Remove-Item -LiteralPath $stage -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $tools | Out-Null

if ($DependencyDir -ne "") {
	if (-not (Test-Path -LiteralPath $DependencyDir)) {
		throw "Dependency directory does not exist: $DependencyDir"
	}
	Get-ChildItem -LiteralPath $DependencyDir -Recurse -File | ForEach-Object {
		$depRootUri = New-Object System.Uri(((Resolve-Path -LiteralPath $DependencyDir).Path.TrimEnd("\", "/") + [System.IO.Path]::DirectorySeparatorChar))
		$fileUri = New-Object System.Uri($_.FullName)
		$relative = [System.Uri]::UnescapeDataString($depRootUri.MakeRelativeUri($fileUri).ToString()).Replace("/", [System.IO.Path]::DirectorySeparatorChar)
		$destination = Join-Path $tools $relative
		New-Item -ItemType Directory -Force -Path (Split-Path -Parent $destination) | Out-Null
		Copy-Item -LiteralPath $_.FullName -Destination $destination -Force
	}
}

$env:GOOS = $goos
$env:GOARCH = $goarch
$env:CGO_ENABLED = "0"

$ldflags = @("-X", "main.version=$Version")
if ($Commit -ne "") {
    $ldflags += @("-X", "main.commit=$Commit")
}
if ($Date -ne "") {
    $ldflags += @("-X", "main.date=$Date")
}

Push-Location $repo
try {
    go build -trimpath -ldflags ($ldflags -join " ") -o (Join-Path $stage $binaryName) .
}
finally {
    Pop-Location
}

Copy-Tool -Name "ffmpeg" -FromDir $DependencyDir -ToDir $tools
Copy-Tool -Name "ffprobe" -FromDir $DependencyDir -ToDir $tools
Copy-Tool -Name "ltcdump" -FromDir $DependencyDir -ToDir $tools

Copy-Item -LiteralPath (Join-Path $repo "LICENSE") -Destination (Join-Path $stage "LICENSE.txt") -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot "THIRD_PARTY_NOTICES.md") -Destination (Join-Path $stage "THIRD_PARTY_NOTICES.md") -Force
Copy-Item -LiteralPath (Join-Path $PSScriptRoot "dependencies.json") -Destination (Join-Path $stage "DEPENDENCIES.json") -Force

Set-Content -LiteralPath (Join-Path $stage "README-FIRST.txt") -Encoding utf8 -Value @"
tcforge $Version

This portable package includes tcforge and bundled external tools in tools/.

Windows: run tcforge.exe from PowerShell or install with the unsigned installer.
macOS Apple Silicon: run ./tcforge from Terminal. Because this build is unsigned, macOS may require you to approve it in Privacy & Security or remove quarantine attributes after download.

No Go installation is required.
"@

New-Sha256Manifest -Root $stage -OutputPath (Join-Path $stage "SHA256SUMS.txt")

if ($Platform -eq "windows-x64") {
    $archive = Join-Path $dist "$stageName.zip"
	if (Test-Path -LiteralPath $archive) {
		Remove-Item -LiteralPath $archive -Force
	}
	Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $archive -Force

    if ($BuildInstaller) {
        $iscc = Find-CommandPath "ISCC.exe"
        if ($null -eq $iscc) {
            throw "Inno Setup compiler ISCC.exe was not found. Install Inno Setup before passing -BuildInstaller."
        }
        & $iscc "/DAppVersion=$Version" "/DSourceDir=$stage" "/DOutputDir=$dist" (Join-Path $PSScriptRoot "windows-installer.iss")
    }
}
else {
    $archive = Join-Path $dist "$stageName.tar.gz"
    if (Test-Path -LiteralPath $archive) {
        Remove-Item -LiteralPath $archive -Force
    }
    Push-Location $dist
    try {
        tar -czf "$stageName.tar.gz" $stageName
    }
    finally {
        Pop-Location
    }
}

Write-Host "Packaged $Platform in $stage"
