# PowerShell script to download aapt from Android SDK build-tools
# This script downloads aapt for Windows and places it in the bin/windows directory

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = Split-Path -Parent $ScriptDir
$BinDir = Join-Path $ProjectRoot "bin"
$WindowsBinDir = Join-Path $BinDir "windows"

# Create bin directory if it doesn't exist
New-Item -ItemType Directory -Force -Path $WindowsBinDir | Out-Null

# Android SDK build-tools version
$Version = "33.0.0"
$BaseUrl = "https://dl.google.com/android/repository"

Write-Host "Downloading aapt from Android SDK build-tools $Version..." -ForegroundColor Green

$ZipFile = Join-Path $env:TEMP "build-tools-windows.zip"
$Url = "$BaseUrl/build-tools_r$Version-windows.zip"

Write-Host "Downloading from: $Url" -ForegroundColor Yellow

try {
    # Download the zip file
    Invoke-WebRequest -Uri $Url -OutFile $ZipFile -UseBasicParsing
    
    Write-Host "Extracting aapt from zip..." -ForegroundColor Yellow
    
    # Extract aapt.exe from zip
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $Zip = [System.IO.Compression.ZipFile]::OpenRead($ZipFile)
    
    $AaptFound = $false
    foreach ($Entry in $Zip.Entries) {
        if ($Entry.Name -eq "aapt.exe" -or $Entry.FullName -like "*/aapt.exe") {
            $OutputPath = Join-Path $WindowsBinDir "aapt.exe"
            [System.IO.Compression.ZipFileExtensions]::ExtractToFile($Entry, $OutputPath, $true)
            Write-Host "✓ Successfully extracted aapt.exe" -ForegroundColor Green
            $AaptFound = $true
            break
        }
    }
    
    $Zip.Dispose()
    Remove-Item $ZipFile -Force
    
    if (-not $AaptFound) {
        Write-Host "✗ aapt.exe not found in zip file" -ForegroundColor Red
        exit 1
    }
    
    Write-Host ""
    Write-Host "Done! aapt.exe should now be in: $WindowsBinDir" -ForegroundColor Green
} catch {
    Write-Host "Error: $_" -ForegroundColor Red
    Write-Host ""
    Write-Host "If download failed, you can:" -ForegroundColor Yellow
    Write-Host "1. Install Android SDK and copy aapt.exe from build-tools directory"
    Write-Host "2. Download manually from: $BaseUrl"
    exit 1
}

