param (
  [switch] $Update = $false
)

function Get-Architecture {
  $architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture

  $arch = switch ($architecture) {
    "X64"  { "amd64" }
    "Arm64" { "arm64" }
    Default { "" }
  }

  if ($arch -eq "") {
    throw "Unsupported architecture: ${architecture}"
  }

  return $arch
}

function Get-Version {
  param ([Parameter(Mandatory=$true)][bool] $Update)

  if ($Update) {
    return "latest"
  }

  return git describe --tags --exact-match 2>$null || "latest"
}

function New-TemporaryDirectory {
  $tmp = [System.IO.Path]::GetTempPath()
  $name = (New-Guid).ToString("N")
  $dir = New-Item -ItemType Directory -Path (Join-Path $tmp $name)
  return $dir.FullName
}

function Get-Url {
  param ([Parameter(Mandatory=$true)][string] $Version, [Parameter(Mandatory=$true)][string] $Architecture)

  if ($Version -eq "latest") {
    return "https://github.com/databus23/helm-diff/releases/latest/download/helm-diff-windows-${Architecture}.tgz"
  }
  return "https://github.com/databus23/helm-diff/releases/download/${Version}/helm-diff-windows-${Architecture}.tgz"
}

function Download-Plugin {
  param ([Parameter(Mandatory=$true)][string] $Url, [Parameter(Mandatory=$true)][string] $Output)

  Invoke-WebRequest -OutFile $Output $Url
}

function Install-Plugin {
  param ([Parameter(Mandatory=$true)][string] $ArchiveDirectory, [Parameter(Mandatory=$true)][string] $ArchiveName, [Parameter(Mandatory=$true)][string] $Destination)

  tar -xzf (Join-Path $ArchiveDirectory $ArchiveName) -C $ArchiveDirectory

  New-Item -ItemType Directory -Path $Destination -Force
  Copy-Item -Path (Join-Path $ArchiveDirectory "diff" "bin" "diff.exe") -Destination $Destination -Force
}

$ErrorActionPreference = "Stop"

$archiveName = "helm-diff.tgz"
$arch = Get-Architecture
$version = Get-Version -Update $Update
$tmpDir = New-TemporaryDirectory

trap {  Remove-Item -path $tmpDir -Recurse -Force }

$url = Get-Url -Version $version -Architecture $arch
$output = Join-Path $tmpDir $archiveName

Download-Plugin -Url $url -Output $output
Install-Plugin -ArchiveDirectory $tmpDir -ArchiveName $archiveName -Destination (Join-Path $env:HELM_PLUGIN_DIR "bin")