#!/usr/bin/env bash
# Smoke test for network filtering (022-network-filtering)
# Requires: cc-deck binary built, podman running
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CC_DECK="${SCRIPT_DIR}/../cc-deck"
BUILD_DIR=$(mktemp -d)
DEPLOY_DIR=$(mktemp -d)
SESSION="smoke-test-$$"

PASS=0
FAIL=0
ERRORS=()

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BOLD='\033[1m'
RESET='\033[0m'

pass() { ((PASS++)); echo -e "  ${GREEN}PASS${RESET} $1"; }
fail() { ((FAIL++)); ERRORS+=("$1"); echo -e "  ${RED}FAIL${RESET} $1"; }
section() { echo -e "\n${BOLD}=== $1 ===${RESET}"; }

# check "description" grep_args...
# Runs grep and passes/fails based on exit code
check() {
    local desc="$1"; shift
    if "$@" >/dev/null 2>&1; then
        pass "$desc"
    else
        fail "$desc"
    fi
}

# check_not "description" grep_args...
# Passes when the command FAILS (e.g., domain NOT found)
check_not() {
    local desc="$1"; shift
    if "$@" >/dev/null 2>&1; then
        fail "$desc"
    else
        pass "$desc"
    fi
}

cleanup() {
    echo ""
    section "Cleanup"
    podman rm -f "${SESSION}" "${SESSION}-proxy" 2>/dev/null || true
    podman network ls --format '{{.Name}}' 2>/dev/null | grep "${SESSION}" | xargs -r podman network rm 2>/dev/null || true
    rm -rf "$BUILD_DIR" "$DEPLOY_DIR"
    echo "  Cleaned up temp dirs and containers"
}
trap cleanup EXIT

# Check prerequisites
if ! command -v podman &>/dev/null; then
    echo "ERROR: podman is required" && exit 1
fi
if [[ ! -x "$CC_DECK" ]]; then
    echo "ERROR: cc-deck binary not found at $CC_DECK"
    echo "  Run: cd cc-deck && go build ./cmd/cc-deck/"
    exit 1
fi

# Prepare test manifest
cat > "$BUILD_DIR/cc-deck-build.yaml" <<EOF
version: 1
image:
  name: quay.io/cc-deck/cc-deck-demo
network:
  allowed_domains:
    - github
    - python
EOF

# ============================================================
section "1. Domain CLI commands"
# ============================================================

output=$("$CC_DECK" domains list 2>&1)
if echo "$output" | grep -qF "python"; then pass "domains list shows python"; else fail "domains list missing python"; fi
if echo "$output" | grep -qF "anthropic"; then pass "domains list shows anthropic"; else fail "domains list missing anthropic"; fi
if echo "$output" | grep -qF "golang"; then pass "domains list shows golang"; else fail "domains list missing golang"; fi

output=$("$CC_DECK" domains show python 2>&1)
if echo "$output" | grep -qF "pypi.org"; then pass "domains show python includes pypi.org"; else fail "domains show python missing pypi.org"; fi

output=$("$CC_DECK" domains show anthropic 2>&1)
if echo "$output" | grep -qF "platform.claude.com"; then pass "domains show anthropic includes platform.claude.com"; else fail "domains show anthropic missing platform.claude.com"; fi
if echo "$output" | grep -qF "statsigapi.net"; then pass "domains show anthropic includes statsigapi.net"; else fail "domains show anthropic missing statsigapi.net"; fi

# ============================================================
section "2. Compose generation"
# ============================================================

"$CC_DECK" deploy "$SESSION" --compose "$BUILD_DIR" --output-dir "$DEPLOY_DIR"

check "compose.yaml generated" test -f "$DEPLOY_DIR/compose.yaml"
check ".env.example generated" test -f "$DEPLOY_DIR/.env.example"
check "tinyproxy.conf generated" test -f "$DEPLOY_DIR/proxy/tinyproxy.conf"
check "whitelist generated" test -f "$DEPLOY_DIR/proxy/whitelist"

check "FilterDefaultDeny enabled" grep -qF "FilterDefaultDeny Yes" "$DEPLOY_DIR/proxy/tinyproxy.conf"
check "FilterExtended enabled" grep -qF "FilterExtended On" "$DEPLOY_DIR/proxy/tinyproxy.conf"
check_not "No LogFile (logs to stdout)" grep -qF "LogFile" "$DEPLOY_DIR/proxy/tinyproxy.conf"

check "whitelist contains python domains" grep -qF "pypi" "$DEPLOY_DIR/proxy/whitelist"
check "whitelist contains github domains" grep -qF "github" "$DEPLOY_DIR/proxy/whitelist"
check "whitelist contains anthropic (auto-injected FR-002)" grep -qF "anthropic" "$DEPLOY_DIR/proxy/whitelist"

check "session uses sleep infinity" grep -qF "sleep infinity" "$DEPLOY_DIR/compose.yaml"
check "internal network defined" grep -qF "internal" "$DEPLOY_DIR/compose.yaml"

# ============================================================
section "3. Deploy-time overrides"
# ============================================================

OVERRIDE_DIR=$(mktemp -d)

# +rust (add)
"$CC_DECK" deploy "$SESSION" --compose "$BUILD_DIR" --output-dir "$OVERRIDE_DIR/add" --allowed-domains +rust
check "+rust adds crates.io" grep -qF "crates" "$OVERRIDE_DIR/add/proxy/whitelist"
check "+rust preserves python" grep -qF "pypi" "$OVERRIDE_DIR/add/proxy/whitelist"

