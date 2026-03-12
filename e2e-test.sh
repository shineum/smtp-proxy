#!/usr/bin/env bash
# E2E Integration Test for smtp-proxy
# Tests: clean build, API data setup, SMTP send (simple + complex)
set -euo pipefail

API="http://localhost:8080/api/v1"
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }
info() { echo -e "${CYAN}[INFO]${NC} $1"; }

# JSON field extractor using python3
jv() { python3 -c "import sys,json; print(json.load(sys.stdin).get('$1',''))" <<< "$2"; }

# =============================================================================
# Step 1: Clean Build
# =============================================================================
info "Step 1: Clean build"
docker compose down -v --remove-orphans 2>/dev/null || true
docker compose up -d --build

info "Waiting for API server health..."
for i in $(seq 1 60); do
  if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
    break
  fi
  if [ "$i" -eq 60 ]; then
    fail "API server did not become healthy in 60s"
  fi
  sleep 1
done
pass "All services up and healthy"

# =============================================================================
# Step 2: Test Data Setup via API
# =============================================================================
info "Step 2: Setting up test data via API"

# 2.0 Login as system admin
info "2.0 Admin login"
LOGIN_RESP=$(curl -s -X POST "$API/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@localhost","password":"admin"}')
TOKEN=$(jv access_token "$LOGIN_RESP")
[ -n "$TOKEN" ] || fail "Admin login failed: $LOGIN_RESP"
pass "Admin login OK"

AUTH="Authorization: Bearer $TOKEN"

# 2.1 Add test provider 1 - stdout, private (default visibility)
info "2.1 Create Provider 1: stdout private"
P1_RESP=$(curl -s -X POST "$API/providers" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"test-stdout-private","provider_type":"stdout","enabled":true,"visibility":"private"}')
P1_ID=$(jv id "$P1_RESP")
[ -n "$P1_ID" ] || fail "Provider 1 creation failed: $P1_RESP"
pass "Provider 1 created: $P1_ID (stdout, private)"

# 2.2 Add test provider 2 - stdout, global (public)
info "2.2 Create Provider 2: stdout global"
P2_RESP=$(curl -s -X POST "$API/providers" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"test-stdout-global","provider_type":"stdout","enabled":true,"visibility":"global"}')
P2_ID=$(jv id "$P2_RESP")
[ -n "$P2_ID" ] || fail "Provider 2 creation failed: $P2_RESP"
pass "Provider 2 created: $P2_ID (stdout, global)"

# 2.3 Add test group
info "2.3 Create test group"
GRP_RESP=$(curl -s -X POST "$API/groups" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"e2e-test-group","monthly_limit":1000,"display_name":"E2E Test Group"}')
GRP_ID=$(jv id "$GRP_RESP")
GRP_KEY=$(jv group_key "$GRP_RESP")
[ -n "$GRP_ID" ] || fail "Group creation failed: $GRP_RESP"
pass "Group created: $GRP_ID (group_key: $GRP_KEY)"

# Switch to test group context for provider 3
info "Switching to test group context"
SW_RESP=$(curl -s -X POST "$API/auth/switch-group" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"group_id\":\"$GRP_ID\"}")
GRP_TOKEN=$(jv access_token "$SW_RESP")
[ -n "$GRP_TOKEN" ] || fail "Group switch failed: $SW_RESP"
GRP_AUTH="Authorization: Bearer $GRP_TOKEN"
pass "Switched to test group"

