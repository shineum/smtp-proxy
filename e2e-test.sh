#!/usr/bin/env bash
# E2E Integration Test for smtp-proxy
# Tests: clean build, API data setup, SMTP send, auth negative cases,
#        provider access control, API key reset/expiration
set -euo pipefail

API="http://localhost:8080/api/v1"
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[0;33m'
NC='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0

pass() { echo -e "${GREEN}[PASS]${NC} $1"; PASS_COUNT=$((PASS_COUNT + 1)); }
fail() { echo -e "${RED}[FAIL]${NC} $1"; FAIL_COUNT=$((FAIL_COUNT + 1)); }
fail_exit() { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }
info() { echo -e "${CYAN}[INFO]${NC} $1"; }

# JSON field extractor using python3
jv() { python3 -c "import sys,json; print(json.load(sys.stdin).get('$1',''))" <<< "$2"; }
jv_err() { python3 -c "import sys,json; print(json.load(sys.stdin).get('error',''))" <<< "$1"; }

# SMTP auth attempt helper - returns 0 on success, 1 on failure
smtp_auth_test() {
  local user="$1" pass="$2"
  docker compose run --rm -T test-client \
    --host=smtp-server --port=587 --tls=starttls --insecure \
    --user="$user" --password="$pass" \
    --from="sender@example.com" --to="recipient@example.com" \
    --subject="Auth Test" --body="test" 2>&1
}

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
    fail_exit "API server did not become healthy in 60s"
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
[ -n "$TOKEN" ] || fail_exit "Admin login failed: $LOGIN_RESP"
pass "Admin login OK"

AUTH="Authorization: Bearer $TOKEN"

# 2.1 Provider 1 - stdout, private (system group)
info "2.1 Create Provider 1: stdout private"
P1_RESP=$(curl -s -X POST "$API/providers" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"test-stdout-private","provider_type":"stdout","enabled":true,"visibility":"private"}')
P1_ID=$(jv id "$P1_RESP")
[ -n "$P1_ID" ] || fail_exit "Provider 1 creation failed: $P1_RESP"
pass "Provider 1 created: $P1_ID (stdout, private)"

# 2.2 Provider 2 - stdout, global
info "2.2 Create Provider 2: stdout global"
P2_RESP=$(curl -s -X POST "$API/providers" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"test-stdout-global","provider_type":"stdout","enabled":true,"visibility":"global"}')
P2_ID=$(jv id "$P2_RESP")
[ -n "$P2_ID" ] || fail_exit "Provider 2 creation failed: $P2_RESP"
pass "Provider 2 created: $P2_ID (stdout, global)"

# 2.3 Provider 4 - stdout, shared (for access grant tests)
info "2.3 Create Provider 4: stdout shared"
P4_RESP=$(curl -s -X POST "$API/providers" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"test-stdout-shared","provider_type":"stdout","enabled":true,"visibility":"shared"}')
P4_ID=$(jv id "$P4_RESP")
[ -n "$P4_ID" ] || fail_exit "Provider 4 creation failed: $P4_RESP"
pass "Provider 4 created: $P4_ID (stdout, shared)"

# 2.4 Create test group A
info "2.4 Create test group A"
GRP_RESP=$(curl -s -X POST "$API/groups" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"e2e-test-group-a","monthly_limit":1000,"display_name":"E2E Test Group A"}')
GRP_ID=$(jv id "$GRP_RESP")
[ -n "$GRP_ID" ] || fail_exit "Group A creation failed: $GRP_RESP"
pass "Group A created: $GRP_ID"

# 2.5 Create test group B (for cross-group provider tests)
info "2.5 Create test group B"
GRPB_RESP=$(curl -s -X POST "$API/groups" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"e2e-test-group-b","monthly_limit":500,"display_name":"E2E Test Group B"}')
GRPB_ID=$(jv id "$GRPB_RESP")
[ -n "$GRPB_ID" ] || fail_exit "Group B creation failed: $GRPB_RESP"
pass "Group B created: $GRPB_ID"

# Switch to test group A
info "Switching to test group A context"
SW_RESP=$(curl -s -X POST "$API/auth/switch-group" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"group_id\":\"$GRP_ID\"}")
GRP_TOKEN=$(jv access_token "$SW_RESP")
[ -n "$GRP_TOKEN" ] || fail_exit "Group switch failed: $SW_RESP"
GRP_AUTH="Authorization: Bearer $GRP_TOKEN"
pass "Switched to test group A"

# 2.6 Provider 3 - stdout, private (for test group A)
info "2.6 Create Provider 3: stdout private (group A)"
P3_RESP=$(curl -s -X POST "$API/providers" \
  -H "$GRP_AUTH" -H 'Content-Type: application/json' \
  -d '{"name":"test-group-provider","provider_type":"stdout","enabled":true,"visibility":"private"}')
P3_ID=$(jv id "$P3_RESP")
[ -n "$P3_ID" ] || fail_exit "Provider 3 creation failed: $P3_RESP"
pass "Provider 3 created: $P3_ID (stdout, private, group A)"

# 2.7 Human user on test group A
info "2.7 Create test human user"
HU_RESP=$(curl -s -X POST "$API/users" \
  -H "$GRP_AUTH" -H 'Content-Type: application/json' \
  -d "{\"email\":\"testuser@example.com\",\"password\":\"testpass123\",\"account_type\":\"user\",\"group_id\":\"$GRP_ID\"}")
HU_ID=$(jv id "$HU_RESP")
[ -n "$HU_ID" ] || fail_exit "Human user creation failed: $HU_RESP"
pass "Human user created: $HU_ID"

# 2.8 Service account on test group A
info "2.8 Create test service account"
SA_RESP=$(curl -s -X POST "$API/groups/$GRP_ID/service-accounts" \
  -H "$GRP_AUTH" -H 'Content-Type: application/json' \
  -d "{\"username\":\"e2e-smtp\",\"provider_id\":\"$P3_ID\",\"api_key_expires_in\":\"30d\"}")
SA_ID=$(jv id "$SA_RESP")
SA_KEY=$(jv api_key "$SA_RESP")
[ -n "$SA_KEY" ] || fail_exit "Service account creation failed: $SA_RESP"
pass "Service account created: $SA_ID (username: e2e-smtp)"

echo ""
info "SMTP login: e2e-smtp@$GRP_ID"
echo ""

# =============================================================================
# Step 3: SMTP Send Tests (positive cases)
# =============================================================================
info "Step 3: SMTP send tests"
sleep 2

# 3.1 Simple case
info "3.1 Simple email (from, to, subject, body)"
smtp_auth_test "e2e-smtp@$GRP_ID" "$SA_KEY" > /dev/null
pass "Simple email sent successfully"

# 3.2 Complex case
info "3.2 Complex email (from, to, cc, bcc, html, attachments)"
mkdir -p test-data
echo "This is a test attachment file for E2E testing." > test-data/sample.txt
echo "<html><body><h1>HTML Attachment</h1></body></html>" > test-data/sample.html

docker compose run --rm -T test-client \
  --host=smtp-server --port=587 --tls=starttls --insecure \
  --user="e2e-smtp@$GRP_ID" --password="$SA_KEY" \
  --from="sender@example.com" \
  --to="recipient@example.com" \
  --cc="cc-user@example.com" \
  --bcc="bcc-user@example.com" \
  --subject="E2E Complex Test" \
  --body="Complex case with CC, BCC, and attachments." \
  --html="<h1>E2E Complex Test</h1><p>With HTML body.</p>" \
  --attach=/test-data/sample.txt \
  --attach=/test-data/sample.html > /dev/null
pass "Complex email sent successfully"

# =============================================================================
# Step 4: SMTP Auth Negative Cases
# =============================================================================
info "Step 4: SMTP auth negative cases"

# 4.1 Human user cannot SMTP auth
info "4.1 Human user SMTP auth (should fail)"
if smtp_auth_test "testuser@example.com@$GRP_ID" "testpass123" > /dev/null 2>&1; then
  fail "Human user should NOT be able to SMTP auth"
else
  pass "Human user SMTP auth correctly rejected"
fi

# 4.2 Suspended service account cannot SMTP auth
info "4.2 Suspended service account SMTP auth (should fail)"
# Suspend the service account
SUSPEND_RESP=$(curl -s -X PATCH "$API/users/$SA_ID/status" \
  -H "$GRP_AUTH" -H 'Content-Type: application/json' \
  -d '{"status":"suspended"}')
SUSPEND_STATUS=$(jv status "$SUSPEND_RESP")
[ "$SUSPEND_STATUS" = "suspended" ] || fail "Failed to suspend user: $SUSPEND_RESP"

if smtp_auth_test "e2e-smtp@$GRP_ID" "$SA_KEY" > /dev/null 2>&1; then
  fail "Suspended account should NOT be able to SMTP auth"
else
  pass "Suspended account SMTP auth correctly rejected"
fi

# Reactivate for subsequent tests
ACTIVATE_RESP=$(curl -s -X PATCH "$API/users/$SA_ID/status" \
  -H "$GRP_AUTH" -H 'Content-Type: application/json' \
  -d '{"status":"active"}')
ACTIVATE_STATUS=$(jv status "$ACTIVATE_RESP")
[ "$ACTIVATE_STATUS" = "active" ] || fail_exit "Failed to reactivate user: $ACTIVATE_RESP"
info "Service account reactivated"

# 4.3 Expired API key cannot SMTP auth
info "4.3 Expired API key SMTP auth (should fail)"
# Directly set api_key_expires_at to the past via DB
docker compose exec -T postgres psql -U smtp_proxy -d smtp_proxy -c \
  "UPDATE users SET api_key_expires_at = NOW() - INTERVAL '1 day' WHERE id = $SA_ID;" > /dev/null

if smtp_auth_test "e2e-smtp@$GRP_ID" "$SA_KEY" > /dev/null 2>&1; then
  fail "Expired API key should NOT be able to SMTP auth"
else
  pass "Expired API key SMTP auth correctly rejected"
fi

# Restore expiration to future
docker compose exec -T postgres psql -U smtp_proxy -d smtp_proxy -c \
  "UPDATE users SET api_key_expires_at = NOW() + INTERVAL '30 days' WHERE id = $SA_ID;" > /dev/null
info "API key expiration restored"

# Verify SMTP auth works again after restoring expiration
if smtp_auth_test "e2e-smtp@$GRP_ID" "$SA_KEY" > /dev/null 2>&1; then
  pass "SMTP auth works after restoring expiration"
else
  fail "SMTP auth should work after restoring expiration"
fi

# =============================================================================
# Step 5: Provider Access Control
# =============================================================================
info "Step 5: Provider access control"

# Switch to group B context
SWB_RESP=$(curl -s -X POST "$API/auth/switch-group" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"group_id\":\"$GRPB_ID\"}")
GRPB_TOKEN=$(jv access_token "$SWB_RESP")
[ -n "$GRPB_TOKEN" ] || fail_exit "Group B switch failed: $SWB_RESP"
GRPB_AUTH="Authorization: Bearer $GRPB_TOKEN"

# 5.1 Private provider from group A is NOT accessible to group B
info "5.1 Private provider (group A) not accessible from group B"
SA_CROSS_RESP=$(curl -s -X POST "$API/groups/$GRPB_ID/service-accounts" \
  -H "$GRPB_AUTH" -H 'Content-Type: application/json' \
  -d "{\"username\":\"cross-test\",\"provider_id\":\"$P3_ID\"}")
CROSS_ERR=$(jv_err "$SA_CROSS_RESP")
if echo "$CROSS_ERR" | grep -qi "not accessible\|not found"; then
  pass "Private provider correctly blocked for other group"
else
  fail "Private provider should NOT be accessible to group B: $SA_CROSS_RESP"
fi

# 5.2 Global provider IS accessible to group B
info "5.2 Global provider accessible from group B"
SA_GLOBAL_RESP=$(curl -s -X POST "$API/groups/$GRPB_ID/service-accounts" \
  -H "$GRPB_AUTH" -H 'Content-Type: application/json' \
  -d "{\"username\":\"global-test\",\"provider_id\":\"$P2_ID\"}")
SA_GLOBAL_ID=$(jv id "$SA_GLOBAL_RESP")
if [ -n "$SA_GLOBAL_ID" ]; then
  pass "Global provider correctly accessible to group B"
else
  fail "Global provider should be accessible to group B: $SA_GLOBAL_RESP"
fi

# 5.3 Shared provider NOT accessible before grant
info "5.3 Shared provider not accessible before grant"
SA_SHARED_RESP=$(curl -s -X POST "$API/groups/$GRPB_ID/service-accounts" \
  -H "$GRPB_AUTH" -H 'Content-Type: application/json' \
  -d "{\"username\":\"shared-test\",\"provider_id\":\"$P4_ID\"}")
SHARED_ERR=$(jv_err "$SA_SHARED_RESP")
if echo "$SHARED_ERR" | grep -qi "not accessible\|not found"; then
  pass "Shared provider correctly blocked before access grant"
else
  fail "Shared provider should NOT be accessible before grant: $SA_SHARED_RESP"
fi

# 5.4 Grant shared provider access to group B, then verify access
info "5.4 Grant shared provider access to group B"
GRANT_RESP=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$API/providers/$P4_ID/access" \
  -H "$AUTH" -H 'Content-Type: application/json' \
  -d "{\"group_id\":\"$GRPB_ID\"}")
if [ "$GRANT_RESP" = "204" ] || [ "$GRANT_RESP" = "200" ] || [ "$GRANT_RESP" = "201" ]; then
  pass "Shared provider access granted to group B"
else
  fail "Failed to grant shared provider access (HTTP $GRANT_RESP)"
fi

# 5.5 Shared provider IS accessible after grant
info "5.5 Shared provider accessible after grant"
SA_SHARED2_RESP=$(curl -s -X POST "$API/groups/$GRPB_ID/service-accounts" \
  -H "$GRPB_AUTH" -H 'Content-Type: application/json' \
  -d "{\"username\":\"shared-after-grant\",\"provider_id\":\"$P4_ID\"}")
SA_SHARED2_ID=$(jv id "$SA_SHARED2_RESP")
if [ -n "$SA_SHARED2_ID" ]; then
  pass "Shared provider correctly accessible after grant"
else
  fail "Shared provider should be accessible after grant: $SA_SHARED2_RESP"
fi

# 5.6 Revoke shared provider access, verify blocked again
info "5.6 Revoke shared provider access from group B"
REVOKE_RESP=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$API/providers/$P4_ID/access/$GRPB_ID" \
  -H "$AUTH")
if [ "$REVOKE_RESP" = "204" ] || [ "$REVOKE_RESP" = "200" ]; then
  pass "Shared provider access revoked from group B"
else
  fail "Failed to revoke shared provider access (HTTP $REVOKE_RESP)"
fi

SA_SHARED3_RESP=$(curl -s -X POST "$API/groups/$GRPB_ID/service-accounts" \
  -H "$GRPB_AUTH" -H 'Content-Type: application/json' \
  -d "{\"username\":\"shared-after-revoke\",\"provider_id\":\"$P4_ID\"}")
SHARED3_ERR=$(jv_err "$SA_SHARED3_RESP")
if echo "$SHARED3_ERR" | grep -qi "not accessible\|not found"; then
  pass "Shared provider correctly blocked after revoke"
else
  fail "Shared provider should NOT be accessible after revoke: $SA_SHARED3_RESP"
fi

# =============================================================================
# Step 6: API Key Reset
# =============================================================================
info "Step 6: API key reset"

OLD_KEY="$SA_KEY"

# 6.1 Reset API key
info "6.1 Reset service account API key"
RESET_RESP=$(curl -s -X POST "$API/groups/$GRP_ID/service-accounts/$SA_ID/reset-api-key" \
  -H "$GRP_AUTH" -H 'Content-Type: application/json' \
  -d '{"api_key_expires_in":"30d"}')
NEW_KEY=$(jv api_key "$RESET_RESP")
[ -n "$NEW_KEY" ] || fail_exit "API key reset failed: $RESET_RESP"
pass "API key reset successful (new key: ${NEW_KEY:0:12}...)"

# 6.2 Old key should NOT work
info "6.2 Old API key SMTP auth (should fail)"
if smtp_auth_test "e2e-smtp@$GRP_ID" "$OLD_KEY" > /dev/null 2>&1; then
  fail "Old API key should NOT work after reset"
else
  pass "Old API key correctly rejected after reset"
fi

# 6.3 New key SHOULD work
info "6.3 New API key SMTP auth (should succeed)"
if smtp_auth_test "e2e-smtp@$GRP_ID" "$NEW_KEY" > /dev/null 2>&1; then
  pass "New API key works after reset"
else
  fail "New API key should work after reset"
fi

# =============================================================================
# Step 7: Activity Log Verification
# =============================================================================
info "Step 7: Activity log verification"

ACTIVITY_RESP=$(curl -s "$API/groups/$GRP_ID/activity?limit=20" -H "$GRP_AUTH")
ACTIVITY_COUNT=$(python3 -c "import sys,json; print(len(json.load(sys.stdin)))" <<< "$ACTIVITY_RESP" 2>/dev/null || echo "0")

if [ "$ACTIVITY_COUNT" -gt 0 ]; then
  pass "Activity logs recorded ($ACTIVITY_COUNT entries)"
else
  fail "Activity logs should have entries after all operations"
fi

# =============================================================================
# Results
# =============================================================================
echo ""
sleep 2
info "Delivery logs (last 10):"
echo "---"
docker compose logs --tail=20 queue-worker 2>&1 | grep -i "delivered" || echo "(no delivery logs)"
echo "---"

echo ""
TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo -e "${CYAN}=============================================${NC}"
echo -e "  Results: ${GREEN}$PASS_COUNT passed${NC}, ${RED}$FAIL_COUNT failed${NC} / $TOTAL total"
echo -e "${CYAN}=============================================${NC}"
echo ""
echo "Test Data:"
echo "  Group A: e2e-test-group-a ($GRP_ID)"
echo "  Group B: e2e-test-group-b ($GRPB_ID)"
echo "  Provider 1: private (system) | Provider 2: global"
echo "  Provider 3: private (group A) | Provider 4: shared"
echo "  Service account: e2e-smtp@$GRP_ID"

if [ "$FAIL_COUNT" -gt 0 ]; then
  exit 1
fi
