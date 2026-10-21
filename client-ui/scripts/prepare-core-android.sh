#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLIENT_UI_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${CLIENT_UI_ROOT}/.." && pwd)"
JNI_ROOT="${CLIENT_UI_ROOT}/android/app/src/main/jniLibs"

command -v go >/dev/null 2>&1 || {
  echo "Go toolchain not found. Install Go 1.27.3+ or set go on PATH." >&2
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

android_abi="${FRP_ANDROID_ABI:-arm64-v8a}"
case "$android_abi" in
  arm64-v8a)
    build_android_core arm64 arm64-v8a
    ;;
  armeabi-v7a)
    # Android/ARM requires external linking with the NDK compiler.
    ndk_version="${ANDROID_NDK_VERSION:-27.2.12479018}"
    ndk_root="${ANDROID_NDK_HOME:-${ANDROID_HOME:-${ANDROID_SDK_ROOT:-}}/ndk/${ndk_version}}"
    host_tag="${ANDROID_NDK_HOST_TAG:-linux-x86_64}"
    compiler="${CC:-${ndk_root}/toolchains/llvm/prebuilt/${host_tag}/bin/armv7a-linux-androideabi24-clang}"
    if [[ ! -x "$compiler" ]]; then
      echo "Android ARMv7 NDK compiler not found: $compiler" >&2
      exit 1
    fi
    output_dir="${JNI_ROOT}/armeabi-v7a"
    mkdir -p "$output_dir"
    CGO_ENABLED=1 CC="$compiler" GOOS=android GOARCH=arm GOARM=7 \
      go build -trimpath -buildmode=pie -o "${output_dir}/libfrpp.so" -ldflags "${ldflags}" ./cmd/frppc
    ;;
  *)
    echo "Unsupported Android ABI: $android_abi" >&2
    exit 1
    ;;
esac
popd >/dev/null

echo "Built Android core clients in ${JNI_ROOT}"
