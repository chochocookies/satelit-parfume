#!/usr/bin/env bash
set -euo pipefail

# Backs up the Satelit Parfume database — meant to run on a schedule
# (cron or a systemd timer) on the actual database host, not inside a
# container that gets recreated on every deploy: a backup that only
# ever lives inside a disposable container isn't a real backup.
#
# Required env:
#   DATABASE_URL   — same connection string the API itself uses (see
#                    .env.example)
# Optional env:
#   BACKUP_DIR              — where dumps are written (default: ./backups)
#   BACKUP_RETENTION_DAYS   — how long to keep local dumps (default: 14)
#   BACKUP_S3_BUCKET        — if set, also uploads via `aws s3 cp`
#                             (requires the AWS CLI + credentials already
#                             configured). Left unset, off-site upload is
#                             just skipped — this script's job is to make
#                             a good local dump either way, and off-site
#                             storage is a separate, optional layer on
#                             top of that, not a hard requirement to run
#                             this at all.

DATABASE_URL="${DATABASE_URL:?DATABASE_URL is required}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-14}"

mkdir -p "$BACKUP_DIR"

timestamp=$(date -u +%Y%m%d-%H%M%S)
filename="satelit-parfume-${timestamp}.sql.gz"
filepath="${BACKUP_DIR}/${filename}"

echo "[backup] dumping database to ${filepath}..."
pg_dump "$DATABASE_URL" --no-owner --no-privileges | gzip > "$filepath"

# An empty or truncated file passing silently as "done" is worse than
# this job failing loudly — better to find out now than during an
# actual restore, when it's too late to just run it again.
if [ ! -s "$filepath" ]; then
  echo "[backup] ERROR: ${filepath} is empty — dump likely failed" >&2
  rm -f "$filepath"
  exit 1
fi

echo "[backup] wrote $(du -h "$filepath" | cut -f1) to ${filepath}"

if [ -n "${BACKUP_S3_BUCKET:-}" ]; then
  echo "[backup] uploading to s3://${BACKUP_S3_BUCKET}/${filename}..."
  aws s3 cp "$filepath" "s3://${BACKUP_S3_BUCKET}/${filename}"
fi

echo "[backup] pruning local backups older than ${RETENTION_DAYS} days..."
find "$BACKUP_DIR" -name "satelit-parfume-*.sql.gz" -mtime "+${RETENTION_DAYS}" -delete

echo "[backup] done."
