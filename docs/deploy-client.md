# 部署 Client

Client 负责连接内网服务并执行面板下发的 FRPC、Worker 和 WireGuard 配置。

## 从面板取得连接信息

1. 登录 Master。
2. 打开 **客户端**，创建 Client。
3. 点击 Client ID 打开安装向导。
4. 确认 API 与 RPC 都是目标机器能够访问的公网地址。
5. 复制对应系统命令。

推荐使用 HTTPS/WSS：

```text
API URL: https://manager.example.com
RPC URL: wss://manager.example.com
```

## Linux systemd

```bash
curl -fSL https://raw.githubusercontent.com/XMRayLabs/Frp-Manager/main/install.sh \
  | bash -s -- --version latest client \
  -i CLIENT_ID -s CLIENT_SECRET \
  --api-url https://manager.example.com \
  --rpc-url wss://manager.example.com
```

安装脚本只从 GitHub Releases 直连下载，并校验 `SHA256SUMS-core.txt`。检查服务：

```bash
systemctl status frpp
journalctl -u frpp -f
```

## Windows

推荐使用最新版
[Windows GUI](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-client-windows-x64.exe)。
点击 **Install / Apply** 后批准 UAC 提示。GUI 会把核心与仅管理员可读的配置安装到
`%ProgramFiles%\frp-manager`。
也可以在面板安装向导中复制 PowerShell 命令，安装无界面 Windows 服务。

## macOS

桌面用户可使用 Intel 或 Apple Silicon GUI DMG。无人值守环境可下载完整二进制并运行面板生成的启动命令。
无签名 GUI 首次打开需要在 **系统设置 → 隐私与安全性** 中点击 **仍要打开**。

## Android

ARM64 设备可安装最新版
[Android APK](https://github.com/XMRayLabs/Frp-Manager/releases/latest/download/frp-manager-client-android.apk)。
创建或导入配置后点击 **Install / Apply**。客户端核心会在前台服务中运行，连接期间 Android 会显示常驻通知。
点击 **Stop** 会同时关闭开机恢复；再次点击 **Start** 或 **Install / Apply** 会重新启用。

## 已有二进制

```bash
./frp-manager client \
  -i CLIENT_ID -s CLIENT_SECRET \
  --api-url https://manager.example.com \
  --rpc-url wss://manager.example.com
```

不要把 Client Secret 写入公开日志、工单或代码仓库。
