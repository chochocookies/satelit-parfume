#!/usr/bin/env bash
set -euo pipefail

# Restores a Satelit Parfume database backup created by backup-db.sh.
# DESTRUCTIVE: this replaces the target database's contents outright.
# Meant for disaster recovery or standing up a fresh environment from a
# known-good backup — not something to run against a live production
# database without being certain that's actually the intent, which is
# exactly what the confirmation prompt below is for.
#
# Usage:   DATABASE_URL=... ./restore-db.sh <path-to-backup.sql.gz>
# Required env: DATABASE_URL (the database to restore INTO)

DATABASE_URL="${DATABASE_URL:?DATABASE_URL is required}"
BACKUP_FILE="${1:?Usage: $0 <path-to-backup.sql.gz>}"

if [ ! -f "$BACKUP_FILE" ]; then
  echo "[restore] ERROR: ${BACKUP_FILE} not found" >&2
  exit 1
fi

db_name=$(echo "$DATABASE_URL" | sed -E 's#.*/([^/?]+).*#\1#')

echo "[restore] this will overwrite every table in '${db_name}' with the contents of ${BACKUP_FILE}."
read -r -p "[restore] type the database name to confirm: " confirm
if [ "$confirm" != "$db_name" ]; then
  echo "[restore] confirmation did not match '${db_name}' — aborting, nothing was touched." >&2
  exit 1
fi

echo "[restore] restoring ${BACKUP_FILE} into ${db_name}..."
gunzip -c "$BACKUP_FILE" | psql "$DATABASE_URL"
echo "[restore] done."
