#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# sync-db-from-prod.sh
# Copy the production database from :3010 into the :3011 test environment.
# Both stacks run on the same host (homelab), so this is a local operation.
#
# Prerequisites:
#   - Run on the host where both :3010 and :3011 docker stacks are running
#   - :3011 test stack running (docker-compose.test.yml)
#
# Usage:
#   ./sync-db-from-prod.sh
# ============================================================================

PROD_PG_CONTAINER="newapi-postgres-1"
PROD_DB_USER="newapi"
PROD_DB_NAME="newapi"

TEST_PG_CONTAINER="new-api-test-pg"
TEST_DB_USER="newapi"
TEST_DB_NAME="newapi"

DUMP_FILE="/tmp/new-api-prod-dump.sql"

echo "[1/5] Dumping production database..."
docker exec "${PROD_PG_CONTAINER}" pg_dump -U "${PROD_DB_USER}" -d "${PROD_DB_NAME}" --no-owner --no-acl --clean --if-exists > "${DUMP_FILE}"
echo "      Dump size: $(du -sh "${DUMP_FILE}" | cut -f1)"

echo "[2/5] Copying dump to test container..."
docker cp "${DUMP_FILE}" "${TEST_PG_CONTAINER}:/tmp/prod-dump.sql"

echo "[3/5] Restoring into test database..."
docker exec "${TEST_PG_CONTAINER}" psql -U "${TEST_DB_USER}" -d "${TEST_DB_NAME}" -f /tmp/prod-dump.sql > /tmp/restore.log 2>&1 || true
WARN_COUNT=$(grep -c "warning" /tmp/restore.log 2>/dev/null || echo 0)
ERR_COUNT=$(grep -ci "error" /tmp/restore.log 2>/dev/null || echo 0)
echo "      Restore complete (warnings: ${WARN_COUNT}, errors in log: ${ERR_COUNT})"

echo "[4/5] Cleaning up temp files..."
rm -f "${DUMP_FILE}"
docker exec "${TEST_PG_CONTAINER}" rm -f /tmp/prod-dump.sql

echo "[5/5] Verifying table count..."
TABLE_COUNT=$(docker exec "${TEST_PG_CONTAINER}" psql -U "${TEST_DB_USER}" -d "${TEST_DB_NAME}" -t -c "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';")
echo "      Test database tables: ${TABLE_COUNT}"

echo ""
echo "✓ Database sync complete. Test environment (:3011) is ready."
