param (
  [switch] $Update = $false
)

function Get-Architecture {
  $architecture = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
  $arch = switch ($architecture) {
    "X64"   { "amd64" }
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
  # Retry with backoff to absorb transient failures, e.g. a release window
  # where the "latest" asset is already published but not fully uploaded yet.
  $maxAttempts = 5
  for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
    try {
      Invoke-WebRequest -OutFile $Output $Url
      return
    } catch {
      if ($attempt -eq $maxAttempts) {
        throw "Failed to download $Url after $maxAttempts attempts: $_"
      }
      $backoff = $attempt * 3
      Write-Host "Download failed (attempt $attempt/$maxAttempts), retrying in ${backoff}s..."
      Start-Sleep -Seconds $backoff
    }
  }
}

function Install-Plugin {
  param ([Parameter(Mandatory=$true)][string] $ArchiveDirectory, [Parameter(Mandatory=$true)][string] $ArchiveName, [Parameter(Mandatory=$true)][string] $Destination)
  Push-Location $ArchiveDirectory
  tar -xzf $ArchiveName -C .
  Pop-Location
  New-Item -ItemType Directory -Path $Destination -Force
  # Release archives wrap their content in a "diff" directory — the layout
  # the install/update hooks of every released version expect, so it must not
  # change (issue #1076). The 3.15.14 release instead wrapped the content in a
  # directory named after the archive itself (e.g. helm-diff-windows-amd64/
  # bin/diff.exe), as required by helm 4 when installing directly from a
  # tarball (issue #1071); support that layout as well so 3.15.14 tarballs
  # keep installing and updating.
  $wrapDir = [System.IO.Path]::GetFileNameWithoutExtension($ArchiveName)
  $binary = Join-Path $ArchiveDirectory $wrapDir "bin" "diff.exe"
  if (-not (Test-Path $binary -PathType Leaf)) {
    $binary = Join-Path $ArchiveDirectory "diff" "bin" "diff.exe"
  }
  Copy-Item -Path $binary -Destination $Destination -Force
}

$ErrorActionPreference = "Stop"

$arch = Get-Architecture

# The 3.15.14 release archives wrapped their content in a directory named
# after the archive itself (see Install-Plugin below), so the temporary copy
# of HELM_DIFF_BIN_TGZ must keep its original file name.
$archiveName = "helm-diff-windows-${arch}.tgz"

# If installing (not updating) and the binary is already staged in the
# plugin dir (e.g. installing from a release archive that bundles the
# correct platform binary), skip the redundant download. Update mode
# always re-downloads.
$pluginBin = Join-Path $env:HELM_PLUGIN_DIR "bin" "diff.exe"
if (-not $Update -and (Test-Path $pluginBin -PathType Leaf)) {
    Write-Host "Binary already present at $pluginBin, skipping download"
    exit 0
}

$tmpDir = New-TemporaryDirectory
trap {   Remove-Item -path $tmpDir -Recurse -Force }

# Check for offline installation via environment variable
if ($env:HELM_DIFF_BIN_TGZ) {
    Write-Host "HELM_DIFF_BIN_TGZ is set. Using local package at: $($env:HELM_DIFF_BIN_TGZ)"
    if (-not (Test-Path $env:HELM_DIFF_BIN_TGZ -PathType Leaf)) {
        throw "Offline installation failed: File not found at '$($env:HELM_DIFF_BIN_TGZ)'"
    }
    $archiveName = [System.IO.Path]::GetFileName($env:HELM_DIFF_BIN_TGZ)
    $output = Join-Path $tmpDir $archiveName
    Copy-Item -Path $env:HELM_DIFF_BIN_TGZ -Destination $output
}
else {
    # Proceed with online installation
    $output = Join-Path $tmpDir $archiveName
    $version = Get-Version -Update $Update
    $url = Get-Url -Version $version -Architecture $arch
    Download-Plugin -Url $url -Output $output
}

Install-Plugin -ArchiveDirectory $tmpDir -ArchiveName $archiveName -Destination (Join-Path $env:HELM_PLUGIN_DIR "bin")
