param(
  [string]$GoExe = $env:GO_EXE,
  [string]$RunnerVersion = $env:RUNNER_VERSION
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$artifactDir = Join-Path $repoRoot "artifacts\runner"
New-Item -ItemType Directory -Force -Path $artifactDir | Out-Null

if ([string]::IsNullOrWhiteSpace($GoExe)) {
  $goCommand = Get-Command go -ErrorAction SilentlyContinue
  if ($goCommand) {
    $GoExe = $goCommand.Source
  }
}

if ([string]::IsNullOrWhiteSpace($GoExe)) {
  $sdkGo = Join-Path $env:USERPROFILE "sdk\go1.26.1\bin\go.exe"
  if (Test-Path -LiteralPath $sdkGo) {
    $GoExe = $sdkGo
  }
}

if ([string]::IsNullOrWhiteSpace($GoExe) -or -not (Test-Path -LiteralPath $GoExe)) {
  throw "Go executable not found. Add go to PATH or set GO_EXE to the full go.exe path."
}
if ([string]::IsNullOrWhiteSpace($RunnerVersion)) {
  $RunnerVersion = "dev"
}

$targets = @(
  @{ GOOS = "windows"; GOARCH = "amd64"; Output = "runner-windows-amd64.exe" },
  @{ GOOS = "linux"; GOARCH = "amd64"; Output = "runner-linux-amd64" },
  @{ GOOS = "linux"; GOARCH = "arm64"; Output = "runner-linux-arm64" },
  @{ GOOS = "darwin"; GOARCH = "amd64"; Output = "runner-darwin-amd64" },
  @{ GOOS = "darwin"; GOARCH = "arm64"; Output = "runner-darwin-arm64" }
)

$oldGoos = $env:GOOS
$oldGoarch = $env:GOARCH
$oldCgoEnabled = $env:CGO_ENABLED

try {
  foreach ($target in $targets) {
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $env:CGO_ENABLED = "0"
    $out = Join-Path $artifactDir $target.Output
    Write-Host "Building $($target.GOOS)/$($target.GOARCH) -> $out"
    & $GoExe build -trimpath -ldflags="-s -w -X main.RunnerVersion=$RunnerVersion" -o $out .\runner\cmd\runner
  }
}
finally {
  if ($null -eq $oldGoos) {
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
  } else {
    $env:GOOS = $oldGoos
  }

  if ($null -eq $oldGoarch) {
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
  } else {
    $env:GOARCH = $oldGoarch
  }

  if ($null -eq $oldCgoEnabled) {
    Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
  } else {
    $env:CGO_ENABLED = $oldCgoEnabled
  }
}

Write-Host "Runner artifacts written to $artifactDir"
