package control

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (a *API) handleRunnerInstallPowerShell(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	controlURL := publicControlURL(r)
	runnerToken := a.runnerToken(r.Context())
	codexBypassApprovalsAndSandbox := codexBypassApprovalsAndSandboxValue()
	runnerID := strings.TrimSpace(r.URL.Query().Get("runner_id"))
	codexPath := strings.TrimSpace(r.URL.Query().Get("codex_path"))
	if codexPath == "" {
		codexPath = "codex"
	}
	runAs := strings.TrimSpace(r.URL.Query().Get("run_as"))
	if runAs == "" {
		runAs = "user"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, `$ErrorActionPreference = "Stop"
$ControlUrl = %q
$RunnerId = %q
if ([string]::IsNullOrWhiteSpace($RunnerId)) {
  $RunnerName = $env:COMPUTERNAME
  if ([string]::IsNullOrWhiteSpace($RunnerName)) { $RunnerName = [System.Net.Dns]::GetHostName() }
  $MachineSuffix = ""
  try {
    $MachineGuid = (Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Cryptography" -Name MachineGuid -ErrorAction Stop).MachineGuid
    if (-not [string]::IsNullOrWhiteSpace($MachineGuid)) {
      $MachineSuffix = $MachineGuid.Replace("-", "")
      if ($MachineSuffix.Length -gt 12) { $MachineSuffix = $MachineSuffix.Substring(0, 12) }
    }
  } catch {}
  $RunnerId = $RunnerName
  if (-not [string]::IsNullOrWhiteSpace($MachineSuffix)) { $RunnerId = "$RunnerName-$MachineSuffix" }
}
$CodexPath = %q
$RunAs = %q
$CodexBypassApprovalsAndSandbox = %q
if ([string]::IsNullOrWhiteSpace($RunAs)) { $RunAs = "user" }
$RunAs = $RunAs.ToLowerInvariant()
if ($RunAs -ne "user" -and $RunAs -ne "system") { throw "run_as must be user or system." }

function Test-IsAdmin {
  $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
  $principal = New-Object Security.Principal.WindowsPrincipal($identity)
  return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Invoke-Checked {
  param(
    [Parameter(Mandatory=$true)][string]$FilePath,
    [Parameter(Mandatory=$true)][string[]]$Arguments
  )
  & $FilePath @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "$FilePath failed with exit code $LASTEXITCODE."
  }
}

function Stop-ExistingRunner {
  param([Parameter(Mandatory=$true)][string]$RunnerExePath)
  $existing = @(Get-Process -Name "codex-task-workbench-runner" -ErrorAction SilentlyContinue)
  $failures = @()
  foreach ($process in $existing) {
    $processPath = "unknown path"
    try {
      if ($process.Path) { $processPath = $process.Path }
    } catch {}
    Write-Host "Stopping existing runner process $($process.Id) ($processPath)"
    try {
      Stop-Process -Id $process.Id -Force -ErrorAction Stop
      Wait-Process -Id $process.Id -Timeout 10 -ErrorAction SilentlyContinue
    } catch {
      $failures += "PID $($process.Id): $($_.Exception.Message)"
    }
  }
  if ($failures.Count -gt 0) {
    throw "Unable to stop existing runner process(es). Close them or rerun from an elevated PowerShell. " + ($failures -join "; ")
  }
}

if ($RunAs -eq "system") {
  if (-not (Test-IsAdmin)) {
    throw "Administrator PowerShell is required when run_as=system. Re-run the install command from an elevated PowerShell, or omit run_as=system to install for the current user."
  }
  $InstallRoot = $env:ProgramData
  $TaskName = "CodexTaskWorkbenchRunner"
} else {
  if ([string]::IsNullOrWhiteSpace($env:LOCALAPPDATA)) {
    throw "LOCALAPPDATA is not set. Use run_as=system from an elevated PowerShell, or set LOCALAPPDATA for this user."
  }
  $InstallRoot = $env:LOCALAPPDATA
}

$InstallDir = Join-Path $InstallRoot "CodexTaskWorkbench\runner"
$RunnerExe = Join-Path $InstallDir "codex-task-workbench-runner.exe"
$Wrapper = Join-Path $InstallDir "run-runner.cmd"
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Remove-Item -LiteralPath (Join-Path $InstallDir "runner.disabled") -Force -ErrorAction SilentlyContinue
if (-not [Environment]::Is64BitOperatingSystem) { throw "Only windows-amd64 runner artifacts are supported by this bootstrap script." }
$RunnerToken = %q
$RunnerHeaders = @{"Cache-Control"="no-cache"}
if (-not [string]::IsNullOrWhiteSpace($RunnerToken)) {
  $RunnerHeaders["Authorization"] = "Bearer $RunnerToken"
}
$ArtifactUrl = "$ControlUrl/api/v1/runner/artifacts/runner-windows-amd64.exe"
Write-Host "Downloading runner from $ArtifactUrl"
Stop-ExistingRunner $RunnerExe
$TempRunner = Join-Path $InstallDir ("codex-task-workbench-runner." + [guid]::NewGuid().ToString("N") + ".download")
Invoke-WebRequest -UseBasicParsing -Uri ($ArtifactUrl + "?t=" + [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()) -Headers $RunnerHeaders -OutFile $TempRunner
try { Unblock-File -Path $TempRunner -ErrorAction SilentlyContinue } catch {}
Move-Item -Force -Path $TempRunner -Destination $RunnerExe
@"
@echo off
set "CONTROL_URL=$ControlUrl"
set "RUNNER_ID=$RunnerId"
set "CODEX_PATH=$CodexPath"
set "RUNNER_TOKEN=$RunnerToken"
set "CODEX_BYPASS_APPROVALS_AND_SANDBOX=$CodexBypassApprovalsAndSandbox"
set "RUNNER_RUN_AS=$RunAs"
cd /d "$InstallDir"
"$RunnerExe"
"@ | Set-Content -Encoding ASCII -Path $Wrapper
if ($RunAs -eq "system") {
  $TaskCommand = '"' + $Wrapper + '"'
  Invoke-Checked "schtasks.exe" @("/Create", "/TN", $TaskName, "/TR", $TaskCommand, "/SC", "ONSTART", "/RU", "SYSTEM", "/RL", "HIGHEST", "/F")
  Invoke-Checked "schtasks.exe" @("/Run", "/TN", $TaskName)
  Write-Host "Runner installed and started as scheduled task $TaskName with RUNNER_ID=$RunnerId run_as=$RunAs"
} else {
  $StartupDir = [Environment]::GetFolderPath("Startup")
  if ([string]::IsNullOrWhiteSpace($StartupDir)) {
    throw "Windows Startup folder was not found for the current user. Use run_as=system from an elevated PowerShell if you need a machine-level startup task."
  }
  New-Item -ItemType Directory -Force -Path $StartupDir | Out-Null
  $StartupWrapper = Join-Path $StartupDir "CodexTaskWorkbenchRunner.cmd"
  @"
@echo off
set "CONTROL_URL=$ControlUrl"
set "RUNNER_ID=$RunnerId"
set "CODEX_PATH=$CodexPath"
set "RUNNER_TOKEN=$RunnerToken"
set "CODEX_BYPASS_APPROVALS_AND_SANDBOX=$CodexBypassApprovalsAndSandbox"
set "RUNNER_RUN_AS=$RunAs"
cd /d "$InstallDir"
start "Meridian Runner" /min "$RunnerExe"
"@ | Set-Content -Encoding ASCII -Path $StartupWrapper
  $env:CONTROL_URL = $ControlUrl
  $env:RUNNER_ID = $RunnerId
  $env:CODEX_PATH = $CodexPath
  $env:RUNNER_TOKEN = $RunnerToken
  $env:CODEX_BYPASS_APPROVALS_AND_SANDBOX = $CodexBypassApprovalsAndSandbox
  $env:RUNNER_RUN_AS = $RunAs
  Start-Process -FilePath $RunnerExe -WorkingDirectory $InstallDir -WindowStyle Hidden
  Write-Host "Runner installed and started for current user with RUNNER_ID=$RunnerId startup=$StartupWrapper"
}
`, controlURL, runnerID, codexPath, runAs, codexBypassApprovalsAndSandbox, runnerToken)
}

