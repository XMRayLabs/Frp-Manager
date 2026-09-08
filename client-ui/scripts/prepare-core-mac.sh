#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLIENT_UI_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${CLIENT_UI_ROOT}/.." && pwd)"
OUTPUT_DIR="${CLIENT_UI_ROOT}/resources/binaries"
OUTPUT="${OUTPUT_DIR}/frpp"

command -v go >/dev/null 2>&1 || {
  echo "Go toolchain not found. Install Go 1.25+ or set go on PATH." >&2
  exit 1
}

mkdir -p "${OUTPUT_DIR}"

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

target_arch="${TARGET_ARCH:-$(uname -m)}"
case "${target_arch}" in
  x86_64|x64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *)
    echo "Unsupported macOS architecture: ${target_arch}" >&2
    exit 1
    ;;
esac

ldflags="-checklinkname=0 -X github.com/Sakurame1/frp-manager/conf.buildDate=${build_date} -X github.com/Sakurame1/frp-manager/conf.gitCommit=${git_commit} -X github.com/Sakurame1/frp-manager/conf.gitVersion=${version} -X github.com/Sakurame1/frp-manager/conf.gitBranch=${git_branch} -X github.com/Sakurame1/frp-manager/conf.binaryType=client"

CGO_ENABLED=0 GOOS=darwin GOARCH="${goarch}" go build -o "${OUTPUT}" -ldflags "${ldflags}" ./cmd/frppc
chmod +x "${OUTPUT}"
echo "Built core client: ${OUTPUT}"
popd >/dev/null
