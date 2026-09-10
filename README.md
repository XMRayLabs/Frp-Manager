# frp-manager

English | [中文](README_zh.md)

`frp-manager` is a centralized FRP control plane with a Web dashboard, managed FRP server/client nodes, WireGuard
networking, and Windows/macOS/Android client managers. Downloads use GitHub Releases directly; containers use Docker
Hub and official upstream base images.

## Choose A Package

| Package | Best for |
| --- | --- |
| Full binary `frp-manager-*` | Master, Server, Client, Linux automation |
| Client-only binary `frp-manager-client-*` | Headless client nodes and systemd |
| Windows GUI installer | Managing and controlling a few desktop Client profiles |
| macOS GUI DMG | Desktop Client profiles; ad-hoc signed builds require manual approval |
| Android ARM64 APK | Mobile Client profiles backed by a persistent foreground service |

The GUI packages use the same Go client core as the headless package. Use the binary for servers, unattended nodes,
containers, and automation. Use the GUI when an operator needs profile switching and manual start/stop controls.

## Latest Downloads

| Platform | GUI | Full binary |
| --- | --- | --- |
| Windows x64 | [GUI installer](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-client-windows-x64.exe) | [Binary](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-windows-amd64.exe) |
| Windows ARM64 | Not available | [Binary](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-windows-arm64.exe) |
| macOS Intel | [GUI DMG](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-client-macos-x64.dmg) | [Binary](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-darwin-amd64) |
| macOS Apple Silicon | [GUI DMG](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-client-macos-arm64.dmg) | [Binary](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-darwin-arm64) |
| Linux x64 | Not needed | [Binary](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-linux-amd64) |
| Linux ARM64 | Not needed | [Binary](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-linux-arm64) |
| Android ARM64 | [APK](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-client-android.apk) | Included in APK |

