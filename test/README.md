# Ward e2e / integration harness

`e2e.compose.yml` stands up the real edge — the custom **Caddy + Coraza** image (`../caddy`) — plus a
`traefik/whoami` upstream and the **Ward** control plane (`../backend`), all on one Docker network.
It's how the control-plane ↔ data-plane flow is verified against real components: config apply, WAF
detect → exclude, IP block, TLS.

## Run

```bash
docker compose -p wardtest -f test/e2e.compose.yml up --build -d

# Drive Ward's API from *inside* the network (host↔container port forwarding is flaky in this
# Docker setup), e.g.:
docker run --rm --network wardtest_default curlimages/curl -s -X POST http://ward:8080/auth/setup \
  -H 'Content-Type: application/json' -d '{"username":"owner","password":"supersecret1"}'
# ... create services, curl through http://caddy:80, inspect /waf-events, etc.

docker compose -p wardtest -f test/e2e.compose.yml down -v
```

## Notes

- Ward reaches Caddy's admin at `http://caddy:2019`; Coraza writes its audit log to a shared volume
  that Ward tails (`/shared/audit.log`).
- `bootstrap.json` is the minimal config Caddy boots with before Ward pushes the generated config.
- `WARD_AUTO_HTTPS=1 docker compose ... up` enables auto-TLS (Caddy internal CA) for TLS testing;
  hit HTTPS via `curl -k --connect-to <host>:443:caddy:443 https://<host>/`.
- Ward runs as root here only so it can read Caddy's root-written audit log; a real deployment aligns
  users/perms instead.
