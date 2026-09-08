# frp-manager

[English](README.md) | 中文

`frp-manager` 是基于 FRP 的集中管理平台，包含管理面板、FRP 服务端、FRP 客户端、WireGuard 组网以及
Windows/macOS/Android 图形客户端。项目下载仅使用 GitHub Releases，容器仅使用 Docker Hub 和官方基础镜像。

## 版本怎么选

| 版本 | 适用场景 | 建议 |
| --- | --- | --- |
| 完整二进制 `frp-manager-*` | Master 管理面板、Server、Client、自动化部署 | Linux 服务器首选 |
| 客户端二进制 `frp-manager-client-*` | 无界面客户端、systemd、脚本与容器 | Linux 节点和批量部署首选 |
| Windows GUI 安装包 | 管理多个 Client 配置并安装、启动、停止本地客户端 | Windows 桌面用户首选 |
| macOS GUI DMG | 与 Windows GUI 相同；无签名包需要用户手动批准 | macOS 桌面用户 |
| Android ARM64 APK | 管理 Client 配置并通过前台服务运行内置核心 | Android 移动节点 |

GUI 版本内部仍使用同一个 Go 客户端核心。GUI 适合人工管理少量节点；纯二进制版本资源占用更低，适合服务器、
无人值守运行、远程维护和批量部署。Server 和 Master 必须使用完整二进制或 Docker 镜像。

## 最新版下载

所有链接直接指向 `github.com/XMRayLabs/Frp-Manager`，安装脚本还会校验 Release 中的 SHA256 清单。

| 系统 | 图形客户端 | 完整二进制 |
| --- | --- | --- |
| Windows x64 | [GUI 安装包](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-client-windows-x64.exe) | [完整二进制](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-windows-amd64.exe) |
| Windows ARM64 | 暂无 GUI | [完整二进制](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-windows-arm64.exe) |
| macOS Intel | [GUI DMG](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-client-macos-x64.dmg) | [完整二进制](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-darwin-amd64) |
| macOS Apple Silicon | [GUI DMG](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-client-macos-arm64.dmg) | [完整二进制](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-darwin-arm64) |
| Linux x64 | 不需要 GUI | [完整二进制](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-linux-amd64) |
| Linux ARM64 | 不需要 GUI | [完整二进制](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-linux-arm64) |
| Android ARM64 | [APK](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-client-android.apk) | APK 已内置核心 |

