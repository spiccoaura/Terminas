Write-Host "==========================================" -ForegroundColor Cyan
Write-Host "        TERMINAS IDE INSTALLER            " -ForegroundColor Magenta
Write-Host "==========================================" -ForegroundColor Cyan
Write-Host ""

Write-Host "[1/3] Compiling Terminas IDE..." -ForegroundColor Yellow
go build -ldflags="-s -w" -o terminas.exe Main\Main.go

if ($LASTEXITCODE -ne 0) {
    Write-Host "[X] Build failed! Make sure you have Go installed." -ForegroundColor Red
    exit 1
}
Write-Host "[OK] Build successful." -ForegroundColor Green
Write-Host ""

$InstallDir = "$env:USERPROFILE\.terminas\bin"

Write-Host "[2/3] Setting up environment..." -ForegroundColor Yellow
if (-not (Test-Path -Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir | Out-Null
    Write-Host "[OK] Created directory $InstallDir" -ForegroundColor Green
}

Write-Host "[OK] Moving terminas.exe to $InstallDir" -ForegroundColor Green
Move-Item -Path .\terminas.exe -Destination "$InstallDir\terminas.exe" -Force

Write-Host ""
Write-Host "[3/3] Configuring System PATH..." -ForegroundColor Yellow
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
