# 可选网站域名映射

这是一项独立的可选功能。服务端默认不启用，用户也只有选择“网站映射”才使用自动子域名；原 TCP、HTTP/SOCKS5 代理不变。无需为每个网站新增 DNS、修改 Nginx 或重启面板。

## 一台服务端的推荐部署

以下以面板 `frp.xmray.de`、FRPS 与宿主机 Nginx 在同一台机器、Docker 使用 `network_mode: host` 为例。现有 PostgreSQL、Compose 环境变量、RPC/API 地址不需要改。

1. 在 DNS 管理中添加 `*.frp` 的 A 记录指向这台 **FRPS 网站入口服务器**的 IPv4。确实有可用 IPv6 时再添加对应 AAAA；不要给不可达的 IPv6 添加记录。面板域名本身的解析保留。
2. 准备覆盖 `*.frp.xmray.de` 的 TLS 证书。`*.xmray.de` 不能覆盖这一层。使用 Let's Encrypt 时需 DNS-01 验证，建议使用 DNS 提供商的插件自动续期；仅做手动 DNS 验证的证书续期也需要人工参与。
3. 在 Nginx 的 http 配置内加载 [nginx.website.conf.example](../deploy/nginx.website.conf.example)，保留原面板/RPC server 块。修改其中证书路径。80 跳转 HTTPS，443 把所有网站子域名转到本机 `9080`，保留 Host，并支持 WebSocket 与 AI/SSE 流式响应。
4. 先将承载网站的服务端内核升级至 1.1.7 或更新版本（内置服务端随面板镜像更新，独立服务端使用“升级内核”）。然后在面板“服务端 → 修改配置 → 常用配置”中勾选“网站域名映射（可选）”，填写：

   | 字段 | 值 |
   | --- | --- |
   | 子域名后缀 | `frp.xmray.de` |
   | HTTP 入口端口 | `9080` |
   | 对外网站地址 | `https://frp.xmray.de` |

   `9080` 必须空闲，不能使用面板 API 的 9000、RPC 的 9001、FRPS 的 7000 或已由 Nginx 使用的 80/443。保存会自动下发服务端配置，FRPS 可能短暂重连。不要为了仅保护网站入口就随意把全局 `proxyBindAddr` 改为回环地址，那会影响已有 TCP/UDP 映射。
5. 检查后加载 Nginx：

   ```bash
   sudo nginx -t && sudo systemctl reload nginx
   ```

6. 用户打开“客户端 → 修改配置 → 网站映射”，或“隧道管理 → 添加隧道 → 网站映射”。例如账号 `ig01` 输入名称 `qwen`、本地端口 `3000`，即可得到 `https://qwen-ig01.frp.xmray.de`。无需填写服务器端口。默认本地目标 `127.0.0.1`，可以展开修改成局域网地址。

`对外网站地址` 是生成子域名 URL 的模板，域名部分必须与子域名后缀一致；该裸域名继续作为面板使用，不会把面板根路径改成用户网站。自定义公共 HTTPS 端口可填 `https://frp.xmray.de:8443`，相应修改 Nginx 与防火墙。

普通网站映射复用现有 FRP HTTP 子域名功能；只有叠加多账号“网站访问密码”时，客户端需要升级至 1.1.7 或更新版本并在线。密码默认关闭；开启后最多 32 组，关闭可恢复直接访问。仅支持后端 HTTP 网站，HTTPS 在公网入口 Nginx 终止。

防火墙放行公网 80/443。FRPS 的 9080 推荐仅允许本机 Nginx访问（集中入口时仅允许入口地址），不要无意影响现有隧道端口。面板 Cookie 域保持默认空值，即 `APP_COOKIE_DOMAIN` 不设置，不要改为父域共享。

## 多个服务端

所有支持 HTTP 的 FRPS 都具备底层能力，但每台 **需要承载网站** 的服务端必须分别配置；不使用网站的服务端无需改动。

最简单可靠的方式是每台使用不同后缀。例如 A 用 `frp.xmray.de`，B 用 `frp2.xmray.de`；分别配置泛解析、对应泛域名证书、Nginx 和面板服务端入口。用户选择 B 后，地址自动变为 `qwen-ig01.frp2.xmray.de`。

不要把同一个泛解析同时指向几台没有共享路由的服务器，那会随机访问错误的机器。软件阻止将同一个公共网站入口重复分配给两个服务端。新版服务端注册网站路由时同时校验数据库里的所属账号、名称、域名与路径，停止或未授权的路由不允许注册。若必须共用一个后缀，需要额外的集中网关按完整主机名路由到对应 FRPS，此次快捷功能不自动生成跨机器网关配置。

服务端端口授权继续生效：网站类型使用该服务端的 `vhostHTTPPort`，若某语系仅获授权 `62000-63999`，入口 `9080` 不在范围内就不能创建网站。可让该服务端的网站入口使用范围内一个未占用端口并同步 Nginx，或按实际需求调整授权；系统不会绕过端口权限。

## 自动应用和地址保留

保存网站后沿用已验证的客户端配置下发与定时同步流程。在线节点自动应用，离线节点在重新上线后同步。创建成功说明配置已保存，不代表 DNS、TLS 或目标网站一定可访问。

同服务端的域名在数据库事务内检查占用，包括自定义域名、大小写变体、通配域名和路径路由。停用不会释放地址，删除隧道后才释放。已有网站时不能直接关闭服务端入口或修改后缀，需要先删除或转换这些网站，避免原地址静默失效。

## 检查

```bash
# 使用真实生成的域名；无密码应直接返回网站，有密码应先返回 401。
curl -I https://qwen-ig01.frp.xmray.de

# 在 FRPS / Nginx 宿主机本地检查 Host 路由，绕过外部 DNS/TLS。
curl -I -H 'Host: qwen-ig01.frp.xmray.de' http://127.0.0.1:9080/
```

公网有错误而本地正常时检查 DNS、证书和 Nginx；本地 404 检查域名、节点在线与隧道应用情况；502 检查客户端本地网站是否监听正确地址、端口。

参考：[FRP 子域名](https://gofrp.org/en/docs/features/http-https/subdomain/)、[Nginx Host 转发](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_set_header)、[Nginx WebSocket](https://nginx.org/en/docs/http/websocket.html)、[Let's Encrypt DNS-01](https://letsencrypt.org/docs/challenge-types/#dns-01-challenge)。
