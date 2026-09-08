# frp-manager Client UI

Cross-platform client manager for frp-manager.

## Targets

- Windows: Electron desktop app, packaged as `exe`.
- macOS: Electron desktop app, packaged as `dmg` or `zip`.
- Android ARM64: Capacitor app with a native foreground service and bundled Go client core.

## Development

Use Node.js 24.x.

```bash
pnpm install
pnpm dev:desktop
```

Use `FRP_MANAGER_CLIENT_BIN` to point the desktop shell at a local client binary:

```powershell
$env:FRP_MANAGER_CLIENT_BIN = "C:\Program Files\frp-manager\frpp.exe"
pnpm dev:desktop
```

For packaged Windows builds, the client binary is built from `../cmd/frppc` into `resources/binaries/frpp.exe` automatically.

## Build

```bash
pnpm build:win
pnpm build:mac
pnpm cap:add:android
pnpm build:apk
```

Android requires a local Android SDK/Gradle toolchain. `pnpm build:apk` cross-compiles the ARM64 Go core, packages it
as `libfrpp.so`, and connects the UI through the native `FrpManager` Capacitor plugin. Android stores the connection
environment in app-private storage and runs the core through a persistent foreground service.

The release workflow signs and notarizes macOS packages when Apple credentials are available. Otherwise it applies an
ad-hoc signature and publishes artifacts marked `adhoc`; users can approve those packages from **System Settings →
Privacy & Security** after checking the Release SHA256.

On macOS, **Install / Apply** uses the native administrator authorization dialog. It copies the daemon core to
`/usr/local/libexec/frp-manager/frpp` and stores its root-only environment at `/etc/frpp/.env` instead of placing the
client Secret in LaunchDaemon arguments. The desktop application itself must not be run with `sudo`.

On Windows, **Install / Apply** requests UAC elevation, copies the core to `%ProgramFiles%\frp-manager`, and applies
an administrator/System-only ACL to its `.env` file. Service commands always use that installed copy. The action
waits for manager registration and returns the tail of `%ProgramFiles%\frp-manager\service.log` when startup fails.

The macOS app bundle is never modified in place because that would invalidate its signature. The standalone daemon
installed by the GUI follows the desktop package version; update the GUI by installing a newer DMG/ZIP.
