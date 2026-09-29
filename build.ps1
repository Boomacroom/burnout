# Builds both executables from the same source:
#   burnout.exe      console build, for the terminal (double-clicking it opens the launcher too)
#   burnout-gui.exe  windowless build, for double-clicking or a shortcut
# Pass -Resources to regenerate the icon/manifest resource file after editing winres\.
param([switch]$Resources)
$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot

if ($Resources) {
    & C:\msys64\ucrt64\bin\windres.exe -O coff --include-dir winres -o rsrc_windows_amd64.syso winres\burnout.rc
}
go vet .
go test .
go build -o burnout.exe .
go build -ldflags "-H=windowsgui" -o burnout-gui.exe .
Write-Host "Built burnout.exe and burnout-gui.exe"
