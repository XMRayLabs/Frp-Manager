#!/usr/bin/env /bin/bash
set -euo pipefail

# Function to print usage
usage() {
    echo "Usage: $0 [--platform <platform>] [--bintype <bintype>] [--arch <arch>] [--skip-frontend] [--current]"
    echo "Set RELEASE_VERSION to override the default version from ./VERSION."
    echo "Platforms: windows, linux, darwin, all"
    echo "Binary Types: full, client, all"
    echo "Architectures: amd64, arm64, arm, all"
    echo "Example: $0 --platform linux --bintype full --arch amd64"
    exit 1
}

# Default values
PLATFORM="all"
BINTYPE="all"
ARCH="all"
SKIP_FRONTEND=false
CURRENT=false

# build variables
BUILD_DATE="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
GIT_COMMIT="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
GIT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
DEFAULT_VERSION="1.0.0"
if [[ -f VERSION ]]; then
    DEFAULT_VERSION="$(tr -d '\r\n' < VERSION)"
fi
VERSION="${RELEASE_VERSION:-$DEFAULT_VERSION}"
if [[ -z "${RELEASE_VERSION:-}" && "${GITHUB_REF_TYPE:-}" == "tag" && -n "${GITHUB_REF_NAME:-}" ]]; then
    VERSION="$GITHUB_REF_NAME"
fi
VERSION="${VERSION#v}"

# Parse arguments
while [[ "$#" -gt 0 ]]; do
    case $1 in
        --platform) PLATFORM="$2"; shift ;;
        --bintype) BINTYPE="$2"; shift ;;
        --arch) ARCH="$2"; shift ;;
        --skip-frontend) SKIP_FRONTEND=true ;;
        --current) CURRENT=true ;;
        *) usage ;;
    esac
    shift
done

if [[ "$CURRENT" == "true" ]]; then
    PLATFORM=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)
    if [[ "$ARCH" == "x86_64" ]]; then
        ARCH="amd64"
    fi
    if [[ "$ARCH" == "aarch64" ]]; then
        ARCH="arm64"
    fi
    BINTYPE="full"
fi

echo "Building for platform: $PLATFORM, binary type: $BINTYPE, architecture: $ARCH"
echo "Build Date: $BUILD_DATE"
echo "Git Commit: $GIT_COMMIT"
echo "Version: $VERSION"

BUILD_LD_FLAGS="-checklinkname=0 -X 'github.com/Sakurame1/frp-manager/conf.buildDate=${BUILD_DATE}' -X 'github.com/Sakurame1/frp-manager/conf.gitCommit=${GIT_COMMIT}' -X 'github.com/Sakurame1/frp-manager/conf.gitVersion=${VERSION}' -X 'github.com/Sakurame1/frp-manager/conf.gitBranch=${GIT_BRANCH}'"

mkdir -p dist

if [[ "$SKIP_FRONTEND" == "true" ]]; then
    echo "Skipping frontend build"
else
    echo "Building frontend"
    # Prepare build environment
    rm -rf dist/*

    # Build frontend
    pushd www
    pnpm install --frozen-lockfile
    pnpm build
    popd
fi

# Build function
build_binary() {
    local platform=$1
    local arch=$2
    local bintype=$3
    local output_name=""
    local source_path=""
    local binary_ld_flags=""

    # Determine output name and source path
    if [[ "$bintype" == "full" ]]; then
        source_path="./cmd/frpp"
        output_name="frp-manager"
    elif [[ "$bintype" == "client" ]]; then
        source_path="./cmd/frppc"
        output_name="frp-manager-client"
    else
        echo "Invalid binary type"
        return 1
    fi
    binary_ld_flags="${BUILD_LD_FLAGS} -X 'github.com/Sakurame1/frp-manager/conf.binaryType=${bintype}'"

    # Set executable extension for Windows
    local exe_ext=""
    if [[ "$platform" == "windows" ]]; then
        exe_ext=".exe"
    fi

    # Special handling for ARM architectures
    local goarch="$arch"
    local goarm=""
    if [[ "$arch" == "arm" ]]; then
        goarch="arm"
        if [[ "$platform" == "linux" ]]; then
            # Build for ARMv7 and ARMv6
            for arm_version in 7 6; do
                local arm_output="${output_name}-${platform}-armv${arm_version}l${exe_ext}"
                CGO_ENABLED=0 GOOS="$platform" GOARCH="$goarch" GOARM="$arm_version" \
                go build -o "dist/${arm_output}" -ldflags "$binary_ld_flags" $source_path
            done
            return 0
        fi
    fi

    # Standard build
    local output="${output_name}-${platform}-${arch}${exe_ext}"
    CGO_ENABLED=0 GOOS="$platform" GOARCH="$goarch" \
    go build -o "dist/${output}" -ldflags "$binary_ld_flags" $source_path
}

# Platforms array
PLATFORMS=()
if [[ "$PLATFORM" == "all" ]]; then
    PLATFORMS=("windows" "linux" "darwin" "android")
else
    PLATFORMS=("$PLATFORM")
fi

# Architectures array
ARCHS=()
if [[ "$ARCH" == "all" ]]; then
    ARCHS=("amd64" "arm64" "arm" "riscv64")
else
    ARCHS=("$ARCH")
fi

# Binary types array
BINTYPES=()
if [[ "$BINTYPE" == "all" ]]; then
    BINTYPES=("full" "client")
else
    BINTYPES=("$BINTYPE")
fi

# Build matrix
for platform in "${PLATFORMS[@]}"; do
    for arch in "${ARCHS[@]}"; do
        for bintype in "${BINTYPES[@]}"; do
            # 鐠佸墽鐤哾arwin閸滃瘍indows閻ㄥ嫮娅ч崥宥呭礋arch閿涘苯褰ч懗鑺ユЦ arm64 amd64
            if [[ "$platform" == "darwin" && "$arch" != "arm64" && "$arch" != "amd64" ]]; then continue; fi
            if [[ "$platform" == "windows" && "$arch" != "arm64" && "$arch" != "amd64" ]]; then continue; fi
            if [[ "$platform" == "android" && "$arch" != "arm64" ]]; then continue; fi
            echo "Building $bintype binary for $platform-$arch"
            build_binary "$platform" "$arch" "$bintype"
        done
    done
done

# Move to current directory if current enabled
if [[ "$CURRENT" == "true" ]]; then
    cp dist/frp* ./frp-manager
fi

echo "Build Done!"