[查看所有版本和 SHA256 文件](https://github.com/XMRayLabs/Frp-Manager/releases/latest)

## 安装管理面板

### Docker Compose（推荐）

要求：Linux、Docker Engine、Docker Compose v2、一个可解析到服务器的域名。

```bash
git clone https://github.com/XMRayLabs/Frp-Manager.git
cd frp-manager
cp deploy/.env.example deploy/.env
openssl rand -base64 48
```

把随机值写入 `deploy/.env` 的 `APP_GLOBAL_SECRET`，并修改：

```dotenv
FRP_MANAGER_VERSION=latest
MASTER_API_HOST=manager.example.com
MASTER_API_PORT=9000
MASTER_API_SCHEME=https
MASTER_RPC_HOST=manager.example.com
MASTER_RPC_PORT=9001
APP_COOKIE_SECURE=true
APP_TRUSTED_PROXIES=127.0.0.1,::1
APP_ENABLE_REGISTER=true
APP_AUTO_UPDATE=true
```

首次启动：

```bash
docker compose --env-file deploy/.env -f deploy/docker-compose.yml pull
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
docker compose --env-file deploy/.env -f deploy/docker-compose.yml logs -f
```

打开 `http://服务器地址:9000`，创建首个管理员。创建完成后立即把
`APP_ENABLE_REGISTER=false`，然后再次执行 `docker compose ... up -d`。

数据保存在 `frp-manager-data` 卷。模板启用了只读根文件系统、`no-new-privileges`、删除 Linux capabilities、
HttpOnly Cookie 和 TLS 校验。远程 Shell 按当前项目要求默认开启。

### HTTPS 反向代理

复制 [deploy/nginx.conf.example](deploy/nginx.conf.example)，替换域名和证书路径。该配置同时支持网页 API 与
WebSocket RPC。外部使用 443 时，面板中的客户端连接地址应为：

```text
API: https://manager.example.com
RPC: wss://manager.example.com
```

如果反向代理不在本机，把它的真实 IP 或 CIDR 加入 `APP_TRUSTED_PROXIES`，不要使用 `0.0.0.0/0`。

## 安装 Server

Server 是公网入口，推荐使用 Linux 完整二进制。

1. 登录面板，进入 **服务端**。
2. 创建 Server，填写 FRPS 配置和公网监听端口。
3. 点击表格中的 Server ID。
4. 核对显示的 API 与 RPC 地址。
5. 复制 Linux 安装命令，在目标服务器以 root 或 sudo 运行。

示例：

```bash
curl -fSL https://raw.githubusercontent.com/XMRayLabs/Frp-Manager/main/install.sh \
  | bash -s -- --version latest server \
  -i SERVER_ID -s SERVER_SECRET \
  --api-url https://manager.example.com \
  --rpc-url wss://manager.example.com
```

脚本直接从 GitHub Releases 下载，校验 `SHA256SUMS-core.txt`，安装 systemd 服务并启动。请在防火墙放行 FRPS
配置所需端口。Windows Server 也可使用面板生成的 PowerShell 命令，但长期运行仍推荐 Linux。

Client 和 Server 每次启动都会检查 GitHub Releases。发现更高的语义化版本后，会下载与当前完整/精简二进制
类型一致的文件，校验 `SHA256SUMS-core.txt`，完成替换并重新启动。设置 `APP_AUTO_UPDATE=false` 可以关闭。
Android 核心随签名 APK 更新；macOS GUI 随新 DMG/ZIP 更新，GUI 安装的独立守护进程核心仍会自动更新。

## 安装 Client

### 纯二进制方式

适合 Linux、云主机、NAS、无人值守节点和自动化部署。

1. 面板进入 **客户端**，创建 Client。
2. 点击 Client ID，确认 API/RPC 地址。
3. 复制对应系统命令。

Linux 示例：

```bash
curl -fSL https://raw.githubusercontent.com/XMRayLabs/Frp-Manager/main/install.sh \
  | bash -s -- --version latest client \
  -i CLIENT_ID -s CLIENT_SECRET \
  --api-url https://manager.example.com \
  --rpc-url wss://manager.example.com
```

已有二进制时可直接运行：

```bash
./frp-manager client \
  -i CLIENT_ID -s CLIENT_SECRET \
  --api-url https://manager.example.com \
  --rpc-url wss://manager.example.com
```

### Windows/macOS 图形客户端

适合需要切换多个配置、查看运行状态和手动启停的桌面用户。

1. 从“最新版下载”表安装 GUI。
2. 新建 Profile。
3. 从面板 Client ID 弹窗复制 `Client ID`、`Secret`、`API URL`、`RPC URL`。
4. GUI 中保存 Profile。
5. Windows 安装包已内置客户端核心；点击 **Install / Apply** 并批准 UAC。核心和管理员专用配置会安装到
   `%ProgramFiles%\frp-manager`。
6. macOS 选择内置核心或完整二进制路径，点击 **Install / Apply**，在系统授权框中输入管理员密码。

macOS GUI 会把守护进程核心安装到 `/usr/local/libexec/frp-manager/frpp`，并把连接配置以 `0600`
权限写入 `/etc/frpp/.env`。Secret 不会写入 LaunchDaemon 命令行；不要使用 `sudo` 启动整个 GUI。
公网连接应使用 `https://` API 和 `grpc://`（TLS）或 `wss://` RPC。GUI 默认拒绝不安全的远程
`http://`/`ws://`，仅隔离可信网络可显式启用例外。

无签名 macOS 包不会阻止 Release 构建。第一次打开时：

1. 尝试打开应用一次。
2. 打开 **系统设置 → 隐私与安全性**。
3. 在安全提示旁点击 **仍要打开**，再次确认。

只应对从本仓库下载且 SHA256 匹配的包执行该操作。有 Apple 签名密钥时，Action 会自动签名并公证，不需要手动批准。

### Android 图形客户端

在 ARM64 设备安装 APK，新建或导入 Profile 后点击 **Install / Apply**。内置核心会在 Android 前台服务中
运行，连接期间显示常驻系统通知。点击 **Stop** 会关闭服务并禁用开机恢复；再次点击 **Start** 或
**Install / Apply** 会重新启用。更新 APK 时必须使用相同的固定签名密钥。

## WireGuard 组网

当前组网运行端要求 Linux，并需要 root 权限和 `/dev/net/tun`。

```bash
sudo modprobe tun
echo 'net.ipv4.ip_forward = 1' | sudo tee /etc/sysctl.d/99-frp-manager.conf
echo 'net.ipv6.conf.all.forwarding = 1' | sudo tee -a /etc/sysctl.d/99-frp-manager.conf
sudo sysctl -p /etc/sysctl.d/99-frp-manager.conf
```

面板操作顺序：

1. 先确保所有参与组网的 Client 在线。
2. 进入 **组网 → 网络**，创建 CIDR，例如 `10.10.0.0/24`。
3. 至少选择一个具有公网 IP 的 Linux Client 作为中继。
4. 在 **组网 → 端点** 创建公网端点，填写域名/IP、UDP 或 WebSocket 类型及端口。
5. 在 **组网 → 设备** 点击 **加入网络**，为每个 Client 创建接口，例如 `frpp0`。
6. 公网节点绑定端点；内网节点可不绑定端点。
7. 为设备分配同一 CIDR 中不重复的地址，例如 `10.10.0.1`、`10.10.0.2`。
8. 在网络拓扑中确认链路，然后互相 `ping` 虚拟地址。

生产环境优先使用 ACL 和自动最短路径。手工连接会绕过 ACL 且不会随端点变化自动调整。
完整说明见 [WireGuard 文档](docs/wireguard.md)。

## 项目目录

| 目录 | 内容 |
| --- | --- |
| `cmd/frpp` | Master/Server/Client 完整命令与内嵌网页 |
| `cmd/frppc` | 桌面包使用的精简 Client 核心 |
| `biz`, `services`, `models` | 业务、服务与数据模型 |
| `middleware`, `conf`, `utils` | 安全中间件、配置与基础能力 |
| `www` | Next.js 管理面板 |
| `client-ui` | Electron/Capacitor 图形客户端 |
| `docs` | VitePress 文档 |
| `deploy` | Compose、环境变量、Nginx 模板 |
| `.github/workflows` | 测试、Release、Docker 和文档自动化 |

详见 [项目结构说明](docs/project-structure.md)。

## 开发与发布

- Node.js：24.x
- Go：1.25+
- 版本：语义化版本，例如 `1.0.1`

```bash
git tag v1.0.1
git push origin v1.0.1
```

Release 同时发布核心二进制、Windows EXE、macOS DMG/ZIP、Android APK、SHA256 文件和 Docker Hub 镜像。
macOS 无证书时发布带 ad-hoc 签名且未公证的 `adhoc` 包；Android 正式 APK 需要固定 keystore。
如果未配置 Android 签名 Secrets，CI 会发布带 `ANDROID-BUILD-NOTICE.txt` 提示的调试签名预览 APK。

Actions Secrets 和标签发布说明见 [GitHub 自动发布](docs/github-release.md)。

## License

GPL-3.0

自动接入与启动更新：参见 [使用说明](docs/auto-enrollment.md)，支持客户端/服务端一条命令注册、自动保存身份和启动时检查更新。
