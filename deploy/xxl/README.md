# XXL runtime

This deployment is deliberately one application container. It connects to the
private `listmonk` schema in managed Supabase; Docker does not run a second
Postgres database for customer data.

The host-only listener is `127.0.0.1:9100`. Caddy or Reach may expose a
customer route only after WorkMate SSO and scope enforcement are present.