# -python (remove)
"$CC_DECK" deploy "$SESSION" --compose "$BUILD_DIR" --output-dir "$OVERRIDE_DIR/remove" --allowed-domains -python
check_not "-python removes pypi.org" grep -qF "pypi" "$OVERRIDE_DIR/remove/proxy/whitelist"
check "-python preserves github" grep -qF "github" "$OVERRIDE_DIR/remove/proxy/whitelist"

# bare replace
"$CC_DECK" deploy "$SESSION" --compose "$BUILD_DIR" --output-dir "$OVERRIDE_DIR/replace" --allowed-domains golang
check "bare replace has golang" grep -qF "golang" "$OVERRIDE_DIR/replace/proxy/whitelist"
check_not "bare replace dropped python" grep -qF "pypi" "$OVERRIDE_DIR/replace/proxy/whitelist"

# all (disable)
"$CC_DECK" deploy "$SESSION" --compose "$BUILD_DIR" --output-dir "$OVERRIDE_DIR/all" --allowed-domains all 2>&1 || true
check_not "--allowed-domains all: no proxy dir" test -d "$OVERRIDE_DIR/all/proxy"

rm -rf "$OVERRIDE_DIR"

# ============================================================
section "4. Error handling"
# ============================================================

ERR_OUTPUT=$("$CC_DECK" deploy "$SESSION" --compose "$BUILD_DIR" --output-dir /tmp/cc-err --allowed-domains +bogusgroup 2>&1 || true)
if echo "$ERR_OUTPUT" | grep -qiE "unknown|not found|available"; then
    pass "unknown group reports error"
else
    fail "unknown group should report error"
fi
rm -rf /tmp/cc-err

# ============================================================
section "5. Container integration"
# ============================================================

# Create .env (empty is fine for this test, we just need containers to start)
cp "$DEPLOY_DIR/.env.example" "$DEPLOY_DIR/.env"

cd "$DEPLOY_DIR"
podman-compose up -d 2>&1 | tail -3

# Wait for proxy to be healthy
echo "  Waiting for proxy to start..."
for i in $(seq 1 15); do
    if podman exec "${SESSION}-proxy" true 2>/dev/null; then
        break
    fi
    sleep 1
done

# Check proxy is running
if podman exec "${SESSION}-proxy" true 2>/dev/null; then
    pass "proxy container running"
else
    fail "proxy container not running"
    podman logs "${SESSION}-proxy" 2>&1 | tail -5
    echo ""
    section "Results"
    echo -e "  ${GREEN}Passed: $PASS${RESET}  ${RED}Failed: $FAIL${RESET}"
    exit 1
fi

# Test allowed domain
HTTP_CODE=$(podman exec "$SESSION" curl -s -o /dev/null -w "%{http_code}" --max-time 10 https://pypi.org 2>/dev/null || echo "000")
[[ "$HTTP_CODE" == "200" || "$HTTP_CODE" == "301" || "$HTTP_CODE" == "302" ]] && pass "pypi.org allowed (HTTP $HTTP_CODE)" || fail "pypi.org blocked (HTTP $HTTP_CODE)"

HTTP_CODE=$(podman exec "$SESSION" curl -s -o /dev/null -w "%{http_code}" --max-time 10 https://github.com 2>/dev/null || echo "000")
[[ "$HTTP_CODE" == "200" || "$HTTP_CODE" == "301" || "$HTTP_CODE" == "302" ]] && pass "github.com allowed (HTTP $HTTP_CODE)" || fail "github.com blocked (HTTP $HTTP_CODE)"

# Test blocked domain (example.com is a real, resolvable domain not in our whitelist)
HTTP_CODE=$(podman exec "$SESSION" curl -s -o /dev/null -w "%{http_code}" --max-time 10 https://example.com 2>/dev/null || echo "000")
[[ "$HTTP_CODE" != "200" ]] && pass "example.com blocked (HTTP $HTTP_CODE)" || fail "example.com should be blocked (HTTP $HTTP_CODE)"

# ============================================================
section "6. Audit blocked requests"
# ============================================================

cd "$SCRIPT_DIR/.."
BLOCKED_OUTPUT=$("$CC_DECK" domains blocked "$SESSION" 2>&1)
if echo "$BLOCKED_OUTPUT" | grep -qF "example.com"; then
    pass "domains blocked shows example.com"
else
    fail "domains blocked missing example.com"
fi

# ============================================================
section "7. Live add/remove"
# ============================================================

"$CC_DECK" domains add "$SESSION" example.com
sleep 2

HTTP_CODE=$(podman exec "$SESSION" curl -s -o /dev/null -w "%{http_code}" --max-time 10 https://example.com 2>/dev/null || echo "000")
[[ "$HTTP_CODE" == "200" || "$HTTP_CODE" == "301" || "$HTTP_CODE" == "302" ]] && pass "example.com allowed after add (HTTP $HTTP_CODE)" || fail "example.com still blocked after add (HTTP $HTTP_CODE)"

"$CC_DECK" domains remove "$SESSION" example.com
sleep 2

HTTP_CODE=$(podman exec "$SESSION" curl -s -o /dev/null -w "%{http_code}" --max-time 10 https://example.com 2>/dev/null || echo "000")
[[ "$HTTP_CODE" != "200" ]] && pass "example.com blocked after remove (HTTP $HTTP_CODE)" || fail "example.com should be blocked after remove (HTTP $HTTP_CODE)"

# ============================================================
section "Results"
# ============================================================

TOTAL=$((PASS + FAIL))
echo -e "  ${GREEN}Passed: $PASS${RESET} / $TOTAL"
if [[ $FAIL -gt 0 ]]; then
    echo -e "  ${RED}Failed: $FAIL${RESET}"
    for e in "${ERRORS[@]}"; do
        echo -e "    ${RED}- $e${RESET}"
    done
    exit 1
else
    echo -e "  ${GREEN}All tests passed!${RESET}"
fi
