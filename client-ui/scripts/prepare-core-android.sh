#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLIENT_UI_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${CLIENT_UI_ROOT}/.." && pwd)"
JNI_ROOT="${CLIENT_UI_ROOT}/android/app/src/main/jniLibs"

command -v go >/dev/null 2>&1 || {
  echo "Go toolchain not found. Install Go 1.25+ or set go on PATH." >&2
  exit 1
}

pushd "${REPO_ROOT}" >/dev/null
build_date="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
git_commit="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
git_branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
version="${RELEASE_VERSION:-}"
if [[ -z "${version}" && -f VERSION ]]; then
  version="$(tr -d '\r\n' < VERSION)"
fi
version="${version:-1.0.0}"
version="${version#v}"

ldflags="-checklinkname=0 -s -w -X github.com/Sakurame1/frp-manager/conf.buildDate=${build_date} -X github.com/Sakurame1/frp-manager/conf.gitCommit=${git_commit} -X github.com/Sakurame1/frp-manager/conf.gitVersion=${version} -X github.com/Sakurame1/frp-manager/conf.gitBranch=${git_branch} -X github.com/Sakurame1/frp-manager/conf.binaryType=client"

build_android_core() {
  local goarch="$1"
  local android_abi="$2"
  local output_dir="${JNI_ROOT}/${android_abi}"
  mkdir -p "${output_dir}"
  CGO_ENABLED=0 GOOS=android GOARCH="${goarch}" \
    go build -trimpath -buildmode=pie -o "${output_dir}/libfrpp.so" -ldflags "${ldflags}" ./cmd/frppc
}

build_android_core arm64 arm64-v8a
popd >/dev/null

echo "Built Android core clients in ${JNI_ROOT}"
