#!/usr/bin/env bash
set -euo pipefail

OS="$(uname -s)"
ARCH="$(uname -m)"

frp_manager_args=()
version="${FRP_MANAGER_VERSION:-latest}"

while [[ "$#" -gt 0 ]]; do
    case "$1" in
        --version)
            if [[ -z "${2:-}" ]]; then
                echo "Error: --version requires a version argument." >&2
                exit 1
            fi
            version="$2"
            shift 2
            ;;
        *)
            frp_manager_args+=("$1")
            shift
            ;;
    esac
done

release_base="https://github.com/XMRayLabs/Frp-Manager/releases/download/${version}"
if [[ "$version" == "latest" ]]; then
    release_base="https://github.com/XMRayLabs/Frp-Manager/releases/latest/download"
fi

asset_name=""
case "$OS" in
    Linux)
        case "$ARCH" in
            x86_64) asset_name="frp-manager-linux-amd64" ;;
            aarch64|arm64) asset_name="frp-manager-linux-arm64" ;;
            armv7l) asset_name="frp-manager-linux-armv7l" ;;
            armv6l) asset_name="frp-manager-linux-armv6l" ;;
            *)
                echo "Unsupported Linux architecture: $ARCH" >&2
                exit 1
                ;;
        esac
        ;;
    *)
        echo "This service installer supports Linux only. Download macOS binaries directly from GitHub Releases." >&2
        exit 1
        ;;
esac

download_url="${release_base}/${asset_name}"
current_dir="$(pwd)"
temp_dir="$(mktemp -d)"
trap 'rm -rf "$temp_dir"' EXIT

download_file() {
    local url="$1"
    local output="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fL "$url" -o "$output"
        return
    fi
    if command -v wget >/dev/null 2>&1; then
        wget -O "$output" "$url"
        return
    fi
    echo "Error: curl or wget is required to download frp-manager." >&2
    exit 1
}

echo "Downloading frp-manager ${version}: ${asset_name}"
download_file "$download_url" "$temp_dir/frp-manager"
download_file "${release_base}/SHA256SUMS-core.txt" "$temp_dir/SHA256SUMS-core.txt"

expected_hash="$(awk -v name="$asset_name" '$2 == name || $2 == "*" name { print tolower($1); exit }' "$temp_dir/SHA256SUMS-core.txt")"
if [[ -z "$expected_hash" ]]; then
    echo "Error: checksum for ${asset_name} was not found in SHA256SUMS-core.txt." >&2
    exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
    actual_hash="$(sha256sum "$temp_dir/frp-manager" | awk '{print tolower($1)}')"
elif command -v shasum >/dev/null 2>&1; then
    actual_hash="$(shasum -a 256 "$temp_dir/frp-manager" | awk '{print tolower($1)}')"
else
    echo "Error: sha256sum or shasum is required to verify the download." >&2
    exit 1
fi
if [[ "$actual_hash" != "$expected_hash" ]]; then
    echo "Error: SHA256 verification failed for ${asset_name}." >&2
    exit 1
fi
echo "SHA256 verified: ${actual_hash}"
chmod +x "$temp_dir/frp-manager"

find_frpp_executable() {
    local service_file
    service_file="$(systemctl show -p FragmentPath frpp.service 2>/dev/null | cut -d= -f2)"
    if [[ -z "$service_file" || ! -f "$service_file" ]]; then
        return 1
    fi
    local exec_start
    exec_start="$(grep -oP '^ExecStart=\K.*' "$service_file" || true)"
    if [[ -z "$exec_start" ]]; then
        return 1
    fi
    awk '{print $1}' <<< "$exec_start"
}

if systemctl list-units --type=service --all | grep -q '^  *frpp\.service'; then
    executable_path="$(find_frpp_executable || true)"
    if [[ -z "${executable_path:-}" ]]; then
        echo "Error: frpp.service exists, but its executable path could not be detected." >&2
        exit 1
    fi
    echo "Updating existing frpp service at: $executable_path"
    sudo "$executable_path" stop || true
    sudo "$executable_path" uninstall || true
    sudo install -m 0755 "$temp_dir/frp-manager" "$executable_path"
    sudo "$executable_path" version
    sudo "$executable_path" install "${frp_manager_args[@]}"
    sudo systemctl daemon-reload
    echo "Updated. Run: sudo systemctl restart frpp"
    exit 0
fi

echo "Installing frp-manager into: $current_dir/frp-manager"
sudo install -m 0755 "$temp_dir/frp-manager" "$current_dir/frp-manager"
sudo "$current_dir/frp-manager" install "${frp_manager_args[@]}"
sudo systemctl daemon-reload
sudo "$current_dir/frp-manager" start
sudo "$current_dir/frp-manager" version
sudo systemctl restart frpp
sudo systemctl enable frpp
echo "frp-manager service is installed and started."
