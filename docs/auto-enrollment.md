# 自动接入与启动更新

在管理面板的「客户端」或「服务端」列表点击「自动接入」，面板自动生成有效期为 24 小时的接入 Token 和对应命令。首次接入无需手动创建节点，也无需填写节点 ID、节点连接密钥。服务端接入按钮目前显示给管理员。

下载新版内核后执行面板生成的命令，例如：

```sh
frp-manager client --api-url https://panel.example.com --rpc-url wss://panel.example.com --join-token TOKEN
frp-manager server --api-url https://panel.example.com --rpc-url wss://panel.example.com --join-token TOKEN
```

省略 `--rpc-url` 时，自动从 HTTP/HTTPS 面板地址推导 WS/WSS 连接；反向代理需支持 `/wsgrpc`。Windows PowerShell 中当前目录程序使用 `./frp-manager.exe`。

节点注册后立即出现在对应列表，列表每 5 秒刷新；在列表中进入编辑即可配置隧道、备注等。服务端首次使用面板观察到的连接来源 IP，NAT 或反向代理环境下请检查并修改为可访问的公网地址；反向代理的真实来源地址取决于面板的 `APP_TRUSTED_PROXIES` 设置。自动注册并不自动创建业务隧道。

## 身份与重启

默认创建长期节点。内核先保存随机生成的节点名称，再向面板注册并取得独立连接密钥，最后保存完整身份。重新运行相同命令会复用该身份，即使接入 Token 已过期也不需要重新注册。也可删除命令中的 `--join-token` 参数，保留相同面板地址和可选 `--id`。

身份按面板 API 地址、角色和显式 `--id` 隔离，保存在运行账户的系统配置目录 `frp-manager/node-*.json` 中（Windows 通常为 `%APPDATA%`，Linux 通常为 `$XDG_CONFIG_HOME` 或 `~/.config`）。文件使用私有权限创建，包含连接密钥，请勿分享或随意删除。更换运行账户、改变面板地址或切换 `--id` 会使用另一份身份；将前台运行改成系统服务时应在最终服务账户下首次接入。

客户端可显式使用 `--ephemeral=true` 创建临时节点，此模式不保存身份。旧的 `--id` / `--secret` 直接连接方式继续可用。

前台命令保持运行即在线。需要安装系统服务时，可以使用相同连接参数：

```sh
frp-manager install client --api-url https://panel.example.com --rpc-url wss://panel.example.com --join-token TOKEN
frp-manager start
```

服务端将 `client` 改为 `server`。系统服务安装通常需要管理员/root 权限，服务首次启动需在 Token 过期前完成。

## 自动更新

客户端与独立服务端每次启动内核，默认检查 GitHub 发布的新版本（`APP_AUTO_UPDATE=true`）。发现更新后下载、校验官方 SHA256 和二进制、保留备份、替换并重新启动；Windows 使用独立更新进程，系统服务通过已有更新服务/执行器完成替换。

检查失败或下载失败时记录警告并继续使用当前内核。版本检查有短超时，下载/更新准备阶段最长 3 分钟。可以通过 `APP_AUTO_UPDATE=false` 禁用。这里的启动指 `client` / `server` 内核进程启动，不包括只重载单条隧道配置，也不改变面板 `master` 进程的更新策略。Android 和 macOS App 包内核沿用项目现有的应用包更新限制。

## 验证

新增测试覆盖客户端/服务端首次注册、令牌缺失时重启复用身份、注册失败不保存有效凭据、临时节点不持久化及 Windows 覆盖身份文件。自动更新复用现有校验、替换和版本检测测试。实际 GitHub 发布升级、系统服务重启及跨设备网络连接需要在部署环境验证。