# 2.4 Add test provider 3 - stdout, private (for test group only)
info "2.4 Create Provider 3: stdout private (test group)"
P3_RESP=$(curl -s -X POST "$API/providers" \
  -H "$GRP_AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"test-group-provider","provider_type":"stdout","enabled":true,"visibility":"private"}')
P3_ID=$(jv id "$P3_RESP")
[ -n "$P3_ID" ] || fail "Provider 3 creation failed: $P3_RESP"
pass "Provider 3 created: $P3_ID (stdout, private, test-group)"

# 2.5 Add test human user on test group
info "2.5 Create test human user"
HU_RESP=$(curl -s -X POST "$API/users" \
  -H "$GRP_AUTH" -H 'Content-Type: application/json' \
  -d "{\"email\":\"testuser@example.com\",\"password\":\"testpass123\",\"account_type\":\"user\",\"group_id\":\"$GRP_ID\"}")
HU_ID=$(jv id "$HU_RESP")
[ -n "$HU_ID" ] || fail "Human user creation failed: $HU_RESP"
pass "Human user created: $HU_ID"

# 2.6 Add test service account on test group
info "2.6 Create test service account"
SA_RESP=$(curl -s -X POST "$API/groups/$GRP_ID/service-accounts" \
  -H "$GRP_AUTH" -H 'Content-Type: application/json' \
  -d "{\"username\":\"e2e-smtp\",\"provider_id\":\"$P3_ID\"}")
SA_ID=$(jv id "$SA_RESP")
SA_KEY=$(jv api_key "$SA_RESP")
[ -n "$SA_KEY" ] || fail "Service account creation failed: $SA_RESP"
pass "Service account created: $SA_ID (username: e2e-smtp, api_key: ${SA_KEY:0:12}...)"

echo ""
info "SMTP login will be: e2e-smtp@$GRP_KEY"
echo ""

# =============================================================================
# Step 3: SMTP Client Tests
# =============================================================================
info "Step 3: SMTP client tests"

# Wait briefly for SMTP server to be fully ready
sleep 2

# 3.1 Simple case: from, to, subject, body
info "3.1 Simple email (from, to, subject, body)"
docker compose run --rm -T test-client \
  --host=smtp-server --port=587 --tls=starttls --insecure \
  --user="e2e-smtp@$GRP_KEY" --password="$SA_KEY" \
  --from="sender@example.com" \
  --to="recipient@example.com" \
  --subject="E2E Simple Test" \
  --body="Hello from E2E test - simple case."
pass "Simple email sent successfully"

# 3.2 Complex case: from, to, cc, bcc, attachment, subject, body
info "3.2 Complex email (from, to, cc, bcc, attachment, subject, body)"

# Create test attachment files
mkdir -p test-data
echo "This is a test attachment file for E2E testing." > test-data/sample.txt
echo "<html><body><h1>HTML Attachment</h1></body></html>" > test-data/sample.html

docker compose run --rm -T test-client \
  --host=smtp-server --port=587 --tls=starttls --insecure \
  --user="e2e-smtp@$GRP_KEY" --password="$SA_KEY" \
  --from="sender@example.com" \
  --to="recipient@example.com" \
  --cc="cc-user@example.com" \
  --bcc="bcc-user@example.com" \
  --subject="E2E Complex Test" \
  --body="Hello from E2E test - complex case with CC, BCC, and attachments." \
  --html="<h1>E2E Complex Test</h1><p>With HTML body, CC, BCC, and attachments.</p>" \
  --attach=/test-data/sample.txt \
  --attach=/test-data/sample.html
pass "Complex email sent successfully"

# =============================================================================
# Step 4: Check Results
# =============================================================================
info "Step 4: Checking delivery results"

sleep 2

echo ""
info "SMTP server logs (delivery-related):"
echo "---"
docker compose logs --tail=50 smtp-server 2>&1 | grep -iE "(deliver|stdout|sent|accepted|DATA|message)" || echo "(no matching logs)"
echo "---"

echo ""
info "Queue worker logs (delivery-related):"
echo "---"
docker compose logs --tail=50 queue-worker 2>&1 | grep -iE "(deliver|stdout|sent|process|message)" || echo "(no matching logs)"
echo "---"

echo ""
echo -e "${GREEN}=============================================${NC}"
echo -e "${GREEN}  E2E Test Complete - All steps passed!${NC}"
echo -e "${GREEN}=============================================${NC}"
echo ""
echo "Summary:"
echo "  - Provider 1: stdout private (system group)"
echo "  - Provider 2: stdout global"
echo "  - Provider 3: stdout private (test group)"
echo "  - Group: e2e-test-group ($GRP_ID)"
echo "  - Human user: testuser@example.com"
echo "  - Service account: e2e-smtp@$GRP_KEY"
echo "  - Simple email: sent OK"
echo "  - Complex email (cc, bcc, html, attachments): sent OK"
