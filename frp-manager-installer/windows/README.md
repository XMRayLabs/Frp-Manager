# frp-manager Windows Client Installer

This directory is intentionally outside the GitHub project. It is for local installer packaging only.

## Files

- `frp-manager-client.iss`: Inno Setup script.
- `frp-manager-client-manager.ps1`: local GUI manager launched after installation.
- `download-client.ps1`: downloads the versioned Windows client to `files\frpp.exe` (defaults to `1.0.0`).
- `build-installer.ps1`: compiles the installer with Inno Setup.
- `files\frpp.exe`: bundled frp-manager client binary.

## Build

Run in PowerShell:

```powershell
cd C:\Software\Workspace\Server\frp.xmray.de\frp-manager-installer\windows
.\download-client.ps1
.\build-installer.ps1
```

The installer output will be generated in:

```text
C:\Software\Workspace\Server\frp.xmray.de\frp-manager-installer\windows\output
```

## Installer behavior

- Installs to `%ProgramFiles%\frp-manager\frpp.exe`.
- Before installation, stops and uninstalls legacy `C:\frpp\frpp.exe`, then deletes `C:\frpp`.
- Also stops and uninstalls an existing `%ProgramFiles%\frp-manager\frpp.exe` service before upgrading.
- After installation, opens `frp-manager Client Manager`.

## Manager behavior

- Prompts for the full client start command copied from frp-manager.
- Parses `-s`, `-i`, `--api-url`, and `--rpc-url` automatically.
- Example:

```text
frp-manager client -s ca9f9bc1-8696-4a6d-8955-556cbc6f91ad -i admin.c.01 --api-url http://frp.xmray.de:9000 --rpc-url grpc://frp.xmray.de:9001
```

- Runs:

```text
frpp.exe install client -s <secret> -i <clientId> --api-url <apiUrl> --rpc-url <rpcUrl>
frpp.exe start
```

- Shows service status.
- Supports start, stop, restart, uninstall, delete config, and edit/apply config.
