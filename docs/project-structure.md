# Project structure

| Path | Responsibility |
| --- | --- |
| `cmd/frpp` | Full master/server/client command and embedded Web UI |
| `cmd/frppc` | Reduced client-only binary used by desktop packages |
| `biz` | HTTP/RPC use cases and orchestration |
| `services` | Database, RPC, tunnel, Worker, and WireGuard services |
| `models` | Persistent entities and migrations |
| `middleware` | Authentication, authorization, rate limiting, and browser security |
| `conf`, `defs`, `common`, `utils` | Configuration and shared infrastructure |
| `idl`, `pb` | Protobuf source and generated Go bindings |
| `www` | Next.js management panel |
| `client-ui` | Electron and Capacitor client manager |
| `docs` | VitePress operator documentation |
| `deploy` | Docker Compose, environment, and reverse-proxy templates |
| `.github/workflows` | Validation, release, Docker Hub, docs, and workerd automation |
| `packaging` | Platform packaging metadata |

Generated directories such as `dist`, `tmp`, `www/out`, and `client-ui/release` are build output and are not source
ownership boundaries. `cmd/frpp/out` is intentionally committed because the Go full binary embeds that static export.
