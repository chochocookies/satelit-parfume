# Production hardening (Phase 15)

What's actually running by the time this phase is done: HTTPS with a
real, auto-renewing certificate; nginx rate-limiting and gzip in front
of both apps; scheduled, tested database backups; and logs/health
checks a real monitoring setup can act on. Four pieces — SSL, backups,
monitoring, and the nginx config that ties the first two together —
covered in the order you'd actually set them up in.

## 1. Point a domain here first

Everything below assumes `DOMAIN` (in `.env`) is a real domain whose
DNS `A` record already points at this server's public IP, and that
ports 80/443 are reachable from the internet. Certbot's HTTP-01
challenge — the one this setup uses — fails outright without that; it's
not something to defer until after the certificate step.

## 2. First certificate

Nginx's HTTPS server block (`docker/nginx/nginx.conf`) won't start
without a certificate already sitting at
`/etc/letsencrypt/live/$DOMAIN/`. Get that first, before nginx's HTTPS
side is ever asked to start:

```bash
# Start everything except nginx first, so certbot's webroot path has
# somewhere to actually be served from — plain HTTP still works fine at
# this point, since the ordering here is nginx-with-HTTPS-disabled,
# then a certificate, then nginx for real.
docker compose up -d postgres redis api web

docker compose run --rm --entrypoint "\
  certbot certonly --webroot -w /var/www/certbot \
    -d $DOMAIN \
    --email you@example.com --agree-tos --no-eff-email" certbot

docker compose up -d nginx certbot
```

Replace `you@example.com` with a real address — Let's Encrypt uses it
only for expiry warnings if renewal ever starts failing, not for
anything else.

**Renewal is already handled**: the `certbot` service's own container
command (see `docker-compose.yml`) checks every 12 hours and renews
automatically inside the certificate's last 30 days of validity — the
command above is only ever needed once, for the very first certificate,
or again later if you add a second domain.

## 3. Backups

```bash
# One-off, run manually:
DATABASE_URL="postgres://user:pass@localhost:5432/satelit_parfume" \
  ./scripts/backup-db.sh

# Scheduled — add to crontab (crontab -e) on the host running postgres,
# NOT inside a container that gets recreated on every deploy:
0 2 * * * DATABASE_URL="postgres://..." BACKUP_DIR=/var/backups/satelit-parfume /path/to/scripts/backup-db.sh >> /var/log/satelit-backup.log 2>&1
```

`BACKUP_S3_BUCKET` (optional) uploads the same dump off-site via the
AWS CLI, on top of the local, rotated copy `backup-db.sh` always keeps
regardless. Off-site storage matters here specifically: a backup living
on the same disk as the database it's backing up survives a bad
migration or an accidental `DROP TABLE`, but not a lost or failed
server.

**A backup nobody has restored is a hypothesis, not a backup.** Run
`restore-db.sh` against a scratch database at least once before trusting
this:

```bash
DATABASE_URL="postgres://user:pass@localhost:5432/scratch_db" \
  ./scripts/restore-db.sh /var/backups/satelit-parfume/satelit-parfume-<timestamp>.sql.gz
```

Both scripts were tested end-to-end while building this (backup → drop
table → restore → verify data matches) before being written up here —
not just written and assumed correct.

## 4. Monitoring

- **`GET /api/v1/health`** already checks the API process plus both
  hard dependencies (Postgres, Redis) individually — point any uptime
  monitor (a paid service, or something as simple as a cron job that
  curls it and alerts on a non-200) at this rather than just pinging
  `/`, which would report healthy even with the database down.
- **Logs are now structured JSON** (`pkg/logger`, rebuilt on the
  standard library's `log/slog` this phase) — one object per line, with
  a real `level` field. A request line that hits `status >= 500` logs
  at `error` and one that hits `>= 400` logs at `warn`, so a log
  aggregator (Loki, CloudWatch, ELK, whatever's actually in use) can
  alert on "any error-level line", not on grepping for a status-code
  substring.
- **Log rotation** — Docker's own `json-file` log driver doesn't rotate
  by itself. Already configured on both `api` and `web` in
  `docker-compose.yml` (`max-size: 10m`, `max-file: 3` — roughly 30MB
  of logs kept per service before the oldest rolls off), so there's
  nothing extra to do here unless those defaults need adjusting for
  actual traffic volume.

## Pre-launch checklist

- [ ] `DOMAIN`'s DNS `A` record points at this server
- [ ] First certificate obtained (step 2 above); `docker compose logs
      certbot` shows the renewal loop running, not erroring
- [ ] `nginx -t` (or `docker compose exec nginx nginx -t`) reports the
      config as valid after any change to `docker/nginx/nginx.conf`
- [ ] Backup cron entry installed on the actual database host, not
      inside a container
- [ ] A restore has actually been run once, against a scratch
      database, and the data checked
- [ ] `DATABASE_URL`, `DUITKU_MERCHANT_CODE`/`KEY`, and every JWT
      secret in `.env` are real production values — see `.env.example`'s
      own comments for which ones default to obviously-fake local-dev
      values that must not ship as-is
- [ ] Uptime monitoring points at `/api/v1/health`, not `/`
