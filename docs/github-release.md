# GitHub 自动发布

仓库通过 `.github/workflows/master.workflow.yml` 验证默认分支，通过 `.github/workflows/tag.workflow.yml` 发布版本。

推送 `v1.2.3` 或 `1.2.3` 格式的标签后会生成：

- Windows、Linux、macOS、Android 的完整核心与精简客户端二进制
- Windows Electron 安装包
- macOS DMG/ZIP
- Android APK
- SHA256 校验文件
- GitHub Release
- Docker Hub `sakurame1/frp-manager` 标准版与 workerd 多架构镜像

## GitHub Actions Secrets

在 **Settings → Secrets and variables → Actions** 配置：

| Secret | 用途 |
| --- | --- |
| `DOCKERHUB_USERNAME` | Docker Hub 用户名 |
| `DOCKERHUB_TOKEN` | 对 `sakurame1/frp-manager` 有写权限的 Docker Hub Access Token |

Android 正式签名可选配置 `ANDROID_KEYSTORE_BASE64`、`ANDROID_KEYSTORE_PASSWORD`、`ANDROID_KEY_ALIAS`、`ANDROID_KEY_PASSWORD`。缺失时发布可安装的 debug 签名预览 APK。

macOS 签名与公证可选配置 `MAC_CSC_LINK`、`MAC_CSC_KEY_PASSWORD`、`APPLE_ID`、`APPLE_APP_SPECIFIC_PASSWORD`、`APPLE_TEAM_ID`。缺失时发布 ad-hoc 签名包。

## 发布

```bash
git tag v1.2.3
git push origin v1.2.3
```

稳定版本会推送 Docker 标签 `1.2.3`、`latest`、`1.2.3-workerd`、`latest-workerd`。预发布版本不会覆盖 `latest`。