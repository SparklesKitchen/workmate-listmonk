# XXL runtime

This deployment is deliberately one application container. It connects to the
private `listmonk` schema in managed Supabase; Docker does not run a second
Postgres database for customer data.

The host-only listener is `127.0.0.1:9100`. Caddy or Reach may expose a
customer route only after WorkMate SSO and scope enforcement are present.

## Self-hosted Postgres (2026-09-12)

The database is the self-hosted Supabase Postgres on the same host, bound to
`127.0.0.1:5432`. The container therefore runs with `network_mode: host`
(`docker-compose.override.yml`, applied automatically by compose) and listens
on `LISTMONK_app__address=127.0.0.1:9100` for Caddy. The `listmonk_app` role
needs `USAGE` and `CREATE` on schema `extensions` (pgcrypto's `gen_salt`),
matching the managed project's grants.

## Self-hosted Postgres (2026-09-12)

The database is the self-hosted Supabase Postgres on the same host, bound to
`127.0.0.1:5432`, so the container runs with `network_mode: host` and listens
on `LISTMONK_app__address=127.0.0.1:9100` for Caddy. The `listmonk_app` role
needs `USAGE` and `CREATE` on schema `extensions` (pgcrypto `gen_salt`),
matching the managed project's grants.
