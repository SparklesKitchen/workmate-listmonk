# WorkMate Listmonk

- This is WorkMate's maintained fork of upstream `knadh/listmonk`; preserve upstream licence notices and keep changes narrowly scoped.
- Deploy one pinned Listmonk application and one Postgres database on XXL. Never create a Listmonk application, database, container, or network per tenant or workspace.
- Every customer-facing operation must be constrained by the authenticated WorkMate tenant and workspace. Native list roles protect lists and subscribers; extend upstream enforcement where templates, campaigns, media, sender settings, or analytics are otherwise global.
- WorkMate Reach is the authenticated product door. Use OIDC SSO rather than sharing Listmonk administrator credentials with the browser.
- WorkMate appearance belongs in Listmonk's supported custom CSS/JS configuration and the minimal maintained fork surface, not an HTML-rewriting proxy.
- Do not ship until XXL verifies two distinct workspace users cannot read, mutate, or send using each other's Listmonk data.
