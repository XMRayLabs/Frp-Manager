# Deployment templates

- `docker-compose.yml`: hardened Linux host-network deployment.
- `.env.example`: required public endpoints and security settings.
- `nginx.conf.example`: HTTPS and WebSocket reverse proxy example.

Quick start:

```bash
cp deploy/.env.example deploy/.env
openssl rand -base64 48
# Put the generated value in deploy/.env, then edit the public host names.
docker compose --env-file deploy/.env -f deploy/docker-compose.yml up -d
```

Set `APP_ENABLE_REGISTER=true` only while creating the first administrator. Return it to `false` immediately and run
the Compose command again. All images and downloads in these templates use Docker Hub, Alpine, and GitHub directly.

## PostgreSQL

生产多用户部署支持 PostgreSQL；升级旧 SQLite 时请先停机迁移，再启动新数据库配置。参见[部署及迁移说明](../docs/postgresql.md)。Docker 镜像和原 /data 挂载不变。
