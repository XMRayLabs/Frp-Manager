# Configuration Reference

## Advanced frp Tunnel Configuration

This panel fully supports frp鈥檚 original JSON configuration format. Simply paste your configuration JSON into the Server/Client **Advanced Mode** editor and save. For detailed usage, see the [frp documentation](https://gofrp.org/zh-cn/docs/features/common/configure/).

## Startup Configuration Files

The application loads configuration in the following order:

1. `.env` in the working directory  
2. `/etc/frpp/.env`  

## Environment Variable Reference

> Documentation may be somewhat outdated.
>  
> For the complete and latest configuration reference, see: [settings.go](https://github.com/Sakurame1/frp-manager/blob/main/conf/settings.go)

| Type   | Environment Variable                    | Default             | Description                                                                                                    |
|:-------|:---------------------------------------|:--------------------|:---------------------------------------------------------------------------------------------------------------|
| string | `APP_SECRET`                           | 鈥?                  | Application secret used to encrypt communication between Client, Server, and Master                             |
| string | `APP_GLOBAL_SECRET`                    | empty, runtime-generated | Global secret used to generate keys. Set a long random value in production so sessions survive restarts.       |
| int    | `APP_COOKIE_AGE`                       | `86400`             | Cookie lifetime in seconds (default: 1 day)                                                                    |
| string | `APP_COOKIE_NAME`                      | `frp-manager-cookie`  | Cookie name                                                                                                    |
| string | `APP_COOKIE_PATH`                      | `/`                 | Cookie path                                                                                                    |
| string | `APP_COOKIE_DOMAIN`                    | 鈥?                  | Cookie domain                                                                                                  |
| bool   | `APP_COOKIE_SECURE`                    | `false`             | Whether the cookie is marked Secure                                                                            |
| bool   | `APP_COOKIE_HTTP_ONLY`                 | `true`              | Whether the cookie is HTTP-only                                                                                |
| bool   | `APP_ENABLE_REGISTER`                  | `false`             | Enable user registration explicitly. Disable it immediately after controlled account creation.                  |
| bool   | `APP_AUTO_UPDATE`                      | `true`              | Check, verify, and install Client/Server binary updates on every startup.                                       |
| string | `LOGGER_FILE`                         | -                   | Optional log file path; GUI-managed services use `service.log`.                                                |
| int    | `MASTER_API_PORT`                      | `9000`              | Master API port                                                                                                |
| string | `MASTER_API_HOST`                      | 鈥?                  | Master API host (can be behind a reverse proxy or CDN)                                                         |
| string | `MASTER_API_SCHEME`                    | `http`              | Master API scheme (for client command generation; HTTPS must be handled via reverse proxy)                     |
| int    | `MASTER_CACHE_SIZE`                    | `10`                | Cache size in MB                                                                                                |
| string | `MASTER_RPC_HOST`                      | `127.0.0.1`         | Master RPC host or public IP                                                                                    |
| int    | `MASTER_RPC_PORT`                      | `9001`              | Master RPC port                                                                                                |
| bool   | `MASTER_COMPATIBLE_MODE`               | `false`             | Compatibility mode for official frp clients                                                                    |
| string | `MASTER_INTERNAL_FRP_SERVER_HOST`      | 鈥?                  | Host for Master鈥檚 built-in frps instance (for client connections)                                              |
| int    | `MASTER_INTERNAL_FRP_SERVER_PORT`      | `9002`              | Port for Master鈥檚 built-in frps instance (for client connections)                                              |
| string | `MASTER_INTERNAL_FRP_AUTH_SERVER_HOST` | `127.0.0.1`         | Host for Master鈥檚 built-in frps authentication service                                                         |
| int    | `MASTER_INTERNAL_FRP_AUTH_SERVER_PORT` | `8999`              | Port for Master鈥檚 built-in frps authentication service                                                         |
| string | `MASTER_INTERNAL_FRP_AUTH_SERVER_PATH` | `/auth`             | Path for Master鈥檚 built-in frps authentication service                                                         |
| int    | `SERVER_API_PORT`                      | `8999`              | Server API port                                                                                                |
| string | `DB_TYPE`                              | `sqlite3`           | Database type (e.g., `mysql`, `postgres`, `sqlite3`)                                                           |
| string | `DB_DSN`                               | `data.db`           | Database DSN. For `sqlite3`, this is a file path (default in working directory). For other databases, use DSN. |
| string | `CLIENT_ID`                            | 鈥?                  | Client ID                                                                                                      |
| string | `CLIENT_SECRET`                        | 鈥?                  | Client secret                                                                                                  |
| bool   | `IS_DEBUG`                              | `false`             | Enable debug mode (affects logging / some components behavior)                                                |
| bool   | `DEBUG_PROFILER_ENABLED`                | `false`             | Enable profiler (pprof) HTTP server (by default listens on 127.0.0.1 only)                                    |
| int    | `DEBUG_PROFILER_PORT`                   | `6961`              | Profiler (pprof) HTTP port                                                                                    |
