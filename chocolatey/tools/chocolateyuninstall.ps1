$ErrorActionPreference = 'Stop'
$packageName = 's3ry'
$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"

# Remove the binary
$exePath = Join-Path $toolsDir "s3ry.exe"
if (Test-Path $exePath) {
    Remove-Item $exePath -Force
    Write-Host "s3ry has been uninstalled successfully!"
} else {
    Write-Host "s3ry executable not found, may have already been removed."
}