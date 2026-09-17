<#
.SYNOPSIS
    Automated Ducke Release Builder & GitHub Publisher.
.DESCRIPTION
    1. Validates and updates the application version across Go and frontend sources.
    2. Builds the frontend production bundle (Vite/Svelte).
    3. Compiles the native Windows x64 binary (Wails).
    4. Compiles the native Linux AMD64 binary and Steam Deck Flatpak bundle in WSL.
    5. Packages all release archives in build\bin.
    6. Commits changes, creates an annotated Git tag, and pushes to GitHub.
    7. Creates a GitHub Release and uploads all binary archives.
.EXAMPLE
    .\scripts\release.ps1 -Version 1.1.6
    .\release.bat 1.1.6 "Ducke v1.1.6 - Release"
#>

[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Version,

    [Parameter(Position = 1)]
    [string]$Title,

    [Parameter(Position = 2)]
    [string]$Notes,

    [switch]$SkipLinux,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

# Helper for colored logging
function Log-Step($msg) {
    Write-Host "`n[+] $msg" -ForegroundColor Cyan
}

function Log-Success($msg) {
    Write-Host "[OK] $msg" -ForegroundColor Green
}

function Log-Warn($msg) {
    Write-Host "[!] $msg" -ForegroundColor Yellow
}

function Log-Error($msg) {
    Write-Host "[ERR] $msg" -ForegroundColor Red
}

$rootDir = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
Set-Location $rootDir

Write-Host "========================================================" -ForegroundColor Magenta
Write-Host "     Ducke Automated Release and Deployment Engine      " -ForegroundColor Magenta
Write-Host "========================================================" -ForegroundColor Magenta

# 1. Resolve Target Version
if (-not $Version) {
    $currentVer = "1.1.5"
    try {
        $pkgJson = Get-Content (Join-Path $rootDir "frontend\package.json") -Raw | ConvertFrom-Json
        if ($pkgJson.version) { $currentVer = $pkgJson.version }
    } catch {}

    Write-Host "`nCurrent detected version: $currentVer" -ForegroundColor Gray
    $prompt = Read-Host "Enter target release version (e.g. 1.1.6)"
    if ([string]::IsNullOrWhiteSpace($prompt)) {
        Log-Error "Version cannot be empty. Aborting."
        exit 1
    }
    $Version = $prompt.Trim()
}

$cleanVersion = $Version -replace '^[vV]', ''
$tag = "v$cleanVersion"

if (-not ($cleanVersion -match '^\d+\.\d+\.\d+')) {
    Log-Error "Invalid semantic version: '$Version'. Expected format: X.Y.Z (e.g. 1.1.6)"
    exit 1
}

if (-not $Title) {
    $Title = "Ducke $tag"
}

Write-Host "Target Version: $cleanVersion ($tag)" -ForegroundColor Yellow
Write-Host "Release Title:  $Title" -ForegroundColor Yellow
Write-Host "Root Directory: $rootDir" -ForegroundColor Gray
if ($DryRun) {
    Log-Warn "DRY RUN MODE: No Git commits, tags, or GitHub releases will be created."
}

# 2. Check Prerequisites
Log-Step "Verifying build tools and toolchain..."

$tools = @("go", "wails", "npm", "git")
foreach ($tool in $tools) {
    $path = Get-Command $tool -ErrorAction SilentlyContinue
    if (-not $path) {
        Log-Error "Required tool '$tool' is not installed or not in PATH."
        exit 1
    }
}

Log-Success "Go, Wails, NPM, and Git verified."

if (-not $SkipLinux) {
    $wslCheck = wsl.exe -l -v 2>$null
    if ($LASTEXITCODE -ne 0) {
        Log-Warn "WSL is not available. Linux and Flatpak build will be skipped."
        $SkipLinux = $true
    } else {
        Log-Success "WSL Linux environment verified."
    }
}

# 3. Update Version in Codebase
Log-Step "Updating version to $cleanVersion across codebase..."
$utf8NoBom = New-Object System.Text.UTF8Encoding($false)

# A. app.go
$appGoPath = Join-Path $rootDir "app.go"
if (Test-Path $appGoPath) {
    $appGo = [System.IO.File]::ReadAllText($appGoPath, [System.Text.Encoding]::UTF8)
    $appGo = [System.Text.RegularExpressions.Regex]::Replace(
        $appGo,
        'log\.Printf\("\[System\] Ducke v[0-9\.]+ starting up"\)',
        "log.Printf(`"[System] Ducke v$cleanVersion starting up`")"
    )
    $appGo = [System.Text.RegularExpressions.Regex]::Replace(
        $appGo,
        '(?s)(func \(a \*App\) GetAppInfo\(\) AppInfo\s*\{[^}]*Version:\s*")[^"]*(")',
        ('${1}' + $cleanVersion + '${2}')
    )
    [System.IO.File]::WriteAllText($appGoPath, $appGo, $utf8NoBom)
    Log-Success "Updated app.go"
}

# B. frontend/package.json
$pkgPath = Join-Path $rootDir "frontend\package.json"
if (Test-Path $pkgPath) {
    $pkg = [System.IO.File]::ReadAllText($pkgPath, [System.Text.Encoding]::UTF8)
    $pkg = [System.Text.RegularExpressions.Regex]::Replace(
        $pkg,
        '("name":\s*"frontend",\s*"private":\s*true,\s*"version":\s*")[^"]*(")',
        ('${1}' + $cleanVersion + '${2}')
    )
    [System.IO.File]::WriteAllText($pkgPath, $pkg, $utf8NoBom)
    Log-Success "Updated frontend/package.json"
}

# C. frontend/package-lock.json
$pkgLockPath = Join-Path $rootDir "frontend\package-lock.json"
if (Test-Path $pkgLockPath) {
    $pkgLock = [System.IO.File]::ReadAllText($pkgLockPath, [System.Text.Encoding]::UTF8)
    $pkgLock = [System.Text.RegularExpressions.Regex]::Replace(
        $pkgLock,
        '("name":\s*"frontend",\s*"version":\s*")[^"]*(")',
        ('${1}' + $cleanVersion + '${2}')
    )
    $pkgLock = [System.Text.RegularExpressions.Regex]::Replace(
        $pkgLock,
        '("packages":\s*\{\s*"":\s*\{\s*"name":\s*"frontend",\s*"version":\s*")[^"]*(")',
        ('${1}' + $cleanVersion + '${2}')
    )
    [System.IO.File]::WriteAllText($pkgLockPath, $pkgLock, $utf8NoBom)
    Log-Success "Updated frontend/package-lock.json"
}

# D. frontend/src/lib/components/SettingsView.svelte
$settingsViewPath = Join-Path $rootDir "frontend\src\lib\components\SettingsView.svelte"
if (Test-Path $settingsViewPath) {
    $sv = [System.IO.File]::ReadAllText($settingsViewPath, [System.Text.Encoding]::UTF8)
    $sv = [System.Text.RegularExpressions.Regex]::Replace(
        $sv,
        "(let appInfo = \$state<\{ name: string; version: string \}>\(\{ name: 'Ducke', version: ')[^']*('\s*\}\);)",
        ('${1}' + $cleanVersion + '${2}')
    )
    [System.IO.File]::WriteAllText($settingsViewPath, $sv, $utf8NoBom)
    Log-Success "Updated SettingsView.svelte"
}

# E. frontend/src/lib/components/bigpicture/BigPictureSettings.svelte
$bpSettingsViewPath = Join-Path $rootDir "frontend\src\lib\components\bigpicture\BigPictureSettings.svelte"
if (Test-Path $bpSettingsViewPath) {
    $bps = [System.IO.File]::ReadAllText($bpSettingsViewPath, [System.Text.Encoding]::UTF8)
    $bps = [System.Text.RegularExpressions.Regex]::Replace(
        $bps,
        "(let appInfo = \$state<\{ name: string; version: string \}>\(\{ name: 'Ducke', version: ')[^']*('\s*\}\);)",
        ('${1}' + $cleanVersion + '${2}')
    )
    [System.IO.File]::WriteAllText($bpSettingsViewPath, $bps, $utf8NoBom)
    Log-Success "Updated BigPictureSettings.svelte"
}

# 4. Build Frontend Bundle
Log-Step "Building frontend production assets (Vite/Svelte)..."
Set-Location (Join-Path $rootDir "frontend")
npm run build
if ($LASTEXITCODE -ne 0) {
    Log-Error "Frontend compilation failed!"
    exit 1
}
Log-Success "Frontend build complete."
Set-Location $rootDir

# 5. Build Native Windows x64 Binary
Log-Step "Compiling native Windows binary via Wails..."
wails build -s -clean=false
if ($LASTEXITCODE -ne 0) {
    Log-Error "Windows binary compilation failed!"
    exit 1
}

$winExe = Join-Path $rootDir "build\bin\Ducke.exe"
if (-not (Test-Path $winExe)) {
    Log-Error "Expected Windows binary not found at $winExe"
    exit 1
}
$winSizeMB = [math]::Round((Get-Item $winExe).Length / 1MB, 2)
Log-Success "Windows binary compiled: Ducke.exe ($winSizeMB MB)"

# 6. Build Linux & Steam Deck Bundle (if not skipped)
if (-not $SkipLinux) {
    Log-Step "Compiling Linux AMD64 binary and Steam Deck Flatpak bundle in WSL..."
    cmd.exe /c "build_linux.bat"
    if ($LASTEXITCODE -ne 0) {
        Log-Error "Linux / Flatpak build failed in WSL!"
        exit 1
    }
    Log-Success "Linux and Flatpak bundle compiled successfully."
}

# 7. Package Release Archives
Log-Step "Packaging release archives in build\bin..."
$binDir = Join-Path $rootDir "build\bin"
Set-Location $binDir

$winZip = "Ducke-$tag-windows-x64.zip"
$linuxTar = "Ducke-$tag-linux-x64.tar.gz"
$deckZip = "Ducke-$tag-SteamDeck.zip"

Write-Host "Creating $winZip..." -ForegroundColor Gray
Compress-Archive -Path "Ducke.exe" -DestinationPath $winZip -Force
Log-Success "Packaged $winZip ($([math]::Round((Get-Item $winZip).Length / 1MB, 2)) MB)"

if (-not $SkipLinux -and (Test-Path (Join-Path $binDir "Ducke")) -and (Test-Path (Join-Path $binDir "Ducke.flatpak"))) {
    Write-Host "Creating $deckZip..." -ForegroundColor Gray
    Compress-Archive -Path "Ducke.flatpak", "install_steamdeck.sh" -DestinationPath $deckZip -Force
    Log-Success "Packaged $deckZip ($([math]::Round((Get-Item $deckZip).Length / 1MB, 2)) MB)"

    Write-Host "Creating $linuxTar..." -ForegroundColor Gray
    tar -czf $linuxTar Ducke
    Log-Success "Packaged $linuxTar ($([math]::Round((Get-Item $linuxTar).Length / 1MB, 2)) MB)"
}

Set-Location $rootDir

if ($DryRun) {
    Log-Success "DRY RUN COMPLETE: All binaries and archives built successfully. Skipping Git and GitHub release."
    exit 0
}

# 8. Git Commit & Tag
Log-Step "Committing version bump and tagging $tag in Git..."
git add app.go frontend/package.json frontend/package-lock.json frontend/src/lib/components/SettingsView.svelte frontend/src/lib/components/bigpicture/BigPictureSettings.svelte

$diff = git diff --staged
if ($diff) {
    git commit -m "chore(release): bump version to $cleanVersion"
    Log-Success "Committed version bump."
} else {
    Write-Host "No version changes needed to commit." -ForegroundColor Gray
}

# Check if tag already exists locally or remotely
$tagExistsLocally = git tag -l $tag
if ($tagExistsLocally) {
    Log-Warn "Tag $tag already exists locally. Updating tag..."
    git tag -d $tag | Out-Null
}

git tag -a $tag -m "Release $($tag) - Ducke v$cleanVersion"
Log-Success "Created annotated tag $tag."

Log-Step "Pushing commit and tags to origin/main..."
git push origin main
git push origin $tag
Log-Success "Git push completed."

# 9. GitHub Release Publication
Log-Step "Resolving GitHub API credentials..."
$token = $env:GITHUB_TOKEN

if (-not $token) {
    # Attempt extraction from Git Credential Manager
    $procInfo = New-Object System.Diagnostics.ProcessStartInfo
    $procInfo.FileName = "git.exe"
    $procInfo.Arguments = "credential fill"
    $procInfo.UseShellExecute = $false
    $procInfo.RedirectStandardInput = $true
    $procInfo.RedirectStandardOutput = $true
    $procInfo.CreateNoWindow = $true

    $proc = [System.Diagnostics.Process]::Start($procInfo)
    $proc.StandardInput.WriteLine("protocol=https")
    $proc.StandardInput.WriteLine("host=github.com")
    $proc.StandardInput.WriteLine("")
    $out = $proc.StandardOutput.ReadToEnd()
    $proc.WaitForExit()

    foreach ($line in ($out -split "`r?`n")) {
        if ($line -match '^password=(.+)$') {
            $token = $Matches[1]
        }
    }
}

if (-not $token) {
    Log-Error "No GitHub token found via GITHUB_TOKEN or Git Credential Manager. Could not publish release."
    Log-Warn "Archives are available locally in build\bin. You can upload them manually to https://github.com/makswh/ducke/releases/tag/$tag"
    exit 1
}

$repo = "makswh/ducke"
$headers = @{
    "Authorization" = "Bearer $token"
    "Accept"        = "application/vnd.github.v3+json"
    "User-Agent"    = "Ducke-Release-Builder"
}

# Generate changelog from git log if no notes provided
if (-not $Notes) {
    $lastTag = ""
    try {
        $tags = git tag --sort=-creatordate
        foreach ($t in $tags) {
            if ($t -ne $tag) {
                $lastTag = $t
                break
            }
        }
    } catch {}

    if ($lastTag) {
        $commits = git log "$lastTag..HEAD" --pretty=format:"- %s"
        $Notes = "## Что нового в Ducke $tag`n`n$commits"
    } else {
        $Notes = "## Что нового в Ducke $tag`n`n- Обновление и оптимизация приложения Ducke."
    }
}

Log-Step "Creating release on GitHub ($repo)..."
$releaseUrl = "https://api.github.com/repos/$repo/releases/tags/$tag"
$release = $null

try {
    $release = Invoke-RestMethod -Uri $releaseUrl -Headers $headers -Method Get -TimeoutSec 10
    Log-Warn "Existing release for $tag found (ID: $($release.id)). Reusing."
} catch {
    $createPayload = @{
        tag_name   = $tag
        name       = $Title
        body       = $Notes
        draft      = $false
        prerelease = $false
    } | ConvertTo-Json -Compress

    $createUrl = "https://api.github.com/repos/$repo/releases"
    $release = Invoke-RestMethod -Uri $createUrl -Headers $headers -Method Post -Body ([System.Text.Encoding]::UTF8.GetBytes($createPayload)) -ContentType "application/json; charset=utf-8" -TimeoutSec 15
    Log-Success "Release created on GitHub (ID: $($release.id))."
}

# 10. Upload Assets to GitHub
$uploadBase = $release.upload_url -replace '\{\?name,label\}', ''
$assetsToUpload = @($winZip)
if (-not $SkipLinux) {
    $assetsToUpload += $linuxTar
    $assetsToUpload += $deckZip
}

foreach ($assetName in $assetsToUpload) {
    $assetPath = Join-Path $binDir $assetName
    if (-not (Test-Path $assetPath)) {
        Log-Warn "Asset not found, skipping: $assetPath"
        continue
    }

    # If already exists in release, delete before re-uploading
    if ($release.assets) {
        $existingAsset = $release.assets | Where-Object { $_.name -eq $assetName }
        if ($existingAsset) {
            Write-Host "Deleting existing asset $($existingAsset.name)..." -ForegroundColor Gray
            $delUrl = "https://api.github.com/repos/$repo/releases/assets/$($existingAsset.id)"
            Invoke-RestMethod -Uri $delUrl -Headers $headers -Method Delete -TimeoutSec 15
        }
    }

    $assetSizeMB = [math]::Round((Get-Item $assetPath).Length / 1MB, 2)
    Write-Host "Uploading $assetName ($assetSizeMB MB)..." -ForegroundColor Cyan

    $contentType = "application/zip"
    if ($assetName -match '\.tar\.gz$') {
        $contentType = "application/gzip"
    }

    $uploadHeaders = @{
        "Authorization" = "Bearer $token"
        "Content-Type"  = $contentType
        "User-Agent"    = "Ducke-Release-Builder"
    }

    $fileBytes = [System.IO.File]::ReadAllBytes($assetPath)
    $uploadUrl = "$uploadBase`?name=$assetName"
    $res = Invoke-RestMethod -Uri $uploadUrl -Headers $uploadHeaders -Method Post -Body $fileBytes -TimeoutSec 180
    Log-Success "Uploaded $assetName (Asset ID: $($res.id))"
}

Write-Host "`n========================================================" -ForegroundColor Green
Write-Host "   RELEASE $tag PUBLISHED SUCCESSFULLY!                " -ForegroundColor Green
Write-Host "========================================================" -ForegroundColor Green
Write-Host "View release: https://github.com/$repo/releases/tag/$tag`n" -ForegroundColor White
