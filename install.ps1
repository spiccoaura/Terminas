Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "        TERMINAS IDE INSTALLER            " -ForegroundColor Magenta
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

$InstallDir = "$env:USERPROFILE\.terminas\bin"
$ExePath = "$InstallDir\terminas.exe"

if (-not (Test-Path -Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
}

if (Test-Path ".\Main\Main.go") {
    Write-Host "[1/2] Compiling Terminas IDE from source..." -ForegroundColor Yellow
    go build -ldflags="-s -w" -o terminas.exe Main\Main.go
    if ($LASTEXITCODE -ne 0) {
        Write-Host "[X] Build failed! Make sure you have Go installed." -ForegroundColor Red
        exit 1
    }
    Move-Item -Path .\terminas.exe -Destination $ExePath -Force
} else {
    Write-Host "[1/2] Downloading latest Terminas release..." -ForegroundColor Yellow
    $DownloadUrl = "https://github.com/spiccoaura/Terminas/releases/latest/download/terminas.exe"
    try {
        Invoke-WebRequest -Uri $DownloadUrl -OutFile $ExePath -UseBasicParsing
    } catch {
        Write-Host "[X] Download failed! Did you upload terminas.exe to your latest GitHub release?" -ForegroundColor Red
        exit 1
    }
}

Write-Host "[OK] Terminas installed to $InstallDir" -ForegroundColor Green
Write-Host ""

Write-Host "[2/2] Configuring System PATH..." -ForegroundColor Yellow
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")

if ($UserPath -notlike "*$InstallDir*") {
    $NewPath = "$UserPath;$InstallDir"
    [Environment]::SetEnvironmentVariable("PATH", $NewPath, "User")
    $env:PATH = "$env:PATH;$InstallDir"
    Write-Host "[OK] Added $InstallDir to User PATH." -ForegroundColor Green
} else {
    Write-Host "[OK] PATH is already configured." -ForegroundColor Green
}

Write-Host ""
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "[DONE] INSTALLATION COMPLETE!" -ForegroundColor Magenta
Write-Host "Terminas IDE is now globally installed." -ForegroundColor White
Write-Host "You can type 'terminas' from any powershell window to launch it!" -ForegroundColor White
Write-Host "==========================================" -ForegroundColor Cyan