func (a *API) handleRunnerInstallShell(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	controlURL := publicControlURL(r)
	runnerToken := shellQuote(a.runnerToken(r.Context()))
	codexBypassApprovalsAndSandbox := shellQuote(codexBypassApprovalsAndSandboxValue())
	runnerID := shellQuote(strings.TrimSpace(r.URL.Query().Get("runner_id")))
	codexPath := shellQuote(strings.TrimSpace(r.URL.Query().Get("codex_path")))
	if codexPath == "''" {
		codexPath = "'codex'"
	}
	runAs := shellQuote(strings.TrimSpace(r.URL.Query().Get("run_as")))
	if runAs == "''" {
		runAs = "'user'"
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, `#!/usr/bin/env sh
set -eu
CONTROL_URL=%s
RUNNER_ID=%s
CODEX_PATH=%s
RUNNER_TOKEN=%s
CODEX_BYPASS_APPROVALS_AND_SANDBOX=%s
RUN_AS=%s
if [ -z "$RUN_AS" ]; then
  RUN_AS="user"
fi
RUN_AS="$(printf '%%s' "$RUN_AS" | tr '[:upper:]' '[:lower:]')"
case "$RUN_AS" in
  user|system) ;;
  *) echo "run_as must be user or system." >&2; exit 1 ;;
esac
DEFAULT_RUNNER_PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
if [ -n "${PATH:-}" ]; then
  RUNNER_PATH="$PATH:$DEFAULT_RUNNER_PATH"
else
  RUNNER_PATH="$DEFAULT_RUNNER_PATH"
fi
PATH="$RUNNER_PATH"
export PATH
case "$CODEX_PATH" in
  */*) ;;
  *)
    RESOLVED_CODEX="$(command -v "$CODEX_PATH" 2>/dev/null || true)"
    if [ -n "$RESOLVED_CODEX" ]; then
      CODEX_PATH="$RESOLVED_CODEX"
    else
      echo "Warning: Codex executable '$CODEX_PATH' was not found during install. Set codex_path=/absolute/path in the install URL if task runs fail." >&2
    fi
    ;;
esac
OS="$(uname -s)"
if [ -z "$RUNNER_ID" ]; then
  RUNNER_HOSTNAME="$(hostname 2>/dev/null || printf 'runner')"
  MACHINE_SUFFIX=""
  case "$OS" in
    Linux)
      if [ -r /etc/machine-id ]; then
        MACHINE_ID="$(cat /etc/machine-id | tr -d '[:space:]' | tr -d '-')"
        MACHINE_SUFFIX="$(printf '%%s' "$MACHINE_ID" | cut -c1-12)"
      fi
      ;;
    Darwin)
      if command -v ioreg >/dev/null 2>&1; then
        MACHINE_ID="$(ioreg -rd1 -c IOPlatformExpertDevice 2>/dev/null | awk -F '"' '/IOPlatformUUID/{print $4}' | tr -d '-' || true)"
        MACHINE_SUFFIX="$(printf '%%s' "$MACHINE_ID" | cut -c1-12)"
      fi
      ;;
  esac
  RUNNER_ID="$RUNNER_HOSTNAME"
  if [ -n "$MACHINE_SUFFIX" ]; then
    RUNNER_ID="$RUNNER_HOSTNAME-$MACHINE_SUFFIX"
  fi
fi
ARCH="$(uname -m)"
case "$OS" in
  Linux)
    case "$ARCH" in
      x86_64|amd64) ARTIFACT="runner-linux-amd64" ;;
      aarch64|arm64) ARTIFACT="runner-linux-arm64" ;;
      *) echo "unsupported Linux architecture: $ARCH" >&2; exit 1 ;;
    esac
    INSTALL_DIR="/opt/codex-task-workbench/runner"
    RUNNER_BIN="$INSTALL_DIR/codex-task-workbench-runner"
    ENV_FILE="/etc/codex-task-workbench-runner.env"
    SERVICE_FILE="/etc/systemd/system/codex-task-workbench-runner.service"
    WRAPPER="$INSTALL_DIR/run-runner.sh"
    PID_FILE="$INSTALL_DIR/runner.pid"
    RUNNER_LOG="$INSTALL_DIR/runner.log"
    RUNNER_ERR_LOG="$INSTALL_DIR/runner.err.log"
    PLATFORM="linux"
    ;;
  Darwin)
    case "$ARCH" in
      x86_64|amd64) ARTIFACT="runner-darwin-amd64" ;;
      aarch64|arm64) ARTIFACT="runner-darwin-arm64" ;;
      *) echo "unsupported macOS architecture: $ARCH" >&2; exit 1 ;;
    esac
    INSTALL_DIR="/usr/local/lib/codex-task-workbench/runner"
    RUNNER_BIN="$INSTALL_DIR/codex-task-workbench-runner"
    WRAPPER="/usr/local/bin/codex-task-workbench-runner"
    PLIST="/Library/LaunchDaemons/com.codex-task-workbench.runner.plist"
    LAUNCHD_LABEL="com.codex-task-workbench.runner"
    PLATFORM="darwin"
    ;;
  *)
    echo "unsupported operating system: $OS" >&2
    exit 1
    ;;
esac
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
else
  SUDO=""
fi
RUNNER_USER=""
RUNNER_HOME=""
RUNNER_ENV_USER_LINES=""
RUNNER_WRAPPER_USER_EXPORTS=""
RUNNER_SERVICE_USER_LINES=""
if [ "$PLATFORM" = "linux" ] && [ "$RUN_AS" = "user" ]; then
  if [ "$(id -u)" -eq 0 ] && [ -n "${SUDO_USER:-}" ] && [ "${SUDO_USER:-}" != "root" ]; then
    RUNNER_USER="$SUDO_USER"
  else
    RUNNER_USER="$(id -un)"
  fi
  if [ "$RUNNER_USER" = "root" ] && [ -e "$CODEX_PATH" ]; then
    CODEX_OWNER="$(stat -c '%%U' "$CODEX_PATH" 2>/dev/null || true)"
    if [ -n "$CODEX_OWNER" ] && [ "$CODEX_OWNER" != "root" ] && [ "$CODEX_OWNER" != "UNKNOWN" ]; then
      RUNNER_USER="$CODEX_OWNER"
    fi
  fi
  RUNNER_HOME="$(getent passwd "$RUNNER_USER" 2>/dev/null | awk -F: '{print $6; exit}' || true)"
  if [ -z "$RUNNER_HOME" ] && [ "$RUNNER_USER" = "$(id -un)" ]; then
    RUNNER_HOME="${HOME:-}"
  fi
  if [ -z "$RUNNER_HOME" ]; then
    echo "Unable to determine home directory for runner user '$RUNNER_USER'." >&2
    exit 1
  fi
  RUNNER_ENV_USER_LINES="HOME=$RUNNER_HOME
USER=$RUNNER_USER
LOGNAME=$RUNNER_USER"
  RUNNER_WRAPPER_USER_EXPORTS="export HOME=$RUNNER_HOME
export USER=$RUNNER_USER
export LOGNAME=$RUNNER_USER"
  RUNNER_SERVICE_USER_LINES="User=$RUNNER_USER
WorkingDirectory=$RUNNER_HOME"
fi
$SUDO mkdir -p "$INSTALL_DIR"
$SUDO rm -f "$INSTALL_DIR/runner.disabled"
echo "Downloading runner from $CONTROL_URL/api/v1/runner/artifacts/$ARTIFACT"
RUNNER_TMP="$INSTALL_DIR/codex-task-workbench-runner.$$.download"
cleanup_runner_tmp() {
  $SUDO rm -f "$RUNNER_TMP"
}
trap cleanup_runner_tmp EXIT
$SUDO rm -f "$RUNNER_TMP"
if [ -n "$RUNNER_TOKEN" ]; then
  $SUDO sh -c "curl -fsSL -H 'Authorization: Bearer $RUNNER_TOKEN' '$CONTROL_URL/api/v1/runner/artifacts/$ARTIFACT?t=$(date +%%s)' -o '$RUNNER_TMP'"
else
  $SUDO sh -c "curl -fsSL '$CONTROL_URL/api/v1/runner/artifacts/$ARTIFACT?t=$(date +%%s)' -o '$RUNNER_TMP'"
fi
$SUDO chmod +x "$RUNNER_TMP"
$SUDO mv -f "$RUNNER_TMP" "$RUNNER_BIN"
trap - EXIT
case "$PLATFORM" in
  linux)
    linux_systemd_available() {
      command -v systemctl >/dev/null 2>&1 || return 1
      [ -d /run/systemd/system ] || return 1
      [ "$(ps -p 1 -o comm= 2>/dev/null | tr -d '[:space:]')" = "systemd" ] || return 1
      systemctl show --property=Version --value >/dev/null 2>&1 || return 1
      return 0
    }
    start_standalone_runner() {
      $SUDO sh -c "cat > '$WRAPPER' <<EOF
#!/bin/sh
export CONTROL_URL=$CONTROL_URL
export RUNNER_ID=$RUNNER_ID
export CODEX_PATH=$CODEX_PATH
export RUNNER_TOKEN=$RUNNER_TOKEN
export CODEX_BYPASS_APPROVALS_AND_SANDBOX=$CODEX_BYPASS_APPROVALS_AND_SANDBOX
export RUNNER_RUN_AS=$RUN_AS
export PATH=$RUNNER_PATH
$RUNNER_WRAPPER_USER_EXPORTS
cd '$INSTALL_DIR'
exec '$RUNNER_BIN'
EOF"
      $SUDO chmod +x "$WRAPPER"
      if [ "$RUN_AS" = "user" ] && [ -n "$RUNNER_USER" ]; then
        $SUDO chown -R "$RUNNER_USER" "$INSTALL_DIR"
      fi
      if [ -f "$PID_FILE" ]; then
        OLD_PID="$(cat "$PID_FILE" 2>/dev/null || true)"
        case "$OLD_PID" in
          ''|*[!0-9]*) OLD_PID="" ;;
        esac
        if [ -n "$OLD_PID" ] && $SUDO kill -0 "$OLD_PID" 2>/dev/null; then
          if [ -r "/proc/$OLD_PID/cmdline" ] && tr '\000' ' ' < "/proc/$OLD_PID/cmdline" | grep -F "$RUNNER_BIN" >/dev/null 2>&1; then
            $SUDO kill "$OLD_PID" 2>/dev/null || true
            sleep 1
          fi
        fi
      fi
      START_CMD="cd '$INSTALL_DIR' && nohup '$WRAPPER' >> '$RUNNER_LOG' 2>> '$RUNNER_ERR_LOG' < /dev/null & echo \$! > '$PID_FILE'"
      if [ "$RUN_AS" = "user" ] && [ -n "$RUNNER_USER" ] && [ "$(id -un)" != "$RUNNER_USER" ]; then
        if command -v runuser >/dev/null 2>&1; then
          $SUDO runuser -u "$RUNNER_USER" -- sh -c "$START_CMD"
        else
          $SUDO su -s /bin/sh "$RUNNER_USER" -c "$START_CMD"
        fi
      else
        sh -c "$START_CMD"
      fi
      echo "Runner installed and started in standalone background mode with RUNNER_ID=$RUNNER_ID pid_file=$PID_FILE log=$RUNNER_LOG"
      echo "This host is not running systemd; restart the runner manually or rerun this installer after container/host restarts."
    }
    $SUDO sh -c "cat > '$ENV_FILE' <<EOF
CONTROL_URL=$CONTROL_URL
RUNNER_ID=$RUNNER_ID
CODEX_PATH=$CODEX_PATH
RUNNER_TOKEN=$RUNNER_TOKEN
CODEX_BYPASS_APPROVALS_AND_SANDBOX=$CODEX_BYPASS_APPROVALS_AND_SANDBOX
RUNNER_RUN_AS=$RUN_AS
PATH=$RUNNER_PATH
$RUNNER_ENV_USER_LINES
EOF"
    if linux_systemd_available; then
      $SUDO sh -c "cat > '$SERVICE_FILE' <<EOF
[Unit]
Description=Meridian Runner
After=network-online.target
Wants=network-online.target

[Service]
EnvironmentFile=$ENV_FILE
$RUNNER_SERVICE_USER_LINES
ExecStart=$RUNNER_BIN
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF"
      $SUDO systemctl daemon-reload
      $SUDO systemctl enable codex-task-workbench-runner.service
      $SUDO systemctl restart codex-task-workbench-runner.service
      echo "Runner installed and started as codex-task-workbench-runner.service with RUNNER_ID=$RUNNER_ID"
    else
      start_standalone_runner
    fi
    ;;
  darwin)
    $SUDO sh -c "cat > '$WRAPPER' <<EOF
#!/bin/sh
export CONTROL_URL=$CONTROL_URL
export RUNNER_ID=$RUNNER_ID
export CODEX_PATH=$CODEX_PATH
export RUNNER_TOKEN=$RUNNER_TOKEN
export CODEX_BYPASS_APPROVALS_AND_SANDBOX=$CODEX_BYPASS_APPROVALS_AND_SANDBOX
export RUNNER_RUN_AS=launchd
export PATH=$RUNNER_PATH
cd '$INSTALL_DIR'
exec '$RUNNER_BIN'
EOF"
    $SUDO chmod +x "$WRAPPER"
    $SUDO sh -c "cat > '$PLIST' <<EOF
<?xml version=\"1.0\" encoding=\"UTF-8\"?>
<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">
<plist version=\"1.0\">
<dict>
  <key>Label</key>
  <string>$LAUNCHD_LABEL</string>
  <key>ProgramArguments</key>
  <array>
    <string>$WRAPPER</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>WorkingDirectory</key>
  <string>$INSTALL_DIR</string>
  <key>StandardOutPath</key>
  <string>/var/log/codex-task-workbench-runner.log</string>
  <key>StandardErrorPath</key>
  <string>/var/log/codex-task-workbench-runner.err.log</string>
</dict>
</plist>
EOF"
    $SUDO launchctl bootout system "$PLIST" >/dev/null 2>&1 || true
    $SUDO launchctl bootstrap system "$PLIST"
    $SUDO launchctl enable "system/$LAUNCHD_LABEL" >/dev/null 2>&1 || true
    $SUDO launchctl kickstart -k "system/$LAUNCHD_LABEL" >/dev/null 2>&1 || true
    echo "Runner installed and started as launchd service $LAUNCHD_LABEL with RUNNER_ID=$RUNNER_ID"
    ;;
esac
`, shellQuote(controlURL), runnerID, codexPath, runnerToken, codexBypassApprovalsAndSandbox, runAs)
}

