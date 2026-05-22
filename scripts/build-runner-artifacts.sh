#!/usr/bin/env sh
set -eu

script_dir="$(dirname "$0")"
repo_root="$(CDPATH= cd "$script_dir/.." && pwd)"
artifact_dir="${RUNNER_ARTIFACT_DIR:-$repo_root/artifacts/runner}"
go_exe="${GO_EXE:-go}"
runner_version="${RUNNER_VERSION:-dev}"

mkdir -p "$artifact_dir"

build_runner() {
  goos="$1"
  goarch="$2"
  output="$3"
  echo "Building $goos/$goarch -> $artifact_dir/$output"
  (
    cd "$repo_root"
    GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 "$go_exe" build \
      -trimpath \
      -ldflags="-s -w -X main.RunnerVersion=$runner_version" \
      -o "$artifact_dir/$output" \
      ./runner/cmd/runner
  )
}

build_runner windows amd64 runner-windows-amd64.exe
build_runner linux amd64 runner-linux-amd64
build_runner linux arm64 runner-linux-arm64
build_runner darwin amd64 runner-darwin-amd64
build_runner darwin arm64 runner-darwin-arm64

echo "Runner artifacts written to $artifact_dir"
