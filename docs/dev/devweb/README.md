# Caddy and web

Artifacts, not a working setup. Kept for reference

## How it ran

An additional compose profile:

```sh
docker compose -f docker-compose.yml -f dev/devweb/docker-compose.web.yml up -d --build
```

A `caddy:2-alpine` container publishes `127.0.0.1:${UI_PORT:-8099}` and mounts `Caddyfile` and `web/` as a root.
The browser goes to Caddy on 8099.

`Caddyfile` splits the traffic by path: `/api/v1/*` is reverse-proxied to `app` service,
everything else is served from disk `/srv`.

That single origin is why `web/app.js` uses a relative `const API = "/api/v1"` and no CORS logic was needed in web or app