func (a *API) handleRunnerArtifact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	name := trimPrefix(r.URL.Path, "/api/v1/runner/artifacts/")
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, `\`) || strings.Contains(name, "..") {
		writeError(w, http.StatusNotFound, "not_found", "Resource not found.", nil)
		return
	}
	dir := os.Getenv("RUNNER_ARTIFACT_DIR")
	if dir == "" {
		dir = filepath.Join("artifacts", "runner")
	}
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Runner artifact not found. Build the runner binary and place it in RUNNER_ARTIFACT_DIR.", nil)
		return
	}
	http.ServeFile(w, r, path)
}

func (a *API) handleRunnerUpdateAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	servers, err := a.store.ListServers(r.Context())
	if err != nil {
		a.respond(w, http.StatusOK, nil, err)
		return
	}
	connected := a.runners.ConnectedRunnerIDs()
	results := make([]RunnerUpdateServerResult, 0, len(servers))
	response := RunnerUpdateAllResponse{
		RequestedAt: time.Now().UTC(),
		Results:     results,
	}
	for _, server := range servers {
		result := RunnerUpdateServerResult{
			ServerID:   server.ID,
			ServerName: serverDisplayName(server),
			RunnerID:   server.RunnerID,
		}
		if info := a.runners.Info(server.RunnerID); info != nil {
			if info.Version != "" {
				result.PreviousVersion = &info.Version
			}
		}
		if !connected[server.RunnerID] {
			result.Status = "skipped"
			result.Message = "Runner is not connected."
			response.Skipped++
			response.Results = append(response.Results, result)
			continue
		}
		if !a.runners.Supports(server.RunnerID, "self_update") {
			result.Status = "skipped"
			result.Message = "Connected runner is too old for in-app updates; reinstall it once from the install menu."
			response.Skipped++
			response.Results = append(response.Results, result)
			continue
		}
		env, err := a.runners.Request(server.RunnerID, "runner.update", RunnerUpdateRequestPayload{}, 10*time.Second)
		if err != nil {
			if errors.Is(err, ErrRunnerRequestTimeout) {
				result.Status = "failed"
				result.Message = "Runner did not acknowledge the update request in time."
			} else {
				a.logger.Warn("runner update request failed", "runner_id", server.RunnerID, "error", err)
				result.Status = "failed"
				result.Message = "Unable to send update request to runner."
			}
			response.Failed++
			response.Results = append(response.Results, result)
			continue
		}
		var payload RunnerUpdateResponsePayload
		if !decodeEnvelopePayload(env.Payload, &payload, a, "runner.update.response") {
			result.Status = "failed"
			result.Message = "Runner returned an invalid update response."
			response.Failed++
			response.Results = append(response.Results, result)
			continue
		}
		if payload.Accepted && payload.Error == nil {
			result.Status = "accepted"
			result.Message = payload.Message
			if result.Message == "" {
				result.Message = "Runner accepted the update request."
			}
			response.Accepted++
		} else {
			result.Status = "failed"
			result.Message = payload.Message
			if payload.Error != nil && strings.TrimSpace(*payload.Error) != "" {
				result.Message = *payload.Error
			}
			if result.Message == "" {
				result.Message = "Runner rejected the update request."
			}
			response.Failed++
		}
		response.Results = append(response.Results, result)
	}
	writeJSON(w, http.StatusOK, response)
}
