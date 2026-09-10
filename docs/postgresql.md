# PostgreSQL 部署与 SQLite 迁移（1.1.0）

PostgreSQL 是多用户生产部署的推荐数据库。镜像仍为 sakurame1/frp-manager，原 frp-manager-data 卷及 /data 挂载不变；数据库另用 frp-manager-postgres 卷。保留基础 Compose 的 SQLite 配置，防止老部署拉取新版后突然接入空数据库。使用 PostgreSQL 时叠加 docker-compose.postgres.yml。

## 新部署

复制 .env.example 为 .env，配置域名、APP_GLOBAL_SECRET、FRP_MANAGER_VERSION=1.1.0，以及 POSTGRES_PASSWORD（使用 openssl rand -hex 32 生成，不要使用示例密码）。

运行：

~~~sh
docker compose -f docker-compose.yml -f docker-compose.postgres.yml up -d
~~~

面板保持 host 网络；PostgreSQL 容器仅映射本机 127.0.0.1:5433，供面板访问，不暴露公网端口。该 Compose 适用于 Linux 主机部署。内部回环连接使用 sslmode=disable；连接远程 PostgreSQL 应使用 sslmode=verify-full 和可信 CA。

## 已有 SQLite 数据（先迁移，再启动新面板）

1. 保持原 Compose 项目名、卷名、/data 挂载、APP_GLOBAL_SECRET 和外部 API/RPC 地址不变。备份现有 Compose、.env 和整个 /data 卷（包括 SQLite 的 WAL/SHM 文件）。如果原来使用自定义绑定目录，请保留原挂载，叠加配置不会修改它。
2. 停止全部面板实例，确认没有其他进程写入此 SQLite 文件。数据库迁移需要维护窗口。
3. 设置 POSTGRES_PASSWORD 和 FRP_MANAGER_VERSION=1.1.0，使用包含 migrate-database 命令的镜像。只启动 PostgreSQL，不要先启动新面板初始化空库。

~~~sh
docker compose stop frp-manager
docker compose -f docker-compose.yml -f docker-compose.postgres.yml up -d postgres
docker compose -f docker-compose.yml -f docker-compose.postgres.yml run --rm --no-deps frp-manager migrate-database --source /data/data.db --temp-dir /data --offline
docker compose -f docker-compose.yml -f docker-compose.postgres.yml up -d frp-manager
~~~

请在迁移成功退出后才执行最后一条命令。迁移使用 --temp-dir /data 将临时快照写入已有数据卷，避免 /tmp 的 128MB 内存限制；请确保该卷有至少一个完整数据库副本的额外空间，成功或失败后快照都会清理。

迁移工具会：

- 用 SQLite VACUUM INTO 生成一致性副本，在副本上执行 1.1.0 升级，不修改原 SQLite 的业务记录。
- 要求目标 PostgreSQL schema 没有任何业务表；已有表就拒绝，不清空、不覆盖。
- 分批复制全部已知表，包括软删除记录、账号密码哈希、设备密钥、证书、权限关联、语系、流量和审计历史。
- 在同一个 PostgreSQL 事务中创建表、导入数据、核对行数、验证外键并修复自增编号。未知表或字段、无效数据、外键错误均中止并回滚，避免静默丢数据。
- 输出每表验证行数，成功后保留原 SQLite。后续重启不会再次导入。

迁移后检查登录、用户与语系归属、设备上线、隧道、证书和历史数据。由于密码、设备密钥及 APP_GLOBAL_SECRET 保留，已有客户端无需重新安装。

## 回退与备份

迁移失败时原 SQLite 可继续使用。停掉 PostgreSQL 版面板，使用原配置和原版本镜像启动 SQLite 即可。切换后新写入的数据仅在 PostgreSQL 中，回退旧 SQLite 不会自动带回这些变更。

PostgreSQL 上线后需定期 pg_dump，备份范围包括数据库、.env 和原 /data 目录。不要只备份旧 data.db；不要执行 docker compose down -v。PostgreSQL 大版本升级需单独迁移，不能直接复用旧卷修改主版本。

## 独立 PostgreSQL

设置 DB_TYPE=postgres 和 DB_DSN（libpq 格式或 PostgreSQL URL）；使用独立数据库及拥有该库 schema 建表权限的账号。不要使用默认的 /data/data.db 作为 DSN。程序没有将所有旧安装强制改成 PostgreSQL。

连接池默认 DB_MAX_OPEN_CONNS=20、DB_MAX_IDLE_CONNS=5、DB_CONN_MAX_LIFETIME_SECONDS=1800。多实例的连接总数需为 PostgreSQL 运维和其他业务保留余量。

## 自动验证

设置 FRP_TEST_POSTGRES_DSN 后运行 go test ./models ./services/dao ./cmd/frpp/shared。测试创建随机独立 schema，结束后清理，不能提供生产凭据。GitHub Actions 使用临时 PostgreSQL 17 服务执行相同测试。

参考：[PostgreSQL 官方镜像](https://hub.docker.com/_/postgres)、[PostgreSQL 并发控制](https://www.postgresql.org/docs/17/mvcc.html)。