[All latest assets and SHA256 files](https://github.com/XMRayLabs/Frp-Manager/releases/latest)

## Install The Management Panel

Requirements: Linux, Docker Engine, Docker Compose v2, and a public domain name.

```bash
git clone https://github.com/XMRayLabs/Frp-Manager.git
cd frp-manager
cp deploy/.env.example deploy/.env
openssl rand -base64 48
```

Put the generated secret in `deploy/.env`. Set the public API/RPC host names and temporarily set
`APP_ENABLE_REGISTER=true`.

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.yml pull
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
docker compose --env-file deploy/.env -f deploy/docker-compose.yml logs -f
```

Open `http://SERVER_IP:9000` and create the first administrator. Immediately change
`APP_ENABLE_REGISTER=false` and run the Compose `up -d` command again.

Use [deploy/nginx.conf.example](deploy/nginx.conf.example) for HTTPS and WebSocket proxying. With one HTTPS domain,
the generated endpoints should be:

```text
API: https://manager.example.com
RPC: wss://manager.example.com
```

## Install A Server Node

1. Open **Servers** in the dashboard and create a Server.
2. Click its ID to open the install guide.
3. Verify the displayed API and RPC endpoints.
4. Run the Linux command on the public server.

```bash
curl -fSL https://raw.githubusercontent.com/XMRayLabs/Frp-Manager/main/install.sh \
  | bash -s -- --version latest server \
  -i SERVER_ID -s SERVER_SECRET \
  --api-url https://manager.example.com \
  --rpc-url wss://manager.example.com
```

The installer downloads directly from GitHub Releases, verifies `SHA256SUMS-core.txt`, installs a systemd service,
and starts it. Open the FRPS ports configured for this node. Linux is recommended for long-running Server nodes.

Client and Server processes check GitHub Releases every time they start. When a newer semantic version exists, the
matching full or client-only binary is downloaded, verified against `SHA256SUMS-core.txt`, replaced, and restarted.
Set `APP_AUTO_UPDATE=false` to disable this behavior. Signed application bundles are updated as complete packages:
the Android core is updated with the APK, while desktop GUI-managed services are updated with the Windows installer
or macOS DMG/ZIP. Auto-update only installs a strictly newer semantic version; rebuilt assets under the same version
are never treated as an update.

## Install A Client Node

### Headless binary

Create a Client in the dashboard, click its ID, verify the endpoints, and copy the generated command:

```bash
curl -fSL https://raw.githubusercontent.com/XMRayLabs/Frp-Manager/main/install.sh \
  | bash -s -- --version latest client \
  -i CLIENT_ID -s CLIENT_SECRET \
  --api-url https://manager.example.com \
  --rpc-url wss://manager.example.com
```

With an existing binary:

```bash
./frp-manager client \
  -i CLIENT_ID -s CLIENT_SECRET \
  --api-url https://manager.example.com \
  --rpc-url wss://manager.example.com
```

### Windows/macOS GUI

1. Install the GUI from the download table.
2. Create a profile.
3. Copy the Client ID, secret, API URL, and RPC URL from the dashboard install guide.
4. Save the profile.
5. On Windows, select **Install / Apply** and approve the UAC prompt. The GUI installs the core and protected
   configuration under `%ProgramFiles%\frp-manager`. It only reports success after the service registers with the
   manager; service diagnostics are written to `%ProgramFiles%\frp-manager\service.log`.
6. On macOS, select the bundled/full client binary, click **Install / Apply**, and approve the native administrator prompt.

The macOS GUI installs the daemon core at `/usr/local/libexec/frp-manager/frpp` and writes connection settings to
`/etc/frpp/.env` with mode `0600`. The Secret is not stored in LaunchDaemon arguments; do not launch the whole GUI
with `sudo`. Public deployments should use an `https://` API and `grpc://` (TLS) or `wss://` RPC. The GUI rejects
remote insecure `http://`/`ws://` endpoints unless the operator explicitly enables the isolated-network override.

When signing credentials are unavailable, CI publishes an ad-hoc signed, non-notarized macOS package. Open it once, then go to
**System Settings → Privacy & Security → Open Anyway**. Only approve a package downloaded from this repository whose
SHA256 matches the Release checksum. Signed/notarized builds open normally.

### Android GUI

Install the ARM64 APK, create or import a Client profile, and select **Install / Apply**. Android runs the bundled
core as a foreground service and keeps a persistent system notification while connected. **Stop** disables restart
after reboot; **Start** or **Install / Apply** enables it again. Android updates must install a newer APK signed with
the same persistent key.

## WireGuard Networking

Runtime nodes must currently be Linux clients with root access and `/dev/net/tun`.

```bash
sudo modprobe tun
echo 'net.ipv4.ip_forward = 1' | sudo tee /etc/sysctl.d/99-frp-manager.conf
echo 'net.ipv6.conf.all.forwarding = 1' | sudo tee -a /etc/sysctl.d/99-frp-manager.conf
sudo sysctl -p /etc/sysctl.d/99-frp-manager.conf
```

1. Make every participating Client online.
2. Create a CIDR under **Networking → Networks**, for example `10.10.0.0/24`.
3. Choose at least one public Linux Client as a relay.
4. Create its UDP or WebSocket endpoint under **Endpoints**.
5. Use **Devices → Join Network** for each Client.
6. Bind the endpoint only to public nodes and assign unique addresses inside the CIDR.
7. Verify the topology, then ping the virtual addresses.

Prefer ACLs and automatic shortest-path routing. Manual links bypass ACL behavior and do not adapt to endpoint changes.
See the [full WireGuard guide](docs/en/wireguard.md).

## Repository Layout

| Path | Responsibility |
| --- | --- |
| `cmd/frpp`, `cmd/frppc` | Full and client-only Go entry points |
| `biz`, `services`, `models` | Business logic, infrastructure, persistence |
| `middleware`, `conf`, `utils` | Security, configuration, shared utilities |
| `www` | Next.js management panel |
| `client-ui` | Electron/Capacitor client manager |
| `docs` | VitePress documentation |
| `deploy` | Compose, environment, and Nginx templates |
| `.github/workflows` | Validation and release automation |

See [docs/project-structure.md](docs/project-structure.md).

## Development And Release

- Node.js 24.x
- Go 1.25+
- Semantic versions such as `1.0.1`

```bash
git tag v1.0.1
git push origin v1.0.1
```

The release workflow publishes core binaries, Windows EXE, macOS DMG/ZIP, Android APK, SHA256 files, and Docker Hub
images. Ad-hoc signed macOS artifacts are marked `adhoc`; Android release APKs require a persistent keystore.
Without Android signing secrets, CI publishes a debug-signed preview APK and an `ANDROID-BUILD-NOTICE.txt` warning.

See [GitHub automated releases](docs/github-release.md) for Actions secrets, signing, Docker Hub publishing, and version tags.

## License

GPL-3.0

### PostgreSQL 多用户部署

1.1.0 支持 PostgreSQL 部署和 SQLite 离线迁移，保留账号及设备密钥。参见[PostgreSQL 部署与迁移指南](docs/postgresql.md)。
