$ErrorActionPreference = 'Stop'
$packageName = 's3ry'
$toolsDir = "$(Split-Path -parent $MyInvocation.MyCommand.Definition)"
$packageVersion = $env:ChocolateyPackageVersion
$url64 = "https://github.com/seike460/s3ry/releases/download/v$packageVersion/s3ry_windows_amd64.zip"

$packageArgs = @{
  packageName    = $packageName
  unzipLocation  = $toolsDir
  url64bit       = $url64
  
  checksum64     = ''  # Will be filled by release process
  checksumType64 = 'sha256'
  
  validExitCodes = @(0)
}

# Download and extract the package
Install-ChocolateyZipPackage @packageArgs

# Add the binary to PATH by creating a shim
$exePath = Join-Path $toolsDir "s3ry.exe"
if (Test-Path $exePath) {
    Write-Host "s3ry has been installed successfully!"
    Write-Host ""
    Write-Host "Quick Start:"
    Write-Host "  1. Configure AWS credentials: aws configure"
    Write-Host "  2. Run s3ry: s3ry"
    Write-Host "  3. Use TUI mode: s3ry --tui"
    Write-Host ""
    Write-Host "Documentation: https://github.com/seike460/s3ry"
} else {
    throw "Installation failed: s3ry.exe not found in package"
}